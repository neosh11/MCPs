package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/neosh11/MCPs/openapi-mcp/catalog"
)

type manifest struct {
	Name                   string            `json:"name"`
	Spec                   string            `json:"spec"`
	SpecVersion            string            `json:"specVersion"`
	BaseURLs               map[string]string `json:"baseUrls"`
	Auth                   string            `json:"auth"`
	Headers                map[string]string `json:"headers"`
	ServiceFrom            string            `json:"serviceFrom"`
	MethodKey              string            `json:"methodKey"`
	WriteMethods           []string          `json:"writeMethods"`
	ExcludeReleaseStatuses []string          `json:"excludeReleaseStatuses"`
	ExcludeServices        []string          `json:"excludeServices"`
}

type openAPI struct {
	OpenAPI    string                   `json:"openapi"`
	Info       info                     `json:"info"`
	Paths      map[string]map[string]op `json:"paths"`
	Components components               `json:"components"`
}

type info struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

type components struct {
	Schemas map[string]schema `json:"schemas"`
}

type op struct {
	Tags        []string     `json:"tags"`
	Summary     string       `json:"summary"`
	OperationID string       `json:"operationId"`
	Description string       `json:"description"`
	Parameters  []parameter  `json:"parameters"`
	RequestBody *requestBody `json:"requestBody"`
	Release     string       `json:"x-release-status"`
}

type parameter struct {
	Name        string `json:"name"`
	In          string `json:"in"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Schema      schema `json:"schema"`
}

type requestBody struct {
	Required bool                 `json:"required"`
	Content  map[string]mediaType `json:"content"`
}

type mediaType struct {
	Schema schema `json:"schema"`
}

type schema struct {
	Ref         string            `json:"$ref"`
	Type        string            `json:"type"`
	Format      string            `json:"format"`
	Description string            `json:"description"`
	ReadOnly    bool              `json:"readOnly"`
	Required    []string          `json:"required"`
	Properties  map[string]schema `json:"properties"`
	Items       *schema           `json:"items"`
	Additional  any               `json:"additionalProperties"`
	AllOf       []schema          `json:"allOf"`
	OneOf       []schema          `json:"oneOf"`
	AnyOf       []schema          `json:"anyOf"`
}

func main() {
	manifestPath := flag.String("manifest", "", "manifest JSON path")
	outPath := flag.String("out", "", "catalog output path")
	flag.Parse()
	if *manifestPath == "" || *outPath == "" {
		fmt.Fprintln(os.Stderr, "usage: openapi-mcp-gen -manifest manifest.json -out catalog.json")
		os.Exit(2)
	}
	if err := run(*manifestPath, *outPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(manifestPath, outPath string) error {
	manBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var man manifest
	if err := json.Unmarshal(manBytes, &man); err != nil {
		return err
	}
	specPath := man.Spec
	if !filepath.IsAbs(specPath) {
		specPath = filepath.Join(filepath.Dir(manifestPath), "..", specPath)
	}
	specBytes, err := os.ReadFile(filepath.Clean(specPath))
	if err != nil {
		return err
	}
	var spec openAPI
	if err := json.Unmarshal(specBytes, &spec); err != nil {
		return err
	}
	cat := generate(spec, man, man.Spec)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return catalog.Write(f, cat)
}

func generate(spec openAPI, man manifest, specPath string) *catalog.Catalog {
	writeMethods := map[string]bool{}
	for _, method := range man.WriteMethods {
		writeMethods[strings.ToLower(method)] = true
	}
	excluded := map[string]bool{}
	for _, service := range man.ExcludeServices {
		excluded[serviceKey(service)] = true
	}
	excludedStatuses := map[string]bool{}
	for _, status := range man.ExcludeReleaseStatuses {
		excludedStatuses[strings.ToUpper(status)] = true
	}
	cat := &catalog.Catalog{
		API: catalog.APIInfo{
			Name:        spec.Info.Title,
			Description: spec.Info.Description,
			SpecVersion: man.SpecVersion,
			SpecSource:  specPath,
		},
		Services: map[string]catalog.Service{},
		Types:    map[string][]catalog.Type{},
	}
	if cat.API.Name == "" {
		cat.API.Name = man.Name
	}

	paths := sortedKeys(spec.Paths)
	for _, path := range paths {
		methods := sortedKeys(spec.Paths[path])
		for _, httpMethod := range methods {
			if !isHTTPMethod(httpMethod) {
				continue
			}
			operation := spec.Paths[path][httpMethod]
			if operation.OperationID == "" {
				continue
			}
			if excludedStatuses[strings.ToUpper(operation.Release)] {
				continue
			}
			tag := firstTag(operation.Tags)
			key := serviceKey(tag)
			if key == "" || excluded[key] {
				continue
			}
			svc := cat.Services[key]
			if svc.Name == "" {
				svc.Name = tag
				svc.Methods = map[string]catalog.Method{}
			}
			description := firstNonEmpty(operation.Description, operation.Summary, operation.OperationID)
			pathParams, queryParams := splitParams(operation.Parameters)
			requestType, requestTypes := buildRequestTypes(operation, pathParams, queryParams, spec.Components.Schemas)
			methodKey := uniqueMethodKey(svc.Methods, deriveMethodKey(operation.OperationID, tag, path))
			svc.Methods[methodKey] = catalog.Method{
				Description:  description,
				HTTPMethod:   strings.ToLower(httpMethod),
				Path:         path,
				PathParams:   pathParams,
				QueryParams:  queryParams,
				RequestType:  requestType,
				IsWrite:      writeMethods[strings.ToLower(httpMethod)],
				IsMultipart:  isMultipart(operation),
				OriginalName: operation.OperationID,
			}
			cat.Services[key] = svc
			cat.Types[requestType] = requestTypes
		}
	}
	return cat
}

func splitParams(params []parameter) ([]catalog.Parameter, []catalog.Parameter) {
	pathParams := []catalog.Parameter{}
	queryParams := []catalog.Parameter{}
	for _, p := range params {
		out := catalog.Parameter{
			Name:        p.Name,
			Type:        schemaType(p.Schema),
			Description: p.Description,
			Required:    p.Required || p.In == "path",
		}
		switch p.In {
		case "path":
			pathParams = append(pathParams, out)
		case "query":
			queryParams = append(queryParams, out)
		}
	}
	return pathParams, queryParams
}

func buildRequestTypes(operation op, pathParams, queryParams []catalog.Parameter, schemas map[string]schema) (string, []catalog.Type) {
	requestType := operation.OperationID + "Request"
	bodySchema := schema{}
	if operation.RequestBody != nil {
		for contentType, media := range operation.RequestBody.Content {
			if strings.Contains(contentType, "json") || strings.Contains(contentType, "multipart") {
				bodySchema = media.Schema
				break
			}
		}
		if name := refName(bodySchema.Ref); name != "" {
			requestType = name
		}
	}

	required := map[string]bool{}
	props := make([]catalog.Property, 0, len(pathParams)+len(queryParams))
	for _, p := range append(pathParams, queryParams...) {
		props = append(props, catalog.Property{
			Name:        p.Name,
			Type:        p.Type,
			Description: p.Description,
			Required:    p.Required,
			ReadOnly:    false,
		})
		required[p.Name] = p.Required
	}

	seen := map[string]bool{}
	var types []catalog.Type
	if resolved, ok := resolveSchema(bodySchema, schemas); ok {
		for _, name := range resolved.Required {
			required[name] = true
		}
		for _, propName := range sortedKeys(resolved.Properties) {
			propSchema := resolved.Properties[propName]
			props = append(props, propertyFromSchema(propName, propSchema, required[propName]))
		}
		types = appendNestedTypes(types, resolved, schemas, seen)
	}
	types = append([]catalog.Type{{Name: requestType, Properties: props}}, types...)
	return requestType, types
}

func appendNestedTypes(types []catalog.Type, s schema, schemas map[string]schema, seen map[string]bool) []catalog.Type {
	for _, propName := range sortedKeys(s.Properties) {
		prop := s.Properties[propName]
		names := referencedSchemaNames(prop)
		for _, name := range names {
			if seen[name] {
				continue
			}
			nested, ok := schemas[name]
			if !ok {
				continue
			}
			seen[name] = true
			resolved, _ := resolveSchema(nested, schemas)
			types = append(types, typeFromSchema(name, resolved))
		}
	}
	return types
}

func typeFromSchema(name string, s schema) catalog.Type {
	required := map[string]bool{}
	for _, r := range s.Required {
		required[r] = true
	}
	props := make([]catalog.Property, 0, len(s.Properties))
	for _, propName := range sortedKeys(s.Properties) {
		props = append(props, propertyFromSchema(propName, s.Properties[propName], required[propName]))
	}
	return catalog.Type{Name: name, Properties: props}
}

func propertyFromSchema(name string, s schema, required bool) catalog.Property {
	arrayType := (*string)(nil)
	if s.Type == "array" && s.Items != nil {
		itemType := schemaType(*s.Items)
		arrayType = &itemType
	}
	return catalog.Property{
		Name:        name,
		Type:        schemaType(s),
		Description: s.Description,
		Required:    required,
		ReadOnly:    s.ReadOnly,
		ArrayType:   arrayType,
		IsFile:      s.Format == "binary",
	}
}

func resolveSchema(s schema, schemas map[string]schema) (schema, bool) {
	if name := refName(s.Ref); name != "" {
		resolved, ok := schemas[name]
		return resolved, ok
	}
	if len(s.AllOf) > 0 {
		return mergeSchemas(s.AllOf, schemas), true
	}
	if len(s.Properties) > 0 {
		return s, true
	}
	return schema{}, false
}

func mergeSchemas(parts []schema, schemas map[string]schema) schema {
	merged := schema{Type: "object", Properties: map[string]schema{}}
	for _, part := range parts {
		resolved, ok := resolveSchema(part, schemas)
		if !ok {
			continue
		}
		merged.Required = append(merged.Required, resolved.Required...)
		for k, v := range resolved.Properties {
			merged.Properties[k] = v
		}
	}
	return merged
}

func schemaType(s schema) string {
	if name := refName(s.Ref); name != "" {
		return name
	}
	if s.Type == "array" {
		return "array"
	}
	if s.Type != "" {
		return s.Type
	}
	if len(s.Properties) > 0 {
		return "object"
	}
	if len(s.OneOf) > 0 || len(s.AnyOf) > 0 || len(s.AllOf) > 0 {
		return "object"
	}
	return "string"
}

func referencedSchemaNames(s schema) []string {
	var names []string
	if name := refName(s.Ref); name != "" {
		names = append(names, name)
	}
	if s.Items != nil {
		names = append(names, referencedSchemaNames(*s.Items)...)
	}
	for _, part := range append(append(s.AllOf, s.OneOf...), s.AnyOf...) {
		names = append(names, referencedSchemaNames(part)...)
	}
	sort.Strings(names)
	return names
}

func isMultipart(operation op) bool {
	if operation.RequestBody == nil {
		return false
	}
	for contentType := range operation.RequestBody.Content {
		if strings.Contains(contentType, "multipart") {
			return true
		}
	}
	return false
}

func deriveMethodKey(operationID, tag, path string) string {
	name := operationID
	tag = strings.ReplaceAll(tag, " ", "")
	for _, prefix := range []string{"List", "Retrieve", "Get", "Create", "Update", "Delete", "Search", "Cancel", "Bulk"} {
		if strings.HasPrefix(name, prefix) {
			remainder := strings.TrimPrefix(name, prefix)
			switch prefix {
			case "Retrieve":
				return methodWithSuffix("get", remainder, tag, path)
			case "Get":
				return methodWithSuffix("get", remainder, tag, path)
			case "List":
				return methodWithSuffix("list", remainder, tag, path)
			default:
				return methodWithSuffix(strings.ToLower(prefix), remainder, tag, path)
			}
		}
	}
	name = strings.TrimPrefix(name, tag)
	return lowerCamel(name)
}

func methodWithSuffix(base, remainder, tag, path string) string {
	remainder = trimServiceName(remainder, tag)
	if remainder == "" {
		return base
	}
	if strings.Contains(path, "/by-") || strings.Contains(remainder, "By") {
		return base + remainder
	}
	return base
}

func trimServiceName(name, tag string) string {
	candidates := []string{tag, singular(tag)}
	for _, candidate := range candidates {
		if candidate != "" && strings.HasPrefix(name, candidate) {
			return strings.TrimPrefix(name, candidate)
		}
	}
	return name
}

func singular(name string) string {
	switch {
	case strings.HasSuffix(name, "ies"):
		return strings.TrimSuffix(name, "ies") + "y"
	case strings.HasSuffix(name, "s"):
		return strings.TrimSuffix(name, "s")
	default:
		return name
	}
}

func uniqueMethodKey(existing map[string]catalog.Method, key string) string {
	if _, ok := existing[key]; !ok {
		return key
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s%d", key, i)
		if _, ok := existing[candidate]; !ok {
			return candidate
		}
	}
}

var nonAlphaNum = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func serviceKey(name string) string {
	return strings.ToLower(nonAlphaNum.ReplaceAllString(name, ""))
}

func lowerCamel(name string) string {
	if name == "" {
		return ""
	}
	return strings.ToLower(name[:1]) + name[1:]
}

func firstTag(tags []string) string {
	if len(tags) == 0 {
		return "default"
	}
	return tags[0]
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func refName(ref string) string {
	if ref == "" {
		return ""
	}
	const prefix = "#/components/schemas/"
	if strings.HasPrefix(ref, prefix) {
		return strings.TrimPrefix(ref, prefix)
	}
	parts := strings.Split(ref, "/")
	return parts[len(parts)-1]
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func isHTTPMethod(method string) bool {
	switch strings.ToLower(method) {
	case "get", "post", "put", "patch", "delete":
		return true
	default:
		return false
	}
}

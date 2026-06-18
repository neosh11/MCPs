package openapimcp

import (
	"context"
	"fmt"
	"sort"

	"github.com/neosh11/MCPs/openapi-mcp/catalog"
)

type Toolset struct {
	client *Client
}

type ToolDefinition struct {
	Name        string `json:"name"`
	Service     string `json:"service"`
	Method      string `json:"method"`
	Description string `json:"description"`
	HTTPMethod  string `json:"httpMethod"`
	Path        string `json:"path"`
	RequestType string `json:"requestType"`
	IsWrite     bool   `json:"isWrite"`
	IsMultipart bool   `json:"isMultipart"`
}

type Tool struct {
	ToolDefinition
	Types []catalog.Type
	call  func(context.Context, map[string]any) (string, error)
}

func NewToolset(cat *catalog.Catalog, cfg Config) (*Toolset, error) {
	client, err := New(cat, cfg)
	if err != nil {
		return nil, err
	}
	return &Toolset{client: client}, nil
}

func (t *Toolset) ListTools() []ToolDefinition {
	var tools []ToolDefinition
	for _, serviceKey := range t.client.catalog.ServiceKeys() {
		svc := t.client.catalog.Services[serviceKey]
		for _, methodKey := range svc.MethodKeys() {
			method := svc.Methods[methodKey]
			tools = append(tools, ToolDefinition{
				Name:        serviceKey + "." + methodKey,
				Service:     serviceKey,
				Method:      methodKey,
				Description: method.Description,
				HTTPMethod:  method.HTTPMethod,
				Path:        method.Path,
				RequestType: method.RequestType,
				IsWrite:     method.IsWrite,
				IsMultipart: method.IsMultipart,
			})
		}
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	return tools
}

func (t *Toolset) GetTool(service, method string) (Tool, error) {
	def, err := t.definition(service, method)
	if err != nil {
		return Tool{}, err
	}
	types, err := t.client.GetTypeInfo(service, method)
	if err != nil {
		return Tool{}, err
	}
	return Tool{
		ToolDefinition: def,
		Types:          types,
		call: func(ctx context.Context, request map[string]any) (string, error) {
			return t.client.MakeAPIRequest(ctx, service, method, request)
		},
	}, nil
}

func (t Tool) Call(ctx context.Context, request map[string]any) (string, error) {
	if t.call == nil {
		return "", fmt.Errorf("tool %q is not bound to a caller", t.Name)
	}
	return t.call(ctx, request)
}

func (t *Toolset) CallTool(ctx context.Context, service, method string, request map[string]any) (string, error) {
	return t.client.MakeAPIRequest(ctx, service, method, request)
}

func (t *Toolset) ToolTypes(service, method string) ([]catalog.Type, error) {
	return t.client.GetTypeInfo(service, method)
}

func (t *Toolset) definition(service, method string) (ToolDefinition, error) {
	m, err := t.client.lookupMethod(service, method)
	if err != nil {
		return ToolDefinition{}, err
	}
	return ToolDefinition{
		Name:        service + "." + method,
		Service:     service,
		Method:      method,
		Description: m.Description,
		HTTPMethod:  m.HTTPMethod,
		Path:        m.Path,
		RequestType: m.RequestType,
		IsWrite:     m.IsWrite,
		IsMultipart: m.IsMultipart,
	}, nil
}

package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/neosh11/MCPs/openapi-mcp/catalog"
	apiexec "github.com/neosh11/MCPs/openapi-mcp/exec"
)

type Config struct {
	BaseURL  string
	Headers  map[string]string
	Auth     apiexec.CredentialProvider
	ReadOnly bool
	Hook     apiexec.RequestHook
}

type Server struct {
	catalog  *catalog.Catalog
	executor *apiexec.Executor
}

type ToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func New(cat *catalog.Catalog, cfg Config) (*Server, error) {
	if cat == nil {
		return nil, errors.New("catalog is nil")
	}
	if err := cat.Validate(); err != nil {
		return nil, err
	}
	executor, err := apiexec.New(apiexec.Config{
		BaseURL:  cfg.BaseURL,
		Headers:  cfg.Headers,
		Auth:     cfg.Auth,
		ReadOnly: cfg.ReadOnly,
		Hook:     cfg.Hook,
	})
	if err != nil {
		return nil, err
	}
	return &Server{catalog: cat, executor: executor}, nil
}

func (s *Server) ListTools() []map[string]any {
	services := s.catalog.ServiceKeys()
	return []map[string]any{
		{
			"name":        "get_service_info",
			"description": "Get method names and short descriptions for one Square API service.",
			"inputSchema": objectSchema(map[string]any{
				"service": stringSchema("The service key, for example locations, catalog, or payments."),
			}, []string{"service"}),
		},
		{
			"name":        "get_type_info",
			"description": "Get request field information for one Square API service method. Call this before make_api_request.",
			"inputSchema": objectSchema(map[string]any{
				"service": stringSchema("The service key, for example locations, catalog, or payments."),
				"method":  stringSchema("The method key returned by get_service_info."),
			}, []string{"service", "method"}),
		},
		{
			"name": "make_api_request",
			"description": fmt.Sprintf(
				"Execute a Square API method. Available services: %s. Call get_service_info and get_type_info first.",
				join(services, ", "),
			),
			"inputSchema": objectSchema(map[string]any{
				"service": stringSchema("The service key, for example locations, catalog, or payments."),
				"method":  stringSchema("The method key returned by get_service_info."),
				"request": map[string]any{
					"type":                 "object",
					"description":          "Request fields for path, query, and JSON body parameters.",
					"additionalProperties": true,
				},
			}, []string{"service", "method"}),
		},
	}
}

func (s *Server) CallTool(ctx context.Context, name string, arguments map[string]any) ToolResult {
	text, err := s.callTool(ctx, name, arguments)
	if err != nil {
		payload, _ := json.MarshalIndent(map[string]string{"error": err.Error()}, "", "  ")
		return ToolResult{Content: []Content{{Type: "text", Text: string(payload)}}, IsError: true}
	}
	return ToolResult{Content: []Content{{Type: "text", Text: text}}}
}

func (s *Server) callTool(ctx context.Context, name string, args map[string]any) (string, error) {
	switch name {
	case "get_service_info":
		serviceKey, err := requireString(args, "service")
		if err != nil {
			return "", err
		}
		svc, ok := s.catalog.Services[serviceKey]
		if !ok {
			return "", fmt.Errorf("invalid service %q; available services: %s", serviceKey, join(s.catalog.ServiceKeys(), ", "))
		}
		out := map[string]map[string]string{}
		for _, methodKey := range svc.MethodKeys() {
			out[methodKey] = map[string]string{"description": svc.Methods[methodKey].Description}
		}
		return marshalText(out)
	case "get_type_info":
		serviceKey, methodKey, method, err := s.lookupMethod(args)
		if err != nil {
			return "", err
		}
		types, ok := s.catalog.Types[method.RequestType]
		if !ok {
			return "", fmt.Errorf("type information not found for %s.%s request type %q", serviceKey, methodKey, method.RequestType)
		}
		return marshalText(types)
	case "make_api_request":
		serviceKey, methodKey, method, err := s.lookupMethod(args)
		if err != nil {
			return "", err
		}
		request, _ := args["request"].(map[string]any)
		return s.executor.Execute(ctx, serviceKey, methodKey, method, request)
	default:
		return "", fmt.Errorf("unknown tool %q", name)
	}
}

func (s *Server) lookupMethod(args map[string]any) (string, string, catalog.Method, error) {
	serviceKey, err := requireString(args, "service")
	if err != nil {
		return "", "", catalog.Method{}, err
	}
	methodKey, err := requireString(args, "method")
	if err != nil {
		return "", "", catalog.Method{}, err
	}
	svc, ok := s.catalog.Services[serviceKey]
	if !ok {
		return "", "", catalog.Method{}, fmt.Errorf("invalid service %q; available services: %s", serviceKey, join(s.catalog.ServiceKeys(), ", "))
	}
	method, ok := svc.Methods[methodKey]
	if !ok {
		return "", "", catalog.Method{}, fmt.Errorf("invalid method %q for service %q; available methods: %s", methodKey, serviceKey, join(svc.MethodKeys(), ", "))
	}
	return serviceKey, methodKey, method, nil
}

func (s *Server) ServeStdio(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			_ = enc.Encode(rpcError(nil, -32700, "parse error"))
			continue
		}
		if req.ID == nil {
			continue
		}
		resp := s.handleRPC(ctx, req)
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (s *Server) handleRPC(ctx context.Context, req rpcRequest) any {
	switch req.Method {
	case "initialize":
		return rpcResponse(req.ID, map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]string{
				"name":    "openapi-mcp",
				"version": "0.1.0",
			},
		})
	case "ping":
		return rpcResponse(req.ID, map[string]any{})
	case "tools/list":
		return rpcResponse(req.ID, map[string]any{"tools": s.ListTools()})
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return rpcError(req.ID, -32602, "invalid tools/call params")
		}
		return rpcResponse(req.ID, s.CallTool(ctx, params.Name, params.Arguments))
	default:
		return rpcError(req.ID, -32601, "method not found")
	}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func rpcResponse(id any, result any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
}

func rpcError(id any, code int, message string) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	sort.Strings(required)
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func requireString(args map[string]any, key string) (string, error) {
	value, ok := args[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	s, ok := value.(string)
	if !ok || s == "" {
		return "", fmt.Errorf("%s must be a non-empty string", key)
	}
	return s, nil
}

func marshalText(v any) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func join(values []string, sep string) string {
	return stringsJoin(values, sep)
}

func stringsJoin(values []string, sep string) string {
	if len(values) == 0 {
		return ""
	}
	out := values[0]
	for _, value := range values[1:] {
		out += sep + value
	}
	return out
}

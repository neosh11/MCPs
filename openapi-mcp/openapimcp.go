package openapimcp

import (
	"context"
	"fmt"
	"os"

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

type Client struct {
	catalog  *catalog.Catalog
	executor *apiexec.Executor
}

type MethodSummary struct {
	Description string `json:"description"`
}

func LoadCatalogFile(path string) (*catalog.Catalog, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return catalog.Load(f)
}

func New(cat *catalog.Catalog, cfg Config) (*Client, error) {
	if cat == nil {
		return nil, fmt.Errorf("catalog is nil")
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
	return &Client{catalog: cat, executor: executor}, nil
}

func (c *Client) Services() []string {
	return c.catalog.ServiceKeys()
}

func (c *Client) GetServiceInfo(service string) (map[string]MethodSummary, error) {
	svc, ok := c.catalog.Services[service]
	if !ok {
		return nil, fmt.Errorf("invalid service %q; available services: %v", service, c.catalog.ServiceKeys())
	}
	info := map[string]MethodSummary{}
	for _, methodKey := range svc.MethodKeys() {
		info[methodKey] = MethodSummary{Description: svc.Methods[methodKey].Description}
	}
	return info, nil
}

func (c *Client) GetTypeInfo(service, method string) ([]catalog.Type, error) {
	m, err := c.lookupMethod(service, method)
	if err != nil {
		return nil, err
	}
	types, ok := c.catalog.Types[m.RequestType]
	if !ok {
		return nil, fmt.Errorf("type information not found for %s.%s request type %q", service, method, m.RequestType)
	}
	return types, nil
}

func (c *Client) MakeAPIRequest(ctx context.Context, service, method string, request map[string]any) (string, error) {
	m, err := c.lookupMethod(service, method)
	if err != nil {
		return "", err
	}
	return c.executor.Execute(ctx, service, method, m, request)
}

func (c *Client) lookupMethod(service, method string) (catalog.Method, error) {
	svc, ok := c.catalog.Services[service]
	if !ok {
		return catalog.Method{}, fmt.Errorf("invalid service %q; available services: %v", service, c.catalog.ServiceKeys())
	}
	m, ok := svc.Methods[method]
	if !ok {
		return catalog.Method{}, fmt.Errorf("invalid method %q for service %q; available methods: %v", method, service, svc.MethodKeys())
	}
	return m, nil
}

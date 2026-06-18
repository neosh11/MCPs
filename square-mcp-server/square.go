package squaremcpserver

import (
	"bytes"
	"context"
	_ "embed"

	openapimcp "github.com/neosh11/MCPs/openapi-mcp"
	"github.com/neosh11/MCPs/openapi-mcp/catalog"
	apiexec "github.com/neosh11/MCPs/openapi-mcp/exec"
)

const (
	APIVersion      = "2025-04-16"
	ProductionURL   = "https://connect.squareup.com"
	SandboxURL      = "https://connect.squareupsandbox.com"
	CatalogFileName = "catalogs/square.json"
)

type Toolset = openapimcp.Toolset
type Tool = openapimcp.Tool
type ToolDefinition = openapimcp.ToolDefinition

//go:embed catalogs/square.json
var squareCatalogJSON []byte

type Config struct {
	BaseURL  string
	Sandbox  bool
	Headers  map[string]string
	Auth     apiexec.CredentialProvider
	ReadOnly bool
	Hook     apiexec.RequestHook
}

func LoadCatalog() (*catalog.Catalog, error) {
	return catalog.Load(bytes.NewReader(squareCatalogJSON))
}

func NewToolset(cfg Config) (*openapimcp.Toolset, error) {
	cat, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	return openapimcp.NewToolset(cat, genericConfig(cfg))
}

func NewClient(cfg Config) (*openapimcp.Client, error) {
	cat, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	return openapimcp.New(cat, genericConfig(cfg))
}

func StaticToken(token string) apiexec.CredentialProvider {
	return func(context.Context) (string, error) {
		return token, nil
	}
}

func genericConfig(cfg Config) openapimcp.Config {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		if cfg.Sandbox {
			baseURL = SandboxURL
		} else {
			baseURL = ProductionURL
		}
	}
	headers := map[string]string{
		"Square-Version": APIVersion,
		"User-Agent":     "square-mcp-server/0.1.0",
	}
	for k, v := range cfg.Headers {
		headers[k] = v
	}
	return openapimcp.Config{
		BaseURL:  baseURL,
		Headers:  headers,
		Auth:     cfg.Auth,
		ReadOnly: cfg.ReadOnly,
		Hook:     cfg.Hook,
	}
}

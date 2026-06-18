# MCPs

A Go monorepo for Model Context Protocol (MCP) servers and tooling.

## openapi-mcp

Turn **any OpenAPI 3.x spec** into an MCP server that exposes a small, constant
set of generic tools (`get_service_info`, `get_type_info`, `make_api_request`)
instead of one tool per endpoint — so large APIs stay usable inside a model's
context. A build-time generator produces a data **catalog** from the spec; one
generic runtime serves the three tools and executes calls. No per-endpoint code.

Design: [SPEC.md](SPEC.md).

Prior art: [square/square-mcp-server](https://github.com/square/square-mcp-server).

## square-mcp-server

Square-specific package built on `openapi-mcp`. It embeds the pinned Square
catalog and exposes Square operations as plain Go tool abstractions. A proxy can
import this module, enumerate Square operations, inspect request types, and
execute calls without running an MCP server:

```go
tools, err := squaremcpserver.NewToolset(squaremcpserver.Config{
    Sandbox:  true,
    ReadOnly: true,
    Auth: func(ctx context.Context) (string, error) {
        return tokenForTenant(ctx), nil
    },
})

defs := tools.ListTools()
locationsList, err := tools.GetTool("locations", "list")
resp, err := locationsList.Call(ctx, map[string]any{})
```

The optional `square-mcp` command is only an adapter around the same package.

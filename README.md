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

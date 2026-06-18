package server

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/neosh11/MCPs/openapi-mcp/catalog"
)

func TestServeStdioToolsListAndCall(t *testing.T) {
	srv, err := New(testCatalog(), Config{BaseURL: "https://example.test", ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_service_info","arguments":{"service":"locations"}}}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	if err := srv.ServeStdio(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 responses, got %d: %s", len(lines), out.String())
	}
	var response map[string]any
	if err := json.Unmarshal([]byte(lines[2]), &response); err != nil {
		t.Fatal(err)
	}
	result := response["result"].(map[string]any)
	content := result["content"].([]any)[0].(map[string]any)
	if !strings.Contains(content["text"].(string), "List locations.") {
		t.Fatalf("unexpected tool response: %s", content["text"])
	}
}

func testCatalog() *catalog.Catalog {
	return &catalog.Catalog{
		API: catalog.APIInfo{Name: "Square"},
		Services: map[string]catalog.Service{
			"locations": {
				Name: "Locations",
				Methods: map[string]catalog.Method{
					"list": {
						Description: "List locations.",
						HTTPMethod:  "get",
						Path:        "/v2/locations",
						RequestType: "ListLocationsRequest",
					},
				},
			},
		},
		Types: map[string][]catalog.Type{
			"ListLocationsRequest": {{Name: "ListLocationsRequest", Properties: []catalog.Property{}}},
		},
	}
}

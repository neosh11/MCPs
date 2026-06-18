package squaremcpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewToolsetListsBundledSquareOperations(t *testing.T) {
	tools, err := NewToolset(Config{
		BaseURL:  "https://example.test",
		ReadOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defs := tools.ListTools()
	if len(defs) != 283 {
		t.Fatalf("expected 283 Square tools, got %d", len(defs))
	}
	locationsList, err := tools.GetTool("locations", "list")
	if err != nil {
		t.Fatal(err)
	}
	if locationsList.Name != "locations.list" || locationsList.RequestType != "ListLocationsRequest" {
		t.Fatalf("unexpected locations.list definition: %+v", locationsList.ToolDefinition)
	}
	if len(locationsList.Types) != 1 || locationsList.Types[0].Name != "ListLocationsRequest" {
		t.Fatalf("unexpected type info: %+v", locationsList.Types)
	}
}

func TestToolsetCallToolUsesConfiguredHTTPExecutor(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/locations" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Square-Version") != APIVersion {
			t.Fatalf("missing square version header")
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("missing auth header")
		}
		_, _ = w.Write([]byte(`{"locations":[]}`))
	}))
	defer ts.Close()

	tools, err := NewToolset(Config{
		BaseURL:  ts.URL,
		Auth:     StaticToken("test-token"),
		ReadOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := tools.CallTool(context.Background(), "locations", "list", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != `{"locations":[]}` {
		t.Fatalf("unexpected result: %s", result)
	}
}

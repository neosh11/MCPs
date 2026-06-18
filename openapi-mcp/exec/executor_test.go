package exec

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/neosh11/MCPs/openapi-mcp/catalog"
)

func TestExecuteBuildsPathQueryBodyAndHeaders(t *testing.T) {
	var got struct {
		Method  string         `json:"method"`
		Path    string         `json:"path"`
		Query   string         `json:"query"`
		Token   string         `json:"token"`
		Version string         `json:"version"`
		Body    map[string]any `json:"body"`
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Method = r.Method
		got.Path = r.URL.EscapedPath()
		got.Query = r.URL.RawQuery
		got.Token = r.Header.Get("Authorization")
		got.Version = r.Header.Get("Square-Version")
		_ = json.NewDecoder(r.Body).Decode(&got.Body)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	executor, err := New(Config{
		BaseURL: ts.URL,
		Headers: map[string]string{
			"Square-Version": "2025-04-16",
		},
		Auth: func(context.Context) (string, error) { return "secret", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := executor.Execute(context.Background(), "locations", "update", catalog.Method{
		HTTPMethod: "put",
		Path:       "/v2/locations/{location_id}",
		PathParams: []catalog.Parameter{{
			Name:     "location_id",
			Type:     "string",
			Required: true,
		}},
		QueryParams: []catalog.Parameter{{Name: "include", Type: "string"}},
		IsWrite:     true,
	}, map[string]any{
		"location_id": "LOC 1",
		"include":     "details",
		"name":        "Main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != `{"ok":true}` {
		t.Fatalf("unexpected result: %s", result)
	}
	if got.Method != "PUT" || got.Path != "/v2/locations/LOC%201" || got.Query != "include=details" {
		t.Fatalf("unexpected request route: %+v", got)
	}
	if got.Token != "Bearer secret" || got.Version != "2025-04-16" {
		t.Fatalf("unexpected headers: %+v", got)
	}
	if got.Body["name"] != "Main" {
		t.Fatalf("unexpected body: %+v", got.Body)
	}
}

func TestExecuteReadOnlyRejectsWrites(t *testing.T) {
	executor, err := New(Config{BaseURL: "https://example.test", ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = executor.Execute(context.Background(), "locations", "create", catalog.Method{
		HTTPMethod: "post",
		Path:       "/v2/locations",
		IsWrite:    true,
	}, nil)
	if err == nil {
		t.Fatal("expected read-only error")
	}
}

func TestExecuteReturnsAPIErrorBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":[{"code":"BAD_REQUEST"}]}`))
	}))
	defer ts.Close()
	executor, err := New(Config{BaseURL: ts.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = executor.Execute(context.Background(), "locations", "list", catalog.Method{
		HTTPMethod: "get",
		Path:       "/v2/locations",
	}, nil)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T %v", err, err)
	}
	if apiErr.StatusCode != http.StatusBadRequest || apiErr.Body == "" {
		t.Fatalf("unexpected api error: %+v", apiErr)
	}
}

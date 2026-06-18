package openapimcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/neosh11/MCPs/openapi-mcp/catalog"
)

func TestClientDiscoveryAndRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"locations":[]}`))
	}))
	defer ts.Close()
	client, err := New(testCatalog(), Config{BaseURL: ts.URL, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	serviceInfo, err := client.GetServiceInfo("locations")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := serviceInfo["list"]; !ok {
		t.Fatalf("missing list method: %+v", serviceInfo)
	}
	typeInfo, err := client.GetTypeInfo("locations", "list")
	if err != nil {
		t.Fatal(err)
	}
	if len(typeInfo) != 1 || typeInfo[0].Name != "ListLocationsRequest" {
		t.Fatalf("unexpected type info: %+v", typeInfo)
	}
	result, err := client.MakeAPIRequest(context.Background(), "locations", "list", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != `{"locations":[]}` {
		t.Fatalf("unexpected result: %s", result)
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

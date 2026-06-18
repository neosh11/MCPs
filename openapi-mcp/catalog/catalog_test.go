package catalog

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoadWriteAndValidate(t *testing.T) {
	cat := &Catalog{
		API: APIInfo{Name: "Square"},
		Services: map[string]Service{
			"locations": {
				Name: "Locations",
				Methods: map[string]Method{
					"list": {
						Description: "List locations.",
						HTTPMethod:  "get",
						Path:        "/v2/locations",
						PathParams:  []Parameter{},
						QueryParams: []Parameter{},
						RequestType: "ListLocationsRequest",
					},
				},
			},
		},
		Types: map[string][]Type{
			"ListLocationsRequest": {{Name: "ListLocationsRequest", Properties: []Property{}}},
		},
	}
	var buf bytes.Buffer
	if err := Write(&buf, cat); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"services"`) {
		t.Fatalf("expected JSON output, got %s", buf.String())
	}
	loaded, err := Load(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.API.Name != "Square" || len(loaded.Services) != 1 {
		t.Fatalf("unexpected loaded catalog: %+v", loaded)
	}
}

func TestValidateRejectsMissingRequestType(t *testing.T) {
	cat := &Catalog{
		API: APIInfo{Name: "Square"},
		Services: map[string]Service{
			"locations": {
				Name: "Locations",
				Methods: map[string]Method{
					"list": {
						Description: "List locations.",
						HTTPMethod:  "get",
						Path:        "/v2/locations",
						RequestType: "MissingType",
					},
				},
			},
		},
		Types: map[string][]Type{},
	}
	if err := cat.Validate(); err == nil {
		t.Fatal("expected missing request type error")
	}
}

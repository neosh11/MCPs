package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
)

type Catalog struct {
	API      APIInfo            `json:"api"`
	Services map[string]Service `json:"services"`
	Types    map[string][]Type  `json:"types"`
}

type APIInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	SpecVersion string `json:"specVersion,omitempty"`
	SpecSource  string `json:"specSource,omitempty"`
}

type Service struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Methods     map[string]Method `json:"methods"`
}

type Method struct {
	Description  string      `json:"description"`
	HTTPMethod   string      `json:"httpMethod"`
	Path         string      `json:"path"`
	PathParams   []Parameter `json:"pathParams"`
	QueryParams  []Parameter `json:"queryParams"`
	RequestType  string      `json:"requestType"`
	IsWrite      bool        `json:"isWrite"`
	IsMultipart  bool        `json:"isMultipart"`
	OriginalName string      `json:"originalName,omitempty"`
}

type Parameter struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type Type struct {
	Name       string     `json:"name"`
	Properties []Property `json:"properties"`
}

type Property struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Description string  `json:"description,omitempty"`
	Required    bool    `json:"required"`
	ReadOnly    bool    `json:"readOnly"`
	ArrayType   *string `json:"arrayType,omitempty"`
	IsFile      bool    `json:"isFile,omitempty"`
}

func Load(r io.Reader) (*Catalog, error) {
	var c Catalog
	dec := json.NewDecoder(r)
	if err := dec.Decode(&c); err != nil {
		return nil, err
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func Write(w io.Writer, c *Catalog) error {
	if c == nil {
		return errors.New("catalog is nil")
	}
	if err := c.Validate(); err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(c)
}

func (c *Catalog) Validate() error {
	if c == nil {
		return errors.New("catalog is nil")
	}
	if c.API.Name == "" {
		return errors.New("catalog api.name is required")
	}
	if len(c.Services) == 0 {
		return errors.New("catalog services are required")
	}
	if c.Types == nil {
		return errors.New("catalog types are required")
	}
	for serviceKey, svc := range c.Services {
		if serviceKey == "" {
			return errors.New("service key is empty")
		}
		if svc.Name == "" {
			return fmt.Errorf("service %q name is required", serviceKey)
		}
		if len(svc.Methods) == 0 {
			return fmt.Errorf("service %q has no methods", serviceKey)
		}
		for methodKey, method := range svc.Methods {
			if methodKey == "" {
				return fmt.Errorf("service %q has empty method key", serviceKey)
			}
			if method.HTTPMethod == "" {
				return fmt.Errorf("%s.%s httpMethod is required", serviceKey, methodKey)
			}
			if method.Path == "" {
				return fmt.Errorf("%s.%s path is required", serviceKey, methodKey)
			}
			if method.RequestType == "" {
				return fmt.Errorf("%s.%s requestType is required", serviceKey, methodKey)
			}
			if _, ok := c.Types[method.RequestType]; !ok {
				return fmt.Errorf("%s.%s requestType %q is missing from types", serviceKey, methodKey, method.RequestType)
			}
		}
	}
	return nil
}

func (c *Catalog) ServiceKeys() []string {
	keys := make([]string, 0, len(c.Services))
	for k := range c.Services {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (s Service) MethodKeys() []string {
	keys := make([]string, 0, len(s.Methods))
	for k := range s.Methods {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

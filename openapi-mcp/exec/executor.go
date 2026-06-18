package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/neosh11/MCPs/openapi-mcp/catalog"
)

type CredentialProvider func(context.Context) (string, error)

type RequestHook func(context.Context, PlannedRequest) error

type Config struct {
	BaseURL    string
	Headers    map[string]string
	Auth       CredentialProvider
	ReadOnly   bool
	Hook       RequestHook
	HTTPClient *http.Client
}

type Executor struct {
	cfg Config
}

type PlannedRequest struct {
	Service    string
	Method     string
	HTTPMethod string
	URL        string
	Body       map[string]any
	IsWrite    bool
}

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("api returned status %d", e.StatusCode)
	}
	return e.Body
}

func New(cfg Config) (*Executor, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("baseURL is required")
	}
	if _, err := url.ParseRequestURI(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("invalid baseURL: %w", err)
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	return &Executor{cfg: cfg}, nil
}

func (e *Executor) Execute(ctx context.Context, serviceKey, methodKey string, method catalog.Method, request map[string]any) (string, error) {
	if e.cfg.ReadOnly && method.IsWrite {
		return "", fmt.Errorf("write operations are disabled: %s.%s", serviceKey, methodKey)
	}
	if request == nil {
		request = map[string]any{}
	}

	remaining := cloneMap(request)
	path, err := fillPath(method.Path, method.PathParams, remaining)
	if err != nil {
		return "", err
	}

	u, err := url.Parse(strings.TrimRight(e.cfg.BaseURL, "/") + path)
	if err != nil {
		return "", err
	}
	q := u.Query()
	for _, param := range method.QueryParams {
		value, ok := remaining[param.Name]
		if ok {
			q.Set(param.Name, fmt.Sprint(value))
			delete(remaining, param.Name)
		} else if param.Required {
			return "", fmt.Errorf("missing required query parameter: %s", param.Name)
		}
	}
	u.RawQuery = q.Encode()

	bodyFields := map[string]any{}
	if isBodyMethod(method.HTTPMethod) {
		bodyFields = remaining
	}

	planned := PlannedRequest{
		Service:    serviceKey,
		Method:     methodKey,
		HTTPMethod: strings.ToUpper(method.HTTPMethod),
		URL:        u.String(),
		Body:       bodyFields,
		IsWrite:    method.IsWrite,
	}
	if e.cfg.Hook != nil {
		if err := e.cfg.Hook(ctx, planned); err != nil {
			return "", err
		}
	}

	var body io.Reader
	if isBodyMethod(method.HTTPMethod) && len(bodyFields) > 0 {
		payload, err := json.Marshal(bodyFields)
		if err != nil {
			return "", err
		}
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, planned.HTTPMethod, planned.URL, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range e.cfg.Headers {
		req.Header.Set(k, v)
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if e.cfg.Auth != nil {
		token, err := e.cfg.Auth(ctx)
		if err != nil {
			return "", err
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	resp, err := e.cfg.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	if len(respBody) == 0 {
		return `{"success":true}`, nil
	}
	return string(respBody), nil
}

func fillPath(path string, params []catalog.Parameter, remaining map[string]any) (string, error) {
	for _, param := range params {
		value, ok := remaining[param.Name]
		if !ok {
			if param.Required {
				return "", fmt.Errorf("missing required path parameter: %s", param.Name)
			}
			continue
		}
		path = strings.ReplaceAll(path, "{"+param.Name+"}", url.PathEscape(fmt.Sprint(value)))
		delete(remaining, param.Name)
	}
	if strings.Contains(path, "{") || strings.Contains(path, "}") {
		return "", fmt.Errorf("unresolved path template: %s", path)
	}
	return path, nil
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func isBodyMethod(method string) bool {
	switch strings.ToLower(method) {
	case "post", "put", "patch":
		return true
	default:
		return false
	}
}

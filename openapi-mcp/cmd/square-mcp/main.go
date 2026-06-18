package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neosh11/MCPs/openapi-mcp/catalog"
	"github.com/neosh11/MCPs/openapi-mcp/server"
)

const (
	defaultCatalogPath = "catalogs/square.json"
	productionBaseURL  = "https://connect.squareup.com"
	sandboxBaseURL     = "https://connect.squareupsandbox.com"
	defaultVersion     = "2025-04-16"
)

func main() {
	catalogPath := flag.String("catalog", defaultCatalogPath, "Square catalog JSON path")
	baseURL := flag.String("base-url", "", "Square API base URL; defaults from SANDBOX/PRODUCTION")
	flag.Parse()

	if err := loadFirstDotEnv(".env", "../.env"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cat, err := loadCatalog(*catalogPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cfg := server.Config{
		BaseURL: chooseBaseURL(*baseURL),
		Headers: map[string]string{
			"Square-Version": envDefault("SQUARE_VERSION", defaultVersion),
			"User-Agent":     "openapi-mcp-square/0.1.0",
		},
		Auth: func(context.Context) (string, error) {
			return firstEnv("ACCESS_TOKEN", "TOKEN"), nil
		},
		ReadOnly: strings.ToLower(os.Getenv("ALLOW_WRITES")) != "true",
	}
	srv, err := server.New(cat, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := srv.ServeStdio(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loadCatalog(path string) (*catalog.Catalog, error) {
	f, err := os.Open(path)
	if err != nil && !filepath.IsAbs(path) {
		f, err = os.Open(filepath.Join("openapi-mcp", path))
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return catalog.Load(f)
}

func chooseBaseURL(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if strings.ToLower(os.Getenv("SANDBOX")) == "true" {
		return sandboxBaseURL
	}
	return productionBaseURL
}

func envDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func loadDotEnv(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
	return nil
}

func loadFirstDotEnv(paths ...string) error {
	for _, path := range paths {
		err := loadDotEnv(path)
		if err == nil {
			return nil
		}
		if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

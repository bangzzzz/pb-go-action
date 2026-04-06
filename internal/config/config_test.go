package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Valid(t *testing.T) {
	content := `
services:
  - name: user-service
    proto_dir: proto/user
    include_dirs:
      - proto/common
    target_repo: org/user-pb-go
    go_module: github.com/org/user-pb-go

plugins:
  - name: go
    out: .
    opt:
      - paths=source_relative
  - name: go-grpc
    out: .
    opt:
      - paths=source_relative
      - require_unimplemented_servers=false
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(cfg.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(cfg.Services))
	}
	svc := cfg.Services[0]
	if svc.Name != "user-service" {
		t.Errorf("unexpected name: %s", svc.Name)
	}
	if svc.ProtoDir != "proto/user" {
		t.Errorf("unexpected proto_dir: %s", svc.ProtoDir)
	}
	if len(svc.IncludeDirs) != 1 || svc.IncludeDirs[0] != "proto/common" {
		t.Errorf("unexpected include_dirs: %v", svc.IncludeDirs)
	}

	if len(cfg.Plugins) != 2 {
		t.Fatalf("expected 2 global plugins, got %d", len(cfg.Plugins))
	}
	if cfg.Plugins[0].Name != "go" {
		t.Errorf("unexpected plugin name: %s", cfg.Plugins[0].Name)
	}
	if cfg.Plugins[1].Name != "go-grpc" {
		t.Errorf("unexpected plugin name: %s", cfg.Plugins[1].Name)
	}

	// Global plugins should be returned for this service (no service-level plugins).
	resolved := cfg.PluginsFor(&cfg.Services[0])
	if len(resolved) != 2 {
		t.Errorf("expected 2 resolved plugins, got %d", len(resolved))
	}
}

func TestLoad_ServicePluginsOverride(t *testing.T) {
	content := `
services:
  - name: user-service
    proto_dir: proto/user
    target_repo: org/user-pb-go
    plugins:
      - name: go
        out: .
        opt:
          - paths=source_relative
  - name: order-service
    proto_dir: proto/order
    target_repo: org/order-pb-go
    plugins:
      - name: go
        out: .
      - name: go-grpc
        out: .
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	userPlugins := cfg.PluginsFor(&cfg.Services[0])
	if len(userPlugins) != 1 || userPlugins[0].Name != "go" {
		t.Errorf("user-service: expected [go], got %v", pluginNames(userPlugins))
	}

	orderPlugins := cfg.PluginsFor(&cfg.Services[1])
	if len(orderPlugins) != 2 {
		t.Errorf("order-service: expected 2 plugins, got %v", pluginNames(orderPlugins))
	}
}

func TestLoad_MixedPlugins(t *testing.T) {
	// user-service has its own plugins; order-service falls back to global.
	content := `
services:
  - name: user-service
    proto_dir: proto/user
    target_repo: org/user-pb-go
    plugins:
      - name: go-grpc
        out: .
  - name: order-service
    proto_dir: proto/order
    target_repo: org/order-pb-go

plugins:
  - name: go
    out: .
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	userPlugins := cfg.PluginsFor(&cfg.Services[0])
	if len(userPlugins) != 1 || userPlugins[0].Name != "go-grpc" {
		t.Errorf("user-service: expected [go-grpc], got %v", pluginNames(userPlugins))
	}

	orderPlugins := cfg.PluginsFor(&cfg.Services[1])
	if len(orderPlugins) != 1 || orderPlugins[0].Name != "go" {
		t.Errorf("order-service: expected [go], got %v", pluginNames(orderPlugins))
	}
}

func TestLoad_DefaultOut_GlobalPlugin(t *testing.T) {
	content := `
services:
  - name: svc
    proto_dir: proto
    target_repo: org/svc-pb-go
plugins:
  - name: go
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Plugins[0].Out != "." {
		t.Errorf("expected default out '.', got %q", cfg.Plugins[0].Out)
	}
}

func TestLoad_DefaultOut_ServicePlugin(t *testing.T) {
	content := `
services:
  - name: svc
    proto_dir: proto
    target_repo: org/svc-pb-go
    plugins:
      - name: go
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Services[0].Plugins[0].Out != "." {
		t.Errorf("expected default out '.', got %q", cfg.Services[0].Plugins[0].Out)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/.pb-go-action.yml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_NoServices(t *testing.T) {
	content := `
plugins:
  - name: go
`
	path := writeTemp(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing services")
	}
}

func TestLoad_NoPlugins_ServiceHasOwn(t *testing.T) {
	// No global plugins, but each service defines its own — should be valid.
	content := `
services:
  - name: svc
    proto_dir: proto
    target_repo: org/svc-pb-go
    plugins:
      - name: go
`
	path := writeTemp(t, content)
	_, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error when all services define their own plugins, got: %v", err)
	}
}

func TestLoad_NoPlugins_ServiceMissingPlugins(t *testing.T) {
	// No global plugins and at least one service has none — should fail.
	content := `
services:
  - name: svc
    proto_dir: proto
    target_repo: org/svc-pb-go
`
	path := writeTemp(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error when service has no plugins and no global plugins defined")
	}
}

func TestLoad_MissingRequiredFields(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{
			name: "missing service name",
			content: `
services:
  - proto_dir: proto
    target_repo: org/svc-pb-go
plugins:
  - name: go
`,
		},
		{
			name: "missing proto_dir",
			content: `
services:
  - name: svc
    target_repo: org/svc-pb-go
plugins:
  - name: go
`,
		},
		{
			name: "missing target_repo",
			content: `
services:
  - name: svc
    proto_dir: proto
plugins:
  - name: go
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTemp(t, tc.content)
			_, err := Load(path)
			if err == nil {
				t.Fatalf("expected validation error")
			}
		})
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "cfg-*.yml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	return filepath.Clean(f.Name())
}

func pluginNames(plugins []Plugin) []string {
	names := make([]string, len(plugins))
	for i, p := range plugins {
		names[i] = p.Name
	}
	return names
}

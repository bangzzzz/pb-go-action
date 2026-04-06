package detector

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/bangzzzz/pb-go-action/internal/config"
)

// ── ParseImports ──────────────────────────────────────────────────────────────

func TestParseImports(t *testing.T) {
	dir := t.TempDir()
	protoFile := filepath.Join(dir, "a.proto")
	content := `
syntax = "proto3";

import "google/protobuf/timestamp.proto";
import "common/base.proto";
import weak "other/file.proto";
import public "third/thing.proto";
`
	if err := os.WriteFile(protoFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	imports, err := ParseImports(protoFile)
	if err != nil {
		t.Fatalf("ParseImports: %v", err)
	}

	want := []string{
		"google/protobuf/timestamp.proto",
		"common/base.proto",
		"other/file.proto",
		"third/thing.proto",
	}
	for _, w := range want {
		if !slices.Contains(imports, w) {
			t.Errorf("expected import %q not found in %v", w, imports)
		}
	}
}

// ── FindProtoFiles ────────────────────────────────────────────────────────────

func TestFindProtoFiles(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		filepath.Join(dir, "a.proto"),
		filepath.Join(dir, "sub", "b.proto"),
		filepath.Join(dir, "sub", "c.go"), // should be ignored
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	protos, err := FindProtoFiles(dir)
	if err != nil {
		t.Fatalf("FindProtoFiles: %v", err)
	}
	if len(protos) != 2 {
		t.Errorf("expected 2 proto files, got %d: %v", len(protos), protos)
	}
}

// ── FindAffectedServices ──────────────────────────────────────────────────────

func TestFindAffectedServices_DirectChange(t *testing.T) {
	workspace := buildWorkspace(t, map[string]string{
		"proto/user/user.proto": `syntax = "proto3";`,
	})

	cfg := &config.Config{
		Services: []config.Service{
			{Name: "user-svc", ProtoDir: "proto/user", TargetRepo: "org/user-pb"},
		},
		Plugins: []config.Plugin{{Name: "go"}},
	}

	affected, err := FindAffectedServices(workspace, cfg, []string{"proto/user/user.proto"})
	if err != nil {
		t.Fatalf("FindAffectedServices: %v", err)
	}
	if len(affected) != 1 || affected[0].Name != "user-svc" {
		t.Errorf("expected [user-svc], got %v", serviceNames(affected))
	}
}

func TestFindAffectedServices_TransitiveImport(t *testing.T) {
	// common.proto is imported by user.proto.
	// When common.proto changes, user-svc should be affected.
	workspace := buildWorkspace(t, map[string]string{
		"proto/common/common.proto": `syntax = "proto3";`,
		"proto/user/user.proto": `syntax = "proto3";
import "proto/common/common.proto";`,
	})

	cfg := &config.Config{
		Services: []config.Service{
			{
				Name:        "user-svc",
				ProtoDir:    "proto/user",
				IncludeDirs: []string{"proto/common"},
				TargetRepo:  "org/user-pb",
			},
		},
		Plugins: []config.Plugin{{Name: "go"}},
	}

	affected, err := FindAffectedServices(workspace, cfg, []string{"proto/common/common.proto"})
	if err != nil {
		t.Fatalf("FindAffectedServices: %v", err)
	}
	if len(affected) != 1 || affected[0].Name != "user-svc" {
		t.Errorf("expected [user-svc], got %v", serviceNames(affected))
	}
}

func TestFindAffectedServices_NoChange(t *testing.T) {
	workspace := buildWorkspace(t, map[string]string{
		"proto/user/user.proto": `syntax = "proto3";`,
	})

	cfg := &config.Config{
		Services: []config.Service{
			{Name: "user-svc", ProtoDir: "proto/user", TargetRepo: "org/user-pb"},
		},
		Plugins: []config.Plugin{{Name: "go"}},
	}

	affected, err := FindAffectedServices(workspace, cfg, []string{"proto/order/order.proto"})
	if err != nil {
		t.Fatalf("FindAffectedServices: %v", err)
	}
	if len(affected) != 0 {
		t.Errorf("expected no affected services, got %v", serviceNames(affected))
	}
}

func TestFindAffectedServices_MultipleServices(t *testing.T) {
	workspace := buildWorkspace(t, map[string]string{
		"proto/user/user.proto":   `syntax = "proto3";`,
		"proto/order/order.proto": `syntax = "proto3";`,
	})

	cfg := &config.Config{
		Services: []config.Service{
			{Name: "user-svc", ProtoDir: "proto/user", TargetRepo: "org/user-pb"},
			{Name: "order-svc", ProtoDir: "proto/order", TargetRepo: "org/order-pb"},
		},
		Plugins: []config.Plugin{{Name: "go"}},
	}

	// Only order.proto changed
	affected, err := FindAffectedServices(workspace, cfg, []string{"proto/order/order.proto"})
	if err != nil {
		t.Fatalf("FindAffectedServices: %v", err)
	}
	if len(affected) != 1 || affected[0].Name != "order-svc" {
		t.Errorf("expected [order-svc], got %v", serviceNames(affected))
	}
}

func TestFindAffectedServices_EmptyChangedFiles(t *testing.T) {
	cfg := &config.Config{
		Services: []config.Service{{Name: "svc", ProtoDir: "proto", TargetRepo: "org/pb"}},
		Plugins:  []config.Plugin{{Name: "go"}},
	}
	affected, err := FindAffectedServices(t.TempDir(), cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(affected) != 0 {
		t.Errorf("expected no affected services")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func buildWorkspace(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func serviceNames(svcs []*config.Service) []string {
	names := make([]string, len(svcs))
	for i, s := range svcs {
		names[i] = s.Name
	}
	return names
}

package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bangzzzz/pb-go-action/internal/config"
)

func TestDedup(t *testing.T) {
	input := []string{"a", "b", "a", "c", "b"}
	got := dedup(input)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("dedup(%v) = %v, want %v", input, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("dedup[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGenerate_EmptyProtoDir(t *testing.T) {
	workspace := t.TempDir()
	protoDir := filepath.Join(workspace, "proto", "empty")
	if err := os.MkdirAll(protoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Plugins: []config.Plugin{{Name: "go", Out: "."}},
	}
	svc := &config.Service{
		Name:     "empty-svc",
		ProtoDir: "proto/empty",
	}

	outDir, err := Generate(workspace, cfg, svc)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	defer os.RemoveAll(outDir)

	if outDir == "" {
		t.Error("expected a non-empty outDir")
	}
	if _, err := os.Stat(outDir); err != nil {
		t.Errorf("outDir %q does not exist: %v", outDir, err)
	}
}

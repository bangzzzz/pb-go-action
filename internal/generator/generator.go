// Package generator runs protoc to generate Go code from proto files.
package generator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bangzzzz/pb-go-action/internal/config"
	"github.com/bangzzzz/pb-go-action/internal/detector"
)

// Generate compiles all .proto files for the given service using the configured
// plugins and returns the path to the directory containing the generated files.
// The caller is responsible for removing the returned directory when done.
func Generate(workspace string, cfg *config.Config, svc *config.Service) (string, error) {
	// Resolve absolute path to the service proto directory.
	protoDir := filepath.Join(workspace, svc.ProtoDir)

	protoFiles, err := detector.FindProtoFiles(protoDir)
	if err != nil {
		return "", fmt.Errorf("finding proto files in %q: %w", protoDir, err)
	}
	if len(protoFiles) == 0 {
		// Nothing to compile; return an empty temp dir.
		return os.MkdirTemp("", "pb-go-action-*")
	}

	// Create a temporary output root.
	outRoot, err := os.MkdirTemp("", "pb-go-action-*")
	if err != nil {
		return "", fmt.Errorf("creating output dir: %w", err)
	}

	// Build the protoc include (-I) path list.
	// We always include the workspace root and the service proto dir so that
	// both absolute and relative imports resolve correctly.
	includeDirs := []string{workspace, protoDir}
	for _, d := range svc.IncludeDirs {
		includeDirs = append(includeDirs, filepath.Join(workspace, d))
	}

	// Remove duplicate include paths while preserving order.
	includeDirs = dedup(includeDirs)

	// Assemble protoc arguments.
	plugins := cfg.PluginsFor(svc)
	args := make([]string, 0, len(includeDirs)+len(plugins)*2+len(protoFiles))
	for _, d := range includeDirs {
		args = append(args, "-I"+d)
	}

	// Add one --<plugin>_out (and optional --<plugin>_opt) flag per plugin.
	for _, plugin := range plugins {
		pluginOut := filepath.Join(outRoot, plugin.Out)
		if err := os.MkdirAll(pluginOut, 0o755); err != nil {
			return "", fmt.Errorf("creating output dir for plugin %q: %w", plugin.Name, err)
		}
		args = append(args, fmt.Sprintf("--%s_out=%s", plugin.Name, pluginOut))
		if len(plugin.Opt) > 0 {
			args = append(args, fmt.Sprintf("--%s_opt=%s", plugin.Name, strings.Join(plugin.Opt, ",")))
		}
	}

	// Append proto source files.
	args = append(args, protoFiles...)

	cmd := exec.Command("protoc", args...)
	cmd.Dir = workspace
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Clean up on failure.
		_ = os.RemoveAll(outRoot)
		return "", fmt.Errorf("protoc failed for service %q: %w", svc.Name, err)
	}

	return outRoot, nil
}

// dedup removes duplicate strings from s while preserving order.
func dedup(s []string) []string {
	seen := make(map[string]bool, len(s))
	out := make([]string, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

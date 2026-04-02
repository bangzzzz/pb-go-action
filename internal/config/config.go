// Package config handles loading and validating the pb-go-action YAML configuration.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level pb-go-action configuration.
type Config struct {
	Services []Service `yaml:"services"`
	Plugins  []Plugin  `yaml:"plugins"`
}

// Service describes a single micro-service whose protobuf IDL files should be
// compiled and pushed to a target GitHub repository.
type Service struct {
	// Name is a human-readable label used in logs and error messages.
	Name string `yaml:"name"`

	// ProtoDir is the directory (relative to the workspace root) that contains
	// all .proto files belonging to this service.
	ProtoDir string `yaml:"proto_dir"`

	// IncludeDirs are extra -I include paths passed to protoc (relative to the
	// workspace root). Useful for shared/common proto files.
	IncludeDirs []string `yaml:"include_dirs"`

	// TargetRepo is the GitHub repository slug (owner/repo) where the generated
	// Go code will be pushed.
	TargetRepo string `yaml:"target_repo"`

	// GoModule is the Go module name written into the generated go.mod.
	// If empty no go.mod is created / updated.
	GoModule string `yaml:"go_module"`
}

// Plugin describes a single protoc generator plugin (e.g. go, go-grpc).
type Plugin struct {
	// Name is the plugin identifier, e.g. "go" → protoc-gen-go,
	// "go-grpc" → protoc-gen-go-grpc.
	Name string `yaml:"name"`

	// Out is the output directory passed to --<name>_out (relative to the
	// per-service generated output root). Defaults to ".".
	Out string `yaml:"out"`

	// Opt holds the option strings passed to --<name>_opt, e.g.
	// ["paths=source_relative", "require_unimplemented_servers=false"].
	Opt []string `yaml:"opt"`
}

// Load reads and validates the configuration from the given YAML file path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func validate(cfg *Config) error {
	if len(cfg.Services) == 0 {
		return fmt.Errorf("at least one service must be defined under 'services'")
	}

	for i := range cfg.Services {
		svc := &cfg.Services[i]
		if svc.Name == "" {
			return fmt.Errorf("services[%d]: 'name' is required", i)
		}
		if svc.ProtoDir == "" {
			return fmt.Errorf("service %q: 'proto_dir' is required", svc.Name)
		}
		if svc.TargetRepo == "" {
			return fmt.Errorf("service %q: 'target_repo' is required", svc.Name)
		}
	}

	if len(cfg.Plugins) == 0 {
		return fmt.Errorf("at least one plugin must be defined under 'plugins'")
	}

	for i := range cfg.Plugins {
		p := &cfg.Plugins[i]
		if p.Name == "" {
			return fmt.Errorf("plugins[%d]: 'name' is required", i)
		}
		if p.Out == "" {
			p.Out = "."
		}
	}

	return nil
}

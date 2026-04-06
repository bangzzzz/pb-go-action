package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bangzzzz/pb-go-action/internal/config"
	"github.com/bangzzzz/pb-go-action/internal/detector"
	"github.com/bangzzzz/pb-go-action/internal/generator"
	"github.com/bangzzzz/pb-go-action/internal/pusher"
)

func main() {
	workspace := getEnv("GITHUB_WORKSPACE", ".")
	configPath := getInput("config", ".pb-go-action.yml")
	token := getInput("github_token", "")
	changedFilesInput := getInput("changed_files", "")
	gitUserName := getInput("git_user_name", "PB Go Action")
	gitUserEmail := getInput("git_user_email", "pb-go-action[bot]@users.noreply.github.com")

	branch := currentBranch()

	log.Printf("workspace: %s", workspace)
	log.Printf("branch: %s", branch)

	// Load configuration
	cfgFile := filepath.Join(workspace, configPath)
	cfg, err := config.Load(cfgFile)
	if err != nil {
		log.Fatalf("failed to load config %s: %v", cfgFile, err)
	}

	// Determine changed proto files
	var changedFiles []string
	if changedFilesInput != "" {
		changedFiles = parseLines(changedFilesInput)
	} else {
		changedFiles, err = detector.GetChangedFiles(workspace)
		if err != nil {
			log.Fatalf("failed to detect changed files: %v", err)
		}
	}

	log.Printf("changed proto files: %v", changedFiles)

	if len(changedFiles) == 0 {
		fmt.Println("No proto files changed — nothing to do.")
		return
	}

	// Find affected services
	affected, err := detector.FindAffectedServices(workspace, cfg, changedFiles)
	if err != nil {
		log.Fatalf("failed to find affected services: %v", err)
	}

	if len(affected) == 0 {
		fmt.Println("No services are affected by the changed proto files — nothing to do.")
		return
	}

	log.Printf("affected services: %v", serviceNames(affected))

	// Generate and push for each affected service
	var errs []string
	for _, svc := range affected {
		log.Printf("generating code for service: %s", svc.Name)

		outDir, err := generator.Generate(workspace, cfg, svc)
		if err != nil {
			errs = append(errs, fmt.Sprintf("service %s: generate: %v", svc.Name, err))
			continue
		}

		log.Printf("pushing generated code for service: %s -> %s (branch: %s)", svc.Name, svc.TargetRepo, branch)

		pushCfg := pusher.Config{
			Token:         token,
			Branch:        branch,
			GitUserName:   gitUserName,
			GitUserEmail:  gitUserEmail,
		}
		if err := pusher.Push(svc, outDir, pushCfg); err != nil {
			errs = append(errs, fmt.Sprintf("service %s: push: %v", svc.Name, err))
			continue
		}

		log.Printf("done: %s -> %s", svc.Name, svc.TargetRepo)
	}

	if len(errs) > 0 {
		log.Fatalf("errors occurred:\n  %s", strings.Join(errs, "\n  "))
	}
}

// currentBranch returns the current Git branch name from GITHUB_REF_NAME, or
// attempts to read it from GITHUB_REF. Falls back to "main".
func currentBranch() string {
	if b := os.Getenv("GITHUB_REF_NAME"); b != "" {
		return b
	}
	ref := os.Getenv("GITHUB_REF")
	if strings.HasPrefix(ref, "refs/heads/") {
		return strings.TrimPrefix(ref, "refs/heads/")
	}
	// Try the push event payload
	if eventPath := os.Getenv("GITHUB_EVENT_PATH"); eventPath != "" {
		data, err := os.ReadFile(eventPath)
		if err == nil {
			var event struct {
				Ref string `json:"ref"`
			}
			if json.Unmarshal(data, &event) == nil && strings.HasPrefix(event.Ref, "refs/heads/") {
				return strings.TrimPrefix(event.Ref, "refs/heads/")
			}
		}
	}
	return "main"
}

// getInput reads a GitHub Actions input (INPUT_<NAME> env var).
func getInput(name, defaultValue string) string {
	key := "INPUT_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

// getEnv reads an environment variable with a fallback default.
func getEnv(name, defaultValue string) string {
	if val := os.Getenv(name); val != "" {
		return val
	}
	return defaultValue
}

// parseLines splits a newline/space-separated string into a slice of non-empty strings.
func parseLines(s string) []string {
	var out []string
	for _, line := range strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == '\r' }) {
		if t := strings.TrimSpace(line); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func serviceNames(svcs []*config.Service) []string {
	names := make([]string, len(svcs))
	for i, s := range svcs {
		names[i] = s.Name
	}
	return names
}

// Package detector finds proto files changed in a commit and determines which
// configured services are affected (including via transitive imports).
package detector

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bangzzzz/pb-go-action/internal/config"
)

// importRe matches proto import statements:
//
//	import "path/to/file.proto";
//	import weak "path/to/file.proto";
//	import public "path/to/file.proto";
var importRe = regexp.MustCompile(`^\s*import\s+(?:weak\s+|public\s+)?"([^"]+)"`)

// GetChangedFiles returns the .proto files that changed in the most recent push.
// It reads GITHUB_EVENT_PATH to obtain the before/after SHAs for push events;
// otherwise it falls back to comparing HEAD with its parent.
func GetChangedFiles(workspace string) ([]string, error) {
	before, after := gitSHARange()
	var (
		out []byte
		err error
	)

	if before != "" && after != "" {
		out, err = gitDiff(workspace, before, after)
	} else {
		// First commit or unknown range – diff against parent.
		out, err = gitDiff(workspace, "HEAD~1", "HEAD")
		if err != nil {
			// Possibly a single-commit repo; list all tracked proto files.
			out, err = gitListAll(workspace)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("getting changed files: %w", err)
	}

	return filterProto(strings.Split(string(out), "\n")), nil
}

// FindAffectedServices returns the services that need to be regenerated because
// at least one of their proto files (or a file they transitively import) changed.
func FindAffectedServices(workspace string, cfg *config.Config, changedFiles []string) ([]*config.Service, error) {
	if len(changedFiles) == 0 {
		return nil, nil
	}

	// Build reverse import graph: importedPath → []files that import it.
	reverseGraph, err := buildReverseImportGraph(workspace, cfg)
	if err != nil {
		return nil, fmt.Errorf("building import graph: %w", err)
	}

	// Expand the changed set to include all transitively affected files.
	affected := make(map[string]bool)
	for _, f := range changedFiles {
		markAffected(filepath.ToSlash(f), reverseGraph, affected)
	}

	// Identify which services own at least one affected proto file.
	seen := make(map[string]bool)
	var result []*config.Service

	for i := range cfg.Services {
		svc := &cfg.Services[i]
		protos, err := FindProtoFiles(filepath.Join(workspace, svc.ProtoDir))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("finding proto files for service %q: %w", svc.Name, err)
		}

		for _, p := range protos {
			rel, err := filepath.Rel(workspace, p)
			if err != nil {
				continue
			}
			if affected[filepath.ToSlash(rel)] {
				if !seen[svc.Name] {
					seen[svc.Name] = true
					result = append(result, svc)
				}
				break
			}
		}
	}

	return result, nil
}

// FindProtoFiles returns all .proto files found recursively under dir.
func FindProtoFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".proto") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// ParseImports returns the import paths declared in the given .proto file.
func ParseImports(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var imports []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if m := importRe.FindStringSubmatch(scanner.Text()); m != nil {
			imports = append(imports, m[1])
		}
	}
	return imports, scanner.Err()
}

// ── helpers ───────────────────────────────────────────────────────────────────

// buildReverseImportGraph scans all proto files referenced by the config and
// builds a map: importedPath → []files that import it (all paths relative to
// workspace root and using forward slashes).
func buildReverseImportGraph(workspace string, cfg *config.Config) (map[string][]string, error) {
	// Collect all directories to scan.
	dirs := make(map[string]bool)
	for _, svc := range cfg.Services {
		dirs[svc.ProtoDir] = true
		for _, d := range svc.IncludeDirs {
			dirs[d] = true
		}
	}

	graph := make(map[string][]string)

	for dir := range dirs {
		protos, err := FindProtoFiles(filepath.Join(workspace, dir))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		for _, protoAbs := range protos {
			imports, err := ParseImports(protoAbs)
			if err != nil {
				return nil, fmt.Errorf("parsing imports in %s: %w", protoAbs, err)
			}

			relImporter, _ := filepath.Rel(workspace, protoAbs)
			relImporter = filepath.ToSlash(relImporter)

			for _, imp := range imports {
				// Skip well-known Google protobuf types.
				if strings.HasPrefix(imp, "google/protobuf/") {
					continue
				}
				graph[imp] = append(graph[imp], relImporter)
			}
		}
	}

	return graph, nil
}

// markAffected recursively marks file and all files that import it as affected.
func markAffected(file string, reverseGraph map[string][]string, visited map[string]bool) {
	if visited[file] {
		return
	}
	visited[file] = true
	for _, importer := range reverseGraph[file] {
		markAffected(importer, reverseGraph, visited)
	}
}

// gitDiff returns the names of files changed between two git references.
func gitDiff(workspace, before, after string) ([]byte, error) {
	cmd := exec.Command("git", "diff", "--name-only", before, after)
	cmd.Dir = workspace
	return cmd.Output()
}

// gitListAll returns all tracked files in the repository.
func gitListAll(workspace string) ([]byte, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = workspace
	return cmd.Output()
}

// filterProto filters a slice of file paths, keeping only .proto files.
func filterProto(lines []string) []string {
	var out []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" && strings.HasSuffix(l, ".proto") {
			out = append(out, l)
		}
	}
	return out
}

// sha1HexLen is the length of a hex-encoded SHA-1 hash (Git object ID).
const sha1HexLen = 40

// gitSHARange reads the GitHub push event payload and returns before/after SHAs.
func gitSHARange() (before, after string) {
	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	if eventPath == "" {
		return "", ""
	}
	data, err := os.ReadFile(eventPath)
	if err != nil {
		return "", ""
	}
	var event struct {
		Before string `json:"before"`
		After  string `json:"after"`
	}
	if json.Unmarshal(data, &event) != nil {
		return "", ""
	}
	// All-zero SHA means the branch was just created – no "before" exists.
	allZero := strings.Repeat("0", sha1HexLen)
	if event.Before == allZero {
		return "", ""
	}
	return event.Before, event.After
}

package stage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/simtabi/probaci/internal/detect"
	"github.com/simtabi/probaci/internal/tool"
)

// TestDetectToolsResolveInRegistry guards against drift: every lint/audit tool
// that detection can emit for each ecosystem must exist in the tool registry,
// otherwise the lint/audit stages silently skip with "unknown tool".
func TestDetectToolsResolveInRegistry(t *testing.T) {
	fixtures := map[string][]string{
		"go":     {"go.mod"},
		"python": {"pyproject.toml"},
		"node":   {"package.json", "package-lock.json"},
		"rust":   {"Cargo.toml"},
		"ruby":   {"Gemfile"},
		"java":   {"pom.xml"},
	}
	reg := tool.New(nil)
	for name, markers := range fixtures {
		dir := t.TempDir()
		for _, m := range markers {
			if err := os.WriteFile(filepath.Join(dir, m), []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		proj := detect.DetectProject(dir)
		if len(proj.Languages) == 0 {
			t.Errorf("%s fixture not detected", name)
			continue
		}
		for _, l := range proj.Languages {
			for _, toolName := range []string{l.LintTool, l.AuditTool} {
				if toolName == "" {
					continue
				}
				if _, err := reg.Resolve(toolName); err != nil {
					t.Errorf("%s: tool %q referenced by detect but not in registry: %v", l.Name, toolName, err)
				}
			}
		}
	}
}

// Package detect inspects a repository to infer its languages and the default
// install/test commands, mirroring (and extending) the heuristics of the
// original bash tool. Empty config fields fall back to these guesses.
package detect

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Language describes a detected ecosystem and its default commands/tools.
type Language struct {
	Name       string
	InstallCmd string
	TestCmd    string
	LintTool   string // tool-registry key
	AuditTool  string // tool-registry key
	// Dir is the module's directory relative to the repository root ("" for
	// the root itself). Commands and tools for this language run there.
	Dir string
}

// Project is the result of detection.
type Project struct {
	Languages []Language
	// NoLock is set when a lockfile is expected but missing (CI installs may drift).
	NoLock bool
}

// exists reports whether dir/name is present.
func exists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

// DetectProject returns the languages present in dir. A polyglot repo can match
// more than one.
func DetectProject(dir string) Project {
	var p Project
	if lang, ok := detectNode(dir, &p); ok {
		p.Languages = append(p.Languages, lang)
	}
	if exists(dir, "pyproject.toml") || exists(dir, "requirements.txt") {
		p.Languages = append(p.Languages, detectPython(dir))
	}
	if modDir, ok := goModuleDir(dir); ok {
		p.Languages = append(p.Languages, goLanguage(modDir))
	}
	if exists(dir, "Cargo.toml") {
		p.Languages = append(p.Languages, Language{
			Name: "rust", InstallCmd: "cargo fetch", TestCmd: "cargo test",
			LintTool: "clippy", AuditTool: "cargo-audit",
		})
	}
	if exists(dir, "Gemfile") {
		p.Languages = append(p.Languages, Language{
			Name: "ruby", InstallCmd: "bundle install", TestCmd: "bundle exec rake",
			LintTool: "rubocop", AuditTool: "bundler-audit",
		})
	}
	if exists(dir, "pom.xml") || exists(dir, "build.gradle") || exists(dir, "build.gradle.kts") {
		p.Languages = append(p.Languages, detectJava(dir))
	}
	return p
}

// skipModuleDirs are never the project's own module: vendored copies, test
// fixtures and JavaScript dependencies. Directories starting with "." or "_"
// are skipped too, as the go tool itself ignores them.
var skipModuleDirs = map[string]bool{
	"vendor":       true,
	"testdata":     true,
	"node_modules": true,
}

// goModuleDir finds the Go module: go.mod at the root, else in src/ (the
// layout every Simtabi Go repository uses), else in the first other immediate
// subdirectory, in name order. It never looks deeper than one level, so a
// fixture module elsewhere in the tree is not mistaken for the project.
func goModuleDir(dir string) (string, bool) {
	if exists(dir, "go.mod") {
		return "", true
	}
	if exists(filepath.Join(dir, "src"), "go.mod") {
		return "src", true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || skipModuleDirs[name] ||
			strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if exists(filepath.Join(dir, name), "go.mod") {
			return name, true
		}
	}
	return "", false
}

// goLanguage returns the Go defaults for a module in modDir (relative to the
// repository root). A nested module gets `go -C <dir>` commands, because a
// bare `go test ./...` at the root finds no packages and passes.
func goLanguage(modDir string) Language {
	l := Language{
		Name: "go", InstallCmd: "go mod download", TestCmd: "go test ./...",
		LintTool: "golangci-lint", AuditTool: "govulncheck", Dir: modDir,
	}
	if modDir != "" {
		l.InstallCmd = "go -C " + modDir + " mod download"
		l.TestCmd = "go -C " + modDir + " test ./..."
	}
	return l
}

func detectNode(dir string, p *Project) (Language, bool) {
	if !exists(dir, "package.json") {
		return Language{}, false
	}
	l := Language{Name: "node", LintTool: "eslint"}
	switch {
	case exists(dir, "pnpm-lock.yaml"):
		l.InstallCmd, l.TestCmd, l.AuditTool = "pnpm install --frozen-lockfile", "pnpm test", "pnpm-audit"
	case exists(dir, "yarn.lock"):
		l.InstallCmd, l.TestCmd, l.AuditTool = "yarn install --frozen-lockfile", "yarn test", "yarn-audit"
	case exists(dir, "package-lock.json"):
		l.InstallCmd, l.TestCmd, l.AuditTool = "npm ci", "npm test", "npm-audit"
	default:
		l.InstallCmd, l.TestCmd, l.AuditTool = "npm install", "npm test", "npm-audit"
		p.NoLock = true
	}
	return l, true
}

func detectPython(dir string) Language {
	l := Language{Name: "python", LintTool: "ruff", AuditTool: "pip-audit"}
	switch {
	case exists(dir, "uv.lock"):
		l.InstallCmd, l.TestCmd = "uv sync --frozen", "uv run pytest -q"
	case exists(dir, "poetry.lock"):
		l.InstallCmd, l.TestCmd = "poetry install", "poetry run pytest -q"
	case exists(dir, "requirements.txt"):
		l.InstallCmd, l.TestCmd = "pip install -r requirements.txt", "pytest -q"
	default:
		l.InstallCmd, l.TestCmd = "pip install -e .", "pytest -q"
	}
	return l
}

func detectJava(dir string) Language {
	// No reliable zero-config containerized Java linter (checkstyle/PMD need a
	// project ruleset); leave LintTool empty so users opt in via config.
	l := Language{Name: "java", AuditTool: "owasp-dependency-check"}
	if exists(dir, "pom.xml") {
		l.InstallCmd, l.TestCmd = "mvn -q -B dependency:go-offline", "mvn -q -B test"
	} else {
		l.InstallCmd, l.TestCmd = "./gradlew dependencies", "./gradlew test"
	}
	return l
}

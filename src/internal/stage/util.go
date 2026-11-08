package stage

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/simtabi/probaci/internal/result"
)

// worsen returns the more severe of two statuses, so a folded result reflects
// the worst outcome. Order (best→worst): skip < pass < fail < error.
func worsen(a, b result.Status) result.Status {
	rank := map[result.Status]int{
		result.StatusSkip:  0,
		result.StatusPass:  1,
		result.StatusFail:  2,
		result.StatusError: 3,
	}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func fileExists(repo, rel string) bool {
	_, err := os.Stat(filepath.Join(repo, rel))
	return err == nil
}

var versionRe = regexp.MustCompile(`[0-9]+(\.[0-9]+)*`)

// grepVersion finds the first version-looking token on a line containing key.
func grepVersion(content, key string) string {
	for _, line := range strings.Split(content, "\n") {
		if !strings.Contains(line, key) {
			continue
		}
		if m := versionRe.FindString(line); m != "" {
			return m
		}
	}
	return ""
}

// installCmd resolves the install command: config override, else the first
// detected language's default.
func (c *stageCtx) installCmd() string {
	if c.cfg.Project.InstallCmd != "" {
		return c.cfg.Project.InstallCmd
	}
	for _, l := range c.project.Languages {
		if l.InstallCmd != "" {
			return l.InstallCmd
		}
	}
	return ""
}

// testCmd resolves the test command similarly.
func (c *stageCtx) testCmd() string {
	if c.cfg.Project.TestCmd != "" {
		return c.cfg.Project.TestCmd
	}
	for _, l := range c.project.Languages {
		if l.TestCmd != "" {
			return l.TestCmd
		}
	}
	return ""
}

// cleanCloneImage picks a base image suited to the detected language so the
// committed state can be installed and tested in a clean container.
func (c *stageCtx) cleanCloneImage() string {
	lang := ""
	if len(c.project.Languages) > 0 {
		lang = c.project.Languages[0].Name
	}
	switch lang {
	case "go":
		return "golang:latest"
	case "python":
		return "python:3-slim"
	case "node":
		return "node:lts-slim"
	case "rust":
		return "rust:latest"
	case "ruby":
		return "ruby:latest"
	case "java":
		return "eclipse-temurin:latest"
	default:
		return "debian:stable-slim"
	}
}

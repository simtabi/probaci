package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// legacyConfig mirrors the old ci-local.config.json (bash tool) schema.
type legacyConfig struct {
	InstallCmd  string   `json:"install_cmd"`
	TestCmd     string   `json:"test_cmd"`
	LintCmd     string   `json:"lint_cmd"`
	AuditCmd    string   `json:"audit_cmd"`
	RemoteURL   string   `json:"remote_url"`
	SecretsFile string   `json:"secrets_file"`
	EnvFile     string   `json:"env_file"`
	Stages      []string `json:"stages"`
}

// legacyStageMap renames bash-era stage names to their probaci equivalents.
var legacyStageMap = map[string]string{
	"actionlint":  "workflow-lint",
	"act":         "workflow-run",
	"secrets":     "secrets",
	"versions":    "versions",
	"lint":        "lint",
	"clean-clone": "clean-clone",
	"audit":       "audit",
}

// Migrate reads a legacy ci-local.config.json from legacyPath and returns an
// equivalent probaci Config built on top of the defaults.
func Migrate(legacyPath string) (Config, error) {
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return Config{}, err
	}
	var legacy legacyConfig
	if err := json.Unmarshal(data, &legacy); err != nil {
		return Config{}, fmt.Errorf("parse legacy config: %w", err)
	}

	cfg := Default()
	cfg.Project.InstallCmd = legacy.InstallCmd
	cfg.Project.TestCmd = legacy.TestCmd
	cfg.Project.LintCmd = legacy.LintCmd
	cfg.Project.AuditCmd = legacy.AuditCmd
	if legacy.SecretsFile != "" {
		cfg.Secrets = legacy.SecretsFile
	}
	if legacy.EnvFile != "" {
		cfg.EnvFile = legacy.EnvFile
	}

	if len(legacy.Stages) > 0 {
		enabled := map[string]bool{}
		for _, s := range legacy.Stages {
			if mapped, ok := legacyStageMap[s]; ok {
				enabled[mapped] = true
			}
		}
		stages := make([]Stage, 0, len(DefaultStageOrder))
		for _, name := range DefaultStageOrder {
			stages = append(stages, Stage{Name: name, Enabled: enabled[name]})
		}
		cfg.Stages = stages
	}
	return cfg, nil
}

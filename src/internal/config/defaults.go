package config

// DefaultStageOrder is the canonical pipeline order. Stages run cheapest and
// highest-signal first so the slow Docker workflow run surfaces last.
var DefaultStageOrder = []string{
	"detect",
	"workflow-lint",
	"secrets",
	"versions",
	"lint",
	"clean-clone",
	"audit",
	"sast",
	"dockerfile-lint",
	"yaml-lint",
	"container-scan",
	"commitlint",
	"workflow-run",
}

// stagesEnabledByDefault are the stages turned on out of the box. The rest are
// present in the config (so users can flip them on) but disabled by default to
// keep a first run fast.
var stagesEnabledByDefault = map[string]bool{
	"detect":          true,
	"workflow-lint":   true,
	"secrets":         true,
	"versions":        true,
	"lint":            true,
	"clean-clone":     true,
	"audit":           true,
	"sast":            false,
	"dockerfile-lint": true,
	"yaml-lint":       false,
	"container-scan":  false,
	"commitlint":      false,
	"workflow-run":    true,
}

// Default returns the built-in configuration. This is the lowest layer of the
// precedence chain and is embedded in the binary so a clean install needs no
// files or network.
func Default() Config {
	stages := make([]Stage, 0, len(DefaultStageOrder))
	for _, name := range DefaultStageOrder {
		stages = append(stages, Stage{Name: name, Enabled: stagesEnabledByDefault[name]})
	}
	return Config{
		Version: SchemaVersion,
		Project: Project{},
		Docker: Docker{
			Enabled: BoolPtr(true),
			Backend: BackendAuto,
			Pull:    PullMissing,
			Resources: Resources{
				PidsLimit: 512,
			},
			AllowSocketMount: BoolPtr(false),
		},
		Security: Security{
			VerifyImages: "advisory",
		},
		Platforms: map[string]Platform{
			"github":    {Enabled: true},
			"gitlab":    {Enabled: true, TokenEnv: "GITLAB_TOKEN"},
			"bitbucket": {Enabled: true},
			"gitea":     {Enabled: true},
			"azure":     {Enabled: true},
		},
		Stages:  stages,
		Secrets: ".probaci/secrets",
		EnvFile: ".env",
		TUI:     TUI{Theme: "auto"},
	}
}

// EnabledStages returns the ordered names of stages that are enabled.
func (c Config) EnabledStages() []string {
	out := make([]string, 0, len(c.Stages))
	for _, s := range c.Stages {
		if s.Enabled {
			out = append(out, s.Name)
		}
	}
	return out
}

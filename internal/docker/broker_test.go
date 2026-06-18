package docker

import (
	"strings"
	"testing"

	"github.com/simtabi/probaci/internal/config"
)

// TestBuildArgsHardening asserts the least-privilege defaults are always set and
// that offline tools get no network.
func TestBuildArgsHardening(t *testing.T) {
	b := &Broker{bin: "docker", cfg: config.Docker{Resources: config.Resources{PidsLimit: 512}}}
	args := b.buildArgs(RunSpec{Image: "alpine:3", Args: []string{"true"}, RepoMount: "/repo"})
	joined := strings.Join(args, " ")

	for _, want := range []string{
		"--rm", "--cap-drop ALL", "--security-opt no-new-privileges",
		"--read-only", "--network none", "--pids-limit 512",
		"-v /repo:/workspace:ro", "-w /workspace", "alpine:3 true",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected args to contain %q; got: %s", want, joined)
		}
	}
}

// TestBuildArgsWritable asserts clean-clone style writable mounts drop the :ro
// flag and the read-only rootfs.
func TestBuildArgsWritable(t *testing.T) {
	b := &Broker{bin: "docker", cfg: config.Docker{}}
	args := b.buildArgs(RunSpec{Image: "golang", Args: []string{"go", "test"}, RepoMount: "/x", Writable: true, Network: "bridge"})
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "--read-only") {
		t.Error("writable run should not be read-only")
	}
	if !strings.Contains(joined, "-v /x:/workspace ") {
		t.Errorf("writable mount should omit :ro; got %s", joined)
	}
	if !strings.Contains(joined, "--network bridge") {
		t.Errorf("explicit network not honored; got %s", joined)
	}
}

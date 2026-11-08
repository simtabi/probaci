package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	installSystem bool
	installDir    string
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Place this probaci binary on your PATH (ddev-style global install)",
	Long: "Copies the currently-running probaci executable to a directory on your " +
		"PATH. Defaults to a per-user location; use --system for a machine-wide install.",
	RunE: func(cmd *cobra.Command, args []string) error {
		t := theme()
		src, err := os.Executable()
		if err != nil {
			return failure(fmt.Errorf("locate running binary: %w", err))
		}
		src, _ = filepath.EvalSymlinks(src)

		dst := installTarget()
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return failure(fmt.Errorf("create %s (try --system with elevation, or --dir): %w", dst, err))
		}
		out := filepath.Join(dst, binaryName())
		if err := copyExecutable(src, out); err != nil {
			return failure(fmt.Errorf("install to %s: %w", out, err))
		}
		fmt.Printf("%s installed probaci to %s\n", t.Pass("ok"), out)
		if !onPath(dst) {
			fmt.Println(t.Warn("note: " + dst + " is not on your PATH; add it to use `probaci` from anywhere"))
		}
		return nil
	},
}

var uninstallPurge bool

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the installed probaci binary (and optionally the config home)",
	RunE: func(cmd *cobra.Command, args []string) error {
		t := theme()
		out := filepath.Join(installTarget(), binaryName())
		if err := os.Remove(out); err != nil && !os.IsNotExist(err) {
			return failure(fmt.Errorf("remove %s: %w", out, err))
		}
		fmt.Printf("%s removed %s\n", t.Pass("ok"), out)
		if uninstallPurge {
			home := installHome()
			if err := os.RemoveAll(home); err != nil {
				return failure(fmt.Errorf("purge %s: %w", home, err))
			}
			fmt.Printf("%s purged config home %s\n", t.Pass("ok"), home)
		}
		return nil
	},
}

func init() {
	installCmd.Flags().BoolVar(&installSystem, "system", false, "install machine-wide (requires elevation)")
	installCmd.Flags().StringVar(&installDir, "dir", "", "explicit install directory (overrides default)")
	uninstallCmd.Flags().BoolVar(&installSystem, "system", false, "remove the machine-wide install")
	uninstallCmd.Flags().StringVar(&installDir, "dir", "", "explicit directory to remove from")
	uninstallCmd.Flags().BoolVar(&uninstallPurge, "purge", false, "also remove the config home (~/.config/probaci)")
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "probaci.exe"
	}
	return "probaci"
}

// installTarget resolves the destination directory for the binary.
func installTarget() string {
	if installDir != "" {
		return installDir
	}
	if installSystem {
		if runtime.GOOS == "windows" {
			base := os.Getenv("ProgramFiles")
			if base == "" {
				base = `C:\Program Files`
			}
			return filepath.Join(base, "probaci")
		}
		return "/usr/local/bin"
	}
	// Per-user default.
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = os.Getenv("APPDATA")
		}
		return filepath.Join(base, "Programs", "probaci")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin")
}

func installHome() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		base := os.Getenv("AppData")
		return filepath.Join(base, "probaci")
	}
	return filepath.Join(home, ".config", "probaci")
}

// copyExecutable copies src to dst (0755), writing atomically via a temp file.
func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".probaci-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dst)
}

func onPath(dir string) bool {
	dir = filepath.Clean(dir)
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if filepath.Clean(p) == dir {
			return true
		}
	}
	return false
}

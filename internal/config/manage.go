package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// BackupSuffix is appended to a config file when it is backed up before a
// destructive operation.
const BackupSuffix = ".bak"

// Write persists a config to path with 0644 permissions, creating parent dirs.
// The write itself is atomic (temp + rename); callers that mutate the shared
// user home should wrap this in Locked for cross-process safety.
func Write(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeJSON(path, c, 0o644)
}

// Locked runs fn while holding an exclusive advisory lock on lockPath, so two
// concurrent probaci processes can't corrupt the user home. A blank lockPath
// runs fn without locking.
func Locked(lockPath string, fn func() error) error {
	if lockPath == "" {
		return fn()
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return err
	}
	lock := flock.New(lockPath)
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("acquire config lock: %w", err)
	}
	defer func() { _ = lock.Unlock() }()
	return fn()
}

// Init writes a fresh config built from defaults to path. It refuses to
// overwrite an existing file unless force is set (in which case it backs up).
func Init(path string, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("%s already exists (use --force to overwrite)", path)
	}
	if _, err := os.Stat(path); err == nil && force {
		if _, err := Backup(path); err != nil {
			return err
		}
	}
	return Write(path, Default())
}

// Reset restores path to built-in defaults, backing up any existing file first.
func Reset(path string) (backup string, err error) {
	if _, err := os.Stat(path); err == nil {
		if backup, err = Backup(path); err != nil {
			return "", err
		}
	}
	return backup, Write(path, Default())
}

// Backup copies path to path+stamp+.bak and returns the backup path.
func Backup(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	backup := fmt.Sprintf("%s.%s%s", path, stamp, BackupSuffix)
	// #nosec G703 -- backup path is derived from the probaci-controlled config path
	if err := os.WriteFile(backup, data, 0o644); err != nil {
		return "", err
	}
	return backup, nil
}

// Restore writes the contents of backup back to path. If backup is empty, the
// most recent *.bak sibling of path is used.
func Restore(path, backup string) (string, error) {
	if backup == "" {
		latest, err := latestBackup(path)
		if err != nil {
			return "", err
		}
		backup = latest
	}
	data, err := os.ReadFile(backup)
	if err != nil {
		return "", err
	}
	if _, err := parse(data); err != nil {
		return "", fmt.Errorf("backup %s is not valid config: %w", backup, err)
	}
	// #nosec G703 -- restores to the probaci-controlled config path
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return backup, nil
}

func latestBackup(path string) (string, error) {
	matches, err := filepath.Glob(path + ".*" + BackupSuffix)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", errors.New("no backup found to restore")
	}
	var newest string
	var newestMod time.Time
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		if info.ModTime().After(newestMod) {
			newestMod, newest = info.ModTime(), m
		}
	}
	if newest == "" {
		return "", errors.New("no readable backup found")
	}
	return newest, nil
}

// Marshal returns the indented JSON form of a config (used by `config show`).
func Marshal(c Config) ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// WriteSchema writes the embedded JSON Schema to path (atomically), so editors
// can validate and autocomplete probaci.json.
func WriteSchema(path string) error {
	return atomicWrite(path, Schema(), 0o644)
}

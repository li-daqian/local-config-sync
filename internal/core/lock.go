package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

func WithRepositoryLock(lockPath string, operation func() error) error {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, os.ErrExist) {
		owner, readErr := os.ReadFile(lockPath)
		if readErr != nil {
			owner = []byte("unknown")
		}
		return NewError(ErrRepositoryLocked, "Repository is currently being synchronized", map[string]any{"lockPath": lockPath, "owner": string(owner)})
	}
	if err != nil {
		return err
	}
	owner, _ := json.Marshal(map[string]any{"pid": os.Getpid(), "createdAt": time.Now().UTC().Format(time.RFC3339Nano)})
	if _, err := file.Write(owner); err != nil {
		_ = file.Close()
		_ = os.Remove(lockPath)
		return err
	}
	defer func() { _ = file.Close(); _ = os.Remove(lockPath) }()
	return operation()
}

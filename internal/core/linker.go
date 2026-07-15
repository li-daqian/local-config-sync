package core

import (
	"errors"
	"os"
	"path/filepath"
)

func Materialize(source, target string, mode LinkMode, replaceCopy bool) error {
	if err := os.MkdirAll(source, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	stat, statErr := os.Lstat(target)
	exists := statErr == nil
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	if mode == LinkModeSymlink {
		if exists && stat.Mode()&os.ModeSymlink != 0 {
			current, err := os.Readlink(target)
			if err != nil {
				return err
			}
			if !filepath.IsAbs(current) {
				current = filepath.Join(filepath.Dir(target), current)
			}
			current, _ = filepath.Abs(current)
			expected, _ := filepath.Abs(source)
			if current == expected {
				return nil
			}
			if err := os.Remove(target); err != nil {
				return err
			}
		} else if exists {
			return NewError(ErrFilesystemFailed, "Target already exists and is not a symlink: "+target, map[string]any{"target": target})
		}
		relative, err := filepath.Rel(filepath.Dir(target), source)
		if err != nil {
			return err
		}
		return os.Symlink(relative, target)
	}
	if exists && !replaceCopy {
		return NewError(ErrFilesystemFailed, "Copy target already exists: "+target, map[string]any{"target": target})
	}
	if replaceCopy {
		if err := os.RemoveAll(target); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	return CopyTree(source, target)
}

func RemoveMaterialized(target string, mode LinkMode, keepFiles bool) error {
	if keepFiles && mode == LinkModeSymlink {
		source, err := os.Readlink(target)
		if err != nil {
			return err
		}
		if !filepath.IsAbs(source) {
			source = filepath.Join(filepath.Dir(target), source)
		}
		if err := os.Remove(target); err != nil {
			return err
		}
		if err := os.MkdirAll(target, 0o755); err != nil {
			return err
		}
		return CopyTree(source, target)
	}
	if !keepFiles {
		return os.RemoveAll(target)
	}
	return nil
}

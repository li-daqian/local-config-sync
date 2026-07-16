package core

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func SafeRelativePath(value, field string) (string, error) {
	normalized := strings.ReplaceAll(value, "\\", "/")
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.TrimSuffix(normalized, "/")
	if normalized == "" || filepath.IsAbs(value) || normalized == ".." || strings.HasPrefix(normalized, "../") || strings.Contains(normalized, "/../") {
		return "", NewError(ErrInvalidArguments, field+" must be a safe relative path", map[string]any{"field": field, "value": value})
	}
	return normalized, nil
}

func ResolveInside(root, relativePath string) (string, error) {
	root, _ = filepath.Abs(root)
	result, _ := filepath.Abs(filepath.Join(root, filepath.FromSlash(relativePath)))
	fromRoot, err := filepath.Rel(root, result)
	if err != nil || fromRoot == ".." || strings.HasPrefix(fromRoot, ".."+string(filepath.Separator)) || filepath.IsAbs(fromRoot) {
		return "", NewError(ErrInvalidArguments, "Path escapes its allowed root", map[string]any{"root": root, "path": relativePath})
	}
	return result, nil
}

func ListFiles(root string) ([]string, error) {
	output := []string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			output = append(output, filepath.ToSlash(relative))
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Strings(output)
	return output, nil
}

func SnapshotDirectory(root, prefix string) (map[string]FileSnapshot, error) {
	files, err := ListFiles(root)
	if err != nil {
		return nil, err
	}
	result := map[string]FileSnapshot{}
	for _, relativePath := range files {
		path, err := ResolveInside(root, relativePath)
		if err != nil {
			return nil, err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(content)
		key := relativePath
		if prefix != "" {
			key = strings.TrimSuffix(prefix, "/") + "/" + relativePath
		}
		result[key] = FileSnapshot{SHA256: hex.EncodeToString(sum[:]), Size: int64(len(content))}
	}
	return result, nil
}

func SnapshotPath(path, prefix string, kind MappingKind) (map[string]FileSnapshot, error) {
	if kind != MappingKindFile {
		return SnapshotDirectory(path, prefix)
	}
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]FileSnapshot{}, nil
	}
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(content)
	return map[string]FileSnapshot{prefix: {SHA256: hex.EncodeToString(sum[:]), Size: int64(len(content))}}, nil
}

func SnapshotsEqual(a FileSnapshot, aOK bool, b FileSnapshot, bOK bool) bool {
	return aOK == bOK && (!aOK || (a.SHA256 == b.SHA256 && a.Size == b.Size && a.Deleted == b.Deleted))
}

func CopyTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(destination, info.Mode().Perm())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, destination)
		}
		return copyFile(path, destination, info.Mode().Perm())
	})
}

func copyFile(source, target string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		_ = input.Close()
		return err
	}
	_, copyErr := io.Copy(output, input)
	inputCloseErr := input.Close()
	outputCloseErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if inputCloseErr != nil {
		return inputCloseErr
	}
	return outputCloseErr
}

func CopyFileReplace(source, target string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return NewError(ErrFilesystemFailed, "Mapped file is not a regular file: "+source, map[string]any{"path": source})
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), ".local-config-copy-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return err
	}
	_ = os.Remove(temporaryPath)
	if err := copyFile(source, temporaryPath, info.Mode().Perm()); err != nil {
		_ = os.Remove(temporaryPath)
		return err
	}
	if err := atomicReplace(temporaryPath, target); err != nil {
		_ = os.Remove(temporaryPath)
		return err
	}
	return nil
}

func mapsEqual(a, b map[string]FileSnapshot) bool {
	if len(a) != len(b) {
		return false
	}
	for path, left := range a {
		right, ok := b[path]
		if !ok || left != right {
			return false
		}
	}
	return true
}

package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const excludeMarker = "# local-config-sync"

func excludeRule(targetPath string) string {
	return "/" + strings.Trim(strings.ReplaceAll(targetPath, "\\", "/"), "/")
}

func legacyExcludeRule(targetPath string) string {
	return excludeRule(targetPath) + "/"
}

func readOptional(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []byte{}, nil
	}
	return content, err
}

func AddExclude(path, targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content, err := readOptional(path)
	if err != nil {
		return err
	}
	existing := string(content)
	rule := excludeRule(targetPath)
	lines := strings.Split(strings.ReplaceAll(existing, "\r\n", "\n"), "\n")
	for _, line := range lines {
		if line == rule || line == legacyExcludeRule(targetPath) {
			return nil
		}
	}
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	hasMarker := false
	for _, line := range lines {
		if line == excludeMarker {
			hasMarker = true
		}
	}
	if !hasMarker {
		existing += excludeMarker + "\n"
	}
	return os.WriteFile(path, []byte(existing+rule+"\n"), 0o644)
}

func RemoveExclude(path, targetPath string) error {
	content, err := readOptional(path)
	if err != nil {
		return err
	}
	rule := excludeRule(targetPath)
	legacyRule := legacyExcludeRule(targetPath)
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n") {
		if line != rule && line != legacyRule {
			lines = append(lines, line)
		}
	}
	marker := -1
	for i, line := range lines {
		if line == excludeMarker {
			marker = i
			break
		}
	}
	if marker >= 0 {
		hasRules := false
		for _, line := range lines[marker+1:] {
			if strings.HasPrefix(line, "/") {
				hasRules = true
			}
		}
		if !hasRules {
			lines = append(lines[:marker], lines[marker+1:]...)
		}
	}
	value := strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
	return os.WriteFile(path, []byte(value), 0o644)
}

func HasExclude(path, targetPath string) (bool, error) {
	content, err := readOptional(path)
	if err != nil {
		return false, err
	}
	rule := excludeRule(targetPath)
	legacyRule := legacyExcludeRule(targetPath)
	for _, line := range strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n") {
		if line == rule || line == legacyRule {
			return true, nil
		}
	}
	return false, nil
}

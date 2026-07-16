package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var sensitiveExact = map[string]bool{".env": true, "id_rsa": true, "id_ed25519": true, "credentials": true, "credentials.json": true, "application-prod.yml": true, "application-production.yml": true}
var sensitiveSuffixes = []string{".pem", ".key", ".p12", ".pfx"}

func ScanSensitive(root string, scopes []string) ([]string, error) {
	matches := []string{}
	for _, scope := range scopes {
		scopePath := filepath.Join(root, filepath.FromSlash(scope))
		info, statErr := os.Stat(scopePath)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return nil, statErr
		}
		files := []string{}
		if info.Mode().IsRegular() {
			files = append(files, "")
		} else {
			var err error
			files, err = ListFiles(scopePath)
			if err != nil {
				return nil, err
			}
		}
		for _, file := range files {
			name := strings.ToLower(filepath.Base(scope))
			path := scope
			if file != "" {
				name = strings.ToLower(filepath.Base(file))
				path = strings.TrimSuffix(scope, "/") + "/" + file
			}
			sensitive := sensitiveExact[name] || strings.HasPrefix(name, ".env.")
			for _, suffix := range sensitiveSuffixes {
				if strings.HasSuffix(name, suffix) {
					sensitive = true
				}
			}
			if sensitive {
				matches = append(matches, path)
			}
		}
	}
	return matches, nil
}

func AssertNoSensitive(matches []string, allow bool) error {
	if len(matches) > 0 && !allow {
		return NewError(ErrUnsafeSecretPattern, "Sensitive file patterns were found. Review them before explicitly allowing this sync.", map[string]any{"paths": matches})
	}
	return nil
}

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunOutputsNormalizedJSON(t *testing.T) {
	directory := t.TempDir()
	manifestPath := filepath.Join(directory, "release.yaml")
	manifest := `
schemaVersion: 1
releaseId: release-2026.07.22.1
artifacts:
  jetbrains:
    version: 0.8.0
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(
		[]string{"--file", manifestPath, "--tag", "release-2026.07.22.1", "--json"},
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"channel":"default"`) {
		t.Fatalf("stdout = %q, want normalized default channel", stdout.String())
	}
}

func TestRunRejectsMissingArguments(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(nil, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("run() exit code = %d, want 2", exitCode)
	}
	if !strings.Contains(stderr.String(), "--file is required") {
		t.Fatalf("stderr = %q, want missing file message", stderr.String())
	}
}

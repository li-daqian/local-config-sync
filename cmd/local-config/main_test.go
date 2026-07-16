package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func captureStdout(t *testing.T, operation func() error) []byte {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writer
	operationErr := operation()
	_ = writer.Close()
	os.Stdout = original
	content, readErr := io.ReadAll(reader)
	_ = reader.Close()
	if operationErr != nil {
		t.Fatal(operationErr)
	}
	if readErr != nil {
		t.Fatal(readErr)
	}
	return content
}

func TestFilePreviewAndLinkJSONContract(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	repository := filepath.Join(root, "repository")
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(filepath.Join(project, "src/main/resources"), 0o755); err != nil {
		t.Fatal(err)
	}
	git := exec.Command("git", "init", "--initial-branch", "main")
	git.Dir = project
	if output, err := git.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, output)
	}
	localFile := filepath.Join(project, "src/main/resources/application-dev.yml")
	if err := os.WriteFile(localFile, []byte("profile: dev\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOCAL_CONFIG_HOME", home)
	if _, err := run([]string{"init", "--default-link-mode", "copy", "--json"}); err != nil {
		t.Fatal(err)
	}
	if _, err := run([]string{"repository", "add", "local-folder", "--id", "personal", "--path", repository, "--json"}); err != nil {
		t.Fatal(err)
	}

	previewOutput := captureStdout(t, func() error {
		_, err := run([]string{
			"preview", "--project", project, "--repository", "personal",
			"--source-path", "project/application-dev.yml",
			"--target", "src/main/resources/application-dev.yml", "--kind", "file", "--json",
		})
		return err
	})
	var preview struct {
		OK    bool   `json:"ok"`
		State string `json:"state"`
		Kind  string `json:"kind"`
	}
	if err := json.Unmarshal(previewOutput, &preview); err != nil {
		t.Fatalf("invalid preview JSON: %v\n%s", err, previewOutput)
	}
	if !preview.OK || preview.State != "local_only" || preview.Kind != "file" {
		t.Fatalf("unexpected preview response: %#v", preview)
	}

	if _, err := run([]string{
		"link", "--project", project, "--repository", "personal",
		"--source-path", "project/application-dev.yml",
		"--target", "src/main/resources/application-dev.yml", "--kind", "file",
		"--mode", "copy", "--initial-strategy", "local", "--json",
	}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(repository, "project/application-dev.yml"))
	if err != nil || string(content) != "profile: dev\n" {
		t.Fatalf("unexpected repository file %q, %v", content, err)
	}
}

func TestInitAndRepositoryJSONContract(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	repositoryPath := filepath.Join(t.TempDir(), "repository")
	t.Setenv("LOCAL_CONFIG_HOME", home)

	initOutput := captureStdout(t, func() error {
		command, err := run([]string{"init", "--json"})
		if command != "init" {
			t.Fatalf("unexpected command %q", command)
		}
		return err
	})
	var initialized struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Home    string `json:"home"`
	}
	if err := json.Unmarshal(initOutput, &initialized); err != nil {
		t.Fatalf("invalid init JSON: %v\n%s", err, initOutput)
	}
	if !initialized.OK || initialized.Command != "init" || initialized.Home != home {
		t.Fatalf("unexpected init response: %#v", initialized)
	}

	addOutput := captureStdout(t, func() error {
		_, err := run([]string{"repository", "add", "local-folder", "--id", "personal", "--path", repositoryPath, "--json"})
		return err
	})
	var added struct {
		OK         bool   `json:"ok"`
		Command    string `json:"command"`
		Repository struct {
			ID                   string `json:"id"`
			CredentialConfigured bool   `json:"credentialConfigured"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(addOutput, &added); err != nil {
		t.Fatalf("invalid repository JSON: %v\n%s", err, addOutput)
	}
	if !added.OK || added.Command != "repository.add" || added.Repository.ID != "personal" || added.Repository.CredentialConfigured {
		t.Fatalf("unexpected repository response: %#v", added)
	}
}

func TestInvalidCommandPreservesMachineErrorContract(t *testing.T) {
	command, err := run([]string{"unknown", "--json"})
	if command != "unknown" || err == nil {
		t.Fatalf("expected command error, got %q, %v", command, err)
	}
	output := captureStdout(t, func() error {
		if code := outputFailure(command, err, true); code != 2 {
			t.Fatalf("unexpected exit code %d", code)
		}
		return nil
	})
	var failure struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(output, &failure); err != nil {
		t.Fatalf("invalid error JSON: %v\n%s", err, output)
	}
	if failure.OK || failure.Command != "unknown" || failure.Error.Code != "invalid_arguments" {
		t.Fatalf("unexpected error response: %#v", failure)
	}
}

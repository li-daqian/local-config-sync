package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/li-daqian/local-config-sync/internal/core"
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
	filesOutput := captureStdout(t, func() error {
		_, err := run([]string{"repository", "files", "personal", "--json"})
		return err
	})
	var filesResponse struct {
		Files json.RawMessage `json:"files"`
	}
	if err := json.Unmarshal(filesOutput, &filesResponse); err != nil {
		t.Fatalf("invalid repository files JSON: %v\n%s", err, filesOutput)
	}
	if string(filesResponse.Files) != "[]" {
		t.Fatalf("empty repository files must be an array, got %s", filesResponse.Files)
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
	if _, err := run([]string{"sync", "--project", project, "--json"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localFile, []byte("profile: local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	remoteFile := filepath.Join(repository, "project/application-dev.yml")
	if err := os.WriteFile(remoteFile, []byte("profile: remote\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statusOutput := captureStdout(t, func() error {
		_, err := run([]string{"status", "--project", project, "--json"})
		return err
	})
	var status struct {
		State string `json:"state"`
		Files []struct {
			MappingID  string `json:"mappingId"`
			RemotePath string `json:"remotePath"`
			Status     string `json:"status"`
		} `json:"files"`
	}
	if err := json.Unmarshal(statusOutput, &status); err != nil || status.State != "conflict" || len(status.Files) != 1 || status.Files[0].Status != "conflict" {
		t.Fatalf("unexpected file status response: %#v, %v", status, err)
	}
	diffOutput := captureStdout(t, func() error {
		_, err := run([]string{
			"diff", "--project", project, "--mapping", status.Files[0].MappingID,
			"--path", status.Files[0].RemotePath, "--json",
		})
		return err
	})
	var diff struct {
		ContentEncoding string `json:"contentEncoding"`
		RemoteRevision  string `json:"remoteRevision"`
		LocalContent    string `json:"localContent"`
		RemoteContent   string `json:"remoteContent"`
	}
	if err := json.Unmarshal(diffOutput, &diff); err != nil || diff.ContentEncoding != "base64" || diff.RemoteRevision == "" || diff.LocalContent == "" || diff.RemoteContent == "" {
		t.Fatalf("unexpected diff response: %#v, %v", diff, err)
	}
	if _, err := run([]string{
		"resolve", "--project", project, "--mapping", status.Files[0].MappingID,
		"--path", status.Files[0].RemotePath, "--expected-revision", diff.RemoteRevision,
		"--strategy", "remote", "--json",
	}); err != nil {
		t.Fatal(err)
	}
	content, err = os.ReadFile(localFile)
	if err != nil || string(content) != "profile: remote\n" {
		t.Fatalf("remote resolution did not update local file: %q, %v", content, err)
	}
}

func TestSensitiveFilePreviewAndLinkJSONContract(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	repository := filepath.Join(root, "repository")
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	git := exec.Command("git", "init", "--initial-branch", "main")
	git.Dir = project
	if output, err := git.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, output)
	}
	if err := os.WriteFile(filepath.Join(project, ".env.local"), []byte("TOKEN=review-me\n"), 0o600); err != nil {
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
			"--source-path", "project/.env.local", "--target", ".env.local",
			"--kind", "file", "--json",
		})
		return err
	})
	var preview struct {
		SensitivePaths []string `json:"sensitivePaths"`
	}
	if err := json.Unmarshal(previewOutput, &preview); err != nil {
		t.Fatalf("invalid sensitive preview JSON: %v\n%s", err, previewOutput)
	}
	if len(preview.SensitivePaths) != 2 {
		t.Fatalf("expected sensitive paths in preview, got %#v", preview)
	}

	linkArguments := []string{
		"link", "--project", project, "--repository", "personal",
		"--source-path", "project/.env.local", "--target", ".env.local",
		"--kind", "file", "--mode", "copy", "--initial-strategy", "local", "--json",
	}
	if _, err := run(linkArguments); err == nil || core.AsError(err).Code != core.ErrUnsafeSecretPattern {
		t.Fatalf("expected unsafe_secret_pattern, got %v", err)
	}
	if _, err := run(append(linkArguments, "--allow-sensitive")); err != nil {
		t.Fatal(err)
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

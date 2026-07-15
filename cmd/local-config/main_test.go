package main

import (
	"encoding/json"
	"io"
	"os"
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

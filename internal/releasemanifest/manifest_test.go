package releasemanifest

import (
	"strings"
	"testing"
)

func TestParseValidManifest(t *testing.T) {
	manifest, err := Parse(strings.NewReader(`
schemaVersion: 1
releaseId: release-2026.07.22.1
artifacts:
  jetbrains:
    version: 0.8.0
`), "release-2026.07.22.1")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	artifact := manifest.Artifacts["jetbrains"]
	if artifact.Version != "0.8.0" {
		t.Fatalf("Version = %q, want %q", artifact.Version, "0.8.0")
	}
	if artifact.Channel != "default" {
		t.Fatalf("Channel = %q, want %q", artifact.Channel, "default")
	}
}

func TestParseRejectsInvalidManifest(t *testing.T) {
	tests := []struct {
		name              string
		yaml              string
		expectedReleaseID string
		wantError         string
	}{
		{
			name: "unknown field",
			yaml: `
schemaVersion: 1
releaseId: release-1
unexpected: true
artifacts:
  jetbrains:
    version: 1.0.0
`,
			expectedReleaseID: "release-1",
			wantError:         "field unexpected not found",
		},
		{
			name: "unsupported schema",
			yaml: `
schemaVersion: 2
releaseId: release-1
artifacts:
  jetbrains:
    version: 1.0.0
`,
			expectedReleaseID: "release-1",
			wantError:         "unsupported schemaVersion 2",
		},
		{
			name: "tag mismatch",
			yaml: `
schemaVersion: 1
releaseId: release-1
artifacts:
  jetbrains:
    version: 1.0.0
`,
			expectedReleaseID: "release-2",
			wantError:         `does not match tag "release-2"`,
		},
		{
			name: "invalid version",
			yaml: `
schemaVersion: 1
releaseId: release-1
artifacts:
  jetbrains:
    version: 01.0.0
`,
			expectedReleaseID: "release-1",
			wantError:         "is not valid SemVer",
		},
		{
			name: "numeric prerelease identifier with leading zero",
			yaml: `
schemaVersion: 1
releaseId: release-1
artifacts:
  jetbrains:
    version: 1.0.0-beta.01
`,
			expectedReleaseID: "release-1",
			wantError:         "is not valid SemVer",
		},
		{
			name: "unsupported artifact",
			yaml: `
schemaVersion: 1
releaseId: release-1
artifacts:
  vscode:
    version: 1.0.0
`,
			expectedReleaseID: "release-1",
			wantError:         `unsupported artifact "vscode"`,
		},
		{
			name: "invalid channel",
			yaml: `
schemaVersion: 1
releaseId: release-1
artifacts:
  jetbrains:
    version: 1.0.0
    channel: "beta; publish"
`,
			expectedReleaseID: "release-1",
			wantError:         "artifacts.jetbrains.channel",
		},
		{
			name: "empty artifacts",
			yaml: `
schemaVersion: 1
releaseId: release-1
artifacts: {}
`,
			expectedReleaseID: "release-1",
			wantError:         "at least one publishable artifact",
		},
		{
			name: "multiple documents",
			yaml: `
schemaVersion: 1
releaseId: release-1
artifacts:
  jetbrains:
    version: 1.0.0
---
schemaVersion: 1
releaseId: release-2
artifacts:
  jetbrains:
    version: 2.0.0
`,
			expectedReleaseID: "release-1",
			wantError:         "multiple YAML documents",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(test.yaml), test.expectedReleaseID)
			if err == nil {
				t.Fatal("Parse() error = nil, want error")
			}
			if !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Parse() error = %q, want substring %q", err, test.wantError)
			}
		})
	}
}

func TestParseAcceptsSemverPrereleaseAndChannel(t *testing.T) {
	manifest, err := Parse(strings.NewReader(`
schemaVersion: 1
releaseId: release-beta.1
artifacts:
  jetbrains:
    version: 1.2.0-beta.1+build.7
    channel: beta
`), "release-beta.1")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	artifact := manifest.Artifacts["jetbrains"]
	if artifact.Channel != "beta" {
		t.Fatalf("Channel = %q, want %q", artifact.Channel, "beta")
	}
}

package releasemanifest

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const CurrentSchemaVersion = 1

var (
	releaseIDPattern        = regexp.MustCompile(`^release-[0-9A-Za-z][0-9A-Za-z._-]*$`)
	semverIdentifierPattern = regexp.MustCompile(`^[0-9A-Za-z-]+$`)
	channelPattern          = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z._-]*$`)
)

var supportedArtifacts = map[string]struct{}{
	"jetbrains": {},
}

// Manifest is the immutable release contract stored at the commit referenced by a release tag.
type Manifest struct {
	SchemaVersion int                 `json:"schemaVersion" yaml:"schemaVersion"`
	ReleaseID     string              `json:"releaseId" yaml:"releaseId"`
	Artifacts     map[string]Artifact `json:"artifacts" yaml:"artifacts"`
}

// Artifact describes one independently versioned artifact in a release batch.
type Artifact struct {
	Version string `json:"version" yaml:"version"`
	Channel string `json:"channel" yaml:"channel"`
}

func Load(path string, expectedReleaseID string) (Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("open release manifest: %w", err)
	}
	defer file.Close()

	return Parse(file, expectedReleaseID)
}

func Parse(reader io.Reader, expectedReleaseID string) (Manifest, error) {
	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)

	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode release manifest: %w", err)
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Manifest{}, fmt.Errorf("decode release manifest: multiple YAML documents are not allowed")
		}
		return Manifest{}, fmt.Errorf("decode release manifest: %w", err)
	}

	if err := validate(manifest, expectedReleaseID); err != nil {
		return Manifest{}, err
	}

	for id, artifact := range manifest.Artifacts {
		if artifact.Channel == "" {
			artifact.Channel = "default"
			manifest.Artifacts[id] = artifact
		}
	}

	return manifest, nil
}

func validate(manifest Manifest, expectedReleaseID string) error {
	if manifest.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf(
			"unsupported schemaVersion %d; expected %d",
			manifest.SchemaVersion,
			CurrentSchemaVersion,
		)
	}
	if !releaseIDPattern.MatchString(manifest.ReleaseID) {
		return fmt.Errorf("releaseId %q must match %s", manifest.ReleaseID, releaseIDPattern)
	}
	if expectedReleaseID == "" {
		return fmt.Errorf("expected release tag is required")
	}
	if manifest.ReleaseID != expectedReleaseID {
		return fmt.Errorf(
			"releaseId %q does not match tag %q",
			manifest.ReleaseID,
			expectedReleaseID,
		)
	}
	if len(manifest.Artifacts) == 0 {
		return fmt.Errorf("artifacts must contain at least one publishable artifact")
	}

	artifactIDs := make([]string, 0, len(manifest.Artifacts))
	for id := range manifest.Artifacts {
		artifactIDs = append(artifactIDs, id)
	}
	sort.Strings(artifactIDs)

	for _, id := range artifactIDs {
		artifact := manifest.Artifacts[id]
		if _, ok := supportedArtifacts[id]; !ok {
			return fmt.Errorf(
				"unsupported artifact %q; supported artifacts: %s",
				id,
				strings.Join(supportedArtifactIDs(), ", "),
			)
		}
		if !isSemver(artifact.Version) {
			return fmt.Errorf("artifacts.%s.version %q is not valid SemVer", id, artifact.Version)
		}
		if artifact.Channel != "" && !channelPattern.MatchString(artifact.Channel) {
			return fmt.Errorf(
				"artifacts.%s.channel %q must match %s",
				id,
				artifact.Channel,
				channelPattern,
			)
		}
	}

	return nil
}

func isSemver(version string) bool {
	if version == "" || strings.Count(version, "+") > 1 {
		return false
	}

	versionAndBuild := strings.SplitN(version, "+", 2)
	if len(versionAndBuild) == 2 && !validSemverIdentifiers(versionAndBuild[1], false) {
		return false
	}

	coreAndPrerelease := strings.SplitN(versionAndBuild[0], "-", 2)
	core := strings.Split(coreAndPrerelease[0], ".")
	if len(core) != 3 {
		return false
	}
	for _, identifier := range core {
		if !isNumericIdentifier(identifier) || hasLeadingZero(identifier) {
			return false
		}
	}

	return len(coreAndPrerelease) != 2 || validSemverIdentifiers(coreAndPrerelease[1], true)
}

func validSemverIdentifiers(value string, rejectNumericLeadingZeros bool) bool {
	identifiers := strings.Split(value, ".")
	for _, identifier := range identifiers {
		if !semverIdentifierPattern.MatchString(identifier) {
			return false
		}
		if rejectNumericLeadingZeros && isNumericIdentifier(identifier) && hasLeadingZero(identifier) {
			return false
		}
	}
	return true
}

func isNumericIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func hasLeadingZero(value string) bool {
	return len(value) > 1 && value[0] == '0'
}

func supportedArtifactIDs() []string {
	ids := make([]string, 0, len(supportedArtifacts))
	for id := range supportedArtifacts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

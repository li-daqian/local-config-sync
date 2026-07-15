package core

import (
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type MappingManager struct{ Storage *Storage }

func (m *MappingManager) List() ([]Mapping, error) {
	file, err := m.Storage.ReadMappings()
	return file.Mappings, err
}

func samePath(a, b string) bool {
	left, _ := filepath.Abs(a)
	right, _ := filepath.Abs(b)
	return filepath.Clean(left) == filepath.Clean(right)
}

func (m *MappingManager) ForProject(projectPath string) ([]Mapping, error) {
	all, err := m.List()
	if err != nil {
		return nil, err
	}
	result := []Mapping{}
	for _, mapping := range all {
		if samePath(mapping.ProjectPath, projectPath) {
			result = append(result, mapping)
		}
	}
	return result, nil
}

func (m *MappingManager) ForRepository(repositoryID string) ([]Mapping, error) {
	all, err := m.List()
	if err != nil {
		return nil, err
	}
	result := []Mapping{}
	for _, mapping := range all {
		if mapping.RepositoryID == repositoryID {
			result = append(result, mapping)
		}
	}
	return result, nil
}

func overlaps(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

var unsafeIDCharacters = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func randomSuffix() string {
	buffer := make([]byte, 4)
	_, _ = rand.Read(buffer)
	return hex.EncodeToString(buffer)
}

type AddMappingInput struct {
	ID, ProjectPath, RepositoryID, SourcePath, TargetPath string
	Mode                                                  LinkMode
}

func (m *MappingManager) Add(input AddMappingInput) (Mapping, error) {
	file, err := m.Storage.ReadMappings()
	if err != nil {
		return Mapping{}, err
	}
	sourcePath, err := SafeRelativePath(input.SourcePath, "sourcePath")
	if err != nil {
		return Mapping{}, err
	}
	targetPath, err := SafeRelativePath(input.TargetPath, "targetPath")
	if err != nil {
		return Mapping{}, err
	}
	for _, existing := range file.Mappings {
		if existing.RepositoryID == input.RepositoryID && overlaps(existing.SourcePath, sourcePath) {
			return Mapping{}, NewError(ErrInvalidArguments, "Mapping source path overlaps an existing mapping", map[string]any{"existingMappingId": existing.ID, "sourcePath": sourcePath})
		}
		if samePath(existing.ProjectPath, input.ProjectPath) && overlaps(existing.TargetPath, targetPath) {
			return Mapping{}, NewError(ErrInvalidArguments, "Mapping target path overlaps an existing mapping", map[string]any{"existingMappingId": existing.ID, "targetPath": targetPath})
		}
	}
	projectPath, _ := filepath.Abs(input.ProjectPath)
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = unsafeIDCharacters.ReplaceAllString(filepath.Base(projectPath), "-") + "-" + randomSuffix()
	}
	for _, existing := range file.Mappings {
		if existing.ID == id {
			return Mapping{}, Invalidf("Mapping id already exists: %s", id)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	mapping := Mapping{ID: id, ProjectPath: projectPath, ProjectName: filepath.Base(projectPath), RepositoryID: input.RepositoryID, SourcePath: sourcePath, TargetPath: targetPath, Mode: input.Mode, CreatedAt: now, UpdatedAt: now}
	file.Mappings = append(file.Mappings, mapping)
	if err := m.Storage.WriteMappings(file); err != nil {
		return Mapping{}, err
	}
	return mapping, nil
}

func (m *MappingManager) Remove(ids []string) ([]Mapping, error) {
	file, err := m.Storage.ReadMappings()
	if err != nil {
		return nil, err
	}
	selected := map[string]bool{}
	for _, id := range ids {
		selected[id] = true
	}
	removed, kept := []Mapping{}, []Mapping{}
	for _, mapping := range file.Mappings {
		if selected[mapping.ID] {
			removed = append(removed, mapping)
		} else {
			kept = append(kept, mapping)
		}
	}
	file.Mappings = kept
	if err := m.Storage.WriteMappings(file); err != nil {
		return nil, err
	}
	return removed, nil
}

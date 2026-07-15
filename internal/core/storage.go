package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var defaultConfig = GlobalConfig{Version: 1, DefaultLinkMode: LinkModeSymlink, AutoSync: AutoSyncConfig{Enabled: false, DebounceSeconds: 60}}

type Storage struct{ Paths AppPaths }

func NewStorage(paths AppPaths) *Storage { return &Storage{Paths: paths} }

func (s *Storage) Initialize(mode LinkMode) (GlobalConfig, error) {
	for _, path := range []string{s.Paths.Home, s.Paths.Workspaces, s.Paths.States, s.Paths.Locks, s.Paths.Logs} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return GlobalConfig{}, WrapError(ErrFilesystemFailed, "Cannot create "+path, err, map[string]any{"path": path})
		}
	}
	config, err := s.ReadConfig()
	if err != nil {
		return GlobalConfig{}, err
	}
	if mode != "" {
		config.DefaultLinkMode = mode
	}
	repositories, err := s.ReadRepositories()
	if err != nil {
		return GlobalConfig{}, err
	}
	mappings, err := s.ReadMappings()
	if err != nil {
		return GlobalConfig{}, err
	}
	if err := s.WriteConfig(config); err != nil {
		return GlobalConfig{}, err
	}
	if err := s.WriteRepositories(repositories); err != nil {
		return GlobalConfig{}, err
	}
	if err := s.WriteMappings(mappings); err != nil {
		return GlobalConfig{}, err
	}
	return config, nil
}

func readYAML[T any](path string, fallback T) (T, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return fallback, nil
	}
	if err != nil {
		return fallback, WrapError(ErrFilesystemFailed, "Cannot read "+path, err, map[string]any{"path": path})
	}
	var value T
	if err := yaml.Unmarshal(content, &value); err != nil {
		return fallback, WrapError(ErrFilesystemFailed, "Cannot parse "+path, err, map[string]any{"path": path})
	}
	return value, nil
}

func atomicWrite(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary := fmt.Sprintf("%s.%d.tmp", path, os.Getpid())
	if err := os.WriteFile(temporary, content, 0o600); err != nil {
		return err
	}
	if err := atomicReplace(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func writeYAML(path string, value any) error {
	content, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	if err := atomicWrite(path, content); err != nil {
		return WrapError(ErrFilesystemFailed, "Cannot write "+path, err, map[string]any{"path": path})
	}
	return nil
}

func (s *Storage) ReadConfig() (GlobalConfig, error) { return readYAML(s.Paths.Config, defaultConfig) }
func (s *Storage) ReadRepositories() (RepositoryRegistryFile, error) {
	return readYAML(s.Paths.Repositories, RepositoryRegistryFile{Version: 1, Repositories: []Repository{}})
}
func (s *Storage) ReadMappings() (MappingRegistryFile, error) {
	return readYAML(s.Paths.Mappings, MappingRegistryFile{Version: 1, Mappings: []Mapping{}})
}
func (s *Storage) WriteConfig(value GlobalConfig) error { return writeYAML(s.Paths.Config, value) }
func (s *Storage) WriteRepositories(value RepositoryRegistryFile) error {
	return writeYAML(s.Paths.Repositories, value)
}
func (s *Storage) WriteMappings(value MappingRegistryFile) error {
	return writeYAML(s.Paths.Mappings, value)
}

func (s *Storage) ReadState(repositoryID string) (RepositoryState, error) {
	path := filepath.Join(s.Paths.States, repositoryID+".json")
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return RepositoryState{Version: 1, RepositoryID: repositoryID, Files: map[string]FileSnapshot{}}, nil
	}
	if err != nil {
		return RepositoryState{}, WrapError(ErrFilesystemFailed, "Cannot read repository state for "+repositoryID, err, map[string]any{"repositoryId": repositoryID})
	}
	var state RepositoryState
	if err := json.Unmarshal(content, &state); err != nil {
		return RepositoryState{}, WrapError(ErrFilesystemFailed, "Cannot parse repository state for "+repositoryID, err, map[string]any{"repositoryId": repositoryID})
	}
	if state.Files == nil {
		state.Files = map[string]FileSnapshot{}
	}
	return state, nil
}

func (s *Storage) WriteState(value RepositoryState) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return atomicWrite(filepath.Join(s.Paths.States, value.RepositoryID+".json"), content)
}

func (s *Storage) RemoveState(repositoryID string) error {
	err := os.Remove(filepath.Join(s.Paths.States, repositoryID+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

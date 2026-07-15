package core

import (
	"path/filepath"
	"regexp"
	"strings"
)

var repositoryIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

type RepositoryRegistry struct {
	Storage *Storage
	Paths   AppPaths
	git     GitDriver
	local   LocalFolderDriver
}

func NewRepositoryRegistry(storage *Storage, paths AppPaths) *RepositoryRegistry {
	return &RepositoryRegistry{Storage: storage, Paths: paths}
}

func validateRepository(repository Repository) (Repository, error) {
	if !repositoryIDPattern.MatchString(repository.ID) || repository.Name == "" || repository.WorkspacePath == "" {
		return Repository{}, NewError(ErrNotConfigured, "Invalid repository registry entry", nil)
	}
	switch repository.Type {
	case "git":
		if repository.Options.RemoteURL == "" || repository.Options.Branch == "" {
			return Repository{}, NewError(ErrNotConfigured, "Invalid Git repository options: "+repository.ID, nil)
		}
	case "local-folder":
		if repository.Options.Path == "" {
			return Repository{}, NewError(ErrNotConfigured, "Invalid local-folder repository options: "+repository.ID, nil)
		}
	default:
		return Repository{}, NewError(ErrUnsupportedCapability, "Unsupported repository type: "+repository.Type, nil)
	}
	return repository, nil
}

func (r *RepositoryRegistry) List() ([]Repository, error) {
	file, err := r.Storage.ReadRepositories()
	if err != nil {
		return nil, err
	}
	result := make([]Repository, 0, len(file.Repositories))
	for _, repository := range file.Repositories {
		valid, err := validateRepository(repository)
		if err != nil {
			return nil, err
		}
		result = append(result, valid)
	}
	return result, nil
}

func (r *RepositoryRegistry) Get(id string) (Repository, error) {
	all, err := r.List()
	if err != nil {
		return Repository{}, err
	}
	for _, repository := range all {
		if repository.ID == id {
			return repository, nil
		}
	}
	return Repository{}, NewError(ErrRepositoryNotFound, "Repository not found: "+id, map[string]any{"repositoryId": id})
}

func validateRepositoryID(id string) error {
	if !repositoryIDPattern.MatchString(id) {
		return NewError(ErrInvalidArguments, "Repository id must contain only lowercase letters, numbers, dots, underscores, and dashes", map[string]any{"id": id})
	}
	return nil
}

func (r *RepositoryRegistry) AddGit(id, name, remoteURL, branch string) (Repository, error) {
	if err := validateRepositoryID(id); err != nil {
		return Repository{}, err
	}
	file, err := r.Storage.ReadRepositories()
	if err != nil {
		return Repository{}, err
	}
	for _, repository := range file.Repositories {
		if repository.ID == id {
			return Repository{}, Invalidf("Repository id already exists: %s", id)
		}
	}
	remoteURL = strings.TrimSpace(remoteURL)
	if remoteURL == "" {
		return Repository{}, Invalidf("Git remote URL is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = id
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "main"
	}
	repository := Repository{ID: id, Name: name, Type: "git", WorkspacePath: filepath.Join(r.Paths.Workspaces, id), Options: RepositoryOptions{RemoteURL: remoteURL, Branch: branch}}
	if err := r.git.Prepare(repository); err != nil {
		return Repository{}, err
	}
	file.Repositories = append(file.Repositories, repository)
	if err := r.Storage.WriteRepositories(file); err != nil {
		return Repository{}, err
	}
	return repository, nil
}

func (r *RepositoryRegistry) AddLocalFolder(id, name, path string) (Repository, error) {
	if err := validateRepositoryID(id); err != nil {
		return Repository{}, err
	}
	file, err := r.Storage.ReadRepositories()
	if err != nil {
		return Repository{}, err
	}
	for _, repository := range file.Repositories {
		if repository.ID == id {
			return Repository{}, Invalidf("Repository id already exists: %s", id)
		}
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return Repository{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = id
	}
	repository := Repository{ID: id, Name: name, Type: "local-folder", WorkspacePath: path, Options: RepositoryOptions{Path: path}}
	if err := r.local.Prepare(repository); err != nil {
		return Repository{}, err
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		repository.WorkspacePath = real
		repository.Options.Path = real
	}
	file.Repositories = append(file.Repositories, repository)
	if err := r.Storage.WriteRepositories(file); err != nil {
		return Repository{}, err
	}
	return repository, nil
}

func (r *RepositoryRegistry) Driver(repository Repository) RepositoryDriver {
	if repository.Type == "git" {
		return &r.git
	}
	return &r.local
}

func (r *RepositoryRegistry) Remove(id string) (Repository, error) {
	mappings, err := r.Storage.ReadMappings()
	if err != nil {
		return Repository{}, err
	}
	ids := []string{}
	for _, mapping := range mappings.Mappings {
		if mapping.RepositoryID == id {
			ids = append(ids, mapping.ID)
		}
	}
	if len(ids) > 0 {
		return Repository{}, NewError(ErrInvalidArguments, "Repository is still referenced by mappings", map[string]any{"repositoryId": id, "mappingIds": ids})
	}
	file, err := r.Storage.ReadRepositories()
	if err != nil {
		return Repository{}, err
	}
	for index, repository := range file.Repositories {
		if repository.ID == id {
			file.Repositories = append(file.Repositories[:index], file.Repositories[index+1:]...)
			if err := r.Storage.WriteRepositories(file); err != nil {
				return Repository{}, err
			}
			_ = r.Storage.RemoveState(id)
			return repository, nil
		}
	}
	return Repository{}, NewError(ErrRepositoryNotFound, "Repository not found: "+id, nil)
}

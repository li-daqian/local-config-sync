package core

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	Paths        AppPaths
	Storage      *Storage
	Repositories *RepositoryRegistry
	Mappings     *MappingManager
}

func NewService(home string) *Service {
	paths := GetAppPaths(home)
	storage := NewStorage(paths)
	return &Service{Paths: paths, Storage: storage, Repositories: NewRepositoryRegistry(storage, paths), Mappings: &MappingManager{Storage: storage}}
}

func (s *Service) Init(mode LinkMode) (GlobalConfig, error) {
	if mode == "" {
		mode = LinkModeSymlink
	}
	return s.Storage.Initialize(mode)
}

type LinkInput struct {
	Project, RepositoryID, SourcePath, TargetPath string
	Mode                                          LinkMode
	ID                                            string
}

func (s *Service) Link(input LinkInput) (Mapping, error) {
	if _, err := s.Storage.Initialize(""); err != nil {
		return Mapping{}, err
	}
	project, err := ResolveProject(input.Project)
	if err != nil {
		return Mapping{}, err
	}
	repository, err := s.Repositories.Get(input.RepositoryID)
	if err != nil {
		return Mapping{}, err
	}
	if err := s.Repositories.Driver(repository).Prepare(repository); err != nil {
		return Mapping{}, err
	}
	source, err := ResolveInside(repository.WorkspacePath, input.SourcePath)
	if err != nil {
		return Mapping{}, err
	}
	target, err := ResolveInside(project.Root, input.TargetPath)
	if err != nil {
		return Mapping{}, err
	}
	mode := input.Mode
	if mode == "" {
		config, err := s.Storage.ReadConfig()
		if err != nil {
			return Mapping{}, err
		}
		mode = config.DefaultLinkMode
	}
	if err := Materialize(source, target, mode, false); err != nil {
		return Mapping{}, err
	}
	if err := AddExclude(project.ExcludePath, input.TargetPath); err != nil {
		_ = os.RemoveAll(target)
		return Mapping{}, err
	}
	mapping, err := s.Mappings.Add(AddMappingInput{ID: input.ID, ProjectPath: project.Root, RepositoryID: input.RepositoryID, SourcePath: input.SourcePath, TargetPath: input.TargetPath, Mode: mode})
	if err != nil {
		_ = os.RemoveAll(target)
		return Mapping{}, err
	}
	return mapping, nil
}

func (s *Service) Unlink(projectPath string, keepFiles, keepExclude bool) ([]Mapping, error) {
	project, err := ResolveProject(projectPath)
	if err != nil {
		return nil, err
	}
	mappings, err := s.Mappings.ForProject(project.Root)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(mappings))
	for _, mapping := range mappings {
		target, err := ResolveInside(project.Root, mapping.TargetPath)
		if err != nil {
			return nil, err
		}
		if err := RemoveMaterialized(target, mapping.Mode, keepFiles); err != nil {
			return nil, err
		}
		if !keepExclude {
			if err := RemoveExclude(project.ExcludePath, mapping.TargetPath); err != nil {
				return nil, err
			}
		}
		ids = append(ids, mapping.ID)
	}
	return s.Mappings.Remove(ids)
}

func (s *Service) Authenticate(repositoryID, method string) ([]DiagnosticCheck, error) {
	repository, err := s.Repositories.Get(repositoryID)
	if err != nil {
		return nil, err
	}
	if repository.Type != "git" {
		return nil, NewError(ErrInvalidArguments, "Authentication is only applicable to Git repositories", map[string]any{"repositoryId": repositoryID})
	}
	return AuthenticateGit(repository, method)
}

func (s *Service) AuthenticateURL(remoteURL, method string) ([]DiagnosticCheck, error) {
	repository := Repository{ID: "authentication-check", Name: "Authentication Check", Type: "git", WorkspacePath: s.Paths.Workspaces, Options: RepositoryOptions{RemoteURL: remoteURL, Branch: "main"}}
	return AuthenticateGit(repository, method)
}

type SyncOptions struct {
	Project, RepositoryID string
	AllowSensitive        bool
}
type repositoryScope struct {
	Repository Repository
	Mappings   []Mapping
}

func (s *Service) resolveScope(options SyncOptions) ([]repositoryScope, error) {
	if (options.Project == "") == (options.RepositoryID == "") {
		return nil, Invalidf("Specify exactly one of --project or --repository")
	}
	if options.RepositoryID != "" {
		repository, err := s.Repositories.Get(options.RepositoryID)
		if err != nil {
			return nil, err
		}
		mappings, err := s.Mappings.ForRepository(repository.ID)
		if err != nil {
			return nil, err
		}
		if len(mappings) == 0 {
			return nil, NewError(ErrNotConfigured, "Repository has no mappings", map[string]any{"repositoryId": repository.ID})
		}
		return []repositoryScope{{Repository: repository, Mappings: mappings}}, nil
	}
	project, err := ResolveProject(options.Project)
	if err != nil {
		return nil, err
	}
	mappings, err := s.Mappings.ForProject(project.Root)
	if err != nil {
		return nil, err
	}
	if len(mappings) == 0 {
		return nil, NewError(ErrNotConfigured, "Project has no mappings", map[string]any{"projectPath": project.Root})
	}
	grouped := map[string][]Mapping{}
	order := []string{}
	for _, mapping := range mappings {
		if _, ok := grouped[mapping.RepositoryID]; !ok {
			order = append(order, mapping.RepositoryID)
		}
		grouped[mapping.RepositoryID] = append(grouped[mapping.RepositoryID], mapping)
	}
	result := []repositoryScope{}
	for _, id := range order {
		repository, err := s.Repositories.Get(id)
		if err != nil {
			return nil, err
		}
		result = append(result, repositoryScope{Repository: repository, Mappings: grouped[id]})
	}
	return result, nil
}

func snapshotChanged(left map[string]FileSnapshot, right map[string]FileSnapshot, prefix string) bool {
	paths := map[string]bool{}
	for path := range left {
		paths[path] = true
	}
	for path := range right {
		paths[path] = true
	}
	for path := range paths {
		if path != prefix && !strings.HasPrefix(path, strings.TrimSuffix(prefix, "/")+"/") {
			continue
		}
		a, aOK := left[path]
		b, bOK := right[path]
		if !SnapshotsEqual(a, aOK, b, bOK) {
			return true
		}
	}
	return false
}

func (s *Service) reconcileCopiesFromWorkspace(mappings []Mapping, state RepositoryState) error {
	for _, mapping := range mappings {
		if mapping.Mode != LinkModeCopy {
			continue
		}
		repository, err := s.Repositories.Get(mapping.RepositoryID)
		if err != nil {
			return err
		}
		source, err := ResolveInside(repository.WorkspacePath, mapping.SourcePath)
		if err != nil {
			return err
		}
		target, err := ResolveInside(mapping.ProjectPath, mapping.TargetPath)
		if err != nil {
			return err
		}
		targetSnapshot, err := SnapshotDirectory(target, mapping.SourcePath)
		if err != nil {
			return err
		}
		if snapshotChanged(targetSnapshot, state.Files, mapping.SourcePath) {
			return NewError(ErrConflict, "Copy target has local changes while repository changed", map[string]any{"mappingId": mapping.ID})
		}
		if err := Materialize(source, target, LinkModeCopy, true); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) reconcileCopiesToWorkspace(mappings []Mapping, state RepositoryState) error {
	for _, mapping := range mappings {
		if mapping.Mode != LinkModeCopy {
			continue
		}
		repository, err := s.Repositories.Get(mapping.RepositoryID)
		if err != nil {
			return err
		}
		source, err := ResolveInside(repository.WorkspacePath, mapping.SourcePath)
		if err != nil {
			return err
		}
		target, err := ResolveInside(mapping.ProjectPath, mapping.TargetPath)
		if err != nil {
			return err
		}
		workspaceSnapshot, err := SnapshotDirectory(source, mapping.SourcePath)
		if err != nil {
			return err
		}
		targetSnapshot, err := SnapshotDirectory(target, mapping.SourcePath)
		if err != nil {
			return err
		}
		externallyChanged := snapshotChanged(workspaceSnapshot, state.Files, mapping.SourcePath)
		locallyChanged := snapshotChanged(targetSnapshot, state.Files, mapping.SourcePath)
		if externallyChanged && locallyChanged && !mapsEqual(workspaceSnapshot, targetSnapshot) {
			return NewError(ErrConflict, "Both copy target and repository changed", map[string]any{"mappingId": mapping.ID})
		}
		if locallyChanged {
			if err := Materialize(target, source, LinkModeCopy, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) updateState(repository Repository, mappings []Mapping, state RepositoryState) (RepositoryState, error) {
	if state.Files == nil {
		state.Files = map[string]FileSnapshot{}
	}
	for _, mapping := range mappings {
		for path := range state.Files {
			if path == mapping.SourcePath || strings.HasPrefix(path, mapping.SourcePath+"/") {
				delete(state.Files, path)
			}
		}
		source, err := ResolveInside(repository.WorkspacePath, mapping.SourcePath)
		if err != nil {
			return state, err
		}
		snapshot, err := SnapshotDirectory(source, mapping.SourcePath)
		if err != nil {
			return state, err
		}
		for path, file := range snapshot {
			state.Files[path] = file
		}
	}
	state.Version = 1
	state.RepositoryID = repository.ID
	state.LastSyncTime = time.Now().UTC().Format(time.RFC3339Nano)
	state.LastError = nil
	if err := s.Storage.WriteState(state); err != nil {
		return state, err
	}
	return state, nil
}

func uniqueProjectNames(mappings []Mapping) string {
	seen := map[string]bool{}
	names := []string{}
	for _, mapping := range mappings {
		if !seen[mapping.ProjectName] {
			seen[mapping.ProjectName] = true
			names = append(names, mapping.ProjectName)
		}
	}
	return strings.Join(names, ",")
}

func (s *Service) Sync(options SyncOptions, operation string) ([]SyncResult, error) {
	groups, err := s.resolveScope(options)
	if err != nil {
		return nil, err
	}
	results := []SyncResult{}
	for _, group := range groups {
		var result SyncResult
		err := WithRepositoryLock(filepath.Join(s.Paths.Locks, group.Repository.ID+".lock"), func() error {
			driver := s.Repositories.Driver(group.Repository)
			if err := driver.Prepare(group.Repository); err != nil {
				return err
			}
			state, err := s.Storage.ReadState(group.Repository.ID)
			if err != nil {
				return err
			}
			scopes := make([]string, 0, len(group.Mappings))
			for _, mapping := range group.Mappings {
				scopes = append(scopes, mapping.SourcePath)
			}
			before, err := driver.Inspect(DriverContext{Repository: group.Repository, Scopes: scopes, ExpectedRevision: state.RemoteRevision})
			if err != nil {
				return err
			}
			baseline := state.RemoteRevision
			if (operation == "pull" || operation == "sync") && before.RemoteChanged {
				if baseline != "" && len(before.LocalChanges) > 0 {
					return NewError(ErrConflict, "Local and remote repository changed since the last sync", map[string]any{"repositoryId": group.Repository.ID, "paths": before.LocalChanges})
				}
				pulled, err := driver.Pull(DriverContext{Repository: group.Repository, Scopes: scopes, ExpectedRevision: baseline})
				if err != nil {
					return err
				}
				state.RemoteRevision = pulled.RemoteRevision
				if err := s.reconcileCopiesFromWorkspace(group.Mappings, state); err != nil {
					return err
				}
			}
			if operation == "push" || operation == "sync" {
				if err := s.reconcileCopiesToWorkspace(group.Mappings, state); err != nil {
					return err
				}
				matches, err := ScanSensitive(group.Repository.WorkspacePath, scopes)
				if err != nil {
					return err
				}
				if err := AssertNoSensitive(matches, options.AllowSensitive); err != nil {
					return err
				}
				expected := state.RemoteRevision
				if expected == "" {
					inspected, err := driver.Inspect(DriverContext{Repository: group.Repository, Scopes: scopes})
					if err != nil {
						return err
					}
					expected = inspected.RemoteRevision
				}
				pushed, err := driver.Push(DriverContext{Repository: group.Repository, Scopes: scopes, ExpectedRevision: expected}, "chore("+uniqueProjectNames(group.Mappings)+"): sync local config")
				if err != nil {
					return err
				}
				state.RemoteRevision = pushed.RemoteRevision
			}
			state, err = s.updateState(group.Repository, group.Mappings, state)
			if err != nil {
				return err
			}
			result = SyncResult{RepositoryID: group.Repository.ID, State: "synced", RemoteRevision: state.RemoteRevision, LastSyncTime: state.LastSyncTime}
			return nil
		})
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func (s *Service) Status(projectPath string) (StatusResult, error) {
	project, err := ResolveProject(projectPath)
	if err != nil {
		return StatusResult{}, err
	}
	mappings, err := s.Mappings.ForProject(project.Root)
	if err != nil {
		return StatusResult{}, err
	}
	if len(mappings) == 0 {
		return StatusResult{ProjectPath: project.Root, State: "not_configured", Repositories: []RepositorySummary{}, Mappings: []MappingSummary{}}, nil
	}
	result := StatusResult{ProjectPath: project.Root, State: "synced", Repositories: []RepositorySummary{}, Mappings: []MappingSummary{}}
	seen := map[string]bool{}
	for _, mapping := range mappings {
		if seen[mapping.RepositoryID] {
			continue
		}
		seen[mapping.RepositoryID] = true
		repository, err := s.Repositories.Get(mapping.RepositoryID)
		if err != nil {
			return StatusResult{}, err
		}
		scopes := []string{}
		for _, item := range mappings {
			if item.RepositoryID == mapping.RepositoryID {
				scopes = append(scopes, item.SourcePath)
			}
		}
		status, err := s.Repositories.Driver(repository).Inspect(DriverContext{Repository: repository, Scopes: scopes})
		if err != nil {
			return StatusResult{}, err
		}
		if status.State != "synced" {
			result.State = "pending"
		}
		result.Repositories = append(result.Repositories, RepositorySummary{ID: repository.ID, Name: repository.Name, Type: repository.Type, State: status.State, WorkspacePath: repository.WorkspacePath, RemoteRevision: status.RemoteRevision, Capabilities: status.Capabilities})
	}
	for _, mapping := range mappings {
		target, err := ResolveInside(project.Root, mapping.TargetPath)
		if err != nil {
			return StatusResult{}, err
		}
		files, err := ListFiles(target)
		if err != nil {
			return StatusResult{}, err
		}
		for index := range files {
			files[index] = strings.TrimSuffix(mapping.TargetPath, "/") + "/" + files[index]
		}
		excluded, err := HasExclude(project.ExcludePath, mapping.TargetPath)
		if err != nil {
			return StatusResult{}, err
		}
		result.Mappings = append(result.Mappings, MappingSummary{ID: mapping.ID, RepositoryID: mapping.RepositoryID, SourcePath: mapping.SourcePath, TargetPath: mapping.TargetPath, Mode: mapping.Mode, MappedFiles: files, ExcludeConfigured: excluded})
	}
	lastTimes := []string{}
	for _, repository := range result.Repositories {
		state, err := s.Storage.ReadState(repository.ID)
		if err != nil {
			return StatusResult{}, err
		}
		if state.LastSyncTime != "" {
			lastTimes = append(lastTimes, state.LastSyncTime)
		}
	}
	sort.Strings(lastTimes)
	if len(lastTimes) > 0 {
		result.LastSyncTime = lastTimes[len(lastTimes)-1]
	}
	return result, nil
}

func (s *Service) Doctor(projectPath, repositoryID string) (DiagnosticResult, error) {
	if repositoryID != "" {
		repository, err := s.Repositories.Get(repositoryID)
		if err != nil {
			return DiagnosticResult{}, err
		}
		return s.Repositories.Driver(repository).Doctor(repository)
	}
	checks := []DiagnosticCheck{}
	if projectPath != "" {
		project, err := ResolveProject(projectPath)
		if err != nil {
			return DiagnosticResult{}, err
		}
		checks = append(checks, DiagnosticCheck{Name: "git-project", OK: true, Message: "Git project found at " + project.Root})
		mappings, err := s.Mappings.ForProject(project.Root)
		if err != nil {
			return DiagnosticResult{}, err
		}
		mappingCheck := DiagnosticCheck{Name: "mappings", OK: len(mappings) > 0}
		if mappingCheck.OK {
			mappingCheck.Message = strconv.Itoa(len(mappings)) + " mapping(s) configured"
		} else {
			mappingCheck.Message = "No mappings configured"
			mappingCheck.Remediation = "Run local-config link"
		}
		checks = append(checks, mappingCheck)
		seen := map[string]bool{}
		for _, mapping := range mappings {
			if seen[mapping.RepositoryID] {
				continue
			}
			seen[mapping.RepositoryID] = true
			repository, err := s.Repositories.Get(mapping.RepositoryID)
			if err != nil {
				return DiagnosticResult{}, err
			}
			result, err := s.Repositories.Driver(repository).Doctor(repository)
			if err != nil {
				return DiagnosticResult{}, err
			}
			for _, check := range result.Checks {
				check.Name = mapping.RepositoryID + ":" + check.Name
				checks = append(checks, check)
			}
		}
	} else {
		repositories, err := s.Repositories.List()
		if err != nil {
			return DiagnosticResult{}, err
		}
		checks = append(checks, DiagnosticCheck{Name: "configuration", OK: len(repositories) > 0, Message: strconv.Itoa(len(repositories)) + " repository/repositories configured"})
	}
	ok := true
	for _, check := range checks {
		if !check.OK {
			ok = false
		}
	}
	return DiagnosticResult{OK: ok, Checks: checks}, nil
}

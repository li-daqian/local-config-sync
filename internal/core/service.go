package core

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (s *Service) RepositoryFiles(repositoryID string) (RepositoryFileList, error) {
	repository, err := s.Repositories.Get(repositoryID)
	if err != nil {
		return RepositoryFileList{}, err
	}
	driver := s.Repositories.Driver(repository)
	if err := WithRepositoryLock(filepath.Join(s.Paths.Locks, repository.ID+".lock"), func() error {
		if err := driver.Prepare(repository); err != nil {
			return err
		}
		status, err := driver.Inspect(DriverContext{Repository: repository})
		if err != nil {
			return err
		}
		if status.RemoteChanged && len(status.LocalChanges) > 0 {
			return NewError(ErrConflict, "Cannot refresh repository files while the managed workspace has local changes", map[string]any{"repositoryId": repository.ID, "paths": status.LocalChanges})
		}
		if status.RemoteChanged {
			_, err = driver.Pull(DriverContext{Repository: repository})
		}
		return err
	}); err != nil {
		return RepositoryFileList{}, err
	}
	files, err := ListFiles(repository.WorkspacePath)
	if err != nil {
		return RepositoryFileList{}, err
	}
	return RepositoryFileList{RepositoryID: repositoryID, Files: files}, nil
}

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
	Kind                                          MappingKind
	InitialStrategy                               InitialStrategy
	ID                                            string
	AllowSensitive                                bool
}

func normalizeMappingKind(kind MappingKind) (MappingKind, error) {
	if kind == "" {
		return MappingKindDirectory, nil
	}
	if kind != MappingKindDirectory && kind != MappingKindFile {
		return "", Invalidf("mapping kind must be file or directory")
	}
	return kind, nil
}

func fileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, NewError(ErrFilesystemFailed, "File mapping path is not a regular file: "+path, map[string]any{"path": path})
	}
	return true, nil
}

func ensureTargetIsUntracked(projectRoot, targetPath string) error {
	result, err := RunProcess("git", []string{"ls-files", "--error-unmatch", "--", targetPath}, projectRoot, nil, true)
	if err != nil {
		return err
	}
	if result.ExitCode == 0 {
		return NewError(ErrInvalidArguments, "Mapped project file is already tracked by Git; remove it from the business repository index before setup", map[string]any{"targetPath": targetPath})
	}
	return nil
}

func previewFileMapping(source, target, sourcePath, targetPath string) (MappingPreview, error) {
	sourceExists, err := fileExists(source)
	if err != nil {
		return MappingPreview{}, err
	}
	targetExists, err := fileExists(target)
	if err != nil {
		return MappingPreview{}, err
	}
	preview := MappingPreview{
		Kind: MappingKindFile, SourcePath: sourcePath, TargetPath: targetPath,
		SourceAbsolutePath: source, TargetAbsolutePath: target,
		SourceExists: sourceExists, TargetExists: targetExists,
		SensitivePaths: SensitivePaths(sourcePath, targetPath),
	}
	switch {
	case sourceExists && !targetExists:
		preview.State = "remote_only"
	case !sourceExists && targetExists:
		preview.State = "local_only"
	case !sourceExists && !targetExists:
		preview.State = "missing_both"
	default:
		sourceSnapshot, err := SnapshotPath(source, sourcePath, MappingKindFile)
		if err != nil {
			return MappingPreview{}, err
		}
		targetSnapshot, err := SnapshotPath(target, sourcePath, MappingKindFile)
		if err != nil {
			return MappingPreview{}, err
		}
		if mapsEqual(sourceSnapshot, targetSnapshot) {
			preview.State = "identical"
		} else {
			preview.State = "conflict"
		}
	}
	return preview, nil
}

func (s *Service) PreviewLink(input LinkInput) (MappingPreview, error) {
	project, err := ResolveProject(input.Project)
	if err != nil {
		return MappingPreview{}, err
	}
	repository, err := s.Repositories.Get(input.RepositoryID)
	if err != nil {
		return MappingPreview{}, err
	}
	if err := s.Repositories.Driver(repository).Prepare(repository); err != nil {
		return MappingPreview{}, err
	}
	kind, err := normalizeMappingKind(input.Kind)
	if err != nil {
		return MappingPreview{}, err
	}
	if kind != MappingKindFile {
		return MappingPreview{}, Invalidf("link preview currently supports file mappings")
	}
	source, err := ResolveInside(repository.WorkspacePath, input.SourcePath)
	if err != nil {
		return MappingPreview{}, err
	}
	target, err := ResolveInside(project.Root, input.TargetPath)
	if err != nil {
		return MappingPreview{}, err
	}
	return previewFileMapping(source, target, input.SourcePath, input.TargetPath)
}

func initializeFileMapping(source, target string, mode LinkMode, strategy InitialStrategy, preview MappingPreview) error {
	if strategy == "" {
		strategy = InitialStrategyAuto
	}
	if strategy != InitialStrategyAuto && strategy != InitialStrategyLocal && strategy != InitialStrategyRemote {
		return Invalidf("initial strategy must be auto, local, or remote")
	}
	if preview.State == "missing_both" {
		return NewError(ErrInvalidArguments, "Neither the local nor repository file exists", map[string]any{"sourcePath": preview.SourcePath, "targetPath": preview.TargetPath})
	}
	if preview.State == "conflict" && strategy == InitialStrategyAuto {
		return NewError(ErrConflict, "Local and repository files differ; choose an initial strategy after reviewing the diff", map[string]any{"sourcePath": preview.SourcePath, "targetPath": preview.TargetPath, "sourceAbsolutePath": preview.SourceAbsolutePath, "targetAbsolutePath": preview.TargetAbsolutePath})
	}
	winner := strategy
	if winner == InitialStrategyAuto {
		if preview.State == "local_only" {
			winner = InitialStrategyLocal
		} else {
			winner = InitialStrategyRemote
		}
	}
	if winner == InitialStrategyLocal && !preview.TargetExists {
		return Invalidf("local initial strategy requires an existing local file")
	}
	if winner == InitialStrategyRemote && !preview.SourceExists {
		return Invalidf("remote initial strategy requires an existing repository file")
	}
	if winner == InitialStrategyLocal {
		if err := CopyFileReplace(target, source); err != nil {
			return err
		}
	}
	if mode == LinkModeSymlink {
		return MaterializeFile(source, target, mode, preview.TargetExists)
	}
	if winner == InitialStrategyRemote {
		return MaterializeFile(source, target, mode, preview.TargetExists)
	}
	return nil
}

type fileBackup struct {
	path, temporary string
	existed         bool
}

func backupFile(path string) (fileBackup, error) {
	exists, err := fileExists(path)
	if err != nil {
		return fileBackup{}, err
	}
	backup := fileBackup{path: path, existed: exists}
	if !exists {
		return backup, nil
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".local-config-backup-*")
	if err != nil {
		return fileBackup{}, err
	}
	backup.temporary = temporary.Name()
	if err := temporary.Close(); err != nil {
		_ = os.Remove(backup.temporary)
		return fileBackup{}, err
	}
	if err := CopyFileReplace(path, backup.temporary); err != nil {
		_ = os.Remove(backup.temporary)
		return fileBackup{}, err
	}
	return backup, nil
}

func (backup fileBackup) restore() {
	if backup.existed {
		_ = CopyFileReplace(backup.temporary, backup.path)
	} else {
		_ = os.Remove(backup.path)
	}
	backup.cleanup()
}

func (backup fileBackup) cleanup() {
	if backup.temporary != "" {
		_ = os.Remove(backup.temporary)
	}
}

func (s *Service) linkFileMapping(project ResolvedProject, input LinkInput, source, target string, mode LinkMode) (Mapping, error) {
	if err := ensureTargetIsUntracked(project.Root, input.TargetPath); err != nil {
		return Mapping{}, err
	}
	mappingInput := AddMappingInput{ID: input.ID, ProjectPath: project.Root, RepositoryID: input.RepositoryID, SourcePath: input.SourcePath, TargetPath: input.TargetPath, Mode: mode, Kind: MappingKindFile}
	if err := s.Mappings.Validate(mappingInput); err != nil {
		return Mapping{}, err
	}
	preview, err := previewFileMapping(source, target, input.SourcePath, input.TargetPath)
	if err != nil {
		return Mapping{}, err
	}
	if err := AssertNoSensitive(preview.SensitivePaths, input.AllowSensitive); err != nil {
		return Mapping{}, err
	}
	sourceBackup, err := backupFile(source)
	if err != nil {
		return Mapping{}, err
	}
	targetBackup, err := backupFile(target)
	if err != nil {
		sourceBackup.cleanup()
		return Mapping{}, err
	}
	rollback := func() {
		sourceBackup.restore()
		targetBackup.restore()
	}
	if err := initializeFileMapping(source, target, mode, input.InitialStrategy, preview); err != nil {
		rollback()
		return Mapping{}, err
	}
	if err := AddExclude(project.ExcludePath, input.TargetPath); err != nil {
		rollback()
		return Mapping{}, err
	}
	mapping, err := s.Mappings.Add(mappingInput)
	if err != nil {
		_ = RemoveExclude(project.ExcludePath, input.TargetPath)
		rollback()
		return Mapping{}, err
	}
	sourceBackup.cleanup()
	targetBackup.cleanup()
	return mapping, nil
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
	kind, err := normalizeMappingKind(input.Kind)
	if err != nil {
		return Mapping{}, err
	}
	if kind == MappingKindFile {
		return s.linkFileMapping(project, input, source, target, mode)
	}
	matches, err := ScanSensitive(repository.WorkspacePath, []string{input.SourcePath})
	if err != nil {
		return Mapping{}, err
	}
	localMatches, err := ScanSensitive(project.Root, []string{input.TargetPath})
	if err != nil {
		return Mapping{}, err
	}
	if err := AssertNoSensitive(append(matches, localMatches...), input.AllowSensitive); err != nil {
		return Mapping{}, err
	}
	if err := Materialize(source, target, mode, false); err != nil {
		return Mapping{}, err
	}
	if err := AddExclude(project.ExcludePath, input.TargetPath); err != nil {
		_ = os.RemoveAll(target)
		return Mapping{}, err
	}
	mapping, err := s.Mappings.Add(AddMappingInput{ID: input.ID, ProjectPath: project.Root, RepositoryID: input.RepositoryID, SourcePath: input.SourcePath, TargetPath: input.TargetPath, Mode: mode, Kind: kind})
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
		if err := RemoveMappedPath(target, mapping.Mode, mapping.Kind, keepFiles); err != nil {
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
		workspaceSnapshot, err := SnapshotPath(source, mapping.SourcePath, mapping.Kind)
		if err != nil {
			return err
		}
		if !snapshotChanged(workspaceSnapshot, state.Files, mapping.SourcePath) {
			continue
		}
		targetSnapshot, err := SnapshotPath(target, mapping.SourcePath, mapping.Kind)
		if err != nil {
			return err
		}
		if snapshotChanged(targetSnapshot, state.Files, mapping.SourcePath) && !mapsEqual(workspaceSnapshot, targetSnapshot) {
			return NewError(ErrConflict, "Copy target has local changes while repository changed", map[string]any{"mappingId": mapping.ID})
		}
		if mapping.Kind == MappingKindFile {
			if err := CopyFileReplace(source, target); err != nil {
				return err
			}
		} else if err := Materialize(source, target, LinkModeCopy, true); err != nil {
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
		workspaceSnapshot, err := SnapshotPath(source, mapping.SourcePath, mapping.Kind)
		if err != nil {
			return err
		}
		targetSnapshot, err := SnapshotPath(target, mapping.SourcePath, mapping.Kind)
		if err != nil {
			return err
		}
		externallyChanged := snapshotChanged(workspaceSnapshot, state.Files, mapping.SourcePath)
		locallyChanged := snapshotChanged(targetSnapshot, state.Files, mapping.SourcePath)
		if externallyChanged && locallyChanged && !mapsEqual(workspaceSnapshot, targetSnapshot) {
			return NewError(ErrConflict, "Both copy target and repository changed", map[string]any{"mappingId": mapping.ID})
		}
		if locallyChanged {
			if mapping.Kind == MappingKindFile {
				if err := CopyFileReplace(target, source); err != nil {
					return err
				}
			} else if err := Materialize(target, source, LinkModeCopy, true); err != nil {
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
		snapshot, err := SnapshotPath(source, mapping.SourcePath, mapping.Kind)
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

func classifyFile(local FileSnapshot, localOK bool, remote FileSnapshot, remoteOK bool, baseline FileSnapshot, baselineOK bool) string {
	localChanged := !SnapshotsEqual(local, localOK, baseline, baselineOK)
	remoteChanged := !SnapshotsEqual(remote, remoteOK, baseline, baselineOK)
	switch {
	case localChanged && remoteChanged && !SnapshotsEqual(local, localOK, remote, remoteOK):
		return "conflict"
	case localChanged && !remoteChanged:
		return "local_changes"
	case remoteChanged && !localChanged:
		return "remote_changes"
	default:
		return "synced"
	}
}

func mappingLocalPath(mapping Mapping, remotePath string) string {
	if remotePath == mapping.SourcePath {
		return mapping.TargetPath
	}
	relative := strings.TrimPrefix(remotePath, strings.TrimSuffix(mapping.SourcePath, "/")+"/")
	return strings.TrimSuffix(mapping.TargetPath, "/") + "/" + relative
}

func mappingFileStatuses(mapping Mapping, local, remote, baseline map[string]FileSnapshot) []FileStatusSummary {
	paths := map[string]bool{}
	for path := range local {
		paths[path] = true
	}
	for path := range remote {
		if inScope(path, []string{mapping.SourcePath}) {
			paths[path] = true
		}
	}
	for path := range baseline {
		if inScope(path, []string{mapping.SourcePath}) {
			paths[path] = true
		}
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	result := make([]FileStatusSummary, 0, len(ordered))
	for _, path := range ordered {
		localFile, localOK := local[path]
		remoteFile, remoteOK := remote[path]
		baselineFile, baselineOK := baseline[path]
		result = append(result, FileStatusSummary{
			MappingID: mapping.ID, RepositoryID: mapping.RepositoryID,
			LocalPath: mappingLocalPath(mapping, path), RemotePath: path,
			Status:      classifyFile(localFile, localOK, remoteFile, remoteOK, baselineFile, baselineOK),
			LocalExists: localOK, RemoteExists: remoteOK,
		})
	}
	return result
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
			}
			if operation == "pull" || operation == "sync" {
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
		return StatusResult{ProjectPath: project.Root, State: "not_configured", Repositories: []RepositorySummary{}, Mappings: []MappingSummary{}, Files: []FileStatusSummary{}}, nil
	}
	result := StatusResult{ProjectPath: project.Root, State: "synced", Repositories: []RepositorySummary{}, Mappings: []MappingSummary{}, Files: []FileStatusSummary{}}
	seen := map[string]bool{}
	remoteSnapshots := map[string]map[string]FileSnapshot{}
	repositoryStates := map[string]RepositoryState{}
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
		remoteSnapshot, err := s.Repositories.Driver(repository).Snapshot(DriverContext{Repository: repository, Scopes: scopes}, status.RemoteRevision)
		if err != nil {
			return StatusResult{}, err
		}
		state, err := s.Storage.ReadState(repository.ID)
		if err != nil {
			return StatusResult{}, err
		}
		remoteSnapshots[repository.ID] = remoteSnapshot
		repositoryStates[repository.ID] = state
		result.Repositories = append(result.Repositories, RepositorySummary{ID: repository.ID, Name: repository.Name, Type: repository.Type, State: status.State, WorkspacePath: repository.WorkspacePath, RemoteRevision: status.RemoteRevision, Capabilities: status.Capabilities})
	}
	for _, mapping := range mappings {
		target, err := ResolveInside(project.Root, mapping.TargetPath)
		if err != nil {
			return StatusResult{}, err
		}
		snapshotPath := target
		if mapping.Mode == LinkModeSymlink {
			repository, err := s.Repositories.Get(mapping.RepositoryID)
			if err != nil {
				return StatusResult{}, err
			}
			snapshotPath, err = ResolveInside(repository.WorkspacePath, mapping.SourcePath)
			if err != nil {
				return StatusResult{}, err
			}
		}
		files := []string{}
		if mapping.Kind == MappingKindFile {
			if exists, err := fileExists(snapshotPath); err != nil {
				return StatusResult{}, err
			} else if exists {
				files = append(files, mapping.TargetPath)
			}
		} else {
			files, err = ListFiles(snapshotPath)
			if err != nil {
				return StatusResult{}, err
			}
			for index := range files {
				files[index] = strings.TrimSuffix(mapping.TargetPath, "/") + "/" + files[index]
			}
		}
		excluded, err := HasExclude(project.ExcludePath, mapping.TargetPath)
		if err != nil {
			return StatusResult{}, err
		}
		result.Mappings = append(result.Mappings, MappingSummary{ID: mapping.ID, RepositoryID: mapping.RepositoryID, SourcePath: mapping.SourcePath, TargetPath: mapping.TargetPath, Mode: mapping.Mode, Kind: mapping.Kind, MappedFiles: files, ExcludeConfigured: excluded})
		localSnapshot, err := SnapshotPath(snapshotPath, mapping.SourcePath, mapping.Kind)
		if err != nil {
			return StatusResult{}, err
		}
		fileStatuses := mappingFileStatuses(mapping, localSnapshot, remoteSnapshots[mapping.RepositoryID], repositoryStates[mapping.RepositoryID].Files)
		for _, file := range fileStatuses {
			if file.Status == "conflict" {
				result.State = "conflict"
			} else if file.Status != "synced" && result.State == "synced" {
				result.State = "pending"
			}
		}
		result.Files = append(result.Files, fileStatuses...)
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

func (s *Service) projectMapping(projectPath, mappingID, remotePath string) (ResolvedProject, Mapping, Repository, string, error) {
	project, err := ResolveProject(projectPath)
	if err != nil {
		return ResolvedProject{}, Mapping{}, Repository{}, "", err
	}
	mappings, err := s.Mappings.ForProject(project.Root)
	if err != nil {
		return ResolvedProject{}, Mapping{}, Repository{}, "", err
	}
	for _, mapping := range mappings {
		if mapping.ID != mappingID {
			continue
		}
		path, err := SafeRelativePath(remotePath, "path")
		if err != nil {
			return ResolvedProject{}, Mapping{}, Repository{}, "", err
		}
		if !inScope(path, []string{mapping.SourcePath}) {
			return ResolvedProject{}, Mapping{}, Repository{}, "", Invalidf("path is outside the selected mapping")
		}
		repository, err := s.Repositories.Get(mapping.RepositoryID)
		return project, mapping, repository, path, err
	}
	return ResolvedProject{}, Mapping{}, Repository{}, "", NewError(ErrNotConfigured, "Mapping was not found for this project", map[string]any{"mappingId": mappingID})
}

func (s *Service) Diff(projectPath, mappingID, remotePath string) (FileDiff, error) {
	project, mapping, repository, path, err := s.projectMapping(projectPath, mappingID, remotePath)
	if err != nil {
		return FileDiff{}, err
	}
	driver := s.Repositories.Driver(repository)
	status, err := driver.Inspect(DriverContext{Repository: repository, Scopes: []string{path}})
	if err != nil {
		return FileDiff{}, err
	}
	remoteContent, remoteExists, err := driver.ReadFile(DriverContext{Repository: repository, Scopes: []string{path}}, status.RemoteRevision, path)
	if err != nil {
		return FileDiff{}, err
	}
	localPath := mappingLocalPath(mapping, path)
	localAbsolute, err := ResolveInside(project.Root, localPath)
	if err != nil {
		return FileDiff{}, err
	}
	localContent, err := os.ReadFile(localAbsolute)
	localExists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return FileDiff{}, err
	}
	return FileDiff{
		MappingID: mapping.ID, RepositoryID: repository.ID,
		LocalPath: localPath, RemotePath: path, RemoteRevision: status.RemoteRevision,
		LocalExists: localExists, RemoteExists: remoteExists, ContentEncoding: "base64",
		LocalContent:  base64.StdEncoding.EncodeToString(localContent),
		RemoteContent: base64.StdEncoding.EncodeToString(remoteContent),
	}, nil
}

func replaceOrRemove(source, target string, exists bool) error {
	if exists {
		return CopyFileReplace(source, target)
	}
	err := os.Remove(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Service) ResolveConflict(projectPath, mappingID, remotePath, expectedRevision string, strategy ConflictStrategy, allowSensitive bool) (SyncResult, error) {
	project, mapping, repository, path, err := s.projectMapping(projectPath, mappingID, remotePath)
	if err != nil {
		return SyncResult{}, err
	}
	if strategy != ConflictStrategyLocal && strategy != ConflictStrategyRemote {
		return SyncResult{}, Invalidf("strategy must be local or remote")
	}
	if mapping.Mode != LinkModeCopy || mapping.Kind != MappingKindFile || path != mapping.SourcePath {
		return SyncResult{}, NewError(ErrUnsupportedCapability, "Single-file conflict resolution currently requires a copy-mode file mapping", map[string]any{"mappingId": mapping.ID})
	}
	var result SyncResult
	err = WithRepositoryLock(filepath.Join(s.Paths.Locks, repository.ID+".lock"), func() error {
		driver := s.Repositories.Driver(repository)
		state, err := s.Storage.ReadState(repository.ID)
		if err != nil {
			return err
		}
		context := DriverContext{Repository: repository, Scopes: []string{path}, ExpectedRevision: state.RemoteRevision}
		before, err := driver.Inspect(context)
		if err != nil {
			return err
		}
		if expectedRevision == "" || before.RemoteRevision != expectedRevision {
			return NewError(ErrConflict, "Repository changed after the diff was reviewed; refresh and review it again", map[string]any{"mappingId": mapping.ID, "paths": []string{path}})
		}
		remoteSnapshot, err := driver.Snapshot(context, before.RemoteRevision)
		if err != nil {
			return err
		}
		localAbsolute, err := ResolveInside(project.Root, mapping.TargetPath)
		if err != nil {
			return err
		}
		localSnapshot, err := SnapshotPath(localAbsolute, path, MappingKindFile)
		if err != nil {
			return err
		}
		files := mappingFileStatuses(mapping, localSnapshot, remoteSnapshot, state.Files)
		if len(files) != 1 || files[0].Status != "conflict" {
			return NewError(ErrConflict, "File status changed; refresh and review the latest diff before resolving", map[string]any{"mappingId": mapping.ID, "paths": []string{path}})
		}
		localExists := files[0].LocalExists
		if err := driver.RestoreWorkspace(context); err != nil {
			return err
		}
		pulled, err := driver.Pull(context)
		if err != nil {
			return err
		}
		if pulled.RemoteRevision != before.RemoteRevision {
			return NewError(ErrConflict, "Repository changed while resolving; refresh and review the latest diff", map[string]any{"mappingId": mapping.ID, "paths": []string{path}})
		}
		state.RemoteRevision = pulled.RemoteRevision
		workspaceAbsolute, err := ResolveInside(repository.WorkspacePath, path)
		if err != nil {
			return err
		}
		if strategy == ConflictStrategyRemote {
			if err := replaceOrRemove(workspaceAbsolute, localAbsolute, files[0].RemoteExists); err != nil {
				return err
			}
		} else {
			if err := replaceOrRemove(localAbsolute, workspaceAbsolute, localExists); err != nil {
				return err
			}
			matches, err := ScanSensitive(repository.WorkspacePath, []string{path})
			if err != nil {
				return err
			}
			if err := AssertNoSensitive(matches, allowSensitive); err != nil {
				return err
			}
			pushed, err := driver.Push(DriverContext{Repository: repository, Scopes: []string{path}, ExpectedRevision: state.RemoteRevision}, "chore("+mapping.ProjectName+"): resolve local config conflict")
			if err != nil {
				return err
			}
			state.RemoteRevision = pushed.RemoteRevision
		}
		delete(state.Files, path)
		resolved, err := SnapshotPath(workspaceAbsolute, path, MappingKindFile)
		if err != nil {
			return err
		}
		for resolvedPath, snapshot := range resolved {
			state.Files[resolvedPath] = snapshot
		}
		state.LastSyncTime = time.Now().UTC().Format(time.RFC3339Nano)
		state.LastError = nil
		if err := s.Storage.WriteState(state); err != nil {
			return err
		}
		result = SyncResult{RepositoryID: repository.ID, State: "synced", RemoteRevision: state.RemoteRevision, LastSyncTime: state.LastSyncTime}
		return nil
	})
	return result, err
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

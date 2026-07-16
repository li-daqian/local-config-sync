package core

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runTestCommand(t *testing.T, cwd, command string, args ...string) string {
	t.Helper()
	cmd := exec.Command(command, args...)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", command, strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func initProject(t *testing.T, root, name string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	runTestCommand(t, path, "git", "init", "--initial-branch", "main")
	runTestCommand(t, path, "git", "config", "user.name", "Local Config Test")
	runTestCommand(t, path, "git", "config", "user.email", "test@example.invalid")
	if err := os.WriteFile(filepath.Join(path, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestCommand(t, path, "git", "add", "README.md")
	runTestCommand(t, path, "git", "commit", "-m", "initial")
	return path
}

func createRemote(t *testing.T, root string) (bare, seed string) {
	t.Helper()
	bare = filepath.Join(root, "remote.git")
	seed = initProject(t, root, "seed")
	runTestCommand(t, root, "git", "init", "--bare", "--initial-branch", "main", bare)
	runTestCommand(t, seed, "git", "remote", "add", "origin", bare)
	runTestCommand(t, seed, "git", "push", "-u", "origin", "main")
	return bare, seed
}

func requireErrorCode(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	var local *Error
	if !errors.As(err, &local) {
		t.Fatalf("expected Local Config error, got %v", err)
	}
	if local.Code != code {
		t.Fatalf("expected error %s, got %s: %s", code, local.Code, local.Message)
	}
}

func TestGitDriverPushPullAndDelete(t *testing.T) {
	root := t.TempDir()
	bare, seed := createRemote(t, root)
	project := initProject(t, root, "business")
	service := NewService(filepath.Join(root, "home"))
	if _, err := service.Init(LinkModeSymlink); err != nil {
		t.Fatal(err)
	}
	checks, err := service.AuthenticateURL(bare, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) == 0 || !checks[len(checks)-1].OK {
		t.Fatalf("authentication did not succeed: %#v", checks)
	}
	repository, err := service.Repositories.AddGit("personal", "", bare, "main")
	if err != nil {
		t.Fatal(err)
	}
	runTestCommand(t, repository.WorkspacePath, "git", "config", "user.name", "Local Config Test")
	runTestCommand(t, repository.WorkspacePath, "git", "config", "user.email", "test@example.invalid")
	if _, err := service.Link(LinkInput{Project: project, RepositoryID: "personal", SourcePath: "business/config", TargetPath: "config", Mode: LinkModeSymlink}); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(project, "config", "application-dev.yml")
	if err := os.WriteFile(file, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Sync(SyncOptions{Project: project}, "sync"); err != nil {
		t.Fatal(err)
	}
	runTestCommand(t, seed, "git", "pull", "--ff-only")
	content, err := os.ReadFile(filepath.Join(seed, "business/config/application-dev.yml"))
	if err != nil || string(content) != "version: 1\n" {
		t.Fatalf("unexpected pushed content %q, %v", content, err)
	}
	if err := os.WriteFile(filepath.Join(seed, "business/config/application-dev.yml"), []byte("version: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestCommand(t, seed, "git", "add", "business/config/application-dev.yml")
	runTestCommand(t, seed, "git", "commit", "-m", "remote change")
	runTestCommand(t, seed, "git", "push")
	if _, err := service.Sync(SyncOptions{Project: project}, "sync"); err != nil {
		t.Fatal(err)
	}
	content, _ = os.ReadFile(file)
	if string(content) != "version: 2\n" {
		t.Fatalf("unexpected pulled content %q", content)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Sync(SyncOptions{Project: project}, "sync"); err != nil {
		t.Fatal(err)
	}
	runTestCommand(t, seed, "git", "pull", "--ff-only")
	if _, err := os.Stat(filepath.Join(seed, "business/config/application-dev.yml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected remote file deletion, got %v", err)
	}
}

func TestFileMappingInitializesFromRemote(t *testing.T) {
	root := t.TempDir()
	bare, seed := createRemote(t, root)
	if err := os.MkdirAll(filepath.Join(seed, "ai-rvis-agent"), 0o755); err != nil {
		t.Fatal(err)
	}
	remoteFile := filepath.Join(seed, "ai-rvis-agent", "application-dev.yml")
	if err := os.WriteFile(remoteFile, []byte("source: remote\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestCommand(t, seed, "git", "add", "ai-rvis-agent/application-dev.yml")
	runTestCommand(t, seed, "git", "commit", "-m", "add remote config")
	runTestCommand(t, seed, "git", "push")

	project := initProject(t, root, "business")
	service := NewService(filepath.Join(root, "home"))
	if _, err := service.Init(LinkModeCopy); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Repositories.AddGit("personal", "", bare, "main"); err != nil {
		t.Fatal(err)
	}
	input := LinkInput{
		Project: project, RepositoryID: "personal",
		SourcePath: "ai-rvis-agent/application-dev.yml",
		TargetPath: "src/main/resources/application-dev.yml",
		Mode:       LinkModeCopy, Kind: MappingKindFile,
	}
	preview, err := service.PreviewLink(input)
	if err != nil {
		t.Fatal(err)
	}
	if preview.State != "remote_only" {
		t.Fatalf("expected remote_only, got %#v", preview)
	}
	mapping, err := service.Link(input)
	if err != nil {
		t.Fatal(err)
	}
	if mapping.Kind != MappingKindFile {
		t.Fatalf("expected file mapping, got %#v", mapping)
	}
	content, err := os.ReadFile(filepath.Join(project, "src/main/resources/application-dev.yml"))
	if err != nil || string(content) != "source: remote\n" {
		t.Fatalf("unexpected local content %q, %v", content, err)
	}
	if status := runTestCommand(t, project, "git", "status", "--short", "--untracked-files=all"); status != "" {
		t.Fatalf("mapped file should be excluded from business Git: %s", status)
	}
}

func TestFileMappingInitializesFromLocalAndPushes(t *testing.T) {
	root := t.TempDir()
	bare, seed := createRemote(t, root)
	project := initProject(t, root, "business")
	localFile := filepath.Join(project, "src/main/resources/application-dev.yml")
	if err := os.MkdirAll(filepath.Dir(localFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localFile, []byte("source: local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewService(filepath.Join(root, "home"))
	if _, err := service.Init(LinkModeCopy); err != nil {
		t.Fatal(err)
	}
	repository, err := service.Repositories.AddGit("personal", "", bare, "main")
	if err != nil {
		t.Fatal(err)
	}
	runTestCommand(t, repository.WorkspacePath, "git", "config", "user.name", "Local Config Test")
	runTestCommand(t, repository.WorkspacePath, "git", "config", "user.email", "test@example.invalid")
	input := LinkInput{
		Project: project, RepositoryID: "personal",
		SourcePath: "ai-rvis-agent/application-dev.yml",
		TargetPath: "src/main/resources/application-dev.yml",
		Mode:       LinkModeCopy, Kind: MappingKindFile,
	}
	preview, err := service.PreviewLink(input)
	if err != nil || preview.State != "local_only" {
		t.Fatalf("unexpected preview %#v, %v", preview, err)
	}
	if _, err := service.Link(input); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Sync(SyncOptions{Project: project}, "sync"); err != nil {
		t.Fatal(err)
	}
	runTestCommand(t, seed, "git", "pull", "--ff-only")
	content, err := os.ReadFile(filepath.Join(seed, "ai-rvis-agent/application-dev.yml"))
	if err != nil || string(content) != "source: local\n" {
		t.Fatalf("unexpected remote content %q, %v", content, err)
	}
}

func TestFileMappingConflictRequiresExplicitStrategy(t *testing.T) {
	root := t.TempDir()
	bare, seed := createRemote(t, root)
	if err := os.MkdirAll(filepath.Join(seed, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seed, "config/application-dev.yml"), []byte("winner: remote\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestCommand(t, seed, "git", "add", "config/application-dev.yml")
	runTestCommand(t, seed, "git", "commit", "-m", "remote config")
	runTestCommand(t, seed, "git", "push")
	project := initProject(t, root, "business")
	localFile := filepath.Join(project, "application-dev.yml")
	if err := os.WriteFile(localFile, []byte("winner: local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewService(filepath.Join(root, "home"))
	if _, err := service.Init(LinkModeCopy); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Repositories.AddGit("personal", "", bare, "main"); err != nil {
		t.Fatal(err)
	}
	input := LinkInput{Project: project, RepositoryID: "personal", SourcePath: "config/application-dev.yml", TargetPath: "application-dev.yml", Mode: LinkModeCopy, Kind: MappingKindFile}
	preview, err := service.PreviewLink(input)
	if err != nil || preview.State != "conflict" {
		t.Fatalf("unexpected preview %#v, %v", preview, err)
	}
	if _, err := service.Link(input); err == nil {
		t.Fatal("expected conflict without explicit strategy")
	} else {
		requireErrorCode(t, err, ErrConflict)
	}
	input.InitialStrategy = InitialStrategyRemote
	if _, err := service.Link(input); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(localFile)
	if err != nil || string(content) != "winner: remote\n" {
		t.Fatalf("unexpected conflict result %q, %v", content, err)
	}
}

func TestFileMappingRejectsTrackedBusinessFile(t *testing.T) {
	root := t.TempDir()
	project := initProject(t, root, "business")
	trackedFile := filepath.Join(project, "application-dev.yml")
	if err := os.WriteFile(trackedFile, []byte("tracked: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestCommand(t, project, "git", "add", "application-dev.yml")
	runTestCommand(t, project, "git", "commit", "-m", "tracked config")
	repositoryPath := filepath.Join(root, "repository")
	service := NewService(filepath.Join(root, "home"))
	if _, err := service.Init(LinkModeCopy); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Repositories.AddLocalFolder("personal", "", repositoryPath); err != nil {
		t.Fatal(err)
	}
	_, err := service.Link(LinkInput{
		Project: project, RepositoryID: "personal", SourcePath: "application-dev.yml",
		TargetPath: "application-dev.yml", Mode: LinkModeCopy, Kind: MappingKindFile,
	})
	requireErrorCode(t, err, ErrInvalidArguments)
}

func TestFileMappingRollsBackBothFilesWhenPersistenceFails(t *testing.T) {
	root := t.TempDir()
	project := initProject(t, root, "business")
	localFile := filepath.Join(project, "application-dev.yml")
	if err := os.WriteFile(localFile, []byte("winner: local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repositoryPath := filepath.Join(root, "repository")
	remoteFile := filepath.Join(repositoryPath, "application-dev.yml")
	if err := os.MkdirAll(repositoryPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(remoteFile, []byte("winner: remote\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewService(filepath.Join(root, "home"))
	if _, err := service.Init(LinkModeCopy); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Repositories.AddLocalFolder("personal", "", repositoryPath); err != nil {
		t.Fatal(err)
	}
	excludePath := filepath.Join(project, ".git/info/exclude")
	if err := os.Remove(excludePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(excludePath, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := service.Link(LinkInput{
		Project: project, RepositoryID: "personal", SourcePath: "application-dev.yml",
		TargetPath: "application-dev.yml", Mode: LinkModeCopy, Kind: MappingKindFile,
		InitialStrategy: InitialStrategyRemote,
	})
	if err == nil {
		t.Fatal("expected exclude persistence failure")
	}
	localContent, _ := os.ReadFile(localFile)
	remoteContent, _ := os.ReadFile(remoteFile)
	if string(localContent) != "winner: local\n" || string(remoteContent) != "winner: remote\n" {
		t.Fatalf("rollback failed: local=%q remote=%q", localContent, remoteContent)
	}
	mappings, mappingErr := service.Mappings.ForProject(project)
	if mappingErr != nil || len(mappings) != 0 {
		t.Fatalf("failed setup must not persist a mapping: %#v, %v", mappings, mappingErr)
	}
}

func TestGitDriverStopsConcurrentChanges(t *testing.T) {
	root := t.TempDir()
	bare, seed := createRemote(t, root)
	project := initProject(t, root, "business")
	service := NewService(filepath.Join(root, "home"))
	if _, err := service.Init(LinkModeSymlink); err != nil {
		t.Fatal(err)
	}
	repository, err := service.Repositories.AddGit("personal", "", bare, "main")
	if err != nil {
		t.Fatal(err)
	}
	runTestCommand(t, repository.WorkspacePath, "git", "config", "user.name", "Local Config Test")
	runTestCommand(t, repository.WorkspacePath, "git", "config", "user.email", "test@example.invalid")
	if _, err := service.Link(LinkInput{Project: project, RepositoryID: "personal", SourcePath: "business/config", TargetPath: "config", Mode: LinkModeSymlink}); err != nil {
		t.Fatal(err)
	}
	localFile := filepath.Join(project, "config", "value.yml")
	if err := os.WriteFile(localFile, []byte("value: baseline\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Sync(SyncOptions{Project: project}, "sync"); err != nil {
		t.Fatal(err)
	}
	runTestCommand(t, seed, "git", "pull", "--ff-only")
	if err := os.WriteFile(localFile, []byte("value: local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seed, "business/config/value.yml"), []byte("value: remote\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestCommand(t, seed, "git", "add", "business/config/value.yml")
	runTestCommand(t, seed, "git", "commit", "-m", "remote conflict")
	runTestCommand(t, seed, "git", "push")
	_, err = service.Sync(SyncOptions{Project: project}, "sync")
	requireErrorCode(t, err, ErrConflict)
	content, _ := os.ReadFile(localFile)
	if string(content) != "value: local\n" {
		t.Fatalf("local content was overwritten: %q", content)
	}
}

func TestLocalFolderLinkStatusAndSensitiveFiles(t *testing.T) {
	root := t.TempDir()
	project := initProject(t, root, "project")
	repositoryPath := filepath.Join(root, "repository")
	service := NewService(filepath.Join(root, "home"))
	if _, err := service.Init(LinkModeSymlink); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Repositories.AddLocalFolder("local", "", repositoryPath); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Link(LinkInput{Project: project, RepositoryID: "local", SourcePath: "sample/config", TargetPath: "config", Mode: LinkModeSymlink}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(filepath.Join(project, "config"))
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("target is not a symlink: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, "config", "application-dev.yml"), []byte("feature: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Sync(SyncOptions{Project: project}, "sync"); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(filepath.Join(project, ".git/info/exclude"))
	if !strings.Contains(string(content), "/config\n") {
		t.Fatalf("exclude rule missing: %s", content)
	}
	if businessStatus := runTestCommand(t, project, "git", "status", "--short", "--untracked-files=all"); businessStatus != "" {
		t.Fatalf("mapped symlink should be excluded from business Git: %s", businessStatus)
	}
	status, err := service.Status(project)
	if err != nil || status.State != "synced" {
		t.Fatalf("unexpected status %#v, %v", status, err)
	}
	if err := os.WriteFile(filepath.Join(project, "config", ".env"), []byte("PASSWORD=nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = service.Sync(SyncOptions{Project: project}, "sync")
	requireErrorCode(t, err, ErrUnsafeSecretPattern)
	if _, err := service.Sync(SyncOptions{Project: project, AllowSensitive: true}, "sync"); err != nil {
		t.Fatal(err)
	}
}

func TestCopyModeRoundTrip(t *testing.T) {
	root := t.TempDir()
	project := initProject(t, root, "project")
	repositoryPath := filepath.Join(root, "repository")
	if err := os.MkdirAll(filepath.Join(repositoryPath, "sample/config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repositoryPath, "sample/config/value.yml"), []byte("value: one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewService(filepath.Join(root, "home"))
	if _, err := service.Init(LinkModeCopy); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Repositories.AddLocalFolder("local", "", repositoryPath); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Link(LinkInput{Project: project, RepositoryID: "local", SourcePath: "sample/config", TargetPath: "config", Mode: LinkModeCopy}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Sync(SyncOptions{Project: project}, "sync"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "config/value.yml"), []byte("value: two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Sync(SyncOptions{Project: project}, "sync"); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(filepath.Join(repositoryPath, "sample/config/value.yml"))
	if string(content) != "value: two\n" {
		t.Fatalf("copy did not reconcile: %q", content)
	}
}

func TestMappingOverlapAndRepositoryLock(t *testing.T) {
	root := t.TempDir()
	first := initProject(t, root, "first")
	second := initProject(t, root, "second")
	service := NewService(filepath.Join(root, "home"))
	if _, err := service.Init(LinkModeSymlink); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Repositories.AddLocalFolder("local", "", filepath.Join(root, "repo")); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Link(LinkInput{Project: first, RepositoryID: "local", SourcePath: "shared", TargetPath: "config"}); err != nil {
		t.Fatal(err)
	}
	_, err := service.Link(LinkInput{Project: second, RepositoryID: "local", SourcePath: "shared/nested", TargetPath: "config"})
	requireErrorCode(t, err, ErrInvalidArguments)
	lockPath := filepath.Join(root, "repo.lock")
	acquired := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		finished <- WithRepositoryLock(lockPath, func() error { close(acquired); <-release; return nil })
	}()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("lock was not acquired")
	}
	err = WithRepositoryLock(lockPath, func() error { return nil })
	requireErrorCode(t, err, ErrRepositoryLocked)
	close(release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}

func TestReadsTypeScriptYAMLState(t *testing.T) {
	home := t.TempDir()
	content := `version: 1
repositories:
  - id: personal
    name: Personal
    type: git
    workspacePath: /tmp/workspace
    options:
      remoteUrl: git@example.invalid:user/config.git
      branch: main
`
	if err := os.WriteFile(filepath.Join(home, "repositories.yml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	repositories, err := NewStorage(GetAppPaths(home)).ReadRepositories()
	if err != nil {
		t.Fatal(err)
	}
	if len(repositories.Repositories) != 1 || repositories.Repositories[0].Options.RemoteURL != "git@example.invalid:user/config.git" {
		t.Fatalf("unexpected compatibility result: %#v", repositories)
	}
	storage := NewStorage(GetAppPaths(home))
	if err := storage.WriteRepositories(repositories); err != nil {
		t.Fatal(err)
	}
	roundTripped, err := storage.ReadRepositories()
	if err != nil {
		t.Fatal(err)
	}
	if len(roundTripped.Repositories) != 1 || roundTripped.Repositories[0] != repositories.Repositories[0] {
		t.Fatalf("state changed after Go YAML rewrite: %#v", roundTripped)
	}
}

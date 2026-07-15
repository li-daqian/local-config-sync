package core

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var gitCapabilities = DriverCapabilities{History: true, ConditionalWrite: true, AtomicPublish: true}
var emptyRemotePattern = regexp.MustCompile(`(?i)empty repository|remote branch .* not found`)
var authPattern = regexp.MustCompile(`(?i)authentication|permission denied|could not read|access denied`)
var rejectedPattern = regexp.MustCompile(`(?i)rejected|fetch first|non-fast-forward`)

type GitDriver struct{}

func splitLines(value string) []string {
	if value == "" {
		return []string{}
	}
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func normalizeStatusPath(line string) string {
	if len(line) >= 3 {
		line = strings.TrimSpace(line[3:])
	}
	if index := strings.LastIndex(line, " -> "); index >= 0 {
		line = line[index+4:]
	}
	return strings.Trim(line, `"`)
}

func inScope(path string, scopes []string) bool {
	for _, scope := range scopes {
		if path == scope || strings.HasPrefix(path, scope+"/") {
			return true
		}
	}
	return false
}

func remoteRevision(repository Repository) (string, error) {
	result, err := RunProcess("git", []string{"ls-remote", repository.Options.RemoteURL, "refs/heads/" + repository.Options.Branch}, "", nil, true)
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		code := ErrRepositoryFailed
		if authPattern.MatchString(result.Stderr) {
			code = ErrAuthFailed
		}
		return "", NewError(code, "Cannot read Git remote revision", map[string]any{"repositoryId": repository.ID})
	}
	fields := strings.Fields(result.Stdout)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], nil
}

func localRevision(repository Repository) (string, error) {
	result, err := RunProcess("git", []string{"rev-parse", "HEAD"}, repository.WorkspacePath, nil, true)
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", nil
	}
	return result.Stdout, nil
}

func (d *GitDriver) Prepare(repository Repository) error {
	if err := os.MkdirAll(repository.WorkspacePath, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(repository.WorkspacePath)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		clone, err := RunProcess("git", []string{"clone", "--origin", "origin", "--branch", repository.Options.Branch, "--single-branch", repository.Options.RemoteURL, repository.WorkspacePath}, "", nil, true)
		if err != nil {
			return err
		}
		if clone.ExitCode != 0 {
			if !emptyRemotePattern.MatchString(clone.Stdout + "\n" + clone.Stderr) {
				code := ErrRepositoryFailed
				if authPattern.MatchString(clone.Stderr) {
					code = ErrAuthFailed
				}
				return NewError(code, "Cannot clone Git repository: "+clone.Stderr, map[string]any{"repositoryId": repository.ID})
			}
			if _, err := RunProcess("git", []string{"init", "--initial-branch", repository.Options.Branch}, repository.WorkspacePath, nil, false); err != nil {
				return err
			}
			_, err = RunProcess("git", []string{"remote", "add", "origin", repository.Options.RemoteURL}, repository.WorkspacePath, nil, false)
			return err
		}
		return nil
	}
	inside, err := RunProcess("git", []string{"rev-parse", "--is-inside-work-tree"}, repository.WorkspacePath, nil, true)
	if err != nil {
		return err
	}
	if inside.Stdout != "true" {
		return NewError(ErrRepositoryFailed, "Managed Git workspace is not a Git repository", map[string]any{"workspacePath": repository.WorkspacePath})
	}
	remote, err := RunProcess("git", []string{"remote", "get-url", "origin"}, repository.WorkspacePath, nil, true)
	if err != nil {
		return err
	}
	if remote.ExitCode != 0 || remote.Stdout != repository.Options.RemoteURL {
		return NewError(ErrRepositoryFailed, "Managed workspace origin does not match configured remote", map[string]any{"configured": repository.Options.RemoteURL, "actual": remote.Stdout})
	}
	return nil
}

func (d *GitDriver) Inspect(context DriverContext) (RepositoryStatus, error) {
	if err := d.Prepare(context.Repository); err != nil {
		return RepositoryStatus{}, err
	}
	remote, err := remoteRevision(context.Repository)
	if err != nil {
		return RepositoryStatus{}, err
	}
	local, err := localRevision(context.Repository)
	if err != nil {
		return RepositoryStatus{}, err
	}
	dirty, err := RunProcess("git", []string{"status", "--porcelain=v1", "--untracked-files=all"}, context.Repository.WorkspacePath, nil, false)
	if err != nil {
		return RepositoryStatus{}, err
	}
	changes := []string{}
	for _, line := range splitLines(dirty.Stdout) {
		changes = append(changes, normalizeStatusPath(line))
	}
	state := "synced"
	if len(changes) > 0 || remote != local {
		state = "pending"
	}
	return RepositoryStatus{State: state, RemoteRevision: remote, LocalChanges: changes, RemoteChanged: remote != local, Capabilities: gitCapabilities}, nil
}

func (d *GitDriver) Pull(context DriverContext) (PullResult, error) {
	before, err := localRevision(context.Repository)
	if err != nil {
		return PullResult{}, err
	}
	remote, err := remoteRevision(context.Repository)
	if err != nil {
		return PullResult{}, err
	}
	if remote == "" || remote == before {
		if remote == "" {
			remote = before
		}
		return PullResult{RemoteRevision: remote}, nil
	}
	dirty, err := RunProcess("git", []string{"status", "--porcelain=v1", "--untracked-files=all"}, context.Repository.WorkspacePath, nil, false)
	if err != nil {
		return PullResult{}, err
	}
	paths := []string{}
	for _, line := range splitLines(dirty.Stdout) {
		paths = append(paths, normalizeStatusPath(line))
	}
	if len(paths) > 0 {
		return PullResult{}, NewError(ErrConflict, "Remote changed while the managed workspace has local changes", map[string]any{"repositoryId": context.Repository.ID, "paths": paths})
	}
	if _, err := RunProcess("git", []string{"fetch", "--prune", "origin", context.Repository.Options.Branch}, context.Repository.WorkspacePath, nil, false); err != nil {
		return PullResult{}, err
	}
	merge, err := RunProcess("git", []string{"merge", "--ff-only", "origin/" + context.Repository.Options.Branch}, context.Repository.WorkspacePath, nil, true)
	if err != nil {
		return PullResult{}, err
	}
	if merge.ExitCode != 0 {
		return PullResult{}, NewError(ErrConflict, "Git history diverged; resolve it manually in the managed workspace", map[string]any{"repositoryId": context.Repository.ID, "workspacePath": context.Repository.WorkspacePath})
	}
	revision, err := localRevision(context.Repository)
	return PullResult{RemoteRevision: revision, Changed: true}, err
}

func (d *GitDriver) Push(context DriverContext, commitMessage string) (PushResult, error) {
	currentRemote, err := remoteRevision(context.Repository)
	if err != nil {
		return PushResult{}, err
	}
	if currentRemote != context.ExpectedRevision {
		return PushResult{}, NewError(ErrConflict, "Remote revision changed before push", map[string]any{"repositoryId": context.Repository.ID, "expectedRevision": context.ExpectedRevision, "remoteRevision": currentRemote})
	}
	status, err := RunProcess("git", []string{"status", "--porcelain=v1", "--untracked-files=all"}, context.Repository.WorkspacePath, nil, false)
	if err != nil {
		return PushResult{}, err
	}
	changed, outside := []string{}, []string{}
	for _, line := range splitLines(status.Stdout) {
		path := normalizeStatusPath(line)
		changed = append(changed, path)
		if !inScope(path, context.Scopes) {
			outside = append(outside, path)
		}
	}
	if len(outside) > 0 {
		return PushResult{}, NewError(ErrRepositoryDirtyOutsideScope, "Repository has changes outside the requested mapping scope", map[string]any{"repositoryId": context.Repository.ID, "paths": outside})
	}
	if len(changed) == 0 {
		return PushResult{RemoteRevision: currentRemote}, nil
	}
	args := append([]string{"add", "--all", "--"}, context.Scopes...)
	if _, err := RunProcess("git", args, context.Repository.WorkspacePath, nil, false); err != nil {
		return PushResult{}, err
	}
	staged, err := RunProcess("git", []string{"diff", "--cached", "--quiet"}, context.Repository.WorkspacePath, nil, true)
	if err != nil {
		return PushResult{}, err
	}
	if staged.ExitCode == 0 {
		return PushResult{RemoteRevision: currentRemote}, nil
	}
	identity, err := RunProcess("git", []string{"config", "user.email"}, context.Repository.WorkspacePath, nil, true)
	if err != nil {
		return PushResult{}, err
	}
	if identity.ExitCode != 0 {
		return PushResult{}, NewError(ErrRepositoryFailed, "Git author identity is not configured. Set user.name and user.email.", map[string]any{"workspacePath": context.Repository.WorkspacePath})
	}
	if _, err := RunProcess("git", []string{"commit", "-m", commitMessage}, context.Repository.WorkspacePath, nil, false); err != nil {
		return PushResult{}, err
	}
	pushed, err := RunProcess("git", []string{"push", "origin", "HEAD:refs/heads/" + context.Repository.Options.Branch}, context.Repository.WorkspacePath, nil, true)
	if err != nil {
		return PushResult{}, err
	}
	if pushed.ExitCode != 0 {
		code := ErrRepositoryFailed
		if authPattern.MatchString(pushed.Stderr) {
			code = ErrAuthFailed
		} else if rejectedPattern.MatchString(pushed.Stderr) {
			code = ErrConflict
		}
		return PushResult{}, NewError(code, "Git push failed: "+pushed.Stderr, map[string]any{"repositoryId": context.Repository.ID})
	}
	revision, err := localRevision(context.Repository)
	return PushResult{RemoteRevision: revision, Changed: true}, err
}

func (d *GitDriver) Doctor(repository Repository) (DiagnosticResult, error) {
	checks := []DiagnosticCheck{}
	git, _ := RunProcess("git", []string{"--version"}, "", nil, true)
	if git.ExitCode == 0 {
		checks = append(checks, DiagnosticCheck{Name: "git-cli", OK: true, Message: git.Stdout})
	} else {
		checks = append(checks, DiagnosticCheck{Name: "git-cli", Message: "Git CLI is not available", Remediation: "Install Git and ensure it is on PATH"})
	}
	if git.ExitCode == 0 {
		authChecks, err := DiagnoseGitAuth(repository)
		if err != nil {
			return DiagnosticResult{}, err
		}
		checks = append(checks, authChecks...)
	}
	usable, _ := RunProcess("git", []string{"rev-parse", "--is-inside-work-tree"}, repository.WorkspacePath, nil, true)
	if usable.Stdout == "true" {
		checks = append(checks, DiagnosticCheck{Name: "workspace", OK: true, Message: "Managed workspace is valid"})
	} else {
		checks = append(checks, DiagnosticCheck{Name: "workspace", Message: "Managed workspace is not ready: " + filepath.Base(repository.WorkspacePath)})
	}
	ok := true
	for _, check := range checks {
		if !check.OK && check.Name != "github-cli-auth" {
			ok = false
		}
	}
	return DiagnosticResult{OK: ok, Checks: checks}, nil
}

package core

import (
	"os"
	"path/filepath"
)

type ResolvedProject struct {
	Root        string
	ExcludePath string
}

func ResolveProject(projectPath string) (ResolvedProject, error) {
	candidate, err := filepath.Abs(projectPath)
	if err != nil {
		return ResolvedProject{}, err
	}
	if real, err := filepath.EvalSymlinks(candidate); err == nil {
		candidate = real
	}
	rootResult, err := RunProcess("git", []string{"-C", candidate, "rev-parse", "--show-toplevel"}, "", nil, true)
	if err != nil {
		return ResolvedProject{}, err
	}
	if rootResult.ExitCode != 0 {
		return ResolvedProject{}, NewError(ErrNotConfigured, candidate+" is not a Git working tree", map[string]any{"projectPath": candidate})
	}
	root := rootResult.Stdout
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	exclude, err := RunProcess("git", []string{"-C", root, "rev-parse", "--path-format=absolute", "--git-path", "info/exclude"}, "", nil, false)
	if err != nil {
		return ResolvedProject{}, err
	}
	if !filepath.IsAbs(exclude.Stdout) {
		exclude.Stdout = filepath.Join(root, exclude.Stdout)
	}
	if _, err := os.Stat(root); err != nil {
		return ResolvedProject{}, err
	}
	return ResolvedProject{Root: root, ExcludePath: exclude.Stdout}, nil
}

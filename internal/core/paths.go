package core

import (
	"os"
	"path/filepath"
)

type AppPaths struct {
	Home         string
	Config       string
	Repositories string
	Mappings     string
	Workspaces   string
	States       string
	Locks        string
	Logs         string
}

func GetAppPaths(home string) AppPaths {
	if home == "" {
		home = os.Getenv("LOCAL_CONFIG_HOME")
	}
	if home == "" {
		userHome, _ := os.UserHomeDir()
		home = filepath.Join(userHome, ".local-config-sync")
	}
	home, _ = filepath.Abs(home)
	return AppPaths{
		Home: home, Config: filepath.Join(home, "config.yml"),
		Repositories: filepath.Join(home, "repositories.yml"),
		Mappings:     filepath.Join(home, "mappings.yml"), Workspaces: filepath.Join(home, "workspaces"),
		States: filepath.Join(home, "state", "repositories"), Locks: filepath.Join(home, "locks"),
		Logs: filepath.Join(home, "logs"),
	}
}

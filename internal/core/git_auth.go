package core

import (
	"net/url"
	"regexp"
	"strings"
)

var scpHostPattern = regexp.MustCompile(`^[^@]+@([^:]+):`)
var authFailurePattern = regexp.MustCompile(`(?i)authentication|permission denied|could not read|access denied|not found`)

func hostFromURL(value string) string {
	if match := scpHostPattern.FindStringSubmatch(value); len(match) > 1 {
		return match[1]
	}
	parsed, err := url.Parse(value)
	if err == nil {
		return parsed.Hostname()
	}
	return ""
}

func isSSHURL(value string) bool {
	return strings.HasPrefix(value, "git@") || strings.HasPrefix(value, "ssh://")
}

func DiagnoseGitAuth(repository Repository) ([]DiagnosticCheck, error) {
	host := hostFromURL(repository.Options.RemoteURL)
	checks := []DiagnosticCheck{}
	if host == "github.com" {
		gh, err := RunProcess("gh", []string{"auth", "status", "--hostname", host}, "", nil, true)
		if err != nil {
			gh = ProcessResult{ExitCode: 127, Stderr: "GitHub CLI is not installed"}
		}
		check := DiagnosticCheck{Name: "github-cli-auth", OK: gh.ExitCode == 0}
		if check.OK {
			check.Message = "GitHub CLI authentication is available"
		} else {
			check.Message = "GitHub CLI authentication is not available; SSH or Git credential may still work"
			check.Remediation = "Run: gh auth login"
		}
		checks = append(checks, check)
	}
	remote, err := RunProcess("git", []string{"ls-remote", "--exit-code", repository.Options.RemoteURL, "HEAD"}, "", nil, true)
	if err != nil {
		return nil, err
	}
	emptyRemote := remote.ExitCode == 2 && !authFailurePattern.MatchString(remote.Stderr)
	ok := remote.ExitCode == 0 || emptyRemote
	check := DiagnosticCheck{Name: "git-remote-auth", OK: ok}
	if ok {
		check.Message = "Git remote authentication succeeded"
	} else {
		check.Message = "Git could not authenticate to the remote"
		if isSSHURL(repository.Options.RemoteURL) {
			if host == "" {
				host = "<host>"
			}
			check.Remediation = "Verify your SSH key with: ssh -T git@" + host
		} else {
			check.Remediation = "Configure a Git credential helper or run the provider CLI login command"
		}
	}
	return append(checks, check), nil
}

func AuthenticateGit(repository Repository, method string) ([]DiagnosticCheck, error) {
	host := hostFromURL(repository.Options.RemoteURL)
	if method == "gh" || (method == "auto" && host == "github.com" && (strings.HasPrefix(repository.Options.RemoteURL, "http://") || strings.HasPrefix(repository.Options.RemoteURL, "https://"))) {
		if host != "github.com" {
			return nil, NewError(ErrInvalidArguments, "gh auth can only configure github.com repositories", map[string]any{"host": host})
		}
		status, err := RunProcess("gh", []string{"auth", "status", "--hostname", host}, "", nil, true)
		if err != nil {
			status = ProcessResult{ExitCode: 127}
		}
		if status.ExitCode != 0 && method == "gh" {
			return nil, NewError(ErrAuthFailed, "GitHub CLI is not authenticated. Run `gh auth login` in an interactive terminal.", map[string]any{"host": host})
		}
		if status.ExitCode == 0 {
			if _, err := RunProcess("gh", []string{"auth", "setup-git"}, "", nil, false); err != nil {
				return nil, err
			}
		}
	} else if method == "ssh" && !isSSHURL(repository.Options.RemoteURL) {
		return nil, NewError(ErrInvalidArguments, "Repository URL is not an SSH URL", map[string]any{"remoteUrl": repository.Options.RemoteURL})
	} else if method == "credential" && isSSHURL(repository.Options.RemoteURL) {
		return nil, NewError(ErrInvalidArguments, "Git credential helpers apply to HTTP(S) URLs, not SSH URLs", map[string]any{"remoteUrl": repository.Options.RemoteURL})
	}
	checks, err := DiagnoseGitAuth(repository)
	if err != nil {
		return nil, err
	}
	for _, check := range checks {
		if check.Name == "git-remote-auth" && check.OK {
			return checks, nil
		}
	}
	return nil, NewError(ErrAuthFailed, "Git authentication check failed", map[string]any{"repositoryId": repository.ID, "checks": checks})
}

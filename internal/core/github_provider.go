package core

import "encoding/json"

func AuthenticateGitHubProvider() ([]DiagnosticCheck, error) {
	status, err := RunProcess("gh", []string{"auth", "status", "--hostname", "github.com"}, "", nil, true)
	if err != nil {
		return nil, NewError(ErrAuthFailed, "GitHub CLI is not installed. Install `gh` and run `gh auth login`.", map[string]any{"provider": "github"})
	}
	if status.ExitCode != 0 {
		return nil, NewError(ErrAuthFailed, "GitHub authentication is required. Run `gh auth login --hostname github.com` and retry.", map[string]any{"provider": "github"})
	}
	if _, err := RunProcess("gh", []string{"auth", "setup-git"}, "", nil, false); err != nil {
		return nil, err
	}
	return []DiagnosticCheck{{Name: "github-auth", OK: true, Message: "GitHub CLI authentication is available"}}, nil
}

func ListGitHubRepositories() ([]GitHubRepository, error) {
	if _, err := AuthenticateGitHubProvider(); err != nil {
		return nil, err
	}
	result, err := RunProcess("gh", []string{
		"repo", "list", "--limit", "1000", "--no-archived",
		"--json", "nameWithOwner,isPrivate,sshUrl,url,defaultBranchRef",
	}, "", nil, true)
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, NewError(ErrRepositoryFailed, "Cannot list GitHub repositories: "+result.Stderr, map[string]any{"provider": "github"})
	}
	return parseGitHubRepositories([]byte(result.Stdout))
}

func parseGitHubRepositories(content []byte) ([]GitHubRepository, error) {
	var payload []struct {
		NameWithOwner    string `json:"nameWithOwner"`
		Private          bool   `json:"isPrivate"`
		SSHURL           string `json:"sshUrl"`
		URL              string `json:"url"`
		DefaultBranchRef *struct {
			Name string `json:"name"`
		} `json:"defaultBranchRef"`
	}
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, WrapError(ErrRepositoryFailed, "GitHub CLI returned invalid repository data", err, nil)
	}
	repositories := make([]GitHubRepository, 0, len(payload))
	for _, item := range payload {
		branch := "main"
		if item.DefaultBranchRef != nil && item.DefaultBranchRef.Name != "" {
			branch = item.DefaultBranchRef.Name
		}
		repositories = append(repositories, GitHubRepository{
			NameWithOwner: item.NameWithOwner,
			Private:       item.Private,
			SSHURL:        item.SSHURL,
			URL:           item.URL,
			DefaultBranch: branch,
		})
	}
	return repositories, nil
}

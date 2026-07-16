package core

import "testing"

func TestParseGitHubRepositoriesIncludesPrivateAndEmptyRepositories(t *testing.T) {
	repositories, err := parseGitHubRepositories([]byte(`[
        {"nameWithOwner":"owner/private-config","isPrivate":true,"sshUrl":"git@github.com:owner/private-config.git","url":"https://github.com/owner/private-config","defaultBranchRef":{"name":"develop"}},
        {"nameWithOwner":"owner/empty","isPrivate":false,"sshUrl":"git@github.com:owner/empty.git","url":"https://github.com/owner/empty","defaultBranchRef":null}
    ]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(repositories) != 2 || !repositories[0].Private || repositories[0].DefaultBranch != "develop" {
		t.Fatalf("unexpected private repository: %#v", repositories)
	}
	if repositories[1].DefaultBranch != "main" {
		t.Fatalf("empty repository should default to main: %#v", repositories[1])
	}
}

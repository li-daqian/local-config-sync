# Local Config Sync for VS Code

Local Config Sync keeps local overlay configuration inside the project where tools can discover it,
while excluding it from the business repository and synchronizing it through a user-configured
Git or local-folder Repository.

The extension bundles the native `local-config` CLI. Repository access, mapping, conflict detection,
sensitive-file checks, and `.git/info/exclude` updates remain in the shared Go core.

## Features

- File-level synchronization status for every workspace folder.
- Safe manual synchronization with upload/download direction review.
- GitHub Repository discovery through the existing GitHub CLI login.
- Single-file mapping setup with initial diff review.
- Explicit conflict resolution using the VS Code diff editor.
- Remote SSH, WSL, and Dev Container support through the workspace extension host.

The extension does not store Git credentials and is not a secret manager. It is disabled in
untrusted and virtual workspaces.

## Requirements

- A trusted file-system workspace backed by a Git repository.
- System `git`.
- GitHub CLI (`gh`) for GitHub discovery and browser authentication.

Use `Local Config Sync: Add Mapping` from the Command Palette or the Local Config Sync Activity Bar
view to get started.

import * as vscode from "vscode";
import { CliClient, CliError } from "../cli/client";
import { commandResponseSchema } from "../cli/models";
import { ProjectStore } from "../state/projectStore";
import { FileNode } from "../views/statusTree";
import { resolveProjectFolder, saveDirtyProjectDocuments } from "../workspace/projectResolver";
import { confirmSensitivePaths, DiffService, showCommandError } from "./diff";

export class CoreCommands {
  private readonly activeMutations = new Set<string>();

  constructor(
    private readonly client: CliClient,
    private readonly store: ProjectStore,
    private readonly diffs: DiffService
  ) {}

  async sync(value?: unknown): Promise<void> {
    const folder = await resolveProjectFolder(value);
    if (folder === undefined) {
      return;
    }
    const snapshot = this.store.get(folder);
    if (snapshot.state !== "ready") {
      await vscode.window.showInformationMessage("Refresh Local Config Sync status before syncing.");
      return;
    }
    const conflicts = snapshot.response.files.filter((file) => file.status === "conflict");
    if (conflicts.length > 0) {
      await vscode.window.showWarningMessage(
        `${conflicts.length} conflict${conflicts.length === 1 ? "" : "s"} must be reviewed before syncing.`
      );
      const first = conflicts[0];
      if (first !== undefined) {
        await this.diffs.show(new FileNode(
          folder,
          first,
          snapshot.response.mappings.find((mapping) => mapping.id === first.mappingId)
        ));
      }
      return;
    }
    const uploads = snapshot.response.files.filter((file) => file.status === "local_changes");
    const downloads = snapshot.response.files.filter((file) => file.status === "remote_changes");
    if (uploads.length === 0 && downloads.length === 0) {
      await vscode.window.showInformationMessage("Local and Repository files are already synchronized.");
      return;
    }
    const confirmation = await vscode.window.showInformationMessage(
      syncSummary(uploads.map((file) => file.localPath), downloads.map((file) => file.localPath)),
      { modal: true },
      "Sync"
    );
    if (confirmation === undefined) {
      return;
    }
    await this.runMutation(folder, "Synchronizing local configuration", async () => {
      await saveDirtyProjectDocuments(folder);
      const args = ["sync", "--project", folder.uri.fsPath];
      try {
        await this.client.run(args, commandResponseSchema, { cwd: folder.uri.fsPath });
      } catch (error) {
        if (!(error instanceof CliError) || error.code !== "unsafe_secret_pattern") {
          throw error;
        }
        if (!await confirmSensitivePaths(error.paths)) {
          return;
        }
        await this.client.run([...args, "--allow-sensitive"], commandResponseSchema, { cwd: folder.uri.fsPath });
      }
    });
  }

  async authenticate(value?: unknown): Promise<void> {
    const folder = await resolveProjectFolder(value);
    if (folder === undefined) {
      return;
    }
    const snapshot = this.store.get(folder);
    if (snapshot.state !== "ready") {
      await vscode.window.showInformationMessage("Refresh Local Config Sync status before authenticating.");
      return;
    }
    const repositories = snapshot.response.repositories.filter((repository) => repository.type === "git");
    const repository = repositories.length === 1
      ? repositories[0]
      : (await vscode.window.showQuickPick(
          repositories.map((candidate) => ({
            label: candidate.name || candidate.id,
            description: candidate.id,
            repository: candidate
          })),
          { placeHolder: "Choose a Git Repository" }
        ))?.repository;
    if (repository === undefined) {
      if (repositories.length === 0) {
        await vscode.window.showInformationMessage("No Git Repository is configured for this project.");
      }
      return;
    }
    const method = await vscode.window.showQuickPick(
      [
        { label: "Automatic", description: "Use GitHub CLI or the existing system Git credential chain", value: "auto" },
        { label: "GitHub CLI", description: "Use gh auth and configure Git credentials", value: "gh" },
        { label: "SSH", description: "Use ssh-agent or an existing SSH key", value: "ssh" },
        { label: "Credential Helper", description: "Use the system Git credential helper", value: "credential" }
      ],
      { placeHolder: "Choose an authentication method" }
    );
    if (method === undefined) {
      return;
    }
    await this.runMutation(folder, "Checking Git authentication", async () => {
      try {
        await this.client.run(
          ["repository", "auth", repository.id, "--method", method.value],
          commandResponseSchema,
          { cwd: folder.uri.fsPath }
        );
      } catch (error) {
        if (!(error instanceof CliError) || error.code !== "auth_failed" || !["auto", "gh"].includes(method.value)) {
          throw error;
        }
        const action = await vscode.window.showWarningMessage(
          "GitHub CLI authentication is not ready for this Repository.",
          "Open Login Terminal"
        );
        if (action !== undefined) {
          const terminal = vscode.window.createTerminal({
            name: "Local Config Sync · GitHub Login",
            cwd: folder.uri
          });
          terminal.show();
          terminal.sendText("gh auth login --hostname github.com --git-protocol https --web", true);
        }
        return;
      }
      await vscode.window.showInformationMessage(`Authentication succeeded for ${repository.name || repository.id}.`);
    });
  }

  private async runMutation(
    folder: vscode.WorkspaceFolder,
    title: string,
    operation: () => Promise<void>
  ): Promise<void> {
    const key = folder.uri.toString();
    if (this.activeMutations.has(key)) {
      await vscode.window.showInformationMessage("A Local Config Sync operation is already running for this project.");
      return;
    }
    this.activeMutations.add(key);
    try {
      await vscode.window.withProgress(
        { location: vscode.ProgressLocation.Notification, title },
        operation
      );
      await this.store.refreshAll();
    } catch (error) {
      await showCommandError(error);
    } finally {
      this.activeMutations.delete(key);
    }
  }
}

function syncSummary(uploads: readonly string[], downloads: readonly string[]): string {
  const lines = ["Review the synchronization direction:"];
  if (uploads.length > 0) {
    lines.push("", `Upload to Repository (${uploads.length})`, ...uploads.slice(0, 8).map((path) => `• ${path}`));
  }
  if (downloads.length > 0) {
    lines.push("", `Download to Local (${downloads.length})`, ...downloads.slice(0, 8).map((path) => `• ${path}`));
  }
  if (uploads.length + downloads.length > 16) {
    lines.push("", "Additional mapped files are not shown in this summary.");
  }
  return lines.join("\n");
}

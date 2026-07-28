import * as vscode from "vscode";
import { CliClient, CliError } from "../cli/client";
import { statusResponseSchema, type StatusResponse } from "../cli/models";
import { supportedWorkspaceFolders } from "../workspace/projectResolver";

export type ProjectSnapshot =
  | { state: "loading" }
  | { state: "ready"; response: StatusResponse }
  | { state: "error"; error: CliError };

export class ProjectStore implements vscode.Disposable {
  private readonly snapshots = new Map<string, ProjectSnapshot>();
  private readonly changes = new vscode.EventEmitter<void>();
  private refreshGeneration = 0;

  readonly onDidChange = this.changes.event;

  constructor(private readonly client: CliClient) {}

  get(folder: vscode.WorkspaceFolder): ProjectSnapshot {
    return this.snapshots.get(folder.uri.toString()) ?? { state: "loading" };
  }

  async refreshAll(): Promise<void> {
    const generation = ++this.refreshGeneration;
    const folders = supportedWorkspaceFolders();
    this.removeMissingFolders(folders);
    await Promise.all(folders.map((folder) => this.refresh(folder, generation)));
    if (generation === this.refreshGeneration) {
      await this.updateContext();
    }
  }

  async refresh(folder: vscode.WorkspaceFolder, generation = ++this.refreshGeneration): Promise<void> {
    const key = folder.uri.toString();
    this.snapshots.set(key, { state: "loading" });
    this.changes.fire();
    try {
      const response = await this.client.run(
        ["status", "--project", folder.uri.fsPath],
        statusResponseSchema,
        { cwd: folder.uri.fsPath, timeoutMs: 60_000 }
      );
      if (generation === this.refreshGeneration) {
        this.snapshots.set(key, { state: "ready", response });
      }
    } catch (error) {
      const cliError = error instanceof CliError
        ? error
        : new CliError("unexpected_error", error instanceof Error ? error.message : String(error));
      if (generation === this.refreshGeneration) {
        this.snapshots.set(key, { state: "error", error: cliError });
      }
    }
    this.changes.fire();
    await this.updateContext();
  }

  dispose(): void {
    this.changes.dispose();
  }

  private removeMissingFolders(folders: readonly vscode.WorkspaceFolder[]): void {
    const active = new Set(folders.map((folder) => folder.uri.toString()));
    for (const key of this.snapshots.keys()) {
      if (!active.has(key)) {
        this.snapshots.delete(key);
      }
    }
  }

  private async updateContext(): Promise<void> {
    const hasWorkspace = supportedWorkspaceFolders().length > 0;
    const hasMappings = [...this.snapshots.values()].some(
      (snapshot) => snapshot.state === "ready" && snapshot.response.mappings.length > 0
    );
    await Promise.all([
      vscode.commands.executeCommand("setContext", "localConfigSync.hasWorkspace", hasWorkspace),
      vscode.commands.executeCommand("setContext", "localConfigSync.hasMappings", hasMappings)
    ]);
  }
}

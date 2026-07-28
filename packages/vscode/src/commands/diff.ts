import { TextDecoder } from "node:util";
import * as vscode from "vscode";
import { CliClient, CliError } from "../cli/client";
import {
  commandResponseSchema,
  fileDiffResponseSchema,
  type FileDiffResponse
} from "../cli/models";
import { ProjectStore } from "../state/projectStore";
import { FileNode } from "../views/statusTree";
import { resolveProjectFolder, saveDirtyProjectDocuments } from "../workspace/projectResolver";

export class DiffService implements vscode.TextDocumentContentProvider, vscode.Disposable {
  private readonly contents = new Map<string, string>();
  private readonly reviewedRevisions = new Map<string, string>();
  private readonly registration: vscode.Disposable;

  constructor(
    private readonly client: CliClient,
    private readonly store: ProjectStore
  ) {
    this.registration = vscode.workspace.registerTextDocumentContentProvider("local-config-sync", this);
  }

  provideTextDocumentContent(uri: vscode.Uri): string {
    return this.contents.get(uri.toString()) ?? "";
  }

  async show(value?: unknown): Promise<FileDiffResponse | undefined> {
    const file = await resolveFile(value, this.store);
    if (file === undefined) {
      return undefined;
    }
    await saveDirtyProjectDocuments(file.folder);
    try {
      const diff = await vscode.window.withProgress(
        {
          location: vscode.ProgressLocation.Notification,
          title: `Loading diff for ${file.file.localPath}`
        },
        () => this.client.run(
          [
            "diff",
            "--project", file.folder.uri.fsPath,
            "--mapping", file.file.mappingId,
            "--path", file.file.remotePath
          ],
          fileDiffResponseSchema,
          { cwd: file.folder.uri.fsPath, timeoutMs: 60_000 }
        )
      );
      this.reviewedRevisions.set(reviewKey(file), diff.remoteRevision);
      const localUri = this.addContent(diff, "local", diff.localContent);
      const remoteUri = this.addContent(diff, "repository", diff.remoteContent);
      await vscode.commands.executeCommand(
        "vscode.diff",
        localUri,
        remoteUri,
        `Local Config Sync · ${file.file.localPath}`
      );
      return diff;
    } catch (error) {
      await showCommandError(error);
      return undefined;
    }
  }

  async resolve(value: unknown, strategy: "local" | "remote"): Promise<void> {
    const file = await resolveFile(value, this.store, true);
    if (file === undefined) {
      return;
    }
    let expectedRevision = this.reviewedRevisions.get(reviewKey(file));
    if (expectedRevision === undefined) {
      await vscode.window.showWarningMessage("Review the diff before choosing a conflict version.");
      const diff = await this.show(file);
      expectedRevision = diff?.remoteRevision;
      if (expectedRevision === undefined) {
        return;
      }
    }

    const useLocal = strategy === "local";
    const confirmation = await vscode.window.showWarningMessage(
      useLocal
        ? "Publish the reviewed local file to the Repository? Remote content for this mapped file will be replaced."
        : "Replace the local file with the reviewed Repository version? Unpublished local changes will be discarded.",
      { modal: true },
      useLocal ? "Upload Local" : "Download Repository"
    );
    if (confirmation === undefined) {
      return;
    }

    await saveDirtyProjectDocuments(file.folder);
    const args = [
      "resolve",
      "--project", file.folder.uri.fsPath,
      "--mapping", file.file.mappingId,
      "--path", file.file.remotePath,
      "--expected-revision", expectedRevision,
      "--strategy", strategy
    ];
    try {
      const resolved = await vscode.window.withProgress(
        {
          location: vscode.ProgressLocation.Notification,
          title: "Resolving Local Config Sync conflict"
        },
        () => this.runResolveWithSensitiveConfirmation(file.folder, args)
      );
      if (!resolved) {
        return;
      }
      this.reviewedRevisions.delete(reviewKey(file));
      await this.store.refreshAll();
    } catch (error) {
      await showCommandError(error);
    }
  }

  dispose(): void {
    this.registration.dispose();
    this.contents.clear();
    this.reviewedRevisions.clear();
  }

  private addContent(diff: FileDiffResponse, side: string, encoded: string): vscode.Uri {
    const content = decodeText(encoded);
    if (this.contents.size >= 20) {
      const oldest = this.contents.keys().next().value as string | undefined;
      if (oldest !== undefined) {
        this.contents.delete(oldest);
      }
    }
    const uri = vscode.Uri.from({
      scheme: "local-config-sync",
      path: `/${encodeURIComponent(diff.mappingId)}/${side}/${encodeURIComponent(diff.localPath)}`,
      query: `${Date.now()}-${Math.random()}`
    });
    this.contents.set(uri.toString(), content);
    return uri;
  }

  private async runResolveWithSensitiveConfirmation(
    folder: vscode.WorkspaceFolder,
    args: string[]
  ): Promise<boolean> {
    try {
      await this.client.run(args, commandResponseSchema, { cwd: folder.uri.fsPath });
      return true;
    } catch (error) {
      if (!(error instanceof CliError) || error.code !== "unsafe_secret_pattern") {
        throw error;
      }
      const accepted = await confirmSensitivePaths(error.paths);
      if (!accepted) {
        return false;
      }
      await this.client.run([...args, "--allow-sensitive"], commandResponseSchema, { cwd: folder.uri.fsPath });
      return true;
    }
  }
}

export async function resolveFile(
  value: unknown,
  store: ProjectStore,
  conflictsOnly = false
): Promise<FileNode | undefined> {
  if (value instanceof FileNode) {
    return value;
  }
  const folder = await resolveProjectFolder(value);
  if (folder === undefined) {
    return undefined;
  }
  const snapshot = store.get(folder);
  if (snapshot.state !== "ready") {
    await vscode.window.showInformationMessage("Local Config Sync status is not ready yet.");
    return undefined;
  }
  const candidates = snapshot.response.files.filter((file) =>
    conflictsOnly ? file.status === "conflict" : file.status !== "synced"
  );
  const selected = await vscode.window.showQuickPick(
    candidates.map((file) => ({
      label: file.localPath,
      description: file.status.replaceAll("_", " "),
      file
    })),
    { placeHolder: conflictsOnly ? "Choose a conflict to resolve" : "Choose a mapped file to compare" }
  );
  if (selected === undefined) {
    return undefined;
  }
  return new FileNode(
    folder,
    selected.file,
    snapshot.response.mappings.find((mapping) => mapping.id === selected.file.mappingId)
  );
}

export async function confirmSensitivePaths(paths: readonly string[]): Promise<boolean> {
  const listed = paths.length === 0 ? "The selected file" : paths.map((path) => `• ${path}`).join("\n");
  return await vscode.window.showWarningMessage(
    `${listed}\n\nThe file matches a sensitive-file pattern. Local Config Sync is not a secret manager.`,
    { modal: true },
    "Sync Anyway"
  ) !== undefined;
}

export async function showCommandError(error: unknown): Promise<void> {
  const cliError = error instanceof CliError
    ? error
    : new CliError("unexpected_error", error instanceof Error ? error.message : String(error));
  const action = await vscode.window.showErrorMessage(
    `${cliError.message} (${cliError.code})`,
    "Open Logs"
  );
  if (action === "Open Logs") {
    await vscode.commands.executeCommand("localConfigSync.openLogs");
  }
}

function decodeText(encoded: string): string {
  const bytes = Buffer.from(encoded, "base64");
  try {
    return new TextDecoder("utf-8", { fatal: true }).decode(bytes);
  } catch {
    throw new CliError("binary_diff_unsupported", "VS Code text diff cannot display this binary or non-UTF-8 file.");
  }
}

function reviewKey(file: FileNode): string {
  return `${file.folder.uri.toString()}\0${file.file.mappingId}\0${file.file.remotePath}`;
}

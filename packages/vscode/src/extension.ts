import * as path from "node:path";
import * as vscode from "vscode";
import { CliClient } from "./cli/client";
import { CliLocator } from "./cli/locator";
import { CoreCommands } from "./commands/coreCommands";
import { DiffService } from "./commands/diff";
import { SetupService } from "./commands/setup";
import { ProjectStore } from "./state/projectStore";
import { StatusTreeProvider } from "./views/statusTree";

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  const output = vscode.window.createOutputChannel("Local Config Sync", { log: true });
  const locator = new CliLocator(context);
  const client = new CliClient(locator, output);
  const store = new ProjectStore(client);
  const tree = new StatusTreeProvider(store);
  const diffs = new DiffService(client, store);
  const coreCommands = new CoreCommands(client, store, diffs);
  const setup = new SetupService(client, store);
  const saveRefreshTimers = new Map<string, NodeJS.Timeout>();

  context.subscriptions.push(
    output,
    store,
    tree,
    diffs,
    vscode.window.createTreeView("localConfigSync.projects", {
      treeDataProvider: tree,
      showCollapseAll: true
    }),
    vscode.commands.registerCommand("localConfigSync.setup", (value?: unknown) => setup.start(value)),
    vscode.commands.registerCommand("localConfigSync.refresh", async (value?: unknown) => {
      if (isProjectScoped(value)) {
        await store.refresh(value.folder);
      } else {
        await store.refreshAll();
      }
    }),
    vscode.commands.registerCommand("localConfigSync.sync", (value?: unknown) => coreCommands.sync(value)),
    vscode.commands.registerCommand("localConfigSync.authenticate", (value?: unknown) => coreCommands.authenticate(value)),
    vscode.commands.registerCommand("localConfigSync.diff", (value?: unknown) => diffs.show(value)),
    vscode.commands.registerCommand(
      "localConfigSync.resolveLocal",
      (value?: unknown) => diffs.resolve(value, "local")
    ),
    vscode.commands.registerCommand(
      "localConfigSync.resolveRemote",
      (value?: unknown) => diffs.resolve(value, "remote")
    ),
    vscode.commands.registerCommand("localConfigSync.openLogs", () => output.show(true)),
    vscode.workspace.onDidChangeWorkspaceFolders(() => {
      void store.refreshAll();
    }),
    vscode.workspace.onDidChangeConfiguration((event) => {
      if (event.affectsConfiguration("localConfigSync.cliPath")) {
        client.reset();
        void store.refreshAll();
      }
    }),
    vscode.workspace.onDidSaveTextDocument((document) => {
      const folder = vscode.workspace.getWorkspaceFolder(document.uri);
      if (folder === undefined || !isMappedDocument(store, folder, document.uri)) {
        return;
      }
      const key = folder.uri.toString();
      const existing = saveRefreshTimers.get(key);
      if (existing !== undefined) {
        clearTimeout(existing);
      }
      saveRefreshTimers.set(key, setTimeout(() => {
        saveRefreshTimers.delete(key);
        void store.refresh(folder);
      }, 750));
    }),
    {
      dispose: () => {
        for (const timer of saveRefreshTimers.values()) {
          clearTimeout(timer);
        }
        saveRefreshTimers.clear();
      }
    }
  );

  await store.refreshAll();
}

export function deactivate(): void {}

function isProjectScoped(value: unknown): value is { folder: vscode.WorkspaceFolder } {
  return typeof value === "object" && value !== null && "folder" in value;
}

function isMappedDocument(
  store: ProjectStore,
  folder: vscode.WorkspaceFolder,
  uri: vscode.Uri
): boolean {
  const snapshot = store.get(folder);
  if (snapshot.state !== "ready" || uri.scheme !== "file") {
    return false;
  }
  const relative = path.relative(folder.uri.fsPath, uri.fsPath).replaceAll(path.sep, "/");
  return snapshot.response.files.some((file) => file.localPath === relative);
}

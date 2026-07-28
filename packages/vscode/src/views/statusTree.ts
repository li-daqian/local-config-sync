import * as path from "node:path";
import * as vscode from "vscode";
import type {
  FileStatusSummary,
  MappingSummary,
  RepositorySummary
} from "../cli/models";
import { ProjectStore, type ProjectSnapshot } from "../state/projectStore";
import { supportedWorkspaceFolders, type ProjectScoped } from "../workspace/projectResolver";

type StatusNode = ProjectNode | RepositoryNode | FileNode | MessageNode;

export class StatusTreeProvider implements vscode.TreeDataProvider<StatusNode>, vscode.Disposable {
  private readonly changes = new vscode.EventEmitter<StatusNode | undefined>();
  readonly onDidChangeTreeData = this.changes.event;

  constructor(private readonly store: ProjectStore) {
    this.store.onDidChange(() => this.changes.fire(undefined));
  }

  getTreeItem(element: StatusNode): vscode.TreeItem {
    return element;
  }

  getChildren(element?: StatusNode): StatusNode[] {
    if (element === undefined) {
      return supportedWorkspaceFolders().map((folder) => new ProjectNode(folder, this.store.get(folder)));
    }
    if (element instanceof ProjectNode) {
      return projectChildren(element);
    }
    if (element instanceof RepositoryNode) {
      return repositoryChildren(element);
    }
    return [];
  }

  dispose(): void {
    this.changes.dispose();
  }
}

export class ProjectNode extends vscode.TreeItem implements ProjectScoped {
  constructor(
    readonly folder: vscode.WorkspaceFolder,
    readonly snapshot: ProjectSnapshot
  ) {
    super(folder.name, vscode.TreeItemCollapsibleState.Expanded);
    this.contextValue = "localConfigSync.project";
    this.description = snapshotDescription(snapshot);
    this.tooltip = folder.uri.fsPath;
    this.iconPath = new vscode.ThemeIcon(snapshotIcon(snapshot), snapshotColor(snapshot));
  }
}

export class RepositoryNode extends vscode.TreeItem implements ProjectScoped {
  constructor(
    readonly folder: vscode.WorkspaceFolder,
    readonly snapshot: Extract<ProjectSnapshot, { state: "ready" }>,
    readonly repository: RepositorySummary
  ) {
    super(repository.name || repository.id, vscode.TreeItemCollapsibleState.Expanded);
    this.contextValue = "localConfigSync.repository";
    this.description = `${repository.type} · ${stateLabel(repository.state)}`;
    this.tooltip = new vscode.MarkdownString(
      `**${repository.name || repository.id}**\n\n` +
      `Type: \`${repository.type}\`\n\nWorkspace: \`${repository.workspacePath}\``
    );
    this.iconPath = new vscode.ThemeIcon("repo");
  }
}

export class FileNode extends vscode.TreeItem implements ProjectScoped {
  constructor(
    readonly folder: vscode.WorkspaceFolder,
    readonly file: FileStatusSummary,
    readonly mapping: MappingSummary | undefined
  ) {
    super(path.posix.basename(file.localPath), vscode.TreeItemCollapsibleState.None);
    this.description = fileStatusLabel(file.status);
    this.tooltip = new vscode.MarkdownString(
      `Local: \`${file.localPath}\`\n\nRepository: \`${file.remotePath}\``
    );
    this.resourceUri = vscode.Uri.joinPath(folder.uri, ...file.localPath.split("/"));
    this.iconPath = new vscode.ThemeIcon(fileStatusIcon(file.status), fileStatusColor(file.status));
    const resolvable = file.status === "conflict" && mapping?.kind === "file" && mapping.mode === "copy";
    this.contextValue = file.status === "conflict"
      ? `localConfigSync.file.conflict${resolvable ? ".resolvable" : ""}`
      : file.status === "synced"
        ? "localConfigSync.file.synced"
        : "localConfigSync.file.changed";
    if (file.status !== "synced") {
      this.command = {
        command: "localConfigSync.diff",
        title: "View Local Config Diff",
        arguments: [this]
      };
    }
  }
}

class MessageNode extends vscode.TreeItem implements ProjectScoped {
  constructor(
    readonly folder: vscode.WorkspaceFolder,
    label: string,
    icon: string,
    tooltip?: string
  ) {
    super(label, vscode.TreeItemCollapsibleState.None);
    this.iconPath = new vscode.ThemeIcon(icon);
    this.tooltip = tooltip;
    this.contextValue = "localConfigSync.message";
  }
}

function projectChildren(project: ProjectNode): StatusNode[] {
  const snapshot = project.snapshot;
  if (snapshot.state === "loading") {
    return [new MessageNode(project.folder, "Reading mappings and file status…", "loading~spin")];
  }
  if (snapshot.state === "error") {
    return [new MessageNode(
      project.folder,
      snapshot.error.message,
      "error",
      snapshot.error.diagnostics
    )];
  }
  if (snapshot.response.repositories.length === 0) {
    return [new MessageNode(project.folder, "No repositories or mappings configured", "info")];
  }
  return snapshot.response.repositories.map(
    (repository) => new RepositoryNode(project.folder, snapshot, repository)
  );
}

function repositoryChildren(node: RepositoryNode): StatusNode[] {
  const files = node.snapshot.response.files.filter((file) => file.repositoryId === node.repository.id);
  if (files.length === 0) {
    return [new MessageNode(node.folder, "No mapped files", "info")];
  }
  return files.map((file) => new FileNode(
    node.folder,
    file,
    node.snapshot.response.mappings.find((mapping) => mapping.id === file.mappingId)
  ));
}

function snapshotDescription(snapshot: ProjectSnapshot): string {
  if (snapshot.state === "loading") {
    return "Checking";
  }
  if (snapshot.state === "error") {
    return "Failed";
  }
  return stateLabel(snapshot.response.state);
}

function snapshotIcon(snapshot: ProjectSnapshot): string {
  if (snapshot.state === "loading") {
    return "loading~spin";
  }
  if (snapshot.state === "error") {
    return "error";
  }
  return snapshot.response.state === "conflict" ? "warning" : "folder";
}

function snapshotColor(snapshot: ProjectSnapshot): vscode.ThemeColor | undefined {
  if (snapshot.state === "error") {
    return new vscode.ThemeColor("testing.iconFailed");
  }
  if (snapshot.state === "ready" && snapshot.response.state === "conflict") {
    return new vscode.ThemeColor("list.warningForeground");
  }
  return undefined;
}

function stateLabel(state: string): string {
  return state.replaceAll("_", " ").replace(/\b\w/g, (character) => character.toUpperCase());
}

function fileStatusLabel(status: string): string {
  switch (status) {
    case "local_changes":
      return "↑ Upload";
    case "remote_changes":
      return "↓ Download";
    case "conflict":
      return "Conflict";
    case "synced":
      return "Synced";
    default:
      return stateLabel(status);
  }
}

function fileStatusIcon(status: string): string {
  switch (status) {
    case "local_changes":
      return "cloud-upload";
    case "remote_changes":
      return "cloud-download";
    case "conflict":
      return "warning";
    default:
      return "pass";
  }
}

function fileStatusColor(status: string): vscode.ThemeColor | undefined {
  if (status === "conflict") {
    return new vscode.ThemeColor("list.errorForeground");
  }
  if (status === "local_changes" || status === "remote_changes") {
    return new vscode.ThemeColor("list.warningForeground");
  }
  return undefined;
}

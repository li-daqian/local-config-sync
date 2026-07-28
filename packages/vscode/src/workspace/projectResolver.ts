import * as vscode from "vscode";

export interface ProjectScoped {
  readonly folder: vscode.WorkspaceFolder;
}

export async function resolveProjectFolder(value?: unknown): Promise<vscode.WorkspaceFolder | undefined> {
  if (isProjectScoped(value)) {
    return value.folder;
  }

  const activeUri = vscode.window.activeTextEditor?.document.uri;
  if (activeUri !== undefined) {
    const activeFolder = vscode.workspace.getWorkspaceFolder(activeUri);
    if (activeFolder !== undefined) {
      return activeFolder;
    }
  }

  const folders = supportedWorkspaceFolders();
  if (folders.length === 1) {
    return folders[0];
  }
  if (folders.length > 1) {
    return vscode.window.showWorkspaceFolderPick({
      placeHolder: "Choose the project to use with Local Config Sync"
    });
  }
  await vscode.window.showInformationMessage("Open a local or remote workspace folder first.");
  return undefined;
}

export function supportedWorkspaceFolders(): readonly vscode.WorkspaceFolder[] {
  return (vscode.workspace.workspaceFolders ?? []).filter((folder) => folder.uri.scheme === "file");
}

export async function saveDirtyProjectDocuments(folder: vscode.WorkspaceFolder): Promise<void> {
  const documents = vscode.workspace.textDocuments.filter((document) => {
    if (!document.isDirty || document.isUntitled) {
      return false;
    }
    return vscode.workspace.getWorkspaceFolder(document.uri)?.uri.toString() === folder.uri.toString();
  });
  for (const document of documents) {
    if (!await document.save()) {
      throw new Error(`Cannot save ${vscode.workspace.asRelativePath(document.uri, false)}.`);
    }
  }
}

function isProjectScoped(value: unknown): value is ProjectScoped {
  if (typeof value !== "object" || value === null || !("folder" in value)) {
    return false;
  }
  return (value as { folder?: unknown }).folder !== undefined;
}

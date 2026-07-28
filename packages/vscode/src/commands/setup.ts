import { createHash } from "node:crypto";
import * as path from "node:path";
import * as vscode from "vscode";
import { CliClient, CliError } from "../cli/client";
import {
  commandResponseSchema,
  githubRepositoriesResponseSchema,
  mappingPreviewResponseSchema,
  repositoryFilesResponseSchema,
  repositoryListResponseSchema,
  type ConfiguredRepository,
  type GitHubRepository,
  type MappingPreviewResponse
} from "../cli/models";
import { ProjectStore } from "../state/projectStore";
import { resolveProjectFolder, saveDirtyProjectDocuments } from "../workspace/projectResolver";
import { confirmSensitivePaths, showCommandError } from "./diff";

interface MappingPaths {
  remotePath: string;
  localPath: string;
}

export class SetupService {
  constructor(
    private readonly client: CliClient,
    private readonly store: ProjectStore
  ) {}

  async start(value?: unknown): Promise<void> {
    const folder = await resolveProjectFolder(value);
    if (folder === undefined) {
      return;
    }
    try {
      await saveDirtyProjectDocuments(folder);
      await this.client.run(
        ["init", "--default-link-mode", "copy"],
        commandResponseSchema,
        { cwd: folder.uri.fsPath }
      );
      if (!await this.ensureGitHubAuthentication(folder)) {
        return;
      }
      const repositories = await vscode.window.withProgress(
        {
          location: vscode.ProgressLocation.Notification,
          title: "Loading GitHub repositories"
        },
        async () => (await this.client.run(
          ["provider", "github", "repositories"],
          githubRepositoriesResponseSchema,
          { cwd: folder.uri.fsPath, timeoutMs: 120_000 }
        )).repositories
      );
      const selected = await chooseGitHubRepository(repositories);
      if (selected === undefined) {
        return;
      }
      const repositoryId = await this.ensureRepositoryConfigured(folder, selected);
      const remoteFiles = (await this.client.run(
        ["repository", "files", repositoryId],
        repositoryFilesResponseSchema,
        { cwd: folder.uri.fsPath, timeoutMs: 120_000 }
      )).files;
      const paths = await chooseMappingPaths(folder, remoteFiles);
      if (paths === undefined) {
        return;
      }
      const preview = await this.client.run(
        [
          "preview",
          "--project", folder.uri.fsPath,
          "--repository", repositoryId,
          "--source-path", paths.remotePath,
          "--target", paths.localPath,
          "--kind", "file"
        ],
        mappingPreviewResponseSchema,
        { cwd: folder.uri.fsPath, timeoutMs: 60_000 }
      );
      const strategy = await chooseInitialStrategy(preview);
      if (strategy === undefined) {
        return;
      }
      const allowSensitive = preview.sensitivePaths.length > 0
        && await confirmSensitivePaths(preview.sensitivePaths);
      if (preview.sensitivePaths.length > 0 && !allowSensitive) {
        return;
      }
      const linkArgs = [
        "link",
        "--project", folder.uri.fsPath,
        "--repository", repositoryId,
        "--source-path", paths.remotePath,
        "--target", paths.localPath,
        "--kind", "file",
        "--mode", "copy",
        "--initial-strategy", strategy
      ];
      if (allowSensitive) {
        linkArgs.push("--allow-sensitive");
      }
      await vscode.window.withProgress(
        {
          location: vscode.ProgressLocation.Notification,
          title: `Creating mapping for ${paths.localPath}`
        },
        async () => {
          await this.client.run(linkArgs, commandResponseSchema, { cwd: folder.uri.fsPath });
          const syncArgs = ["sync", "--project", folder.uri.fsPath];
          if (allowSensitive) {
            syncArgs.push("--allow-sensitive");
          }
          await this.client.run(syncArgs, commandResponseSchema, { cwd: folder.uri.fsPath });
        }
      );
      await this.store.refreshAll();
      await vscode.window.showInformationMessage(
        `${paths.localPath} is synchronized with ${selected.nameWithOwner}:${paths.remotePath}.`
      );
    } catch (error) {
      await showCommandError(error);
    }
  }

  private async ensureGitHubAuthentication(folder: vscode.WorkspaceFolder): Promise<boolean> {
    try {
      await this.client.run(
        ["provider", "github", "auth"],
        commandResponseSchema,
        { cwd: folder.uri.fsPath, timeoutMs: 60_000 }
      );
      return true;
    } catch (error) {
      if (!(error instanceof CliError) || error.code !== "auth_failed") {
        throw error;
      }
      const action = await vscode.window.showWarningMessage(
        "GitHub CLI authentication is required. Credentials remain managed by GitHub CLI.",
        "Open Login Terminal"
      );
      if (action === undefined) {
        return false;
      }
      const terminal = vscode.window.createTerminal({
        name: "Local Config Sync · GitHub Login",
        cwd: folder.uri
      });
      terminal.show();
      terminal.sendText("gh auth login --hostname github.com --git-protocol https --web", true);
      await vscode.window.showInformationMessage("Complete GitHub login in the terminal, then run Add Mapping again.");
      return false;
    }
  }

  private async ensureRepositoryConfigured(
    folder: vscode.WorkspaceFolder,
    selected: GitHubRepository
  ): Promise<string> {
    const configured = (await this.client.run(
      ["repository", "list"],
      repositoryListResponseSchema,
      { cwd: folder.uri.fsPath, timeoutMs: 30_000 }
    )).repositories;
    const existing = configured.find((repository) => matchesRepository(repository, selected));
    if (existing !== undefined) {
      return existing.id;
    }
    const baseId = githubRepositoryId(selected.nameWithOwner);
    const repositoryId = configured.some((repository) => repository.id === baseId)
      ? `${baseId}-${createHash("sha256").update(selected.nameWithOwner).digest("hex").slice(0, 8)}`
      : baseId;
    await this.client.run(
      [
        "repository", "add", "git",
        "--id", repositoryId,
        "--name", selected.nameWithOwner,
        "--url", selected.url,
        "--branch", selected.defaultBranch
      ],
      commandResponseSchema,
      { cwd: folder.uri.fsPath }
    );
    return repositoryId;
  }
}

async function chooseGitHubRepository(
  repositories: readonly GitHubRepository[]
): Promise<GitHubRepository | undefined> {
  if (repositories.length === 0) {
    await vscode.window.showInformationMessage("No accessible GitHub repositories were found.");
    return undefined;
  }
  return (await vscode.window.showQuickPick(
    repositories.map((repository) => ({
      label: repository.nameWithOwner,
      description: repository.private ? "Private" : "Public",
      detail: repository.url,
      repository
    })),
    {
      placeHolder: "Choose a GitHub Repository",
      matchOnDescription: true,
      matchOnDetail: true
    }
  ))?.repository;
}

async function chooseMappingPaths(
  folder: vscode.WorkspaceFolder,
  remoteFiles: readonly string[]
): Promise<MappingPaths | undefined> {
  const createLabel = "$(new-file) Create a Repository file from a local file";
  const choice = await vscode.window.showQuickPick(
    [
      { label: createLabel, create: true as const },
      ...remoteFiles.map((remotePath) => ({
        label: remotePath,
        description: "Existing Repository file",
        create: false as const,
        remotePath
      }))
    ],
    {
      placeHolder: "Choose an existing Repository file or create one from a project file",
      matchOnDescription: true
    }
  );
  if (choice === undefined) {
    return undefined;
  }
  if (choice.create) {
    return chooseLocalUploadPaths(folder);
  }
  return chooseLocalDownloadPath(folder, choice.remotePath);
}

async function chooseLocalUploadPaths(folder: vscode.WorkspaceFolder): Promise<MappingPaths | undefined> {
  const selected = (await vscode.window.showOpenDialog({
    title: "Choose a local configuration file",
    defaultUri: folder.uri,
    canSelectFiles: true,
    canSelectFolders: false,
    canSelectMany: false
  }))?.[0];
  if (selected === undefined) {
    return undefined;
  }
  const localPath = projectRelativePath(folder, selected);
  if (localPath === undefined) {
    await vscode.window.showErrorMessage("Choose a file inside the selected project.");
    return undefined;
  }
  const remotePath = await vscode.window.showInputBox({
    title: "Repository file location",
    prompt: "Enter a safe relative path inside the selected Repository",
    value: `${folder.name}/${localPath}`,
    validateInput: validateRelativePath
  });
  return remotePath === undefined ? undefined : { localPath, remotePath: normalizeRelativePath(remotePath) };
}

async function chooseLocalDownloadPath(
  folder: vscode.WorkspaceFolder,
  remotePath: string
): Promise<MappingPaths | undefined> {
  const selected = (await vscode.window.showOpenDialog({
    title: "Choose the project folder for the Repository file",
    defaultUri: folder.uri,
    canSelectFiles: false,
    canSelectFolders: true,
    canSelectMany: false
  }))?.[0];
  if (selected === undefined) {
    return undefined;
  }
  const directory = projectRelativePath(folder, selected, true);
  if (directory === undefined) {
    await vscode.window.showErrorMessage("Choose a folder inside the selected project.");
    return undefined;
  }
  const fileName = path.posix.basename(normalizeRelativePath(remotePath));
  if (fileName === "." || fileName === ".." || fileName === "") {
    await vscode.window.showErrorMessage("The Repository file path does not have a valid file name.");
    return undefined;
  }
  return {
    remotePath,
    localPath: directory === "" ? fileName : `${directory}/${fileName}`
  };
}

async function chooseInitialStrategy(
  preview: MappingPreviewResponse
): Promise<"auto" | "local" | "remote" | undefined> {
  switch (preview.state) {
    case "remote_only":
      return "remote";
    case "local_only":
      return "local";
    case "identical":
      return "auto";
    case "conflict": {
      await vscode.commands.executeCommand(
        "vscode.diff",
        vscode.Uri.file(preview.targetAbsolutePath),
        vscode.Uri.file(preview.sourceAbsolutePath),
        `Local Config Sync · Initial Conflict · ${preview.targetPath}`
      );
      const selected = await vscode.window.showWarningMessage(
        "The local and Repository files differ. Choose the version that should become the initial synchronized version.",
        { modal: true },
        "Use Local",
        "Use Repository"
      );
      return selected === "Use Local" ? "local" : selected === "Use Repository" ? "remote" : undefined;
    }
    default:
      throw new CliError("invalid_arguments", "Neither the local nor Repository file exists.");
  }
}

export function githubRepositoryId(nameWithOwner: string): string {
  return `github-${nameWithOwner.toLowerCase().replace(/[^a-z0-9._-]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 56)}`;
}

function matchesRepository(repository: ConfiguredRepository, github: GitHubRepository): boolean {
  return repository.type === "git"
    && (repository.options.remoteUrl === github.url || repository.options.remoteUrl === github.sshUrl);
}

function projectRelativePath(
  folder: vscode.WorkspaceFolder,
  selected: vscode.Uri,
  allowRoot = false
): string | undefined {
  if (selected.scheme !== "file") {
    return undefined;
  }
  const relative = path.relative(folder.uri.fsPath, selected.fsPath);
  if (relative === "" && allowRoot) {
    return "";
  }
  if (relative === "" || relative === ".." || relative.startsWith(`..${path.sep}`) || path.isAbsolute(relative)) {
    return undefined;
  }
  return normalizeRelativePath(relative);
}

function validateRelativePath(value: string): string | undefined {
  const normalized = normalizeRelativePath(value.trim());
  if (
    normalized === ""
    || normalized === "."
    || normalized === ".."
    || normalized.startsWith("../")
    || path.posix.isAbsolute(normalized)
  ) {
    return "Enter a relative path inside the Repository.";
  }
  return undefined;
}

function normalizeRelativePath(value: string): string {
  return value.replaceAll("\\", "/").replace(/^\.\/+/, "");
}

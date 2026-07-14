import { realpath } from "node:fs/promises";
import { join, resolve } from "node:path";
import { LocalConfigError } from "./errors.js";
import type { GitRepositoryConfig, LocalFolderRepositoryConfig, RepositoryConfig, RepositoryType } from "./model.js";
import type { AppPaths } from "./paths.js";
import type { RepositoryDriver } from "./repository-driver.js";
import { GitDriver } from "./drivers/git-driver.js";
import { LocalFolderDriver } from "./drivers/local-folder-driver.js";
import type { Storage } from "./storage.js";

const ID_PATTERN = /^[a-z0-9][a-z0-9._-]{0,63}$/;

export class RepositoryRegistry {
  private readonly git = new GitDriver();
  private readonly localFolder = new LocalFolderDriver();
  constructor(private readonly storage: Storage, private readonly paths: AppPaths) {}

  async list(): Promise<RepositoryConfig[]> { return (await this.storage.readRepositories()).repositories.map(validateRepository); }
  async get(id: string): Promise<RepositoryConfig> {
    const repository = (await this.list()).find((item) => item.id === id);
    if (!repository) throw new LocalConfigError("repository_not_found", `Repository not found: ${id}`, { repositoryId: id });
    return repository;
  }

  async addGit(input: { id: string; name?: string; url: string; branch?: string }): Promise<GitRepositoryConfig> {
    validateId(input.id);
    const file = await this.storage.readRepositories();
    if (file.repositories.some((item) => item.id === input.id)) throw new LocalConfigError("invalid_arguments", `Repository id already exists: ${input.id}`);
    if (!input.url.trim()) throw new LocalConfigError("invalid_arguments", "Git remote URL is required");
    const repository: GitRepositoryConfig = {
      id: input.id,
      name: input.name?.trim() || input.id,
      type: "git",
      workspacePath: join(this.paths.workspaces, input.id),
      options: { remoteUrl: input.url.trim(), branch: input.branch?.trim() || "main" },
    };
    await this.git.prepare(repository);
    file.repositories.push(repository);
    await this.storage.writeRepositories(file);
    return repository;
  }

  async addLocalFolder(input: { id: string; name?: string; path: string }): Promise<LocalFolderRepositoryConfig> {
    validateId(input.id);
    const file = await this.storage.readRepositories();
    if (file.repositories.some((item) => item.id === input.id)) throw new LocalConfigError("invalid_arguments", `Repository id already exists: ${input.id}`);
    const folder = resolve(input.path);
    const repository: LocalFolderRepositoryConfig = {
      id: input.id,
      name: input.name?.trim() || input.id,
      type: "local-folder",
      workspacePath: folder,
      options: { path: folder },
    };
    await this.localFolder.prepare(repository);
    repository.workspacePath = await realpath(folder);
    repository.options.path = repository.workspacePath;
    file.repositories.push(repository);
    await this.storage.writeRepositories(file);
    return repository;
  }

  driver<T extends RepositoryConfig>(repository: T): RepositoryDriver<T> {
    return (repository.type === "git" ? this.git : this.localFolder) as RepositoryDriver<T>;
  }

  async remove(id: string): Promise<RepositoryConfig> {
    const mappings = (await this.storage.readMappings()).mappings.filter((mapping) => mapping.repositoryId === id);
    if (mappings.length) throw new LocalConfigError("invalid_arguments", "Repository is still referenced by mappings", { repositoryId: id, mappingIds: mappings.map((item) => item.id) });
    const file = await this.storage.readRepositories();
    const index = file.repositories.findIndex((item) => item.id === id);
    if (index < 0) throw new LocalConfigError("repository_not_found", `Repository not found: ${id}`);
    const [removed] = file.repositories.splice(index, 1);
    await this.storage.writeRepositories(file);
    await this.storage.removeState(id);
    return removed!;
  }
}

function validateId(id: string): void {
  if (!ID_PATTERN.test(id)) throw new LocalConfigError("invalid_arguments", "Repository id must contain only lowercase letters, numbers, dots, underscores, and dashes", { id });
}

function validateRepository(value: RepositoryConfig): RepositoryConfig {
  if (!value || !ID_PATTERN.test(value.id) || !value.name || !value.workspacePath) throw new LocalConfigError("not_configured", "Invalid repository registry entry");
  if (value.type === "git") {
    if (!value.options?.remoteUrl || !value.options.branch) throw new LocalConfigError("not_configured", `Invalid Git repository options: ${value.id}`);
    return value;
  }
  if (value.type === "local-folder") {
    if (!value.options?.path) throw new LocalConfigError("not_configured", `Invalid local-folder repository options: ${value.id}`);
    return value;
  }
  throw new LocalConfigError("unsupported_capability", `Unsupported repository type: ${(value as { type: RepositoryType }).type}`);
}

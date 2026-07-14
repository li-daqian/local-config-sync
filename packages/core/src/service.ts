import { rm } from "node:fs/promises";
import { join } from "node:path";
import { LocalConfigError } from "./errors.js";
import { materialize, removeMaterialized } from "./file-linker.js";
import { listFiles, resolveInside, snapshotDirectory, snapshotsEqual } from "./files.js";
import { authenticateGit, type GitAuthMethod } from "./git-auth.js";
import { addExclude, hasExclude, removeExclude } from "./ignore-manager.js";
import { withRepositoryLock } from "./lock.js";
import { MappingManager } from "./mapping-manager.js";
import type { DiagnosticResult, GitRepositoryConfig, LinkMode, Mapping, MappingSummary, RepositoryConfig, RepositoryState, RepositorySummary, SyncState } from "./model.js";
import { getAppPaths, type AppPaths } from "./paths.js";
import { resolveProject } from "./project-resolver.js";
import { RepositoryRegistry } from "./repository-registry.js";
import { assertNoSensitive, scanSensitive } from "./sensitive-files.js";
import { Storage } from "./storage.js";

export interface SyncOptions { project?: string; repositoryId?: string; allowSensitive?: boolean }

export class LocalConfigService {
  readonly paths: AppPaths;
  readonly storage: Storage;
  readonly repositories: RepositoryRegistry;
  readonly mappings: MappingManager;

  constructor(home?: string) {
    this.paths = getAppPaths(home);
    this.storage = new Storage(this.paths);
    this.repositories = new RepositoryRegistry(this.storage, this.paths);
    this.mappings = new MappingManager(this.storage);
  }

  async init(mode: LinkMode = "symlink") { return await this.storage.initialize(mode); }

  async link(input: { project: string; repositoryId: string; sourcePath: string; targetPath: string; mode?: LinkMode; id?: string }): Promise<Mapping> {
    await this.storage.initialize();
    const project = await resolveProject(input.project);
    const repository = await this.repositories.get(input.repositoryId);
    await this.repositories.driver(repository).prepare(repository);
    const source = resolveInside(repository.workspacePath, input.sourcePath);
    const target = resolveInside(project.root, input.targetPath);
    const mode = input.mode ?? (await this.storage.readConfig()).defaultLinkMode;
    await materialize(source, target, mode);
    try {
      await addExclude(project.excludePath, input.targetPath);
      return await this.mappings.add({ ...input, projectPath: project.root, mode });
    } catch (error) {
      await rm(target, { recursive: true, force: true });
      throw error;
    }
  }

  async unlink(projectPath: string, options: { keepFiles?: boolean; keepExclude?: boolean } = {}): Promise<Mapping[]> {
    const project = await resolveProject(projectPath);
    const mappings = await this.mappings.forProject(project.root);
    for (const mapping of mappings) {
      const target = resolveInside(project.root, mapping.targetPath);
      await removeMaterialized(target, mapping.mode, Boolean(options.keepFiles));
      if (!options.keepExclude) await removeExclude(project.excludePath, mapping.targetPath);
    }
    return await this.mappings.remove(mappings.map((mapping) => mapping.id));
  }

  async authenticate(repositoryId: string, method: GitAuthMethod) {
    const repository = await this.repositories.get(repositoryId);
    if (repository.type !== "git") throw new LocalConfigError("invalid_arguments", "Authentication is only applicable to Git repositories", { repositoryId });
    return await authenticateGit(repository, method);
  }

  async authenticateUrl(remoteUrl: string, method: GitAuthMethod) {
    const repository: GitRepositoryConfig = {
      id: "authentication-check",
      name: "Authentication Check",
      type: "git",
      workspacePath: this.paths.workspaces,
      options: { remoteUrl, branch: "main" },
    };
    return await authenticateGit(repository, method);
  }

  async sync(options: SyncOptions, operation: "pull" | "push" | "sync" = "sync") {
    const groups = await this.resolveScope(options);
    const results = [];
    for (const [repository, mappings] of groups) {
      results.push(await withRepositoryLock(join(this.paths.locks, `${repository.id}.lock`), async () => {
        const driver = this.repositories.driver(repository);
        await driver.prepare(repository);
        let state = await this.storage.readState(repository.id);
        const scopes = mappings.map((mapping) => mapping.sourcePath);
        const before = await driver.inspect({ repository, scopes, expectedRevision: state.remoteRevision });
        const baseline = state.remoteRevision;
        if ((operation === "pull" || operation === "sync") && before.remoteChanged) {
          // A missing baseline on first use trusts the current clone; later changes are conditional.
          if (baseline && before.localChanges.length) throw new LocalConfigError("conflict", "Local and remote repository changed since the last sync", { repositoryId: repository.id, paths: before.localChanges });
          const pulled = await driver.pull({ repository, scopes, expectedRevision: baseline });
          state.remoteRevision = pulled.remoteRevision;
          await this.reconcileCopiesFromWorkspace(mappings, state);
        }
        if (operation === "push" || operation === "sync") {
          await this.reconcileCopiesToWorkspace(mappings, state);
          assertNoSensitive(await scanSensitive(repository.workspacePath, scopes), options.allowSensitive);
          const expectedRevision = state.remoteRevision ?? (await driver.inspect({ repository, scopes })).remoteRevision;
          const projectNames = [...new Set(mappings.map((mapping) => mapping.projectName))].join(",");
          const pushed = await driver.push({ repository, scopes, expectedRevision }, `chore(${projectNames}): sync local config`);
          state.remoteRevision = pushed.remoteRevision;
        }
        state = await this.updateState(repository, mappings, state);
        return { repositoryId: repository.id, state: "synced" as const, remoteRevision: state.remoteRevision, lastSyncTime: state.lastSyncTime };
      }));
    }
    return results;
  }

  async status(projectPath: string) {
    const project = await resolveProject(projectPath);
    const mappings = await this.mappings.forProject(project.root);
    if (!mappings.length) return { projectPath: project.root, state: "not_configured" as const, repositories: [], mappings: [] };
    const repositorySummaries: RepositorySummary[] = [];
    let aggregate: SyncState = "synced";
    for (const repositoryId of [...new Set(mappings.map((mapping) => mapping.repositoryId))]) {
      const repository = await this.repositories.get(repositoryId);
      const selected = mappings.filter((mapping) => mapping.repositoryId === repositoryId);
      const status = await this.repositories.driver(repository).inspect({ repository, scopes: selected.map((mapping) => mapping.sourcePath) });
      if (status.state !== "synced") aggregate = "pending";
      repositorySummaries.push({ id: repository.id, name: repository.name, type: repository.type, state: status.state, workspacePath: repository.workspacePath, remoteRevision: status.remoteRevision, capabilities: status.capabilities });
    }
    const mappingSummaries: MappingSummary[] = await Promise.all(mappings.map(async (mapping) => ({
      id: mapping.id,
      repositoryId: mapping.repositoryId,
      sourcePath: mapping.sourcePath,
      targetPath: mapping.targetPath,
      mode: mapping.mode,
      mappedFiles: (await listFiles(resolveInside(project.root, mapping.targetPath))).map((file) => `${mapping.targetPath}/${file}`),
      excludeConfigured: await hasExclude(project.excludePath, mapping.targetPath),
    })));
    const states = await Promise.all(repositorySummaries.map((repository) => this.storage.readState(repository.id)));
    return { projectPath: project.root, state: aggregate, repositories: repositorySummaries, mappings: mappingSummaries, lastSyncTime: states.map((state) => state.lastSyncTime).filter(Boolean).sort().at(-1) };
  }

  async doctor(projectPath?: string, repositoryId?: string): Promise<DiagnosticResult> {
    const checks = [];
    if (repositoryId) {
      const repository = await this.repositories.get(repositoryId);
      return await this.repositories.driver(repository).doctor(repository);
    }
    if (projectPath) {
      const project = await resolveProject(projectPath);
      checks.push({ name: "git-project", ok: true, message: `Git project found at ${project.root}` });
      const mappings = await this.mappings.forProject(project.root);
      checks.push({ name: "mappings", ok: mappings.length > 0, message: mappings.length ? `${mappings.length} mapping(s) configured` : "No mappings configured", remediation: mappings.length ? undefined : "Run local-config link" });
      for (const repositoryId of [...new Set(mappings.map((mapping) => mapping.repositoryId))]) {
        const repository = await this.repositories.get(repositoryId);
        checks.push(...(await this.repositories.driver(repository).doctor(repository)).checks.map((check) => ({ ...check, name: `${repositoryId}:${check.name}` })));
      }
    } else {
      checks.push({ name: "configuration", ok: (await this.repositories.list()).length > 0, message: `${(await this.repositories.list()).length} repository/repositories configured` });
    }
    return { ok: checks.every((check) => check.ok), checks };
  }

  private async resolveScope(options: SyncOptions): Promise<Array<[RepositoryConfig, Mapping[]]>> {
    if (Boolean(options.project) === Boolean(options.repositoryId)) throw new LocalConfigError("invalid_arguments", "Specify exactly one of --project or --repository");
    if (options.repositoryId) {
      const repository = await this.repositories.get(options.repositoryId);
      const mappings = await this.mappings.forRepository(repository.id);
      if (!mappings.length) throw new LocalConfigError("not_configured", "Repository has no mappings", { repositoryId: repository.id });
      return [[repository, mappings]];
    }
    const project = await resolveProject(options.project!);
    const mappings = await this.mappings.forProject(project.root);
    if (!mappings.length) throw new LocalConfigError("not_configured", "Project has no mappings", { projectPath: project.root });
    const result: Array<[RepositoryConfig, Mapping[]]> = [];
    for (const id of [...new Set(mappings.map((mapping) => mapping.repositoryId))]) result.push([await this.repositories.get(id), mappings.filter((mapping) => mapping.repositoryId === id)]);
    return result;
  }

  private async reconcileCopiesFromWorkspace(mappings: Mapping[], state: RepositoryState): Promise<void> {
    for (const mapping of mappings.filter((item) => item.mode === "copy")) {
      const repository = await this.repositories.get(mapping.repositoryId);
      const source = resolveInside(repository.workspacePath, mapping.sourcePath);
      const target = resolveInside(mapping.projectPath, mapping.targetPath);
      const targetSnapshot = await snapshotDirectory(target, mapping.sourcePath);
      const locallyChanged = Object.keys({ ...targetSnapshot, ...state.files }).some((path) => path.startsWith(`${mapping.sourcePath}/`) && !snapshotsEqual(targetSnapshot[path], state.files[path]));
      if (locallyChanged) throw new LocalConfigError("conflict", "Copy target has local changes while repository changed", { mappingId: mapping.id });
      await materialize(source, target, "copy", true);
    }
  }

  private async reconcileCopiesToWorkspace(mappings: Mapping[], state: RepositoryState): Promise<void> {
    for (const mapping of mappings.filter((item) => item.mode === "copy")) {
      const repository = await this.repositories.get(mapping.repositoryId);
      const source = resolveInside(repository.workspacePath, mapping.sourcePath);
      const target = resolveInside(mapping.projectPath, mapping.targetPath);
      const workspaceSnapshot = await snapshotDirectory(source, mapping.sourcePath);
      const externallyChanged = Object.keys({ ...workspaceSnapshot, ...state.files }).some((path) => path.startsWith(`${mapping.sourcePath}/`) && !snapshotsEqual(workspaceSnapshot[path], state.files[path]));
      const targetSnapshot = await snapshotDirectory(target, mapping.sourcePath);
      const locallyChanged = Object.keys({ ...targetSnapshot, ...state.files }).some((path) => path.startsWith(`${mapping.sourcePath}/`) && !snapshotsEqual(targetSnapshot[path], state.files[path]));
      if (externallyChanged && locallyChanged && JSON.stringify(workspaceSnapshot) !== JSON.stringify(targetSnapshot)) throw new LocalConfigError("conflict", "Both copy target and repository changed", { mappingId: mapping.id });
      if (locallyChanged) await materialize(target, source, "copy", true);
    }
  }

  private async updateState(repository: RepositoryConfig, mappings: Mapping[], state: RepositoryState): Promise<RepositoryState> {
    const files = { ...state.files };
    for (const mapping of mappings) {
      for (const path of Object.keys(files)) {
        if (path === mapping.sourcePath || path.startsWith(`${mapping.sourcePath}/`)) delete files[path];
      }
      Object.assign(files, await snapshotDirectory(resolveInside(repository.workspacePath, mapping.sourcePath), mapping.sourcePath));
    }
    const next = { ...state, files, lastSyncTime: new Date().toISOString(), lastError: undefined };
    await this.storage.writeState(next);
    return next;
  }
}

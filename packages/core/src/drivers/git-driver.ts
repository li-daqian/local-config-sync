import { mkdir, readdir } from "node:fs/promises";
import { basename } from "node:path";
import { LocalConfigError } from "../errors.js";
import { diagnoseGitAuth } from "../git-auth.js";
import type { DiagnosticCheck, GitRepositoryConfig, RepositoryStatus } from "../model.js";
import { runProcess } from "../process.js";
import type { DriverContext, GitDriverContract, PullResult, PushResult } from "../repository-driver.js";

const capabilities = { history: true, conditionalWrite: true, atomicPublish: true } as const;

function lines(value: string): string[] { return value ? value.split(/\r?\n/).filter(Boolean) : []; }
function normalizeStatusPath(line: string): string {
  const raw = line.slice(3).trim();
  const renamed = raw.includes(" -> ") ? raw.split(" -> ").at(-1)! : raw;
  return renamed.replace(/^"|"$/g, "");
}
function inScope(path: string, scopes: string[]): boolean { return scopes.some((scope) => path === scope || path.startsWith(`${scope}/`)); }

async function remoteRevision(repository: GitRepositoryConfig): Promise<string | undefined> {
  const result = await runProcess("git", ["ls-remote", repository.options.remoteUrl, `refs/heads/${repository.options.branch}`], { allowFailure: true });
  if (result.exitCode !== 0) {
    const auth = /authentication|permission denied|could not read|access denied/i.test(result.stderr);
    throw new LocalConfigError(auth ? "auth_failed" : "repository_failed", "Cannot read Git remote revision", { repositoryId: repository.id });
  }
  return result.stdout.split(/\s+/)[0] || undefined;
}

async function localRevision(repository: GitRepositoryConfig): Promise<string | undefined> {
  const result = await runProcess("git", ["rev-parse", "HEAD"], { cwd: repository.workspacePath, allowFailure: true });
  return result.exitCode === 0 ? result.stdout : undefined;
}

export class GitDriver implements GitDriverContract {
  readonly type = "git" as const;
  readonly capabilities = capabilities;

  async prepare(repository: GitRepositoryConfig): Promise<void> {
    await mkdir(repository.workspacePath, { recursive: true });
    const entries = await readdir(repository.workspacePath);
    if (entries.length === 0) {
      const clone = await runProcess("git", ["clone", "--origin", "origin", "--branch", repository.options.branch, "--single-branch", repository.options.remoteUrl, repository.workspacePath], { allowFailure: true });
      if (clone.exitCode !== 0) {
        // An empty remote has no branch yet; initialize it without inventing a remote commit.
        const emptyRemote = /empty repository|remote branch .* not found/i.test(`${clone.stdout}\n${clone.stderr}`);
        if (!emptyRemote) {
          const auth = /authentication|permission denied|could not read|access denied/i.test(clone.stderr);
          throw new LocalConfigError(auth ? "auth_failed" : "repository_failed", `Cannot clone Git repository: ${clone.stderr}`, { repositoryId: repository.id });
        }
        await runProcess("git", ["init", "--initial-branch", repository.options.branch], { cwd: repository.workspacePath });
        await runProcess("git", ["remote", "add", "origin", repository.options.remoteUrl], { cwd: repository.workspacePath });
      }
      return;
    }
    const inside = await runProcess("git", ["rev-parse", "--is-inside-work-tree"], { cwd: repository.workspacePath, allowFailure: true });
    if (inside.stdout !== "true") throw new LocalConfigError("repository_failed", "Managed Git workspace is not a Git repository", { workspacePath: repository.workspacePath });
    const url = await runProcess("git", ["remote", "get-url", "origin"], { cwd: repository.workspacePath, allowFailure: true });
    if (url.exitCode !== 0 || url.stdout !== repository.options.remoteUrl) {
      throw new LocalConfigError("repository_failed", "Managed workspace origin does not match configured remote", { configured: repository.options.remoteUrl, actual: url.stdout });
    }
  }

  async inspect(context: DriverContext<GitRepositoryConfig>): Promise<RepositoryStatus> {
    await this.prepare(context.repository);
    const [remote, local, dirty] = await Promise.all([
      remoteRevision(context.repository),
      localRevision(context.repository),
      runProcess("git", ["status", "--porcelain=v1", "--untracked-files=all"], { cwd: context.repository.workspacePath }),
    ]);
    const localChanges = lines(dirty.stdout).map(normalizeStatusPath);
    return {
      state: localChanges.length || remote !== local ? "pending" : "synced",
      remoteRevision: remote,
      localChanges,
      remoteChanged: remote !== local,
      capabilities,
    };
  }

  async pull(context: DriverContext<GitRepositoryConfig>): Promise<PullResult> {
    const before = await localRevision(context.repository);
    const remote = await remoteRevision(context.repository);
    if (!remote || remote === before) return { remoteRevision: remote ?? before, changed: false };
    const dirty = lines((await runProcess("git", ["status", "--porcelain=v1", "--untracked-files=all"], { cwd: context.repository.workspacePath })).stdout).map(normalizeStatusPath);
    if (dirty.length) {
      throw new LocalConfigError("conflict", "Remote changed while the managed workspace has local changes", { repositoryId: context.repository.id, paths: dirty });
    }
    await runProcess("git", ["fetch", "--prune", "origin", context.repository.options.branch], { cwd: context.repository.workspacePath });
    const merge = await runProcess("git", ["merge", "--ff-only", `origin/${context.repository.options.branch}`], { cwd: context.repository.workspacePath, allowFailure: true });
    if (merge.exitCode !== 0) throw new LocalConfigError("conflict", "Git history diverged; resolve it manually in the managed workspace", { repositoryId: context.repository.id, workspacePath: context.repository.workspacePath });
    return { remoteRevision: await localRevision(context.repository), changed: true };
  }

  async push(context: DriverContext<GitRepositoryConfig>, commitMessage: string): Promise<PushResult> {
    const currentRemote = await remoteRevision(context.repository);
    if (currentRemote !== context.expectedRevision) {
      throw new LocalConfigError("conflict", "Remote revision changed before push", { repositoryId: context.repository.id, expectedRevision: context.expectedRevision, remoteRevision: currentRemote });
    }
    const status = lines((await runProcess("git", ["status", "--porcelain=v1", "--untracked-files=all"], { cwd: context.repository.workspacePath })).stdout);
    const changed = status.map(normalizeStatusPath);
    const outside = changed.filter((path) => !inScope(path, context.scopes));
    if (outside.length) throw new LocalConfigError("repository_dirty_outside_scope", "Repository has changes outside the requested mapping scope", { repositoryId: context.repository.id, paths: outside });
    if (!changed.length) return { remoteRevision: currentRemote, changed: false };
    await runProcess("git", ["add", "--all", "--", ...context.scopes], { cwd: context.repository.workspacePath });
    const staged = await runProcess("git", ["diff", "--cached", "--quiet"], { cwd: context.repository.workspacePath, allowFailure: true });
    if (staged.exitCode === 0) return { remoteRevision: currentRemote, changed: false };
    const identity = await runProcess("git", ["config", "user.email"], { cwd: context.repository.workspacePath, allowFailure: true });
    if (identity.exitCode !== 0) throw new LocalConfigError("repository_failed", "Git author identity is not configured. Set user.name and user.email.", { workspacePath: context.repository.workspacePath });
    await runProcess("git", ["commit", "-m", commitMessage], { cwd: context.repository.workspacePath });
    const pushed = await runProcess("git", ["push", "origin", `HEAD:refs/heads/${context.repository.options.branch}`], { cwd: context.repository.workspacePath, allowFailure: true });
    if (pushed.exitCode !== 0) {
      const auth = /authentication|permission denied|could not read|access denied/i.test(pushed.stderr);
      const rejected = /rejected|fetch first|non-fast-forward/i.test(pushed.stderr);
      throw new LocalConfigError(auth ? "auth_failed" : rejected ? "conflict" : "repository_failed", `Git push failed: ${pushed.stderr}`, { repositoryId: context.repository.id });
    }
    return { remoteRevision: await localRevision(context.repository), changed: true };
  }

  async doctor(repository: GitRepositoryConfig) {
    const checks: DiagnosticCheck[] = [];
    const git = await runProcess("git", ["--version"], { allowFailure: true });
    checks.push({ name: "git-cli", ok: git.exitCode === 0, message: git.exitCode === 0 ? git.stdout : "Git CLI is not available", remediation: git.exitCode === 0 ? undefined : "Install Git and ensure it is on PATH" });
    if (git.exitCode === 0) checks.push(...await diagnoseGitAuth(repository));
    const usable = await runProcess("git", ["rev-parse", "--is-inside-work-tree"], { cwd: repository.workspacePath, allowFailure: true });
    checks.push({ name: "workspace", ok: usable.stdout === "true", message: usable.stdout === "true" ? "Managed workspace is valid" : `Managed workspace is not ready: ${basename(repository.workspacePath)}` });
    return { ok: checks.every((check) => check.ok || check.name === "github-cli-auth"), checks };
  }
}

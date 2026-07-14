import { LocalConfigError } from "./errors.js";
import type { DiagnosticCheck, GitRepositoryConfig } from "./model.js";
import { runProcess } from "./process.js";

export type GitAuthMethod = "auto" | "ssh" | "credential" | "gh";

function hostFromUrl(url: string): string | undefined {
  const scp = /^[^@]+@([^:]+):/.exec(url);
  if (scp?.[1]) return scp[1];
  try { return new URL(url).hostname; } catch { return undefined; }
}

export async function diagnoseGitAuth(repository: GitRepositoryConfig): Promise<DiagnosticCheck[]> {
  const host = hostFromUrl(repository.options.remoteUrl);
  const checks: DiagnosticCheck[] = [];
  if (host === "github.com") {
    const gh = await runProcess("gh", ["auth", "status", "--hostname", host], { allowFailure: true })
      .catch(() => ({ stdout: "", stderr: "GitHub CLI is not installed", exitCode: 127 }));
    checks.push({
      name: "github-cli-auth",
      ok: gh.exitCode === 0,
      message: gh.exitCode === 0 ? "GitHub CLI authentication is available" : "GitHub CLI authentication is not available; SSH or Git credential may still work",
      remediation: gh.exitCode === 0 ? undefined : "Run: gh auth login",
    });
  }
  const remote = await runProcess("git", ["ls-remote", "--exit-code", repository.options.remoteUrl, "HEAD"], { allowFailure: true });
  // Empty repositories return exit 2 with --exit-code even when authentication succeeds.
  const emptyRemote = remote.exitCode === 2 && !/authentication|permission denied|could not read|access denied|not found/i.test(remote.stderr);
  checks.push({
    name: "git-remote-auth",
    ok: remote.exitCode === 0 || emptyRemote,
    message: remote.exitCode === 0 || emptyRemote ? "Git remote authentication succeeded" : "Git could not authenticate to the remote",
    remediation: remote.exitCode === 0 || emptyRemote ? undefined : (/^(git@|ssh:\/\/)/.test(repository.options.remoteUrl)
      ? `Verify your SSH key with: ssh -T git@${host ?? "<host>"}`
      : "Configure a Git credential helper or run the provider CLI login command"),
  });
  return checks;
}

export async function authenticateGit(repository: GitRepositoryConfig, method: GitAuthMethod): Promise<DiagnosticCheck[]> {
  const host = hostFromUrl(repository.options.remoteUrl);
  if (method === "gh" || (method === "auto" && host === "github.com" && /^https?:\/\//.test(repository.options.remoteUrl))) {
    if (host !== "github.com") throw new LocalConfigError("invalid_arguments", "gh auth can only configure github.com repositories", { host });
    const status = await runProcess("gh", ["auth", "status", "--hostname", host], { allowFailure: true })
      .catch(() => ({ stdout: "", stderr: "GitHub CLI is not installed", exitCode: 127 }));
    if (status.exitCode !== 0 && method === "gh") {
      throw new LocalConfigError("auth_failed", "GitHub CLI is not authenticated. Run `gh auth login` in an interactive terminal.", { host });
    }
    if (status.exitCode === 0) await runProcess("gh", ["auth", "setup-git"]);
  } else if (method === "ssh" && !/^(git@|ssh:\/\/)/.test(repository.options.remoteUrl)) {
    throw new LocalConfigError("invalid_arguments", "Repository URL is not an SSH URL", { remoteUrl: repository.options.remoteUrl });
  } else if (method === "credential" && /^(git@|ssh:\/\/)/.test(repository.options.remoteUrl)) {
    throw new LocalConfigError("invalid_arguments", "Git credential helpers apply to HTTP(S) URLs, not SSH URLs", { remoteUrl: repository.options.remoteUrl });
  }
  const checks = await diagnoseGitAuth(repository);
  if (!checks.find((check) => check.name === "git-remote-auth")?.ok) {
    throw new LocalConfigError("auth_failed", "Git authentication check failed", { repositoryId: repository.id, checks });
  }
  return checks;
}

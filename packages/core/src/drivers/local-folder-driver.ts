import { createHash } from "node:crypto";
import { access, mkdir } from "node:fs/promises";
import { constants } from "node:fs";
import type { LocalFolderRepositoryConfig, RepositoryStatus } from "../model.js";
import { snapshotDirectory } from "../files.js";
import type { DriverContext, LocalFolderDriverContract } from "../repository-driver.js";

const capabilities = { history: false, conditionalWrite: true, atomicPublish: true } as const;

export class LocalFolderDriver implements LocalFolderDriverContract {
  readonly type = "local-folder" as const;
  readonly capabilities = capabilities;
  async prepare(repository: LocalFolderRepositoryConfig): Promise<void> { await mkdir(repository.workspacePath, { recursive: true }); }
  async inspect(context: DriverContext<LocalFolderRepositoryConfig>): Promise<RepositoryStatus> {
    const snapshot = await snapshotDirectory(context.repository.workspacePath);
    return { state: "synced", remoteRevision: revision(snapshot), localChanges: [], remoteChanged: false, capabilities };
  }
  async pull(context: DriverContext<LocalFolderRepositoryConfig>) { return { remoteRevision: (await this.inspect(context)).remoteRevision, changed: false }; }
  async push(context: DriverContext<LocalFolderRepositoryConfig>, _commitMessage: string) {
    return { remoteRevision: (await this.inspect(context)).remoteRevision, changed: false };
  }
  async doctor(repository: LocalFolderRepositoryConfig) {
    await this.prepare(repository);
    const checks = [];
    try {
      await access(repository.workspacePath, constants.R_OK | constants.W_OK);
      checks.push({ name: "folder-access", ok: true, message: "Local folder is readable and writable" });
    } catch {
      checks.push({ name: "folder-access", ok: false, message: "Local folder is not readable and writable", remediation: "Check mount status and filesystem permissions" });
    }
    return { ok: checks.every((check) => check.ok), checks };
  }
}

function revision(snapshot: Record<string, { sha256: string }>): string {
  const manifest = Object.entries(snapshot).map(([path, file]) => `${path}:${file.sha256}`).sort().join("|");
  return createHash("sha256").update(manifest).digest("hex");
}

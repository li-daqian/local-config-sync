export type RepositoryType = "git" | "local-folder";
export type LinkMode = "symlink" | "copy";
export type SyncState = "not_configured" | "synced" | "pending" | "syncing" | "failed" | "conflict" | "paused";

export interface GlobalConfig {
  version: 1;
  defaultRepositoryId?: string;
  defaultLinkMode: LinkMode;
  autoSync: { enabled: boolean; debounceSeconds: number };
}

export interface DriverCapabilities {
  history: boolean;
  conditionalWrite: boolean;
  atomicPublish: boolean;
}

export interface GitRepositoryOptions {
  remoteUrl: string;
  branch: string;
}

export interface LocalFolderRepositoryOptions {
  path: string;
}

interface RepositoryBase {
  id: string;
  name: string;
  workspacePath: string;
  credentialRef?: string;
}

export interface GitRepositoryConfig extends RepositoryBase {
  type: "git";
  options: GitRepositoryOptions;
}

export interface LocalFolderRepositoryConfig extends RepositoryBase {
  type: "local-folder";
  options: LocalFolderRepositoryOptions;
}

export type RepositoryConfig = GitRepositoryConfig | LocalFolderRepositoryConfig;

export interface RepositoryRegistryFile { version: 1; repositories: RepositoryConfig[] }

export interface Mapping {
  id: string;
  projectPath: string;
  projectName: string;
  repositoryId: string;
  sourcePath: string;
  targetPath: string;
  mode: LinkMode;
  createdAt: string;
  updatedAt: string;
}

export interface MappingRegistryFile { version: 1; mappings: Mapping[] }

export interface FileSnapshot {
  sha256: string;
  size: number;
  deleted?: boolean;
}

export interface RepositoryState {
  version: 1;
  repositoryId: string;
  remoteRevision?: string;
  lastSyncTime?: string;
  files: Record<string, FileSnapshot>;
  lastError?: { code: ErrorCode; message: string; time: string };
}

export type ErrorCode =
  | "generic_error" | "invalid_arguments" | "not_configured" | "conflict"
  | "auth_failed" | "repository_failed" | "filesystem_failed"
  | "unsafe_secret_pattern" | "repository_not_found" | "unsupported_capability"
  | "repository_locked" | "repository_dirty_outside_scope";

export interface DiagnosticCheck {
  name: string;
  ok: boolean;
  message: string;
  remediation?: string;
}

export interface DiagnosticResult { ok: boolean; checks: DiagnosticCheck[] }

export interface RepositoryStatus {
  state: SyncState;
  remoteRevision?: string;
  localChanges: string[];
  remoteChanged: boolean;
  capabilities: DriverCapabilities;
}

export interface RepositorySummary {
  id: string;
  name: string;
  type: RepositoryType;
  state: SyncState;
  workspacePath: string;
  remoteRevision?: string;
  capabilities: DriverCapabilities;
}

export interface MappingSummary {
  id: string;
  repositoryId: string;
  sourcePath: string;
  targetPath: string;
  mode: LinkMode;
  mappedFiles: string[];
  excludeConfigured: boolean;
}

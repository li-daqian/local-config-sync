import type {
  DiagnosticResult, DriverCapabilities, GitRepositoryConfig, LocalFolderRepositoryConfig,
  RepositoryConfig, RepositoryStatus,
} from "./model.js";

export interface DriverContext<T extends RepositoryConfig> {
  repository: T;
  scopes: string[];
  expectedRevision?: string;
}

export interface PullResult { remoteRevision?: string; changed: boolean }
export interface PushResult { remoteRevision?: string; changed: boolean }

export interface RepositoryDriver<T extends RepositoryConfig> {
  readonly type: T["type"];
  readonly capabilities: DriverCapabilities;
  prepare(repository: T): Promise<void>;
  inspect(context: DriverContext<T>): Promise<RepositoryStatus>;
  pull(context: DriverContext<T>): Promise<PullResult>;
  push(context: DriverContext<T>, commitMessage: string): Promise<PushResult>;
  doctor(repository: T): Promise<DiagnosticResult>;
}

export type GitDriverContract = RepositoryDriver<GitRepositoryConfig>;
export type LocalFolderDriverContract = RepositoryDriver<LocalFolderRepositoryConfig>;

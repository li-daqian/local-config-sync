import { z } from "zod";

const successBase = z.object({
  ok: z.literal(true),
  command: z.string()
});

export const commandResponseSchema = successBase;

export const versionResponseSchema = successBase.extend({
  command: z.literal("version"),
  version: z.string(),
  contractVersion: z.number().int()
});

const capabilitiesSchema = z.object({
  history: z.boolean(),
  conditionalWrite: z.boolean(),
  atomicPublish: z.boolean()
});

export const repositorySummarySchema = z.object({
  id: z.string(),
  name: z.string(),
  type: z.string(),
  state: z.string(),
  workspacePath: z.string(),
  remoteRevision: z.string().optional(),
  capabilities: capabilitiesSchema
});

export const mappingSummarySchema = z.object({
  id: z.string(),
  repositoryId: z.string(),
  sourcePath: z.string(),
  targetPath: z.string(),
  mode: z.string(),
  kind: z.string(),
  mappedFiles: z.array(z.string()),
  excludeConfigured: z.boolean()
});

export const fileStatusSummarySchema = z.object({
  mappingId: z.string(),
  repositoryId: z.string(),
  localPath: z.string(),
  remotePath: z.string(),
  status: z.string(),
  localExists: z.boolean(),
  remoteExists: z.boolean()
});

export const statusResponseSchema = successBase.extend({
  command: z.literal("status"),
  projectPath: z.string(),
  state: z.string(),
  repositories: z.array(repositorySummarySchema),
  mappings: z.array(mappingSummarySchema),
  files: z.array(fileStatusSummarySchema),
  lastSyncTime: z.string().optional()
});

export const fileDiffResponseSchema = successBase.extend({
  command: z.literal("diff"),
  mappingId: z.string(),
  repositoryId: z.string(),
  localPath: z.string(),
  remotePath: z.string(),
  remoteRevision: z.string(),
  localExists: z.boolean(),
  remoteExists: z.boolean(),
  contentEncoding: z.literal("base64"),
  localContent: z.string().default(""),
  remoteContent: z.string().default("")
});

const repositoryOptionsSchema = z.object({
  remoteUrl: z.string().optional(),
  branch: z.string().optional(),
  path: z.string().optional()
});

export const configuredRepositorySchema = z.object({
  id: z.string(),
  name: z.string(),
  type: z.string(),
  workspacePath: z.string().optional(),
  options: repositoryOptionsSchema
});

export const repositoryListResponseSchema = successBase.extend({
  command: z.literal("repository.list"),
  repositories: z.array(configuredRepositorySchema).nullish().transform((value) => value ?? [])
});

export const githubRepositorySchema = z.object({
  nameWithOwner: z.string(),
  private: z.boolean(),
  sshUrl: z.string(),
  url: z.string(),
  defaultBranch: z.string()
});

export const githubRepositoriesResponseSchema = successBase.extend({
  command: z.literal("provider.github.repositories"),
  repositories: z.array(githubRepositorySchema).nullish().transform((value) => value ?? [])
});

export const repositoryFilesResponseSchema = successBase.extend({
  command: z.literal("repository.files"),
  repositoryId: z.string(),
  files: z.array(z.string()).nullish().transform((value) => value ?? [])
});

export const mappingPreviewResponseSchema = successBase.extend({
  command: z.literal("preview"),
  state: z.string(),
  kind: z.string(),
  sourcePath: z.string(),
  targetPath: z.string(),
  sourceAbsolutePath: z.string(),
  targetAbsolutePath: z.string(),
  sourceExists: z.boolean(),
  targetExists: z.boolean(),
  sensitivePaths: z.array(z.string()).nullish().transform((value) => value ?? [])
});

export type StatusResponse = z.infer<typeof statusResponseSchema>;
export type RepositorySummary = z.infer<typeof repositorySummarySchema>;
export type MappingSummary = z.infer<typeof mappingSummarySchema>;
export type FileStatusSummary = z.infer<typeof fileStatusSummarySchema>;
export type FileDiffResponse = z.infer<typeof fileDiffResponseSchema>;
export type ConfiguredRepository = z.infer<typeof configuredRepositorySchema>;
export type GitHubRepository = z.infer<typeof githubRepositorySchema>;
export type MappingPreviewResponse = z.infer<typeof mappingPreviewResponseSchema>;

export interface CliFailurePayload {
  code: string;
  message: string;
  details?: {
    paths?: string[];
    [key: string]: unknown;
  };
}

export function parseFailure(value: unknown): CliFailurePayload | undefined {
  const schema = z.object({
    ok: z.literal(false),
    error: z.object({
      code: z.string(),
      message: z.string(),
      details: z.object({
        paths: z.array(z.string()).optional()
      }).passthrough().optional()
    })
  });
  return schema.safeParse(value).data?.error;
}

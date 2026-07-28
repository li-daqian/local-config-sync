import { describe, expect, it } from "vitest";
import {
  parseFailure,
  repositoryFilesResponseSchema,
  statusResponseSchema,
  versionResponseSchema
} from "./models";

describe("CLI response schemas", () => {
  it("accepts the supported version contract", () => {
    expect(versionResponseSchema.parse({
      ok: true,
      command: "version",
      version: "0.1.0",
      contractVersion: 1
    }).contractVersion).toBe(1);
  });

  it("normalizes legacy null file arrays", () => {
    expect(repositoryFilesResponseSchema.parse({
      ok: true,
      command: "repository.files",
      repositoryId: "personal",
      files: null
    }).files).toEqual([]);
  });

  it("rejects a status response with unknown file shape", () => {
    expect(statusResponseSchema.safeParse({
      ok: true,
      command: "status",
      projectPath: "/project",
      state: "synced",
      repositories: [],
      mappings: [],
      files: [{ status: "synced" }]
    }).success).toBe(false);
  });

  it("extracts stable error fields and sensitive paths", () => {
    expect(parseFailure({
      ok: false,
      command: "sync",
      error: {
        code: "unsafe_secret_pattern",
        message: "Sensitive path found",
        details: { paths: [".env.local"], repositoryId: "personal" }
      }
    })).toEqual({
      code: "unsafe_secret_pattern",
      message: "Sensitive path found",
      details: { paths: [".env.local"], repositoryId: "personal" }
    });
  });
});

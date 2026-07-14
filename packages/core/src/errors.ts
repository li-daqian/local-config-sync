import type { ErrorCode } from "./model.js";

export const EXIT_CODES: Record<ErrorCode, number> = {
  generic_error: 1,
  invalid_arguments: 2,
  not_configured: 3,
  conflict: 4,
  auth_failed: 5,
  repository_failed: 6,
  filesystem_failed: 7,
  unsafe_secret_pattern: 8,
  repository_not_found: 9,
  unsupported_capability: 10,
  repository_locked: 11,
  repository_dirty_outside_scope: 12,
};

export class LocalConfigError extends Error {
  constructor(
    public readonly code: ErrorCode,
    message: string,
    public readonly details?: Record<string, unknown>,
    options?: ErrorOptions,
  ) {
    super(message, options);
    this.name = "LocalConfigError";
  }
}

export function asLocalConfigError(error: unknown): LocalConfigError {
  if (error instanceof LocalConfigError) return error;
  const message = error instanceof Error ? error.message : String(error);
  return new LocalConfigError("generic_error", message, undefined, { cause: error });
}

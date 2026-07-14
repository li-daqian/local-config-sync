import { realpath } from "node:fs/promises";
import { resolve } from "node:path";
import { LocalConfigError } from "./errors.js";
import { runProcess } from "./process.js";

export interface ResolvedProject { root: string; excludePath: string }

export async function resolveProject(projectPath: string): Promise<ResolvedProject> {
  const candidate = await realpath(resolve(projectPath)).catch(() => resolve(projectPath));
  const rootResult = await runProcess("git", ["-C", candidate, "rev-parse", "--show-toplevel"], { allowFailure: true });
  if (rootResult.exitCode !== 0) {
    throw new LocalConfigError("not_configured", `${candidate} is not a Git working tree`, { projectPath: candidate });
  }
  const root = await realpath(rootResult.stdout);
  const excludeResult = await runProcess("git", ["-C", root, "rev-parse", "--path-format=absolute", "--git-path", "info/exclude"]);
  return { root, excludePath: excludeResult.stdout };
}

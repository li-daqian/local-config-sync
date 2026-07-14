import { spawn } from "node:child_process";
import { LocalConfigError } from "./errors.js";

export interface ProcessResult { stdout: string; stderr: string; exitCode: number }

export async function runProcess(command: string, args: string[], options: { cwd?: string; env?: NodeJS.ProcessEnv; allowFailure?: boolean } = {}): Promise<ProcessResult> {
  return await new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      cwd: options.cwd,
      env: { ...process.env, ...options.env, GIT_TERMINAL_PROMPT: "0" },
      stdio: ["ignore", "pipe", "pipe"],
      windowsHide: true,
    });
    let stdout = "";
    let stderr = "";
    child.stdout.setEncoding("utf8").on("data", (chunk) => { stdout += chunk; });
    child.stderr.setEncoding("utf8").on("data", (chunk) => { stderr += chunk; });
    child.on("error", (error) => reject(new LocalConfigError(
      command === "git" ? "repository_failed" : "generic_error",
      `Cannot start ${command}: ${error.message}`,
      { command },
      { cause: error },
    )));
    child.on("close", (code) => {
      // Git porcelain uses leading columns for index/worktree state; trimming the
      // beginning would turn ` D path` into `D path` and corrupt path parsing.
      const result = { stdout: stdout.trimEnd(), stderr: stderr.trim(), exitCode: code ?? 1 };
      if (result.exitCode !== 0 && !options.allowFailure) {
        reject(new LocalConfigError("repository_failed", `${command} failed: ${result.stderr || result.stdout}`, { command, args, exitCode: result.exitCode }));
      } else resolve(result);
    });
  });
}

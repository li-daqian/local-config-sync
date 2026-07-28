import { spawn } from "node:child_process";
import type { ZodType } from "zod";
import * as vscode from "vscode";
import { CliLocator } from "./locator";
import { parseFailure, versionResponseSchema } from "./models";

const SUPPORTED_CONTRACT_VERSION = 1;
const MAX_OUTPUT_BYTES = 10 * 1024 * 1024;

export class CliError extends Error {
  constructor(
    readonly code: string,
    message: string,
    readonly diagnostics = "",
    readonly paths: string[] = []
  ) {
    super(message);
    this.name = "CliError";
  }
}

export class CliClient {
  private verifiedExecutable: string | undefined;

  constructor(
    private readonly locator: CliLocator,
    private readonly output: vscode.OutputChannel
  ) {}

  reset(): void {
    this.verifiedExecutable = undefined;
    this.locator.reset();
  }

  async run<T>(
    args: readonly string[],
    schema: ZodType<T>,
    options: { cwd?: string; timeoutMs?: number } = {}
  ): Promise<T> {
    const executable = await this.ensureCompatible();
    const value = await this.invoke(executable, [...args, "--json"], options);
    const parsed = schema.safeParse(value);
    if (!parsed.success) {
      throw new CliError(
        "invalid_cli_response",
        "Local Config Sync CLI returned a response that does not match the supported contract.",
        parsed.error.message
      );
    }
    return parsed.data;
  }

  private async ensureCompatible(): Promise<string> {
    const executable = await this.locator.resolve();
    if (this.verifiedExecutable === executable) {
      return executable;
    }
    const value = await this.invoke(executable, ["--version", "--json"], { timeoutMs: 15_000 });
    const parsed = versionResponseSchema.safeParse(value);
    if (!parsed.success) {
      throw new CliError("incompatible_cli", "The configured CLI does not expose a compatible JSON version contract.");
    }
    if (parsed.data.contractVersion !== SUPPORTED_CONTRACT_VERSION) {
      throw new CliError(
        "incompatible_cli",
        `Unsupported CLI contract ${parsed.data.contractVersion}; expected ${SUPPORTED_CONTRACT_VERSION}.`
      );
    }
    this.verifiedExecutable = executable;
    return executable;
  }

  private invoke(
    executable: string,
    args: readonly string[],
    options: { cwd?: string; timeoutMs?: number }
  ): Promise<unknown> {
    this.output.appendLine(`[${new Date().toISOString()}] ${commandLabel(args)}`);
    return new Promise((resolve, reject) => {
      const child = spawn(executable, args, {
        cwd: options.cwd,
        shell: false,
        windowsHide: true,
        env: process.env
      });
      const stdout: Buffer[] = [];
      const stderr: Buffer[] = [];
      let outputBytes = 0;
      let settled = false;
      const timeout = options.timeoutMs === undefined
        ? undefined
        : setTimeout(() => {
            child.kill();
            finishReject(new CliError("timeout", "Local Config Sync CLI command timed out."));
          }, options.timeoutMs);

      const finishReject = (error: Error): void => {
        if (settled) {
          return;
        }
        settled = true;
        if (timeout !== undefined) {
          clearTimeout(timeout);
        }
        reject(error);
      };

      child.stdout.on("data", (chunk: Buffer) => {
        outputBytes += chunk.length;
        if (outputBytes > MAX_OUTPUT_BYTES) {
          child.kill();
          finishReject(new CliError("cli_output_too_large", "Local Config Sync CLI produced too much output."));
          return;
        }
        stdout.push(chunk);
      });
      child.stderr.on("data", (chunk: Buffer) => {
        outputBytes += chunk.length;
        if (outputBytes > MAX_OUTPUT_BYTES) {
          child.kill();
          finishReject(new CliError("cli_output_too_large", "Local Config Sync CLI produced too much output."));
          return;
        }
        stderr.push(chunk);
      });
      child.on("error", (error) => {
        finishReject(new CliError("cli_unavailable", "Cannot start the Local Config Sync CLI.", error.message));
      });
      child.on("close", (code) => {
        if (settled) {
          return;
        }
        settled = true;
        if (timeout !== undefined) {
          clearTimeout(timeout);
        }
        const stdoutText = Buffer.concat(stdout).toString("utf8").trim();
        const stderrText = Buffer.concat(stderr).toString("utf8").trim();
        let value: unknown;
        try {
          value = JSON.parse(stdoutText);
        } catch {
          reject(new CliError("invalid_cli_response", "Local Config Sync CLI returned invalid JSON.", stderrText));
          return;
        }
        if (code !== 0) {
          const failure = parseFailure(value);
          reject(new CliError(
            failure?.code ?? "cli_failed",
            failure?.message ?? "Local Config Sync CLI command failed.",
            stderrText,
            failure?.details?.paths ?? []
          ));
          return;
        }
        resolve(value);
      });
    });
  }
}

function commandLabel(args: readonly string[]): string {
  const command = args.filter((argument) => !argument.startsWith("--")).slice(0, 3).join(" ");
  return command === "" ? "local-config" : `local-config ${command}`;
}

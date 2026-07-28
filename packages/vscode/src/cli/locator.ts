import { createHash } from "node:crypto";
import { promises as fs } from "node:fs";
import * as path from "node:path";
import * as vscode from "vscode";

export class CliLocator {
  private resolved: Promise<string> | undefined;

  constructor(private readonly context: vscode.ExtensionContext) {}

  resolve(): Promise<string> {
    this.resolved ??= this.resolveOnce();
    return this.resolved;
  }

  reset(): void {
    this.resolved = undefined;
  }

  private async resolveOnce(): Promise<string> {
    const override = vscode.workspace.getConfiguration("localConfigSync").get<string>("cliPath", "").trim();
    if (override !== "") {
      return override;
    }

    const executableName = process.platform === "win32" ? "local-config.exe" : "local-config";
    const bundled = path.join(this.context.extensionPath, "bin", executableName);
    if (await isFile(bundled)) {
      return this.extractBundledCli(bundled, executableName);
    }

    if (this.context.extensionMode === vscode.ExtensionMode.Development) {
      const development = path.resolve(this.context.extensionPath, "..", "..", "build", executableName);
      if (await isFile(development)) {
        return development;
      }
    }

    throw new Error(
      "The Local Config Sync extension does not contain a CLI for this platform. " +
      "Build the repository CLI or configure localConfigSync.cliPath."
    );
  }

  private async extractBundledCli(source: string, executableName: string): Promise<string> {
    const content = await fs.readFile(source);
    const digest = createHash("sha256").update(content).digest("hex").slice(0, 16);
    const targetDirectory = path.join(this.context.globalStorageUri.fsPath, "cli");
    const suffix = process.platform === "win32" ? ".exe" : "";
    const target = path.join(targetDirectory, `local-config-${digest}${suffix}`);
    if (await isFile(target)) {
      return target;
    }

    await fs.mkdir(targetDirectory, { recursive: true, mode: 0o700 });
    const temporary = path.join(targetDirectory, `.local-config-${process.pid}-${Date.now()}.tmp`);
    await fs.writeFile(temporary, content, { mode: 0o700 });
    await fs.rename(temporary, target);
    if (process.platform !== "win32") {
      await fs.chmod(target, 0o700);
    }
    return target;
  }
}

async function isFile(candidate: string): Promise<boolean> {
  try {
    return (await fs.stat(candidate)).isFile();
  } catch {
    return false;
  }
}

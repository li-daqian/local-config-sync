import { mkdtemp, mkdir, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { execa } from "./process-helper.js";

export async function temporaryRoot(name: string): Promise<string> {
  return await mkdtemp(join(tmpdir(), `local-config-${name}-`));
}

export async function initProject(root: string, name = "project"): Promise<string> {
  const path = join(root, name);
  await mkdir(path, { recursive: true });
  await execa("git", ["init", "--initial-branch", "main"], path);
  await execa("git", ["config", "user.name", "Local Config Test"], path);
  await execa("git", ["config", "user.email", "test@example.invalid"], path);
  await writeFile(join(path, "README.md"), "test\n");
  await execa("git", ["add", "README.md"], path);
  await execa("git", ["commit", "-m", "initial"], path);
  return path;
}

export async function createRemote(root: string): Promise<{ bare: string; seed: string }> {
  const bare = join(root, "remote.git");
  const seed = await initProject(root, "seed");
  await execa("git", ["init", "--bare", "--initial-branch", "main", bare], root);
  await execa("git", ["remote", "add", "origin", bare], seed);
  await execa("git", ["push", "-u", "origin", "main"], seed);
  return { bare, seed };
}

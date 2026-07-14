import { createHash } from "node:crypto";
import { lstat, mkdir, readFile, readdir } from "node:fs/promises";
import { dirname, isAbsolute, relative, resolve, sep } from "node:path";
import { LocalConfigError } from "./errors.js";
import type { FileSnapshot } from "./model.js";

export function safeRelativePath(value: string, field: string): string {
  const normalized = value.replaceAll("\\", "/").replace(/^\.\//, "").replace(/\/$/, "");
  if (!normalized || isAbsolute(value) || normalized === ".." || normalized.startsWith("../") || normalized.includes("/../")) {
    throw new LocalConfigError("invalid_arguments", `${field} must be a safe relative path`, { field, value });
  }
  return normalized;
}

export function resolveInside(root: string, path: string): string {
  const result = resolve(root, path);
  const fromRoot = relative(resolve(root), result);
  if (fromRoot === ".." || fromRoot.startsWith(`..${sep}`) || isAbsolute(fromRoot)) {
    throw new LocalConfigError("invalid_arguments", "Path escapes its allowed root", { root, path });
  }
  return result;
}

export async function listFiles(root: string): Promise<string[]> {
  try {
    const output: string[] = [];
    async function walk(current: string): Promise<void> {
      for (const entry of await readdir(current, { withFileTypes: true })) {
        if (entry.name === ".git") continue;
        const full = resolve(current, entry.name);
        if (entry.isDirectory()) await walk(full);
        else if (entry.isFile() || entry.isSymbolicLink()) output.push(relative(root, full).split(sep).join("/"));
      }
    }
    await walk(root);
    return output.sort();
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") return [];
    throw error;
  }
}

export async function snapshotDirectory(root: string, prefix = ""): Promise<Record<string, FileSnapshot>> {
  const result: Record<string, FileSnapshot> = {};
  for (const relativePath of await listFiles(root)) {
    const path = resolveInside(root, relativePath);
    const stat = await lstat(path);
    const buffer = stat.isSymbolicLink() ? Buffer.from(await readFile(path)) : await readFile(path);
    result[prefix ? `${prefix}/${relativePath}` : relativePath] = {
      sha256: createHash("sha256").update(buffer).digest("hex"),
      size: buffer.byteLength,
    };
  }
  return result;
}

export function snapshotsEqual(a?: FileSnapshot, b?: FileSnapshot): boolean {
  return a?.sha256 === b?.sha256 && a?.size === b?.size && Boolean(a?.deleted) === Boolean(b?.deleted);
}

export async function ensureParent(path: string): Promise<void> { await mkdir(dirname(path), { recursive: true }); }

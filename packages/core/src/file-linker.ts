import { cp, lstat, mkdir, readlink, rm, symlink } from "node:fs/promises";
import { dirname, relative, resolve } from "node:path";
import { LocalConfigError } from "./errors.js";
import type { LinkMode } from "./model.js";

export async function materialize(source: string, target: string, mode: LinkMode, replaceCopy = false): Promise<void> {
  await mkdir(source, { recursive: true });
  await mkdir(dirname(target), { recursive: true });
  const targetStat = await lstat(target).catch(() => undefined);
  if (mode === "symlink") {
    if (targetStat?.isSymbolicLink()) {
      const current = resolve(dirname(target), await readlink(target));
      if (current === resolve(source)) return;
      await rm(target);
    } else if (targetStat) {
      throw new LocalConfigError("filesystem_failed", `Target already exists and is not a symlink: ${target}`, { target });
    }
    await symlink(relative(dirname(target), source), target, "dir");
    return;
  }
  if (targetStat && !replaceCopy) {
    throw new LocalConfigError("filesystem_failed", `Copy target already exists: ${target}`, { target });
  }
  if (replaceCopy) await rm(target, { recursive: true, force: true });
  await mkdir(target, { recursive: true });
  await cp(source, target, { recursive: true, force: false, errorOnExist: true });
}

export async function removeMaterialized(target: string, mode: LinkMode, keepFiles: boolean): Promise<void> {
  if (keepFiles && mode === "symlink") {
    const source = resolve(dirname(target), await readlink(target));
    await rm(target, { force: true });
    await cp(source, target, { recursive: true });
  } else if (!keepFiles) {
    await rm(target, { recursive: true, force: true });
  }
}

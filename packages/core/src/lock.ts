import { mkdir, open, readFile, rm } from "node:fs/promises";
import { dirname } from "node:path";
import { LocalConfigError } from "./errors.js";

export async function withRepositoryLock<T>(lockPath: string, operation: () => Promise<T>): Promise<T> {
  await mkdir(dirname(lockPath), { recursive: true });
  let handle;
  try {
    handle = await open(lockPath, "wx", 0o600);
    await handle.writeFile(JSON.stringify({ pid: process.pid, createdAt: new Date().toISOString() }));
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "EEXIST") {
      const owner = await readFile(lockPath, "utf8").catch(() => "unknown");
      throw new LocalConfigError("repository_locked", "Repository is currently being synchronized", { lockPath, owner });
    }
    throw error;
  }
  try {
    return await operation();
  } finally {
    await handle.close();
    await rm(lockPath, { force: true });
  }
}

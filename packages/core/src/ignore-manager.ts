import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname } from "node:path";

const MARKER = "# local-config-sync";

export async function addExclude(excludePath: string, targetPath: string): Promise<void> {
  await mkdir(dirname(excludePath), { recursive: true });
  const existing = await readFile(excludePath, "utf8").catch(() => "");
  const rule = `/${targetPath.replaceAll("\\", "/").replace(/^\//, "")}/`;
  const lines = existing.split(/\r?\n/);
  if (lines.includes(rule)) return;
  const prefix = existing && !existing.endsWith("\n") ? "\n" : "";
  const marker = lines.includes(MARKER) ? "" : `${MARKER}\n`;
  await writeFile(excludePath, `${existing}${prefix}${marker}${rule}\n`, "utf8");
}

export async function removeExclude(excludePath: string, targetPath: string): Promise<void> {
  const existing = await readFile(excludePath, "utf8").catch(() => "");
  const rule = `/${targetPath.replaceAll("\\", "/").replace(/^\//, "")}/`;
  const lines = existing.split(/\r?\n/).filter((line) => line !== rule);
  const markerIndex = lines.indexOf(MARKER);
  if (markerIndex >= 0 && !lines.slice(markerIndex + 1).some((line) => line.startsWith("/"))) lines.splice(markerIndex, 1);
  await writeFile(excludePath, `${lines.join("\n").replace(/\n+$/, "")}\n`, "utf8");
}

export async function hasExclude(excludePath: string, targetPath: string): Promise<boolean> {
  const rule = `/${targetPath.replaceAll("\\", "/").replace(/^\//, "")}/`;
  return (await readFile(excludePath, "utf8").catch(() => "")).split(/\r?\n/).includes(rule);
}

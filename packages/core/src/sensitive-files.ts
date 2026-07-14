import { basename } from "node:path";
import { LocalConfigError } from "./errors.js";
import { listFiles } from "./files.js";

const EXACT = new Set([".env", "id_rsa", "id_ed25519", "credentials", "credentials.json", "application-prod.yml", "application-production.yml"]);
const SUFFIXES = [".pem", ".key", ".p12", ".pfx"];

export async function scanSensitive(root: string, scopes: string[]): Promise<string[]> {
  const matches: string[] = [];
  for (const scope of scopes) {
    for (const file of await listFiles(`${root}/${scope}`)) {
      const name = basename(file).toLowerCase();
      if (EXACT.has(name) || name.startsWith(".env.") || SUFFIXES.some((suffix) => name.endsWith(suffix))) matches.push(`${scope}/${file}`);
    }
  }
  return matches;
}

export function assertNoSensitive(matches: string[], allowSensitive = false): void {
  if (matches.length && !allowSensitive) throw new LocalConfigError(
    "unsafe_secret_pattern",
    "Sensitive file patterns were found. Review them before explicitly allowing this sync.",
    { paths: matches },
  );
}

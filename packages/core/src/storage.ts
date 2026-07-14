import { mkdir, readFile, rename, rm, writeFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { parse, stringify } from "yaml";
import { LocalConfigError } from "./errors.js";
import type { AppPaths } from "./paths.js";
import type { GlobalConfig, MappingRegistryFile, RepositoryRegistryFile, RepositoryState } from "./model.js";

const DEFAULT_CONFIG: GlobalConfig = {
  version: 1,
  defaultLinkMode: "symlink",
  autoSync: { enabled: false, debounceSeconds: 60 },
};
const DEFAULT_REPOSITORIES: RepositoryRegistryFile = { version: 1, repositories: [] };
const DEFAULT_MAPPINGS: MappingRegistryFile = { version: 1, mappings: [] };

async function readYaml<T>(path: string, fallback?: T): Promise<T> {
  try {
    return parse(await readFile(path, "utf8")) as T;
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT" && fallback !== undefined) return structuredClone(fallback);
    throw new LocalConfigError("filesystem_failed", `Cannot read ${path}`, { path }, { cause: error });
  }
}

async function atomicWrite(path: string, content: string): Promise<void> {
  await mkdir(dirname(path), { recursive: true, mode: 0o700 });
  const temporary = `${path}.${process.pid}.tmp`;
  await writeFile(temporary, content, { encoding: "utf8", mode: 0o600 });
  await rename(temporary, path);
}

export class Storage {
  constructor(readonly paths: AppPaths) {}

  async initialize(defaultLinkMode: "symlink" | "copy" = "symlink"): Promise<GlobalConfig> {
    await Promise.all(Object.values(this.paths).filter((value) => !value.endsWith(".yml")).map((path) => mkdir(path, { recursive: true, mode: 0o700 })));
    const existing = await this.readConfig();
    const config = { ...existing, defaultLinkMode };
    await Promise.all([
      this.writeConfig(config),
      this.writeRepositories(await this.readRepositories()),
      this.writeMappings(await this.readMappings()),
    ]);
    return config;
  }

  readConfig(): Promise<GlobalConfig> { return readYaml(this.paths.config, DEFAULT_CONFIG); }
  readRepositories(): Promise<RepositoryRegistryFile> { return readYaml(this.paths.repositories, DEFAULT_REPOSITORIES); }
  readMappings(): Promise<MappingRegistryFile> { return readYaml(this.paths.mappings, DEFAULT_MAPPINGS); }
  writeConfig(value: GlobalConfig): Promise<void> { return atomicWrite(this.paths.config, stringify(value)); }
  writeRepositories(value: RepositoryRegistryFile): Promise<void> { return atomicWrite(this.paths.repositories, stringify(value)); }
  writeMappings(value: MappingRegistryFile): Promise<void> { return atomicWrite(this.paths.mappings, stringify(value)); }

  async readState(repositoryId: string): Promise<RepositoryState> {
    const path = join(this.paths.states, `${repositoryId}.json`);
    try {
      return JSON.parse(await readFile(path, "utf8")) as RepositoryState;
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code === "ENOENT") return { version: 1, repositoryId, files: {} };
      throw new LocalConfigError("filesystem_failed", `Cannot read repository state for ${repositoryId}`, { repositoryId }, { cause: error });
    }
  }

  writeState(value: RepositoryState): Promise<void> {
    return atomicWrite(join(this.paths.states, `${value.repositoryId}.json`), `${JSON.stringify(value, null, 2)}\n`);
  }

  async removeState(repositoryId: string): Promise<void> {
    await rm(join(this.paths.states, `${repositoryId}.json`), { force: true });
  }
}

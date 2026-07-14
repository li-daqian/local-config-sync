import { basename, resolve } from "node:path";
import { randomUUID } from "node:crypto";
import { LocalConfigError } from "./errors.js";
import { safeRelativePath } from "./files.js";
import type { LinkMode, Mapping } from "./model.js";
import type { Storage } from "./storage.js";

function overlaps(a: string, b: string): boolean { return a === b || a.startsWith(`${b}/`) || b.startsWith(`${a}/`); }

export class MappingManager {
  constructor(private readonly storage: Storage) {}
  async list(): Promise<Mapping[]> { return (await this.storage.readMappings()).mappings; }
  async forProject(projectPath: string): Promise<Mapping[]> {
    const root = resolve(projectPath);
    return (await this.list()).filter((mapping) => resolve(mapping.projectPath) === root);
  }
  async forRepository(repositoryId: string): Promise<Mapping[]> { return (await this.list()).filter((mapping) => mapping.repositoryId === repositoryId); }

  async add(input: { id?: string; projectPath: string; repositoryId: string; sourcePath: string; targetPath: string; mode: LinkMode }): Promise<Mapping> {
    const file = await this.storage.readMappings();
    const sourcePath = safeRelativePath(input.sourcePath, "sourcePath");
    const targetPath = safeRelativePath(input.targetPath, "targetPath");
    const conflict = file.mappings.find((mapping) => mapping.repositoryId === input.repositoryId && overlaps(mapping.sourcePath, sourcePath));
    if (conflict) throw new LocalConfigError("invalid_arguments", "Mapping source path overlaps an existing mapping", { existingMappingId: conflict.id, sourcePath });
    const duplicateTarget = file.mappings.find((mapping) => resolve(mapping.projectPath) === resolve(input.projectPath) && overlaps(mapping.targetPath, targetPath));
    if (duplicateTarget) throw new LocalConfigError("invalid_arguments", "Mapping target path overlaps an existing mapping", { existingMappingId: duplicateTarget.id, targetPath });
    const now = new Date().toISOString();
    const mapping: Mapping = {
      id: input.id?.trim() || `${basename(input.projectPath).replace(/[^a-zA-Z0-9_-]/g, "-")}-${randomUUID().slice(0, 8)}`,
      projectPath: resolve(input.projectPath),
      projectName: basename(input.projectPath),
      repositoryId: input.repositoryId,
      sourcePath,
      targetPath,
      mode: input.mode,
      createdAt: now,
      updatedAt: now,
    };
    if (file.mappings.some((item) => item.id === mapping.id)) throw new LocalConfigError("invalid_arguments", `Mapping id already exists: ${mapping.id}`);
    file.mappings.push(mapping);
    await this.storage.writeMappings(file);
    return mapping;
  }

  async remove(mappingIds: string[]): Promise<Mapping[]> {
    const file = await this.storage.readMappings();
    const removed = file.mappings.filter((mapping) => mappingIds.includes(mapping.id));
    file.mappings = file.mappings.filter((mapping) => !mappingIds.includes(mapping.id));
    await this.storage.writeMappings(file);
    return removed;
  }
}

import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { withRepositoryLock } from "../packages/core/src/lock.js";
import { LocalConfigService } from "../packages/core/src/service.js";
import { initProject, temporaryRoot } from "./helpers.js";

describe("mapping and repository lock", () => {
  it("rejects overlapping repository scopes", async () => {
    const root = await temporaryRoot("mapping");
    const first = await initProject(root, "first");
    const second = await initProject(root, "second");
    const service = new LocalConfigService(join(root, "home"));
    await service.init();
    await service.repositories.addLocalFolder({ id: "local", path: join(root, "repo") });
    await service.link({ project: first, repositoryId: "local", sourcePath: "shared", targetPath: "config" });
    await expect(service.link({ project: second, repositoryId: "local", sourcePath: "shared/nested", targetPath: "config" })).rejects.toMatchObject({ code: "invalid_arguments" });
  });

  it("prevents concurrent repository operations", async () => {
    const root = await temporaryRoot("lock");
    const lock = join(root, "repo.lock");
    let release!: () => void;
    const blocker = new Promise<void>((resolve) => { release = resolve; });
    const first = withRepositoryLock(lock, async () => await blocker);
    await new Promise((resolve) => setTimeout(resolve, 20));
    await expect(withRepositoryLock(lock, async () => undefined)).rejects.toMatchObject({ code: "repository_locked" });
    release();
    await first;
  });
});

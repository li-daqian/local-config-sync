import { lstat, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { LocalConfigService } from "../packages/core/src/service.js";
import { initProject, temporaryRoot } from "./helpers.js";

describe("local-folder repository", () => {
  it("links a folder, excludes it, and reports status", async () => {
    const root = await temporaryRoot("folder");
    const project = await initProject(root);
    const repository = join(root, "repository");
    const service = new LocalConfigService(join(root, "home"));
    await service.init();
    await service.repositories.addLocalFolder({ id: "local", path: repository });
    await service.link({ project, repositoryId: "local", sourcePath: "sample/config", targetPath: "config", mode: "symlink" });

    expect((await lstat(join(project, "config"))).isSymbolicLink()).toBe(true);
    await writeFile(join(project, "config", "application-dev.yml"), "feature: true\n");
    const result = await service.sync({ project });
    expect(result[0]?.state).toBe("synced");
    expect(await readFile(join(repository, "sample/config/application-dev.yml"), "utf8")).toBe("feature: true\n");
    expect(await readFile(join(project, ".git/info/exclude"), "utf8")).toContain("/config/");
    expect((await service.status(project)).state).toBe("synced");
  });

  it("rejects sensitive files unless explicitly allowed", async () => {
    const root = await temporaryRoot("sensitive");
    const project = await initProject(root);
    const service = new LocalConfigService(join(root, "home"));
    await service.init();
    await service.repositories.addLocalFolder({ id: "local", path: join(root, "repository") });
    await service.link({ project, repositoryId: "local", sourcePath: "config", targetPath: "config", mode: "symlink" });
    await writeFile(join(project, "config", ".env"), "PASSWORD=nope\n");
    await expect(service.sync({ project })).rejects.toMatchObject({ code: "unsafe_secret_pattern" });
    await expect(service.sync({ project, allowSensitive: true })).resolves.toHaveLength(1);
  });
});

import { readFile, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { LocalConfigService } from "../packages/core/src/service.js";
import { createRemote, initProject, temporaryRoot } from "./helpers.js";
import { execa } from "./process-helper.js";

describe("Git repository driver", () => {
  it("pushes mapped changes and pulls remote changes", async () => {
    const root = await temporaryRoot("git");
    const { bare, seed } = await createRemote(root);
    const project = await initProject(root, "business");
    const service = new LocalConfigService(join(root, "home"));
    await service.init();
    await expect(service.authenticateUrl(bare, "auto")).resolves.toEqual(expect.arrayContaining([expect.objectContaining({ name: "git-remote-auth", ok: true })]));
    const repository = await service.repositories.addGit({ id: "personal", url: bare, branch: "main" });
    await execa("git", ["config", "user.name", "Local Config Test"], repository.workspacePath);
    await execa("git", ["config", "user.email", "test@example.invalid"], repository.workspacePath);
    await service.link({ project, repositoryId: "personal", sourcePath: "business/config", targetPath: "config", mode: "symlink" });
    await writeFile(join(project, "config", "application-dev.yml"), "version: 1\n");
    await service.sync({ project });

    await execa("git", ["pull", "--ff-only"], seed);
    expect(await readFile(join(seed, "business/config/application-dev.yml"), "utf8")).toBe("version: 1\n");

    await writeFile(join(seed, "business/config/application-dev.yml"), "version: 2\n");
    await execa("git", ["add", "business/config/application-dev.yml"], seed);
    await execa("git", ["commit", "-m", "remote change"], seed);
    await execa("git", ["push"], seed);
    await service.sync({ project });
    expect(await readFile(join(project, "config", "application-dev.yml"), "utf8")).toBe("version: 2\n");

    await rm(join(project, "config", "application-dev.yml"));
    await service.sync({ project });
    await execa("git", ["pull", "--ff-only"], seed);
    await expect(readFile(join(seed, "business/config/application-dev.yml"), "utf8")).rejects.toMatchObject({ code: "ENOENT" });
  });

  it("stops when local and remote change concurrently", async () => {
    const root = await temporaryRoot("conflict");
    const { bare, seed } = await createRemote(root);
    const project = await initProject(root, "business");
    const service = new LocalConfigService(join(root, "home"));
    await service.init();
    const repository = await service.repositories.addGit({ id: "personal", url: bare });
    await execa("git", ["config", "user.name", "Local Config Test"], repository.workspacePath);
    await execa("git", ["config", "user.email", "test@example.invalid"], repository.workspacePath);
    await service.link({ project, repositoryId: "personal", sourcePath: "business/config", targetPath: "config", mode: "symlink" });
    await writeFile(join(project, "config", "value.yml"), "value: baseline\n");
    await service.sync({ project });
    await execa("git", ["pull", "--ff-only"], seed);

    await writeFile(join(project, "config", "value.yml"), "value: local\n");
    await writeFile(join(seed, "business/config/value.yml"), "value: remote\n");
    await execa("git", ["add", "business/config/value.yml"], seed);
    await execa("git", ["commit", "-m", "remote conflict"], seed);
    await execa("git", ["push"], seed);

    await expect(service.sync({ project })).rejects.toMatchObject({ code: "conflict" });
    expect(await readFile(join(project, "config", "value.yml"), "utf8")).toBe("value: local\n");
  });
});

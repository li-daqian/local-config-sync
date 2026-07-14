#!/usr/bin/env node
import { Command, CommanderError, Option } from "commander";
import {
  EXIT_CODES, LocalConfigError, LocalConfigService, asLocalConfigError,
  type GitAuthMethod, type LinkMode,
} from "@local-config-sync/core";

interface OutputOptions { json?: boolean }
let activeCommand = "unknown";
let wantsJson = process.argv.includes("--json");

function service(): LocalConfigService { return new LocalConfigService(); }

function success(command: string, payload: Record<string, unknown>, options: OutputOptions): void {
  const response = { ok: true, command, ...payload };
  if (options.json || wantsJson) process.stdout.write(`${JSON.stringify(response)}\n`);
  else printHuman(response);
}

function printHuman(response: Record<string, unknown>): void {
  const { ok: _ok, command, ...content } = response;
  process.stdout.write(`${String(command)} completed successfully\n`);
  if (Object.keys(content).length) process.stdout.write(`${JSON.stringify(content, null, 2)}\n`);
}

function outputOptions(command: Command): OutputOptions { return command.optsWithGlobals<OutputOptions>(); }

function addJson(command: Command): Command { return command.option("--json", "output one JSON object to stdout"); }

const program = new Command()
  .name("local-config")
  .description("Synchronize project-local overlay configuration through managed repositories")
  .version("0.1.0")
  .option("--json", "output one JSON object to stdout")
  .showHelpAfterError()
  .exitOverride();

program.hook("preAction", (_thisCommand, actionCommand) => {
  const names: string[] = [];
  for (let current: Command | null = actionCommand; current?.parent; current = current.parent) names.unshift(current.name());
  activeCommand = names[0] === "repository" && names[1] === "add" ? "repository.add" : names.join(".");
  wantsJson = actionCommand.optsWithGlobals<OutputOptions>().json === true;
});

addJson(program.command("init")
  .description("initialize user-level Local Config Sync storage")
  .addOption(new Option("--default-link-mode <mode>").choices(["symlink", "copy"]).default("symlink"))
  .action(async (options: { defaultLinkMode: LinkMode }, command) => {
    const core = service();
    const config = await core.init(options.defaultLinkMode);
    success("init", { home: core.paths.home, config }, outputOptions(command));
  }));

const repository = program.command("repository").description("manage repository instances");
const repositoryAdd = repository.command("add").description("add and prepare a repository");

addJson(repositoryAdd.command("git")
  .requiredOption("--id <id>")
  .option("--name <name>")
  .requiredOption("--url <url>")
  .option("--branch <branch>", "remote branch", "main")
  .action(async (options: { id: string; name?: string; url: string; branch: string }, command) => {
    const core = service();
    await core.init();
    const value = await core.repositories.addGit(options);
    success("repository.add", { repository: sanitizeRepository(value) }, outputOptions(command));
  }));

addJson(repositoryAdd.command("local-folder")
  .requiredOption("--id <id>")
  .option("--name <name>")
  .requiredOption("--path <path>")
  .action(async (options: { id: string; name?: string; path: string }, command) => {
    const core = service();
    await core.init();
    const value = await core.repositories.addLocalFolder(options);
    success("repository.add", { repository: sanitizeRepository(value) }, outputOptions(command));
  }));

addJson(repository.command("list").action(async (_options, command) => {
  const values = await service().repositories.list();
  success("repository.list", { repositories: values.map(sanitizeRepository) }, outputOptions(command));
}));

addJson(repository.command("show").argument("<id>").action(async (id: string, _options, command) => {
  success("repository.show", { repository: sanitizeRepository(await service().repositories.get(id)) }, outputOptions(command));
}));

addJson(repository.command("doctor").argument("<id>").action(async (id: string, _options, command) => {
  const core = service();
  const result = await core.doctor(undefined, id);
  success("repository.doctor", { repositoryId: id, ...result }, outputOptions(command));
}));

addJson(repository.command("auth")
  .description("verify or configure Git authentication without storing credentials")
  .argument("[id]")
  .option("--url <url>", "authenticate a Git URL before registering it")
  .addOption(new Option("--method <method>").choices(["auto", "ssh", "credential", "gh"]).default("auto"))
  .action(async (id: string | undefined, options: { url?: string; method: GitAuthMethod }, command) => {
    if (Boolean(id) === Boolean(options.url)) throw new LocalConfigError("invalid_arguments", "Specify exactly one repository id or --url");
    const core = service();
    const checks = id ? await core.authenticate(id, options.method) : await core.authenticateUrl(options.url!, options.method);
    success("repository.auth", { repositoryId: id, remoteUrl: options.url, method: options.method, checks }, outputOptions(command));
  }));

addJson(repository.command("remove")
  .argument("<id>")
  .description("remove a repository registration; managed files are preserved")
  .action(async (id: string, _options, command) => {
    const removed = await service().repositories.remove(id);
    success("repository.remove", { repository: sanitizeRepository(removed), workspacePreserved: true }, outputOptions(command));
  }));

addJson(program.command("link")
  .description("link a repository directory into a Git project")
  .option("--project <path>", "business project", ".")
  .requiredOption("--repository <id>")
  .requiredOption("--source-path <path>")
  .requiredOption("--target <path>")
  .addOption(new Option("--mode <mode>").choices(["symlink", "copy"]))
  .option("--id <id>")
  .action(async (options: { project: string; repository: string; sourcePath: string; target: string; mode?: LinkMode; id?: string }, command) => {
    const mapping = await service().link({ project: options.project, repositoryId: options.repository, sourcePath: options.sourcePath, targetPath: options.target, mode: options.mode, id: options.id });
    success("link", { projectPath: mapping.projectPath, mapping }, outputOptions(command));
  }));

for (const operation of ["pull", "push", "sync"] as const) {
  addJson(program.command(operation)
    .option("--project <path>")
    .option("--repository <id>")
    .option("--allow-sensitive", "explicitly allow files matching sensitive patterns")
    .action(async (options: { project?: string; repository?: string; allowSensitive?: boolean }, command) => {
      const project = options.project ?? (options.repository ? undefined : ".");
      const results = await service().sync({ project, repositoryId: options.repository, allowSensitive: options.allowSensitive }, operation);
      success(operation, { projectPath: project, repositories: results }, outputOptions(command));
    }));
}

addJson(program.command("status")
  .option("--project <path>", "business project", ".")
  .action(async (options: { project: string }, command) => {
    success("status", await service().status(options.project), outputOptions(command));
  }));

addJson(program.command("doctor")
  .option("--project <path>")
  .action(async (options: { project?: string }, command) => {
    const result = await service().doctor(options.project);
    success("doctor", { projectPath: options.project, ...result }, outputOptions(command));
  }));

addJson(program.command("unlink")
  .option("--project <path>", "business project", ".")
  .option("--keep-files")
  .option("--keep-exclude")
  .action(async (options: { project: string; keepFiles?: boolean; keepExclude?: boolean }, command) => {
    const removed = await service().unlink(options.project, options);
    success("unlink", { projectPath: options.project, removedMappings: removed }, outputOptions(command));
  }));

function sanitizeRepository<T extends { credentialRef?: string }>(value: T): Omit<T, "credentialRef"> & { credentialConfigured: boolean } {
  const { credentialRef, ...safe } = value;
  return { ...safe, credentialConfigured: Boolean(credentialRef) };
}

try {
  await program.parseAsync(process.argv);
} catch (rawError) {
  if (rawError instanceof CommanderError && rawError.code === "commander.helpDisplayed") process.exit(0);
  const error = rawError instanceof CommanderError
    ? new LocalConfigError("invalid_arguments", rawError.message)
    : asLocalConfigError(rawError);
  const response = { ok: false, command: activeCommand, error: { code: error.code, message: error.message, details: error.details } };
  if (wantsJson) process.stdout.write(`${JSON.stringify(response)}\n`);
  else process.stderr.write(`${error.code}: ${error.message}\n`);
  process.exitCode = EXIT_CODES[error.code];
}

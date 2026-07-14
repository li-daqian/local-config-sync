import { homedir } from "node:os";
import { resolve, join } from "node:path";

export interface AppPaths {
  home: string;
  config: string;
  repositories: string;
  mappings: string;
  workspaces: string;
  states: string;
  locks: string;
  logs: string;
}

export function getAppPaths(home = process.env.LOCAL_CONFIG_HOME ?? join(homedir(), ".local-config-sync")): AppPaths {
  const root = resolve(home);
  return {
    home: root,
    config: join(root, "config.yml"),
    repositories: join(root, "repositories.yml"),
    mappings: join(root, "mappings.yml"),
    workspaces: join(root, "workspaces"),
    states: join(root, "state", "repositories"),
    locks: join(root, "locks"),
    logs: join(root, "logs"),
  };
}

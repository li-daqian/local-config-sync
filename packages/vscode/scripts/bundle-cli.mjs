import { mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const extensionRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repositoryRoot = resolve(extensionRoot, "..", "..");
const defaultTarget = `${platformName(process.platform)}-${architectureName(process.arch)}`;
const target = process.env.LOCAL_CONFIG_TARGET || defaultTarget;
const [goos, goarch] = target.split("-");

if (!goos || !goarch) {
  throw new Error(`Invalid LOCAL_CONFIG_TARGET: ${target}`);
}

const executableName = goos === "windows" ? "local-config.exe" : "local-config";
const output = resolve(extensionRoot, "bin", executableName);
mkdirSync(dirname(output), { recursive: true });

const result = spawnSync(
  process.env.GO_EXECUTABLE || "go",
  [
    "build",
    "-trimpath",
    "-buildvcs=false",
    "-ldflags=-s -w -buildid=",
    "-o",
    output,
    "./cmd/local-config"
  ],
  {
    cwd: repositoryRoot,
    stdio: "inherit",
    env: {
      ...process.env,
      CGO_ENABLED: "0",
      GOOS: goos,
      GOARCH: goarch,
      GOTOOLCHAIN: "local",
      GOCACHE: process.env.GOCACHE || resolve(extensionRoot, ".go-cache")
    }
  }
);

if (result.status !== 0) {
  process.exit(result.status ?? 1);
}

function platformName(platform) {
  switch (platform) {
    case "win32":
      return "windows";
    case "darwin":
      return "darwin";
    case "linux":
      return "linux";
    default:
      throw new Error(`Unsupported host platform: ${platform}`);
  }
}

function architectureName(architecture) {
  switch (architecture) {
    case "x64":
      return "amd64";
    case "arm64":
      return "arm64";
    default:
      throw new Error(`Unsupported host architecture: ${architecture}`);
  }
}

import { chmod, mkdir } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { build } from "esbuild";

const output = resolve("packages/jetbrains/build/generated-resources/cli/local-config.mjs");

await mkdir(dirname(output), { recursive: true });
await build({
  entryPoints: [resolve("packages/cli/dist/cli.js")],
  outfile: output,
  bundle: true,
  platform: "node",
  format: "esm",
  target: "node20",
  banner: { js: "import { createRequire } from 'node:module'; const require = createRequire(import.meta.url);" },
  legalComments: "none",
});
await chmod(output, 0o755);

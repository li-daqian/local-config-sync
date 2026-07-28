# VS Code 插件设计与实现

## 定位

VS Code 插件是 `local-config` CLI/core 的薄入口，不实现 Repository Driver、mapping 持久化、
`.git/info/exclude`、同步或冲突算法。

```text
VS Code Tree View / QuickPick / Diff
                 |
                 | process + named JSON contract
                 v
          local-config CLI
                 |
                 v
  core / Repository Driver / workspace
```

插件声明为 workspace extension。Remote SSH、WSL、Dev Container 和 Codespaces 场景下，插件和
native CLI 在 workspace 所在的 remote extension host 运行。

首版明确不支持：

- untrusted workspace；
- virtual workspace；
- 没有 remote extension host 的 `vscode.dev` / `github.dev`；
- 自动同步 daemon；
- Webview；
- secret storage。

## MVP 功能

- multi-root workspace：每个 `WorkspaceFolder` 独立调用 `status --project`；
- Repository / file-level 状态 Tree View；
- 手动 Sync，并在执行前展示 upload / download 方向；
- Git authentication 检查；
- GitHub Repository discovery 和单文件 `copy` mapping Setup；
- initial conflict diff 和显式 baseline 选择；
- 真实 conflict 的 built-in diff viewer 和 expected-revision resolve；
- 敏感文件二次确认；
- 自定义 CLI path 高级 override。

插件保存 project 内尚未落盘的 document 后再执行 preview、diff、link、sync 或 resolve。普通状态刷新
读取磁盘状态，mapped file 保存后会 debounce 刷新，但不会触发自动同步。

## 目录

```text
packages/vscode/
  package.json
  esbuild.mjs
  scripts/bundle-cli.mjs
  src/
    extension.ts
    cli/
      client.ts
      locator.ts
      models.ts
    commands/
      coreCommands.ts
      diff.ts
      setup.ts
    state/projectStore.ts
    views/statusTree.ts
    workspace/projectResolver.ts
```

`CliClient` 使用 `child_process.spawn` 和参数数组，禁止通过 shell 拼接命令。`stdout` 只解析 JSON，
非 0 exit code 优先解析稳定 error payload；DTO 使用 Zod 在边界执行 runtime validation。

首次执行命令时调用：

```bash
local-config --version --json
```

当前插件只接受 `contractVersion: 1`。bundled CLI 会按 SHA-256 digest 提取到 VS Code
`globalStorageUri`，Unix 权限设置为 `0700`。

## 本地开发

```bash
pnpm install --frozen-lockfile
pnpm vscode:check
pnpm --dir packages/vscode bundle-cli
```

`bundle-cli` 默认构建当前 host 的 `packages/vscode/bin/local-config`。也可以显式指定 Go target：

```bash
LOCAL_CONFIG_TARGET=linux-arm64 pnpm --dir packages/vscode bundle-cli
```

开发模式下如果 VSIX 内没有 bundled CLI，locator 会回退到仓库根目录的
`build/local-config`。也可以在 VS Code machine setting 中设置：

```json
{
  "localConfigSync.cliPath": "/absolute/path/to/local-config"
}
```

该配置使用 machine scope，不从业务项目 workspace setting 接受 executable override。

## 打包

VSIX 使用 esbuild 后的单文件 extension bundle，不携带 `node_modules`：

```bash
pnpm --dir packages/vscode package --target linux-x64
```

跨平台发布时，Go target 和 VS Code target 必须匹配：

| Go target | VS Code target |
| --- | --- |
| `windows-amd64` | `win32-x64` |
| `windows-arm64` | `win32-arm64` |
| `darwin-amd64` | `darwin-x64` |
| `darwin-arm64` | `darwin-arm64` |
| `linux-amd64` | `linux-x64` / `alpine-x64` |
| `linux-arm64` | `linux-arm64` / `alpine-arm64` |

Linux CLI 使用 `CGO_ENABLED=0`，Alpine VSIX 可以复用相同 architecture 的 Linux binary。
每个 platform-specific VSIX 只携带一个 native CLI。

## 自动发布

`.github/workflows/vscode-extension.yml` 在 pull request 和 `main` 分支上运行 core、CLI、extension
检查，并生成以下八个 platform-specific VSIX：

- `win32-x64`
- `win32-arm64`
- `darwin-x64`
- `darwin-arm64`
- `linux-x64`
- `linux-arm64`
- `alpine-x64`
- `alpine-arm64`

推送 `release-*` tag 时，workflow 从同名 `.release/manifests/<tag>.yaml` 读取 `vscode`
artifact。只有声明了该 artifact 才发布，例如：

```yaml
schemaVersion: 1
releaseId: release-2026.07.28.1
artifacts:
  vscode:
    version: 0.1.0
    channel: default
```

`channel` 支持 `default` 和 `pre-release`。manifest 中的 version 会写入 VSIX，但不会修改
tag 对应 commit 中的 `package.json`。

发布凭据使用 GitHub Repository secret `VSCE_PAT`。它是 Azure DevOps PAT，Organization 必须为
`All accessible organizations`，scope 只选择 `Marketplace > Manage`；PAT 对应 Microsoft
account 必须能管理 publisher `li-daqian`。Visual Studio Marketplace 的全局 Azure DevOps PAT
将在 2026-12-01 停止工作，因此该认证方式需要迁移到 Microsoft Entra workload identity。

## 验证

当前基线：

- `pnpm check`：Go vet、core/CLI tests 和 native CLI build 通过；
- `pnpm vscode:check`：TypeScript typecheck、CLI contract tests 和 esbuild 通过；
- Linux x64 VSIX 已完成隔离目录安装冒烟验证；
- VS Code desktop 基本功能已完成手工验证。

日常修改至少运行：

```bash
pnpm vscode:check
```

CLI/core contract 有改动时同时运行：

```bash
pnpm check
```

发布前还需要补齐 multi-root、WSL 或 Remote SSH、Dev Container，以及
local/remote/conflict/sensitive-file 四类状态的 runtime matrix。mutation command 不提供
强制取消，避免终止进程后遗留 Repository lock；后续应在 core 增加 stale-lock 诊断和显式恢复。

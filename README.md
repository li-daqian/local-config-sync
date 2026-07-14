# Local Config Sync

> 当前 CLI/core 版本：`0.1.0`；JetBrains 插件版本：`0.1.1`。包含独立 CLI/core、Git 与 local-folder Repository Driver，以及 IntelliJ Platform 2026.1+ 插件。

Local Config Sync 是一个面向开发者的本地配置同步工具。它解决的问题是：

> 我希望把某个项目的本地配置文件放在项目目录中使用，方便维护和 IDE 自动识别；但这些文件不能提交到业务仓库。同时，我换电脑后还希望这些本地配置可以自动恢复。

核心方案：

```text
业务项目/config/application-dev.yml
        |
        | copy or symlink
        v
~/.local-config-sync/workspaces/<repository>/<project>/config/application-dev.yml
        |
        | Repository Driver
        v
Git remote / local folder / WebDAV / S3-compatible storage
```

## 产品定位

本项目不是一个只服务 JetBrains 的插件，而是：

```text
core CLI + thin IDE plugin
```

CLI/core 负责同步、Repository Driver、映射、ignore、冲突检测。IDE 插件只负责点选入口和状态展示。

这样以后切换 IDE 时，只需要替换入口层：

```text
JetBrains Plugin -> VS Code Extension -> Cursor Extension -> CLI
```

底层同步能力不重写。

## 当前代码结构

```text
packages/
  core/       TypeScript：领域模型、Repository Driver、mapping、sync、安全策略
  cli/        TypeScript：命令行与稳定 JSON contract
  jetbrains/  Kotlin：Settings、Setup/Auth/Sync action、status widget
tests/        core 与真实 Git 端到端测试
```

JetBrains 插件不直接操作 Git 或配置文件，只调用 `local-config ... --json`。

## 构建与安装

要求：

- Node.js 20+
- pnpm 10+
- 系统 `git` CLI
- 构建插件时需要 JDK 21；Gradle Wrapper 会自动下载 Gradle 9

构建 CLI：

```bash
pnpm install --frozen-lockfile
pnpm build
pnpm local-config -- --help
```

开发环境中可把 CLI 链接到用户 PATH：

```bash
mkdir -p ~/.local/bin
ln -sfn "$(pwd)/packages/cli/dist/cli.js" ~/.local/bin/local-config
```

构建 JetBrains 插件：

```bash
packages/jetbrains/gradlew -p packages/jetbrains \
  -PlocalIdeaPath=/absolute/path/to/idea buildPlugin
```

安装 `packages/jetbrains/build/distributions/local-config-sync-jetbrains-0.1.1.zip` 后，在 IDE 的 `Settings | Tools | Local Config Sync` 中设置 `local-config` 可执行文件的绝对路径。插件当前以 IntelliJ Platform 2026.1（build 261）为最低版本。

本地构建默认禁止自动下载 IntelliJ SDK，避免意外下载数 GB 文件。必须通过
`-PlocalIdeaPath` 使用已有 IDE；确实需要下载时，显式传入
`-PallowIdeSdkDownload=true`。GitHub Actions 已显式启用下载，用于执行完整兼容性矩阵。

开发机已有 IntelliJ 安装时的完整命令：

```bash
packages/jetbrains/gradlew -p packages/jetbrains \
  -PlocalIdeaPath=/absolute/path/to/idea buildPlugin
```

## 典型使用流程

1. 用户在 IDE 中打开业务项目。
2. 插件检测当前项目路径。
3. 用户点击 `Local Config Sync: Setup`。
4. 选择或创建一个配置仓库，例如 Git repo 或 local folder。
5. 选择仓库内目录，例如 `ai-rvis-agent/config/`。
6. 选择当前项目目标目录，例如 `config/`。
7. 工具拉取配置文件到业务项目。
8. 工具将这些路径写入业务项目 `.git/info/exclude`。
9. 用户修改业务项目内配置文件。
10. 工具手动或自动同步到所选仓库。

## MVP 命令草案

```bash
local-config init
local-config repository add git --id personal --url git@github.com:user/private-configs.git
local-config link --project . --repository personal --source-path ai-rvis-agent/config --target config --mode symlink
local-config pull
local-config push
local-config sync
local-config status
local-config doctor
```

当前命令均已实现；机器调用使用 `--json`，失败同时返回稳定 error code 与非 0 exit code。

## Git authentication

Local Config Sync 不保存 Git token、password 或 SSH private key。Git Driver 复用系统 Git 已有认证能力：

```bash
# SSH URL：复用 ssh-agent / SSH key
local-config repository auth personal --method ssh

# HTTPS URL：复用 git credential helper
local-config repository auth personal --method credential

# GitHub CLI：验证 gh auth，并执行 gh auth setup-git
local-config repository auth personal --method gh

# 自动检测并验证远端（默认）
local-config repository auth personal --method auto

# Repository 尚未注册时，先为 URL 配置/验证认证
local-config repository auth --url https://github.com/user/private-configs.git --method gh
```

如果需要交互登录，请先在终端运行 `gh auth login`，或配置系统 credential helper / SSH key。CLI 子进程禁用 Git 的隐式密码提示，避免 IDE background task 卡住。

## 第一版完整流程

```bash
local-config init

local-config repository add git \
  --id personal \
  --url git@github.com:user/private-configs.git \
  --branch main

local-config repository auth personal --method auto

local-config link \
  --project /path/to/business-project \
  --repository personal \
  --source-path business-project/config \
  --target config \
  --mode symlink

local-config status --project /path/to/business-project
local-config sync --project /path/to/business-project
```

插件对应提供：

- `Setup Local Config Sync`：创建 Git Repository 或使用已有 Repository，并建立 mapping。
- `Authenticate Local Config Git Repository`：验证 `auto` / SSH / credential helper / `gh`。
- `Sync Local Config Now`：background task 中执行安全 sync。
- Status Bar Widget：显示 `Synced` / `Pending` / `Conflict` / `Failed` 等 CLI 状态。

## 安全默认值

- 默认不修改业务项目 `.gitignore`。
- 默认写业务项目 `.git/info/exclude`。
- 默认不自动 push，每次同步先做 status 检查。
- 默认不同步 `.env`、private key、token 文件，除非用户显式允许。
- 冲突时停止自动同步，不自动覆盖。
- 远端凭证只保存引用，不写入 Local Config Sync 配置文件。

额外保护：Repository-level lock 阻止本机并发同步；Git push 以 last-synced revision 为条件，远端变化时停止；`--project` 不会提交其他 mapping scope 的 dirty 文件；禁止 force push、`reset --hard` 和自动覆盖冲突。

## 验证

```bash
pnpm check
pnpm plugin:build
pnpm plugin:check
```

`plugin:check` 是日常本地校验，运行项目配置、插件结构检查并生成 ZIP。开发机已有 IntelliJ 安装时，可避免下载 SDK：

```bash
packages/jetbrains/gradlew -p packages/jetbrains \
  -PlocalIdeaPath=/absolute/path/to/idea \
  verifyPluginProjectConfiguration verifyPluginStructure buildPlugin
```

完整二进制兼容性矩阵由 `.github/workflows/jetbrains-plugin.yml` 在 GitHub Actions 中运行，覆盖最低支持版本 IntelliJ IDEA 2026.1.4 与 IntelliJ IDEA 2026.2 RC（build 262.8665.176），并将 deprecated、scheduled-for-removal、internal 和其他 verifier warning 视为失败。CI 会缓存 Gradle 依赖并上传 `pluginVerifier` 报告。

已登录 GitHub CLI 后，可以等待当前分支最新一次 JetBrains workflow，并自动读取失败步骤、下载 verifier report 和扫描关键诊断：

```bash
gh auth status
pnpm ci:status
```

脚本默认要求 workflow 的 head SHA 与本地 `HEAD` 一致，避免误把旧 run 当作当前提交的结果。诊断指定 run 或只查看、不等待时可使用：

```bash
pnpm ci:status -- --run-id <run-id>
pnpm ci:status -- --no-watch
```

如本机已经安装需要验证的目标 IDE，也可以复用该安装目录运行单版本 verifier，不下载额外 SDK：

```bash
packages/jetbrains/gradlew -p packages/jetbrains \
  -PlocalIdeaPath=/absolute/path/to/idea \
  verifyPluginProjectConfiguration verifyPluginStructure verifyPlugin
```

默认也会使用 `localIdeaPath` 执行 verifier；需要用另一个本地 IDE 验证时，再传入
`-PlocalVerifierIdePath=/absolute/path/to/verifier-idea`。

测试使用真实 bare Git Repository 覆盖 push、pull、远端与本地并发修改冲突、删除同步、scope lock、敏感文件阻断和 local-folder Driver。

## 文档索引

- [产品需求](docs/product-requirements.md)
- [架构设计](docs/architecture.md)
- [实现计划](docs/implementation-plan.md)
- [CLI 规格](docs/cli-spec.md)
- [安全模型](docs/security-model.md)
- [JetBrains 插件设计](docs/jetbrains-plugin.md)
- [数据模型](docs/data-model.md)
- [多仓库与 Repository Driver 设计](docs/repository-backends.md)
- [Backlog](docs/backlog.md)

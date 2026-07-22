# Local Config Sync

> 当前 CLI/core 版本：`0.1.0`；JetBrains 插件版本：`0.1.5`。包含独立 CLI/core、Git 与 local-folder Repository Driver，以及 IntelliJ Platform 2026.1+ 插件。

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
  jetbrains/  Kotlin：Settings、Setup/Auth/Sync action、status widget
cmd/
  local-config/      Go：CLI 与稳定 JSON contract
  release-manifest/  Go：Release manifest 校验与标准化
internal/
  core/              Go：领域模型、Repository Driver、mapping、sync、安全策略与测试
  releasemanifest/   Go：Release manifest schema 与校验规则
```

JetBrains 插件不直接操作 Git 或配置文件，只调用 `local-config ... --json`。

## 构建与安装

要求：

- Go 1.26+
- 系统 `git` CLI
- 构建插件时需要 JDK 21；Gradle Wrapper 会自动下载 Gradle 9

构建 CLI：

```bash
go build -trimpath -o build/local-config ./cmd/local-config
./build/local-config --help
```

开发环境中可把 CLI 链接到用户 PATH：

```bash
mkdir -p ~/.local/bin
go build -trimpath -o ~/.local/bin/local-config ./cmd/local-config
```

构建 JetBrains 插件：

```bash
packages/jetbrains/gradlew -p packages/jetbrains \
  -PlocalIdeaPath=/absolute/path/to/idea buildPlugin
```

安装 `packages/jetbrains/build/distributions/local-config-sync-jetbrains-0.1.5.zip` 后即可使用。插件内置 Linux、macOS、Windows 的 amd64/arm64 六个 native CLI binary，不要求用户安装 Node.js；`Settings | Tools | Local Config Sync` 仅保留自定义 CLI 路径作为高级 override。插件当前以 IntelliJ Platform 2026.1（build 261）为最低版本。

## 发布 JetBrains 插件

仓库使用 Release manifest 描述一次发布批次。`release-*` tag 只用于标识和触发发布，不再充当某个平台的版本号；各 artifact 在 manifest 中声明自己的 SemVer 和发布 channel。

每个 tag 必须有一份同名、已提交的 manifest：

```text
.release/manifests/<tag>.yaml
```

例如 `.release/manifests/release-2026.07.22.1.yaml`：

```yaml
schemaVersion: 1
releaseId: release-2026.07.22.1
artifacts:
  jetbrains:
    version: 0.1.6
    channel: default
```

第一版只注册 `jetbrains` artifact。未知字段、未知 artifact、tag 与 `releaseId` 不一致、非法 SemVer、非法 channel 或多 YAML document 都会在构建和发布前失败。未写入 manifest 的平台不构建、不发布；未来新增平台时，应同时注册 manifest artifact 和对应 publisher workflow。

可以从 `.release/manifest.example.yaml` 复制并编辑，然后在打 tag 前本地校验：

```bash
cp .release/manifest.example.yaml \
  .release/manifests/release-2026.07.22.1.yaml

go run ./cmd/release-manifest \
  --file .release/manifests/release-2026.07.22.1.yaml \
  --tag release-2026.07.22.1
```

校验并提交 manifest 后，创建完全一致的 tag：

```bash
git tag release-2026.07.22.1
git push origin release-2026.07.22.1
```

GitHub Actions 从 tag 对应 commit 读取 manifest。包含 `jetbrains` 时才执行六平台 CLI 构建、完整 Plugin Verifier、签名，并将 manifest 中的 version/channel 传给 `publishPlugin`。同一个 tag 未来可以列出多个独立版本的平台，由各 publisher 并行处理；发布失败时重新运行 failed jobs，不移动 tag、不修改该 tag 下的 manifest。

发布前只需要在仓库的 `Settings | Secrets and variables | Actions` 中配置一个 Repository secret：

- `JETBRAINS_PUBLISH_TOKEN`：JetBrains Marketplace Profile 的 `My Tokens` 页面生成的 Personal Access Token。workflow 仅在 JetBrains publish job 内将其映射为 Gradle 使用的 `PUBLISH_TOKEN` 环境变量，避免与其他平台的发布凭据混淆。

publish job 会在 GitHub-hosted runner 的临时目录中生成一次性 RSA private key、自签名证书和随机密码，并仅在当前 Gradle `publishPlugin` 进程中用于作者签名；这些签名材料不会写入仓库、GitHub Secrets 或 workflow artifact。当前发布目标是 JetBrains Marketplace，因此不需要在发布之间复用作者签名身份；如果未来需要直接分发 GitHub artifact，或 Marketplace 支持并要求绑定作者公钥，应改为受保护的长期 signing key。

首次发布必须先在 JetBrains Marketplace 手动创建插件并上传一个已签名版本；Marketplace 接受该插件 ID 后，后续新版本才可由 `publishPlugin` 自动发布。首次 tag run 即使因插件尚未创建而发布失败，只要签名已完成，workflow artifact 中仍会保留 `*-signed.zip`，可用于首次手动上传。插件 ID 固定为 `io.github.localconfigsync.jetbrains`，Marketplace 不接受重复 artifact 版本，因此每次发布必须在 manifest 中填写尚未发布的新版本。

也可以使用已登录的 GitHub CLI 配置 Secret；命令会交互读取 token，避免进入 shell history：

```bash
gh secret set JETBRAINS_PUBLISH_TOKEN
```

插件右侧 `Local Config Sync` Tool Window 以表格展示本地文件、Repository 文件及 file-level 同步状态，并提供新增 Mapping、diff、显式冲突解决、Sync、Git Auth 和 Refresh。`Sync Now` 作为顶部主操作展示，Project、Repository 与最近同步时间使用只读摘要，不再注册底部状态栏组件。

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
3. 用户点击 `Local Config Sync: Setup` 并选择 GitHub。
4. 插件复用 `gh auth` 完成认证，并列出当前账号名下的 public/private Repository。
5. 选择 Repository，再选择已有远端文件或本地文件。
6. 远端已有文件时选择项目内的本地文件夹，工具保留远端文件名并将文件同步到该文件夹；本地已有文件时选择 Repository 中的目标路径。
7. 只有本地或只有远端文件时，工具自动采用已有一侧初始化；两侧文件不同时先展示 diff，再由用户明确选择初始版本。
8. 工具建立 file mapping，并将项目文件路径写入业务项目 `.git/info/exclude`。
9. 用户修改业务项目内配置文件，点击 `Sync Now` 安全同步。

## MVP 命令草案

```bash
local-config init
local-config repository add git --id personal --url git@github.com:user/private-configs.git
local-config link --project . --repository personal --source-path ai-rvis-agent/config --target config --mode symlink
local-config pull
local-config push
local-config sync
local-config status
local-config diff
local-config resolve
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

# 单文件首次同步；双方都存在且不同时必须显式选择 local 或 remote
local-config preview \
  --project . \
  --repository personal \
  --source-path ai-rvis-agent/application-dev.yml \
  --target src/main/resources/application-dev.yml \
  --kind file

local-config link \
  --project . \
  --repository personal \
  --source-path ai-rvis-agent/application-dev.yml \
  --target src/main/resources/application-dev.yml \
  --kind file \
  --mode copy \
  --initial-strategy local

local-config status --project /path/to/business-project
local-config sync --project /path/to/business-project
```

插件对应提供：

- `Setup Local Config Sync`：GitHub 认证、Repository/file picker、initial diff 和 file mapping。
- `Authenticate Local Config Git Repository`：验证 `auto` / SSH / credential helper / `gh`。
- `Sync Local Config Now`：background task 中执行安全 sync。
- Tool Window file table：显示每个文件的 `Synced` / `Push required` / `Update available` / `Conflict`，并提供 diff 与显式冲突解决。

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
go vet ./...
go test ./...
go build -trimpath -o build/local-config ./cmd/local-config
packages/jetbrains/gradlew -p packages/jetbrains buildPlugin
packages/jetbrains/gradlew -p packages/jetbrains test verifyPluginProjectConfiguration verifyPluginStructure buildPlugin
```

根目录仍保留 `pnpm check`、`pnpm plugin:build` 和 `pnpm plugin:check` 作为兼容的 task alias，但 core/CLI 的构建和运行不依赖 Node.js。

`plugin:check` 是日常本地校验，运行项目配置、插件结构检查并生成 ZIP。开发机已有 IntelliJ 安装时，可避免下载 SDK：

```bash
packages/jetbrains/gradlew -p packages/jetbrains \
  -PlocalIdeaPath=/absolute/path/to/idea \
  verifyPluginProjectConfiguration verifyPluginStructure buildPlugin
```

完整验证由 `.github/workflows/jetbrains-plugin.yml` 在 GitHub Actions 中运行：native CLI 会分别在 Linux、macOS、Windows 的 amd64/arm64 runner 上构建并执行；插件兼容性覆盖最低支持版本 IntelliJ IDEA 2026.1.4 与 IntelliJ IDEA 2026.2 RC（build 262.8665.176），并将 deprecated、scheduled-for-removal、internal 和其他 verifier warning 视为失败。CI 会缓存 Go/Gradle 依赖、在 Job Summary 中按 IDE 展示 verdict 和问题分类、生成 GitHub warning/error annotations，并上传完整 `pluginVerifier` 报告。

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

# 实现计划

## Phase 0: 项目脚手架

目标：

- 建立 CLI/core 项目。
- 明确语言、包结构、测试入口。
- 明确跨 IDE / desktop / web 的调用边界。

最终选择 Go native core/CLI。早期 TypeScript MVP 用于稳定产品边界和 CLI contract；在 JetBrains 插件发行阶段迁移到 Go，消除用户侧 Node.js runtime 要求。

原因：

- Go 可以构建无 CGO 的 Linux、macOS、Windows amd64/arm64 单文件 binary。
- 所有入口继续复用稳定 CLI + JSON contract，不依赖某个 IDE 的语言生态。
- JetBrains 插件可以内置六个平台 binary，运行时无需额外 runtime。
- Desktop、VS Code / Cursor 和未来 local agent 继续调用同一行为源。

阶段性取舍：

- 第一版不追求单一语言覆盖所有入口，而是追求一个稳定 core 行为源。
- Kotlin 只用于 JetBrains 插件入口层，不承载同步逻辑。
- core 与 CLI 一起迁移，不增加只承载部分逻辑的 native helper，避免双实现。
- 公共命令输出必须有命名响应模型，不使用长期传播的匿名 object bag。

建议包结构：

```text
cmd/local-config/ # Go CLI 和 JSON contract adapter
internal/core/    # Go 同步、Repository Driver、映射、ignore、冲突检测
packages/
  vscode/        # 可选，调用 native CLI
  jetbrains/     # Kotlin，内置并调用 native CLI
  desktop/       # 调用 CLI 或 local agent
  local-agent/   # 可选，本机 HTTP/IPC 服务，复用 Go core
```

## Phase 1: CLI MVP

核心文件建议：

```text
cmd/local-config/main.go
internal/core/
  service.go
  model.go
  repositories.go
  driver.go
  git-driver.go
  local-folder-driver.go
  mappings.go
  linker.go
  ignore.go
  storage.go
  files.go
  process.go
```

命令：

```bash
local-config init
local-config repository add
local-config repository list
local-config repository doctor
local-config link
local-config pull
local-config push
local-config sync
local-config status
local-config doctor
```

验收：

- 能在任意 Git 项目中建立映射。
- 能同时配置多个 Repository 实例。
- Git Driver 支持标准 Git URL，不绑定 GitHub。
- Local Folder Driver 支持本地目录、NAS mount 和同步盘目录。
- 能把配置文件 link 到业务项目。
- 能写 `.git/info/exclude`。
- 能按 Mapping scope 安全 pull/push，scope 外 dirty 时停止。
- Repository-level lock 能阻止同一 workspace 并发同步。
- Driver 缺少条件写或等价冲突保护时拒绝覆盖式 push。
- `status`、`link`、`sync` 支持 `--json`，且 `stdout` 只输出机器可读 JSON。
- 命令失败时返回非 0 exit code，插件侧不依赖错误文本判断成功失败。

第一阶段只把 `git` 和 `local-folder` 标记为稳定 Driver。详细契约见[多仓库与 Repository Driver 设计](repository-backends.md)。

## Phase 2: WebDAV / S3 Driver

能力：

- WebDAV ETag 和条件请求。
- S3 version ID、ETag 或等价条件写。
- snapshot manifest / commit marker。
- 发布失败恢复和 orphan cleanup。

验收：

- 两台设备同时修改不会静默覆盖。
- 不完整上传不会成为当前快照。
- 配置文件不保存明文凭证。
- capability 不足时明确拒绝不安全 push。

## Phase 3: 自动同步

能力：

- 文件监听 mapped paths。
- debounce 30s/60s。
- 自动执行安全 `sync`。
- 失败后进入 paused 状态。

验收：

- 连续保存不会产生大量 commit。
- 网络失败不会丢文件。
- 冲突时不自动覆盖。

## Phase 4: JetBrains Plugin MVP

插件职责：

- 展示 Tool Window 或 Settings 页。
- 识别当前 project path。
- 使用 Kotlin 调用 `local-config` CLI。
- 提供 Setup、Sync Now、Status。
- 状态栏显示同步状态。
- 将 CLI JSON 解析为命名 DTO。

插件不实现：

- Git 同步细节。
- Repository Driver 选择和同步细节。
- 文件 merge。
- 配置格式解析。
- Git、WebDAV、S3 等后端命令或 SDK 编排。

实现约束：

- 不在 UI 线程执行 `sync`、`pull`、`push`。
- CLI 路径允许用户配置，不能假设 GUI 启动环境一定有正确 `PATH`。
- 优先调用 `local-config` 可执行文件，不直接调用 `node dist/cli.js`。
- `stdout` 只解析 JSON，`stderr` 只作为诊断展示或日志输入。

## Phase 5: 安全增强

能力：

- 敏感文件 pattern 警告。
- 可选本地加密。
- secret provider integration。
- sync audit log。

## Phase 6: 多入口

候选入口：

- VS Code extension。
- Cursor extension。
- Desktop tray app。
- Web UI。

前提：

- CLI/core API 稳定。
- 状态文件格式稳定。
- CLI JSON contract 稳定。
- 错误码、状态枚举和响应模型稳定。

入口策略：

- VS Code / Cursor extension：调用 native CLI，用户可见行为与其他入口保持一致。
- Desktop tray app：调用 CLI 或启动 local agent，避免复制同步算法。
- Web UI：只访问本机 local agent，不直接访问文件系统或配置仓库。
- local agent：作为 CLI contract 的服务化封装，不引入新的业务规则来源。

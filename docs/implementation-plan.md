# 实现计划

## Phase 0: 项目脚手架

目标：

- 建立 CLI/core 项目。
- 明确语言、包结构、测试入口。
- 明确跨 IDE / desktop / web 的调用边界。

候选技术：

- TypeScript + Node.js：适合写 CLI，也适合被 JetBrains 插件通过进程调用。
- Rust：二进制部署体验好，但插件集成和快速迭代成本更高。
- Kotlin：和 JetBrains 插件生态一致，但跨 IDE 复用不如 CLI 简洁。
- Go：单二进制部署体验好，但与 VS Code / Web / Electron 生态复用不如 TypeScript。

建议 MVP 使用 TypeScript + Node.js。

原因：

- Git、文件系统、远端存储 SDK、文件监听和 CLI 生态成熟。
- 后续 VS Code extension 复用方便。
- JetBrains 插件可直接调用打包后的 CLI。
- Desktop tray app 可以调用 CLI 或 local agent。
- Web UI 必须通过本机 local agent 访问文件系统和 Repository Driver，TypeScript core 可以继续复用。

阶段性取舍：

- 第一版不追求单一语言覆盖所有入口，而是追求一个稳定 core 行为源。
- Kotlin 只用于 JetBrains 插件入口层，不承载同步逻辑。
- Rust/Go 可作为未来 native helper 或重写 CLI 的候选，但必须等 CLI JSON contract 稳定后再评估。
- 公共命令输出必须有命名响应模型，不使用长期传播的匿名 object bag。

建议包结构：

```text
packages/
  core/        # TypeScript，同步、Repository Driver、映射、ignore、冲突检测
  cli/         # TypeScript，命令行入口和 JSON contract
  vscode/      # TypeScript，VS Code extension，调用 CLI 或复用 client
  jetbrains/   # Kotlin，JetBrains UI，调用 CLI
  desktop/     # Electron/Tauri UI，调用 CLI 或 local agent
  local-agent/ # 可选，本机 HTTP/IPC 服务，供 Web UI / desktop 调用
```

## Phase 1: CLI MVP

核心文件建议：

```text
src/
  cli.ts
  core/
    project-resolver.ts
    repository-registry.ts
    mapping-manager.ts
    file-linker.ts
    ignore-manager.ts
    sync-orchestrator.ts
    conflict-detector.ts
    repository-driver.ts
    drivers/
      git-driver.ts
      local-folder-driver.ts
  model/
    config.ts
    repository.ts
    mapping.ts
    command-result.ts
    status.ts
  util/
    fs.ts
    process.ts
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

- VS Code / Cursor extension：优先复用 TypeScript client 或调用 CLI，但用户可见行为要与 CLI 一致。
- Desktop tray app：调用 CLI 或启动 local agent，避免复制同步算法。
- Web UI：只访问本机 local agent，不直接访问文件系统或配置仓库。
- local agent：作为 CLI contract 的服务化封装，不引入新的业务规则来源。

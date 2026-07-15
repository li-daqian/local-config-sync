# 架构设计

## 总览

```text
┌─────────────────────────────────────────────────────────────┐
│ Entry Layer                                                  │
│                                                             │
│  JetBrains Plugin   VS Code Extension   CLI   Tray App       │
└───────────────┬─────────────────────────────────────────────┘
                │
                v
┌─────────────────────────────────────────────────────────────┐
│ local-config-core                                            │
│                                                             │
│  Project Resolver                                            │
│  Repository Registry                                         │
│  Mapping Manager                                             │
│  File Linker                                                 │
│  Ignore Manager                                              │
│  Sync Orchestrator                                           │
│  Conflict Detector                                           │
│  Repository Driver Registry                                  │
└───────────────┬─────────────────────────────────────────────┘
                │
                v
┌─────────────────────────────────────────────────────────────┐
│ Managed Workspace + Repository Drivers                       │
│                                                             │
│  Business Project / .git/info/exclude                        │
│  Git / Local Folder / WebDAV / S3-compatible storage         │
└─────────────────────────────────────────────────────────────┘
```

架构同时支持多个 Repository 实例和多种 Repository 类型。所有后端先同步到 managed local workspace，`File Linker` 只处理 workspace 与业务项目的文件关系。详细 Driver 契约见[多仓库与 Repository Driver 设计](repository-backends.md)。

## 语言与入口边界

核心实现语言使用 Go，发布为不依赖语言 runtime 的 native CLI。

原因：

- CLI/core 可以作为单文件二进制分发，不要求 IDE 用户安装 Node.js 等额外 runtime。
- 文件系统、进程调用、JSON/YAML 和跨平台构建均可由 pure Go 实现。
- JetBrains、VS Code / Cursor 和 Desktop 入口统一通过 CLI + JSON contract 调用，不复制同步算法。
- Web UI 无法直接访问用户本机文件和 Git，仍必须通过 Go local agent 间接调用 core。

边界原则：

- `local-config-core` 保持 IDE 中立，不依赖 JetBrains SDK、VS Code API、Electron、浏览器 API。
- Entry Layer 只负责 UI、当前项目路径识别、命令触发、状态展示和错误展示。
- 跨入口的稳定契约优先使用 CLI + JSON，而不是共享 IDE 内部对象。
- VS Code extension 通过进程调用 native CLI；共享的是命名 JSON contract，而不是语言内部类型。
- JetBrains 插件使用 Kotlin 实现 UI，通过 `local-config` CLI 调用 core。

推荐调用形态：

```text
+--------------------+      CLI process / JSON      +---------------------+
| JetBrains Plugin   | ---------------------------> | local-config CLI    |
| Kotlin             |                              | Go native binary    |
+--------------------+                              +----------+----------+
                                                                  |
+--------------------+      CLI process / JSON                    v
| VS Code Extension  | ---------------------------> +---------------------+
| TypeScript         |                              | local-config-core   |
+--------------------+                              +----------+----------+
                                                                  |
+--------------------+      localhost HTTP / JSON                 v
| Web UI             | ---------------------------> +---------------------+
| Browser            |                              | local agent         |
+--------------------+                              +---------------------+
```

JetBrains 插件同时内置 `linux`、`darwin`、`windows` 的 `amd64` / `arm64` 六个 target，并在运行时只提取当前平台的 binary。Git Driver 仍复用系统 `git` 和用户已有凭证。

## 关键模块

### Entry Layer

职责：

- 识别当前项目路径。
- 提供 Setup / Sync / Status 操作入口。
- 展示同步状态和错误。
- 调用 CLI/core，并解析机器可读 JSON 结果。

不负责：

- Repository Driver 和同步算法。
- 文件冲突处理策略。
- 配置映射持久化格式。
- 直接依赖配置仓库的 Driver 实现细节。

### Project Resolver

职责：

- 确认当前目录是否是 Git 项目。
- 找到业务项目根目录。
- 找到 `.git/info/exclude`。
- 识别已有映射。

### Mapping Manager

职责：

- 保存业务项目和 Repository `sourcePath` 的映射。
- 支持多个业务项目。
- 支持多个 Repository 实例，以及同一 Repository 下多个非重叠项目目录。

### Repository Registry

职责：

- 保存 Repository 实例及其命名配置。
- 根据 `type` 解析和校验 Driver-specific options。
- 管理每个 Repository 的 workspace、状态和锁路径。
- 配置文件只保存 `credentialRef`，不保存远端明文凭证。

### File Linker

职责：

- 支持 `copy` 和 `symlink` 两种模式。
- 默认建议 `symlink`，修改即同步源文件。
- Windows 下需要考虑 symlink 权限，必要时 fallback 到 copy。

### Ignore Manager

职责：

- 默认写业务项目 `.git/info/exclude`。
- 不默认修改 `.gitignore`。
- 写入前检查是否已有规则。
- 支持删除映射时清理规则。

### Repository Driver Registry

职责：

- 按 Repository `type` 选择 Git、Local Folder、WebDAV 或 S3 Driver。
- 通过 `prepare`、`inspect`、`pull`、`push`、`doctor` 统一同步语义。
- 将 commit SHA、ETag、version ID 等后端状态归一化为 remote revision。
- 将底层失败映射为公共错误码和 conflict 结果。
- 第一阶段实现 Git Driver 和 Local Folder Driver。

Git Driver 使用系统 git CLI，不对业务项目执行提交操作。GitHub、GitLab、Gitee 和自建 Git 共用同一个 Driver。

### Sync Orchestrator

职责：

- 编排 pull/push/sync 流程。
- 检查 workspace、mapping scope 和 Repository 状态。
- 获取 Repository-level lock，禁止并发修改同一 workspace。
- 上传前执行敏感文件检查。
- 执行 debounce 后的自动同步。
- 在 conflict 风险时停止。

### CLI Contract Adapter

职责：

- 将 core 的强类型结果转换为 CLI JSON 响应。
- 统一 exit code、stdout、stderr 语义。
- 为 IDE 插件、desktop app 和 local agent 提供稳定的命令边界。

约束：

- `stdout` 在 `--json` 模式下只输出机器可读 JSON。
- `stderr` 用于人类可读诊断信息和底层命令错误。
- 非 0 exit code 表示命令失败，调用方不得仅靠解析错误文本判断失败类型。
- 公共响应应定义命名模型，避免长期传播匿名 object bag。

## 同步模式

### Symlink 模式

```text
business-project/config/application-dev.yml
        -> ~/.local-config-sync/workspaces/personal-git/ai-rvis-agent/config/application-dev.yml
```

优点：

- 修改业务项目文件即修改 Repository workspace 文件。
- 不需要额外 copy back。
- 状态简单。

缺点：

- 路径依赖更强。
- Windows 下 symlink 体验可能较差。

### Copy 模式

```text
Repository workspace file -> business project file
business project file -> Repository workspace file
```

优点：

- 跨平台更稳。
- 文件真实存在于业务项目。

缺点：

- 需要双向同步。
- 容易出现两个副本不一致。

MVP 默认使用 `symlink`，提供 `copy` 作为 fallback。

## 本地状态

建议用户级状态目录：

```text
~/.local-config-sync/
  config.yml
  repositories.yml
  mappings.yml
  workspaces/<repository-id>/
  state/repositories/<repository-id>.json
  locks/<repository-id>.lock
  logs/
```

Mapping 中只保存 `repositoryId` 和 workspace 内相对 `sourcePath`，不重复保存 Repository 的真实路径或凭证。

Git Repository 的 managed workspace 示例：

```text
~/.local-config-sync/workspaces/personal-git/
  ai-rvis-agent/config/application-dev.yml
```

## 通用同步策略

`sync` 建议流程：

```text
validate repository and mapping scope
acquire repository lock
inspect local / remote / last-synced baseline
if conflict risk: stop
pull through Repository Driver
copy reconciliation / symlink validation
scan sensitive files
push with expected remote revision
update state and release lock
```

禁止：

- 缺少条件写或等价保护时静默覆盖远端。
- 自动覆盖冲突文件。
- Git Driver 自动执行 `git reset --hard` 或 force push。
- `sync --project` 提交当前 Mapping scope 以外的 dirty 文件。

发现 scope 外的 dirty 文件时返回 `repository_dirty_outside_scope`。同一个 Repository 正在同步时返回 `repository_locked`。

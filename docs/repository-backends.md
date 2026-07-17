# 多仓库与 Repository Driver 设计

## 目标

Local Config Sync 需要同时支持两个维度：

- 多个仓库实例：例如个人配置仓库、公司配置仓库和家庭 NAS 可以同时存在。
- 多种仓库类型：例如 Git、local folder、WebDAV 和 S3-compatible storage。

GitHub、GitLab、Gitee 和自建 Git 服务都使用标准 Git 协议，统一由 `git` Driver 处理。只有仓库选择器、OAuth 等入口体验需要 provider-specific integration，不应复制同步算法。

第一阶段不把 secret provider 纳入 Repository Driver。Repository Driver 同步文件，secret provider 负责运行时注入敏感值，两者有不同的安全语义和生命周期。

## 核心架构

```text
┌───────────────────────────────────────────────────────────┐
│ IDE Plugin / CLI / Tray App                               │
└──────────────────────────┬────────────────────────────────┘
                           v
┌───────────────────────────────────────────────────────────┐
│ Sync Orchestrator                                         │
│ mapping scope / lock / conflict / safety / result model   │
└──────────────┬────────────────────────────┬───────────────┘
               v                            v
┌──────────────────────────┐     ┌──────────────────────────┐
│ Repository Registry      │     │ File Linker              │
│ repository instances     │     │ symlink / copy           │
└──────────────┬───────────┘     └─────────────┬────────────┘
               v                              v
┌──────────────────────────┐       managed local workspace
│ Repository Driver        │                  ^
├──────────────────────────┤                  │
│ Git Driver               │──────────────────┘
│ Local Folder Driver      │
│ WebDAV Driver            │
│ S3 Driver                │
└──────────────────────────┘
```

所有 Driver 必须向 core 提供一个本地 workspace；远端后端需要先 materialize 到该 workspace。业务项目只与 workspace 建立 `symlink` 或 `copy` 关系，`File Linker` 不感知 Git、WebDAV 或 S3。

这个边界可以避免：

- 在 File Linker 中加入 provider 判断。
- 让 IDE 插件依赖某一种远端 SDK。
- 为每种后端重新实现 mapping、ignore 和敏感文件检查。

## 领域模型

### Repository

Repository 表示一个用户配置的逻辑仓库实例：

```go
type Repository struct {
    ID            string
    Name          string
    Type          string
    WorkspacePath string
    CredentialRef string
    Options       RepositoryOptions
}

// Type 当前支持 "git" 和 "local-folder"，并为 "webdav"、"s3" 保留扩展边界。
```

`options` 在持久化边界和 core 内部必须解析为按 `type` 区分的命名类型，例如 `GitRepositoryOptions`、`WebDavRepositoryOptions`，不能长期使用无校验的动态 object bag。

### Mapping

Mapping 不再持有 `privateRepoPath`，只引用 `repositoryId`：

```go
type Mapping struct {
    ID           string
    ProjectPath  string
    RepositoryID string
    SourcePath   string
    TargetPath   string
    Mode         LinkMode
    Kind         MappingKind
}
```

`sourcePath` 是仓库 workspace 内的相对路径。它取代带有远端实现含义的 `remotePath`。

`kind` 区分单文件和目录 mapping。文件 mapping 可以在不替换既有父目录的情况下同步 `src/main/resources/application-dev.yml` 等文件。

同一 Repository 中的 Mapping 默认禁止 `sourcePath` 重叠，避免一个文件被多个项目以不同同步范围提交。未来如需共享只读配置，应增加显式的 `readOnly` 模式，而不是放宽默认约束。

### Repository State

运行时状态单独保存在用户状态目录，不写入业务项目：

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

Repository state 至少记录：

- 上次成功同步的 remote revision。
- 每个 mapped file 的 hash、size 和删除状态。
- 上次同步时间和最近一次失败。
- Driver 用于并发控制的 ETag、version ID 或 commit SHA。

## Repository Driver 契约

公共接口使用同步语义，不暴露 `commit`、`rebase` 等 Git 专属概念：

```go
type RepositoryDriver interface {
    Prepare(Repository) error
    Inspect(DriverContext) (RepositoryStatus, error)
    Snapshot(DriverContext, revision string) (map[string]FileSnapshot, error)
    ReadFile(DriverContext, revision string, path string) ([]byte, bool, error)
    RestoreWorkspace(DriverContext) error
    Pull(DriverContext) (PullResult, error)
    Push(DriverContext, string) (PushResult, error)
    Doctor(Repository) (DiagnosticResult, error)
}
```

语义约束：

- `prepare` 创建或验证 managed workspace。
- `inspect` 返回统一状态和当前 remote revision。
- `snapshot` 和 `readFile` 按指定 revision 提供 backend-neutral 的文件视图，用于 file-level status 与 diff；不得修改业务项目或 workspace working tree。
- `restoreWorkspace` 仅在用户显式解决 `copy` 单文件 conflict 后，将指定 workspace scope 恢复到 Driver 的 local revision；业务项目副本仍保留用户选择，Git Driver 不得使用 `reset --hard`。
- `pull` 只做安全更新；不能覆盖未同步的本地修改。
- `push` 必须接受 expected revision；远端已变化时返回 conflict。
- `doctor` 检查工具、网络、凭证、路径和后端能力。
- Driver 将底层错误归一化为命名错误码，不让入口层解析错误文本。

Driver 可以声明 `history`、`conditionalWrite`、`atomicPublish` 等 capability，供 CLI/UI 展示。但是安全同步要求不能因为 capability 缺失而静默降级：不能提供条件写或等价冲突保护的后端，默认不得执行可覆盖远端的 push。

## 同步与冲突

统一同步流程：

```text
acquire repository lock
        |
inspect local / remote / baseline
        |
conflict? -------- yes ------> stop and report
        |
pull remote changes
        |
reconcile copy / validate symlink
        |
sensitive-file scan
        |
conditional push(expected revision)
        |
update state and release lock
```

对于支持文件级快照的 Driver，使用 last-synced baseline 做三方判断：

| 本地 | 远端 | 结果 |
|---|---|---|
| 未变化 | 已变化 | pull |
| 已变化 | 未变化 | push |
| 相同变化 | 相同变化 | 更新 baseline |
| 不同变化 | 不同变化 | conflict |

文件删除也是一种变化。删除与另一端修改同时发生时必须报告 conflict。

### Git Driver

- workspace 是本地 clone。
- remote revision 使用 upstream commit SHA。
- 使用系统 `git` CLI 和用户已有 credential。
- 可以使用 fetch/rebase 实现安全更新，但冲突时必须停止。
- 禁止自动 force push、`reset --hard` 和覆盖冲突。
- push 只能 stage 当前 sync scope 下的 mapped paths。

### Local Folder Driver

- workspace 可以直接指向用户选择的目录。
- 适用于 NAS mount、同步盘目录和本地备份盘。
- Repository 本身就是该目录，因此 Driver 不模拟额外的 remote push；通过文件快照、写前复检和原子 rename 保护 Local Config Sync 自己的写入。
- 对 `symlink` mapping，外部同步冲突主要由同步盘客户端处理，Local Config Sync 仍记录文件快照并展示异常变化。
- 对 `copy` mapping，使用本地 manifest 做双向变化检测。

### WebDAV / S3 Driver

- WebDAV 使用 ETag 和条件请求；S3 使用 version ID、ETag 或条件写能力。
- 远端不具备目录级原子事务时，Driver 必须采用 snapshot manifest/commit marker，或明确返回不支持 atomic publish。
- 发布过程中失败不能把不完整快照标记为当前版本。
- 凭证必须通过 `credentialRef` 从 OS keychain、环境或 provider SDK 默认凭证链读取。

## 同步范围和并发

- 锁粒度是 Repository，因为一次 pull 可能影响同一仓库中的多个 Mapping。
- `sync --project` 只允许发布当前项目 Mapping 范围内的变化。
- 如果发现 scope 外存在 dirty 文件，停止并返回 `repository_dirty_outside_scope`，不能顺手提交其他项目配置。
- `sync --repository <id>` 可以显式同步该仓库下所有已注册 Mapping。
- 同一 Repository 的并发操作返回 `repository_locked`，调用方可以稍后重试。

## CLI 方向

Repository 成为一级资源：

```bash
local-config repository add git \
  --id personal-git \
  --url git@github.com:user/private-configs.git

local-config repository add local-folder \
  --id nas \
  --path /mnt/nas/configs

local-config repository list
local-config repository doctor personal-git

local-config link \
  --project . \
  --repository personal-git \
  --source-path ai-rvis-agent/config \
  --target config \
  --mode symlink
```

`init` 只初始化用户目录和默认策略，不再绑定唯一仓库。`pull`、`push`、`sync` 保留用户熟悉的命令名，但它们表达的是下载、发布和双向同步语义，不代表底层一定是 Git。

## 实施顺序

1. 建立 Repository Registry、Repository Driver 接口和新的 Mapping 模型。
2. 实现 Git Driver，覆盖 GitHub、GitLab、Gitee 和自建 Git。
3. 实现 Local Folder Driver，用第二种后端验证抽象没有泄漏 Git 语义。
4. 在统一 conflict contract 稳定后实现 WebDAV Driver。
5. 根据真实需求实现 S3 Driver 和 provider-specific repository picker。

第一阶段以 `git` 和 `local-folder` 作为正式支持的 Driver。WebDAV 和 S3 在 capability、条件写和故障恢复测试完备后再标记为稳定。

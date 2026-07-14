# JetBrains 插件设计

## 插件定位

JetBrains 插件只是入口层，不承载核心同步逻辑。

职责：

- 识别当前打开项目路径。
- 提供 setup wizard。
- 调用 `local-config` CLI。
- 展示状态和错误。
- 提供 `Sync Now` 操作。
- 提供 Git authentication 验证入口，调用 CLI 的 `repository auth`。

不做：

- Repository Driver 和同步算法。
- 复杂 conflict 处理。
- secret 存储。
- 直接修改业务项目 Git history。
- 直接 import 或复刻 Node.js core 内部模块。

## Kotlin 调用方式

JetBrains 插件通过子进程调用 `local-config` CLI，而不是直接调用 Node.js 模块。

调用边界：

```text
+--------------------+      process + JSON      +-------------------+
| JetBrains Plugin   | -----------------------> | local-config CLI  |
| Kotlin             |                          | TypeScript/Node   |
+--------------------+                          +---------+---------+
                                                             |
                                                             v
                                                   +-------------------+
                                                   | local-config-core |
                                                   +-------------------+
```

推荐使用 IntelliJ Platform 的 `GeneralCommandLine` 或等价 API：

```kotlin
val commandLine = GeneralCommandLine()
    .withExePath(cliPath)
    .withParameters("status", "--project", projectPath, "--json")
    .withWorkDirectory(projectPath)

val output = ExecUtil.execAndGetOutput(commandLine)

if (output.exitCode != 0) {
    throw RuntimeException(output.stderr)
}

val status = Json.decodeFromString<StatusResponse>(output.stdout)
```

约束：

- 插件优先调用 `local-config` 可执行文件，不直接调用 `node dist/cli.js`。
- CLI 路径必须允许用户在 Settings 中配置，不能假设 IDE 进程继承了用户 shell 的 `PATH`。
- `sync`、`pull`、`push` 必须在 background task 中执行，不能阻塞 UI 线程。
- `stdout` 在 `--json` 模式下只按 JSON 解析。
- `stderr` 仅作为诊断信息展示或写入日志。
- Kotlin 侧定义命名 DTO，例如 `StatusResponse`、`MappingSummary`，不要长期使用 `Map<String, Any>` 传播状态。

## UI 入口

建议入口：

- Status Bar Widget：显示 `Synced` / `Pending` / `Failed` / `Conflict`。
- Settings Page：配置 CLI 路径、默认 Repository 和 sync 策略。
- Project Context Action：右键项目目录，`Setup Local Config Sync`。
- Tool Window：MVP 可不做，避免重 UI。

## Setup Wizard

步骤：

1. 检查 CLI 是否安装。
2. 检查当前项目是否是 Git repo。
3. 选择或创建 Repository 实例。
4. 选择 Repository 内的 source path。
5. 选择 target path。
6. 选择 link mode：`symlink` 或 `copy`。
7. 预览将写入的文件和 `.git/info/exclude` 规则。
8. 执行 `local-config link`。

## 状态刷新

插件通过调用：

```bash
local-config status --json --project <project>
```

获取状态。

示例响应模型：

```kotlin
@Serializable
data class StatusResponse(
    val ok: Boolean,
    val projectPath: String,
    val state: SyncState,
    val repositories: List<RepositorySummary>,
    val mappings: List<MappingSummary>
)

@Serializable
data class RepositorySummary(
    val id: String,
    val name: String,
    val type: RepositoryType,
    val state: SyncState
)

@Serializable
data class MappingSummary(
    val repositoryId: String,
    val sourcePath: String,
    val targetPath: String,
    val mode: LinkMode,
    val excludeConfigured: Boolean
)

enum class SyncState {
    NOT_CONFIGURED,
    SYNCED,
    PENDING,
    SYNCING,
    FAILED,
    CONFLICT,
    PAUSED
}

enum class LinkMode {
    SYMLINK,
    COPY
}

enum class RepositoryType {
    GIT,
    LOCAL_FOLDER,
    WEBDAV,
    S3
}
```

建议状态：

```text
not_configured
synced
pending
syncing
failed
conflict
paused
```

## 自动同步

第一版不建议插件自己监听文件并直接同步。

更好的方式：

- CLI 提供 `watch` 或 daemon。
- 插件只负责启动/停止和展示状态。

## 错误处理

常见错误：

- CLI 未安装。
- Repository 不存在或 Driver 未配置。
- Git/WebDAV/S3 auth 失败。
- `.git/info/exclude` 不可写。
- symlink 失败。
- Repository 有冲突。
- 条件 push 被拒绝或 Driver 缺少安全发布 capability。

插件应展示简短错误，并提供 `Open Logs`。

错误处理边界：

- exit code 为 0 才视为命令成功。
- 非 0 exit code 时，优先解析 JSON error payload；如果没有 JSON，再展示 `stderr` 摘要。
- 插件不得根据 `stderr` 文本内容推断 conflict、auth failure 等业务状态。
- 底层 Driver 失败由 CLI/core 归一化为错误码和状态枚举。

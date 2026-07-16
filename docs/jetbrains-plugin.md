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
- 通过右侧 Tool Window 集中展示状态、Mapping、Repository、操作入口和错误诊断。

不做：

- Repository Driver 和同步算法。
- 复杂 conflict 处理。
- secret 存储。
- 直接修改业务项目 Git history。
- 直接 import 或复刻 Go core 内部模块。

## Kotlin 调用方式

JetBrains 插件通过子进程调用 `local-config` native CLI，而不是在 Kotlin 中复刻 core。

调用边界：

```text
+--------------------+      process + JSON      +-------------------+
| JetBrains Plugin   | -----------------------> | local-config CLI  |
| Kotlin             |                          | Go native binary |
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

- 插件优先调用内置的 `local-config` native executable。
- 插件发行包内置 Linux、macOS、Windows 的 amd64/arm64 六个 binary；不要求用户安装 Node.js。
- CLI 路径允许用户在 Settings 中高级覆盖。
- `sync`、`pull`、`push` 必须在 background task 中执行，不能阻塞 UI 线程。
- `stdout` 在 `--json` 模式下只按 JSON 解析。
- `stderr` 仅作为诊断信息展示或写入日志。
- Kotlin 侧定义命名 DTO，例如 `StatusResponse`、`MappingSummary`，不要长期使用 `Map<String, Any>` 传播状态。

## UI 入口

建议入口：

- Status Bar Widget：显示 `Synced` / `Pending` / `Failed` / `Conflict`，点击后打开 Tool Window。
- Settings Page：提供自定义 CLI 路径的高级 override。
- Project Context Action：右键项目目录，`Setup Local Config Sync`。
- Tool Window：作为插件主界面，展示当前项目、Repository、Mapping、last sync 和真实错误诊断，并提供 Refresh / Setup / Sync / Git Auth。

## Setup Wizard

当前 GitHub file mapping 流程：

1. 检查 CLI 和当前 Git project。
2. 选择 GitHub provider；验证 `gh auth`，未登录时启动 GitHub CLI browser flow。
3. 加载期间展示 modal progress；随后通过 Combobox 选择 Repository：点击选择框后，popup 顶部的搜索框可按 owner 或仓库名过滤 public/private Repository，下方列表用于确认选择。
4. 选择 Repository 内已有文件，或从项目内选择本地文件创建远端路径。
5. 选择/输入项目内的 target file path。
6. 调用 `preview` 判断 `remote_only`、`local_only`、`identical` 或 `conflict`。
7. `conflict` 时用 IntelliJ diff viewer 展示两侧内容，再由用户明确选择 initial version。
8. 以 `kind=file`、`mode=copy` 建立 mapping，写入 `.git/info/exclude` 并执行首次安全 sync。

GitHub 只是 discovery/auth 入口，不在 Kotlin 中实现 Git clone、commit、push 或冲突检测。

所有 Repository/file list wire DTO 都必须容忍旧 CLI 或空后端返回 `null`，入口层按 empty list 处理；当前 CLI 对空列表稳定输出 `[]`。

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

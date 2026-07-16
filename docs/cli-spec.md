# CLI 规格

## 命令总览

```bash
local-config init
local-config provider github auth
local-config provider github repositories
local-config repository add <type>
local-config repository list
local-config repository show <id>
local-config repository files <id>
local-config repository doctor <id>
local-config repository auth [<id>] [--url <url>]
local-config repository remove <id>
local-config link
local-config preview
local-config pull
local-config push
local-config sync
local-config status
local-config doctor
local-config unlink
```

Repository 是一级资源。一个用户可以配置多个 Repository 实例，每个实例由 `git`、`local-folder`、`webdav`、`s3` 等 Driver 处理。

`provider` 只负责 provider-specific 的认证与 Repository discovery。选择完成后仍注册为普通 `git` Repository，后续同步继续复用统一 Git Driver。

## GitHub provider

```bash
local-config provider github auth --json
local-config provider github repositories --json
```

- 认证复用 GitHub CLI 的 `gh auth`，Local Config Sync 不保存 token。
- Repository 列表包含 `nameWithOwner`、public/private、HTTPS/SSH URL 和 default branch。
- private Repository 是否可见取决于当前 `gh auth` 账号及其权限。

## 通用调用协议

所有需要被 IDE plugin、desktop app 或 local agent 调用的命令必须支持 `--json`。

通用约束：

- `stdout` 在 `--json` 模式下只输出机器可读 JSON。
- `stderr` 用于人类可读诊断信息和底层命令错误摘要。
- exit code 为 0 表示成功。
- exit code 非 0 表示失败，调用方不得只靠解析错误文本判断失败原因。
- 公共 JSON 响应必须对应命名模型，避免在跨模块边界长期传播匿名 object bag。

建议 exit code：

```text
0   success
1   generic_error
2   invalid_arguments
3   not_configured
4   conflict
5   auth_failed
6   repository_failed
7   filesystem_failed
8   unsafe_secret_pattern
9   repository_not_found
10  unsupported_capability
11  repository_locked
12  repository_dirty_outside_scope
```

成功响应基础模型：

```json
{
  "ok": true,
  "command": "status",
  "projectPath": "/path/to/business-project"
}
```

失败响应基础模型：

```json
{
  "ok": false,
  "command": "sync",
  "error": {
    "code": "conflict",
    "message": "Remote and local config changed at the same mapped path.",
    "details": {
      "repositoryId": "personal-git",
      "paths": ["config/application-dev.yml"]
    }
  }
}
```

`details` 只用于边界层承载命令相关的补充诊断信息。已知字段应优先沉淀为命名响应模型，例如 `StatusResponse`、`RepositorySummary`、`MappingSummary`、`ConflictSummary`。

## init

初始化用户级目录和默认策略：

```bash
local-config init --default-link-mode symlink
```

行为：

- 创建 `~/.local-config-sync` 下的配置、状态、workspace 和 lock 目录。
- 写入全局默认策略。
- 不绑定唯一配置仓库；仓库通过 `repository add` 单独创建。

## repository

### add

添加并初始化一个 Repository 实例：

```bash
local-config repository add git \
  --id personal-git \
  --name "Personal Git" \
  --url git@github.com:user/private-configs.git \
  --branch main

local-config repository add local-folder \
  --id nas \
  --name "Home NAS" \
  --path /mnt/nas/configs

local-config repository add webdav \
  --id company-webdav \
  --endpoint https://dav.example.com/configs \
  --credential-ref keychain:local-config/company-webdav
```

行为：

- 校验 Repository id 唯一。
- 根据 type 校验命名 options。
- 准备 managed workspace。
- 调用对应 Driver 执行最小连接和 capability 检查。
- 只保存 `credentialRef`，不保存明文凭证。

### list / show

```bash
local-config repository list --json
local-config repository show personal-git --json
local-config repository files personal-git --json
```

返回 Repository id、name、type、workspace、连接状态和公共 capabilities，不返回凭证内容。
`repository files` 返回 managed workspace 内的文件路径；空 Repository 的 `files` 必须编码为 `[]`，不能编码为 `null`。

### doctor

```bash
local-config repository doctor personal-git --json
```

检查 Driver 所需工具、路径、凭证、远端连接、读写权限和安全同步能力。

### auth

验证或配置 Git authentication，不保存 token、password 或 SSH private key：

```bash
local-config repository auth personal-git --method auto
local-config repository auth personal-git --method ssh
local-config repository auth personal-git --method credential
local-config repository auth personal-git --method gh

# 首次注册前按 URL 配置/验证
local-config repository auth \
  --url https://github.com/user/private-configs.git \
  --method gh
```

语义：

- `ssh` 验证 SSH URL，并复用 `ssh-agent` / 用户 SSH key。
- `credential` 验证 HTTP(S) URL，并复用 Git credential helper。
- `gh` 要求用户先在交互终端完成 `gh auth login`，随后执行 `gh auth setup-git` 并验证远端。
- `auto` 对 GitHub HTTPS 优先复用已登录的 `gh`，其他情况直接验证系统 Git 已有认证链。
- CLI 禁用 Git 的隐式交互提示，避免 IDE background task 无限等待。

### remove

```bash
local-config repository remove personal-git
local-config repository remove personal-git --keep-workspace
```

默认约束：

- Repository 仍被 Mapping 引用时拒绝删除。
- 不删除远端内容。
- 删除本地 workspace 前展示影响范围并显式确认。

## link

建立业务项目和 Repository 目录的映射：

```bash
local-config link \
  --project /path/to/business-project \
  --repository personal-git \
  --source-path ai-rvis-agent/config \
  --target config \
  --mode symlink
```

单文件 mapping：

```bash
local-config preview \
  --project . \
  --repository personal-git \
  --source-path ai-rvis-agent/application-dev.yml \
  --target src/main/resources/application-dev.yml \
  --kind file

local-config link \
  --project . \
  --repository personal-git \
  --source-path ai-rvis-agent/application-dev.yml \
  --target src/main/resources/application-dev.yml \
  --kind file \
  --mode copy \
  --initial-strategy remote
```

`preview` 返回 `remote_only`、`local_only`、`identical`、`conflict` 或 `missing_both`。前三种可以用 `auto` 初始化；`conflict` 必须在 diff 后显式传入 `local` 或 `remote`。该选择只建立 initial baseline，不改变后续同步的 conflict-stop 策略。

行为：

- 验证 business project 是 Git repo。
- 验证 Repository 和 workspace 可用。
- 验证 `source-path` 是 workspace 内的安全相对路径，且不与已有 Mapping 重叠。
- 创建 target 目录。
- 对每个 source file 创建 symlink 或 copy。
- 将 target path 写入 `.git/info/exclude`。
- 持久化 Mapping。
- `kind=file` 时只 materialize 所选文件，不要求替换其父目录。
- target file 已被业务 Git 跟踪时拒绝建立 mapping；`.git/info/exclude` 不能让 tracked file 停止跟踪。

## pull

从 Repository 更新本地 workspace：

```bash
local-config pull --project .
local-config pull --repository personal-git
```

行为：

- 获取 Repository-level lock。
- 检查 remote revision 和 last-synced baseline。
- 通过 Driver 安全拉取；本地和远端同时变化时停止。
- 校验 mapped files，必要时重新 link 或更新 copy。
- 更新 Repository state。

## push

将 workspace 改动安全发布到 Repository：

```bash
local-config push --project .
local-config push --repository personal-git
```

行为：

- 获取 Repository-level lock。
- 上传前检查敏感文件 pattern。
- `--project` 只发布当前项目 Mapping scope；scope 外 dirty 时停止。
- Driver 必须以 expected remote revision 执行条件发布。
- 远端 revision 已变化时停止并返回 conflict。

Git Driver 有 staged changes 时使用默认 commit message：

```text
chore(<project-name>): sync local config
```

非 Git Driver 不需要模拟 commit message 或 Git history。

## sync

安全组合命令：

```bash
local-config sync --project .
local-config sync --repository personal-git
```

建议流程：

```text
validate scope
acquire repository lock
inspect
pull
reconcile mapping
sensitive-file scan
conditional push
status
release lock
```

如果检测到 conflict、scope 外 dirty 文件或后端缺少安全发布能力，退出非 0。

## status

展示当前项目及其 Repository 状态：

```bash
local-config status --project . --json
```

输出信息：

- 当前 project path。
- Repository id、name、type、workspace 和 remote revision。
- Repository 公共 capabilities 和连接状态。
- Mapping source path、target path、link mode 和 mapped files。
- exclude 状态、待上传/待下载状态和 last sync time。

JSON 响应模型：

```json
{
  "ok": true,
  "command": "status",
  "projectPath": "/path/to/business-project",
  "state": "synced",
  "repositories": [
    {
      "id": "personal-git",
      "name": "Personal Git",
      "type": "git",
      "state": "synced",
      "workspacePath": "/home/user/.local-config-sync/workspaces/personal-git",
      "remoteRevision": "9d99b62c",
      "capabilities": {
        "history": true,
        "conditionalWrite": true,
        "atomicPublish": true
      }
    }
  ],
  "mappings": [
    {
      "id": "ai-rvis-agent-dev",
      "repositoryId": "personal-git",
      "sourcePath": "ai-rvis-agent/config",
      "targetPath": "config",
      "mode": "symlink",
      "mappedFiles": ["config/application-dev.yml"],
      "excludeConfigured": true
    }
  ],
  "lastSyncTime": "2026-07-14T10:00:00Z"
}
```

状态枚举：

```text
not_configured
synced
pending
syncing
failed
conflict
paused
```

## doctor

项目级综合诊断：

```bash
local-config doctor --project .
```

检查项：

- business project 存在且为 Git repo。
- Mapping 引用的 Repository 和 Driver 已配置。
- Driver 工具、路径、凭证、连接和 capability 正常。
- `.git/info/exclude` 可写。
- symlink 或 copy 能力可用。
- workspace 与 Mapping scope 状态一致。

## unlink

解除映射：

```bash
local-config unlink --project . --keep-files
```

行为：

- 删除 Mapping。
- 可选删除 symlink。
- 可选清理 `.git/info/exclude`。
- 不删除 Repository 中的真实文件。

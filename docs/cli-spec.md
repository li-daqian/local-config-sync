# CLI 规格

## 命令总览

```bash
local-config init
local-config link
local-config pull
local-config push
local-config sync
local-config status
local-config doctor
local-config unlink
```

## 通用调用协议

所有需要被 IDE plugin、desktop app 或 local agent 调用的命令必须支持 `--json`。

通用约束：

- `stdout` 在 `--json` 模式下只输出机器可读 JSON。
- `stderr` 用于人类可读诊断信息、底层 Git 输出摘要和日志提示。
- exit code 为 0 表示成功。
- exit code 非 0 表示失败，调用方不得只靠解析错误文本判断失败原因。
- 公共 JSON 响应必须对应命名模型，避免在跨模块边界长期传播匿名 object bag。

建议 exit code：

```text
0  success
1  generic_error
2  invalid_arguments
3  not_configured
4  conflict
5  auth_failed
6  git_failed
7  filesystem_failed
8  unsafe_secret_pattern
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
      "paths": ["config/application-dev.yml"]
    }
  }
}
```

`details` 只用于边界层承载命令相关的补充诊断信息。已知字段应优先沉淀为命名响应模型，例如 `StatusResponse`、`MappingSummary`、`ConflictSummary`。

## init

初始化用户级配置。

```bash
local-config init --config-repo ~/private-configs
```

行为：

- 检查 `git` 是否存在。
- 检查 `~/private-configs` 是否是 Git repo。
- 写入 `~/.local-config-sync/config.yml`。

## link

建立业务项目和 private repo 目录的映射。

```bash
local-config link \
  --project /path/to/business-project \
  --remote-path ai-rvis-agent/config \
  --target config \
  --mode symlink
```

行为：

- 验证 business project 是 Git repo。
- 验证 private config repo 存在。
- 创建 target 目录。
- 对每个 remote file 创建 symlink 或 copy。
- 将 target path 写入 `.git/info/exclude`。
- 持久化 mapping。

## pull

从 private config repo 拉取远端改动。

```bash
local-config pull --project .
```

行为：

- 对 private config repo 执行 `git pull --rebase --autostash`。
- 校验 mapped files 是否存在。
- 必要时重新 link。

## push

将本地配置改动提交并推送。

```bash
local-config push --project .
```

行为：

- 检查 private repo 中 mapped path 是否有改动。
- `git add` mapped path。
- 有 staged changes 时 commit。
- push。

默认 commit message：

```text
chore(<project-name>): sync local config
```

## sync

安全组合命令。

```bash
local-config sync --project .
```

建议流程：

```text
status
pull
push
status
```

如果检测到 conflict，退出非 0。

## status

展示当前状态。

```bash
local-config status --project . --json
```

输出信息：

- 当前 project path。
- private repo path。
- remote path。
- target path。
- link mode。
- mapped files。
- exclude 状态。
- private repo dirty 状态。
- last sync time。

JSON 响应模型：

```json
{
  "ok": true,
  "command": "status",
  "projectPath": "/path/to/business-project",
  "state": "synced",
  "privateRepoPath": "/home/user/private-configs",
  "mappings": [
    {
      "remotePath": "ai-rvis-agent/config",
      "targetPath": "config",
      "mode": "symlink",
      "mappedFiles": ["config/application-dev.yml"],
      "excludeConfigured": true
    }
  ],
  "privateRepoDirty": false,
  "lastSyncTime": "2026-07-02T10:00:00Z"
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

诊断环境。

检查项：

- `git` 可用。
- `gh` 可用或 GitHub remote 可 push。
- private repo 存在且为 Git repo。
- business project 存在且为 Git repo。
- `.git/info/exclude` 可写。
- symlink 能力可用。

## unlink

解除映射。

```bash
local-config unlink --project . --keep-files
```

行为：

- 删除 mapping。
- 可选删除 symlink。
- 可选清理 `.git/info/exclude`。
- 不删除 private repo 中的真实文件。

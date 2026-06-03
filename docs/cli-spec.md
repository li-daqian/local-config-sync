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
local-config status --project .
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


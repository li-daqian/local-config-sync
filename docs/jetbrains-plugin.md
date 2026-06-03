# JetBrains 插件设计

## 插件定位

JetBrains 插件只是入口层，不承载核心同步逻辑。

职责：

- 识别当前打开项目路径。
- 提供 setup wizard。
- 调用 `local-config` CLI。
- 展示状态和错误。
- 提供 `Sync Now` 操作。

不做：

- Git 同步算法。
- 复杂 conflict 处理。
- secret 存储。
- 直接修改业务项目 Git history。

## UI 入口

建议入口：

- Status Bar Widget：显示 `Synced` / `Pending` / `Failed` / `Conflict`。
- Settings Page：配置 CLI 路径、private repo、sync 策略。
- Project Context Action：右键项目目录，`Setup Local Config Sync`。
- Tool Window：MVP 可不做，避免重 UI。

## Setup Wizard

步骤：

1. 检查 CLI 是否安装。
2. 检查当前项目是否是 Git repo。
3. 选择 private config repo。
4. 选择 remote path。
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
- private repo 不存在。
- GitHub auth 失败。
- `.git/info/exclude` 不可写。
- symlink 失败。
- private repo 有冲突。
- push 被拒绝。

插件应展示简短错误，并提供 `Open Logs`。


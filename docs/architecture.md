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
│  Mapping Manager                                             │
│  File Linker                                                 │
│  Ignore Manager                                              │
│  Sync Orchestrator                                           │
│  Conflict Detector                                           │
│  Git Adapter                                                 │
└───────────────┬─────────────────────────────────────────────┘
                │
                v
┌─────────────────────────────────────────────────────────────┐
│ Local + Remote State                                         │
│                                                             │
│  Business Project                                            │
│  .git/info/exclude                                           │
│  Private Config Repo                                         │
│  GitHub private repo                                         │
└─────────────────────────────────────────────────────────────┘
```

## 关键模块

### Entry Layer

职责：

- 识别当前项目路径。
- 提供 Setup / Sync / Status 操作入口。
- 展示同步状态和错误。
- 调用 CLI/core。

不负责：

- Git 同步算法。
- 文件冲突处理策略。
- 配置映射持久化格式。

### Project Resolver

职责：

- 确认当前目录是否是 Git 项目。
- 找到业务项目根目录。
- 找到 `.git/info/exclude`。
- 识别已有映射。

### Mapping Manager

职责：

- 保存用户配置仓库路径。
- 保存业务项目和 remote path 的映射。
- 支持多个业务项目。
- 支持同一个 private repo 下多个项目目录。

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

### Git Adapter

职责：

- 对 private config repo 执行 `git status`、`pull --rebase`、`add`、`commit`、`push`。
- 第一版使用系统 git CLI。
- 不对业务项目执行提交操作。

### Sync Orchestrator

职责：

- 编排 pull/push/sync 流程。
- 检查工作区状态。
- 执行 debounce 后的自动同步。
- 在 conflict 风险时停止。

## 同步模式

### Symlink 模式

```text
business-project/config/application-dev.yml
        -> private-configs/ai-rvis-agent/config/application-dev.yml
```

优点：

- 修改业务项目文件即修改 private repo 文件。
- 不需要额外 copy back。
- 状态简单。

缺点：

- 路径依赖更强。
- Windows 下 symlink 体验可能较差。

### Copy 模式

```text
private repo file -> business project file
business project file -> private repo file
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
  state.json
  logs/
```

配置仓库建议目录：

```text
~/private-configs/
  ai-rvis-agent/
    config/
      application-dev.yml
```

## Git 策略

`sync` 建议流程：

```text
validate mapping
check private repo status
git pull --rebase --autostash
copy/link validation
git add mapped paths
if staged changes exist:
  git commit -m "chore(<project>): sync local config"
  git push
else:
  no-op
```

禁止：

- 自动 `git reset --hard`。
- 自动 force push。
- 自动覆盖冲突文件。


# Local Config Sync

Local Config Sync 是一个面向开发者的本地配置同步工具。它解决的问题是：

> 我希望把某个项目的本地配置文件放在项目目录中使用，方便维护和 IDE 自动识别；但这些文件不能提交到业务仓库。同时，我换电脑后还希望这些本地配置可以自动恢复。

核心方案：

```text
业务项目/config/application-dev.yml
        |
        | copy or symlink
        v
~/private-configs/<project>/config/application-dev.yml
        |
        | git commit / push
        v
GitHub private repo
```

## 产品定位

本项目不是一个只服务 JetBrains 的插件，而是：

```text
core CLI + thin IDE plugin
```

CLI/core 负责同步、Git、映射、ignore、冲突检测。IDE 插件只负责点选入口和状态展示。

这样以后切换 IDE 时，只需要替换入口层：

```text
JetBrains Plugin -> VS Code Extension -> Cursor Extension -> CLI
```

底层同步能力不重写。

## 典型使用流程

1. 用户在 IDE 中打开业务项目。
2. 插件检测当前项目路径。
3. 用户点击 `Local Config Sync: Setup`。
4. 选择 GitHub private repo。
5. 选择 private repo 内目录，例如 `ai-rvis-agent/config/`。
6. 选择当前项目目标目录，例如 `config/`。
7. 工具拉取配置文件到业务项目。
8. 工具将这些路径写入业务项目 `.git/info/exclude`。
9. 用户修改业务项目内配置文件。
10. 工具手动或自动同步到 GitHub private repo。

## MVP 命令草案

```bash
local-config init
local-config link --project . --remote-path ai-rvis-agent/config --target config --mode symlink
local-config pull
local-config push
local-config sync
local-config status
local-config doctor
```

## 安全默认值

- 默认不修改业务项目 `.gitignore`。
- 默认写业务项目 `.git/info/exclude`。
- 默认不自动 push，每次同步先做 status 检查。
- 默认不同步 `.env`、private key、token 文件，除非用户显式允许。
- 冲突时停止自动同步，不自动覆盖。

## 文档索引

- [产品需求](docs/product-requirements.md)
- [架构设计](docs/architecture.md)
- [实现计划](docs/implementation-plan.md)
- [CLI 规格](docs/cli-spec.md)
- [安全模型](docs/security-model.md)
- [JetBrains 插件设计](docs/jetbrains-plugin.md)
- [数据模型](docs/data-model.md)
- [Backlog](docs/backlog.md)

# AGENTS.md

本项目用于实现一个跨 IDE 的本地配置同步工具。核心目标：

- 让开发者把项目内的本地 overlay 配置放在业务项目目录中使用。
- 这些本地配置不会提交到业务项目 Git。
- 本地配置可自动同步到用户自己的 GitHub private repo。
- IDE 插件只是入口层，核心同步逻辑必须放在 CLI/core 中，便于未来切换 JetBrains / VS Code / Cursor / CLI。

默认回答和文档使用中文，技术关键词保留英文。

## 当前产品方向

产品形态采用：

```text
IDE Plugin / CLI / Tray App
        |
        v
local-config-core
        |
        v
GitHub private repo / local project / .git/info/exclude
```

核心设计原则：

- 不污染业务仓库：默认写 `.git/info/exclude`，不要默认改 `.gitignore`。
- 不把同步逻辑写死在 IDE 插件中：插件只做 UI 和当前项目上下文识别。
- 不自动处理高风险冲突：检测到 conflict 时停止自动同步并提示用户。
- 不默认同步真实密钥：private repo 只能作为非敏感配置同步方案；密钥要走加密或 secret provider。

## MVP 范围

第一阶段只做：

- CLI 初始化配置仓库映射。
- 从 GitHub private repo 拉取指定目录。
- 将文件复制或软链接到当前业务项目目录。
- 自动维护业务项目 `.git/info/exclude`。
- 手动 `sync`：pull remote config、提交本地改动、push 回 private repo。
- `status`：展示是否已链接、是否有未同步改动、是否有冲突风险。

暂不做：

- 自动后台同步 daemon。
- 多 IDE 插件。
- 加密密钥托管。
- Web UI。

## 技术边界

- CLI/core 必须能独立运行，不依赖 JetBrains SDK。
- JetBrains 插件只能调用 core/CLI，不直接实现 Git 同步算法。
- Git 操作优先使用本机 `git` CLI，便于调试与复用用户已有凭证。
- GitHub auth 第一版优先复用 `gh auth` 或本机 git credential，不强制自建 OAuth flow。
- 所有 destructive Git 操作必须显式确认，禁止自动 `reset --hard`。

## 后续 Codex 工作入口

开始实现前优先阅读：

- `README.md`
- `docs/product-requirements.md`
- `docs/architecture.md`
- `docs/implementation-plan.md`
- `docs/security-model.md`
- `docs/cli-spec.md`


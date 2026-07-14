# AGENTS.md

本项目用于实现一个跨 IDE 的本地配置同步工具。核心目标：

- 让开发者把项目内的本地 overlay 配置放在业务项目目录中使用。
- 这些本地配置不会提交到业务项目 Git。
- 本地配置可同步到用户配置的 Git、local folder、WebDAV 等 Repository。
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
Repository Driver / managed workspace / local project / .git/info/exclude
```

核心设计原则：

- 不污染业务仓库：默认写 `.git/info/exclude`，不要默认改 `.gitignore`。
- 不把同步逻辑写死在 IDE 插件中：插件只做 UI 和当前项目上下文识别。
- 不自动处理高风险冲突：检测到 conflict 时停止自动同步并提示用户。
- 不默认同步真实密钥：Repository 只能作为非敏感配置同步方案；密钥要走加密或 secret provider。

## MVP 范围

第一阶段只做：

- CLI 初始化配置仓库映射。
- 配置多个 Repository 实例，并通过 Git 或 local folder Driver 拉取指定目录。
- 将文件复制或软链接到当前业务项目目录。
- 自动维护业务项目 `.git/info/exclude`。
- 手动 `sync`：pull remote config、协调本地改动并安全 push 回 Repository。
- `status`：展示是否已链接、是否有未同步改动、是否有冲突风险。

暂不做：

- 自动后台同步 daemon。
- 多 IDE 插件。
- 加密密钥托管。
- Web UI。

## 技术边界

- CLI/core 必须能独立运行，不依赖 JetBrains SDK。
- JetBrains 插件只能调用 core/CLI，不直接实现 Repository Driver 或同步算法。
- Git Driver 优先使用本机 `git` CLI，便于调试与复用用户已有凭证。
- Git auth 第一版优先复用 `gh auth`、本机 git credential 或 SSH key，不强制自建 OAuth flow。
- Repository Driver 的公共契约不能暴露 Git 专属语义；所有后端先 materialize 到 managed local workspace。
- 所有 destructive Git 操作必须显式确认，禁止自动 `reset --hard`。

## 后续 Codex 工作入口

开始实现前优先阅读：

- `README.md`
- `docs/product-requirements.md`
- `docs/architecture.md`
- `docs/implementation-plan.md`
- `docs/security-model.md`
- `docs/cli-spec.md`
- `docs/repository-backends.md`

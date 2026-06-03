# Backlog

## P0: CLI/Core

- 选择技术栈并初始化项目脚手架。
- 实现 `doctor`，先验证 git、private repo、business project、symlink 能力。
- 实现 `init`，写入用户级配置。
- 实现 `link`，支持 symlink 模式。
- 实现 `.git/info/exclude` 写入和去重。
- 实现 `status --json`。
- 实现 `pull`、`push`、`sync`。
- 加入最小测试覆盖：mapping、ignore、git adapter dry run。

## P1: 可用性

- 支持 copy 模式。
- 支持自动生成默认 mapping id。
- 支持 `unlink`。
- 支持 `--dry-run`。
- 支持 structured logs。
- 支持自动 commit message 模板。

## P2: 自动同步

- 文件监听。
- debounce。
- 失败后 paused 状态。
- 手动 resume。
- conflict 检测和提示。

## P3: JetBrains 插件

- 插件项目脚手架。
- Settings 页面配置 CLI 路径。
- Project action：Setup Local Config Sync。
- Status bar widget。
- 调用 `local-config status --json`。
- 调用 `local-config sync`。

## P4: 安全增强

- 敏感文件 pattern 检测。
- 上传前 risk summary。
- 可选加密方案调研。
- 1Password / Bitwarden 集成调研。

## 当前建议的第一步

先不要写插件。先实现 CLI MVP：

```bash
local-config doctor
local-config init
local-config link
local-config status --json
local-config sync
```

当 CLI 可以稳定完成一个业务项目的配置同步后，再做 JetBrains 插件外壳。


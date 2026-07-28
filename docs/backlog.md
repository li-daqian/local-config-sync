# Backlog

## P0: CLI/Core

- 选择技术栈并初始化项目脚手架。
- 实现 Repository Registry、Repository Driver contract 和命名配置模型。
- 实现 `repository add/list/show/doctor/remove`。
- 实现 `doctor`，验证 Driver、Repository、business project、ignore 和 symlink 能力。
- 实现 `init`，写入用户级配置。
- 实现 `link`，支持 symlink 模式。
- 实现 `.git/info/exclude` 写入和去重。
- 实现 `status --json`。
- 实现 `pull`、`push`、`sync`。
- 实现 Git Driver，支持所有标准 Git remote。
- 实现 Local Folder Driver，验证 core 不依赖 Git 语义。
- 加入最小测试覆盖：repository registry、mapping scope、ignore、lock、Git Driver dry run、Local Folder Driver。

## P1: 可用性

- 支持 copy 模式。
- 支持自动生成默认 mapping id。
- 支持 `unlink`。
- 支持 `--dry-run`。
- 支持 structured logs。
- 支持自动 commit message 模板。
- 禁止重叠 `sourcePath`，检测 scope 外 dirty 文件。
- 持久化 last-synced manifest 和 remote revision。

## P2: 远端 Driver

- 实现 WebDAV Driver 的 ETag/conditional request。
- 实现 S3 Driver 的 version/conditional write。
- 实现 snapshot manifest / commit marker。
- 覆盖部分上传恢复、并发修改和凭证脱敏测试。

## P3: 自动同步

- 文件监听。
- debounce。
- 失败后 paused 状态。
- 手动 resume。
- conflict 检测和提示。

## P4: JetBrains 插件

- 插件项目脚手架。
- Settings 页面配置 CLI 路径。
- Project action：Setup Local Config Sync。
- Status bar widget。
- 调用 `local-config status --json`。
- 调用 `local-config sync`。

## P5: 安全增强

- 敏感文件 pattern 检测。
- 上传前 risk summary。
- 可选加密方案调研。
- 1Password / Bitwarden 集成调研。

## P6: VS Code 插件

- [x] 插件工程、TypeScript bundle 和 platform-specific VSIX。
- [x] bundled native CLI 和 `contractVersion` compatibility handshake。
- [x] multi-root Tree View 和 file-level status。
- [x] GitHub Setup、Git authentication、Sync、diff 和 conflict resolve。
- [x] sensitive-file 二次确认和 Workspace Trust 边界。
- [x] Windows、macOS、Linux、Alpine 的发布构建矩阵。
- [x] Visual Studio Marketplace release workflow。
- [ ] 使用 publisher `li-daqian` 和已配置的 `VSCE_PAT` 完成首次发布。
- [ ] WSL、Remote SSH、Dev Container 的发布前 runtime matrix。
- [ ] stale Repository lock 诊断和显式恢复。

## 当前建议的下一步

完成 Remote/WSL 和 conflict/sensitive-file runtime matrix，并将 Marketplace PAT 认证迁移到
Microsoft Entra workload identity。

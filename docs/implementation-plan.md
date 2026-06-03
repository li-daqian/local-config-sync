# 实现计划

## Phase 0: 项目脚手架

目标：

- 建立 CLI/core 项目。
- 明确语言、包结构、测试入口。

候选技术：

- TypeScript + Node.js：适合写 CLI，也适合被 JetBrains 插件通过进程调用。
- Rust：二进制部署体验好，但插件集成和快速迭代成本更高。
- Kotlin：和 JetBrains 插件生态一致，但跨 IDE 复用不如 CLI 简洁。

建议 MVP 使用 TypeScript + Node.js。

原因：

- GitHub auth、文件监听、CLI 生态成熟。
- 后续 VS Code extension 复用方便。
- JetBrains 插件可直接调用打包后的 CLI。

## Phase 1: CLI MVP

核心文件建议：

```text
src/
  cli.ts
  core/
    project-resolver.ts
    mapping-manager.ts
    file-linker.ts
    ignore-manager.ts
    git-adapter.ts
    sync-orchestrator.ts
    conflict-detector.ts
  model/
    config.ts
    mapping.ts
  util/
    fs.ts
    process.ts
```

命令：

```bash
local-config init
local-config link
local-config pull
local-config push
local-config sync
local-config status
local-config doctor
```

验收：

- 能在任意 Git 项目中建立映射。
- 能把配置文件 link 到业务项目。
- 能写 `.git/info/exclude`。
- 能 push private repo 改动。

## Phase 2: 自动同步

能力：

- 文件监听 mapped paths。
- debounce 30s/60s。
- 自动执行安全 `sync`。
- 失败后进入 paused 状态。

验收：

- 连续保存不会产生大量 commit。
- 网络失败不会丢文件。
- 冲突时不自动覆盖。

## Phase 3: JetBrains Plugin MVP

插件职责：

- 展示 Tool Window 或 Settings 页。
- 识别当前 project path。
- 调用 CLI。
- 提供 Setup、Sync Now、Status。
- 状态栏显示同步状态。

插件不实现：

- Git 同步细节。
- 文件 merge。
- 配置格式解析。

## Phase 4: 安全增强

能力：

- 敏感文件 pattern 警告。
- 可选本地加密。
- secret provider integration。
- sync audit log。

## Phase 5: 多入口

候选入口：

- VS Code extension。
- Cursor extension。
- Desktop tray app。
- Web UI。

前提：

- CLI/core API 稳定。
- 状态文件格式稳定。


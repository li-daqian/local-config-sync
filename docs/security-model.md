# 安全模型

## 核心判断

Git、local folder、WebDAV 和 S3-compatible storage 可以同步非敏感本地配置，但不应默认作为 secret manager。

原因：

- Git 仓库存在长期历史，误提交后很难彻底清理。
- 云端对象、备份版本和同步盘回收站也可能长期保留已删除内容。
- token、password、private key 一旦上传到任何远端，需要按泄露处理并轮换。
- 企业合规环境可能禁止密钥进入个人云端或非公司管理的存储。

## 默认安全策略

- 默认拒绝同步明显敏感文件。
- 默认不扫描和上传 `.env`。
- 默认不上传 private key。
- 默认不上传云厂商 credential 文件。
- 默认不修改业务项目 tracked files。
- 默认写 `.git/info/exclude`。

## 敏感文件 pattern

第一版建议警告或阻止：

```text
.env
.env.*
*.pem
*.key
*.p12
*.pfx
id_rsa
id_ed25519
credentials
credentials.json
application-prod.yml
application-production.yml
```

允许用户显式 override，但需要二次确认。

## Repository 凭证

Git Driver 优先使用用户本机已有凭证：

- `gh auth`
- git credential helper
- SSH key

WebDAV、S3 等 Driver 通过 `credentialRef` 使用 OS keychain、环境变量或 provider SDK 默认凭证链。

Local Config Sync 配置文件不得保存 Git token、WebDAV password、S3 access key 或 secret key。`status --json`、日志和错误响应不得输出凭证内容。

## Repository 发布保护

- Driver push 必须使用 expected remote revision 或等价条件写。
- 远端 revision 已变化时返回 conflict。
- 后端不能提供条件写或等价保护时，默认拒绝可能覆盖远端的 push。
- 同一 Repository 使用独占 lock，避免本机并发同步。
- `sync --project` 不能发布当前 Mapping scope 以外的 dirty 文件。

## 业务项目保护

工具只能写：

- 用户选择的 target path。
- 业务项目 `.git/info/exclude`。
- 用户级状态目录 `~/.local-config-sync`。

默认不能写：

- 业务项目 `.gitignore`。
- 业务项目 tracked source files。
- 业务项目 Git history。

## 冲突策略

冲突时必须停止自动同步。

不允许：

- 自动覆盖远端。
- 自动覆盖本地。
- 自动 force push。
- 自动 reset。
- 缺少并发保护时执行覆盖式上传。

允许：

- 展示冲突文件。
- 打开 Repository workspace。
- 给出 Driver-specific 手工解决建议。

首次建立单文件 mapping 是唯一的显式初始化例外：如果本地和 Repository 文件同时存在且内容不同，UI 必须先展示 diff，并要求用户明确选择 `local` 或 `remote` 作为 initial baseline。没有显式选择时 core 返回 `conflict`；mapping 建立后的同步不得复用该覆盖选择。

单文件 target 已被业务 Git 跟踪时拒绝 Setup。用户必须先独立处理业务仓库中的 tracked 状态，工具不能通过 `.git/info/exclude` 伪装成已经停止跟踪。

## 后续加密方案

可选增强：

- 使用 age/sops 对 Repository 内容加密。
- 对单个 mapping 开启加密。
- 通过 1Password / Bitwarden / company secret manager 注入敏感值。

MVP 不实现加密，但架构不能阻止后续加入。Secret provider 是独立扩展点，不作为 Repository Driver 的一种类型。

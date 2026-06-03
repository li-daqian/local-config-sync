# 安全模型

## 核心判断

GitHub private repo 可以同步非敏感本地配置，但不应默认作为 secret manager。

原因：

- private repo 仍然是 Git 历史，误提交后很难彻底清理。
- token、password、private key 一旦进入 Git 历史，需要轮换。
- 企业合规环境可能禁止密钥进入个人 GitHub。

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

## GitHub 权限

第一版优先使用用户本机已有凭证：

- `gh auth`
- git credential helper
- SSH key

不在本工具内保存 GitHub token。

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

允许：

- 展示冲突文件。
- 打开 private repo 工作区。
- 给出手工解决命令。

## 后续加密方案

可选增强：

- 使用 age/sops 对 private repo 内容加密。
- 对单个 mapping 开启加密。
- 通过 1Password / Bitwarden / company secret manager 注入敏感值。

MVP 不实现加密，但架构不能阻止后续加入。


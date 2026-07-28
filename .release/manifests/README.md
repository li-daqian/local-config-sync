# Release manifests

每次发布在此目录新增一份与 tag 同名的 YAML，例如：

```text
release-2026.07.22.1.yaml
```

Release manifest 在 tag 创建后视为不可变；发布失败时重新运行 failed jobs，不修改 manifest 或移动 tag。

当前支持的 artifact：

- `jetbrains`
- `vscode`

`vscode.channel` 支持 `default` 和 `pre-release`。只有 manifest 中声明的平台会进入对应的
Marketplace 发布 workflow。

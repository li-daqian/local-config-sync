# 数据模型

## 用户级配置

路径：

```text
~/.local-config-sync/config.yml
```

示例：

```yaml
version: 1
privateRepoPath: /home/user/private-configs
defaultLinkMode: symlink
autoSync:
  enabled: false
  debounceSeconds: 60
```

## 映射状态

路径：

```text
~/.local-config-sync/mappings.yml
```

示例：

```yaml
version: 1
mappings:
  - id: ai-rvis-agent-dev
    projectPath: /home/user/IdeaProjects/ai-rvis-agent
    projectName: ai-rvis-agent
    privateRepoPath: /home/user/private-configs
    remotePath: ai-rvis-agent/config
    targetPath: config
    mode: symlink
    files:
      - application-dev.yml
    createdAt: "2026-06-03T00:00:00Z"
    updatedAt: "2026-06-03T00:00:00Z"
```

## 状态输出 JSON

`local-config status --json` 示例：

```json
{
  "projectPath": "/home/user/IdeaProjects/ai-rvis-agent",
  "configured": true,
  "state": "synced",
  "mapping": {
    "projectName": "ai-rvis-agent",
    "remotePath": "ai-rvis-agent/config",
    "targetPath": "config",
    "mode": "symlink"
  },
  "files": [
    {
      "path": "config/application-dev.yml",
      "ignored": true,
      "linked": true,
      "exists": true,
      "dirty": false
    }
  ],
  "privateRepo": {
    "path": "/home/user/private-configs",
    "dirty": false,
    "ahead": 0,
    "behind": 0
  }
}
```

## 业务项目 ignore

写入目标：

```text
<business-project>/.git/info/exclude
```

示例：

```gitignore
# local-config-sync
config/application-dev.yml
```

不建议默认写 `.gitignore`，因为 `.gitignore` 是团队共享文件。


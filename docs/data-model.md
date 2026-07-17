# 数据模型

## 用户级配置

路径：

```text
~/.local-config-sync/config.yml
```

示例：

```yaml
version: 1
defaultRepositoryId: personal-git
defaultLinkMode: symlink
autoSync:
  enabled: false
  debounceSeconds: 60
```

全局配置只保存默认策略，不再保存唯一的 `privateRepoPath`。

## Repository Registry

路径：

```text
~/.local-config-sync/repositories.yml
```

示例：

```yaml
version: 1
repositories:
  - id: personal-git
    name: Personal Git
    type: git
    workspacePath: /home/user/.local-config-sync/workspaces/personal-git
    options:
      remoteUrl: git@github.com:user/private-configs.git
      branch: main

  - id: company-webdav
    name: Company WebDAV
    type: webdav
    workspacePath: /home/user/.local-config-sync/workspaces/company-webdav
    credentialRef: keychain:local-config/company-webdav
    options:
      endpoint: https://dav.example.com/configs
```

`id` 在用户配置内唯一且持久稳定。Repository-specific `options` 读取后必须按 `type` 校验为命名类型。

`credentialRef` 只引用 OS keychain、环境或 provider 默认凭证链，不得包含明文 token、password 或 access key。

## 映射配置

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
    repositoryId: personal-git
    sourcePath: ai-rvis-agent/config
    targetPath: config
    mode: symlink
    kind: directory
    files:
      - application-dev.yml
    createdAt: "2026-07-14T00:00:00Z"
    updatedAt: "2026-07-14T00:00:00Z"
```

约束：

- `sourcePath` 必须是 Repository workspace 内的相对路径，不能逃逸 workspace。
- 同一 Repository 中不同 Mapping 的 `sourcePath` 默认不能重叠。
- Mapping 只引用 `repositoryId`，不复制 Repository 路径、URL 或凭证。
- `kind` 为 `file` 或 `directory`；历史记录缺少该字段时按 `directory` 读取。

## Repository 运行状态

路径：

```text
~/.local-config-sync/state/repositories/<repository-id>.json
```

示例：

```json
{
  "version": 1,
  "repositoryId": "personal-git",
  "remoteRevision": "9d99b62c",
  "lastSyncTime": "2026-07-14T10:00:00Z",
  "files": {
    "ai-rvis-agent/config/application-dev.yml": {
      "sha256": "...",
      "size": 312,
      "deleted": false
    }
  }
}
```

`remoteRevision` 由 Driver 产生，可以对应 Git commit SHA、WebDAV ETag、S3 version ID 或 Local Folder snapshot revision。

## 状态输出 JSON

`local-config status --json` 示例：

```json
{
  "ok": true,
  "command": "status",
  "projectPath": "/home/user/IdeaProjects/ai-rvis-agent",
  "state": "synced",
  "repositories": [
    {
      "id": "personal-git",
      "name": "Personal Git",
      "type": "git",
      "state": "synced",
      "workspacePath": "/home/user/.local-config-sync/workspaces/personal-git",
      "remoteRevision": "9d99b62c",
      "capabilities": {
        "history": true,
        "conditionalWrite": true,
        "atomicPublish": true
      }
    }
  ],
  "mappings": [
    {
      "id": "ai-rvis-agent-dev",
      "repositoryId": "personal-git",
      "sourcePath": "ai-rvis-agent/config",
      "targetPath": "config",
      "mode": "symlink",
      "mappedFiles": ["config/application-dev.yml"],
      "excludeConfigured": true
    }
  ],
  "files": [
    {
      "mappingId": "ai-rvis-agent-dev",
      "repositoryId": "personal-git",
      "localPath": "config/application-dev.yml",
      "remotePath": "ai-rvis-agent/config/application-dev.yml",
      "status": "synced",
      "localExists": true,
      "remoteExists": true
    }
  ],
  "lastSyncTime": "2026-07-14T10:00:00Z"
}
```

公共状态只暴露所有 Driver 都能保证的字段。Git ahead/behind、S3 versioning 等专属信息应使用按 Repository type 区分的命名状态模型，不能长期传播匿名 `details`。

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

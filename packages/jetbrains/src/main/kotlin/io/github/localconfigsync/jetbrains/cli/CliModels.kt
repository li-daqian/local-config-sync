package io.github.localconfigsync.jetbrains.cli

data class ErrorPayload(val code: String = "generic_error", val message: String = "Unknown error")
data class ErrorResponse(val ok: Boolean = false, val command: String = "unknown", val error: ErrorPayload = ErrorPayload())

data class Capabilities(
    val history: Boolean = false,
    val conditionalWrite: Boolean = false,
    val atomicPublish: Boolean = false,
)

data class RepositorySummary(
    val id: String = "",
    val name: String = "",
    val type: String = "",
    val state: String = "not_configured",
    val workspacePath: String = "",
    val remoteRevision: String? = null,
    val capabilities: Capabilities = Capabilities(),
)

data class MappingSummary(
    val id: String = "",
    val repositoryId: String = "",
    val sourcePath: String = "",
    val targetPath: String = "",
    val mode: String = "symlink",
    val kind: String = "directory",
    val mappedFiles: List<String> = emptyList(),
    val excludeConfigured: Boolean = false,
)

data class StatusResponse(
    val ok: Boolean = false,
    val command: String = "status",
    val projectPath: String = "",
    val state: String = "not_configured",
    val repositories: List<RepositorySummary> = emptyList(),
    val mappings: List<MappingSummary> = emptyList(),
    val lastSyncTime: String? = null,
)

data class RepositoryOptions(
    val remoteUrl: String? = null,
    val branch: String? = null,
    val path: String? = null,
)

data class ConfiguredRepository(
    val id: String = "",
    val name: String = "",
    val type: String = "",
    val options: RepositoryOptions = RepositoryOptions(),
)

data class RepositoryListResponse(
    val ok: Boolean = false,
    val command: String = "repository.list",
    val repositories: List<ConfiguredRepository>? = emptyList(),
)

data class GitHubRepository(
    val nameWithOwner: String = "",
    val private: Boolean = false,
    val sshUrl: String = "",
    val url: String = "",
    val defaultBranch: String = "main",
)

data class GitHubRepositoriesResponse(
    val ok: Boolean = false,
    val command: String = "provider.github.repositories",
    val repositories: List<GitHubRepository>? = emptyList(),
)

data class RepositoryFilesResponse(
    val ok: Boolean = false,
    val command: String = "repository.files",
    val repositoryId: String = "",
    val files: List<String>? = emptyList(),
)

data class MappingPreviewResponse(
    val ok: Boolean = false,
    val command: String = "preview",
    val state: String = "missing_both",
    val kind: String = "file",
    val sourcePath: String = "",
    val targetPath: String = "",
    val sourceAbsolutePath: String = "",
    val targetAbsolutePath: String = "",
    val sourceExists: Boolean = false,
    val targetExists: Boolean = false,
)

data class CommandResponse(val ok: Boolean = false, val command: String = "unknown")

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

data class CommandResponse(val ok: Boolean = false, val command: String = "unknown")

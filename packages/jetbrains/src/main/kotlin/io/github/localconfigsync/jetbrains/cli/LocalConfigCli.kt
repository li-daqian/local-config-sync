package io.github.localconfigsync.jetbrains.cli

import com.google.gson.Gson
import com.intellij.execution.configurations.GeneralCommandLine
import com.intellij.execution.process.CapturingProcessHandler
import com.intellij.openapi.project.Project
import java.nio.charset.StandardCharsets

class CliException(val code: String, override val message: String, val diagnostics: String = "") : RuntimeException(message)

object LocalConfigCli {
    private val gson = Gson()

    fun <T> execute(project: Project?, args: List<String>, responseType: Class<T>): T {
        val projectPath = project?.basePath
        val invocation = CliCommandResolver.resolve()
        val commandLine = GeneralCommandLine()
            .withExePath(invocation.executable)
            .withParameters(args + "--json")
            .withCharset(StandardCharsets.UTF_8)
        if (projectPath != null) commandLine.withWorkDirectory(projectPath)

        val output = try {
            CapturingProcessHandler(commandLine).runProcess(120_000)
        } catch (error: Exception) {
            throw CliException("cli_unavailable", "Cannot start Local Config Sync CLI. Configure its path in Settings.", error.message.orEmpty())
        }
        val json = output.stdout.trim()
        if (output.isTimeout) throw CliException("timeout", "Local Config Sync command timed out", output.stderr)
        if (output.exitCode != 0) {
            val failure = runCatching { gson.fromJson(json, ErrorResponse::class.java) }.getOrNull()
            throw CliException(failure?.error?.code ?: "cli_failed", failure?.error?.message ?: "Local Config Sync command failed", output.stderr)
        }
        return try {
            gson.fromJson(json, responseType)
        } catch (error: Exception) {
            throw CliException("invalid_cli_response", "CLI returned invalid JSON", output.stderr)
        }
    }

    fun status(project: Project): StatusResponse = execute(project, listOf("status", "--project", project.basePath.orEmpty()), StatusResponse::class.java)
    fun command(project: Project?, args: List<String>): CommandResponse = execute(project, args, CommandResponse::class.java)
}

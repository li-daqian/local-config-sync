package io.github.localconfigsync.jetbrains.cli

import com.google.gson.Gson
import com.intellij.execution.configurations.GeneralCommandLine
import com.intellij.execution.process.CapturingProcessHandler
import com.intellij.execution.process.OSProcessHandler
import com.intellij.execution.process.ProcessEvent
import com.intellij.execution.process.ProcessListener
import com.intellij.openapi.project.Project
import com.intellij.openapi.util.Key
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

    fun startGithubAuthentication(
        project: Project,
        onOutput: (String) -> Unit,
        onFinished: (Int) -> Unit,
    ): GithubAuthSession {
        val commandLine = GeneralCommandLine()
            .withExePath("gh")
            .withParameters("auth", "login", "--hostname", "github.com", "--git-protocol", "https", "--web")
            .withCharset(StandardCharsets.UTF_8)
            .withWorkDirectory(project.basePath.orEmpty())
        val handler = try {
            OSProcessHandler(commandLine)
        } catch (error: Exception) {
            throw CliException("github_cli_unavailable", "GitHub CLI is required for GitHub authentication.", error.message.orEmpty())
        }
        handler.addProcessListener(object : ProcessListener {
            override fun onTextAvailable(event: ProcessEvent, outputType: Key<*>) = onOutput(event.text)
            override fun processTerminated(event: ProcessEvent) = onFinished(event.exitCode)
        })
        handler.startNotify()
        // gh may ask for one confirmation before opening its browser-based device flow.
        runCatching {
            handler.processInput.write("\n".toByteArray(StandardCharsets.UTF_8))
            handler.processInput.flush()
        }
        return GithubAuthSession(handler)
    }
}

class GithubAuthSession internal constructor(private val handler: OSProcessHandler) {
    fun cancel() {
        if (!handler.isProcessTerminated) handler.destroyProcess()
    }

    fun waitFor(timeoutMillis: Long): Boolean = handler.waitFor(timeoutMillis)
    val exitCode: Int? get() = handler.exitCode
}

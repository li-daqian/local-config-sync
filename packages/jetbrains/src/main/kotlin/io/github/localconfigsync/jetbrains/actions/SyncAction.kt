package io.github.localconfigsync.jetbrains.actions

import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.project.Project
import io.github.localconfigsync.jetbrains.cli.CliException
import io.github.localconfigsync.jetbrains.cli.LocalConfigCli

class SyncAction : AnAction() {
    override fun update(event: AnActionEvent) { event.presentation.isEnabled = event.project?.basePath != null }
    override fun actionPerformed(event: AnActionEvent) { event.project?.let(::startSync) }
}

internal fun startSync(project: Project) {
    runBackground(project, "Syncing Local Configuration") {
        val arguments = listOf("sync", "--project", project.basePath.orEmpty())
        try {
            LocalConfigCli.command(project, arguments)
        } catch (error: CliException) {
            if (error.code != "unsafe_secret_pattern") throw error
            if (!confirmSensitiveFiles(project, error.paths)) return@runBackground
            LocalConfigCli.command(project, arguments + "--allow-sensitive")
        }
        notify(project, "Local configuration is synchronized")
    }
}

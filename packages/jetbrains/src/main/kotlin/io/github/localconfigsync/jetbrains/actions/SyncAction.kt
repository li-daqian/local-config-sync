package io.github.localconfigsync.jetbrains.actions

import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import io.github.localconfigsync.jetbrains.cli.LocalConfigCli

class SyncAction : AnAction() {
    override fun update(event: AnActionEvent) { event.presentation.isEnabled = event.project?.basePath != null }
    override fun actionPerformed(event: AnActionEvent) {
        val project = event.project ?: return
        runBackground(project, "Syncing Local Configuration") {
            LocalConfigCli.command(project, listOf("sync", "--project", project.basePath.orEmpty()))
            notify(project, "Local configuration is synchronized")
        }
    }
}

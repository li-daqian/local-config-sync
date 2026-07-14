package io.github.localconfigsync.jetbrains.actions

import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.project.Project
import io.github.localconfigsync.jetbrains.cli.LocalConfigCli

class SyncAction : AnAction() {
    override fun update(event: AnActionEvent) { event.presentation.isEnabled = event.project?.basePath != null }
    override fun actionPerformed(event: AnActionEvent) { event.project?.let(::startSync) }
}

internal fun startSync(project: Project) {
    runBackground(project, "Syncing Local Configuration") {
        LocalConfigCli.command(project, listOf("sync", "--project", project.basePath.orEmpty()))
        notify(project, "Local configuration is synchronized")
    }
}

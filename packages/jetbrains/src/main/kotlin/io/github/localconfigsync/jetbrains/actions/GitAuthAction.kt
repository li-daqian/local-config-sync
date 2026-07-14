package io.github.localconfigsync.jetbrains.actions

import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.Messages
import io.github.localconfigsync.jetbrains.cli.LocalConfigCli

class GitAuthAction : AnAction() {
    override fun update(event: AnActionEvent) { event.presentation.isEnabled = event.project != null }
    override fun actionPerformed(event: AnActionEvent) { event.project?.let(::startGitAuth) }
}

internal fun startGitAuth(project: Project, suggestedRepositoryId: String? = null) {
    val repositoryId = Messages.showInputDialog(
        project,
        "Repository ID",
        "Git Authentication",
        null,
        suggestedRepositoryId,
        null,
    )?.trim() ?: return
    if (repositoryId.isBlank()) return
    val methods = arrayOf("auto", "ssh", "credential", "gh")
    val methodIndex = Messages.showDialog(project, "Authentication method", "Git Authentication", methods, 0, null)
    if (methodIndex < 0) return
    runBackground(project, "Checking Git Authentication") {
        LocalConfigCli.command(project, listOf("repository", "auth", repositoryId, "--method", methods[methodIndex]))
        notify(project, "Git authentication succeeded for $repositoryId")
    }
}

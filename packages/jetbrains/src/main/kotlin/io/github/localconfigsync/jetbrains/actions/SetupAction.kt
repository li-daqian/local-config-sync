package io.github.localconfigsync.jetbrains.actions

import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.ui.Messages
import io.github.localconfigsync.jetbrains.cli.LocalConfigCli

class SetupAction : AnAction() {
    override fun update(event: AnActionEvent) { event.presentation.isEnabled = event.project?.basePath != null }

    override fun actionPerformed(event: AnActionEvent) {
        val project = event.project ?: return
        val choices = arrayOf("Use existing repository", "Create Git repository")
        val choice = Messages.showChooseDialog(project, "Choose a repository setup", "Setup Local Config Sync", null, choices, choices[0])
        if (choice < 0) return

        val repositoryId = requiredInput(project, "Repository ID", "Setup Local Config Sync") ?: return
        var remoteUrl: String? = null
        var branch = "main"
        if (choice == 1) {
            remoteUrl = requiredInput(project, "Git remote URL", "Create Git Repository") ?: return
            branch = Messages.showInputDialog(project, "Branch", "Create Git Repository", null, "main", null)?.trim().orEmpty().ifBlank { "main" }
        }
        val sourcePath = requiredInput(project, "Repository source path (for example: my-project/config)", "Setup Local Config Sync") ?: return
        val targetPath = Messages.showInputDialog(project, "Project target path", "Setup Local Config Sync", null, "config", null)?.trim() ?: return
        val modes = arrayOf("symlink", "copy")
        val modeIndex = Messages.showChooseDialog(project, "Link mode", "Setup Local Config Sync", null, modes, modes[0])
        if (modeIndex < 0) return

        runBackground(project, "Setting Up Local Config Sync") {
            LocalConfigCli.command(project, listOf("init"))
            if (remoteUrl != null) {
                LocalConfigCli.command(project, listOf("repository", "auth", "--url", remoteUrl, "--method", "auto"))
                LocalConfigCli.command(project, listOf("repository", "add", "git", "--id", repositoryId, "--url", remoteUrl, "--branch", branch))
            }
            LocalConfigCli.command(project, listOf(
                "link", "--project", project.basePath.orEmpty(), "--repository", repositoryId,
                "--source-path", sourcePath, "--target", targetPath, "--mode", modes[modeIndex]
            ))
            notify(project, "Local configuration mapping is ready")
        }
    }

    private fun requiredInput(project: com.intellij.openapi.project.Project, message: String, title: String): String? {
        val value = Messages.showInputDialog(project, message, title, null)?.trim() ?: return null
        if (value.isBlank()) {
            Messages.showErrorDialog(project, "$message is required", title)
            return null
        }
        return value
    }
}

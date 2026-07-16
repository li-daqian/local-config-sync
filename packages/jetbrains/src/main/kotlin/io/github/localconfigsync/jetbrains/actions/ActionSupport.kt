package io.github.localconfigsync.jetbrains.actions

import com.intellij.notification.NotificationGroupManager
import com.intellij.notification.NotificationType
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.progress.ProgressIndicator
import com.intellij.openapi.progress.Task
import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.Messages
import io.github.localconfigsync.jetbrains.cli.CliException
import io.github.localconfigsync.jetbrains.status.LocalConfigStatusService
import java.util.concurrent.atomic.AtomicReference

internal fun runBackground(project: Project, title: String, operation: (ProgressIndicator) -> Unit) {
    object : Task.Backgroundable(project, title, true) {
        override fun run(indicator: ProgressIndicator) = operation(indicator)
        override fun onThrowable(error: Throwable) {
            reportFailure(project, error)
        }
        override fun onSuccess() {
            LocalConfigStatusService.getInstance(project).refresh()
        }
    }.queue()
}

internal fun reportFailure(project: Project, error: Throwable) {
    val cli = error as? CliException
    LocalConfigStatusService.getInstance(project).recordFailure(
        cli ?: CliException("operation_failed", error.message ?: "Operation failed"),
    )
    notify(project, "${cli?.code ?: "error"}: ${error.message}", NotificationType.ERROR)
}

internal fun notify(project: Project, content: String, type: NotificationType = NotificationType.INFORMATION) {
    NotificationGroupManager.getInstance().getNotificationGroup("Local Config Sync")
        .createNotification("Local Config Sync", content, type)
        .notify(project)
}

internal fun confirmSensitiveFiles(project: Project, paths: List<String>): Boolean = onUiThread {
    val pathSummary = paths.distinct().take(12).joinToString("\n") { "• $it" }
        .ifBlank { "• One or more mapped files" }
    val remaining = (paths.distinct().size - 12).coerceAtLeast(0)
    val remainingSummary = if (remaining > 0) "\n• …and $remaining more" else ""
    Messages.showYesNoDialog(
        project,
        "These files match sensitive file patterns and may contain secrets:\n\n" +
            pathSummary + remainingSummary +
            "\n\nRepository history or backups may retain their contents. " +
            "Only continue if you have reviewed the files and accept that risk.",
        "Sensitive Files Detected",
        "Sync Anyway",
        "Cancel",
        Messages.getWarningIcon(),
    ) == Messages.YES
}

internal fun <T> onUiThread(action: () -> T): T {
    if (ApplicationManager.getApplication().isDispatchThread) return action()
    val value = AtomicReference<Result<T>>()
    ApplicationManager.getApplication().invokeAndWait { value.set(runCatching(action)) }
    return value.get().getOrThrow()
}

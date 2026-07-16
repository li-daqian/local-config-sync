package io.github.localconfigsync.jetbrains.actions

import com.intellij.notification.NotificationGroupManager
import com.intellij.notification.NotificationType
import com.intellij.openapi.progress.ProgressIndicator
import com.intellij.openapi.progress.Task
import com.intellij.openapi.project.Project
import io.github.localconfigsync.jetbrains.cli.CliException
import io.github.localconfigsync.jetbrains.status.LocalConfigStatusService

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

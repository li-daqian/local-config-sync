package io.github.localconfigsync.jetbrains.actions

import com.intellij.notification.NotificationGroupManager
import com.intellij.notification.NotificationType
import com.intellij.openapi.progress.ProgressIndicator
import com.intellij.openapi.progress.Task
import com.intellij.openapi.project.Project
import com.intellij.openapi.wm.WindowManager
import io.github.localconfigsync.jetbrains.cli.CliException

internal fun runBackground(project: Project, title: String, operation: (ProgressIndicator) -> Unit) {
    object : Task.Backgroundable(project, title, true) {
        override fun run(indicator: ProgressIndicator) = operation(indicator)
        override fun onThrowable(error: Throwable) {
            val cli = error as? CliException
            notify(project, "${cli?.code ?: "error"}: ${error.message}", NotificationType.ERROR)
        }
        override fun onSuccess() {
            WindowManager.getInstance().getStatusBar(project)?.updateWidget("LocalConfigSyncStatus")
        }
    }.queue()
}

internal fun notify(project: Project, content: String, type: NotificationType = NotificationType.INFORMATION) {
    NotificationGroupManager.getInstance().getNotificationGroup("Local Config Sync")
        .createNotification("Local Config Sync", content, type)
        .notify(project)
}

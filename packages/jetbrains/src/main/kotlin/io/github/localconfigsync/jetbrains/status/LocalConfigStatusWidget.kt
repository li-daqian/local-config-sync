package io.github.localconfigsync.jetbrains.status

import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.project.Project
import com.intellij.openapi.wm.StatusBar
import com.intellij.openapi.wm.StatusBarWidget
import com.intellij.openapi.wm.WindowManager
import com.intellij.util.Consumer
import io.github.localconfigsync.jetbrains.cli.LocalConfigCli
import java.awt.Component
import java.awt.event.MouseEvent

class LocalConfigStatusWidget(private val project: Project) : StatusBarWidget, StatusBarWidget.TextPresentation {
    @Volatile private var state = "Checking"
    private var statusBar: StatusBar? = null

    override fun ID(): String = ID
    override fun install(statusBar: StatusBar) { this.statusBar = statusBar; refresh() }
    override fun dispose() { statusBar = null }
    override fun getPresentation(): StatusBarWidget.WidgetPresentation = this
    override fun getText(): String = "Local Config: $state"
    override fun getAlignment(): Float = Component.CENTER_ALIGNMENT
    override fun getTooltipText(): String = "Click to refresh Local Config Sync status"
    override fun getClickConsumer(): Consumer<MouseEvent> = Consumer { refresh() }

    private fun refresh() {
        if (project.isDisposed || project.basePath == null) return
        ApplicationManager.getApplication().executeOnPooledThread {
            state = try {
                LocalConfigCli.status(project).state.replaceFirstChar { it.uppercase() }
            } catch (_: Exception) {
                "Failed"
            }
            ApplicationManager.getApplication().invokeLater {
                if (!project.isDisposed) WindowManager.getInstance().getStatusBar(project)?.updateWidget(ID)
            }
        }
    }

    companion object { const val ID = "LocalConfigSyncStatus" }
}

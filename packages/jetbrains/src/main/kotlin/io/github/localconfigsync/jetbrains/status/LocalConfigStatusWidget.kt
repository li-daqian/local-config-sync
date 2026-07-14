package io.github.localconfigsync.jetbrains.status

import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.project.Project
import com.intellij.openapi.wm.CustomStatusBarWidget
import com.intellij.openapi.wm.StatusBar
import com.intellij.ui.components.JBLabel
import com.intellij.util.ui.JBUI
import io.github.localconfigsync.jetbrains.cli.LocalConfigCli
import java.awt.Cursor
import java.awt.event.MouseEvent
import java.awt.event.MouseAdapter
import javax.swing.JComponent

class LocalConfigStatusWidget(private val project: Project) : CustomStatusBarWidget {
    @Volatile private var state = "Checking"
    private val component: JBLabel by lazy {
        JBLabel(statusText()).apply {
            toolTipText = "Click to refresh Local Config Sync status"
            border = JBUI.Borders.empty(0, 6)
            cursor = Cursor.getPredefinedCursor(Cursor.HAND_CURSOR)
            addMouseListener(object : MouseAdapter() {
                override fun mouseClicked(event: MouseEvent) = refresh()
            })
        }
    }

    override fun ID(): String = ID
    override fun getComponent(): JComponent = component
    override fun install(statusBar: StatusBar) = refresh()
    override fun dispose() = Unit

    private fun statusText(): String = "Local Config: $state"

    private fun refresh() {
        if (project.isDisposed || project.basePath == null) return
        ApplicationManager.getApplication().executeOnPooledThread {
            state = try {
                LocalConfigCli.status(project).state.replaceFirstChar { it.uppercase() }
            } catch (_: Exception) {
                "Failed"
            }
            ApplicationManager.getApplication().invokeLater {
                if (!project.isDisposed) component.text = statusText()
            }
        }
    }

    companion object { const val ID = "LocalConfigSyncStatus" }
}

package io.github.localconfigsync.jetbrains.status

import com.intellij.openapi.project.Project
import com.intellij.openapi.wm.CustomStatusBarWidget
import com.intellij.openapi.wm.StatusBar
import com.intellij.openapi.wm.ToolWindowManager
import com.intellij.ui.components.JBLabel
import com.intellij.util.ui.JBUI
import io.github.localconfigsync.jetbrains.toolwindow.LocalConfigToolWindowFactory
import java.awt.Cursor
import java.awt.event.MouseAdapter
import java.awt.event.MouseEvent
import javax.swing.JComponent

class LocalConfigStatusWidget(private val project: Project) : CustomStatusBarWidget, LocalConfigStatusListener {
    private val service = LocalConfigStatusService.getInstance(project)
    private val connection = project.messageBus.connect()
    private val component = JBLabel(textFor(service.snapshot)).apply {
        toolTipText = tooltipFor(service.snapshot)
        border = JBUI.Borders.empty(0, 6)
        cursor = Cursor.getPredefinedCursor(Cursor.HAND_CURSOR)
        addMouseListener(object : MouseAdapter() {
            override fun mouseClicked(event: MouseEvent) {
                ToolWindowManager.getInstance(project).getToolWindow(LocalConfigToolWindowFactory.ID)?.show {
                    service.refresh()
                }
            }
        })
    }

    init {
        connection.subscribe(LocalConfigStatusService.TOPIC, this)
    }

    override fun ID(): String = ID
    override fun getComponent(): JComponent = component
    override fun install(statusBar: StatusBar) = service.refresh()
    override fun dispose() = connection.disconnect()

    override fun statusChanged(snapshot: LocalConfigSnapshot) {
        component.text = textFor(snapshot)
        component.toolTipText = tooltipFor(snapshot)
    }

    private fun textFor(snapshot: LocalConfigSnapshot): String = "Local Config: " + when (snapshot) {
        LocalConfigSnapshot.Loading -> "Checking"
        is LocalConfigSnapshot.Ready -> snapshot.response.state.toDisplayName()
        is LocalConfigSnapshot.Failure -> "Failed"
    }

    private fun tooltipFor(snapshot: LocalConfigSnapshot): String = when (snapshot) {
        LocalConfigSnapshot.Loading -> "Checking Local Config Sync status"
        is LocalConfigSnapshot.Ready -> "Open Local Config Sync"
        is LocalConfigSnapshot.Failure -> "${snapshot.error.code}: ${snapshot.error.message}. Click to open details."
    }

    companion object { const val ID = "LocalConfigSyncStatus" }
}

internal fun String.toDisplayName(): String = split('_').joinToString(" ") { word ->
    word.replaceFirstChar { it.uppercase() }
}

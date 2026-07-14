package io.github.localconfigsync.jetbrains.status

import com.intellij.openapi.project.Project
import com.intellij.openapi.wm.StatusBarWidget
import com.intellij.openapi.wm.StatusBarWidgetFactory

class LocalConfigStatusWidgetFactory : StatusBarWidgetFactory {
    override fun getId(): String = LocalConfigStatusWidget.ID
    override fun getDisplayName(): String = "Local Config Sync"
    override fun isAvailable(project: Project): Boolean = project.basePath != null
    override fun createWidget(project: Project): StatusBarWidget = LocalConfigStatusWidget(project)
    override fun disposeWidget(widget: StatusBarWidget) = widget.dispose()
    override fun canBeEnabledOn(statusBar: com.intellij.openapi.wm.StatusBar): Boolean = true
}

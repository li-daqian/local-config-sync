package io.github.localconfigsync.jetbrains.toolwindow

import com.intellij.openapi.Disposable
import com.intellij.openapi.options.ShowSettingsUtil
import com.intellij.openapi.project.Project
import com.intellij.openapi.wm.ToolWindow
import com.intellij.openapi.wm.ToolWindowFactory
import com.intellij.ui.JBColor
import com.intellij.ui.components.JBLabel
import com.intellij.ui.components.JBScrollPane
import com.intellij.ui.components.JBTextArea
import com.intellij.ui.content.ContentFactory
import com.intellij.util.ui.JBUI
import com.intellij.util.ui.UIUtil
import io.github.localconfigsync.jetbrains.actions.startGitAuth
import io.github.localconfigsync.jetbrains.actions.startSetup
import io.github.localconfigsync.jetbrains.actions.startSync
import io.github.localconfigsync.jetbrains.cli.MappingSummary
import io.github.localconfigsync.jetbrains.cli.RepositorySummary
import io.github.localconfigsync.jetbrains.settings.LocalConfigConfigurable
import io.github.localconfigsync.jetbrains.status.LocalConfigSnapshot
import io.github.localconfigsync.jetbrains.status.LocalConfigStatusListener
import io.github.localconfigsync.jetbrains.status.LocalConfigStatusService
import io.github.localconfigsync.jetbrains.status.toDisplayName
import java.awt.BorderLayout
import java.awt.FlowLayout
import java.awt.Font
import javax.swing.BorderFactory
import javax.swing.BoxLayout
import javax.swing.JComponent
import javax.swing.JButton
import javax.swing.JPanel

class LocalConfigToolWindowFactory : ToolWindowFactory {
    override fun createToolWindowContent(project: Project, toolWindow: ToolWindow) {
        val view = LocalConfigToolWindowPanel(project)
        val content = ContentFactory.getInstance().createContent(view.component, "", false)
        content.setDisposer(view)
        toolWindow.contentManager.addContent(content)
    }

    companion object { const val ID = "Local Config Sync" }
}

private class LocalConfigToolWindowPanel(private val project: Project) : Disposable, LocalConfigStatusListener {
    private val service = LocalConfigStatusService.getInstance(project)
    private val content = JPanel().apply { layout = BoxLayout(this, BoxLayout.Y_AXIS) }
    private val status = JBLabel("Checking").apply { font = font.deriveFont(Font.BOLD) }
    private val syncButton = JButton("Sync Now").apply { addActionListener { startSync(project) } }
    private val authButton = JButton("Git Auth").apply { addActionListener { startGitAuth(project) } }
    val component: JComponent = JPanel(BorderLayout()).apply {
        border = JBUI.Borders.empty(12)
        add(createHeader(), BorderLayout.NORTH)
        add(JBScrollPane(content).apply { border = JBUI.Borders.empty() }, BorderLayout.CENTER)
    }

    init {
        project.messageBus.connect(this).subscribe(LocalConfigStatusService.TOPIC, this)
        render(service.snapshot)
        service.refresh()
    }

    override fun dispose() = Unit

    override fun statusChanged(snapshot: LocalConfigSnapshot) = render(snapshot)

    private fun createHeader(): JComponent = JPanel(BorderLayout()).apply {
        border = JBUI.Borders.emptyBottom(12)
        add(JPanel(BorderLayout()).apply {
            add(JBLabel("Local Config Sync").apply { font = font.deriveFont(Font.BOLD, font.size2D + 2f) }, BorderLayout.WEST)
            add(status, BorderLayout.EAST)
        }, BorderLayout.NORTH)
        add(JPanel(FlowLayout(FlowLayout.LEFT, 6, 8)).apply {
            add(JButton("Refresh").apply { addActionListener { service.refresh() } })
            add(JButton("Setup").apply { addActionListener { startSetup(project) } })
            add(syncButton)
            add(authButton)
            add(JButton("Settings").apply {
                addActionListener { ShowSettingsUtil.getInstance().showSettingsDialog(project, LocalConfigConfigurable::class.java) }
            })
        }, BorderLayout.SOUTH)
    }

    private fun render(snapshot: LocalConfigSnapshot) {
        content.removeAll()
        when (snapshot) {
            LocalConfigSnapshot.Loading -> {
                status.text = "Checking"
                status.foreground = UIUtil.getLabelForeground()
                syncButton.isEnabled = false
                authButton.isEnabled = false
                content.add(message("Reading project mappings and repository state…"))
            }
            is LocalConfigSnapshot.Failure -> {
                status.text = "Failed"
                status.foreground = JBColor.namedColor("Label.errorForeground", JBColor.RED)
                syncButton.isEnabled = false
                authButton.isEnabled = false
                content.add(section("Unable to load status", listOf(
                    "Error: ${snapshot.error.code}",
                    snapshot.error.message,
                    snapshot.error.diagnostics.ifBlank { "Use Settings only if the bundled CLI needs an advanced override." },
                ), error = true))
                content.add(message("The status bar now opens this panel so failures remain visible and actionable."))
            }
            is LocalConfigSnapshot.Ready -> {
                val response = snapshot.response
                status.text = response.state.toDisplayName()
                status.foreground = when (response.state) {
                    "synced" -> JBColor.namedColor("Label.successForeground", JBColor(0x2E7D32, 0x73C991))
                    "conflict", "failed" -> JBColor.namedColor("Label.errorForeground", JBColor.RED)
                    else -> UIUtil.getLabelForeground()
                }
                syncButton.isEnabled = response.mappings.isNotEmpty()
                authButton.isEnabled = response.repositories.any { it.type == "git" }
                content.add(section("Project", listOf(
                    response.projectPath,
                    "Status: ${response.state.toDisplayName()}",
                    response.lastSyncTime?.let { "Last sync: $it" } ?: "Last sync: Never",
                )))
                if (response.mappings.isEmpty()) {
                    content.add(section("Get started", listOf(
                        "No local configuration mapping is registered for this project.",
                        "Choose Setup to connect a Git or local-folder Repository and select the project target path.",
                    ), action = JButton("Set Up This Project").apply { addActionListener { startSetup(project) } }))
                } else {
                    response.repositories.forEach { content.add(repositorySection(it)) }
                    response.mappings.forEach { content.add(mappingSection(it)) }
                }
            }
        }
        content.add(JPanel().apply { isOpaque = false })
        content.revalidate()
        content.repaint()
    }

    private fun repositorySection(repository: RepositorySummary): JComponent = section(
        "Repository · ${repository.name.ifBlank { repository.id }}",
        listOf(
            "ID: ${repository.id}",
            "Type: ${repository.type}",
            "Status: ${repository.state.toDisplayName()}",
            "Workspace: ${repository.workspacePath}",
            "Remote revision: ${repository.remoteRevision ?: "Not available"}",
        ),
        action = if (repository.type == "git") JButton("Check Git Auth").apply {
            addActionListener { startGitAuth(project, repository.id) }
        } else null,
    )

    private fun mappingSection(mapping: MappingSummary): JComponent = section(
        "Mapping · ${mapping.targetPath}",
        listOf(
            "Repository: ${mapping.repositoryId}",
            "Source: ${mapping.sourcePath}",
            "Target: ${mapping.targetPath}",
            "Mode: ${mapping.mode}",
            "Git exclude: ${if (mapping.excludeConfigured) "Configured" else "Missing"}",
            "Mapped files: ${mapping.mappedFiles.size}",
        ),
    )

    private fun section(title: String, lines: List<String>, error: Boolean = false, action: JComponent? = null): JComponent =
        JPanel(BorderLayout()).apply {
            alignmentX = JComponent.LEFT_ALIGNMENT
            border = BorderFactory.createCompoundBorder(
                BorderFactory.createTitledBorder(title),
                JBUI.Borders.empty(6, 8, 8, 8),
            )
            val text = JBTextArea(lines.joinToString("\n")).apply {
                isEditable = false
                isOpaque = false
                lineWrap = true
                wrapStyleWord = true
                border = JBUI.Borders.empty()
                foreground = if (error) JBColor.namedColor("Label.errorForeground", JBColor.RED) else UIUtil.getLabelForeground()
            }
            add(text, BorderLayout.CENTER)
            action?.let { add(JPanel(FlowLayout(FlowLayout.LEFT, 0, 6)).apply { add(it) }, BorderLayout.SOUTH) }
        }

    private fun message(text: String): JComponent = JBTextArea(text).apply {
        isEditable = false
        isOpaque = false
        lineWrap = true
        wrapStyleWord = true
        border = JBUI.Borders.empty(8)
        foreground = UIUtil.getContextHelpForeground()
        alignmentX = JComponent.LEFT_ALIGNMENT
    }
}

package io.github.localconfigsync.jetbrains.toolwindow

import com.intellij.diff.DiffManager
import com.intellij.diff.DiffContentFactory
import com.intellij.diff.requests.SimpleDiffRequest
import com.intellij.openapi.Disposable
import com.intellij.openapi.fileTypes.FileTypeManager
import com.intellij.openapi.options.ShowSettingsUtil
import com.intellij.openapi.progress.ProgressIndicator
import com.intellij.openapi.progress.Task
import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.Messages
import com.intellij.openapi.ui.popup.JBPopupFactory
import com.intellij.openapi.util.IconLoader
import com.intellij.openapi.wm.ToolWindow
import com.intellij.openapi.wm.ToolWindowFactory
import com.intellij.ui.JBColor
import com.intellij.ui.components.JBLabel
import com.intellij.ui.components.JBScrollPane
import com.intellij.ui.content.ContentFactory
import com.intellij.ui.table.JBTable
import com.intellij.util.ui.JBUI
import com.intellij.util.ui.UIUtil
import io.github.localconfigsync.jetbrains.actions.confirmSensitiveFiles
import io.github.localconfigsync.jetbrains.actions.reportFailure
import io.github.localconfigsync.jetbrains.actions.runBackground
import io.github.localconfigsync.jetbrains.actions.startGitAuth
import io.github.localconfigsync.jetbrains.actions.startSetup
import io.github.localconfigsync.jetbrains.actions.startSync
import io.github.localconfigsync.jetbrains.cli.FileDiffResponse
import io.github.localconfigsync.jetbrains.cli.FileStatusSummary
import io.github.localconfigsync.jetbrains.cli.CliException
import io.github.localconfigsync.jetbrains.cli.LocalConfigCli
import io.github.localconfigsync.jetbrains.cli.RepositorySummary
import io.github.localconfigsync.jetbrains.cli.StatusResponse
import io.github.localconfigsync.jetbrains.settings.LocalConfigConfigurable
import io.github.localconfigsync.jetbrains.status.LocalConfigSnapshot
import io.github.localconfigsync.jetbrains.status.LocalConfigStatusListener
import io.github.localconfigsync.jetbrains.status.LocalConfigStatusService
import java.awt.BorderLayout
import java.awt.Component
import java.awt.FlowLayout
import java.awt.Font
import java.awt.GridBagConstraints
import java.awt.GridBagLayout
import java.awt.Insets
import java.awt.event.MouseAdapter
import java.awt.event.MouseEvent
import java.nio.charset.StandardCharsets
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import java.util.Base64
import java.util.Locale
import javax.swing.BorderFactory
import javax.swing.JButton
import javax.swing.JComponent
import javax.swing.Icon
import javax.swing.JPanel
import javax.swing.ListSelectionModel
import javax.swing.SwingConstants
import javax.swing.table.AbstractTableModel
import javax.swing.table.DefaultTableCellRenderer

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
    private val body = JPanel(BorderLayout())
    private val status = JBLabel("Checking").apply { font = font.deriveFont(Font.BOLD) }
    private val syncButton = iconButton(SYNC_ICON, "Sync local and Repository files").apply {
        addActionListener { requestSync() }
    }
    private val authButton = iconButton(KEY_ICON, "Authenticate Git repository").apply {
        addActionListener { startGitAuth(project) }
    }
    private var response: StatusResponse? = null
    private val reviewedConflicts = mutableMapOf<String, String>()
    private var tableModel = FileStatusTableModel(emptyList())
    private val table = JBTable(tableModel).apply {
        selectionModel.selectionMode = ListSelectionModel.SINGLE_SELECTION
        rowHeight = JBUI.scale(30)
        showVerticalLines = false
        showHorizontalLines = true
        intercellSpacing = JBUI.size(0, 1)
        tableHeader.reorderingAllowed = false
        setDefaultRenderer(Any::class.java, FileStatusCellRenderer())
        selectionModel.addListSelectionListener { updateConflictActions() }
        addMouseListener(object : MouseAdapter() {
            override fun mouseClicked(event: MouseEvent) {
                if (event.clickCount == 2) selectedFile()?.takeIf { it.status != "synced" }?.let(::showDiff)
            }
        })
    }
    private val conflictActions = JPanel(FlowLayout(FlowLayout.LEFT, 6, 6))

    val component: JComponent = JPanel(BorderLayout()).apply {
        border = JBUI.Borders.empty(12)
        add(createHeader(), BorderLayout.NORTH)
        add(body, BorderLayout.CENTER)
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
            add(iconButton(REFRESH_ICON, "Refresh status").apply { addActionListener { service.refresh() } })
            add(syncButton)
            add(authButton)
            add(iconButton(SETTINGS_ICON, "Open Local Config Sync settings").apply {
                addActionListener { ShowSettingsUtil.getInstance().showSettingsDialog(project, LocalConfigConfigurable::class.java) }
            })
        }, BorderLayout.SOUTH)
    }

    private fun render(snapshot: LocalConfigSnapshot) {
        body.removeAll()
        when (snapshot) {
            LocalConfigSnapshot.Loading -> renderMessage("Reading mappings and comparing local and repository files…")
            is LocalConfigSnapshot.Failure -> renderFailure(snapshot)
            is LocalConfigSnapshot.Ready -> renderReady(snapshot.response)
        }
        body.revalidate()
        body.repaint()
    }

    private fun renderMessage(text: String) {
        response = null
        status.text = "Checking"
        status.foreground = UIUtil.getLabelForeground()
        syncButton.isEnabled = false
        authButton.isEnabled = false
        body.add(JBLabel(text).apply {
            foreground = UIUtil.getContextHelpForeground()
            border = JBUI.Borders.empty(16, 8)
        }, BorderLayout.NORTH)
    }

    private fun renderFailure(snapshot: LocalConfigSnapshot.Failure) {
        response = null
        status.text = "Failed"
        status.foreground = errorColor()
        syncButton.isEnabled = false
        authButton.isEnabled = false
        body.add(detailsCard(
            "Unable to load status",
            listOf(
                "Error" to snapshot.error.code,
                "Message" to snapshot.error.message,
                "Diagnostics" to snapshot.error.diagnostics.ifBlank { "No additional diagnostics" },
            ),
            true,
        ), BorderLayout.NORTH)
    }

    private fun renderReady(value: StatusResponse) {
        response = value
        reviewedConflicts.clear()
        status.text = syncStateName(value.state)
        status.foreground = statusColor(value.state)
        syncButton.isEnabled = value.mappings.isNotEmpty()
        syncButton.toolTipText = syncTooltip(value.files)
        syncButton.accessibleContext.accessibleDescription = syncButton.toolTipText
        authButton.isEnabled = value.repositories.any { it.type == "git" }

        val summary = JPanel(BorderLayout()).apply {
            border = JBUI.Borders.emptyBottom(10)
            add(JPanel(FlowLayout(FlowLayout.LEFT, 12, 0)).apply {
                isOpaque = false
                add(iconButton(PROJECT_ICON, "Show project details").apply {
                    addActionListener { showProjectDetails(this, value) }
                })
                if (value.repositories.isNotEmpty()) {
                    val tooltip = if (value.repositories.size == 1) {
                        "Show repository details"
                    } else {
                        "Show ${value.repositories.size} repositories"
                    }
                    add(iconButton(REPOSITORY_ICON, tooltip).apply {
                        addActionListener { showRepositoryDetails(this, value.repositories) }
                    })
                }
            }, BorderLayout.WEST)
            add(JBLabel(formatLastSync(value.lastSyncTime)).apply {
                foreground = UIUtil.getContextHelpForeground()
                toolTipText = value.lastSyncTime
            }, BorderLayout.EAST)
        }
        body.add(summary, BorderLayout.NORTH)

        tableModel = FileStatusTableModel(value.files)
        table.model = tableModel
        table.columnModel.getColumn(0).preferredWidth = JBUI.scale(230)
        table.columnModel.getColumn(1).preferredWidth = JBUI.scale(230)
        table.columnModel.getColumn(2).preferredWidth = JBUI.scale(170)
        table.emptyText.text = if (value.mappings.isEmpty()) {
            "No mapped files. Add a mapping to start syncing."
        } else {
            "No files were found in the configured mappings."
        }

        val tablePanel = JPanel(BorderLayout()).apply {
            border = BorderFactory.createCompoundBorder(
                JBUI.Borders.customLine(JBColor.border(), 1),
                JBUI.Borders.empty(),
            )
            add(JPanel(BorderLayout()).apply {
                border = JBUI.Borders.empty(6, 8)
                add(JBLabel("Mapped files").apply { font = font.deriveFont(Font.BOLD) }, BorderLayout.WEST)
                add(JButton("+ Add Mapping").apply { addActionListener { startSetup(project) } }, BorderLayout.EAST)
            }, BorderLayout.NORTH)
            add(JBScrollPane(table).apply { border = JBUI.Borders.empty() }, BorderLayout.CENTER)
            add(conflictActions, BorderLayout.SOUTH)
        }
        body.add(tablePanel, BorderLayout.CENTER)
        updateConflictActions()
    }

    private fun selectedFile(): FileStatusSummary? {
        val selected = table.selectedRow
        return if (selected >= 0) tableModel.rowAt(table.convertRowIndexToModel(selected)) else null
    }

    private fun requestSync() {
        val value = response ?: return
        val conflicts = value.files.filter { it.status == "conflict" }
        if (conflicts.isNotEmpty()) {
            val firstConflict = conflicts.first()
            focusFile(firstConflict)
            Messages.showWarningDialog(
                project,
                "${conflicts.size} conflict${if (conflicts.size == 1) "" else "s"} must be resolved before syncing.\n\n" +
                    "The first conflict will open in the diff viewer. Review both versions, then choose whether " +
                    "the Local version should be uploaded or the Repository version should be downloaded.",
                "Resolve Conflicts Before Sync",
            )
            showDiff(firstConflict)
            return
        }

        val changed = value.files.filter { it.status == "local_changes" || it.status == "remote_changes" }
        if (changed.isEmpty()) {
            Messages.showInfoMessage(project, "Local and Repository files already match.", "Local Config Sync")
            return
        }
        val confirmed = Messages.showYesNoDialog(
            project,
            syncConfirmation(changed),
            "Review Sync Direction",
            "Sync",
            "Cancel",
            Messages.getQuestionIcon(),
        ) == Messages.YES
        if (confirmed) startSync(project)
    }

    private fun focusFile(file: FileStatusSummary) {
        val modelRow = tableModel.indexOf(file)
        if (modelRow < 0) return
        val viewRow = table.convertRowIndexToView(modelRow)
        table.selectionModel.setSelectionInterval(viewRow, viewRow)
        table.scrollRectToVisible(table.getCellRect(viewRow, 0, true))
    }

    private fun updateConflictActions() {
        conflictActions.removeAll()
        val file = selectedFile()
        if (file?.status == "conflict") {
            conflictActions.add(JBLabel("Conflict").apply {
                font = font.deriveFont(Font.BOLD)
                foreground = errorColor()
            })
            conflictActions.add(JButton("View Diff").apply { addActionListener { showDiff(file) } })
            val mapping = response?.mappings?.firstOrNull { it.id == file.mappingId }
            if (mapping?.kind == "file" && mapping.mode == "copy") {
                val reviewed = reviewedConflicts[conflictKey(file)] != null
                conflictActions.add(JButton("Use Local → Repository").apply {
                    isEnabled = reviewed
                    addActionListener { resolve(file, "local") }
                })
                conflictActions.add(JButton("Use Repository → Local").apply {
                    isEnabled = reviewed
                    addActionListener { resolve(file, "remote") }
                })
                if (!reviewed) {
                    conflictActions.add(JBLabel("Review the diff to enable resolution.").apply {
                        foreground = UIUtil.getContextHelpForeground()
                    })
                }
            } else {
                conflictActions.add(JBLabel("Open the diff and resolve this directory or symlink mapping manually.").apply {
                    foreground = UIUtil.getContextHelpForeground()
                })
            }
        }
        conflictActions.isVisible = conflictActions.componentCount > 0
        conflictActions.revalidate()
        conflictActions.repaint()
    }

    private fun showDiff(file: FileStatusSummary) {
        var result: FileDiffResponse? = null
        object : Task.Backgroundable(project, "Loading Local Config Diff", true) {
            override fun run(indicator: ProgressIndicator) {
                result = LocalConfigCli.diff(project, file.mappingId, file.remotePath)
            }

            override fun onSuccess() {
                result?.let(::openDiff)
            }

            override fun onThrowable(error: Throwable) = reportFailure(project, error)
        }.queue()
    }

    private fun openDiff(diff: FileDiffResponse) {
        reviewedConflicts["${diff.mappingId}\u0000${diff.remotePath}"] = diff.remoteRevision
        updateConflictActions()
        val fileType = FileTypeManager.getInstance().getFileTypeByFileName(diff.localPath)
        val factory = DiffContentFactory.getInstance()
        val local = factory.create(project, decode(diff.localContent), fileType)
        val remote = factory.create(project, decode(diff.remoteContent), fileType)
        DiffManager.getInstance().showDiff(
            project,
            SimpleDiffRequest(
                "Local Config Diff · ${displayFileName(diff.localPath)}",
                local,
                remote,
                "Local · ${diff.localPath}${if (diff.localExists) "" else " (deleted)"}",
                "Repository · ${diff.remotePath}${if (diff.remoteExists) "" else " (deleted)"}",
            ),
        )
    }

    private fun resolve(file: FileStatusSummary, strategy: String) {
        val useLocal = strategy == "local"
        val message = if (useLocal) {
            "Use the local file and publish it to the repository?\n\nRemote changes to this mapped file will be replaced by a new, explicit commit."
        } else {
            "Use the repository file and replace the local file?\n\nUnpublished local changes to this mapped file will be discarded."
        }
        val confirmed = Messages.showYesNoDialog(
            project,
            message,
            "Resolve Local Config Conflict",
            if (useLocal) "Upload Local" else "Download Repository",
            "Cancel",
            Messages.getWarningIcon(),
        ) == Messages.YES
        if (!confirmed) return
        val expectedRevision = reviewedConflicts[conflictKey(file)] ?: return
        runBackground(project, "Resolving Local Config Conflict") { indicator ->
            indicator.text = "Applying the selected version safely…"
            val arguments = listOf(
                "resolve", "--project", project.basePath.orEmpty(),
                "--mapping", file.mappingId, "--path", file.remotePath,
                "--expected-revision", expectedRevision, "--strategy", strategy,
            )
            try {
                LocalConfigCli.command(project, arguments)
            } catch (error: CliException) {
                if (error.code != "unsafe_secret_pattern") throw error
                if (!confirmSensitiveFiles(project, error.paths)) return@runBackground
                LocalConfigCli.command(project, arguments + "--allow-sensitive")
            }
        }
    }

    private fun showProjectDetails(anchor: Component, value: StatusResponse) = showDetails(
        anchor,
        "Project",
        listOf(
            "Path" to value.projectPath,
            "Status" to syncStateName(value.state),
            "Last sync" to formatLastSync(value.lastSyncTime).removePrefix("Last sync: "),
            "Mappings" to value.mappings.size.toString(),
        ),
    )

    private fun showRepositoryDetails(anchor: Component, repositories: List<RepositorySummary>) {
        val values = repositories.flatMapIndexed { index, repository ->
            buildList {
                if (repositories.size > 1) add("Repository ${index + 1}" to repository.name.ifBlank { repository.id })
                add("Name" to repository.name.ifBlank { repository.id })
                add("ID" to repository.id)
                add("Type" to repository.type)
                add("Status" to syncStateName(repository.state))
                add("Workspace" to repository.workspacePath)
                add("Revision" to (repository.remoteRevision ?: "Not available"))
            }
        }
        showDetails(anchor, if (repositories.size == 1) "Repository" else "Repositories", values)
    }

    private fun showDetails(anchor: Component, title: String, values: List<Pair<String, String>>) {
        JBPopupFactory.getInstance()
            .createComponentPopupBuilder(detailsCard(null, values), null)
            .setTitle(title)
            .setResizable(true)
            .setMovable(true)
            .setRequestFocus(false)
            .createPopup()
            .showUnderneathOf(anchor)
    }

    private fun detailsCard(title: String?, values: List<Pair<String, String>>, error: Boolean = false): JComponent =
        JPanel(GridBagLayout()).apply {
            border = JBUI.Borders.empty(10, 12)
            var row = 0
            title?.let {
                add(JBLabel(it).apply {
                    font = font.deriveFont(Font.BOLD, font.size2D + 1f)
                    if (error) foreground = errorColor()
                }, GridBagConstraints().apply {
                    gridx = 0
                    gridy = row++
                    gridwidth = 2
                    anchor = GridBagConstraints.WEST
                    insets = Insets(0, 0, JBUI.scale(10), 0)
                })
            }
            values.forEach { (label, value) ->
                add(JBLabel(label).apply { foreground = UIUtil.getContextHelpForeground() }, GridBagConstraints().apply {
                    gridx = 0
                    gridy = row
                    anchor = GridBagConstraints.NORTHWEST
                    insets = Insets(2, 0, 5, JBUI.scale(14))
                })
                add(JBLabel(value).apply {
                    toolTipText = value
                    if (error) foreground = errorColor()
                }, GridBagConstraints().apply {
                    gridx = 1
                    gridy = row++
                    weightx = 1.0
                    fill = GridBagConstraints.HORIZONTAL
                    anchor = GridBagConstraints.NORTHWEST
                    insets = Insets(2, 0, 5, 0)
                })
            }
        }

    private fun decode(content: String): String = String(Base64.getDecoder().decode(content), StandardCharsets.UTF_8)

    private fun conflictKey(file: FileStatusSummary): String = "${file.mappingId}\u0000${file.remotePath}"

    private fun formatLastSync(value: String?): String {
        if (value.isNullOrBlank()) return "Last sync: Never"
        val formatted = runCatching {
            DateTimeFormatter.ofLocalizedDateTime(FormatStyle.MEDIUM)
                .withLocale(Locale.getDefault())
                .withZone(ZoneId.systemDefault())
                .format(Instant.parse(value))
        }.getOrElse { value }
        return "Last sync: $formatted"
    }
}

private class FileStatusTableModel(private val rows: List<FileStatusSummary>) : AbstractTableModel() {
    private val columns = listOf("Local File", "Repository File", "Status")

    override fun getRowCount(): Int = rows.size
    override fun getColumnCount(): Int = columns.size
    override fun getColumnName(column: Int): String = columns[column]
    override fun getValueAt(row: Int, column: Int): Any = when (column) {
        0 -> FilePathCell(displayFileName(rows[row].localPath), rows[row].localPath)
        1 -> FilePathCell(displayFileName(rows[row].remotePath), rows[row].remotePath)
        else -> rows[row].status
    }

    fun rowAt(row: Int): FileStatusSummary = rows[row]
    fun indexOf(file: FileStatusSummary): Int = rows.indexOf(file)
}

private data class FilePathCell(val fileName: String, val fullPath: String)

private class FileStatusCellRenderer : DefaultTableCellRenderer() {
    override fun getTableCellRendererComponent(
        table: javax.swing.JTable,
        value: Any?,
        isSelected: Boolean,
        hasFocus: Boolean,
        row: Int,
        column: Int,
    ): Component {
        super.getTableCellRendererComponent(table, value, isSelected, hasFocus, row, column)
        border = JBUI.Borders.empty(0, 8)
        horizontalAlignment = if (column == 2) SwingConstants.LEFT else SwingConstants.LEADING
        if (column == 2) {
            val state = value?.toString().orEmpty()
            text = fileStatusName(state)
            if (!isSelected) foreground = statusColor(state)
            toolTipText = when (state) {
                "local_changes" -> "The Local version will be uploaded to the Repository; double-click to review the diff"
                "remote_changes" -> "The Repository version will be downloaded to Local; double-click to review the diff"
                "conflict" -> "Both sides changed; review the diff before choosing a version"
                else -> "Local and repository files match"
            }
        } else {
            val path = value as? FilePathCell
            text = path?.fileName ?: value?.toString().orEmpty()
            toolTipText = path?.fullPath ?: value?.toString()
        }
        return this
    }
}

private fun fileStatusName(state: String): String = when (state) {
    "local_changes" -> "Upload → Repository"
    "remote_changes" -> "Download → Local"
    "conflict" -> "Conflict"
    else -> "Synced"
}

private fun syncStateName(state: String): String = state
    .split('_')
    .joinToString(" ") { word -> word.replaceFirstChar { it.uppercase() } }

private fun errorColor(): JBColor = JBColor.namedColor("Label.errorForeground", JBColor(0xC62828, 0xF14C4C))

private fun statusColor(state: String): JBColor = when (state) {
    "synced" -> JBColor.namedColor("Label.successForeground", JBColor(0x2E7D32, 0x73C991))
    "conflict", "failed" -> errorColor()
    "local_changes", "remote_changes", "pending" -> JBColor.namedColor("Label.warningForeground", JBColor(0x9A6700, 0xCCA700))
    else -> JBColor.namedColor("Label.foreground", JBColor.BLACK)
}

internal fun displayFileName(path: String): String = path
    .trimEnd('/', '\\')
    .substringAfterLast('/')
    .substringAfterLast('\\')
    .ifBlank { path }

internal fun syncConfirmation(files: List<FileStatusSummary>): String {
    val uploads = files.filter { it.status == "local_changes" }
    val downloads = files.filter { it.status == "remote_changes" }
    return buildString {
        append("This sync will perform the following actions:\n")
        appendSyncFiles("Upload Local → Repository", uploads)
        appendSyncFiles("Download Repository → Local", downloads)
        append("\nThe CLI will recheck the Repository revision before writing.")
    }
}

private fun StringBuilder.appendSyncFiles(title: String, files: List<FileStatusSummary>) {
    if (files.isEmpty()) return
    append("\n$title (${files.size}):\n")
    files.take(8).forEach { append("  • ${displayFileName(it.localPath)}\n") }
    if (files.size > 8) append("  • …and ${files.size - 8} more\n")
}

private fun syncTooltip(files: List<FileStatusSummary>): String {
    val uploads = files.count { it.status == "local_changes" }
    val downloads = files.count { it.status == "remote_changes" }
    val conflicts = files.count { it.status == "conflict" }
    return when {
        conflicts > 0 -> "Resolve $conflicts conflict${if (conflicts == 1) "" else "s"} before syncing"
        uploads + downloads > 0 -> "Sync $uploads upload${if (uploads == 1) "" else "s"} and $downloads download${if (downloads == 1) "" else "s"}"
        else -> "Local and Repository files are synchronized"
    }
}

private fun iconButton(icon: Icon, tooltip: String): JButton = JButton(icon).apply {
    toolTipText = tooltip
    isFocusable = false
    accessibleContext.accessibleName = tooltip
    accessibleContext.accessibleDescription = tooltip
}

private val REFRESH_ICON = IconLoader.getIcon("/icons/refresh.svg", LocalConfigToolWindowFactory::class.java)
private val SYNC_ICON = IconLoader.getIcon("/icons/sync.svg", LocalConfigToolWindowFactory::class.java)
private val KEY_ICON = IconLoader.getIcon("/icons/key.svg", LocalConfigToolWindowFactory::class.java)
private val SETTINGS_ICON = IconLoader.getIcon("/icons/settings.svg", LocalConfigToolWindowFactory::class.java)
private val PROJECT_ICON = IconLoader.getIcon("/icons/project.svg", LocalConfigToolWindowFactory::class.java)
private val REPOSITORY_ICON = IconLoader.getIcon("/icons/repository.svg", LocalConfigToolWindowFactory::class.java)

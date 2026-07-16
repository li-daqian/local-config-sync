package io.github.localconfigsync.jetbrains.actions

import com.intellij.diff.DiffContentFactory
import com.intellij.diff.DiffDialogHints
import com.intellij.diff.DiffManager
import com.intellij.diff.requests.SimpleDiffRequest
import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.fileChooser.FileChooser
import com.intellij.openapi.fileChooser.FileChooserDescriptorFactory
import com.intellij.openapi.progress.ProgressManager
import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.ComboBox
import com.intellij.openapi.ui.DialogWrapper
import com.intellij.openapi.ui.Messages
import com.intellij.openapi.vfs.LocalFileSystem
import com.intellij.ui.SearchTextField
import com.intellij.ui.components.JBLabel
import com.intellij.util.ui.JBUI
import io.github.localconfigsync.jetbrains.cli.CliException
import io.github.localconfigsync.jetbrains.cli.ConfiguredRepository
import io.github.localconfigsync.jetbrains.cli.GitHubRepositoriesResponse
import io.github.localconfigsync.jetbrains.cli.GitHubRepository
import io.github.localconfigsync.jetbrains.cli.LocalConfigCli
import io.github.localconfigsync.jetbrains.cli.MappingPreviewResponse
import io.github.localconfigsync.jetbrains.cli.RepositoryFilesResponse
import io.github.localconfigsync.jetbrains.cli.RepositoryListResponse
import java.nio.file.Files
import java.nio.file.Path
import java.util.concurrent.atomic.AtomicReference
import javax.swing.Action
import javax.swing.Box
import javax.swing.BoxLayout
import javax.swing.DefaultComboBoxModel
import javax.swing.JComponent
import javax.swing.JPanel
import javax.swing.JScrollPane
import javax.swing.JTextArea
import javax.swing.SwingUtilities
import javax.swing.event.DocumentEvent
import javax.swing.event.DocumentListener

class SetupAction : AnAction() {
    override fun update(event: AnActionEvent) { event.presentation.isEnabled = event.project?.basePath != null }
    override fun actionPerformed(event: AnActionEvent) { event.project?.let(::startSetup) }
}

private const val CREATE_REMOTE_FILE = "Create a new remote file from a local file…"

internal fun startSetup(project: Project) {
    val providers = arrayOf("GitHub")
    val provider = Messages.showDialog(project, "Choose a repository provider", "Setup Local Config Sync", providers, 0, null)
    if (provider < 0) return

    val repositories = try {
        loadGitHubRepositories(project)
    } catch (error: Throwable) {
        reportFailure(project, error)
        return
    } ?: return
    val selected = chooseGitHubRepository(project, repositories) ?: return

    runBackground(project, "Setting Up Local Config Sync") { indicator ->
        indicator.text = "Preparing ${selected.nameWithOwner}…"
        val repositoryId = ensureRepositoryConfigured(project, selected)
        indicator.text = "Loading repository files…"
        val repositoryFiles = LocalConfigCli.execute(
            project,
            listOf("repository", "files", repositoryId),
            RepositoryFilesResponse::class.java,
        ).files.orEmpty()
        val paths = chooseMappingPaths(project, repositoryFiles) ?: return@runBackground
        indicator.text = "Comparing local and repository files…"
        val preview = LocalConfigCli.execute(
            project,
            listOf(
                "preview", "--project", project.basePath.orEmpty(), "--repository", repositoryId,
                "--source-path", paths.remotePath, "--target", paths.localPath, "--kind", "file",
            ),
            MappingPreviewResponse::class.java,
        )
        val strategy = chooseInitialStrategy(project, preview) ?: return@runBackground
        indicator.text = "Creating file mapping and synchronizing…"
        LocalConfigCli.command(
            project,
            listOf(
                "link", "--project", project.basePath.orEmpty(), "--repository", repositoryId,
                "--source-path", paths.remotePath, "--target", paths.localPath,
                "--kind", "file", "--mode", "copy", "--initial-strategy", strategy,
            ),
        )
        LocalConfigCli.command(project, listOf("sync", "--project", project.basePath.orEmpty()))
        notify(project, "${paths.localPath} is synchronized with ${selected.nameWithOwner}:${paths.remotePath}")
    }
}

private fun loadGitHubRepositories(project: Project): List<GitHubRepository>? {
    val result = AtomicReference<Result<List<GitHubRepository>?>?>()
    val completed = ProgressManager.getInstance().runProcessWithProgressSynchronously(
        {
            result.set(runCatching {
                ProgressManager.getInstance().progressIndicator?.text = "Checking GitHub authentication…"
                LocalConfigCli.command(project, listOf("init", "--default-link-mode", "copy"))
                if (!ensureGitHubAuthentication(project)) return@runCatching null
                ProgressManager.getInstance().progressIndicator?.text = "Loading GitHub repositories…"
                LocalConfigCli.execute(
                    project,
                    listOf("provider", "github", "repositories"),
                    GitHubRepositoriesResponse::class.java,
                ).repositories.orEmpty()
            })
        },
        "Loading GitHub Repositories",
        true,
        project,
    )
    if (!completed) return null
    return result.get()?.getOrThrow()
}

private fun ensureGitHubAuthentication(project: Project): Boolean {
    try {
        LocalConfigCli.command(project, listOf("provider", "github", "auth"))
        return true
    } catch (error: CliException) {
        if (error.code != "auth_failed") throw error
    }
    val login = onUiThread {
        Messages.showYesNoDialog(
            project,
            "GitHub authentication is required. Open the GitHub CLI browser login now?\n\n" +
                "Credentials remain managed by GitHub CLI; Local Config Sync does not store the token.",
            "Authenticate with GitHub",
            "Authenticate",
            "Cancel",
            null,
        ) == Messages.YES
    }
    if (!login) return false
    val dialog = onUiThread { GitHubAuthenticationDialog(project) }
    val session = LocalConfigCli.startGithubAuthentication(
        project,
        onOutput = dialog::append,
        onFinished = dialog::finish,
    )
    val authenticated = onUiThread { dialog.showAndGet() }
    if (!authenticated) {
        session.cancel()
        throw CliException(
            "auth_failed",
            "GitHub authentication was cancelled or failed. Run `gh auth login --hostname github.com` in a terminal and retry.",
        )
    }
    if (!session.waitFor(10_000) || session.exitCode != 0) {
        session.cancel()
        throw CliException("auth_failed", "GitHub authentication did not complete successfully.")
    }
    LocalConfigCli.command(project, listOf("provider", "github", "auth"))
    return true
}

private class GitHubAuthenticationDialog(project: Project) : DialogWrapper(project, true) {
    private val output = JTextArea(12, 64).apply {
        isEditable = false
        lineWrap = true
        wrapStyleWord = true
        text = "Starting GitHub browser authentication…\n" +
            "Copy the one-time code shown below if the GitHub page asks for it.\n\n"
    }

    init {
        title = "Authenticate with GitHub"
        init()
    }

    override fun createCenterPanel(): JComponent = JScrollPane(output)
    override fun createActions(): Array<Action> = arrayOf(cancelAction)

    fun append(text: String) {
        SwingUtilities.invokeLater {
            output.append(text.replace(Regex("\\u001B\\[[;\\d]*m"), ""))
            output.caretPosition = output.document.length
        }
    }

    fun finish(exitCode: Int) {
        SwingUtilities.invokeLater {
            if (exitCode == 0) {
                close(OK_EXIT_CODE)
            } else {
                output.append("\nAuthentication failed. Review the message above, then close this dialog.\n")
            }
        }
    }
}

private fun chooseGitHubRepository(project: Project, repositories: List<GitHubRepository>): GitHubRepository? {
    if (repositories.isEmpty()) {
        onUiThread { Messages.showInfoMessage(project, "No accessible GitHub repositories were found.", "Setup Local Config Sync") }
        return null
    }
    return onUiThread {
        GitHubRepositorySelectionDialog(project, repositories).let { dialog ->
            if (dialog.showAndGet()) dialog.selectedRepository else null
        }
    }
}

private data class RepositoryOption(val repository: GitHubRepository) {
    override fun toString(): String = repository.nameWithOwner + if (repository.private) "  · Private" else "  · Public"
}

private class GitHubRepositorySelectionDialog(
    project: Project,
    repositories: List<GitHubRepository>,
) : DialogWrapper(project, true) {
    private val allOptions = repositories.map(::RepositoryOption)
    private val search = SearchTextField(false)
    private val comboBox = ComboBox(DefaultComboBoxModel(allOptions.toTypedArray())).apply {
        setMinimumAndPreferredWidth(560)
    }

    val selectedRepository: GitHubRepository?
        get() = (comboBox.selectedItem as? RepositoryOption)?.repository

    init {
        title = "Choose a GitHub Repository"
        setOKButtonText("Select")
        init()
        search.addDocumentListener(object : DocumentListener {
            override fun insertUpdate(event: DocumentEvent) = filterRepositories()
            override fun removeUpdate(event: DocumentEvent) = filterRepositories()
            override fun changedUpdate(event: DocumentEvent) = filterRepositories()
        })
    }

    override fun createCenterPanel(): JComponent = JPanel().apply {
        layout = BoxLayout(this, BoxLayout.Y_AXIS)
        border = JBUI.Borders.empty(8)
        add(JBLabel("Search repositories"))
        add(Box.createVerticalStrut(JBUI.scale(4)))
        add(search)
        add(Box.createVerticalStrut(JBUI.scale(12)))
        add(JBLabel("Repository"))
        add(Box.createVerticalStrut(JBUI.scale(4)))
        add(comboBox)
    }

    override fun getPreferredFocusedComponent(): JComponent = search.textEditor

    private fun filterRepositories() {
        val query = search.text.trim()
        val matches = filterGitHubRepositories(allOptions.map(RepositoryOption::repository), query).toSet()
        val filtered = allOptions.filter { it.repository in matches }
        comboBox.model = DefaultComboBoxModel(filtered.toTypedArray())
        setOKActionEnabled(filtered.isNotEmpty())
    }
}

internal fun filterGitHubRepositories(repositories: List<GitHubRepository>, query: String): List<GitHubRepository> {
    val normalized = query.trim()
    return repositories.filter { repository ->
        normalized.isEmpty() || repository.nameWithOwner.contains(normalized, ignoreCase = true)
    }
}

private fun ensureRepositoryConfigured(project: Project, selected: GitHubRepository): String {
    val configured = LocalConfigCli.execute(
        project,
        listOf("repository", "list"),
        RepositoryListResponse::class.java,
    ).repositories.orEmpty()
    configured.firstOrNull { it.matches(selected) }?.let { return it.id }

    val baseId = githubRepositoryId(selected.nameWithOwner)
    val repositoryId = if (configured.none { it.id == baseId }) {
        baseId
    } else {
        "$baseId-${selected.nameWithOwner.hashCode().toUInt().toString(16)}"
    }
    LocalConfigCli.command(
        project,
        listOf(
            "repository", "add", "git", "--id", repositoryId, "--name", selected.nameWithOwner,
            "--url", selected.url, "--branch", selected.defaultBranch,
        ),
    )
    return repositoryId
}

private data class MappingPaths(val remotePath: String, val localPath: String)

private fun chooseMappingPaths(project: Project, remoteFiles: List<String>): MappingPaths? {
    val options = (remoteFiles + CREATE_REMOTE_FILE).toTypedArray()
    val selected = onUiThread {
        Messages.showDialog(
            project,
            "Choose a remote file, or create one from an existing local file",
            "Setup File Synchronization",
            options,
            0,
            null,
        )
    }
    if (selected < 0) return null
    if (selected == remoteFiles.size) return chooseLocalUploadPaths(project)

    val remotePath = remoteFiles[selected]
    val localPath = onUiThread {
        Messages.showInputDialog(
            project,
            "Local project file path",
            "Choose Local File Location",
            null,
            remotePath.substringAfterLast('/'),
            null,
        )?.trim()
    }?.takeIf { it.isNotEmpty() } ?: return null
    return MappingPaths(remotePath, localPath)
}

private fun chooseLocalUploadPaths(project: Project): MappingPaths? {
    val projectPath = Path.of(project.basePath.orEmpty()).toAbsolutePath().normalize()
    val initial = LocalFileSystem.getInstance().findFileByNioFile(projectPath)
    val selected = onUiThread {
        FileChooser.chooseFile(FileChooserDescriptorFactory.createSingleFileNoJarsDescriptor(), project, initial)
    } ?: return null
    val selectedPath = Path.of(selected.path).toAbsolutePath().normalize()
    if (!selectedPath.startsWith(projectPath)) {
        onUiThread { Messages.showErrorDialog(project, "Choose a file inside the current project.", "Setup Local Config Sync") }
        return null
    }
    val localPath = projectPath.relativize(selectedPath).toString().replace('\\', '/')
    val remotePath = onUiThread {
        Messages.showInputDialog(
            project,
            "Path inside the GitHub repository",
            "Choose Remote File Location",
            null,
            "${project.name}/$localPath",
            null,
        )?.trim()
    }?.takeIf { it.isNotEmpty() } ?: return null
    return MappingPaths(remotePath, localPath)
}

private fun chooseInitialStrategy(project: Project, preview: MappingPreviewResponse): String? = when (preview.state) {
    "remote_only" -> "remote"
    "local_only" -> "local"
    "identical" -> "auto"
    "conflict" -> {
        val localContent = Files.readString(Path.of(preview.targetAbsolutePath))
        val remoteContent = Files.readString(Path.of(preview.sourceAbsolutePath))
        onUiThread {
            val contentFactory = DiffContentFactory.getInstance()
            val request = SimpleDiffRequest(
                "Local Config Sync · Initial Conflict",
                contentFactory.create(project, localContent),
                contentFactory.create(project, remoteContent),
                "Local · ${preview.targetPath}",
                "GitHub · ${preview.sourcePath}",
            )
            DiffManager.getInstance().showDiff(project, request, DiffDialogHints.MODAL)
        }
        val choice = onUiThread {
            Messages.showDialog(
                project,
                "The files differ. Which version should become the initial synchronized version?",
                "Resolve Initial File Conflict",
                arrayOf("Use Local", "Use GitHub", "Cancel"),
                2,
                null,
            )
        }
        when (choice) {
            0 -> "local"
            1 -> "remote"
            else -> null
        }
    }
    else -> throw CliException("invalid_arguments", "Neither the local nor GitHub file exists.")
}

internal fun githubRepositoryId(nameWithOwner: String): String =
    "github-" + nameWithOwner.lowercase().replace(Regex("[^a-z0-9._-]+"), "-").trim('-').take(56)

private fun ConfiguredRepository.matches(repository: GitHubRepository): Boolean =
    type == "git" && (options.remoteUrl == repository.url || options.remoteUrl == repository.sshUrl)

private fun <T> onUiThread(action: () -> T): T {
    if (ApplicationManager.getApplication().isDispatchThread) return action()
    val value = AtomicReference<Result<T>>()
    ApplicationManager.getApplication().invokeAndWait { value.set(runCatching(action)) }
    return value.get().getOrThrow()
}

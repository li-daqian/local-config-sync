package io.github.localconfigsync.jetbrains.actions

import io.github.localconfigsync.jetbrains.cli.GitHubRepository
import java.nio.file.Path
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class SetupActionTest {
    @Test
    fun `github repository names become valid stable repository ids`() {
        assertEquals("github-li-daqian-private-configs", githubRepositoryId("Li-Daqian/private-configs"))
        assertTrue(githubRepositoryId("Owner/Repo With Spaces").length <= 63)
    }

    @Test
    fun `repository search is case insensitive and matches owner or name`() {
        val repositories = listOf(
            GitHubRepository(nameWithOwner = "li-daqian/private-config", private = true),
            GitHubRepository(nameWithOwner = "another/public-tools"),
        )

        assertEquals(listOf(repositories[0]), filterGitHubRepositories(repositories, "PRIVATE"))
        assertEquals(listOf(repositories[1]), filterGitHubRepositories(repositories, "another"))
        assertEquals(repositories, filterGitHubRepositories(repositories, "  "))
    }

    @Test
    fun `remote file search is case insensitive and matches paths`() {
        val remoteFiles = listOf(
            "rokid/ai-rvis-agent/application-dev.yml",
            "services/billing/application-local.yaml",
            "shared/logging.xml",
        )

        assertEquals(listOf(remoteFiles[0]), filterRemoteFiles(remoteFiles, "RVIS-AGENT"))
        assertEquals(listOf(remoteFiles[1]), filterRemoteFiles(remoteFiles, "billing\\application"))
        assertEquals(remoteFiles, filterRemoteFiles(remoteFiles, "  "))
    }

    @Test
    fun `remote file is placed in the selected project folder`() {
        val project = Path.of("workspace", "project").toAbsolutePath()

        assertEquals(
            "config/application-dev.yml",
            localTargetPath(project, project.resolve("config"), "services/api/application-dev.yml"),
        )
        assertEquals(
            "application-dev.yml",
            localTargetPath(project, project, "application-dev.yml"),
        )
    }

    @Test
    fun `remote file target rejects folders outside the project`() {
        val workspace = Path.of("workspace").toAbsolutePath()
        assertNull(
            localTargetPath(
                workspace.resolve("project"),
                workspace.resolve("another-project/config"),
                "application-dev.yml",
            ),
        )
    }
}

package io.github.localconfigsync.jetbrains.actions

import io.github.localconfigsync.jetbrains.cli.GitHubRepository
import kotlin.test.Test
import kotlin.test.assertEquals
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
}

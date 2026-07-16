package io.github.localconfigsync.jetbrains.actions

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class SetupActionTest {
    @Test
    fun `github repository names become valid stable repository ids`() {
        assertEquals("github-li-daqian-private-configs", githubRepositoryId("Li-Daqian/private-configs"))
        assertTrue(githubRepositoryId("Owner/Repo With Spaces").length <= 63)
    }
}

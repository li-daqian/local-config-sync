package io.github.localconfigsync.jetbrains.toolwindow

import io.github.localconfigsync.jetbrains.cli.FileStatusSummary
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class LocalConfigToolWindowPresentationTest {
    @Test
    fun displaysOnlyTheFileNameForUnixAndWindowsPaths() {
        assertEquals("application-dev.yml", displayFileName("src/main/resources/application-dev.yml"))
        assertEquals("application-dev.yml", displayFileName("src\\main\\resources\\application-dev.yml"))
        assertEquals("config", displayFileName("repository/project/config/"))
    }

    @Test
    fun syncConfirmationMakesBothDirectionsExplicit() {
        val confirmation = syncConfirmation(
            listOf(
                file("src/main/resources/local.yml", "local_changes"),
                file("src/main/resources/remote.yml", "remote_changes"),
                file("src/main/resources/synced.yml", "synced"),
            ),
        )

        assertTrue(confirmation.contains("Upload Local → Repository (1)"))
        assertTrue(confirmation.contains("Download Repository → Local (1)"))
        assertTrue(confirmation.contains("local.yml"))
        assertTrue(confirmation.contains("remote.yml"))
        assertFalse(confirmation.contains("synced.yml"))
    }

    private fun file(localPath: String, status: String) = FileStatusSummary(
        localPath = localPath,
        remotePath = "repository/$localPath",
        status = status,
    )
}

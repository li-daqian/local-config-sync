package io.github.localconfigsync.jetbrains.cli

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith

class CliCommandResolverTest {
    @Test
    fun `maps all bundled operating system and architecture targets`() {
        val cases = listOf(
            Triple("Linux", "amd64", "/cli/linux-amd64/local-config"),
            Triple("Linux", "aarch64", "/cli/linux-arm64/local-config"),
            Triple("Mac OS X", "x86_64", "/cli/darwin-amd64/local-config"),
            Triple("Darwin", "arm64", "/cli/darwin-arm64/local-config"),
            Triple("Windows 11", "amd64", "/cli/windows-amd64/local-config.exe"),
            Triple("Windows 11", "aarch64", "/cli/windows-arm64/local-config.exe"),
        )

        cases.forEach { (os, arch, resource) ->
            assertEquals(resource, CliCommandResolver.resolvePlatform(os, arch).resourcePath)
        }
    }

    @Test
    fun `rejects unsupported platforms explicitly`() {
        assertEquals(
            "unsupported_platform",
            assertFailsWith<CliException> { CliCommandResolver.resolvePlatform("FreeBSD", "amd64") }.code,
        )
        assertEquals(
            "unsupported_platform",
            assertFailsWith<CliException> { CliCommandResolver.resolvePlatform("Linux", "riscv64") }.code,
        )
    }
}

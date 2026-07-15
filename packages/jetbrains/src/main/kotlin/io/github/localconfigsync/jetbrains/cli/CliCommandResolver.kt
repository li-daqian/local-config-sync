package io.github.localconfigsync.jetbrains.cli

import com.intellij.openapi.application.PathManager
import io.github.localconfigsync.jetbrains.settings.LocalConfigSettings
import java.nio.file.Files
import java.nio.file.Path
import java.nio.file.StandardCopyOption
import java.security.MessageDigest

internal data class CliInvocation(
    val executable: String,
)

internal data class CliPlatform(val os: String, val arch: String, val executableName: String) {
    val resourcePath: String = "/cli/$os-$arch/$executableName"
}

internal object CliCommandResolver {
    fun resolve(): CliInvocation {
        val settings = LocalConfigSettings.getInstance()
        if (settings.cliPath.isNotBlank()) return CliInvocation(settings.cliPath)
        return CliInvocation(extractBundledCli(resolvePlatform()).toString())
    }

    internal fun resolvePlatform(
        osName: String = System.getProperty("os.name"),
        osArch: String = System.getProperty("os.arch"),
    ): CliPlatform {
        val os = when {
            osName.startsWith("Windows", ignoreCase = true) -> "windows"
            osName.startsWith("Mac", ignoreCase = true) || osName.startsWith("Darwin", ignoreCase = true) -> "darwin"
            osName.startsWith("Linux", ignoreCase = true) -> "linux"
            else -> throw CliException("unsupported_platform", "The bundled CLI does not support operating system: $osName")
        }
        val arch = when (osArch.lowercase()) {
            "amd64", "x86_64", "x64" -> "amd64"
            "arm64", "aarch64" -> "arm64"
            else -> throw CliException("unsupported_platform", "The bundled CLI does not support architecture: $osArch")
        }
        return CliPlatform(os, arch, if (os == "windows") "local-config.exe" else "local-config")
    }

    private fun extractBundledCli(platform: CliPlatform): Path {
        val bytes = CliCommandResolver::class.java.getResourceAsStream(platform.resourcePath)?.use { it.readBytes() }
            ?: throw CliException(
                "bundled_cli_missing",
                "The plugin package does not contain the Local Config Sync CLI for ${platform.os}-${platform.arch}.",
            )
        val digest = MessageDigest.getInstance("SHA-256").digest(bytes).take(8).joinToString("") { "%02x".format(it) }
        val directory = Path.of(PathManager.getSystemPath(), "local-config-sync", "cli", "${platform.os}-${platform.arch}")
        val suffix = if (platform.os == "windows") ".exe" else ""
        val target = directory.resolve("local-config-$digest$suffix")
        if (Files.isRegularFile(target)) return target

        try {
            Files.createDirectories(directory)
            val temporary = Files.createTempFile(directory, "local-config-", ".tmp")
            Files.write(temporary, bytes)
            runCatching { Files.move(temporary, target, StandardCopyOption.ATOMIC_MOVE) }
                .recoverCatching { Files.move(temporary, target, StandardCopyOption.REPLACE_EXISTING) }
                .getOrThrow()
            if (platform.os != "windows" && !target.toFile().setExecutable(true, true)) {
                throw IllegalStateException("Cannot mark the bundled CLI as executable: $target")
            }
            return target
        } catch (error: Exception) {
            throw CliException("bundled_cli_extract_failed", "Cannot prepare the bundled Local Config Sync CLI.", error.message.orEmpty())
        }
    }
}

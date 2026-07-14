package io.github.localconfigsync.jetbrains.cli

import com.intellij.openapi.application.PathManager
import io.github.localconfigsync.jetbrains.settings.LocalConfigSettings
import java.io.File
import java.nio.file.Files
import java.nio.file.Path
import java.nio.file.StandardCopyOption
import java.security.MessageDigest
import java.util.concurrent.ConcurrentHashMap

internal data class CliInvocation(
    val executable: String,
    val prefixArguments: List<String> = emptyList(),
)

internal object CliCommandResolver {
    private val validatedNodes = ConcurrentHashMap.newKeySet<String>()

    fun resolve(): CliInvocation {
        val settings = LocalConfigSettings.getInstance()
        if (settings.cliPath.isNotBlank()) return CliInvocation(settings.cliPath)

        val node = settings.nodePath.takeIf(String::isNotBlank) ?: findNodeExecutable()
        ?: throw CliException(
            "node_unavailable",
            "The bundled Local Config Sync CLI requires Node.js 20 or newer.",
            "Install Node.js, or configure its executable under Settings | Tools | Local Config Sync.",
        )
        validateNode(node)
        return CliInvocation(node, listOf(extractBundledCli().toString()))
    }

    private fun extractBundledCli(): Path {
        val bytes = CliCommandResolver::class.java.getResourceAsStream("/cli/local-config.mjs")?.use { it.readBytes() }
            ?: throw CliException("bundled_cli_missing", "The plugin package does not contain the bundled CLI.")
        val digest = MessageDigest.getInstance("SHA-256").digest(bytes).take(8).joinToString("") { "%02x".format(it) }
        val directory = Path.of(PathManager.getSystemPath(), "local-config-sync", "cli")
        val target = directory.resolve("local-config-$digest.mjs")
        if (Files.isRegularFile(target)) return target

        try {
            Files.createDirectories(directory)
            val temporary = Files.createTempFile(directory, "local-config-", ".tmp")
            Files.write(temporary, bytes)
            runCatching { Files.move(temporary, target, StandardCopyOption.ATOMIC_MOVE) }
                .recoverCatching { Files.move(temporary, target, StandardCopyOption.REPLACE_EXISTING) }
                .getOrThrow()
            target.toFile().setExecutable(true, true)
            return target
        } catch (error: Exception) {
            throw CliException("bundled_cli_extract_failed", "Cannot prepare the bundled Local Config Sync CLI.", error.message.orEmpty())
        }
    }

    private fun findNodeExecutable(): String? {
        val executableName = if (System.getProperty("os.name").startsWith("Windows", ignoreCase = true)) "node.exe" else "node"
        val candidates = linkedSetOf<Path>()
        System.getenv("PATH").orEmpty().split(File.pathSeparatorChar).filter(String::isNotBlank)
            .mapTo(candidates) { Path.of(it).resolve(executableName) }

        val home = Path.of(System.getProperty("user.home"))
        listOf(
            home.resolve(".volta/bin/$executableName"),
            home.resolve(".asdf/shims/$executableName"),
            home.resolve(".local/share/mise/shims/$executableName"),
            Path.of("/opt/homebrew/bin/$executableName"),
            Path.of("/usr/local/bin/$executableName"),
            Path.of("/usr/bin/$executableName"),
        ).forEach(candidates::add)

        val nvmVersions = home.resolve(".nvm/versions/node")
        if (Files.isDirectory(nvmVersions)) {
            runCatching {
                Files.newDirectoryStream(nvmVersions).use { directories ->
                    directories.filter(Files::isDirectory)
                        .sortedByDescending { it.fileName.toString().removePrefix("v").substringBefore('.').toIntOrNull() ?: -1 }
                        .mapTo(candidates) { it.resolve("bin/$executableName") }
                }
            }
        }
        return candidates.firstOrNull { Files.isRegularFile(it) && (System.getProperty("os.name").startsWith("Windows", true) || Files.isExecutable(it)) }?.toString()
    }

    private fun validateNode(executable: String) {
        if (validatedNodes.contains(executable)) return
        try {
            val process = ProcessBuilder(executable, "--version").redirectErrorStream(true).start()
            val version = process.inputStream.bufferedReader().readText().trim()
            val exitCode = process.waitFor()
            val major = version.removePrefix("v").substringBefore('.').toIntOrNull()
            if (exitCode != 0 || major == null || major < 20) {
                throw CliException("node_incompatible", "Node.js 20 or newer is required; detected ${version.ifBlank { "an unknown version" }}.")
            }
            validatedNodes += executable
        } catch (error: CliException) {
            throw error
        } catch (error: Exception) {
            throw CliException("node_unavailable", "Cannot start Node.js for the bundled CLI.", error.message.orEmpty())
        }
    }
}

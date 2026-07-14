import org.jetbrains.intellij.platform.gradle.IntelliJPlatformType
import org.jetbrains.intellij.platform.gradle.tasks.VerifyPluginTask
import org.jetbrains.kotlin.gradle.dsl.JvmDefaultMode
import org.gradle.api.tasks.Exec

plugins {
    kotlin("jvm") version "2.3.20"
    id("org.jetbrains.intellij.platform")
}

group = "io.github.localconfigsync"
version = "0.1.2"

val localIdeaPath = providers.gradleProperty("localIdeaPath").orNull
val localVerifierIdePath = providers.gradleProperty("localVerifierIdePath").orNull
val allowIdeSdkDownload = providers.gradleProperty("allowIdeSdkDownload")
    .map { it.toBooleanStrict() }
    .orElse(false)

dependencies {
    intellijPlatform {
        if (localIdeaPath != null) {
            local(localIdeaPath)
        } else if (allowIdeSdkDownload.get()) {
            intellijIdea("2026.1.4")
        } else {
            throw GradleException(
                "IntelliJ SDK auto-download is disabled. Provide -PlocalIdeaPath=/path/to/idea, " +
                    "or explicitly opt in with -PallowIdeSdkDownload=true.",
            )
        }
    }
}

kotlin {
    jvmToolchain(21)
    compilerOptions {
        // Compatibility bridges can reintroduce deprecated IntelliJ interface methods into the plugin bytecode.
        jvmDefault.set(JvmDefaultMode.NO_COMPATIBILITY)
    }
}

intellijPlatform {
    pluginConfiguration {
        id = "io.github.localconfigsync.jetbrains"
        name = "Local Config Sync"
        version = project.version.toString()
        description = """
            <p>
              Keep project-local overlay configuration inside your project for convenient editing,
              while excluding it from the project's Git history and synchronizing it through a
              user-configured Git or local-folder repository.
            </p>
            <p>The plugin provides:</p>
            <ul>
              <li>project setup and repository mapping;</li>
              <li>safe manual synchronization with conflict detection;</li>
              <li>Git authentication checks; and</li>
              <li>sync status in the IDE status bar.</li>
            </ul>
            <p>
              The plugin includes a compatible CLI bundle and detects common Node.js 20+
              installations automatically. Advanced executable overrides are available under
              <em>Settings | Tools | Local Config Sync</em>. The plugin does not store Git credentials
              or synchronize detected secret files by default.
            </p>
        """.trimIndent()
        changeNotes = """
            <p>Integrated tool window and bundled CLI release.</p>
            <ul>
              <li>Add a right-side project dashboard with setup, sync, authentication, and diagnostics.</li>
              <li>Open the dashboard from the status bar and preserve actionable CLI errors.</li>
              <li>Bundle the CLI and automatically detect Node.js 20+.</li>
              <li>Place advanced executable overrides under Settings | Tools.</li>
            </ul>
        """.trimIndent()
        ideaVersion {
            sinceBuild = "261"
        }
        vendor {
            name = "Li DaQian"
            email = "hi@lidaqian.me"
            url = "https://github.com/li-daqian/local-config-sync"
        }
    }
    pluginVerification {
        ides {
            val verifierIdePath = localVerifierIdePath ?: localIdeaPath
            if (verifierIdePath != null) {
                local(verifierIdePath)
            } else if (allowIdeSdkDownload.get()) {
                create(IntelliJPlatformType.IntellijIdea, "2026.1.4")
                create(IntelliJPlatformType.IntellijIdea, "262.8665.176")
            }
        }
        failureLevel.set(VerifyPluginTask.FailureLevel.ALL)
        verificationReportsFormats.set(VerifyPluginTask.VerificationReportsFormats.ALL)
    }
    buildSearchableOptions = false
    instrumentCode = false
}

tasks {
    val bundleCli by registering(Exec::class) {
        val repositoryRoot = projectDir.resolve("../..").canonicalFile
        workingDir(repositoryRoot)
        commandLine("pnpm", "bundle:cli")
        inputs.files(
            repositoryRoot.resolve("package.json"),
            repositoryRoot.resolve("pnpm-lock.yaml"),
            repositoryRoot.resolve("scripts/bundle-cli.mjs"),
            fileTree(repositoryRoot.resolve("packages/core/src")),
            fileTree(repositoryRoot.resolve("packages/cli/src")),
        )
        outputs.file(layout.buildDirectory.file("generated-resources/cli/local-config.mjs"))
    }

    processResources {
        dependsOn(bundleCli)
        from(layout.buildDirectory.file("generated-resources/cli/local-config.mjs")) {
            into("cli")
        }
    }

    wrapper {
        gradleVersion = "9.0.0"
        distributionType = Wrapper.DistributionType.BIN
    }
}

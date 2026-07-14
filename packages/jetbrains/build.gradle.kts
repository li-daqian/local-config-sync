import org.jetbrains.intellij.platform.gradle.IntelliJPlatformType
import org.jetbrains.intellij.platform.gradle.tasks.VerifyPluginTask
import org.jetbrains.kotlin.gradle.dsl.JvmDefaultMode

plugins {
    kotlin("jvm") version "2.3.20"
    id("org.jetbrains.intellij.platform")
}

group = "io.github.localconfigsync"
version = "0.1.1"

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
              The <code>local-config</code> CLI is required and its executable path can be configured
              under <em>Settings | Tools | Local Config Sync</em>. The plugin does not store Git
              credentials or synchronize detected secret files by default.
            </p>
        """.trimIndent()
        changeNotes = """
            <p>Compatibility maintenance release.</p>
            <ul>
              <li>Migrate deprecated dialog, file chooser, and status bar APIs.</li>
              <li>Preserve setup, authentication, sync, and status behavior.</li>
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
    wrapper {
        gradleVersion = "9.0.0"
        distributionType = Wrapper.DistributionType.BIN
    }
}

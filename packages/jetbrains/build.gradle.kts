import org.jetbrains.intellij.platform.gradle.IntelliJPlatformType
import org.jetbrains.intellij.platform.gradle.tasks.VerifyPluginTask
import org.jetbrains.kotlin.gradle.dsl.JvmDefaultMode
import org.gradle.api.tasks.Exec

plugins {
    kotlin("jvm") version "2.3.20"
    id("org.jetbrains.intellij.platform")
}

group = "io.github.localconfigsync"
version = "0.1.5"

val localIdeaPath = providers.gradleProperty("localIdeaPath").orNull
val localVerifierIdePath = providers.gradleProperty("localVerifierIdePath").orNull
val allowIdeSdkDownload = providers.gradleProperty("allowIdeSdkDownload")
    .map { it.toBooleanStrict() }
    .orElse(false)
val goExecutable = providers.gradleProperty("goExecutable")
    .orElse(providers.environmentVariable("GO_EXECUTABLE"))
    .orElse("go")

dependencies {
    testImplementation(kotlin("test"))
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
              The plugin includes native CLI binaries for Linux, macOS, and Windows on x64 and
              ARM64, with no Node.js runtime requirement. Advanced executable overrides are available under
              <em>Settings | Tools | Local Config Sync</em>. The plugin does not store Git credentials
              or synchronize detected secret files by default.
            </p>
        """.trimIndent()
        changeNotes = """
            <p>GitHub repository setup UX improvements.</p>
            <ul>
              <li>Add a searchable popover combobox for selecting public and private GitHub repositories.</li>
              <li>Keep the search field inside the repository popup and filter by owner or repository name.</li>
              <li>Support Linux input methods in the repository search field.</li>
              <li>Show modal progress while GitHub authentication and repository loading are running.</li>
              <li>Handle repositories with no files and nullable CLI wire payloads safely.</li>
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
    val repositoryRoot = projectDir.resolve("../..").canonicalFile
    val cliTargets = listOf(
        "linux-amd64", "linux-arm64",
        "darwin-amd64", "darwin-arm64",
        "windows-amd64", "windows-arm64",
    )
    val bundleCliTasks = cliTargets.map { target ->
        val (targetOs, targetArch) = target.split("-")
        val executableName = if (targetOs == "windows") "local-config.exe" else "local-config"
        val outputFile = layout.buildDirectory.file("generated-resources/cli/$target/$executableName").get().asFile
        register<Exec>("bundleCli${targetOs.replaceFirstChar(Char::uppercase)}${targetArch.replaceFirstChar(Char::uppercase)}") {
            workingDir(repositoryRoot)
            doFirst {
                outputFile.parentFile.mkdirs()
            }
            environment("CGO_ENABLED", "0")
            environment("GOOS", targetOs)
            environment("GOARCH", targetArch)
            environment("GOTOOLCHAIN", "local")
            environment("GOCACHE", layout.buildDirectory.dir("go-cache").get().asFile.absolutePath)
            commandLine(
                goExecutable.get(), "build", "-trimpath", "-ldflags=-s -w -buildid=",
                "-o", outputFile.absolutePath,
                "./cmd/local-config",
            )
            inputs.files(repositoryRoot.resolve("go.mod"), repositoryRoot.resolve("go.sum"))
            inputs.files(fileTree(repositoryRoot.resolve("cmd")), fileTree(repositoryRoot.resolve("internal")))
            outputs.file(outputFile)
        }
    }
    val bundleCli by registering {
        dependsOn(bundleCliTasks)
    }

    processResources {
        dependsOn(bundleCli)
        from(layout.buildDirectory.dir("generated-resources/cli")) {
            into("cli")
            cliTargets.forEach { include("$it/**") }
        }
    }

    wrapper {
        gradleVersion = "9.0.0"
        distributionType = Wrapper.DistributionType.BIN
    }
}

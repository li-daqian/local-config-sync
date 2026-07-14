plugins {
    kotlin("jvm") version "2.3.20"
    id("org.jetbrains.intellij.platform")
}

group = "io.github.localconfigsync"
version = "0.1.0"

dependencies {
    intellijPlatform {
        val localIdeaPath = providers.gradleProperty("localIdeaPath").orNull
        if (localIdeaPath != null) {
            local(localIdeaPath)
        } else {
            intellijIdea("2026.1.4")
        }
    }
}

kotlin {
    jvmToolchain(21)
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
            <p>Initial release.</p>
            <ul>
              <li>Set up project-to-repository mappings.</li>
              <li>Run safe manual synchronization from the IDE.</li>
              <li>Verify Git authentication and view synchronization status.</li>
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
    buildSearchableOptions = false
    instrumentCode = false
}

tasks {
    wrapper {
        gradleVersion = "9.0.0"
        distributionType = Wrapper.DistributionType.BIN
    }
}

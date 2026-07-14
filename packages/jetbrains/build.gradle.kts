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
            intellijIdeaCommunity("2026.1.4")
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
        description = "Synchronize project-local overlay configuration through the Local Config Sync CLI."
        ideaVersion {
            sinceBuild = "261"
        }
        vendor {
            name = "Local Config Sync"
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

package io.github.localconfigsync.jetbrains.status

import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.components.Service
import com.intellij.openapi.project.Project
import com.intellij.util.messages.Topic
import io.github.localconfigsync.jetbrains.cli.CliException
import io.github.localconfigsync.jetbrains.cli.LocalConfigCli
import io.github.localconfigsync.jetbrains.cli.StatusResponse
import java.util.concurrent.atomic.AtomicBoolean

sealed interface LocalConfigSnapshot {
    data object Loading : LocalConfigSnapshot
    data class Ready(val response: StatusResponse) : LocalConfigSnapshot
    data class Failure(val error: CliException) : LocalConfigSnapshot
}

fun interface LocalConfigStatusListener {
    fun statusChanged(snapshot: LocalConfigSnapshot)
}

@Service(Service.Level.PROJECT)
class LocalConfigStatusService(private val project: Project) {
    @Volatile
    var snapshot: LocalConfigSnapshot = LocalConfigSnapshot.Loading
        private set

    private val refreshing = AtomicBoolean(false)

    fun refresh() {
        if (project.isDisposed || project.basePath == null || !refreshing.compareAndSet(false, true)) return
        publish(LocalConfigSnapshot.Loading)
        ApplicationManager.getApplication().executeOnPooledThread {
            val next = try {
                LocalConfigSnapshot.Ready(LocalConfigCli.status(project))
            } catch (error: CliException) {
                LocalConfigSnapshot.Failure(error)
            } catch (error: Exception) {
                LocalConfigSnapshot.Failure(CliException("status_failed", error.message ?: "Status check failed"))
            } finally {
                refreshing.set(false)
            }
            publish(next)
        }
    }

    fun recordFailure(error: CliException) = publish(LocalConfigSnapshot.Failure(error))

    private fun publish(value: LocalConfigSnapshot) {
        ApplicationManager.getApplication().invokeLater {
            if (project.isDisposed) return@invokeLater
            snapshot = value
            project.messageBus.syncPublisher(TOPIC).statusChanged(value)
        }
    }

    companion object {
        val TOPIC: Topic<LocalConfigStatusListener> = Topic.create("Local Config Sync status", LocalConfigStatusListener::class.java)
        fun getInstance(project: Project): LocalConfigStatusService = project.getService(LocalConfigStatusService::class.java)
    }
}

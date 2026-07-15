package io.github.localconfigsync.jetbrains.settings

import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.components.PersistentStateComponent
import com.intellij.openapi.components.Service
import com.intellij.openapi.components.State
import com.intellij.openapi.components.Storage

@Service(Service.Level.APP)
@State(name = "LocalConfigSyncSettings", storages = [Storage("localConfigSync.xml")])
class LocalConfigSettings : PersistentStateComponent<LocalConfigSettings.State> {
    data class State(
        var cliPath: String = "",
    )

    private var state = State()

    override fun getState(): State = state
    override fun loadState(state: State) {
        // 0.1.1 used "local-config" as a mandatory PATH lookup. Treat that default as
        // automatic mode so upgrades immediately benefit from the bundled CLI.
        if (state.cliPath == "local-config") state.cliPath = ""
        this.state = state
    }

    var cliPath: String
        get() = state.cliPath
        set(value) { state.cliPath = value.trim() }

    companion object {
        fun getInstance(): LocalConfigSettings = ApplicationManager.getApplication().getService(LocalConfigSettings::class.java)
    }
}

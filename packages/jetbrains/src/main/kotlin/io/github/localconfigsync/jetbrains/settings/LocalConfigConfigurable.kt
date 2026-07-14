package io.github.localconfigsync.jetbrains.settings

import com.intellij.openapi.options.Configurable
import com.intellij.openapi.ui.TextFieldWithBrowseButton
import com.intellij.ui.components.JBLabel
import com.intellij.util.ui.FormBuilder
import javax.swing.JComponent
import javax.swing.JPanel

class LocalConfigConfigurable : Configurable {
    private var panel: JPanel? = null
    private val cliPath = TextFieldWithBrowseButton()

    override fun getDisplayName(): String = "Local Config Sync"

    override fun createComponent(): JComponent {
        cliPath.text = LocalConfigSettings.getInstance().cliPath
        cliPath.addBrowseFolderListener(
            "Select Local Config Sync CLI",
            "Select the local-config executable. IDE processes may not inherit your shell PATH.",
            null,
            com.intellij.openapi.fileChooser.FileChooserDescriptorFactory.createSingleFileDescriptor()
        )
        panel = FormBuilder.createFormBuilder()
            .addLabeledComponent(JBLabel("CLI executable:"), cliPath, 1, false)
            .addComponent(JBLabel("Install the CLI first, then set its absolute path here if it is not on the IDE PATH."))
            .addComponentFillVertically(JPanel(), 0)
            .panel
        return panel!!
    }

    override fun isModified(): Boolean = cliPath.text.trim() != LocalConfigSettings.getInstance().cliPath
    override fun apply() { LocalConfigSettings.getInstance().cliPath = cliPath.text }
    override fun reset() { cliPath.text = LocalConfigSettings.getInstance().cliPath }
    override fun disposeUIResources() { panel = null }
}

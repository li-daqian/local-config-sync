package io.github.localconfigsync.jetbrains.settings

import com.intellij.openapi.options.Configurable
import com.intellij.openapi.fileChooser.FileChooserDescriptorFactory
import com.intellij.openapi.ui.TextFieldWithBrowseButton
import com.intellij.ui.components.JBLabel
import com.intellij.util.ui.FormBuilder
import javax.swing.JComponent
import javax.swing.JPanel

class LocalConfigConfigurable : Configurable {
    private var panel: JPanel? = null
    private val cliPath = TextFieldWithBrowseButton()
    private val nodePath = TextFieldWithBrowseButton()

    override fun getDisplayName(): String = "Local Config Sync"

    override fun createComponent(): JComponent {
        cliPath.text = LocalConfigSettings.getInstance().cliPath
        nodePath.text = LocalConfigSettings.getInstance().nodePath
        val cliDescriptor = FileChooserDescriptorFactory.singleFile()
            .withTitle("Select Custom Local Config Sync CLI")
            .withDescription("Optional override for the CLI bundled with the plugin.")
        cliPath.addBrowseFolderListener(null, cliDescriptor)
        val nodeDescriptor = FileChooserDescriptorFactory.singleFile()
            .withTitle("Select Node.js Executable")
            .withDescription("Optional override used to run the bundled CLI.")
        nodePath.addBrowseFolderListener(null, nodeDescriptor)
        panel = FormBuilder.createFormBuilder()
            .addComponent(JBLabel("The plugin uses its bundled CLI by default. These fields are only needed for advanced overrides."))
            .addLabeledComponent(JBLabel("Custom CLI executable:"), cliPath, 1, false)
            .addLabeledComponent(JBLabel("Node.js executable:"), nodePath, 1, false)
            .addComponent(JBLabel("Node.js 20 or newer is required. Common system, nvm, Volta, asdf, and mise locations are detected automatically."))
            .addComponentFillVertically(JPanel(), 0)
            .panel
        return panel!!
    }

    override fun isModified(): Boolean {
        val settings = LocalConfigSettings.getInstance()
        return cliPath.text.trim() != settings.cliPath || nodePath.text.trim() != settings.nodePath
    }

    override fun apply() {
        LocalConfigSettings.getInstance().apply {
            cliPath = this@LocalConfigConfigurable.cliPath.text
            nodePath = this@LocalConfigConfigurable.nodePath.text
        }
    }

    override fun reset() {
        LocalConfigSettings.getInstance().let {
            cliPath.text = it.cliPath
            nodePath.text = it.nodePath
        }
    }
    override fun disposeUIResources() { panel = null }
}

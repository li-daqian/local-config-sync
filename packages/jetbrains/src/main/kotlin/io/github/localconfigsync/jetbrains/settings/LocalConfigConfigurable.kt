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

    override fun getDisplayName(): String = "Local Config Sync"

    override fun createComponent(): JComponent {
        cliPath.text = LocalConfigSettings.getInstance().cliPath
        val cliDescriptor = FileChooserDescriptorFactory.singleFile()
            .withTitle("Select Custom Local Config Sync CLI")
            .withDescription("Optional override for the CLI bundled with the plugin.")
        cliPath.addBrowseFolderListener(null, cliDescriptor)
        panel = FormBuilder.createFormBuilder()
            .addComponent(JBLabel("The plugin uses its bundled native CLI by default. This field is only needed for advanced overrides."))
            .addLabeledComponent(JBLabel("Custom CLI executable:"), cliPath, 1, false)
            .addComponentFillVertically(JPanel(), 0)
            .panel
        return panel!!
    }

    override fun isModified(): Boolean {
        val settings = LocalConfigSettings.getInstance()
        return cliPath.text.trim() != settings.cliPath
    }

    override fun apply() {
        LocalConfigSettings.getInstance().apply {
            cliPath = this@LocalConfigConfigurable.cliPath.text
        }
    }

    override fun reset() {
        LocalConfigSettings.getInstance().let {
            cliPath.text = it.cliPath
        }
    }
    override fun disposeUIResources() { panel = null }
}

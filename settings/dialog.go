package settings

import (
	"fmt"
	"os/user"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// ShowSettingsDialog displays the settings dialog
func ShowSettingsDialog(window fyne.Window, currentSettings *Settings, onSettingsChanged func()) {
	// Create form fields
	passwordStoreEntry := widget.NewEntry()
	passwordStoreEntry.SetText(currentSettings.PasswordStorePath)
	if passwordStoreEntry.Text == "" {
		// Set default path
		if user, err := user.Current(); err == nil {
			defaultPath := filepath.Join(user.HomeDir, ".password-store")
			passwordStoreEntry.SetText(defaultPath)
		}
	}

	defaultRecipientEntry := widget.NewEntry()
	defaultRecipientEntry.SetText(currentSettings.DefaultRecipient)

	autoCommitCheck := widget.NewCheck("Auto-commit changes", func(checked bool) {
		currentSettings.AutoCommit = checked
	})
	autoCommitCheck.SetChecked(currentSettings.AutoCommit)

	notificationsCheck := widget.NewCheck("Show notifications", func(checked bool) {
		currentSettings.ShowNotifications = checked
	})
	notificationsCheck.SetChecked(currentSettings.ShowNotifications)

	themeSelect := widget.NewSelect(GetAvailableThemes(), func(theme string) {
		currentSettings.Theme = theme
		// Apply theme immediately
		ApplyTheme(fyne.CurrentApp(), theme)
		// Refresh UI if callback provided
		if onSettingsChanged != nil {
			onSettingsChanged()
		}
	})
	themeSelect.SetSelected(currentSettings.Theme)

	// Create form
	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Password Store Path", Widget: passwordStoreEntry, HintText: "Path to your password store directory"},
			{Text: "Default Recipient", Widget: defaultRecipientEntry, HintText: "Default GPG recipient for new files"},
			{Text: "Auto-commit", Widget: autoCommitCheck, HintText: "Automatically commit changes when saving"},
			{Text: "Notifications", Widget: notificationsCheck, HintText: "Show system notifications"},
			{Text: "Theme", Widget: themeSelect, HintText: "Application theme (applied immediately)"},
		},
		OnSubmit: func() {
			// Update settings
			updates := map[string]interface{}{
				"password_store_path": passwordStoreEntry.Text,
				"default_recipient":   defaultRecipientEntry.Text,
				"auto_commit":         autoCommitCheck.Checked,
				"show_notifications":  notificationsCheck.Checked,
				"theme":               themeSelect.Selected,
			}

			if err := UpdateSettings(updates); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to save settings: %v", err), window)
				return
			}

			// Update current settings
			currentSettings.PasswordStorePath = passwordStoreEntry.Text
			currentSettings.DefaultRecipient = defaultRecipientEntry.Text
			currentSettings.AutoCommit = autoCommitCheck.Checked
			currentSettings.ShowNotifications = notificationsCheck.Checked
			currentSettings.Theme = themeSelect.Selected

			// Refresh UI if callback provided
			if onSettingsChanged != nil {
				onSettingsChanged()
			}

			dialog.ShowInformation("Settings Saved", "Settings have been saved successfully.", window)
		},
		OnCancel: func() {
			// Reset form to current settings
			passwordStoreEntry.SetText(currentSettings.PasswordStorePath)
			defaultRecipientEntry.SetText(currentSettings.DefaultRecipient)
			autoCommitCheck.SetChecked(currentSettings.AutoCommit)
			notificationsCheck.SetChecked(currentSettings.ShowNotifications)
			themeSelect.SetSelected(currentSettings.Theme)
		},
	}

	// Create dialog
	settingsDialog := dialog.NewCustom("Settings", "Save Changes", form, window)
	settingsDialog.Resize(fyne.NewSize(500, 400))
	settingsDialog.Show()
}

// ShowAboutDialog displays the about dialog
func ShowAboutDialog(window fyne.Window) {
	aboutContent := widget.NewRichTextFromMarkdown(`
# GPG Password Store Viewer

A modern GUI application for browsing and managing password-store entries.

## Features
- Browse password store structure
- View and edit encrypted files
- Smart GPG passphrase handling
- Git synchronization
- Configurable settings

## Version
1.0.0

## Author
Oleg Zubak <c4point@gmail.com>

## License
MIT License
`)

	aboutDialog := dialog.NewCustom("About", "Close", aboutContent, window)
	aboutDialog.Resize(fyne.NewSize(400, 300))
	aboutDialog.Show()
}

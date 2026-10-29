package settings

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// DefaultStorePath returns the path pass(1) uses when nothing is configured.
func DefaultStorePath() string {
	if u, err := user.Current(); err == nil && u.HomeDir != "" {
		return filepath.Join(u.HomeDir, ".password-store")
	}
	return ".password-store"
}

// ShowSettingsDialog shows the settings form. Nothing is persisted or
// applied until "Save changes" is pressed; onSaved then receives the new
// settings so the caller can react (e.g. rescan a changed store path).
func ShowSettingsDialog(window fyne.Window, current *Settings, onSaved func(updated *Settings)) {
	storePath := current.PasswordStorePath
	if storePath == "" {
		storePath = DefaultStorePath()
	}
	storeEntry := widget.NewEntry()
	storeEntry.SetText(storePath)
	storeEntry.Validator = func(s string) error {
		s = strings.TrimSpace(s)
		if s == "" {
			return errors.New("enter the folder of your password store")
		}
		info, err := os.Stat(expandHome(s))
		if err != nil || !info.IsDir() {
			return errors.New("folder does not exist")
		}
		return nil
	}

	recipientEntry := widget.NewEntry()
	recipientEntry.SetText(current.DefaultRecipient)
	recipientEntry.SetPlaceHolder("Optional: email or key ID")
	recipientEntry.Validator = func(s string) error {
		if !IsValidRecipient(s) {
			return errors.New("use an email address or a hex key ID")
		}
		return nil
	}

	autoCommitCheck := widget.NewCheck("Commit every change to git", nil)
	autoCommitCheck.SetChecked(current.AutoCommit)

	notificationsCheck := widget.NewCheck("Show system notifications", nil)
	notificationsCheck.SetChecked(current.ShowNotifications)

	themeNames := GetAvailableThemes()
	themeLabels := make([]string, len(themeNames))
	for i, name := range themeNames {
		themeLabels[i] = GetThemeDisplayName(name)
	}
	themeSelect := widget.NewSelect(themeLabels, nil)
	themeSelect.SetSelected(GetThemeDisplayName(current.Theme))

	items := []*widget.FormItem{
		{Text: "Password store", Widget: storeEntry, HintText: "Folder with your .gpg files, usually ~/.password-store"},
		{Text: "Default recipient", Widget: recipientEntry, HintText: "Used when a store has no .gpg-id file"},
		{Text: "Git", Widget: autoCommitCheck, HintText: "Like pass: each save, rename or delete becomes a commit"},
		{Text: "Notifications", Widget: notificationsCheck},
		{Text: "Theme", Widget: themeSelect},
	}

	form := dialog.NewForm("Settings", "Save changes", "Cancel", items, func(save bool) {
		if !save {
			return
		}
		updated := *current
		updated.PasswordStorePath = expandHome(strings.TrimSpace(storeEntry.Text))
		updated.DefaultRecipient = strings.TrimSpace(recipientEntry.Text)
		updated.AutoCommit = autoCommitCheck.Checked
		updated.ShowNotifications = notificationsCheck.Checked
		updated.Theme = themeNames[0]
		for i, label := range themeLabels {
			if label == themeSelect.Selected {
				updated.Theme = themeNames[i]
			}
		}

		if err := UpdateSettings(map[string]interface{}{
			"password_store_path": updated.PasswordStorePath,
			"default_recipient":   updated.DefaultRecipient,
			"auto_commit":         updated.AutoCommit,
			"show_notifications":  updated.ShowNotifications,
			"theme":               updated.Theme,
		}); err != nil {
			dialog.ShowError(fmt.Errorf("settings were not saved: %v", err), window)
			return
		}

		*current = updated
		ApplyTheme(fyne.CurrentApp(), updated.Theme)
		if onSaved != nil {
			onSaved(current)
		}
	}, window)
	form.Resize(fyne.NewSize(560, 0))
	form.Show()
}

// expandHome replaces a leading "~" with the user's home directory so the
// path can be typed the way people are used to from the shell.
func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[1:])
		}
	}
	return p
}

// ShowAboutDialog displays the about dialog
func ShowAboutDialog(window fyne.Window) {
	aboutContent := widget.NewRichTextFromMarkdown(`
# GPG Password Store Viewer

A desktop app for browsing and editing a [pass](https://www.passwordstore.org/)
password store: records stay GPG-encrypted on disk and in git.

Version 1.0.0 · MIT License

Oleg Zubak <c4point@gmail.com>
`)
	aboutContent.Wrapping = fyne.TextWrapWord

	aboutDialog := dialog.NewCustom("About", "Close", aboutContent, window)
	aboutDialog.Resize(fyne.NewSize(420, 0))
	aboutDialog.Show()
}

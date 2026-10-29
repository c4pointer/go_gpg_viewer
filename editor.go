package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go_gpg_viewer/internal/clipboard"
	"go_gpg_viewer/internal/gpg"
	"go_gpg_viewer/internal/password"
	"go_gpg_viewer/internal/storepath"
)

// generatedPasswordLength is the length of passwords the Generate buttons
// produce: 20 characters from password.DefaultCharset is about 129 bits.
const generatedPasswordLength = 20

// openingRecord is set while a record is being decrypted so that a quick
// double click does not stack two editor dialogs on top of each other. Once
// a dialog is on screen it is modal and blocks further clicks by itself.
var openingRecord atomic.Bool

// openRecord decrypts the record rel (relative to the store root, without
// the .gpg extension) in the background and opens it in the editor.
func (u *ui) openRecord(rel string) {
	if !openingRecord.CompareAndSwap(false, true) {
		return
	}
	storeRoot := u.storePath
	filePath := filepath.Join(storeRoot, rel+".gpg")

	var decrypt func(passphrase string)
	decrypt = func(passphrase string) {
		plaintext, gpgStderr, err := gpg.Decrypt(filePath, passphrase)
		if err != nil {
			if passphrase == "" {
				// First attempt relied on gpg-agent: ask for the passphrase.
				fyne.Do(func() {
					defer openingRecord.Store(false)
					u.askPassphrase(rel, func(p string) {
						openingRecord.Store(true)
						go decrypt(p)
					})
				})
				return
			}
			// Only stderr is surfaced: stdout may contain partial plaintext
			// and must never reach the UI.
			fyne.Do(func() {
				defer openingRecord.Store(false)
				dialog.ShowError(fmt.Errorf("could not decrypt %s: %v\n%s", rel, err, gpgStderr), u.window)
			})
			return
		}
		// Widgets may only be built and shown on the Fyne thread.
		fyne.Do(func() {
			defer openingRecord.Store(false)
			u.showRecordEditor(storeRoot, rel, string(plaintext))
		})
	}
	go decrypt("")
}

// askPassphrase prompts for the GPG passphrase of rel and hands it to
// onEntered. Must run on the Fyne thread.
func (u *ui) askPassphrase(rel string, onEntered func(passphrase string)) {
	entry := widget.NewPasswordEntry()
	entry.Validator = func(s string) error {
		if s == "" {
			return errors.New("enter the passphrase")
		}
		return nil
	}
	items := []*widget.FormItem{
		{Text: "Passphrase", Widget: entry, HintText: "gpg-agent could not unlock the key for " + rel},
	}
	form := dialog.NewForm("Unlock key", "Decrypt", "Cancel", items, func(ok bool) {
		if ok {
			onEntered(entry.Text)
		}
	}, u.window)
	form.Resize(fyne.NewSize(460, 0))
	form.Show()
	u.window.Canvas().Focus(entry)
}

// showRecordEditor shows the dialog to edit a decrypted record: its name
// (renaming moves the file inside the store), password and notes. Must run
// on the Fyne thread.
func (u *ui) showRecordEditor(storeRoot, recordName, plaintext string) {
	oldPath, err := storepath.ResolveRecord(storeRoot, recordName)
	if err != nil {
		dialog.ShowError(err, u.window)
		return
	}

	// pass(1) convention: first line is the password, the rest is free-form
	// metadata (username, URL, notes, ...).
	pwPart, metaPart := splitPasswordAndMetadata(plaintext)
	originalContent := joinPasswordAndMetadata(pwPart, metaPart)

	nameEntry := widget.NewEntry()
	nameEntry.SetText(recordName)
	nameEntry.Validator = recordNameValidator(storeRoot)

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetText(pwPart)

	notesEntry := widget.NewMultiLineEntry()
	notesEntry.SetText(metaPart)
	notesEntry.SetPlaceHolder("user: alice\nurl: https://example.com")
	notesEntry.Wrapping = fyne.TextWrapOff
	notesEntry.SetMinRowsVisible(6)

	copyBtn := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() {
		clipboard.CopyWithAutoClear(u.app.Clipboard(), passwordEntry.Text, clipboardTTL)
		u.setStatus(fmt.Sprintf("Password copied, clipboard clears in %s", clipboardTTL))
	})
	generateBtn := widget.NewButtonWithIcon("Generate", theme.ViewRefreshIcon(), func() {
		pw, err := password.Generate(generatedPasswordLength, password.DefaultCharset)
		if err != nil {
			dialog.ShowError(fmt.Errorf("could not generate a password: %v", err), u.window)
			return
		}
		passwordEntry.SetText(pw)
	})
	passwordRow := container.NewBorder(nil, nil, nil, container.NewHBox(copyBtn, generateBtn), passwordEntry)

	form := widget.NewForm(
		&widget.FormItem{Text: "Name", Widget: nameEntry, HintText: "Folder/name, e.g. Work/github"},
		&widget.FormItem{Text: "Password", Widget: passwordRow},
		&widget.FormItem{Text: "Notes", Widget: notesEntry},
	)

	var editDialog *dialog.CustomDialog

	saveBtn := widget.NewButtonWithIcon("Save changes", theme.ConfirmIcon(), nil)
	saveBtn.Importance = widget.HighImportance
	saveBtn.OnTapped = func() {
		if err := form.Validate(); err != nil {
			dialog.ShowError(err, u.window)
			return
		}
		newName := cleanRecordName(nameEntry.Text)
		newPath, err := storepath.ResolveRecord(storeRoot, newName)
		if err != nil {
			dialog.ShowError(err, u.window)
			return
		}
		renamed := newPath != oldPath
		if renamed {
			if _, err := os.Stat(newPath); err == nil {
				dialog.ShowError(fmt.Errorf("a record named %q already exists", newName), u.window)
				return
			}
		}

		editedContent := joinPasswordAndMetadata(passwordEntry.Text, notesEntry.Text)
		if editedContent == originalContent {
			if !renamed {
				editDialog.Hide()
				return
			}
			// Only the name changed: move the ciphertext untouched instead
			// of re-encrypting, so all of its recipients are kept.
			if err := storepath.MoveRecord(storeRoot, oldPath, newPath); err != nil {
				dialog.ShowError(err, u.window)
				return
			}
			editDialog.Hide()
			u.afterWrite(fmt.Sprintf("Renamed %s to %s", recordName, newName),
				fmt.Sprintf("Rename %s to %s using GPG Viewer.", recordName, newName))
			return
		}

		encryptAndFinish := func(recipients []string) {
			gpgStderr, err := encryptRecord([]byte(editedContent), newPath, recipients)
			if err != nil {
				dialog.ShowError(fmt.Errorf("could not encrypt %s: %v\n%s", newName, err, gpgStderr), u.window)
				return
			}
			editDialog.Hide()
			if !renamed {
				u.afterWrite("Saved "+newName, fmt.Sprintf("Edit password for %s using GPG Viewer.", newName))
				return
			}
			if err := storepath.RemoveRecord(storeRoot, oldPath); err != nil {
				dialog.ShowError(fmt.Errorf("saved %s, but the old record could not be removed: %v", newName, err), u.window)
				u.afterWrite("Saved "+newName, fmt.Sprintf("Edit password for %s using GPG Viewer.", newName))
				return
			}
			u.afterWrite(fmt.Sprintf("Saved %s as %s", recordName, newName),
				fmt.Sprintf("Rename %s to %s using GPG Viewer.", recordName, newName))
		}

		// Like pass(1), the nearest .gpg-id decides who the record is
		// encrypted for.
		recipients, err := storepath.Recipients(storeRoot, newPath)
		if err != nil {
			dialog.ShowError(err, u.window)
			return
		}
		if len(recipients) > 0 {
			encryptAndFinish(recipients)
			return
		}
		// No .gpg-id: keep the key the original file was encrypted for.
		recipient, gpgStderr, err := gpg.ListRecipientKeyID(oldPath)
		if err != nil {
			dialog.ShowError(fmt.Errorf("could not read the recipient of %s: %v\n%s", recordName, err, gpgStderr), u.window)
			return
		}
		if recipient != "" {
			encryptAndFinish([]string{recipient})
			return
		}
		u.askRecipient(func(r string) { encryptAndFinish([]string{r}) })
	}

	cancelBtn := widget.NewButton("Cancel", func() { editDialog.Hide() })

	deleteBtn := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
		dialog.ShowConfirm("Delete record",
			fmt.Sprintf("Delete %s? This cannot be undone.", recordName),
			func(confirmed bool) {
				if !confirmed {
					return
				}
				if err := storepath.RemoveRecord(storeRoot, oldPath); err != nil {
					dialog.ShowError(err, u.window)
					return
				}
				editDialog.Hide()
				u.afterWrite("Deleted "+recordName, fmt.Sprintf("Remove %s from store using GPG Viewer.", recordName))
			}, u.window)
	})
	deleteBtn.Importance = widget.DangerImportance

	buttons := container.NewBorder(nil, nil, deleteBtn, container.NewHBox(cancelBtn, saveBtn))
	content := container.NewBorder(nil, buttons, nil, nil, form)
	editDialog = dialog.NewCustomWithoutButtons(recordName, content, u.window)
	editDialog.Resize(fyne.NewSize(620, 460))
	editDialog.Show()
}

// askRecipient asks for a GPG recipient when none could be determined.
func (u *ui) askRecipient(onEntered func(recipient string)) {
	entry := widget.NewEntry()
	entry.SetText(u.settings.DefaultRecipient)
	entry.SetPlaceHolder("email or key ID")
	entry.Validator = func(s string) error {
		if strings.TrimSpace(s) == "" {
			return errors.New("enter who the record is encrypted for")
		}
		return nil
	}
	items := []*widget.FormItem{
		{Text: "Recipient", Widget: entry, HintText: "No .gpg-id in the store and the file names no key"},
	}
	form := dialog.NewForm("Choose recipient", "Encrypt", "Cancel", items, func(ok bool) {
		if ok {
			onEntered(strings.TrimSpace(entry.Text))
		}
	}, u.window)
	form.Resize(fyne.NewSize(460, 0))
	form.Show()
}

// showNewRecordDialog shows the form for a new record. prefill is put in
// the name field, e.g. "Work/" when the Work folder is selected.
func (u *ui) showNewRecordDialog(prefill string) {
	storeRoot := u.storePath

	nameEntry := widget.NewEntry()
	nameEntry.SetText(prefill)
	nameEntry.SetPlaceHolder("gmail.com or Work/github")
	nameEntry.Validator = recordNameValidator(storeRoot)

	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("Optional")

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.Validator = func(s string) error {
		if strings.TrimSpace(s) == "" {
			return errors.New("enter or generate a password")
		}
		return nil
	}
	generateBtn := widget.NewButtonWithIcon("Generate", theme.ViewRefreshIcon(), func() {
		pw, err := password.Generate(generatedPasswordLength, password.DefaultCharset)
		if err != nil {
			dialog.ShowError(fmt.Errorf("could not generate a password: %v", err), u.window)
			return
		}
		passwordEntry.SetText(pw)
	})
	passwordRow := container.NewBorder(nil, nil, nil, generateBtn, passwordEntry)

	notesEntry := widget.NewMultiLineEntry()
	notesEntry.SetPlaceHolder("Optional")
	notesEntry.SetMinRowsVisible(3)

	recipientEntry := widget.NewEntry()
	recipientEntry.SetText(u.settings.DefaultRecipient)
	recipientEntry.SetPlaceHolder("Leave empty to use .gpg-id")

	items := []*widget.FormItem{
		{Text: "Name", Widget: nameEntry, HintText: "Folder/name creates the folder"},
		{Text: "Username", Widget: usernameEntry},
		{Text: "Password", Widget: passwordRow},
		{Text: "Notes", Widget: notesEntry},
		{Text: "Encrypt for", Widget: recipientEntry, HintText: "Email or key ID; empty uses the store's .gpg-id"},
	}

	form := dialog.NewForm("New record", "Create record", "Cancel", items, func(create bool) {
		if !create {
			return
		}
		recordName := cleanRecordName(nameEntry.Text)
		username := strings.TrimSpace(usernameEntry.Text)
		// Leading or trailing spaces can be part of a password.
		pw := passwordEntry.Text
		notes := strings.TrimSpace(notesEntry.Text)
		recipient := strings.TrimSpace(recipientEntry.Text)

		var content strings.Builder
		content.WriteString(pw)
		if username != "" {
			content.WriteString("\nusername: " + username)
		}
		if notes != "" {
			content.WriteString("\n" + notes)
		}

		go func() {
			err := createNewPasswordFile(storeRoot, recordName, content.String(), recipient)
			fyne.Do(func() {
				if err != nil {
					dialog.ShowError(fmt.Errorf("could not create %s: %v", recordName, err), u.window)
					return
				}
				u.afterWrite("Created "+recordName, fmt.Sprintf("Add password for %s using GPG Viewer.", recordName))
			})
		}()
	}, u.window)
	form.Resize(fyne.NewSize(560, 0))
	form.Show()
	u.window.Canvas().Focus(nameEntry)
}

// cleanRecordName normalizes a record name typed by the user.
func cleanRecordName(s string) string {
	return strings.Trim(strings.TrimSpace(s), "/")
}

// recordNameValidator checks a record name the way the save path will.
func recordNameValidator(storeRoot string) func(string) error {
	return func(s string) error {
		name := cleanRecordName(s)
		if name == "" {
			return errors.New("enter a name")
		}
		if _, err := storepath.ResolveRecord(storeRoot, name); err != nil {
			return errors.New("name must stay inside the password store")
		}
		return nil
	}
}

// createNewPasswordFile creates a new GPG-encrypted password file. An empty
// recipient means "use the nearest .gpg-id", as pass(1) would.
func createNewPasswordFile(targetPath, recordName, content, recipient string) error {
	filePath, err := storepath.ResolveRecord(targetPath, recordName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("a record named %q already exists", recordName)
	}

	recipients := []string{recipient}
	if recipient == "" {
		if recipients, err = storepath.Recipients(targetPath, filePath); err != nil {
			return err
		}
		if len(recipients) == 0 {
			return errors.New("no recipient: enter one or add a .gpg-id file to the password store")
		}
	}

	// Encrypt the content directly through stdin so the plaintext is never
	// written to disk in the clear.
	gpgStderr, err := encryptRecord([]byte(content), filePath, recipients)
	if err != nil {
		return fmt.Errorf("could not encrypt: %v\n%s", err, gpgStderr)
	}
	return nil
}

// encryptRecord encrypts plaintext for recipients into the record file dest.
// gpg writes to a sibling temp file that is renamed over dest only on
// success, so a failed or killed gpg can never truncate an existing record.
// The temp file only ever holds ciphertext.
func encryptRecord(plaintext []byte, dest string, recipients []string) (stderr []byte, err error) {
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("could not create the folder: %v", err)
	}
	tmp, err := os.CreateTemp(dir, ".gpg-viewer-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("could not create a temp file: %v", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath) // no-op once renamed

	if stderr, err := gpg.EncryptBytes(plaintext, tmpPath, recipients); err != nil {
		return stderr, err
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		return nil, fmt.Errorf("could not replace the record: %v", err)
	}
	return nil, nil
}

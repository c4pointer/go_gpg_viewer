package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go_gpg_viewer/assets"
	"go_gpg_viewer/internal/clipboard"
	"go_gpg_viewer/internal/gpg"
	"go_gpg_viewer/internal/password"
	"go_gpg_viewer/internal/storepath"
	scanpassstore "go_gpg_viewer/scanpassstore"
	"go_gpg_viewer/settings"
)

// AppState holds the UI state shared by the tree, the record list and the
// search box. It is only touched on the Fyne thread.
type AppState struct {
	SelectedNode string   // tree node ID whose records the list shows
	SearchQuery  string   // trimmed, lowercased search text; empty when not searching
	ListItems    []string // records shown in the list: paths relative to the store root, without .gpg
}

// defaultRecipient is populated from settings and used to prefill recipient dialogs
var defaultRecipient string

// clipboardTTL is how long a password copied to the clipboard is allowed to
// survive before it is auto-wiped. Long enough to paste into another app,
// short enough that an unattended clipboard does not leak credentials.
const clipboardTTL = 30 * time.Second

// splitPasswordAndMetadata mirrors the pass(1) on-disk convention: the first
// line of the plaintext is the password, the remaining lines are arbitrary
// metadata (username, notes, ...). Either part may be empty.
func splitPasswordAndMetadata(plaintext string) (password, metadata string) {
	if i := strings.IndexByte(plaintext, '\n'); i >= 0 {
		return plaintext[:i], plaintext[i+1:]
	}
	return plaintext, ""
}

// joinPasswordAndMetadata is the inverse of splitPasswordAndMetadata.
func joinPasswordAndMetadata(password, metadata string) string {
	if metadata == "" {
		return password
	}
	return password + "\n" + metadata
}

// storePtr holds the current PasswordStore snapshot. It is updated atomically
// by the toolbar refresh actions and read concurrently from background
// goroutines (tree/fileList decryption). Every reader should call
// currentStore() to get a snapshot, never dereference storePtr directly.
var storePtr atomic.Pointer[scanpassstore.PasswordStore]

// currentStore returns the latest PasswordStore snapshot. It is safe to call
// from any goroutine; concurrent refreshes will simply expose newer snapshots
// to subsequent calls.
func currentStore() *scanpassstore.PasswordStore {
	return storePtr.Load()
}

// openingRecord is set while a record is being decrypted so that a quick
// double click does not stack two editor dialogs on top of each other. Once
// a dialog is on screen it is modal and blocks further clicks by itself.
var openingRecord atomic.Bool

// decryptAndEditFile decrypts the record recordName (relative to storeRoot,
// without the .gpg extension) and opens it in the editor dialog. It blocks on
// gpg, so call it from a goroutine. onChanged runs on the Fyne thread after
// the record was saved or renamed.
func decryptAndEditFile(storeRoot, recordName string, window fyne.Window, onChanged func()) {
	if !openingRecord.CompareAndSwap(false, true) {
		return
	}
	filePath := filepath.Join(storeRoot, recordName+".gpg")

	var decryptAndEdit func(passphrase string)
	decryptAndEdit = func(passphrase string) {
		plaintext, gpgStderr, err := gpg.Decrypt(filePath, passphrase)
		if err != nil {
			// If this was a first attempt without passphrase, prompt for passphrase
			if passphrase == "" {
				fyne.Do(func() {
					defer openingRecord.Store(false)
					passphraseEntry := widget.NewPasswordEntry()

					passphraseDialog := dialog.NewCustomConfirm(
						"Enter Passphrase",
						"Decrypt",
						"Cancel",
						container.NewVBox(
							widget.NewLabel(fmt.Sprintf("File: %s", recordName)),
							widget.NewLabel("GPG agent requires passphrase. Please enter:"),
							passphraseEntry,
						),
						func(decrypt bool) {
							if !decrypt {
								return
							}
							newPassphrase := passphraseEntry.Text
							if newPassphrase == "" {
								dialog.ShowError(errors.New("Passphrase cannot be empty"), window)
								return
							}
							// Try again with the provided passphrase
							openingRecord.Store(true)
							go decryptAndEdit(newPassphrase)
						},
						window,
					)
					passphraseDialog.Show()
				})
				return
			}
			// This was already a passphrase attempt, show error.
			// Only stderr is surfaced — stdout may contain partial
			// plaintext and must never reach the UI.
			fyne.Do(func() {
				defer openingRecord.Store(false)
				dialog.ShowError(fmt.Errorf("Failed to decrypt file: %v\n%s", err, gpgStderr), window)
			})
			return
		}

		// Widgets may only be built and shown on the Fyne thread.
		fyne.Do(func() {
			defer openingRecord.Store(false)
			showRecordEditor(storeRoot, recordName, string(plaintext), window, onChanged)
		})
	}

	// Start the decryption process
	decryptAndEdit("")
}

// showRecordEditor shows the dialog to edit a decrypted record: its name
// (renaming moves the file inside the store), password and metadata. Must
// run on the Fyne thread.
func showRecordEditor(storeRoot, recordName, plaintext string, window fyne.Window, onChanged func()) {
	oldPath, err := storepath.ResolveRecord(storeRoot, recordName)
	if err != nil {
		dialog.ShowError(err, window)
		return
	}

	// Split into password (first line, masked) and metadata (everything
	// else, plain). pass(1) convention.
	pwPart, metaPart := splitPasswordAndMetadata(plaintext)
	originalContent := joinPasswordAndMetadata(pwPart, metaPart)

	nameEntry := widget.NewEntry()
	nameEntry.SetText(recordName)
	nameEntry.SetPlaceHolder("Record name (e.g., gmail.com, bank/chase)")

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetText(pwPart)

	metadataEntry := widget.NewMultiLineEntry()
	metadataEntry.SetText(metaPart)

	copyBtn := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() {
		app := fyne.CurrentApp()
		if app == nil {
			return
		}
		clipboard.CopyWithAutoClear(app.Clipboard(), passwordEntry.Text, clipboardTTL)
		app.SendNotification(&fyne.Notification{
			Title:   "Password copied",
			Content: fmt.Sprintf("Clipboard will be cleared in %s.", clipboardTTL),
		})
	})
	generateBtn := widget.NewButtonWithIcon("Generate", theme.ViewRefreshIcon(), func() {
		pw, err := password.Generate(20, password.DefaultCharset)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to generate password: %v", err), window)
			return
		}
		passwordEntry.SetText(pw)
	})
	passwordActions := container.NewHBox(copyBtn, generateBtn)
	passwordRow := container.NewBorder(nil, nil, nil, passwordActions, passwordEntry)

	contentBox := container.NewBorder(
		container.NewVBox(
			widget.NewLabel("Name:"),
			nameEntry,
			widget.NewLabel("Password:"),
			passwordRow,
			widget.NewLabel("Metadata:"),
		),
		nil, nil, nil,
		metadataEntry,
	)

	var editDialog *dialog.CustomDialog

	// finish closes the editor, reports success and lets the caller rescan
	// the store so a renamed record shows up under its new name.
	finish := func(message string) {
		editDialog.Hide()
		dialog.ShowInformation("Success", message, window)
		if onChanged != nil {
			onChanged()
		}
	}

	saveBtn := widget.NewButtonWithIcon("Save Changes", theme.DocumentSaveIcon(), func() {
		newName := strings.Trim(strings.TrimSpace(nameEntry.Text), "/")
		newPath, err := storepath.ResolveRecord(storeRoot, newName)
		if err != nil {
			dialog.ShowError(err, window)
			return
		}
		renamed := newPath != oldPath
		if renamed {
			if _, err := os.Stat(newPath); err == nil {
				dialog.ShowError(fmt.Errorf("Password file '%s' already exists", newName), window)
				return
			}
		}

		editedContent := joinPasswordAndMetadata(passwordEntry.Text, metadataEntry.Text)
		if editedContent == originalContent {
			if !renamed {
				editDialog.Hide()
				return
			}
			// Only the name changed: move the ciphertext untouched instead
			// of re-encrypting, so all of its recipients are kept.
			if err := storepath.MoveRecord(storeRoot, oldPath, newPath); err != nil {
				dialog.ShowError(err, window)
				return
			}
			finish(fmt.Sprintf("Record renamed to '%s'", newName))
			return
		}

		// Edited plaintext is streamed straight to gpg over stdin — never
		// staged to a temp file — so a crash here cannot leave plaintext
		// on disk.
		encryptAndFinish := func(recipients []string) {
			gpgStderr, err := encryptRecord([]byte(editedContent), newPath, recipients)
			if err != nil {
				dialog.ShowError(fmt.Errorf("Failed to encrypt file: %v\n%s", err, gpgStderr), window)
				return
			}
			if renamed {
				if err := storepath.RemoveRecord(storeRoot, oldPath); err != nil {
					editDialog.Hide()
					dialog.ShowError(fmt.Errorf("Saved as '%s', but the old file could not be removed: %v", newName, err), window)
					if onChanged != nil {
						onChanged()
					}
					return
				}
				finish(fmt.Sprintf("Record saved as '%s'", newName))
				return
			}
			finish("File saved successfully")
		}

		// Like pass(1), the nearest .gpg-id decides who the record is
		// encrypted for.
		recipients, err := storepath.Recipients(storeRoot, newPath)
		if err != nil {
			dialog.ShowError(err, window)
			return
		}
		if len(recipients) > 0 {
			encryptAndFinish(recipients)
			return
		}

		// No .gpg-id: fall back to the key the original file was encrypted for
		recipient, gpgStderr, err := gpg.ListRecipientKeyID(oldPath)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to get recipient info: %v\n%s", err, gpgStderr), window)
			return
		}
		if recipient != "" {
			encryptAndFinish([]string{recipient})
			return
		}

		// If we couldn't find the recipient, ask the user
		recipientEntry := widget.NewEntry()
		if defaultRecipient != "" {
			recipientEntry.SetText(defaultRecipient)
		} else {
			recipientEntry.SetPlaceHolder("email or key ID")
		}
		recipientDialog := dialog.NewCustomConfirm(
			"Enter Recipient",
			"Encrypt",
			"Cancel",
			container.NewVBox(
				widget.NewLabel("Could not detect recipient automatically."),
				widget.NewLabel("Please enter GPG recipient (email or key ID):"),
				recipientEntry,
			),
			func(confirm bool) {
				if !confirm {
					return
				}
				recipient := strings.TrimSpace(recipientEntry.Text)
				if recipient == "" {
					dialog.ShowError(errors.New("Recipient cannot be empty"), window)
					return
				}
				encryptAndFinish([]string{recipient})
			},
			window,
		)
		recipientDialog.Show()
	})

	closeBtn := widget.NewButtonWithIcon("Close", theme.CancelIcon(), func() {
		editDialog.Hide()
	})

	// Create the dialog with content and buttons
	buttonContainer := container.NewHBox(saveBtn, closeBtn)
	contentContainer := container.NewBorder(nil, buttonContainer, nil, nil, contentBox)
	editDialog = dialog.NewCustomWithoutButtons("Edit Password File", contentContainer, window)
	editDialog.Resize(fyne.NewSize(600, 450))
	editDialog.Show()
}

// encryptRecord encrypts plaintext for recipients into the record file dest.
// gpg writes to a sibling temp file that is renamed over dest only on
// success, so a failed or killed gpg can never truncate an existing record.
// The temp file only ever holds ciphertext.
func encryptRecord(plaintext []byte, dest string, recipients []string) (stderr []byte, err error) {
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create directory structure: %v", err)
	}
	tmp, err := os.CreateTemp(dir, ".gpg-viewer-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %v", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath) // no-op once renamed

	if stderr, err := gpg.EncryptBytes(plaintext, tmpPath, recipients); err != nil {
		return stderr, err
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		return nil, fmt.Errorf("failed to replace record: %v", err)
	}
	return nil, nil
}

// showNewRecordDialog displays a dialog for creating a new password record
func showNewRecordDialog(window fyne.Window, targetPath string, defaultRecipient string, refreshCallback func()) {
	// Create form entries
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Enter record name (e.g., gmail.com, bank/chase)")

	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("Enter username/email")

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Enter password")

	// Generate fills the password entry with a 20-char random password
	// drawn from password.DefaultCharset.
	generateBtn := widget.NewButtonWithIcon("Generate", theme.ViewRefreshIcon(), func() {
		pw, err := password.Generate(20, password.DefaultCharset)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to generate password: %v", err), window)
			return
		}
		passwordEntry.SetText(pw)
	})
	passwordRow := container.NewBorder(nil, nil, nil, generateBtn, passwordEntry)

	notesEntry := widget.NewMultiLineEntry()
	notesEntry.SetPlaceHolder("Additional notes (optional)")
	notesEntry.Resize(fyne.NewSize(400, 100))

	recipientEntry := widget.NewEntry()
	if defaultRecipient != "" {
		recipientEntry.SetText(defaultRecipient)
	} else {
		recipientEntry.SetPlaceHolder("GPG recipient (empty: use .gpg-id)")
	}

	// Create form content
	formContent := container.NewVBox(
		widget.NewLabel("Create New Password Record"),
		widget.NewSeparator(),

		widget.NewLabel("Record Name:"),
		nameEntry,

		widget.NewLabel("Username:"),
		usernameEntry,

		widget.NewLabel("Password:"),
		passwordRow,

		widget.NewLabel("Notes:"),
		notesEntry,

		widget.NewLabel("GPG Recipient:"),
		recipientEntry,
	)

	// Create dialog
	newRecordDialog := dialog.NewCustomConfirm(
		"New Password Record",
		"Create",
		"Cancel",
		formContent,
		func(create bool) {
			if !create {
				return
			}

			// Validate inputs
			recordName := strings.TrimSpace(nameEntry.Text)
			username := strings.TrimSpace(usernameEntry.Text)
			password := passwordEntry.Text
			notes := strings.TrimSpace(notesEntry.Text)
			recipient := strings.TrimSpace(recipientEntry.Text)

			if recordName == "" {
				dialog.ShowError(errors.New("Record name cannot be empty"), window)
				return
			}

			if username == "" {
				dialog.ShowError(errors.New("Username cannot be empty"), window)
				return
			}

			if strings.TrimSpace(password) == "" {
				dialog.ShowError(errors.New("Password cannot be empty"), window)
				return
			}

			// Create password content
			var content strings.Builder
			content.WriteString(password)
			content.WriteString("\n")
			content.WriteString("Username: " + username)
			if notes != "" {
				content.WriteString("\n")
				content.WriteString("Notes: " + notes)
			}

			// Create the GPG file
			go func() {
				err := createNewPasswordFile(targetPath, recordName, content.String(), recipient)
				if err != nil {
					fyne.Do(func() {
						dialog.ShowError(fmt.Errorf("Failed to create password file: %v", err), window)
					})
					return
				}

				// Success - refresh the UI
				fyne.Do(func() {
					dialog.ShowInformation("Success", fmt.Sprintf("Password record '%s' created successfully", recordName), window)
					if refreshCallback != nil {
						refreshCallback()
					}
				})
			}()
		},
		window,
	)

	newRecordDialog.Resize(fyne.NewSize(500, 600))
	newRecordDialog.Show()
}

// createNewPasswordFile creates a new GPG-encrypted password file. An empty
// recipient means "use the nearest .gpg-id", as pass(1) would.
func createNewPasswordFile(targetPath, recordName, content, recipient string) error {
	filePath, err := storepath.ResolveRecord(targetPath, recordName)
	if err != nil {
		return err
	}

	// Check if file already exists
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("password file '%s' already exists", recordName)
	}

	recipients := []string{recipient}
	if recipient == "" {
		if recipients, err = storepath.Recipients(targetPath, filePath); err != nil {
			return err
		}
		if len(recipients) == 0 {
			return errors.New("no GPG recipient: enter one or add a .gpg-id file to the password store")
		}
	}

	// Encrypt the content directly through stdin so the plaintext is never
	// written to disk in the clear.
	gpgStderr, err := encryptRecord([]byte(content), filePath, recipients)
	if err != nil {
		return fmt.Errorf("failed to encrypt file: %v\n%s", err, gpgStderr)
	}

	return nil
}

func main() {
	// Load application settings
	appSettings, err := settings.LoadSettings()
	if err != nil {
		fmt.Println("Error loading settings:", err)
		return
	}

	// Get the current user and home directory
	Target := ".password-store"
	userCurrent, err := user.Current()
	if err != nil {
		fmt.Println("Error getting current user:", err)
		return
	}
	homeDir := userCurrent.HomeDir

	// Use settings path if available, otherwise use default
	targetPath := appSettings.PasswordStorePath
	if targetPath == "" {
		targetPath = filepath.Join(homeDir, Target)
	}

	// Check if the target directory exists
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		fmt.Println("Target directory does not exist:", targetPath)
		return
	}

	// Scan password store
	initialStore, err := scanpassstore.ScanPasswordStore(targetPath)
	if err != nil {
		fmt.Println("Error scanning password store:", err)
		return
	}
	storePtr.Store(initialStore)

	// Initialize GUI
	myApp := app.New()

	// Set application icon
	myApp.SetIcon(assets.GetAppIcon())

	// Apply theme from settings
	settings.ApplyTheme(myApp, appSettings.Theme)
	// Prefill default recipient for encryption dialogs
	defaultRecipient = appSettings.DefaultRecipient

	myWindow := myApp.NewWindow("GPG Password Store Viewer")
	myWindow.Resize(fyne.NewSize(float32(appSettings.WindowWidth), float32(appSettings.WindowHeight)))

	// Save window size when closing
	myWindow.SetOnClosed(func() {
		size := myWindow.Canvas().Size()
		settings.UpdateSettings(map[string]interface{}{
			"window_width":  int(size.Width),
			"window_height": int(size.Height),
		})
	})

	// Create application state
	appState := &AppState{}

	// The tree is rebuilt from the current store snapshot on every refresh;
	// node IDs are full relative paths (see storetree.go).
	tree := widget.NewTree(
		func(id widget.TreeNodeID) []widget.TreeNodeID {
			return nodeChildren(currentStore(), id)
		},
		nodeIsBranch,
		func(branch bool) fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.TreeNodeID, branch bool, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(nodeLabel(id))
		},
	)

	// Content area for displaying selected items
	contentLabel := widget.NewLabel("Select an item to view details")
	contentLabel.Wrapping = fyne.TextWrapWord

	// File list for the selected directory or the search results. It always
	// renders appState.ListItems, so what is shown is exactly what opens.
	fileList := widget.NewList(
		func() int { return len(appState.ListItems) },
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			label := o.(*widget.Label)
			if id < 0 || id >= len(appState.ListItems) {
				label.SetText("")
				return
			}
			item := appState.ListItems[id]
			if appState.SearchQuery == "" {
				// Inside a directory the base name is enough; search results
				// keep the full path so equal names stay distinguishable.
				item = filepath.Base(item)
			}
			label.SetText(item)
		},
	)

	// renderList fills the record list from the search query, or from the
	// selected tree node when no search is active.
	renderList := func() {
		if appState.SearchQuery != "" {
			appState.ListItems = searchRecords(currentStore(), appState.SearchQuery)
			contentLabel.SetText(fmt.Sprintf("Found %d matching entries", len(appState.ListItems)))
		} else {
			var status string
			appState.ListItems, status = nodeListing(currentStore(), appState.SelectedNode)
			contentLabel.SetText(status)
		}
		fileList.UnselectAll()
		fileList.Refresh()
	}

	// reloadStore rescans the password store and refreshes the tree and the
	// list. Must run on the Fyne thread.
	reloadStore := func() bool {
		newStore, err := scanpassstore.ScanPasswordStore(targetPath)
		if err != nil {
			dialog.ShowError(err, myWindow)
			return false
		}
		storePtr.Store(newStore)
		tree.Refresh()
		renderList()
		return true
	}

	openRecord := func(rel string) {
		go decryptAndEditFile(targetPath, rel, myWindow, func() { reloadStore() })
	}

	// Search entry (global search across store)
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search passwords… (name or path)")
	searchEntry.OnChanged = func(query string) {
		appState.SearchQuery = strings.ToLower(strings.TrimSpace(query))
		renderList()
	}

	// Handle tree selection
	tree.OnSelected = func(id widget.TreeNodeID) {
		if rel, ok := recordFromNode(id); ok {
			// A record is an action, not a lasting selection. Tree only fires
			// OnSelected when the selection changes, so keeping the record
			// selected would make a second click on it do nothing.
			tree.Unselect(id)
			if appState.SearchQuery == "" {
				appState.SelectedNode = parentNode(rel)
				renderList()
			}
			openRecord(rel)
			return
		}

		appState.SelectedNode = id
		if appState.SearchQuery != "" {
			// Leaving search mode re-renders the list via OnChanged.
			searchEntry.SetText("")
			return
		}
		renderList()
	}

	// Handle file selection
	fileList.OnSelected = func(id widget.ListItemID) {
		// Same as the tree: drop the selection so the same row can be
		// opened again right after its editor is closed.
		fileList.Unselect(id)
		if id < 0 || id >= len(appState.ListItems) {
			return
		}
		openRecord(appState.ListItems[id])
	}

	// Layout the UI
	split := container.NewHSplit(
		container.NewBorder(
			widget.NewLabel("Password Store Structure"),
			nil, nil, nil,
			container.NewScroll(tree),
		),
		container.NewBorder(
			contentLabel,
			nil, nil, nil,
			container.NewScroll(fileList),
		),
	)
	split.SetOffset(0.3)

	// Function to refresh the UI
	refreshUI := func() {
		// Refresh all UI components
		tree.Refresh()
		fileList.Refresh()
		contentLabel.Refresh()
		myWindow.Canvas().Refresh(myWindow.Content())
	}

	// showGitError reports a failed git step. Notifications are easy to miss
	// (or disabled), so failures get a dialog.
	showGitError := func(progressDialog *dialog.CustomDialog, err error) {
		fyne.Do(func() {
			progressDialog.Hide()
			dialog.ShowError(err, myWindow)
		})
	}

	// Add a toolbar with actions
	toolbar := widget.NewToolbar(
		widget.NewToolbarAction(theme.ViewRefreshIcon(), func() {
			// Refresh the password store data
			if reloadStore() {
				contentLabel.SetText("Password store refreshed")
			}
		}),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.ContentAddIcon(), func() {
			// Show new record creation dialog
			showNewRecordDialog(myWindow, targetPath, defaultRecipient, func() { reloadStore() })
		}),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.DocumentSaveIcon(), func() {
			// Manual commit functionality
			progressBar := widget.NewProgressBar()
			progressLabel := widget.NewLabel("Committing changes...")

			progressDialog := dialog.NewCustomWithoutButtons("Git Commit",
				container.NewVBox(progressLabel, progressBar), myWindow)
			progressDialog.Show()

			go func() {
				fyne.Do(func() {
					progressBar.SetValue(0.2)
				})

				// Check if there are any changes to commit
				statusOutput, statusErr := runGit(targetPath, "status", "--porcelain")
				if statusErr != nil {
					showGitError(progressDialog, statusErr)
					return
				}
				if strings.TrimSpace(statusOutput) == "" {
					fyne.Do(func() {
						fyne.CurrentApp().SendNotification(&fyne.Notification{
							Title:   "No Changes",
							Content: "No changes to commit.",
						})
						progressDialog.Hide()
					})
					return
				}

				fyne.Do(func() {
					progressBar.SetValue(0.5)
				})

				// Commit with timestamp
				commitMsg := fmt.Sprintf("Manual commit: %s", time.Now().Format("2006-01-02 15:04:05"))
				if err := commitAll(targetPath, commitMsg, func(v float64) {
					fyne.Do(func() { progressBar.SetValue(v) })
				}); err != nil {
					showGitError(progressDialog, err)
					return
				}

				fyne.Do(func() {
					progressBar.SetValue(1.0)
					fyne.CurrentApp().SendNotification(&fyne.Notification{
						Title:   "Commit Successful",
						Content: "Changes committed successfully.",
					})
					progressDialog.Hide()
				})
			}()
		}),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.DownloadIcon(), func() {
			// Git sync functionality
			progressBar := widget.NewProgressBar()
			progressLabel := widget.NewLabel("Syncing with remote repository...")

			progressDialog := dialog.NewCustomWithoutButtons("Git Sync",
				container.NewVBox(progressLabel, progressBar), myWindow)
			progressDialog.Show()

			go func() {
				setProgress := func(v float64) {
					fyne.Do(func() { progressBar.SetValue(v) })
				}
				setProgress(0.05)

				// Check if there are any changes to commit
				statusOutput, statusErr := runGit(targetPath, "status", "--porcelain")
				if statusErr != nil {
					showGitError(progressDialog, statusErr)
					return
				}
				hasChanges := strings.TrimSpace(statusOutput) != ""

				// Local changes are committed first: `git pull --rebase`
				// refuses to run on a dirty working tree.
				if hasChanges {
					setProgress(0.1)
					commitMsg := fmt.Sprintf("Auto-commit: %s", time.Now().Format("2006-01-02 15:04:05"))
					if err := commitAll(targetPath, commitMsg, nil); err != nil {
						showGitError(progressDialog, err)
						return
					}
				}

				setProgress(0.3)

				// Pull latest changes, replaying local commits on top
				if _, err := runGit(targetPath, "pull", "--rebase"); err != nil {
					showGitError(progressDialog, err)
					return
				}

				setProgress(0.7)

				// Push to remote
				if _, err := runGit(targetPath, "push"); err != nil {
					showGitError(progressDialog, err)
					return
				}

				fyne.Do(func() {
					progressBar.SetValue(1.0)

					// Refresh the store data
					reloadStore()

					// Show success notification
					var message string
					if hasChanges {
						message = "Successfully committed and pushed changes to remote repository."
					} else {
						message = "Successfully synchronized with remote repository (no local changes)."
					}

					fyne.CurrentApp().SendNotification(&fyne.Notification{
						Title:   "Git Sync Complete",
						Content: message,
					})

					progressDialog.Hide()
				})
			}()
		}),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.SettingsIcon(), func() {
			// Show settings dialog
			settings.ShowSettingsDialog(myWindow, appSettings, refreshUI)
		}),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.InfoIcon(), func() {
			settings.ShowAboutDialog(myWindow)
		}),
	)

	// Top area: toolbar + search
	topContainer := container.NewVBox(
		toolbar,
		container.NewBorder(nil, nil, nil, nil, searchEntry),
	)

	// Main container with toolbar/search and split view
	mainContainer := container.NewBorder(
		topContainer,
		nil, nil, nil,
		split,
	)

	myWindow.SetContent(mainContainer)
	myWindow.ShowAndRun()
}

// runGit runs git with args inside dir. On failure git's combined output is
// folded into the error so the cause is visible in the UI.
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("git %s failed: %v\n%s", args[0], err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

// commitAll stages every change in the repository at dir and commits it
// with message. progress, if non-nil, is called between the two steps.
func commitAll(dir, message string, progress func(float64)) error {
	if _, err := runGit(dir, "add", "-A"); err != nil {
		return err
	}
	if progress != nil {
		progress(0.8)
	}
	_, err := runGit(dir, "commit", "-m", message)
	return err
}

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go_gpg_viewer/assets"
	"go_gpg_viewer/internal/gitsync"
	scanpassstore "go_gpg_viewer/scanpassstore"
	"go_gpg_viewer/settings"
)

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

// storePtr holds the current PasswordStore snapshot. It is replaced
// atomically on every rescan and read from background goroutines, so every
// reader calls currentStore() for a snapshot instead of dereferencing it.
var storePtr atomic.Pointer[scanpassstore.PasswordStore]

// currentStore returns the latest PasswordStore snapshot.
func currentStore() *scanpassstore.PasswordStore {
	return storePtr.Load()
}

// emptyStore returns a store with no records, used until a valid store
// path is configured.
func emptyStore(path string) *scanpassstore.PasswordStore {
	return &scanpassstore.PasswordStore{
		RootPath:    path,
		DirContents: map[string][]string{},
		NestedDirs:  map[string][]string{},
		AllPaths:    map[string]string{},
	}
}

// AppState holds the UI state shared by the tree, the record list and the
// search box. It is only touched on the Fyne thread.
type AppState struct {
	SelectedNode string   // tree node whose records the list shows ("" is the store root)
	SearchQuery  string   // trimmed, lowercased search text; empty when not searching
	ListItems    []string // records shown in the list: paths relative to the store root, without .gpg
}

// ui wires the main window together. All fields are owned by the Fyne
// thread; background work hands results back through fyne.Do.
type ui struct {
	app       fyne.App
	window    fyne.Window
	settings  *settings.Settings
	storePath string
	state     AppState

	tree       *widget.Tree
	list       *widget.List
	search     *widget.Entry
	heading    *widget.Label
	subheading *widget.Label
	emptyState *widget.Label
	status     *widget.Label
	storeInfo  *widget.Label
}

func main() {
	appSettings, err := settings.LoadSettings()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error loading settings:", err)
		os.Exit(1)
	}
	storePath := appSettings.PasswordStorePath
	if storePath == "" {
		storePath = settings.DefaultStorePath()
	}

	fyneApp := app.New()
	fyneApp.SetIcon(assets.GetAppIcon())
	settings.ApplyTheme(fyneApp, appSettings.Theme)

	u := &ui{
		app:       fyneApp,
		window:    fyneApp.NewWindow("GPG Password Store Viewer"),
		settings:  appSettings,
		storePath: storePath,
	}
	u.window.Resize(fyne.NewSize(float32(appSettings.WindowWidth), float32(appSettings.WindowHeight)))

	// The store is scanned before the window is built so the first frame
	// already shows it; a missing store gets an empty window plus a hint
	// instead of a silent exit.
	store, scanErr := scanpassstore.ScanPasswordStore(storePath)
	if scanErr != nil {
		store = emptyStore(storePath)
	}
	storePtr.Store(store)

	split := u.build()
	u.window.SetOnClosed(func() {
		size := u.window.Canvas().Size()
		settings.UpdateSettings(map[string]interface{}{
			"window_width":  int(size.Width),
			"window_height": int(size.Height),
			"split_offset":  split.Offset,
		})
	})

	u.renderList()
	u.updateStoreInfo()
	if scanErr != nil {
		u.setStatus("Password store not found at " + storePath)
		dialog.ShowInformation("Set up your password store",
			"No password store was found at\n"+storePath+"\n\nChoose its folder in Settings.", u.window)
	} else {
		u.setStatus("Ready")
	}

	u.window.ShowAndRun()
}

// build creates the window content and returns the split so its offset
// can be persisted on close.
func (u *ui) build() *container.Split {
	// Header: the actions people reach for on the left, housekeeping on the right.
	newBtn := widget.NewButtonWithIcon("New record", theme.ContentAddIcon(), func() {
		u.showNewRecordDialog(u.newRecordPrefill())
	})
	newBtn.Importance = widget.HighImportance
	syncBtn := widget.NewButtonWithIcon("Sync", assets.SyncIcon(), u.syncStore)
	commitBtn := widget.NewButtonWithIcon("Commit", theme.DocumentSaveIcon(), u.commitStore)

	refreshBtn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		if u.reloadStore() {
			u.setStatus("Password store reloaded")
		}
	})
	refreshBtn.Importance = widget.LowImportance
	settingsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), u.showSettings)
	settingsBtn.Importance = widget.LowImportance
	aboutBtn := widget.NewButtonWithIcon("", theme.InfoIcon(), func() {
		settings.ShowAboutDialog(u.window)
	})
	aboutBtn.Importance = widget.LowImportance

	header := container.NewBorder(nil, nil,
		container.NewHBox(newBtn, syncBtn, commitBtn),
		container.NewHBox(refreshBtn, settingsBtn, aboutBtn),
	)

	// Search across the whole store, with a clear button once there is text.
	u.search = widget.NewEntry()
	u.search.SetPlaceHolder("Search all records")
	clearBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		u.search.SetText("")
	})
	clearBtn.Importance = widget.LowImportance
	clearBtn.Hide()
	u.search.ActionItem = clearBtn
	u.search.OnChanged = func(query string) {
		u.state.SearchQuery = strings.ToLower(strings.TrimSpace(query))
		if query == "" {
			clearBtn.Hide()
		} else {
			clearBtn.Show()
		}
		u.renderList()
	}
	searchRow := container.NewBorder(nil, nil, widget.NewIcon(theme.SearchIcon()), nil, u.search)

	// Folder tree. Node IDs are full relative paths (see storetree.go).
	u.tree = widget.NewTree(
		func(id widget.TreeNodeID) []widget.TreeNodeID {
			return nodeChildren(currentStore(), id)
		},
		nodeIsBranch,
		func(branch bool) fyne.CanvasObject {
			return newIconRow()
		},
		func(id widget.TreeNodeID, branch bool, o fyne.CanvasObject) {
			icon := theme.DocumentIcon()
			if branch {
				icon = theme.FolderIcon()
			}
			setIconRow(o, icon, nodeLabel(id))
		},
	)
	u.tree.OnSelected = func(id widget.TreeNodeID) {
		if rel, ok := recordFromNode(id); ok {
			// A record is an action, not a lasting selection. Tree only fires
			// OnSelected when the selection changes, so keeping the record
			// selected would make a second click on it do nothing.
			u.tree.Unselect(id)
			if u.state.SearchQuery == "" {
				u.state.SelectedNode = parentNode(rel)
				u.renderList()
			}
			u.openRecord(rel)
			return
		}
		u.state.SelectedNode = id
		if u.state.SearchQuery != "" {
			// Leaving search mode re-renders the list via OnChanged.
			u.search.SetText("")
			return
		}
		u.renderList()
	}

	// Record list for the selected folder or the search results. It always
	// renders state.ListItems, so what is shown is exactly what opens.
	u.list = widget.NewList(
		func() int { return len(u.state.ListItems) },
		func() fyne.CanvasObject { return newIconRow() },
		func(id widget.ListItemID, o fyne.CanvasObject) {
			if id < 0 || id >= len(u.state.ListItems) {
				setIconRow(o, theme.DocumentIcon(), "")
				return
			}
			item := u.state.ListItems[id]
			if u.state.SearchQuery == "" {
				// Inside a folder the base name is enough; search results
				// keep the full path so equal names stay distinguishable.
				item = filepath.Base(item)
			}
			setIconRow(o, theme.DocumentIcon(), item)
		},
	)
	u.list.OnSelected = func(id widget.ListItemID) {
		// Same as the tree: drop the selection so the same row can be
		// opened again right after its editor is closed.
		u.list.Unselect(id)
		if id < 0 || id >= len(u.state.ListItems) {
			return
		}
		u.openRecord(u.state.ListItems[id])
	}

	u.heading = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	u.heading.Truncation = fyne.TextTruncateEllipsis
	u.subheading = widget.NewLabel("")
	u.subheading.Alignment = fyne.TextAlignTrailing
	u.emptyState = widget.NewLabelWithStyle("", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
	u.emptyState.Wrapping = fyne.TextWrapWord
	listArea := container.NewStack(u.list, container.NewCenter(u.emptyState))

	records := container.NewBorder(
		container.NewVBox(container.NewBorder(nil, nil, nil, u.subheading, u.heading), widget.NewSeparator()),
		nil, nil, nil,
		listArea,
	)

	split := container.NewHSplit(u.tree, records)
	split.SetOffset(u.settings.SplitOffset)

	// Status bar: what just happened on the left, where we are on the right.
	u.status = widget.NewLabel("")
	u.status.Truncation = fyne.TextTruncateEllipsis
	u.storeInfo = widget.NewLabel("")
	u.storeInfo.Alignment = fyne.TextAlignTrailing
	statusBar := container.NewBorder(widget.NewSeparator(), nil, nil, u.storeInfo, u.status)

	top := container.NewVBox(header, searchRow)
	u.window.SetContent(container.NewBorder(top, statusBar, nil, nil, split))

	u.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyF, Modifier: fyne.KeyModifierControl},
		func(fyne.Shortcut) { u.window.Canvas().Focus(u.search) })
	u.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyN, Modifier: fyne.KeyModifierControl},
		func(fyne.Shortcut) { u.showNewRecordDialog(u.newRecordPrefill()) })

	return split
}

// newIconRow is the template for tree and list rows: an icon and a name.
func newIconRow() fyne.CanvasObject {
	label := widget.NewLabel("")
	label.Truncation = fyne.TextTruncateEllipsis
	return container.NewBorder(nil, nil, widget.NewIcon(theme.DocumentIcon()), nil, label)
}

// setIconRow fills a row created by newIconRow.
func setIconRow(o fyne.CanvasObject, icon fyne.Resource, text string) {
	row := o.(*fyne.Container)
	for _, obj := range row.Objects {
		switch w := obj.(type) {
		case *widget.Icon:
			w.SetResource(icon)
		case *widget.Label:
			w.SetText(text)
		}
	}
}

// newRecordPrefill returns "Folder/" for the selected folder so a new
// record lands next to the ones on screen.
func (u *ui) newRecordPrefill() string {
	if u.state.SearchQuery != "" {
		return ""
	}
	if dir, ok := dirFromNode(u.state.SelectedNode); ok && dir != "" {
		return dir + "/"
	}
	return ""
}

// renderList fills the record list from the search query, or from the
// selected tree node when no search is active.
func (u *ui) renderList() {
	store := currentStore()
	if u.state.SearchQuery != "" {
		u.state.ListItems = searchRecords(store, u.state.SearchQuery)
		u.heading.SetText("Search results")
		u.subheading.SetText(plural(len(u.state.ListItems), "match"))
		u.showEmptyState(len(u.state.ListItems) == 0, fmt.Sprintf("No records match %q", strings.TrimSpace(u.search.Text)))
	} else {
		var heading string
		u.state.ListItems, heading = nodeListing(store, u.state.SelectedNode)
		u.heading.SetText(heading)
		u.subheading.SetText(plural(len(u.state.ListItems), "record"))
		hint := "No records in this folder yet. Open a subfolder on the left or add a record."
		if countRecords(store) == 0 {
			hint = "This password store is empty. Add a record, or choose another folder in Settings."
		}
		u.showEmptyState(len(u.state.ListItems) == 0, hint)
	}
	u.list.UnselectAll()
	u.list.Refresh()
}

// showEmptyState swaps the list for a hint when there is nothing to show.
func (u *ui) showEmptyState(empty bool, hint string) {
	if empty {
		u.emptyState.SetText(hint)
		u.list.Hide()
		u.emptyState.Show()
	} else {
		u.emptyState.Hide()
		u.list.Show()
	}
}

// updateStoreInfo refreshes the record count and path in the status bar.
func (u *ui) updateStoreInfo() {
	path := u.storePath
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home+string(os.PathSeparator)) {
		path = "~" + path[len(home):]
	}
	u.storeInfo.SetText(fmt.Sprintf("%s in %s", plural(countRecords(currentStore()), "record"), path))
}

// setStatus shows a short message in the status bar.
func (u *ui) setStatus(msg string) {
	u.status.SetText(msg)
}

// notify sends a system notification if the user has them enabled.
func (u *ui) notify(title, content string) {
	if u.settings.ShowNotifications {
		u.app.SendNotification(&fyne.Notification{Title: title, Content: content})
	}
}

// reloadStore rescans the password store and refreshes the tree and the
// list. Must run on the Fyne thread.
func (u *ui) reloadStore() bool {
	newStore, err := scanpassstore.ScanPasswordStore(u.storePath)
	if err != nil {
		dialog.ShowError(err, u.window)
		return false
	}
	storePtr.Store(newStore)
	u.tree.Refresh()
	u.renderList()
	u.updateStoreInfo()
	return true
}

// afterWrite runs after a record was created, saved, renamed or deleted:
// it rescans the store, reports what happened and, when auto-commit is
// on, records the change in git with commitMessage.
func (u *ui) afterWrite(summary, commitMessage string) {
	u.reloadStore()
	u.setStatus(summary)
	u.notify("Password store", summary)
	if !u.settings.AutoCommit {
		return
	}
	storeRoot := u.storePath
	go func() {
		err := gitsync.CommitAll(storeRoot, commitMessage)
		fyne.Do(func() {
			if err != nil {
				dialog.ShowError(fmt.Errorf("%s, but the git commit failed: %v", summary, err), u.window)
				return
			}
			u.setStatus(summary + " and committed to git")
		})
	}()
}

// showSettings opens the settings dialog and applies a changed store path.
func (u *ui) showSettings() {
	settings.ShowSettingsDialog(u.window, u.settings, func(updated *settings.Settings) {
		newPath := updated.PasswordStorePath
		if newPath == "" {
			newPath = settings.DefaultStorePath()
		}
		if newPath != u.storePath {
			u.storePath = newPath
			u.state = AppState{}
			u.search.SetText("")
			if u.reloadStore() {
				u.setStatus("Switched to " + newPath)
			}
			return
		}
		u.setStatus("Settings saved")
	})
}

// runGitTask shows a progress dialog while task runs in the background,
// then reports the outcome in the status bar (and a dialog on failure).
func (u *ui) runGitTask(title string, task func(progress func(step string, fraction float64)) (summary string, changed bool, err error)) {
	bar := widget.NewProgressBar()
	label := widget.NewLabel(title + "...")
	progressDialog := dialog.NewCustomWithoutButtons(title, container.NewVBox(label, bar), u.window)
	progressDialog.Resize(fyne.NewSize(360, 0))
	progressDialog.Show()

	go func() {
		summary, changed, err := task(func(step string, fraction float64) {
			fyne.Do(func() {
				label.SetText(step)
				bar.SetValue(fraction)
			})
		})
		fyne.Do(func() {
			progressDialog.Hide()
			if changed || err != nil {
				// Files may have changed on disk even when a later step failed.
				u.reloadStore()
			}
			if err != nil {
				dialog.ShowError(err, u.window)
				return
			}
			u.setStatus(summary)
			u.notify(title, summary)
		})
	}()
}

// syncStore commits, pulls and pushes whatever the remote state requires.
func (u *ui) syncStore() {
	storeRoot := u.storePath
	u.runGitTask("Sync", func(progress func(string, float64)) (string, bool, error) {
		msg := fmt.Sprintf("Sync password store at %s using GPG Viewer.", time.Now().Format("2006-01-02 15:04"))
		res, err := gitsync.Sync(storeRoot, msg, progress)
		return "Sync: " + res.Summary(), res.Changed(), err
	})
}

// commitStore records all local changes in git without touching the remote.
func (u *ui) commitStore() {
	storeRoot := u.storePath
	u.runGitTask("Commit", func(progress func(string, float64)) (string, bool, error) {
		progress("Checking for changes...", 0.3)
		changed, err := gitsync.HasChanges(storeRoot)
		if err != nil {
			return "", false, err
		}
		if !changed {
			return "Nothing to commit", false, nil
		}
		progress("Committing...", 0.7)
		msg := fmt.Sprintf("Commit password store changes at %s using GPG Viewer.", time.Now().Format("2006-01-02 15:04"))
		if err := gitsync.CommitAll(storeRoot, msg); err != nil {
			return "", false, err
		}
		return "Changes committed to git", false, nil
	})
}

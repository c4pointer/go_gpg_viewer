package main

import (
	"errors"
	"fmt"
	"io/ioutil" // Note: In Go 1.16+, this is deprecated; use io and os packages instead
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"main.go/assets"
	scanpassstore "main.go/scanpassstore" // Adjust the import path according to your project structure
	"main.go/settings"
)

// Structure to hold password store data

// Structure to hold application state
type AppState struct {
	SelectedDirectory string
	SearchActive      bool
	SearchResults     []string // relative paths without .gpg, e.g., "Finance/bank"
}

// defaultRecipient is populated from settings and used to prefill recipient dialogs
var defaultRecipient string

// decryptAndEditFile handles the decryption and editing of a GPG file
func decryptAndEditFile(filePath string, window fyne.Window) {
	// Define the decryption function inline to avoid scope issues
	var decryptAndEdit func(string, string)
	decryptAndEdit = func(filePath string, passphrase string) {
		// Create a command to decrypt the GPG file
		var cmd *exec.Cmd
		if passphrase == "" {
			// Try to decrypt without passphrase (using GPG agent)
			cmd = exec.Command("gpg", "--batch", "--decrypt", filePath)
		} else {
			// Use provided passphrase
			cmd = exec.Command("gpg", "--batch", "--passphrase", passphrase, "--decrypt", filePath)
		}

		// Run the command and get the output
		output, err := cmd.CombinedOutput()
		if err != nil {
			// If this was a first attempt without passphrase, prompt for passphrase
			if passphrase == "" {
				// Show passphrase dialog
				fyne.Do(func() {
					passphraseEntry := widget.NewPasswordEntry()
					fileName := filepath.Base(filePath)

					passphraseDialog := dialog.NewCustomConfirm(
						"Enter Passphrase",
						"Decrypt",
						"Cancel",
						container.NewVBox(
							widget.NewLabel(fmt.Sprintf("File: %s", strings.TrimSuffix(fileName, ".gpg"))),
							widget.NewLabel("GPG agent requires passphrase. Please enter:"),
							passphraseEntry,
						),
						func(decrypt bool) {
							if decrypt {
								newPassphrase := passphraseEntry.Text
								if newPassphrase == "" {
									dialog.ShowError(errors.New("Passphrase cannot be empty"), window)
									return
								}

								// Try again with the provided passphrase
								go func() {
									decryptAndEdit(filePath, newPassphrase)
								}()
							}
						},
						window,
					)
					passphraseDialog.Show()
				})
				return
			} else {
				// This was already a passphrase attempt, show error
				fyne.Do(func() {
					dialog.ShowError(fmt.Errorf("Failed to decrypt file: %v\n%s", err, output), window)
				})
				return
			}
		}

		// Filter out GPG header information
		lines := strings.Split(string(output), "\n")
		var contentLines []string
		for _, line := range lines {
			// Skip lines that contain GPG header information
			if strings.HasPrefix(line, "gpg:") ||
				strings.Contains(line, "encrypted with") ||
				strings.Contains(line, "created") ||
				strings.Contains(line, "<c4point@gmail.com>") {
				continue
			}
			contentLines = append(contentLines, line)
		}

		// Join the filtered lines back together
		filteredContent := strings.Join(contentLines, "\n")

		// Create an entry widget with the filtered decrypted content
		contentEntry := widget.NewMultiLineEntry()
		contentEntry.SetText(filteredContent)

		// Create buttons first
		var editDialog *dialog.CustomDialog

		saveBtn := widget.NewButtonWithIcon("Save Changes", theme.DocumentSaveIcon(), func() {
			// Get the edited content
			editedContent := contentEntry.Text

			// Create a temporary file for the edited content
			tmpFile, err := ioutil.TempFile("", "gpg_edit_*")
			if err != nil {
				dialog.ShowError(fmt.Errorf("Failed to create temporary file: %v", err), window)
				return
			}
			tmpFileName := tmpFile.Name()

			// Write the edited content to the temporary file
			if _, err := tmpFile.WriteString(editedContent); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to write to temporary file: %v", err), window)
				os.Remove(tmpFileName)
				return
			}
			tmpFile.Close()

			// Get the recipient from the original file
			recipientCmd := exec.Command("gpg", "--batch", "--list-packets", filePath)
			recipientOutput, err := recipientCmd.CombinedOutput()
			if err != nil {
				dialog.ShowError(fmt.Errorf("Failed to get recipient info: %v\n%s", err, recipientOutput), window)
				return
			}

			// Parse the output to find the recipient
			recipientStr := string(recipientOutput)
			var recipient string

			// Look for keyid in the output
			for _, line := range strings.Split(recipientStr, "\n") {
				if strings.Contains(line, "keyid") {
					parts := strings.Split(line, "keyid")
					if len(parts) > 1 {
						keyidPart := strings.TrimSpace(parts[1])
						if len(keyidPart) > 16 { // Typical keyid length with some buffer
							recipient = keyidPart[:16]
							break
						}
					}
				}
			}

			if recipient == "" {
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
						if confirm {
							recipient = recipientEntry.Text
							if recipient == "" {
								dialog.ShowError(errors.New("Recipient cannot be empty"), window)
								return
							}

							// Now encrypt with the provided recipient
							cmd := exec.Command("gpg", "--batch", "--yes", "--recipient", recipient,
								"--output", filePath, "--encrypt", tmpFileName)

							output, err := cmd.CombinedOutput()
							// Clean up the temporary file
							os.Remove(tmpFileName)

							if err != nil {
								dialog.ShowError(fmt.Errorf("Failed to encrypt file: %v\n%s", err, output), window)
								return
							}

							dialog.ShowInformation("Success", "File saved successfully", window)
							if editDialog != nil {
								editDialog.Hide()
							}
						}
					},
					window,
				)
				recipientDialog.Show()
			} else {
				// Create a command to encrypt the edited content with the detected recipient
				cmd := exec.Command("gpg", "--batch", "--yes", "--recipient", recipient,
					"--output", filePath, "--encrypt", tmpFileName)

				output, err := cmd.CombinedOutput()
				// Clean up the temporary file
				os.Remove(tmpFileName)

				if err != nil {
					dialog.ShowError(fmt.Errorf("Failed to encrypt file: %v\n%s", err, output), window)
					return
				}

				dialog.ShowInformation("Success", "File saved successfully", window)
				if editDialog != nil {
					editDialog.Hide()
				}
			}
		})

		closeBtn := widget.NewButtonWithIcon("Close", theme.CancelIcon(), func() {
			if editDialog != nil {
				editDialog.Hide()
			}
		})

		// Create the dialog with content and buttons
		buttonContainer := container.NewHBox(saveBtn, closeBtn)
		contentContainer := container.NewBorder(nil, buttonContainer, nil, nil, contentEntry)
		editDialog = dialog.NewCustomWithoutButtons("Edit Password File", contentContainer, window)
		editDialog.Resize(fyne.NewSize(600, 400))
		editDialog.Show()
	}

	// Start the decryption process
	decryptAndEdit(filePath, "")
}

func main() {
	startTime := time.Now()
	defer func() {
		fmt.Println("Execution time:", time.Since(startTime))
	}()
	fmt.Println("Starting the program...")

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

	fmt.Println("Current user:", userCurrent.Username, "Home directory:", homeDir, "Target:", targetPath)

	// Check if the target directory exists
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		fmt.Println("Target directory does not exist:", targetPath)
		return
	}

	// Scan password store
	store, err := scanpassstore.ScanPasswordStore(targetPath)
	if err != nil {
		fmt.Println("Error scanning password store:", err)
		return
	}

	fmt.Println("Valid directories with .gpg files:", len(store.Directories))
	fmt.Println("Total root files:", len(store.RootFiles))
	fmt.Println("CLI scan completed successfully.")

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

	// Handle window resize to save size to settings
	myWindow.Canvas().SetOnTypedKey(func(ke *fyne.KeyEvent) {
		// This is a workaround to detect window resize
		// In a real implementation, you might want to use a timer-based approach
	})

	// Save window size when closing
	myWindow.SetOnClosed(func() {
		size := myWindow.Canvas().Size()
		settings.UpdateSettings(map[string]interface{}{
			"window_width":  int(size.Width),
			"window_height": int(size.Height),
		})
	})

	// Create application state
	appState := &AppState{
		SelectedDirectory: "",
	}

	// Create tree for directories with nested support
	tree := widget.NewTree(
		func(id widget.TreeNodeID) []widget.TreeNodeID {
			if id == "" {
				// Root level items
				if len(store.RootFiles) > 0 {
					return []widget.TreeNodeID{"Root", "Directories"}
				}
				return []widget.TreeNodeID{"Directories"}
			} else if id == "Root" {
				// Root files - show them as child nodes
				return store.RootFiles
			} else if id == "Directories" {
				// Directory names
				return store.Directories
			} else if files, ok := store.DirContents[id]; ok {
				// Files in a directory - show them as child nodes
				var children []widget.TreeNodeID
				children = append(children, files...)

				// Add subdirectories if they exist
				if subdirs, ok := store.NestedDirs[id]; ok {
					children = append(children, subdirs...)
				}
				return children
			} else {
				// Check if this is a root file (no parent directory)
				for _, rootFile := range store.RootFiles {
					if rootFile == id {
						return []widget.TreeNodeID{} // Root files are leaf nodes
					}
				}

				// Check if this is a subdirectory path (e.g., "Finance/subfolder1")
				// We need to check if this ID exists as a full path in DirContents
				if files, ok := store.DirContents[id]; ok {
					var children []widget.TreeNodeID
					children = append(children, files...)

					// Add nested subdirectories if they exist
					if subdirs, ok := store.NestedDirs[id]; ok {
						children = append(children, subdirs...)
					}
					return children
				}

				// Check if this is a subdirectory name that should be expanded
				// First, find which parent directory this subdirectory belongs to
				var parentDir string
				for parent, subdirs := range store.NestedDirs {
					for _, subdir := range subdirs {
						if subdir == id {
							parentDir = parent
							break
						}
					}
					if parentDir != "" {
						break
					}
				}

				// If we found the parent, construct the full path and get the contents
				if parentDir != "" {
					fullPath := parentDir + "/" + id
					if files, ok := store.DirContents[fullPath]; ok {
						return files
					}
				}

				// Fallback: Look for entries in DirContents that start with this ID + "/"
				var children []widget.TreeNodeID
				for fullPath := range store.DirContents {
					if strings.HasPrefix(fullPath, id+"/") {
						// Extract the subdirectory name
						parts := strings.Split(fullPath, "/")
						if len(parts) > 1 {
							subdirName := parts[1] // Get the part after the first "/"
							children = append(children, subdirName)
						}
					}
				}
				if len(children) > 0 {
					return children
				}
			}
			return []widget.TreeNodeID{}
		},
		func(id widget.TreeNodeID) bool {
			if id == "" || id == "Directories" {
				return true
			} else if id == "Root" && len(store.RootFiles) > 0 {
				return true
			} else if _, ok := store.NestedDirs[id]; ok {
				// Subdirectories are expandable if they have files or subdirectories
				return true
			} else if _, ok := store.DirContents[id]; ok {
				// Directories are expandable if they have files or subdirectories
				return true
			} else {
				// Check if this is a subdirectory by looking for entries that start with parent + "/"
				for fullPath := range store.DirContents {
					if strings.HasPrefix(fullPath, id+"/") {
						return true
					}
				}

				// Check if this is a subdirectory name that appears in any parent's NestedDirs
				for _, subdirs := range store.NestedDirs {
					for _, subdir := range subdirs {
						if subdir == id {
							return true
						}
					}
				}
			}
			// Files are not expandable
			return false
		},
		func(branch bool) fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.TreeNodeID, branch bool, o fyne.CanvasObject) {
			label := o.(*widget.Label)
			switch id {
			case "":
				label.SetText("Password Store")
			case "Root":
				label.SetText("Root Files")
			case "Directories":
				label.SetText("Directories")
			default:
				// Check if this is a directory, subdirectory, or file
				// First check if it's a subdirectory (in NestedDirs)
				if _, ok := store.NestedDirs[id]; ok {
					// It's a subdirectory
					dirName := id
					if strings.Contains(id, "/") {
						parts := strings.Split(id, "/")
						dirName = parts[len(parts)-1]
					}
					label.SetText("📂 " + dirName)
				} else if _, ok := store.DirContents[id]; ok {
					// It's a directory (has files)
					// Extract just the directory name from the path
					dirName := id
					if strings.Contains(id, "/") {
						parts := strings.Split(id, "/")
						dirName = parts[len(parts)-1]
					}

					// Check if it's a top-level directory or nested
					if strings.Contains(id, "/") {
						label.SetText("📂 " + dirName)
					} else {
						label.SetText("📁 " + dirName)
					}
				} else {
					// Check if this is a subdirectory by looking for entries that start with parent + "/"
					// This handles cases where we have "New" as a subdirectory of "Finance"
					isSubdirectory := false
					for fullPath := range store.DirContents {
						if strings.HasPrefix(fullPath, id+"/") {
							isSubdirectory = true
							break
						}
					}

					if isSubdirectory {
						label.SetText("📂 " + id)
					} else {
						// Check if this is a subdirectory name that appears in any parent's NestedDirs
						// This handles cases where "New" is a subdirectory of "Finance"
						for _, subdirs := range store.NestedDirs {
							for _, subdir := range subdirs {
								if subdir == id {
									label.SetText("📂 " + id)
									return
								}
							}
						}

						// It's a file
						label.SetText("📄 " + id)
					}
				}
			}
		},
	)

	// Content area for displaying selected items
	contentLabel := widget.NewLabel("Select an item to view details")
	contentLabel.Wrapping = fyne.TextWrapWord

	// File list for selected directory or search results
	fileList := widget.NewList(
		func() int { return 0 },
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			// This will be populated when a directory is selected
		},
	)

	// Search entry (global search across store)
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search passwords… (name or path)")
	searchEntry.OnChanged = func(query string) {
		q := strings.TrimSpace(strings.ToLower(query))
		if q == "" {
			// Exit search mode and restore selection-driven list
			appState.SearchActive = false
			appState.SearchResults = nil
			// Repopulate list from current selection
			if appState.SelectedDirectory != "" {
				tree.OnSelected(appState.SelectedDirectory)
			} else {
				fileList.Length = func() int { return 0 }
				fileList.Refresh()
				contentLabel.SetText("Select a directory or file to view details")
			}
			return
		}

		// Build list of all relative paths from store
		// Use AllPaths to traverse; compute relative path and trim .gpg
		var allRel []string
		sep := string(os.PathSeparator)
		prefix := targetPath + sep
		for _, full := range store.AllPaths {
			rel := strings.TrimPrefix(full, prefix)
			if strings.HasSuffix(rel, ".gpg") {
				rel = strings.TrimSuffix(rel, ".gpg")
			}
			allRel = append(allRel, rel)
		}

		// Filter
		var results []string
		for _, rel := range allRel {
			if strings.Contains(strings.ToLower(rel), q) {
				results = append(results, rel)
			}
		}

		// Update state and list
		appState.SearchActive = true
		appState.SearchResults = results
		fileList.Length = func() int { return len(appState.SearchResults) }
		fileList.UpdateItem = func(id widget.ListItemID, o fyne.CanvasObject) {
			label := o.(*widget.Label)
			label.SetText(appState.SearchResults[id])
		}
		fileList.Refresh()
		contentLabel.SetText(fmt.Sprintf("Found %d matching entr(y/ies)", len(results)))
	}

	// Handle tree selection
	tree.OnSelected = func(id widget.TreeNodeID) {
		// Store the selected directory in app state
		appState.SelectedDirectory = id

		if id == "Root" {
			// Show root files
			fileList.Length = func() int { return len(store.RootFiles) }
			fileList.UpdateItem = func(id widget.ListItemID, o fyne.CanvasObject) {
				label := o.(*widget.Label)
				label.SetText(store.RootFiles[id])
			}
			contentLabel.SetText(fmt.Sprintf("Root directory contains %d password files", len(store.RootFiles)))
		} else if files, ok := store.DirContents[id]; ok {
			// Show files in selected directory
			fileList.Length = func() int { return len(files) }
			fileList.UpdateItem = func(id widget.ListItemID, o fyne.CanvasObject) {
				label := o.(*widget.Label)
				label.SetText(files[id])
			}
			contentLabel.SetText(fmt.Sprintf("Directory '%s' contains %d password files", id, len(files)))
		} else {
			// Check if this is a file (either root file or file in directory)
			var fileName string

			// Check if it's a root file
			for _, rootFile := range store.RootFiles {
				if rootFile == id {
					fileName = rootFile
					break
				}
			}

			// If not a root file, check if it's a file in any directory
			if fileName == "" {
				for _, dirFiles := range store.DirContents {
					for _, dirFile := range dirFiles {
						if dirFile == id {
							fileName = dirFile
							break
						}
					}
					if fileName != "" {
						break
					}
				}
			}

			if fileName != "" {
				// This is a file, show it in the file list
				fileList.Length = func() int { return 1 }
				fileList.UpdateItem = func(id widget.ListItemID, o fyne.CanvasObject) {
					label := o.(*widget.Label)
					label.SetText(fileName)
				}
				contentLabel.SetText(fmt.Sprintf("Selected file: %s", fileName))

				// Automatically select the file for editing
				fileList.Select(0)

				// Directly trigger decryption for the selected file
				go func() {
					// Determine the file path based on whether it's a root file or directory file
					var filePath string

					// Check if it's a root file
					for _, rootFile := range store.RootFiles {
						if rootFile == id {
							filePath = filepath.Join(targetPath, fileName+".gpg")
							break
						}
					}

					// If not a root file, check if it's a file in any directory
					if filePath == "" {
						// Use the AllPaths map to get the full path
						if fullPath, ok := store.AllPaths[fileName]; ok {
							filePath = fullPath
						} else {
							// Fallback to old logic for backward compatibility
							for dirName, dirFiles := range store.DirContents {
								for _, dirFile := range dirFiles {
									if dirFile == id {
										filePath = filepath.Join(targetPath, dirName, fileName+".gpg")
										break
									}
								}
								if filePath != "" {
									break
								}
							}
						}
					}

					if filePath != "" {
						// Start the decryption process
						decryptAndEditFile(filePath, myWindow)
					}
				}()
			} else {
				// Reset file list for other selections
				fileList.Length = func() int { return 0 }
				contentLabel.SetText("Select a directory or file to view details")
			}
		}
		fileList.Refresh()
	}

	// Handle file selection
	fileList.OnSelected = func(id widget.ListItemID) {
		// If search is active, resolve selection directly by relative path
		if appState.SearchActive {
			if id < 0 || id >= len(appState.SearchResults) {
				return
			}
			rel := appState.SearchResults[id]
			// Build full path and open
			filePath := filepath.Join(targetPath, rel+".gpg")
			go decryptAndEditFile(filePath, myWindow)
			return
		}

		selectedDir := appState.SelectedDirectory
		var fileName string
		var filePath string

		if selectedDir == "Root" {
			fileName = store.RootFiles[id]
			filePath = filepath.Join(targetPath, fileName+".gpg")
		} else if files, ok := store.DirContents[selectedDir]; ok && id < len(files) {
			fileName = files[id]
			// Use AllPaths for nested directory support
			if fullPath, ok := store.AllPaths[fileName]; ok {
				filePath = fullPath
			} else {
				// Fallback to old logic
				filePath = filepath.Join(targetPath, selectedDir, fileName+".gpg")
			}
		}

		if fileName != "" {
			// Start the decryption process
			go func() {
				// Define the decryption function inline to avoid scope issues
				var decryptAndEdit func(string, string)
				decryptAndEdit = func(filePath string, passphrase string) {
					// Create a command to decrypt the GPG file
					var cmd *exec.Cmd
					if passphrase == "" {
						// Try to decrypt without passphrase (using GPG agent)
						cmd = exec.Command("gpg", "--batch", "--decrypt", filePath)
					} else {
						// Use provided passphrase
						cmd = exec.Command("gpg", "--batch", "--passphrase", passphrase, "--decrypt", filePath)
					}

					// Run the command and get the output
					output, err := cmd.CombinedOutput()
					if err != nil {
						// If this was a first attempt without passphrase, prompt for passphrase
						if passphrase == "" {
							// Show passphrase dialog
							fyne.Do(func() {
								passphraseEntry := widget.NewPasswordEntry()
								fileName := filepath.Base(filePath)

								passphraseDialog := dialog.NewCustomConfirm(
									"Enter Passphrase",
									"Decrypt",
									"Cancel",
									container.NewVBox(
										widget.NewLabel(fmt.Sprintf("File: %s", strings.TrimSuffix(fileName, ".gpg"))),
										widget.NewLabel("GPG agent requires passphrase. Please enter:"),
										passphraseEntry,
									),
									func(decrypt bool) {
										if decrypt {
											newPassphrase := passphraseEntry.Text
											if newPassphrase == "" {
												dialog.ShowError(errors.New("Passphrase cannot be empty"), myWindow)
												return
											}

											// Try again with the provided passphrase
											go func() {
												decryptAndEdit(filePath, newPassphrase)
											}()
										}
									},
									myWindow,
								)
								passphraseDialog.Show()
							})
							return
						} else {
							// This was already a passphrase attempt, show error
							fyne.Do(func() {
								dialog.ShowError(fmt.Errorf("Failed to decrypt file: %v\n%s", err, output), myWindow)
							})
							return
						}
					}

					// Filter out GPG header information
					lines := strings.Split(string(output), "\n")
					var contentLines []string
					for _, line := range lines {
						// Skip lines that contain GPG header information
						if strings.HasPrefix(line, "gpg:") ||
							strings.Contains(line, "encrypted with") ||
							strings.Contains(line, "created") ||
							strings.Contains(line, "<c4point@gmail.com>") {
							continue
						}
						contentLines = append(contentLines, line)
					}

					// Join the filtered lines back together
					filteredContent := strings.Join(contentLines, "\n")

					// Create an entry widget with the filtered decrypted content
					contentEntry := widget.NewMultiLineEntry()
					contentEntry.SetText(filteredContent)

					// Create buttons first
					var editDialog *dialog.CustomDialog

					saveBtn := widget.NewButton("Save Changes", func() {
						// Get the edited content
						editedContent := contentEntry.Text

						// Create a temporary file for the edited content
						tmpFile, err := ioutil.TempFile("", "gpg_edit_*")
						if err != nil {
							dialog.ShowError(fmt.Errorf("Failed to create temporary file: %v", err), myWindow)
							return
						}
						tmpFileName := tmpFile.Name()

						// Write the edited content to the temporary file
						if _, err := tmpFile.WriteString(editedContent); err != nil {
							dialog.ShowError(fmt.Errorf("Failed to write to temporary file: %v", err), myWindow)
							os.Remove(tmpFileName)
							return
						}
						tmpFile.Close()

						// Get the recipient from the original file
						recipientCmd := exec.Command("gpg", "--batch", "--list-packets", filePath)
						recipientOutput, err := recipientCmd.CombinedOutput()
						if err != nil {
							dialog.ShowError(fmt.Errorf("Failed to get recipient info: %v\n%s", err, recipientOutput), myWindow)
							return
						}

						// Parse the output to find the recipient
						recipientStr := string(recipientOutput)
						var recipient string

						// Look for keyid in the output
						for _, line := range strings.Split(recipientStr, "\n") {
							if strings.Contains(line, "keyid") {
								parts := strings.Split(line, "keyid")
								if len(parts) > 1 {
									keyidPart := strings.TrimSpace(parts[1])
									if len(keyidPart) > 16 { // Typical keyid length with some buffer
										recipient = keyidPart[:16]
										break
									}
								}
							}
						}

						if recipient == "" {
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
									if confirm {
										recipient = recipientEntry.Text
										if recipient == "" {
											dialog.ShowError(errors.New("Recipient cannot be empty"), myWindow)
											return
										}

										// Now encrypt with the provided recipient
										cmd := exec.Command("gpg", "--batch", "--yes", "--recipient", recipient,
											"--output", filePath, "--encrypt", tmpFileName)

										output, err := cmd.CombinedOutput()
										// Clean up the temporary file
										os.Remove(tmpFileName)

										if err != nil {
											dialog.ShowError(fmt.Errorf("Failed to encrypt file: %v\n%s", err, output), myWindow)
											return
										}

										dialog.ShowInformation("Success", "File saved successfully", myWindow)
										if editDialog != nil {
											editDialog.Hide()
										}
									}
								},
								myWindow,
							)
							recipientDialog.Show()
						} else {
							// Create a command to encrypt the edited content with the detected recipient
							cmd := exec.Command("gpg", "--batch", "--yes", "--recipient", recipient,
								"--output", filePath, "--encrypt", tmpFileName)

							output, err := cmd.CombinedOutput()
							// Clean up the temporary file
							os.Remove(tmpFileName)

							if err != nil {
								dialog.ShowError(fmt.Errorf("Failed to encrypt file: %v\n%s", err, output), myWindow)
								return
							}

							dialog.ShowInformation("Success", "File saved successfully", myWindow)
							if editDialog != nil {
								editDialog.Hide()
							}
						}
					})

					closeBtn := widget.NewButton("Close", func() {
						if editDialog != nil {
							editDialog.Hide()
						}
					})

					// Create the dialog with content and buttons
					buttonContainer := container.NewHBox(saveBtn, closeBtn)
					contentContainer := container.NewBorder(nil, buttonContainer, nil, nil, contentEntry)
					editDialog = dialog.NewCustomWithoutButtons("Edit Password File", contentContainer, myWindow)
					editDialog.Resize(fyne.NewSize(600, 400))
					editDialog.Show()
				}

				// Start the decryption process
				decryptAndEdit(filePath, "")
			}()
		}
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

	// Add a toolbar with actions
	toolbar := widget.NewToolbar(
		widget.NewToolbarAction(theme.ViewRefreshIcon(), func() {
			// Refresh the password store data
			store, err = scanpassstore.ScanPasswordStore(targetPath)
			if err != nil {
				dialog.ShowError(err, myWindow)
				return
			}
			tree.Refresh()
			fileList.Refresh()
			contentLabel.SetText("Password store refreshed")
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

				gitDir := targetPath
				execGitCommand := func(command string, args ...string) (string, error) {
					cmd := exec.Command(command, args...)
					cmd.Dir = gitDir
					output, err := cmd.CombinedOutput()
					return string(output), err
				}

				// Check if there are any changes to commit
				statusOutput, statusErr := execGitCommand("git", "status", "--porcelain")
				hasChanges := statusErr == nil && strings.TrimSpace(statusOutput) != ""

				if !hasChanges {
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

				// Add all changes
				_, addErr := execGitCommand("git", "add", ".")
				if addErr != nil {
					fyne.Do(func() {
						fyne.CurrentApp().SendNotification(&fyne.Notification{
							Title:   "Git Add Failed",
							Content: fmt.Sprintf("Error: %v", addErr),
						})
						progressDialog.Hide()
					})
					return
				}

				fyne.Do(func() {
					progressBar.SetValue(0.8)
				})

				// Commit with timestamp
				commitMsg := fmt.Sprintf("Manual commit: %s", time.Now().Format("2006-01-02 15:04:05"))
				_, commitErr := execGitCommand("git", "commit", "-m", commitMsg)
				if commitErr != nil {
					fyne.Do(func() {
						fyne.CurrentApp().SendNotification(&fyne.Notification{
							Title:   "Git Commit Failed",
							Content: fmt.Sprintf("Error: %v", commitErr),
						})
						progressDialog.Hide()
					})
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
				fyne.Do(func() {
					progressBar.SetValue(0.05)
				})

				gitDir := targetPath
				execGitCommand := func(command string, args ...string) (string, error) {
					cmd := exec.Command(command, args...)
					cmd.Dir = gitDir
					output, err := cmd.CombinedOutput()
					return string(output), err
				}

				// Check if there are any changes to commit
				statusOutput, statusErr := execGitCommand("git", "status", "--porcelain")
				hasChanges := statusErr == nil && strings.TrimSpace(statusOutput) != ""

				fyne.Do(func() {
					progressBar.SetValue(0.1)
				})

				// Fetch latest changes from remote
				_, fetchErr := execGitCommand("git", "fetch", "--all")
				if fetchErr != nil {
					fyne.Do(func() {
						fyne.CurrentApp().SendNotification(&fyne.Notification{
							Title:   "Git Fetch Failed",
							Content: fmt.Sprintf("Error: %v", fetchErr),
						})
						progressDialog.Hide()
					})
					return
				}

				fyne.Do(func() {
					progressBar.SetValue(0.3)
				})

				// Pull latest changes
				_, pullErr := execGitCommand("git", "pull", "--rebase")
				if pullErr != nil {
					fyne.Do(func() {
						fyne.CurrentApp().SendNotification(&fyne.Notification{
							Title:   "Git Pull Failed",
							Content: fmt.Sprintf("Error: %v", pullErr),
						})
						progressDialog.Hide()
					})
					return
				}

				// If there are local changes, commit them
				if hasChanges {
					fyne.Do(func() {
						progressBar.SetValue(0.5)
					})

					// Add all changes
					_, addErr := execGitCommand("git", "add", ".")
					if addErr != nil {
						fyne.Do(func() {
							fyne.CurrentApp().SendNotification(&fyne.Notification{
								Title:   "Git Add Failed",
								Content: fmt.Sprintf("Error: %v", addErr),
							})
							progressDialog.Hide()
						})
						return
					}

					// Commit with timestamp
					commitMsg := fmt.Sprintf("Auto-commit: %s", time.Now().Format("2006-01-02 15:04:05"))
					_, commitErr := execGitCommand("git", "commit", "-m", commitMsg)
					if commitErr != nil {
						fyne.Do(func() {
							fyne.CurrentApp().SendNotification(&fyne.Notification{
								Title:   "Git Commit Failed",
								Content: fmt.Sprintf("Error: %v", commitErr),
							})
							progressDialog.Hide()
						})
						return
					}
				}

				fyne.Do(func() {
					progressBar.SetValue(0.8)
				})

				// Push to remote
				_, pushErr := execGitCommand("git", "push")
				if pushErr != nil {
					fyne.Do(func() {
						fyne.CurrentApp().SendNotification(&fyne.Notification{
							Title:   "Git Push Failed",
							Content: fmt.Sprintf("Error: %v", pushErr),
						})
						progressDialog.Hide()
					})
					return
				}

				fyne.Do(func() {
					progressBar.SetValue(1.0)

					// Refresh the store data
					store, err = scanpassstore.ScanPasswordStore(targetPath)
					if err == nil {
						tree.Refresh()
						fileList.Refresh()
					}

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

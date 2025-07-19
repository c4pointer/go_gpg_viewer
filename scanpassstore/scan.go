package scanpassstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PasswordStore struct {
	RootPath    string
	RootFiles   []string
	Directories []string
	DirContents map[string][]string
	// New fields for nested structure
	NestedDirs map[string][]string // Maps directory path to its subdirectories
	AllPaths   map[string]string   // Maps display name to full path
}

// scanDirectory recursively scans a directory for .gpg files and subdirectories
func scanDirectory(dirPath string, relativePath string) ([]string, []string, error) {
	var files []string
	var subdirs []string

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading directory %s: %w", dirPath, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Recursively scan subdirectory
			subdirPath := filepath.Join(dirPath, entry.Name())
			subdirRelativePath := filepath.Join(relativePath, entry.Name())

			subdirFiles, subdirSubdirs, err := scanDirectory(subdirPath, subdirRelativePath)
			if err != nil {
				fmt.Printf("Error scanning subdirectory %s: %v\n", subdirPath, err)
				continue
			}

			// Only include subdirectory if it has .gpg files or contains subdirectories with .gpg files
			if len(subdirFiles) > 0 || len(subdirSubdirs) > 0 {
				subdirs = append(subdirs, entry.Name())
				// Don't flatten files from subdirectories into parent directory
				// Files will be handled separately for each directory level
			}
		} else if strings.HasSuffix(entry.Name(), ".gpg") {
			fileName := strings.TrimSuffix(entry.Name(), ".gpg")
			files = append(files, fileName)
		}
	}

	return files, subdirs, nil
}

func ScanPasswordStore(targetPath string) (*PasswordStore, error) {
	store := &PasswordStore{
		RootPath:    targetPath,
		DirContents: make(map[string][]string),
		NestedDirs:  make(map[string][]string),
		AllPaths:    make(map[string]string),
	}

	// Get files in root directory
	filesInRoot, err := os.ReadDir(targetPath)
	if err != nil {
		return nil, fmt.Errorf("error reading target directory: %w", err)
	}

	// Process root files
	for _, file := range filesInRoot {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".gpg") {
			fileName := strings.TrimSuffix(file.Name(), ".gpg")
			store.RootFiles = append(store.RootFiles, fileName)
			store.AllPaths[fileName] = filepath.Join(targetPath, fileName+".gpg")
		}
	}

	// Process directories recursively
	dirs, err := os.ReadDir(targetPath)
	if err != nil {
		return nil, fmt.Errorf("error reading target directory: %w", err)
	}

	for _, dir := range dirs {
		if dir.IsDir() {
			dirPath := filepath.Join(targetPath, dir.Name())

			// Recursively scan the directory
			files, subdirs, err := scanDirectory(dirPath, dir.Name())
			if err != nil {
				fmt.Printf("Error scanning directory %s: %v\n", dir.Name(), err)
				continue
			}

			// Only include directory if it has .gpg files or contains subdirectories with .gpg files
			if len(files) > 0 || len(subdirs) > 0 {
				store.Directories = append(store.Directories, dir.Name())
				store.DirContents[dir.Name()] = files
				store.NestedDirs[dir.Name()] = subdirs

				// Store full paths for files in this directory (not subdirectories)
				for _, file := range files {
					filePath := filepath.Join(dirPath, file+".gpg")
					store.AllPaths[file] = filePath
				}

				// Recursively process subdirectories to build complete structure
				store.processSubdirectories(dirPath, dir.Name(), subdirs)
			}
		}
	}

	return store, nil
}

// processSubdirectories recursively processes subdirectories to build the complete nested structure
func (store *PasswordStore) processSubdirectories(parentPath string, parentRelativePath string, subdirs []string) {
	for _, subdir := range subdirs {
		subdirPath := filepath.Join(parentPath, subdir)
		subdirRelativePath := filepath.Join(parentRelativePath, subdir)

		// Scan this subdirectory
		files, nestedSubdirs, err := scanDirectory(subdirPath, subdirRelativePath)
		if err != nil {
			fmt.Printf("Error scanning subdirectory %s: %v\n", subdirPath, err)
			continue
		}

		// Store the subdirectory's contents
		store.DirContents[subdirRelativePath] = files
		store.NestedDirs[subdirRelativePath] = nestedSubdirs

		// Store full paths for files in this subdirectory
		for _, file := range files {
			filePath := filepath.Join(subdirPath, file+".gpg")
			store.AllPaths[file] = filePath
		}

		// Recursively process nested subdirectories
		if len(nestedSubdirs) > 0 {
			store.processSubdirectories(subdirPath, subdirRelativePath, nestedSubdirs)
		}
	}
}

// findFilePath recursively searches for a .gpg file in the directory tree
func findFilePath(dirPath string, fileName string) string {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Recursively search in subdirectory
			subdirPath := filepath.Join(dirPath, entry.Name())
			if found := findFilePath(subdirPath, fileName); found != "" {
				return found
			}
		} else if strings.HasSuffix(entry.Name(), ".gpg") {
			entryFileName := strings.TrimSuffix(entry.Name(), ".gpg")
			if entryFileName == fileName {
				return filepath.Join(dirPath, entry.Name())
			}
		}
	}

	return ""
}

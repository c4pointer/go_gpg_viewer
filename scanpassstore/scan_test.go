package scanpassstore

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanPasswordStore(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "password_store_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test directory structure
	testStructure := map[string][]string{
		"":                    {"root1", "root2"}, // Root files
		"dir1":                {"file1", "file2"},
		"dir1/subdir1":        {"subfile1"},
		"dir2":                {"file3"},
		"dir2/subdir2":        {"subfile2"},
		"dir2/subdir2/nested": {"nestedfile"},
		"empty_dir":           {},
	}

	// Create the test structure
	for dir, files := range testStructure {
		dirPath := filepath.Join(tempDir, dir)
		if dir != "" {
			err := os.MkdirAll(dirPath, 0755)
			require.NoError(t, err)
		}

		for _, file := range files {
			filePath := filepath.Join(dirPath, file+".gpg")
			err := os.WriteFile(filePath, []byte("test content"), 0644)
			require.NoError(t, err)
		}
	}

	// Test scanning the password store
	store, err := ScanPasswordStore(tempDir)
	require.NoError(t, err)
	assert.NotNil(t, store)

	// Verify root files
	assert.Equal(t, tempDir, store.RootPath)
	assert.Len(t, store.RootFiles, 2)
	assert.Contains(t, store.RootFiles, "root1")
	assert.Contains(t, store.RootFiles, "root2")

	// Verify directories
	assert.Len(t, store.Directories, 2) // dir1 and dir2, empty_dir should be excluded
	assert.Contains(t, store.Directories, "dir1")
	assert.Contains(t, store.Directories, "dir2")

	// Verify directory contents
	assert.Len(t, store.DirContents["dir1"], 2)
	assert.Contains(t, store.DirContents["dir1"], "file1")
	assert.Contains(t, store.DirContents["dir1"], "file2")

	assert.Len(t, store.DirContents["dir2"], 1)
	assert.Contains(t, store.DirContents["dir2"], "file3")

	// Verify nested directories
	assert.Len(t, store.NestedDirs["dir1"], 1)
	assert.Contains(t, store.NestedDirs["dir1"], "subdir1")

	assert.Len(t, store.NestedDirs["dir2"], 1)
	assert.Contains(t, store.NestedDirs["dir2"], "subdir2")

	// Verify nested directory contents
	assert.Len(t, store.DirContents["dir1/subdir1"], 1)
	assert.Contains(t, store.DirContents["dir1/subdir1"], "subfile1")

	assert.Len(t, store.DirContents["dir2/subdir2"], 1)
	assert.Contains(t, store.DirContents["dir2/subdir2"], "subfile2")

	assert.Len(t, store.DirContents["dir2/subdir2/nested"], 1)
	assert.Contains(t, store.DirContents["dir2/subdir2/nested"], "nestedfile")

	// Verify all paths mapping
	expectedPaths := map[string]string{
		"root1":      filepath.Join(tempDir, "root1.gpg"),
		"root2":      filepath.Join(tempDir, "root2.gpg"),
		"file1":      filepath.Join(tempDir, "dir1", "file1.gpg"),
		"file2":      filepath.Join(tempDir, "dir1", "file2.gpg"),
		"file3":      filepath.Join(tempDir, "dir2", "file3.gpg"),
		"subfile1":   filepath.Join(tempDir, "dir1", "subdir1", "subfile1.gpg"),
		"subfile2":   filepath.Join(tempDir, "dir2", "subdir2", "subfile2.gpg"),
		"nestedfile": filepath.Join(tempDir, "dir2", "subdir2", "nested", "nestedfile.gpg"),
	}

	for expectedFile, expectedPath := range expectedPaths {
		actualPath, exists := store.AllPaths[expectedFile]
		assert.True(t, exists, "File %s should exist in AllPaths", expectedFile)
		assert.Equal(t, expectedPath, actualPath, "Path mismatch for file %s", expectedFile)
	}
}

func TestScanPasswordStoreEmptyDirectory(t *testing.T) {
	// Create an empty temporary directory
	tempDir, err := os.MkdirTemp("", "empty_password_store_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Test scanning empty directory
	store, err := ScanPasswordStore(tempDir)
	require.NoError(t, err)
	assert.NotNil(t, store)

	// Verify empty store
	assert.Equal(t, tempDir, store.RootPath)
	assert.Empty(t, store.RootFiles)
	assert.Empty(t, store.Directories)
	assert.Empty(t, store.DirContents)
	assert.Empty(t, store.NestedDirs)
	assert.Empty(t, store.AllPaths)
}

func TestScanPasswordStoreNonExistentDirectory(t *testing.T) {
	// Test scanning non-existent directory
	store, err := ScanPasswordStore("/non/existent/path")
	assert.Error(t, err)
	assert.Nil(t, store)
	assert.Contains(t, err.Error(), "error reading target directory")
}

func TestScanPasswordStoreWithNonGpgFiles(t *testing.T) {
	// Create a temporary directory with mixed file types
	tempDir, err := os.MkdirTemp("", "mixed_files_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create files with different extensions
	files := []string{
		"password1.gpg",
		"password2.gpg",
		"readme.txt",
		"config.yml",
		"backup.gpg.bak",
	}

	for _, file := range files {
		filePath := filepath.Join(tempDir, file)
		err := os.WriteFile(filePath, []byte("test content"), 0644)
		require.NoError(t, err)
	}

	// Test scanning
	store, err := ScanPasswordStore(tempDir)
	require.NoError(t, err)
	assert.NotNil(t, store)

	// Only .gpg files should be included
	assert.Len(t, store.RootFiles, 2)
	assert.Contains(t, store.RootFiles, "password1")
	assert.Contains(t, store.RootFiles, "password2")
}

func TestFindFilePath(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "find_file_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test structure
	dirs := []string{"dir1", "dir1/subdir1", "dir2"}
	for _, dir := range dirs {
		dirPath := filepath.Join(tempDir, dir)
		err := os.MkdirAll(dirPath, 0755)
		require.NoError(t, err)
	}

	// Create test files
	files := map[string]string{
		"dir1":         "file1.gpg",
		"dir1/subdir1": "file2.gpg",
		"dir2":         "file3.gpg",
	}

	for dir, file := range files {
		filePath := filepath.Join(tempDir, dir, file)
		err := os.WriteFile(filePath, []byte("test content"), 0644)
		require.NoError(t, err)
	}

	// Test finding files
	tests := []struct {
		name     string
		fileName string
		expected string
	}{
		{
			name:     "find file1",
			fileName: "file1",
			expected: filepath.Join(tempDir, "dir1", "file1.gpg"),
		},
		{
			name:     "find file2",
			fileName: "file2",
			expected: filepath.Join(tempDir, "dir1", "subdir1", "file2.gpg"),
		},
		{
			name:     "find file3",
			fileName: "file3",
			expected: filepath.Join(tempDir, "dir2", "file3.gpg"),
		},
		{
			name:     "find non-existent file",
			fileName: "nonexistent",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findFilePath(tempDir, tt.fileName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPasswordStoreStructure(t *testing.T) {
	// Test PasswordStore struct creation
	store := &PasswordStore{
		RootPath:    "/test/path",
		RootFiles:   []string{"file1", "file2"},
		Directories: []string{"dir1", "dir2"},
		DirContents: map[string][]string{
			"dir1": {"file3", "file4"},
			"dir2": {"file5"},
		},
		NestedDirs: map[string][]string{
			"dir1": {"subdir1"},
			"dir2": {"subdir2"},
		},
		AllPaths: map[string]string{
			"file1": "/test/path/file1.gpg",
			"file2": "/test/path/file2.gpg",
			"file3": "/test/path/dir1/file3.gpg",
		},
	}

	// Verify structure
	assert.Equal(t, "/test/path", store.RootPath)
	assert.Len(t, store.RootFiles, 2)
	assert.Len(t, store.Directories, 2)
	assert.Len(t, store.DirContents, 2)
	assert.Len(t, store.NestedDirs, 2)
	assert.Len(t, store.AllPaths, 3)
}

// Benchmark tests
func BenchmarkScanPasswordStore(b *testing.B) {
	// Create a test directory structure for benchmarking
	tempDir, err := os.MkdirTemp("", "benchmark_test")
	require.NoError(b, err)
	defer os.RemoveAll(tempDir)

	// Create a larger directory structure for benchmarking
	for i := 0; i < 100; i++ {
		dirPath := filepath.Join(tempDir, fmt.Sprintf("dir%d", i))
		err := os.MkdirAll(dirPath, 0755)
		require.NoError(b, err)

		for j := 0; j < 10; j++ {
			filePath := filepath.Join(dirPath, fmt.Sprintf("file%d.gpg", j))
			err := os.WriteFile(filePath, []byte("test content"), 0644)
			require.NoError(b, err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ScanPasswordStore(tempDir)
		require.NoError(b, err)
	}
}

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	scanpassstore "go_gpg_viewer/scanpassstore"
)

// newTestStore builds a store where "github" exists in two directories and
// "Work" has a nested subdirectory, the layout that broke base-name node IDs.
func newTestStore(t *testing.T) *scanpassstore.PasswordStore {
	t.Helper()
	root := t.TempDir()
	for _, rel := range []string{
		"email",
		"Personal/github",
		"Work/github",
		"Work/Cloud/aws",
	} {
		p := filepath.Join(root, filepath.FromSlash(rel)+".gpg")
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0700))
		require.NoError(t, os.WriteFile(p, []byte("x"), 0600))
	}
	store, err := scanpassstore.ScanPasswordStore(root)
	require.NoError(t, err)
	return store
}

func TestNodeChildren(t *testing.T) {
	store := newTestStore(t)

	assert.Equal(t, []string{nodeRootFiles, nodeDirectories}, nodeChildren(store, ""))
	assert.Equal(t, []string{recordNode("email")}, nodeChildren(store, nodeRootFiles))
	assert.Equal(t, []string{dirNode("Personal"), dirNode("Work")}, nodeChildren(store, nodeDirectories))
	assert.Equal(t, []string{
		recordNode(filepath.Join("Work", "github")),
		dirNode(filepath.Join("Work", "Cloud")),
	}, nodeChildren(store, dirNode("Work")))
	assert.Equal(t, []string{recordNode(filepath.Join("Work", "Cloud", "aws"))},
		nodeChildren(store, dirNode(filepath.Join("Work", "Cloud"))))
	assert.Empty(t, nodeChildren(store, recordNode("email")))
}

func TestNodeChildrenAreUnique(t *testing.T) {
	store := newTestStore(t)
	seen := map[string]bool{}
	var walk func(id string)
	walk = func(id string) {
		for _, child := range nodeChildren(store, id) {
			require.False(t, seen[child], "duplicate node ID %q", child)
			seen[child] = true
			walk(child)
		}
	}
	walk("")
	assert.True(t, seen[recordNode(filepath.Join("Personal", "github"))])
	assert.True(t, seen[recordNode(filepath.Join("Work", "github"))])
}

func TestNodeIsBranchAndLabel(t *testing.T) {
	assert.True(t, nodeIsBranch(""))
	assert.True(t, nodeIsBranch(nodeRootFiles))
	assert.True(t, nodeIsBranch(dirNode("Work")))
	assert.False(t, nodeIsBranch(recordNode("email")))

	assert.Equal(t, "Root Files", nodeLabel(nodeRootFiles))
	assert.Equal(t, "📁 Work", nodeLabel(dirNode("Work")))
	assert.Equal(t, "📂 Cloud", nodeLabel(dirNode(filepath.Join("Work", "Cloud"))))
	assert.Equal(t, "📄 aws", nodeLabel(recordNode(filepath.Join("Work", "Cloud", "aws"))))
}

func TestParentNode(t *testing.T) {
	assert.Equal(t, nodeRootFiles, parentNode("email"))
	assert.Equal(t, dirNode(filepath.Join("Work", "Cloud")), parentNode(filepath.Join("Work", "Cloud", "aws")))
}

func TestNodeListing(t *testing.T) {
	store := newTestStore(t)

	records, _ := nodeListing(store, nodeRootFiles)
	assert.Equal(t, []string{"email"}, records)

	records, _ = nodeListing(store, dirNode("Work"))
	assert.Equal(t, []string{filepath.Join("Work", "github")}, records)

	records, status := nodeListing(store, dirNode("Gone"))
	assert.Empty(t, records)
	assert.Contains(t, status, "no longer exists")

	records, _ = nodeListing(store, "")
	assert.Empty(t, records)
}

func TestSearchRecords(t *testing.T) {
	store := newTestStore(t)

	assert.Equal(t, []string{
		filepath.Join("Personal", "github"),
		filepath.Join("Work", "github"),
	}, searchRecords(store, "github"))
	assert.Equal(t, []string{filepath.Join("Work", "Cloud", "aws")}, searchRecords(store, "cloud"))
	assert.Empty(t, searchRecords(store, "nothing"))
}

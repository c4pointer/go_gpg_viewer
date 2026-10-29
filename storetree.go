package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	scanpassstore "go_gpg_viewer/scanpassstore"
)

// Tree node IDs must be unique across the whole tree, so directories and
// records are identified by their full path relative to the store root,
// prefixed with their kind. Bare base names are not enough: "Work/github"
// and "Personal/github" would collide and the wrong record could be opened.
// The empty ID is the (invisible) root of the tree and stands for the store
// root directory.
const (
	dirNodePrefix    = "dir:"
	recordNodePrefix = "rec:"
)

// dirNode returns the tree node ID of the directory rel (relative to the
// store root). The store root itself is the empty node.
func dirNode(rel string) string {
	if rel == "" || rel == "." {
		return ""
	}
	return dirNodePrefix + rel
}

// recordNode returns the tree node ID of the record rel (relative to the
// store root, without the .gpg extension).
func recordNode(rel string) string { return recordNodePrefix + rel }

// dirFromNode returns the relative directory path of a directory node; the
// root node maps to "".
func dirFromNode(id string) (string, bool) {
	if id == "" {
		return "", true
	}
	return strings.CutPrefix(id, dirNodePrefix)
}

// recordFromNode returns the relative record path of a record node.
func recordFromNode(id string) (string, bool) { return strings.CutPrefix(id, recordNodePrefix) }

// dirRecords returns the records directly inside dir as paths relative to
// the store root.
func dirRecords(store *scanpassstore.PasswordStore, dir string) []string {
	var names []string
	if dir == "" {
		names = store.RootFiles
	} else {
		names = store.DirContents[dir]
	}
	records := make([]string, 0, len(names))
	for _, name := range names {
		records = append(records, filepath.Join(dir, name))
	}
	return records
}

// nodeChildren returns the child node IDs of id within store: folders
// first, then records, each in store order.
func nodeChildren(store *scanpassstore.PasswordStore, id string) []string {
	dir, ok := dirFromNode(id)
	if !ok {
		return nil
	}
	var subdirs []string
	if dir == "" {
		subdirs = store.Directories
	} else {
		subdirs = store.NestedDirs[dir]
	}
	var children []string
	for _, sub := range subdirs {
		children = append(children, dirNode(filepath.Join(dir, sub)))
	}
	for _, rec := range dirRecords(store, dir) {
		children = append(children, recordNode(rec))
	}
	return children
}

// nodeIsBranch reports whether the node id can have children.
func nodeIsBranch(id string) bool {
	_, ok := dirFromNode(id)
	return ok
}

// nodeLabel returns the text shown for the node id in the tree.
func nodeLabel(id string) string {
	if dir, ok := dirFromNode(id); ok {
		if dir == "" {
			return "Password store"
		}
		return filepath.Base(dir)
	}
	if rec, ok := recordFromNode(id); ok {
		return filepath.Base(rec)
	}
	return id
}

// parentNode returns the tree node of the directory that holds the record
// rel: the root node for root records.
func parentNode(rel string) string {
	return dirNode(filepath.Dir(rel))
}

// nodeListing returns the records (relative paths without .gpg) shown in the
// record list when the tree node id is selected, plus a heading for it.
func nodeListing(store *scanpassstore.PasswordStore, id string) (records []string, heading string) {
	dir, ok := dirFromNode(id)
	if !ok {
		return nil, ""
	}
	if dir == "" {
		return dirRecords(store, ""), "Password store"
	}
	if _, exists := store.DirContents[dir]; !exists {
		return nil, fmt.Sprintf("%s (folder no longer exists)", dir)
	}
	return dirRecords(store, dir), dir
}

// searchRecords returns, sorted, every record whose relative path contains
// query (case-insensitive). query must already be lowercased.
func searchRecords(store *scanpassstore.PasswordStore, query string) []string {
	var results []string
	for rel := range store.AllPaths {
		if strings.Contains(strings.ToLower(rel), query) {
			results = append(results, rel)
		}
	}
	sort.Strings(results)
	return results
}

// countRecords returns how many records the store holds.
func countRecords(store *scanpassstore.PasswordStore) int {
	return len(store.AllPaths)
}

// plural returns "n word" or "n words".
func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

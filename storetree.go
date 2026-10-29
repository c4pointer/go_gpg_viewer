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
const (
	nodeRootFiles    = "Root"
	nodeDirectories  = "Directories"
	dirNodePrefix    = "dir:"
	recordNodePrefix = "rec:"
)

// dirNode returns the tree node ID of the directory rel (relative to the store root).
func dirNode(rel string) string { return dirNodePrefix + rel }

// recordNode returns the tree node ID of the record rel (relative to the
// store root, without the .gpg extension).
func recordNode(rel string) string { return recordNodePrefix + rel }

// dirFromNode returns the relative directory path of a directory node.
func dirFromNode(id string) (string, bool) { return strings.CutPrefix(id, dirNodePrefix) }

// recordFromNode returns the relative record path of a record node.
func recordFromNode(id string) (string, bool) { return strings.CutPrefix(id, recordNodePrefix) }

// nodeChildren returns the child node IDs of id within store.
func nodeChildren(store *scanpassstore.PasswordStore, id string) []string {
	switch id {
	case "":
		var children []string
		if len(store.RootFiles) > 0 {
			children = append(children, nodeRootFiles)
		}
		return append(children, nodeDirectories)
	case nodeRootFiles:
		children := make([]string, 0, len(store.RootFiles))
		for _, name := range store.RootFiles {
			children = append(children, recordNode(name))
		}
		return children
	case nodeDirectories:
		children := make([]string, 0, len(store.Directories))
		for _, dir := range store.Directories {
			children = append(children, dirNode(dir))
		}
		return children
	}

	dir, ok := dirFromNode(id)
	if !ok {
		return nil
	}
	var children []string
	for _, name := range store.DirContents[dir] {
		children = append(children, recordNode(filepath.Join(dir, name)))
	}
	for _, sub := range store.NestedDirs[dir] {
		children = append(children, dirNode(filepath.Join(dir, sub)))
	}
	return children
}

// nodeIsBranch reports whether the node id can have children.
func nodeIsBranch(id string) bool {
	if id == "" || id == nodeRootFiles || id == nodeDirectories {
		return true
	}
	_, ok := dirFromNode(id)
	return ok
}

// nodeLabel returns the text shown for the node id in the tree.
func nodeLabel(id string) string {
	switch id {
	case "":
		return "Password Store"
	case nodeRootFiles:
		return "Root Files"
	case nodeDirectories:
		return "Directories"
	}
	if dir, ok := dirFromNode(id); ok {
		if strings.ContainsRune(dir, filepath.Separator) {
			return "📂 " + filepath.Base(dir)
		}
		return "📁 " + dir
	}
	if rec, ok := recordFromNode(id); ok {
		return "📄 " + filepath.Base(rec)
	}
	return id
}

// parentNode returns the tree node that lists the record rel: the
// "Root Files" node for root records, otherwise its directory node.
func parentNode(rel string) string {
	if dir := filepath.Dir(rel); dir != "." {
		return dirNode(dir)
	}
	return nodeRootFiles
}

// nodeListing returns the records (relative paths without .gpg) shown in the
// record list when the tree node id is selected, plus a status line.
func nodeListing(store *scanpassstore.PasswordStore, id string) ([]string, string) {
	if id == nodeRootFiles {
		records := append([]string(nil), store.RootFiles...)
		return records, fmt.Sprintf("Root directory contains %d password files", len(records))
	}
	dir, ok := dirFromNode(id)
	if !ok {
		return nil, "Select a directory or file to view details"
	}
	files, ok := store.DirContents[dir]
	if !ok {
		return nil, fmt.Sprintf("Directory '%s' no longer exists", dir)
	}
	records := make([]string, 0, len(files))
	for _, name := range files {
		records = append(records, filepath.Join(dir, name))
	}
	return records, fmt.Sprintf("Directory '%s' contains %d password files", dir, len(records))
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

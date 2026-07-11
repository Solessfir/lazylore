package ui

import (
	"sort"
	"strings"

	"lazylore/internal/lore"
)

// fileTreeNode is one node in the directory tree built from a lore.Status's
// changed files - either a directory (isDir, with children) or a file leaf
// (carries the FileChange it represents). This only ever covers changed
// paths, never the whole repository, mirroring lazygit's own Files panel
// (pkg/gui/presentation/files.go): it's a tree of what's changed, not a
// repo browser.
type fileTreeNode struct {
	name     string
	path     string // full path from the repo root; "" for the synthetic root
	isDir    bool
	children []*fileTreeNode
	change   lore.FileChange
	staged   bool
}

// buildFileTree groups a Status's staged and unstaged files into a
// directory tree. A path that appears in both Staged and Unstaged (a file
// with further edits on top of what's already staged) produces two
// separate leaf nodes under the same parent, same as the flat list did -
// staged and unstaged are always distinct rows.
func buildFileTree(s lore.Status) *fileTreeNode {
	root := &fileTreeNode{isDir: true}

	insert := func(change lore.FileChange, staged bool) {
		parts := strings.Split(change.Path, "/")
		current := root
		for i, part := range parts {
			if i == len(parts)-1 {
				current.children = append(current.children, &fileTreeNode{
					name: part, path: change.Path, staged: staged, change: change,
				})
				return
			}

			var dir *fileTreeNode
			for _, c := range current.children {
				if c.isDir && c.name == part {
					dir = c
					break
				}
			}
			if dir == nil {
				dirPath := part
				if current.path != "" {
					dirPath = current.path + "/" + part
				}
				dir = &fileTreeNode{name: part, path: dirPath, isDir: true}
				current.children = append(current.children, dir)
			}
			current = dir
		}
	}

	for _, c := range s.Staged {
		insert(c, true)
	}
	for _, c := range s.Unstaged {
		insert(c, false)
	}

	sortFileTree(root)
	return root
}

func sortFileTree(node *fileTreeNode) {
	sort.Slice(node.children, func(i, j int) bool {
		return node.children[i].name < node.children[j].name
	})
	for _, c := range node.children {
		if c.isDir {
			sortFileTree(c)
		}
	}
}

// fileTreeRow is one visible row once a tree is flattened for display: a
// node plus how deep it sits, for indentation.
type fileTreeRow struct {
	node  *fileTreeNode
	depth int
}

// flattenFileTree walks the tree depth-first into the ordered rows a list
// widget actually renders, skipping the children of any directory whose
// path is in collapsed - mirrors lazygit's renderAux/CollapsedPaths.
func flattenFileTree(root *fileTreeNode, collapsed map[string]bool) []fileTreeRow {
	var rows []fileTreeRow
	var walk func(node *fileTreeNode, depth int)
	walk = func(node *fileTreeNode, depth int) {
		for _, child := range node.children {
			rows = append(rows, fileTreeRow{node: child, depth: depth})
			if child.isDir && !collapsed[child.path] {
				walk(child, depth+1)
			}
		}
	}
	walk(root, 0)
	return rows
}

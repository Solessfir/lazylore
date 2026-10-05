package ui

import (
	"sort"
	"strings"

	"github.com/solessfir/lazylore/internal/lore"
)

// fileTreeNode is one node in the directory tree built from a lore.Status's
// changed files - either a directory (isDir, with children) or a file leaf
// (carries the FileChange it represents). Only covers changed paths, never
// the whole repository.
//
// path is always the node's full path from the repo root ("" only for the
// synthetic "/" root node) - unaffected by compressFileTree below, so
// collapse state and lock/status lookups keyed by path keep working
// regardless of where in the (compressed) tree a node ends up rendered.
type fileTreeNode struct {
	name      string
	path      string
	isDir     bool
	children  []*fileTreeNode
	change    lore.FileChange
	hasChange bool
	staged    bool
	// allStaged is only meaningful for a directory node: true when every
	// file anywhere in its subtree is staged (set by computeAllStaged),
	// so a directory row can render green once everything under it is staged.
	allStaged bool
}

// buildFileTree groups a Status's staged and unstaged files into a
// directory tree, wrapped in a synthetic "/" root and compressed: a chain
// of directories that each have exactly one subdirectory collapses into a
// single row (e.g. "Content/Sus/Blueprints" instead of three separate
// rows), and the root row itself collapses away the same way whenever the
// whole tree boils down to one top-level entry.
func buildFileTree(s lore.Status) *fileTreeNode {
	displayRoot := &fileTreeNode{isDir: true} // the "/" row; path == "" marks it

	insert := func(change lore.FileChange, staged bool) {
		if change.Path == "" || strings.HasSuffix(change.Path, "/") {
			// A trailing slash (or an outright empty path) has no filename
			// segment to show - strings.Split would otherwise produce an
			// empty last part and insert a blank-labeled leaf row.
			return
		}
		parts := strings.Split(change.Path, "/")
		current := displayRoot
		for i, part := range parts {
			if i == len(parts)-1 {
				if change.Directory {
					for _, child := range current.children {
						if child.isDir && child.name == part {
							child.change = change
							child.hasChange = true
							child.staged = staged
							return
						}
					}

					current.children = append(current.children, &fileTreeNode{
						name:      part,
						path:      change.Path,
						isDir:     true,
						change:    change,
						hasChange: true,
						staged:    staged,
					})
					return
				}

				current.children = append(current.children, &fileTreeNode{
					name:      part,
					path:      change.Path,
					change:    change,
					hasChange: true,
					staged:    staged,
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

	hiddenTop := &fileTreeNode{isDir: true}
	if len(displayRoot.children) == 0 {
		return hiddenTop
	}

	sortFileTree(displayRoot)
	hiddenTop.children = []*fileTreeNode{displayRoot}
	compressFileTree(hiddenTop)

	// compressFileTree only hoists directory chains - a lone file at repo
	// root needs its own check to skip the "/" wrapper too.
	if root := hiddenTop.children[0]; root.path == "" && len(root.children) == 1 {
		hiddenTop.children[0] = root.children[0]
	}
	computeAllStaged(hiddenTop)
	return hiddenTop
}

// computeAllStaged fills in every directory node's allStaged bottom-up:
// true only when the subtree contains at least one change and every change
// in it is staged. Returns (allStaged, sawChange) for node itself so a parent
// call can fold a child directory's result in without re-walking it.
func computeAllStaged(node *fileTreeNode) (allStaged, sawChange bool) {
	if !node.isDir {
		return node.staged, true
	}
	allStaged = !node.hasChange || node.staged
	sawChange = node.hasChange
	for _, c := range node.children {
		childAllStaged, childSawChange := computeAllStaged(c)
		if childSawChange {
			sawChange = true
			if !childAllStaged {
				allStaged = false
			}
		}
	}
	if !sawChange {
		allStaged = false
	}
	node.allStaged = allStaged
	return allStaged, sawChange
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

// compressFileTree collapses directory chains: a directory with exactly one
// subdirectory is replaced in-place by that subdirectory, repeating until
// the chain ends at a file or a directory with more than one child. Run
// from hiddenTop (never itself rendered) so the synthetic "/" root
// participates too.
func compressFileTree(node *fileTreeNode) *fileTreeNode {
	if !node.isDir {
		return node
	}
	children := node.children
	for i := range children {
		grandchildren := children[i].children
		for !children[i].hasChange && len(grandchildren) == 1 && grandchildren[0].isDir {
			children[i] = grandchildren[0]
			grandchildren = children[i].children
		}
	}
	for i := range children {
		children[i] = compressFileTree(children[i])
	}
	node.children = children
	return node
}

// fileTreeRow is one visible row once a tree is flattened for display: a
// node, how deep it sits (for indentation), and its label - the path
// segments since the nearest rendered ancestor, which is just the node's
// own name in the common case but the full merged remainder for a row a
// directory chain compressed into (e.g. "Content/Sus/Blueprints").
type fileTreeRow struct {
	node  *fileTreeNode
	depth int
	label string
}

// flattenFileTree walks the tree depth-first into the ordered rows a list
// widget actually renders, skipping the children of any directory whose
// path is in collapsed. Starts from hiddenTop's children so the wrapper
// node buildFileTree returns is never rendered as a row of its own.
func flattenFileTree(hiddenTop *fileTreeNode, collapsed map[string]bool) []fileTreeRow {
	var rows []fileTreeRow
	var walk func(node *fileTreeNode, depth int, parentPath string)
	walk = func(node *fileTreeNode, depth int, parentPath string) {
		for _, child := range node.children {
			rows = append(rows, fileTreeRow{node: child, depth: depth, label: rowLabel(child, parentPath)})
			if child.isDir && !collapsed[child.path] {
				walk(child, depth+1, child.path)
			}
		}
	}
	walk(hiddenTop, 0, "")
	return rows
}

// rowLabel is the text a row displays: "/" for the synthetic root, the full
// path for anything directly under it (nothing rendered above it yet to
// trim against), or just the remainder since the nearest rendered ancestor
// otherwise - so a compressed chain like "Content/Sus/Blueprints" shows its
// whole merged name, while a plain child of that row (once expanded) shows
// only its own segment.
func rowLabel(node *fileTreeNode, parentPath string) string {
	if node.path == "" {
		return "/"
	}
	if parentPath == "" {
		return node.path
	}
	return strings.TrimPrefix(node.path, parentPath+"/")
}

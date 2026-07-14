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
//
// path is always the node's full path from the repo root ("" only for the
// synthetic "/" root node) - unaffected by compressFileTree below, so
// collapse state and lock/status lookups keyed by path keep working
// regardless of where in the (compressed) tree a node ends up rendered.
type fileTreeNode struct {
	name     string
	path     string
	isDir    bool
	children []*fileTreeNode
	change   lore.FileChange
	staged   bool
}

// buildFileTree groups a Status's staged and unstaged files into a
// directory tree, wrapped in a synthetic "/" root and compressed the way
// lazygit's own tree does (pkg/gui/filetree/build_tree.go + node.go's
// compressAux): a chain of directories that each have exactly one
// subdirectory collapses into a single row (e.g. "Content/Sus/Blueprints"
// instead of three separate rows), and the root row itself collapses away
// the same way whenever the whole tree boils down to one top-level entry -
// matching lazygit's own root-collapsing behavior rather than always
// showing a "/" row that would just add noise for the common case. A path
// that appears in both Staged and Unstaged (a file with further edits on
// top of what's already staged) produces two separate leaf nodes under the
// same parent, same as the flat list did - staged and unstaged are always
// distinct rows.
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

	hiddenTop := &fileTreeNode{isDir: true}
	if len(displayRoot.children) == 0 {
		return hiddenTop
	}

	sortFileTree(displayRoot)
	hiddenTop.children = []*fileTreeNode{displayRoot}
	compressFileTree(hiddenTop)

	// compressFileTree's chain-collapse only hoists directories (a lone
	// file isn't a chain to collapse through), so the single-file-at-
	// repo-root case - nothing else changed, one plain file at the top -
	// needs its own check: still worth skipping the "/" wrapper for one
	// file, same as it is for one directory chain.
	if root := hiddenTop.children[0]; root.path == "" && len(root.children) == 1 {
		hiddenTop.children[0] = root.children[0]
	}
	return hiddenTop
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

// compressFileTree collapses directory chains the way lazygit's node.go
// compressAux does: a directory with exactly one subdirectory is replaced
// in-place by that subdirectory, repeating until the chain ends at either a
// file or a directory with more than one child. Run from hiddenTop (a
// wrapper never itself rendered - see flattenFileTree) so the synthetic "/"
// root participates in the same rule: it collapses away too when the whole
// tree has only one top-level directory in it.
func compressFileTree(node *fileTreeNode) *fileTreeNode {
	if !node.isDir {
		return node
	}
	children := node.children
	for i := range children {
		grandchildren := children[i].children
		for len(grandchildren) == 1 && grandchildren[0].isDir {
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
// path is in collapsed - mirrors lazygit's renderAux/CollapsedPaths. Starts
// from hiddenTop's children rather than hiddenTop itself, so the wrapper
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

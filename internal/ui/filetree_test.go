package ui

import (
	"testing"

	"lazylore/internal/lore"
)

func TestBuildFileTree_EmptyStatusHasNoRows(t *testing.T) {
	tree := buildFileTree(lore.Status{})
	rows := flattenFileTree(tree, nil)
	if len(rows) != 0 {
		t.Fatalf("rows = %+v, want none for an empty status", rows)
	}
}

func TestBuildFileTree_PreservesChangedEmptyDirectory(t *testing.T) {
	s := lore.Status{Unstaged: []lore.FileChange{{Status: 'A', Path: "Content/Empty", Directory: true}}}
	rows := flattenFileTree(buildFileTree(s), nil)
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want one actionable directory row", rows)
	}
	if !rows[0].node.isDir || !rows[0].node.hasChange || rows[0].node.path != "Content/Empty" {
		t.Fatalf("rows[0] = %+v, want changed directory Content/Empty", rows[0])
	}
}

func TestBuildFileTree_SkipsPathsWithNoFilenameSegment(t *testing.T) {
	// A trailing slash (or an outright empty path) makes strings.Split
	// produce an empty last segment, which would insert a blank-labeled leaf row.
	s := lore.Status{Unstaged: []lore.FileChange{
		{Status: 'M', Path: "a.txt"},
		{Status: 'A', Path: "Resources/"},
		{Status: 'A', Path: ""},
	}}
	tree := buildFileTree(s)
	rows := flattenFileTree(tree, nil)
	if len(rows) != 1 || rows[0].label != "a.txt" {
		t.Fatalf("rows = %+v, want just a.txt - the malformed paths should be skipped", rows)
	}
}

func TestBuildFileTree_DirRowIsAllStagedOnlyWhenEveryDescendantFileIsStaged(t *testing.T) {
	s := lore.Status{
		Staged:   []lore.FileChange{{Status: 'M', Path: "src/a.go"}},
		Unstaged: []lore.FileChange{{Status: 'M', Path: "src/b.go"}},
	}
	tree := buildFileTree(s)
	rows := flattenFileTree(tree, nil)
	var dirRow *fileTreeRow
	for i := range rows {
		if rows[i].node.isDir {
			dirRow = &rows[i]
		}
	}
	if dirRow == nil {
		t.Fatal("expected a 'src' directory row")
	}
	if dirRow.node.allStaged {
		t.Fatal("directory should not be allStaged while one child is still unstaged")
	}

	s2 := lore.Status{Staged: []lore.FileChange{
		{Status: 'M', Path: "src/a.go"},
		{Status: 'M', Path: "src/b.go"},
	}}
	tree2 := buildFileTree(s2)
	rows2 := flattenFileTree(tree2, nil)
	dirRow = nil
	for i := range rows2 {
		if rows2[i].node.isDir {
			dirRow = &rows2[i]
		}
	}
	if dirRow == nil {
		t.Fatal("expected a 'src' directory row")
	}
	if !dirRow.node.allStaged {
		t.Fatal("directory should be allStaged once every file under it is staged")
	}
}

func TestBuildFileTree_SingleTopLevelFileSkipsRootRow(t *testing.T) {
	s := lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}
	tree := buildFileTree(s)
	rows := flattenFileTree(tree, nil)
	if len(rows) != 1 || rows[0].label != "a.txt" || rows[0].depth != 0 {
		t.Fatalf("rows = %+v, want a single a.txt row at depth 0, no root wrapper", rows)
	}
	if rows[0].node.isDir {
		t.Fatalf("rows[0] = %+v, want a.txt as a file leaf, not a directory", rows[0])
	}
}

func TestBuildFileTree_SingleTopLevelDirectoryChainSkipsRootRow(t *testing.T) {
	s := lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "src/a.go"}}}
	tree := buildFileTree(s)
	rows := flattenFileTree(tree, nil)
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want 2 (src/, src/a.go)", rows)
	}
	if rows[0].label != "src" || rows[0].depth != 0 || !rows[0].node.isDir {
		t.Fatalf("rows[0] = %+v, want the 'src' directory at depth 0", rows[0])
	}
	if rows[1].label != "a.go" || rows[1].depth != 1 {
		t.Fatalf("rows[1] = %+v, want a.go at depth 1", rows[1])
	}
}

func TestBuildFileTree_MultipleTopLevelEntriesShowRootRow(t *testing.T) {
	s := lore.Status{Unstaged: []lore.FileChange{
		{Status: 'M', Path: "a.txt"},
		{Status: 'M', Path: "b.txt"},
	}}
	tree := buildFileTree(s)
	rows := flattenFileTree(tree, nil)
	if len(rows) != 3 {
		t.Fatalf("rows = %+v, want 3 (/, a.txt, b.txt)", rows)
	}
	if rows[0].label != "/" || rows[0].depth != 0 || !rows[0].node.isDir {
		t.Fatalf("rows[0] = %+v, want the root '/' row at depth 0", rows[0])
	}
	if rows[1].label != "a.txt" || rows[1].depth != 1 {
		t.Fatalf("rows[1] = %+v, want a.txt at depth 1 under root", rows[1])
	}
	if rows[2].label != "b.txt" || rows[2].depth != 1 {
		t.Fatalf("rows[2] = %+v, want b.txt at depth 1 under root", rows[2])
	}
}

func TestBuildFileTree_ChainOfSingleChildDirectoriesCompressesIntoOneRow(t *testing.T) {
	s := lore.Status{Unstaged: []lore.FileChange{
		{Status: 'M', Path: "Content/Sus/Blueprints/BP_PlayerController.uasset"},
	}}
	tree := buildFileTree(s)
	rows := flattenFileTree(tree, nil)
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want 2 (the merged chain row, then the file)", rows)
	}
	if rows[0].label != "Content/Sus/Blueprints" || rows[0].depth != 0 {
		t.Fatalf("rows[0] = %+v, want the merged 'Content/Sus/Blueprints' row at depth 0", rows[0])
	}
	if rows[0].node.path != "Content/Sus/Blueprints" {
		t.Fatalf("rows[0].node.path = %q, want the full merged path so collapse toggling still works", rows[0].node.path)
	}
	if rows[1].label != "BP_PlayerController.uasset" || rows[1].depth != 1 {
		t.Fatalf("rows[1] = %+v, want the file at depth 1, showing just its own name", rows[1])
	}
}

func TestBuildFileTree_ChainStopsAtADirectoryWithMultipleChildren(t *testing.T) {
	s := lore.Status{Unstaged: []lore.FileChange{
		{Status: 'M', Path: "Plugins/LoreSourceControl/Config/FilterPlugin.ini"},
		{Status: 'M', Path: "Plugins/LoreSourceControl/README.md"},
	}}
	tree := buildFileTree(s)
	rows := flattenFileTree(tree, nil)
	// Plugins/ has one child (LoreSourceControl/) so it merges with it, but
	// LoreSourceControl/ itself has two children (Config/, README.md) so the
	// chain stops there instead of also swallowing Config/.
	if len(rows) != 4 {
		t.Fatalf("rows = %+v, want 4 (Plugins/LoreSourceControl, Config, FilterPlugin.ini, README.md)", rows)
	}
	if rows[0].label != "Plugins/LoreSourceControl" || rows[0].depth != 0 {
		t.Fatalf("rows[0] = %+v, want the merged 'Plugins/LoreSourceControl' row", rows[0])
	}
	if rows[1].label != "Config" || rows[1].depth != 1 || !rows[1].node.isDir {
		t.Fatalf("rows[1] = %+v, want the 'Config' directory at depth 1 (not merged further)", rows[1])
	}
	if rows[2].label != "FilterPlugin.ini" || rows[2].depth != 2 {
		t.Fatalf("rows[2] = %+v, want FilterPlugin.ini at depth 2", rows[2])
	}
	if rows[3].label != "README.md" || rows[3].depth != 1 {
		t.Fatalf("rows[3] = %+v, want README.md at depth 1", rows[3])
	}
}

func TestBuildFileTree_SamePathStagedAndUnstagedAreTwoLeaves(t *testing.T) {
	s := lore.Status{
		Staged:   []lore.FileChange{{Status: 'M', Path: "a.txt"}},
		Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}},
	}
	tree := buildFileTree(s)
	rows := flattenFileTree(tree, nil)
	if len(rows) != 3 {
		t.Fatalf("rows = %+v, want 3 (/, staged a.txt, unstaged a.txt)", rows)
	}
	if rows[1].node.staged == rows[2].node.staged {
		t.Fatalf("expected one staged and one unstaged a.txt leaf, got %+v", rows[1:])
	}
}

func TestBuildFileTree_ChildrenSortedAlphabetically(t *testing.T) {
	s := lore.Status{Unstaged: []lore.FileChange{
		{Status: 'M', Path: "z.txt"},
		{Status: 'M', Path: "a.txt"},
	}}
	tree := buildFileTree(s)
	rows := flattenFileTree(tree, nil)
	if rows[1].label != "a.txt" || rows[2].label != "z.txt" {
		t.Fatalf("rows = %+v, want a.txt before z.txt", rows)
	}
}

func TestFlattenFileTree_CollapsedDirectoryHidesChildren(t *testing.T) {
	s := lore.Status{Unstaged: []lore.FileChange{
		{Status: 'M', Path: "src/a.go"},
		{Status: 'M', Path: "src/b.go"},
	}}
	tree := buildFileTree(s)
	rows := flattenFileTree(tree, map[string]bool{"src": true})

	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want just the collapsed 'src' directory row", rows)
	}
	if !rows[0].node.isDir || rows[0].node.path != "src" {
		t.Fatalf("rows[0] = %+v, want the src directory", rows[0])
	}
}

func TestFlattenFileTree_CollapsingRootHidesEverything(t *testing.T) {
	s := lore.Status{Unstaged: []lore.FileChange{
		{Status: 'M', Path: "a.txt"},
		{Status: 'M', Path: "b.txt"},
	}}
	tree := buildFileTree(s)
	rows := flattenFileTree(tree, map[string]bool{"": true})

	if len(rows) != 1 || rows[0].label != "/" {
		t.Fatalf("rows = %+v, want just the collapsed root row", rows)
	}
}

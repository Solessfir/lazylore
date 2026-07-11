package ui

import (
	"testing"

	"lazylore/internal/lore"
)

func TestBuildFileTree_FlatFileHasNoDirectoryAncestor(t *testing.T) {
	s := lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}
	tree := buildFileTree(s)

	if len(tree.children) != 1 {
		t.Fatalf("root children = %+v, want 1 entry", tree.children)
	}
	if tree.children[0].isDir {
		t.Fatalf("a.txt at repo root should be a file leaf, not a directory")
	}
	if tree.children[0].path != "a.txt" {
		t.Fatalf("path = %q, want %q", tree.children[0].path, "a.txt")
	}
}

func TestBuildFileTree_GroupsFilesUnderSharedDirectory(t *testing.T) {
	s := lore.Status{Unstaged: []lore.FileChange{
		{Status: 'M', Path: "src/a.go"},
		{Status: 'M', Path: "src/b.go"},
	}}
	tree := buildFileTree(s)

	if len(tree.children) != 1 || !tree.children[0].isDir || tree.children[0].name != "src" {
		t.Fatalf("root children = %+v, want a single 'src' directory", tree.children)
	}
	srcDir := tree.children[0]
	if len(srcDir.children) != 2 {
		t.Fatalf("src/ children = %+v, want 2 files", srcDir.children)
	}
}

func TestBuildFileTree_NestedDirectoriesBuildFullChain(t *testing.T) {
	s := lore.Status{Staged: []lore.FileChange{{Status: 'A', Path: "internal/ui/model.go"}}}
	tree := buildFileTree(s)

	internal := tree.children[0]
	if internal.name != "internal" || !internal.isDir {
		t.Fatalf("first level = %+v, want directory 'internal'", internal)
	}
	ui := internal.children[0]
	if ui.name != "ui" || !ui.isDir || ui.path != "internal/ui" {
		t.Fatalf("second level = %+v, want directory 'internal/ui'", ui)
	}
	model := ui.children[0]
	if model.isDir || model.path != "internal/ui/model.go" {
		t.Fatalf("leaf = %+v, want file 'internal/ui/model.go'", model)
	}
}

func TestBuildFileTree_SamePathStagedAndUnstagedAreTwoLeaves(t *testing.T) {
	s := lore.Status{
		Staged:   []lore.FileChange{{Status: 'M', Path: "a.txt"}},
		Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}},
	}
	tree := buildFileTree(s)

	if len(tree.children) != 2 {
		t.Fatalf("root children = %+v, want 2 leaves (staged + unstaged a.txt)", tree.children)
	}
	if tree.children[0].staged == tree.children[1].staged {
		t.Fatalf("expected one staged and one unstaged leaf, got %+v", tree.children)
	}
}

func TestBuildFileTree_ChildrenSortedAlphabetically(t *testing.T) {
	s := lore.Status{Unstaged: []lore.FileChange{
		{Status: 'M', Path: "z.txt"},
		{Status: 'M', Path: "a.txt"},
	}}
	tree := buildFileTree(s)

	if tree.children[0].name != "a.txt" || tree.children[1].name != "z.txt" {
		t.Fatalf("children = %+v, want a.txt before z.txt", tree.children)
	}
}

func TestFlattenFileTree_ExpandedShowsEveryNode(t *testing.T) {
	s := lore.Status{Unstaged: []lore.FileChange{
		{Status: 'M', Path: "src/a.go"},
		{Status: 'M', Path: "b.txt"},
	}}
	tree := buildFileTree(s)
	rows := flattenFileTree(tree, nil)

	if len(rows) != 3 {
		t.Fatalf("rows = %+v, want 3 (b.txt, src/, src/a.go)", rows)
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

func TestFlattenFileTree_DepthIncreasesPerNestingLevel(t *testing.T) {
	s := lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a/b/c.go"}}}
	tree := buildFileTree(s)
	rows := flattenFileTree(tree, nil)

	wantDepths := []int{0, 1, 2} // a/, a/b/, a/b/c.go
	if len(rows) != len(wantDepths) {
		t.Fatalf("rows = %+v, want %d rows", rows, len(wantDepths))
	}
	for i, want := range wantDepths {
		if rows[i].depth != want {
			t.Fatalf("rows[%d].depth = %d, want %d", i, rows[i].depth, want)
		}
	}
}

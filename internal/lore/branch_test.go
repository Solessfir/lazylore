package lore_test

import (
	"testing"

	"lazylore/internal/lore"
)

// Captured verbatim from `lore.exe branch list` with a single branch.
const branchListSingleOutput = `Local branches:
* main
Remote branches:
  main
`

// Captured verbatim from `lore.exe branch list` after creating and switching
// to a second branch.
const branchListMultiOutput = `Local branches:
  main
* my-first-branch
Remote branches:
  main
`

func TestParseBranchList_Single(t *testing.T) {
	branches, err := lore.ParseBranchList(branchListSingleOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []lore.Branch{
		{Name: "main", Current: true, Remote: false},
		{Name: "main", Current: false, Remote: true},
	}
	if len(branches) != len(want) {
		t.Fatalf("branches = %+v, want %+v", branches, want)
	}
	for i := range want {
		if branches[i] != want[i] {
			t.Fatalf("branches[%d] = %+v, want %+v", i, branches[i], want[i])
		}
	}
}

func TestParseBranchList_Multi(t *testing.T) {
	branches, err := lore.ParseBranchList(branchListMultiOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []lore.Branch{
		{Name: "main", Current: false, Remote: false},
		{Name: "my-first-branch", Current: true, Remote: false},
		{Name: "main", Current: false, Remote: true},
	}
	if len(branches) != len(want) {
		t.Fatalf("branches = %+v, want %+v", branches, want)
	}
	for i := range want {
		if branches[i] != want[i] {
			t.Fatalf("branches[%d] = %+v, want %+v", i, branches[i], want[i])
		}
	}
}

func TestParseBranchList_ErrorsWhenNoHeaders(t *testing.T) {
	_, err := lore.ParseBranchList("garbage\n")
	if err == nil {
		t.Fatal("expected an error when no section headers are present")
	}
}

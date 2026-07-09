package lore_test

import (
	"testing"

	"lazylore/internal/lore"
)

// Captured verbatim from `lore.exe status` against a real local demo repo.
const statusCleanOutput = `Repository 019f46fd1f7b7880a88e902a7b44074a
On branch main revision 0 -> 0000000000000000000000000000000000000000000000000000000000000000
Remote revision 0 -> 0000000000000000000000000000000000000000000000000000000000000000
Local branch in sync with remote
`

// Captured verbatim from `lore.exe status --scan` before any file was staged.
const statusUntrackedOutput = `Repository 019f46fd1f7b7880a88e902a7b44074a
On branch main revision 0 -> 0000000000000000000000000000000000000000000000000000000000000000
Remote revision 0 -> 0000000000000000000000000000000000000000000000000000000000000000
Local branch in sync with remote
Untracked files:
A hello.txt
A sample.bin
Tracked changes: 2 added
`

// Captured verbatim from `lore.exe status` after `lore.exe stage hello.txt sample.bin`.
const statusStagedOutput = "Repository 019f46fd1f7b7880a88e902a7b44074a\n" +
	"On branch main revision 0 -> 0000000000000000000000000000000000000000000000000000000000000000\n" +
	"Remote revision 0 -> 0000000000000000000000000000000000000000000000000000000000000000\n" +
	"Local branch in sync with remote\n" +
	"Changes staged for commit:\n" +
	"A hello.txt \n" +
	"A sample.bin \n"

// Captured verbatim from `lore.exe status --scan` after editing a tracked file post-commit.
const statusModifiedOutput = `Repository 019f46fd1f7b7880a88e902a7b44074a
On branch main revision 1 -> b3e648f10d02c6162433bfdb49c06012027d19bc586560647d0bf2ff6278c195
Remote revision 0 -> 0000000000000000000000000000000000000000000000000000000000000000
Local branch is ahead of remote
Changes not staged for commit:
M hello.txt
Tracked changes: 1 modified
`

func TestParseStatus_Clean(t *testing.T) {
	s, err := lore.ParseStatus(statusCleanOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Repository != "019f46fd1f7b7880a88e902a7b44074a" {
		t.Fatalf("Repository = %q", s.Repository)
	}
	if s.Branch != "main" {
		t.Fatalf("Branch = %q, want main", s.Branch)
	}
	if len(s.Staged) != 0 || len(s.Unstaged) != 0 {
		t.Fatalf("expected no staged/unstaged files, got %+v / %+v", s.Staged, s.Unstaged)
	}
}

func TestParseStatus_Untracked(t *testing.T) {
	s, err := lore.ParseStatus(statusUntrackedOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []lore.FileChange{{Status: 'A', Path: "hello.txt"}, {Status: 'A', Path: "sample.bin"}}
	if len(s.Unstaged) != len(want) {
		t.Fatalf("Unstaged = %+v, want %+v", s.Unstaged, want)
	}
	for i, fc := range want {
		if s.Unstaged[i] != fc {
			t.Fatalf("Unstaged[%d] = %+v, want %+v", i, s.Unstaged[i], fc)
		}
	}
	if len(s.Staged) != 0 {
		t.Fatalf("Staged = %+v, want empty", s.Staged)
	}
}

func TestParseStatus_Staged(t *testing.T) {
	s, err := lore.ParseStatus(statusStagedOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []lore.FileChange{{Status: 'A', Path: "hello.txt"}, {Status: 'A', Path: "sample.bin"}}
	if len(s.Staged) != len(want) {
		t.Fatalf("Staged = %+v, want %+v", s.Staged, want)
	}
	for i, fc := range want {
		if s.Staged[i] != fc {
			t.Fatalf("Staged[%d] = %+v, want %+v", i, s.Staged[i], fc)
		}
	}
}

func TestParseStatus_ModifiedUnstaged(t *testing.T) {
	s, err := lore.ParseStatus(statusModifiedOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := lore.FileChange{Status: 'M', Path: "hello.txt"}
	if len(s.Unstaged) != 1 || s.Unstaged[0] != want {
		t.Fatalf("Unstaged = %+v, want [%+v]", s.Unstaged, want)
	}
}

func TestParseStatus_ErrorsOnEmptyInput(t *testing.T) {
	_, err := lore.ParseStatus("")
	if err == nil {
		t.Fatal("expected an error for output missing a Repository line")
	}
}

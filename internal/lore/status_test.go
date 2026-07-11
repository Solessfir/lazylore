package lore_test

import (
	"testing"

	"lazylore/internal/lore"
)

// Captured verbatim from `lore.exe --json status` against a real repo with
// no staged or dirty files.
const statusCleanOutput = `{"tagName":"repositoryStatusRevision","data":{"repository":"019f46fd1f7b7880a88e902a7b44074a","branch":"019f46fd51147821aed79f28a839f591","branchName":"my-first-branch","revision":"45593a0083a67a79602235b4d6c39d9d2dc1fc89375543a1bd6bd43288fa60a2","revisionNumber":2,"revisionStaged":"0000000000000000000000000000000000000000000000000000000000000000","revisionMerged":"0000000000000000000000000000000000000000000000000000000000000000","revisionMergedParentBranch":"0000000000000000000000000000000000000000000000000000000000000000","revisionLocal":"45593a0083a67a79602235b4d6c39d9d2dc1fc89375543a1bd6bd43288fa60a2","revisionLocalNumber":2,"revisionRemote":"0000000000000000000000000000000000000000000000000000000000000000","revisionRemoteNumber":0,"isLocalAhead":0,"isRemoteAhead":0,"remoteAvailable":1,"remoteAuthorized":1,"remoteBranchExist":0}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`

// Captured verbatim from `lore.exe --json status --scan` with a genuinely
// new, never-tracked file present ("action":"add").
const statusUntrackedOutput = `{"tagName":"repositoryStatusRevision","data":{"repository":"019f46fd1f7b7880a88e902a7b44074a","branch":"019f46fd51147821aed79f28a839f591","branchName":"my-first-branch","revision":"45593a0083a67a79602235b4d6c39d9d2dc1fc89375543a1bd6bd43288fa60a2","revisionNumber":2,"revisionStaged":"2c7693347baefa2b6aaec3fabd7b15a79a7de94c3909036e3ffc62a039c40564","revisionMerged":"0000000000000000000000000000000000000000000000000000000000000000","revisionMergedParentBranch":"0000000000000000000000000000000000000000000000000000000000000000","revisionLocal":"45593a0083a67a79602235b4d6c39d9d2dc1fc89375543a1bd6bd43288fa60a2","revisionLocalNumber":2,"revisionRemote":"0000000000000000000000000000000000000000000000000000000000000000","revisionRemoteNumber":0,"isLocalAhead":0,"isRemoteAhead":0,"remoteAvailable":1,"remoteAuthorized":1,"remoteBranchExist":0}}
{"tagName":"repositoryStatusFile","data":{"path":"totally-new-file.txt","size":15,"action":"add","type":"file","flagStaged":false,"flagMerged":false,"flagConflict":false,"flagConflictUnresolved":false,"flagConflictAutomerged":false,"flagConflictMine":false,"flagConflictTheirs":false,"flagDirty":true,"fromPath":""}}
{"tagName":"repositoryStatusSummary","data":{"adds":1,"deletes":0,"modifies":0,"moves":0,"copies":0}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`

// Captured verbatim from `lore.exe --json status --scan` after
// `lore.exe --json stage hello.txt` ("flagStaged":true).
const statusStagedOutput = `{"tagName":"repositoryStatusRevision","data":{"repository":"019f46fd1f7b7880a88e902a7b44074a","branch":"019f46fd51147821aed79f28a839f591","branchName":"my-first-branch","revision":"45593a0083a67a79602235b4d6c39d9d2dc1fc89375543a1bd6bd43288fa60a2","revisionNumber":2,"revisionStaged":"2c7693347baefa2b6aaec3fabd7b15a79a7de94c3909036e3ffc62a039c40564","revisionMerged":"0000000000000000000000000000000000000000000000000000000000000000","revisionMergedParentBranch":"0000000000000000000000000000000000000000000000000000000000000000","revisionLocal":"45593a0083a67a79602235b4d6c39d9d2dc1fc89375543a1bd6bd43288fa60a2","revisionLocalNumber":2,"revisionRemote":"0000000000000000000000000000000000000000000000000000000000000000","revisionRemoteNumber":0,"isLocalAhead":0,"isRemoteAhead":0,"remoteAvailable":1,"remoteAuthorized":1,"remoteBranchExist":0}}
{"tagName":"repositoryStatusFile","data":{"path":"hello.txt","size":40,"action":"keep","type":"file","flagStaged":true,"flagMerged":false,"flagConflict":false,"flagConflictUnresolved":false,"flagConflictAutomerged":false,"flagConflictMine":false,"flagConflictTheirs":false,"flagDirty":true,"fromPath":""}}
{"tagName":"repositoryStatusSummary","data":{"adds":0,"deletes":0,"modifies":1,"moves":0,"copies":0}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`

// Captured verbatim from `lore.exe --json status --scan` on a tracked file
// edited but not yet staged ("flagStaged":false,"flagDirty":true).
const statusModifiedOutput = `{"tagName":"repositoryStatusRevision","data":{"repository":"019f46fd1f7b7880a88e902a7b44074a","branch":"019f46fd51147821aed79f28a839f591","branchName":"my-first-branch","revision":"45593a0083a67a79602235b4d6c39d9d2dc1fc89375543a1bd6bd43288fa60a2","revisionNumber":2,"revisionStaged":"0000000000000000000000000000000000000000000000000000000000000000","revisionMerged":"0000000000000000000000000000000000000000000000000000000000000000","revisionMergedParentBranch":"0000000000000000000000000000000000000000000000000000000000000000","revisionLocal":"45593a0083a67a79602235b4d6c39d9d2dc1fc89375543a1bd6bd43288fa60a2","revisionLocalNumber":2,"revisionRemote":"0000000000000000000000000000000000000000000000000000000000000000","revisionRemoteNumber":0,"isLocalAhead":0,"isRemoteAhead":0,"remoteAvailable":1,"remoteAuthorized":1,"remoteBranchExist":0}}
{"tagName":"repositoryStatusFile","data":{"path":"hello.txt","size":40,"action":"keep","type":"file","flagStaged":false,"flagMerged":false,"flagConflict":false,"flagConflictUnresolved":false,"flagConflictAutomerged":false,"flagConflictMine":false,"flagConflictTheirs":false,"flagDirty":true,"fromPath":""}}
{"tagName":"repositoryStatusSummary","data":{"adds":0,"deletes":0,"modifies":1,"moves":0,"copies":0}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`

func TestParseStatus_Clean(t *testing.T) {
	s, err := lore.ParseStatus(statusCleanOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Repository != "019f46fd1f7b7880a88e902a7b44074a" {
		t.Fatalf("Repository = %q", s.Repository)
	}
	if s.Branch != "my-first-branch" {
		t.Fatalf("Branch = %q, want my-first-branch", s.Branch)
	}
	if len(s.Staged) != 0 || len(s.Unstaged) != 0 {
		t.Fatalf("expected no staged/unstaged files, got %+v / %+v", s.Staged, s.Unstaged)
	}
	if s.AheadCount != 0 || s.BehindCount != 0 {
		t.Fatalf("AheadCount/BehindCount = %d/%d, want 0/0 (remoteBranchExist is 0 in this fixture)", s.AheadCount, s.BehindCount)
	}
}

func TestParseStatus_AheadOnly(t *testing.T) {
	out := `{"tagName":"repositoryStatusRevision","data":{"repository":"r","branch":"b","branchName":"main","revision":"h","revisionNumber":7,"revisionStaged":"0000000000000000000000000000000000000000000000000000000000000000","revisionMerged":"0000000000000000000000000000000000000000000000000000000000000000","revisionMergedParentBranch":"0000000000000000000000000000000000000000000000000000000000000000","revisionLocal":"h","revisionLocalNumber":7,"revisionRemote":"r2","revisionRemoteNumber":4,"isLocalAhead":1,"isRemoteAhead":0,"remoteAvailable":1,"remoteAuthorized":1,"remoteBranchExist":1}}
` + jsonCompleteSuccess
	s, err := lore.ParseStatus(out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.AheadCount != 3 || s.BehindCount != 0 {
		t.Fatalf("AheadCount/BehindCount = %d/%d, want 3/0", s.AheadCount, s.BehindCount)
	}
}

func TestParseStatus_BehindOnly(t *testing.T) {
	out := `{"tagName":"repositoryStatusRevision","data":{"repository":"r","branch":"b","branchName":"main","revision":"h","revisionNumber":4,"revisionStaged":"0000000000000000000000000000000000000000000000000000000000000000","revisionMerged":"0000000000000000000000000000000000000000000000000000000000000000","revisionMergedParentBranch":"0000000000000000000000000000000000000000000000000000000000000000","revisionLocal":"h","revisionLocalNumber":4,"revisionRemote":"r2","revisionRemoteNumber":9,"isLocalAhead":0,"isRemoteAhead":1,"remoteAvailable":1,"remoteAuthorized":1,"remoteBranchExist":1}}
` + jsonCompleteSuccess
	s, err := lore.ParseStatus(out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.AheadCount != 0 || s.BehindCount != 5 {
		t.Fatalf("AheadCount/BehindCount = %d/%d, want 0/5", s.AheadCount, s.BehindCount)
	}
}

func TestParseStatus_IgnoresRemoteComparisonWhenRemoteBranchDoesNotExist(t *testing.T) {
	// remoteBranchExist:0 (branch never pushed) - the local/remote revision
	// numbers aren't a meaningful comparison in that case even if ahead
	// flags happen to be set, so no count should be derived.
	out := `{"tagName":"repositoryStatusRevision","data":{"repository":"r","branch":"b","branchName":"main","revision":"h","revisionNumber":7,"revisionStaged":"0000000000000000000000000000000000000000000000000000000000000000","revisionMerged":"0000000000000000000000000000000000000000000000000000000000000000","revisionMergedParentBranch":"0000000000000000000000000000000000000000000000000000000000000000","revisionLocal":"h","revisionLocalNumber":7,"revisionRemote":"0000000000000000000000000000000000000000000000000000000000000000","revisionRemoteNumber":0,"isLocalAhead":1,"isRemoteAhead":0,"remoteAvailable":1,"remoteAuthorized":1,"remoteBranchExist":0}}
` + jsonCompleteSuccess
	s, err := lore.ParseStatus(out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.AheadCount != 0 || s.BehindCount != 0 {
		t.Fatalf("AheadCount/BehindCount = %d/%d, want 0/0 (branch not pushed yet)", s.AheadCount, s.BehindCount)
	}
}

func TestParseStatus_Untracked(t *testing.T) {
	s, err := lore.ParseStatus(statusUntrackedOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := lore.FileChange{Status: 'A', Path: "totally-new-file.txt"}
	if len(s.Unstaged) != 1 || s.Unstaged[0] != want {
		t.Fatalf("Unstaged = %+v, want [%+v]", s.Unstaged, want)
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
	want := lore.FileChange{Status: 'M', Path: "hello.txt"}
	if len(s.Staged) != 1 || s.Staged[0] != want {
		t.Fatalf("Staged = %+v, want [%+v]", s.Staged, want)
	}
	if len(s.Unstaged) != 0 {
		t.Fatalf("Unstaged = %+v, want empty", s.Unstaged)
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

func TestParseStatus_ErrorsWhenNoRevisionEvent(t *testing.T) {
	_, err := lore.ParseStatus(`{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}` + "\n")
	if err == nil {
		t.Fatal("expected an error for output missing a repositoryStatusRevision event")
	}
}

func TestParseStatus_ErrorsOnMalformedInput(t *testing.T) {
	_, err := lore.ParseStatus(`not json at all`)
	if err == nil {
		t.Fatal("expected an error for malformed --json output")
	}
}

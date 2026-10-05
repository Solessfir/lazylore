package lore_test

import (
	"reflect"
	"testing"

	"github.com/solessfir/lazylore/internal/lore"
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

func TestParseStatus_RealSusRepoAheadFixture(t *testing.T) {
	// Captured verbatim from `lore.exe --json status --scan` against the
	// real Sus repo (local at revision 2, remote still at revision 1 - lore
	// push's own message on this exact repo confirmed "Local branch is 1
	// revision(s) ahead of remote"), used to verify the ahead-count bug
	// report was actually a real 1-revision gap, not a lazylore miscalc.
	const out = `{"tagName":"repositoryStatusRevision","data":{"repository":"019f492871097de0b06e0e77e1119e0d","branch":"e726318bbc3fd75ac8733a7e030cc35b","branchName":"main","revision":"f1044a02cbb3684b4db203c82e8d3aaf3294f67a4b9a29a7d8f684eade1c485f","revisionNumber":2,"revisionStaged":"c22a72bd954a1baaf8e7b7b232b38ca612033b9395f40d28fede6249b688a0f5","revisionMerged":"0000000000000000000000000000000000000000000000000000000000000000","revisionMergedParentBranch":"0000000000000000000000000000000000000000000000000000000000000000","revisionLocal":"f1044a02cbb3684b4db203c82e8d3aaf3294f67a4b9a29a7d8f684eade1c485f","revisionLocalNumber":2,"revisionRemote":"f913ade57f2b133acaf74cfefa7282332238e12ff17ccb7213c1182d28214ce5","revisionRemoteNumber":1,"isLocalAhead":1,"isRemoteAhead":0,"remoteAvailable":1,"remoteAuthorized":1,"remoteBranchExist":1}}
` + jsonCompleteSuccess
	s, err := lore.ParseStatus(out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.AheadCount != 1 || s.BehindCount != 0 {
		t.Fatalf("AheadCount/BehindCount = %d/%d, want 1/0", s.AheadCount, s.BehindCount)
	}
	if !s.HasRemoteInfo || s.RemoteRevisionNumber != 1 {
		t.Fatalf("HasRemoteInfo/RemoteRevisionNumber = %v/%d, want true/1", s.HasRemoteInfo, s.RemoteRevisionNumber)
	}
}

func TestParseStatus_RemoteRevisionNumberSetEvenWhenBranchNeverPushed(t *testing.T) {
	// remoteBranchExist:0 correctly yields RemoteRevisionNumber:0 - every
	// local revision (number >= 1) is then "newer than remote", which is
	// exactly correct: nothing has ever reached the remote.
	out := `{"tagName":"repositoryStatusRevision","data":{"repository":"r","branch":"b","branchName":"main","revision":"h","revisionNumber":3,"revisionStaged":"0000000000000000000000000000000000000000000000000000000000000000","revisionMerged":"0000000000000000000000000000000000000000000000000000000000000000","revisionMergedParentBranch":"0000000000000000000000000000000000000000000000000000000000000000","revisionLocal":"h","revisionLocalNumber":3,"revisionRemote":"0000000000000000000000000000000000000000000000000000000000000000","revisionRemoteNumber":0,"isLocalAhead":0,"isRemoteAhead":0,"remoteAvailable":1,"remoteAuthorized":1,"remoteBranchExist":0}}
` + jsonCompleteSuccess
	s, err := lore.ParseStatus(out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.HasRemoteInfo || s.RemoteRevisionNumber != 0 {
		t.Fatalf("HasRemoteInfo/RemoteRevisionNumber = %v/%d, want true/0", s.HasRemoteInfo, s.RemoteRevisionNumber)
	}
}

func TestParseStatus_NoRemoteInfoWhenUnavailable(t *testing.T) {
	out := `{"tagName":"repositoryStatusRevision","data":{"repository":"r","branch":"b","branchName":"main","revision":"h","revisionNumber":3,"revisionStaged":"0000000000000000000000000000000000000000000000000000000000000000","revisionMerged":"0000000000000000000000000000000000000000000000000000000000000000","revisionMergedParentBranch":"0000000000000000000000000000000000000000000000000000000000000000","revisionLocal":"h","revisionLocalNumber":3,"revisionRemote":"0000000000000000000000000000000000000000000000000000000000000000","revisionRemoteNumber":0,"isLocalAhead":0,"isRemoteAhead":0,"remoteAvailable":0,"remoteAuthorized":0,"remoteBranchExist":0}}
` + jsonCompleteSuccess
	s, err := lore.ParseStatus(out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.HasRemoteInfo {
		t.Fatalf("HasRemoteInfo = true, want false when remote is unavailable (offline)")
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

// Captured verbatim from `lore.exe --json status --scan` on a repo where a
// directory node itself changed (e.g. an added folder), reported as its own
// entry ("type":"directory") alongside the file entries for what's inside
// it.
const statusDirectoryEntryOutput = `{"tagName":"repositoryStatusRevision","data":{"repository":"019f46fd1f7b7880a88e902a7b44074a","branch":"019f46fd51147821aed79f28a839f591","branchName":"my-first-branch","revision":"45593a0083a67a79602235b4d6c39d9d2dc1fc89375543a1bd6bd43288fa60a2","revisionNumber":2,"revisionStaged":"0000000000000000000000000000000000000000000000000000000000000000","revisionMerged":"0000000000000000000000000000000000000000000000000000000000000000","revisionMergedParentBranch":"0000000000000000000000000000000000000000000000000000000000000000","revisionLocal":"45593a0083a67a79602235b4d6c39d9d2dc1fc89375543a1bd6bd43288fa60a2","revisionLocalNumber":2,"revisionRemote":"0000000000000000000000000000000000000000000000000000000000000000","revisionRemoteNumber":0,"isLocalAhead":0,"isRemoteAhead":0,"remoteAvailable":1,"remoteAuthorized":1,"remoteBranchExist":0}}
{"tagName":"repositoryStatusFile","data":{"path":"SonarV2","size":0,"action":"add","type":"directory","flagStaged":false,"flagMerged":false,"flagConflict":false,"flagConflictUnresolved":false,"flagConflictAutomerged":false,"flagConflictMine":false,"flagConflictTheirs":false,"flagDirty":true,"fromPath":""}}
{"tagName":"repositoryStatusFile","data":{"path":"SonarV2/BP_DummySonar.uasset","size":128,"action":"add","type":"file","flagStaged":false,"flagMerged":false,"flagConflict":false,"flagConflictUnresolved":false,"flagConflictAutomerged":false,"flagConflictMine":false,"flagConflictTheirs":false,"flagDirty":true,"fromPath":""}}
{"tagName":"repositoryStatusSummary","data":{"adds":2,"deletes":0,"modifies":0,"moves":0,"copies":0}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`

func TestParseStatus_PreservesDirectoryTypeEntries(t *testing.T) {
	s, err := lore.ParseStatus(statusDirectoryEntryOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []lore.FileChange{
		{Status: 'A', Path: "SonarV2", Directory: true},
		{Status: 'A', Path: "SonarV2/BP_DummySonar.uasset"},
	}
	if !reflect.DeepEqual(s.Unstaged, want) {
		t.Fatalf("Unstaged = %+v, want %+v", s.Unstaged, want)
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

package ui

import "testing"

func TestDistributeSpace_StaticThenWeightedSumsExactly(t *testing.T) {
	boxes := []layoutBox{{Size: 3}, {Weight: 1}, {Weight: 1}, {Weight: 1}}
	got := distributeSpace(boxes, 37)

	sum := 0
	for _, v := range got {
		sum += v
	}
	if sum != 37 {
		t.Fatalf("sum = %d, want 37 (got %+v)", sum, got)
	}
	if got[0] != 3 {
		t.Fatalf("static box = %d, want 3", got[0])
	}
}

func TestDistributeSpace_RemainderNeverDropped(t *testing.T) {
	// (37 - 3) / 3 = 11 remainder 1 with plain division - that 1 row must
	// land somewhere, not vanish.
	boxes := []layoutBox{{Size: 3}, {Weight: 1}, {Weight: 1}, {Weight: 1}}
	got := distributeSpace(boxes, 37)

	for i := 1; i < len(got); i++ {
		if got[i] < 11 {
			t.Fatalf("weighted box %d = %d, want at least 11", i, got[i])
		}
	}
}

func TestDistributeSpace_EqualWeightsSplitEvenlyOnExactFit(t *testing.T) {
	boxes := []layoutBox{{Weight: 1}, {Weight: 2}}
	got := distributeSpace(boxes, 90)

	if got[0] != 30 || got[1] != 60 {
		t.Fatalf("got = %+v, want [30 60]", got)
	}
}

func TestDistributeSpace_StaticBoxCappedAtAvailable(t *testing.T) {
	boxes := []layoutBox{{Size: 100}}
	got := distributeSpace(boxes, 10)

	if got[0] != 10 {
		t.Fatalf("static box = %d, want capped to available (10)", got[0])
	}
}

func TestDistributeSpace_ZeroAvailableReturnsZeros(t *testing.T) {
	boxes := []layoutBox{{Size: 3}, {Weight: 1}, {Weight: 1}}
	got := distributeSpace(boxes, 0)

	for i, v := range got {
		if v != 0 {
			t.Fatalf("got[%d] = %d, want 0", i, v)
		}
	}
}

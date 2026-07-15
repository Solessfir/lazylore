package ui

// layoutBox is one child in a weighted space distribution: either a fixed
// Size, or a proportional Weight sharing whatever space is left over after
// every fixed-size sibling is satisfied. distributeSpace never drops the
// integer-division remainder (plain `available / n` would), so weighted
// panels stay exactly aligned with sibling widgets sized independently.
type layoutBox struct {
	Size   int
	Weight int
}

// distributeSpace assigns each box a share of available: static boxes get
// their Size first, then weighted boxes split whatever's left, with the
// division remainder handed out one unit at a time across the weighted
// boxes so the results always sum to exactly available (never less).
func distributeSpace(boxes []layoutBox, available int) []int {
	reserved := 0
	totalWeight := 0
	for _, b := range boxes {
		if b.Size > 0 {
			reserved += b.Size
		} else {
			totalWeight += b.Weight
		}
	}
	dynamic := max(0, available-reserved)

	unit, extra := 0, 0
	if totalWeight > 0 {
		unit = dynamic / totalWeight
		extra = dynamic % totalWeight
	}

	result := make([]int, len(boxes))
	for i, b := range boxes {
		if b.Size > 0 {
			result[i] = min(available, b.Size)
		} else {
			result[i] = unit * b.Weight
		}
	}
	for extra > 0 {
		for i, b := range boxes {
			if b.Size == 0 && b.Weight > 0 {
				result[i]++
				extra--
				if extra == 0 {
					break
				}
			}
		}
	}
	return result
}

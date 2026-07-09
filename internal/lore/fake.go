package lore

import (
	"fmt"
	"strings"
)

// FakeRunner is a Runner test double that returns scripted Results, keyed
// by the space-joined args of the call it responds to.
type FakeRunner struct {
	Results map[string]Result
	Errs    map[string]error
	Calls   [][]string
}

func (f *FakeRunner) Run(args ...string) (Result, error) {
	f.Calls = append(f.Calls, args)
	key := strings.Join(args, " ")
	if err, ok := f.Errs[key]; ok {
		return Result{}, err
	}
	if res, ok := f.Results[key]; ok {
		return res, nil
	}
	return Result{}, fmt.Errorf("FakeRunner: no result configured for %q", key)
}

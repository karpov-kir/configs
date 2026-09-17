package ecocheck

import (
	"os"
	"strings"
	"sync"
)

// A bash that answers from a table and forks nothing. Every case in this package but
// TestTheParseScanRunsARealBash drives one: two forks per script cost the suite 802 processes at ~100ms
// each, against 0.5ms of CPU apiece, and none of them were about what this package does with the answer.
//
// Exported because the case files are an external test package and build their fixtures on it.
type FakeBash struct {
	binaries []string

	// scanScriptsParse parses in a worker per CPU, so both halves of the table are reached at once.
	mutex    sync.Mutex
	refusals map[string][]string
	asked    int
}

// The binaries this bash reports. A case names its own rather than sharing one set: scripts.go's memo is
// held for the process and keyed on the binary, so two cases under one binary name answer each other's
// parses — harmless for a finding, and fatal for a count of them.
func NewFakeBash(binaries ...string) *FakeBash {
	return &FakeBash{binaries: binaries, refusals: map[string][]string{}}
}

func (b *FakeBash) Binaries() []string {
	return b.binaries
}

// Refuse registers what this bash writes about a script holding `body`: one complaint per line. A body
// nothing was registered for parses clean.
//
// Keyed on the script's bytes and not on its path, because that is what `bash -n` is a function of. A
// path would have to be registered under whichever spelling the walk happened to hand the scan — and the
// symlinked-root fixture hands it one the case never wrote, so every refusal there would silently go
// missing while the case went on passing on the findings of some other scan.
func (b *FakeBash) Refuse(body string, complaints ...string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.refusals[body] = append(b.refusals[body], complaints...)
}

// Each line is led by the path this bash was handed, the way `bash -n` leads every line it writes — and
// that path is the one piece of a finding the reviewed tree chose the bytes of.
func (b *FakeBash) Parse(binary, script string) string {
	body, err := os.ReadFile(script)
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.asked++
	if err != nil {
		return ""
	}
	var output strings.Builder
	for _, complaint := range b.refusals[string(body)] {
		output.WriteString(script + ": " + complaint + "\n")
	}
	return output.String()
}

// Parses answers how many times this bash was asked — the processes a real one would have forked. It is
// the only place the memo is observable: a memo that stopped working changes no finding, it only turns a
// two-second suite back into a seventy-second one and every real check into a slower gate.
func (b *FakeBash) Parses() int {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.asked
}

// Package runtest is what a suite reads back from a run it drove: the two streams it was handed, and
// the exit code it answered.
//
// It sits apart from any one tool's fixtures because the assertion is the same wherever a tool prints
// and exits, and a suite with no installer in it cannot reach for a package named after one. Nothing
// about a home, a checkout or a mount belongs here — ai/tools/installertest holds that, and builds on
// this.
package runtest

import (
	"strings"
	"testing"
)

// Output is the two streams a run was handed. A run that could print elsewhere has output no assertion
// reads, so a fixture hands it these and asserts against them afterwards.
type Output struct {
	t   *testing.T
	Out strings.Builder
	Err strings.Builder
}

func NewOutput(t *testing.T) *Output {
	t.Helper()
	return &Output{t: t}
}

// Reset clears both streams, for a fixture that drives a second run through the same handle.
func (o *Output) Reset() {
	o.Out.Reset()
	o.Err.Reset()
}

// Said is both streams together. A refusal reaches a human whichever one it was written to, and a
// case that named the wrong stream would fail over something no reader would notice.
func (o *Output) Said() string {
	return o.Out.String() + o.Err.String()
}

func (o *Output) ExpectSaid(want string) {
	o.t.Helper()
	if !strings.Contains(o.Said(), want) {
		o.t.Errorf("the run never said %q. It said:\n%s", want, o.Said())
	}
}

func (o *Output) ExpectNotSaid(unwanted string) {
	o.t.Helper()
	if strings.Contains(o.Said(), unwanted) {
		o.t.Errorf("the run said %q, which it must not. It said:\n%s", unwanted, o.Said())
	}
}

// ExpectCode quotes what the run printed, because an exit code alone says nothing about why.
func (o *Output) ExpectCode(got, want int) {
	o.t.Helper()
	if got != want {
		o.t.Errorf("the run exited %d, wanted %d. It said:\n%s", got, want, o.Said())
	}
}

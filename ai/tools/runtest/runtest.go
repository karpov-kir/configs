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

// Some cases are about which stream carried the text. A report written to stderr and a refusal
// written to stdout are both defects, and Said cannot tell either of them from a correct run. These
// read one stream, and quote both when they fail, since the text is usually in the other one.

func (o *Output) ExpectOut(want string) {
	o.t.Helper()
	if !strings.Contains(o.Out.String(), want) {
		o.t.Errorf("stdout never said %q. stdout:\n%s\nstderr:\n%s", want, o.Out.String(), o.Err.String())
	}
}

func (o *Output) ExpectNotOut(unwanted string) {
	o.t.Helper()
	if strings.Contains(o.Out.String(), unwanted) {
		o.t.Errorf("stdout said %q, which it must not. stdout:\n%s", unwanted, o.Out.String())
	}
}

func (o *Output) ExpectErr(want string) {
	o.t.Helper()
	if !strings.Contains(o.Err.String(), want) {
		o.t.Errorf("stderr never said %q. stderr:\n%s\nstdout:\n%s", want, o.Err.String(), o.Out.String())
	}
}

func (o *Output) ExpectNotErr(unwanted string) {
	o.t.Helper()
	if strings.Contains(o.Err.String(), unwanted) {
		o.t.Errorf("stderr said %q, which it must not. stderr:\n%s", unwanted, o.Err.String())
	}
}

// ExpectNoOut is for a run whose whole answer is its exit code. Anything on stdout there is read by a
// caller as the report, so the empty stream is the assertion.
func (o *Output) ExpectNoOut() {
	o.t.Helper()
	if o.Out.String() != "" {
		o.t.Errorf("stdout carried %q, and this run answers with its exit code", o.Out.String())
	}
}

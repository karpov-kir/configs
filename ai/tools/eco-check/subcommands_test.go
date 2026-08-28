package ecocheck_test

// Every subcommand a dispatch accepts has a documented call site — and the scan says so when it cannot
// tell what a dispatch accepts. The shell-to-Go migration moved report.sh's `case` dispatch into a Go
// switch behind a 57-line stub, and the scan then found no `case` in any stub and reported nothing for
// all fourteen of its subcommands. Silence is the failure these cases exist to keep out: a scan that
// reports nothing where it used to report reads to every caller as a pass it did not earn.

import (
	"fmt"
	"strings"
	"testing"
)

const (
	noCallSite      = "subcommand with no call site"
	unreadable      = "cannot read toy.sh's dispatch"
	acceptsUnnamed  = "accepts a subcommand its usage does not name"
	namesUnaccepted = "usage names a subcommand its dispatch does not accept"
)

// The tool source the stub below execs: a dispatch, found by the `usage: toy.sh {` line its refusing
// arm carries rather than by the name of the function around it.
const toyDispatch = `package toy

func (r *run) dispatch() {
	switch r.arg(0) {
	case "alpha":
		r.alpha()
	case "beta":
		r.beta()
	default:
		r.refuse("usage: toy.sh {alpha|beta}")
	}
}
`

// A switch that is not the dispatch, ahead of one that is. Its arms are string literals of the same
// shape, so only the marker tells the two apart.
const toyDecoyAndDispatch = `package toy

func (r *run) stageOf(name string) string {
	switch name {
	case "pending":
		return "p"
	case "closed":
		return "c"
	}
	return ""
}

func (r *run) dispatch() {
	switch r.arg(0) {
	case "alpha":
		r.alpha()
	default:
		r.refuse("usage: toy.sh {alpha}")
	}
}
`

func TestGoDispatchSubcommandCallSites(t *testing.T) {
	t.Run("fires on a Go-dispatched subcommand no file documents a call site for", func(t *testing.T) {
		newToolStub(t, "toy.sh {alpha|beta}", toyDispatch).reports(noCallSite + ": beta")
	})

	t.Run("accepts one a document does name", func(t *testing.T) {
		f := newToolStub(t, "toy.sh {alpha|beta}", toyDispatch)
		f.write(f.root+"/kk-flavor/standards/how.md", "run `toy.sh beta` when the stage returns\n")
		f.doesNotReport(noCallSite + ": beta")
	})

	// The half the stub broke. Both lists are read off the tree, so a subcommand present in either
	// has to be checked — and neither list may be the only one that ever answers.
	t.Run("reads a grammar wrapped across comment lines", func(t *testing.T) {
		newWrappedGrammarStub(t).reports(noCallSite + ": gamma")
	})

	t.Run("and agrees with the dispatch about the wrapped half", func(t *testing.T) {
		newWrappedGrammarStub(t).doesNotReport(acceptsUnnamed)
	})

	t.Run("takes labels from the dispatch and not from another switch beside it", func(t *testing.T) {
		newToolStub(t, "toy.sh {alpha}", toyDecoyAndDispatch).doesNotReport(noCallSite + ": pending")
	})

	// Without this the case above passes on a run that read no dispatch at all.
	t.Run("while still taking the dispatch's own (control for the case above)", func(t *testing.T) {
		newToolStub(t, "toy.sh {alpha}", toyDecoyAndDispatch).reports(noCallSite + ": alpha")
	})
}

// The regression itself: the scan could not enumerate, and returned nothing. Either way it could fail
// to read a dispatch, it has to say which way — and then check what it does know rather than nothing.
func TestADispatchThatCannotBeReadIsReported(t *testing.T) {
	t.Run("fires when the tool ships no source to read a dispatch out of", func(t *testing.T) {
		newStubWithoutSource(t).reports(unreadable)
	})

	t.Run("and names that as the way it could not read it", func(t *testing.T) {
		newStubWithoutSource(t).reports("no source directory at")
	})

	t.Run("and still checks the subcommands the usage names", func(t *testing.T) {
		newStubWithoutSource(t).reports(noCallSite + ": beta")
	})

	t.Run("fires when the source holds no switch carrying the stub's usage line", func(t *testing.T) {
		newStubWithUnmarkedSource(t).reports(unreadable)
	})

	t.Run("and names that as the way it could not read it", func(t *testing.T) {
		newStubWithUnmarkedSource(t).reports("no switch under")
	})

	// A stub that names no grammar and a tool that holds no dispatch takes no subcommand: check.sh
	// and stats.sh each take a root and nothing else. That is a determination, not a failure.
	t.Run("stays quiet on a stub whose tool takes no subcommand at all", func(t *testing.T) {
		f := newRoot(t)
		f.mkdirAll(f.root + "/tools/toy")
		f.write(f.root+"/tools/toy/toy.go", "package toy\n\nfunc main() {}\n")
		f.newScript("toy.sh", "#!/usr/bin/env bash\n# untested: fixture\n#   usage: toy.sh [<root>]\ntool=\"toy\"\ntrue")
		f.doesNotReport(unreadable)
	})
}

// The two lists disagreeing is a defect in whichever one the name is missing from, and reporting it is
// what keeps either from silently becoming the only authority.
func TestUsageAndDispatchAreHeldAgainstEachOther(t *testing.T) {
	t.Run("fires on a subcommand the dispatch accepts and the usage does not name", func(t *testing.T) {
		newToolStub(t, "toy.sh {alpha}", toyDispatch).reports(acceptsUnnamed + ": beta")
	})

	t.Run("fires on one the usage names and the dispatch does not accept", func(t *testing.T) {
		newToolStub(t, "toy.sh {alpha|beta|gamma}", toyDispatch).reports(namesUnaccepted + ": gamma")
	})

	t.Run("stays quiet when the two agree", func(t *testing.T) {
		newToolStub(t, "toy.sh {alpha|beta}", toyDispatch).doesNotReport("subcommand its usage")
	})
}

// The bound on how many subcommands one scan carries: each costs a substring test per file, and the
// dispatch they were read from is a file the reviewed branch wrote. It has to *report*, never quietly
// check fewer than it looks like it checked.
func TestSubcommandCountIsBounded(t *testing.T) {
	// The usage names every arm the dispatch has, deliberately: a fixture where the two lists disagree
	// floods the one finding class both cases read out of, and the control below would then fail on
	// the flood rather than on the bound.
	newFloodedDispatch := func(t *testing.T) *fixture {
		var source, grammar strings.Builder
		source.WriteString("package toy\n\nfunc (r *run) dispatch() {\n\tswitch r.arg(0) {\n")
		for i := 1; i <= 300; i++ {
			fmt.Fprintf(&source, "\tcase \"s%03d\":\n\t\tr.s%03d()\n", i, i)
			if i > 1 {
				grammar.WriteString("|")
			}
			fmt.Fprintf(&grammar, "s%03d", i)
		}
		usage := "toy.sh {" + grammar.String() + "}"
		fmt.Fprintf(&source, "\tdefault:\n\t\tr.refuse(%q)\n\t}\n}\n", "usage: "+usage)
		return newToolStub(t, usage, source.String())
	}

	t.Run("reports the subcommands it did not carry", func(t *testing.T) {
		newFloodedDispatch(t).reports("were NOT checked")
	})

	// Without this the case above passes on a run that checked none of them.
	t.Run("and carries the ones up to the bound (control for the case above)", func(t *testing.T) {
		newFloodedDispatch(t).reports(noCallSite + ": s001")
	})
}

// A stub reaching a Go tool, and that tool's source — the shape report.sh and eco-report have had since
// the shell implementation moved into Go. The stub is a stub: what it declares is the tool it execs.
func newToolStub(t *testing.T, usage, source string) *fixture {
	t.Helper()
	f := newRoot(t)
	f.mkdirAll(f.root + "/tools/toy")
	f.write(f.root+"/tools/toy/toy.go", source)
	f.newStubScript(usage)
	return f
}

func newStubWithoutSource(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.newStubScript("toy.sh {alpha|beta}")
	return f
}

func newStubWithUnmarkedSource(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.mkdirAll(f.root + "/tools/toy")
	// A real dispatch, refusing under a different script's name: the marker is what says which stub
	// a dispatch belongs to, and no other line in the file does.
	f.write(f.root+"/tools/toy/toy.go", strings.Replace(toyDispatch, "usage: toy.sh {", "usage: other.sh {", 1))
	f.newStubScript("toy.sh {alpha|beta}")
	return f
}

func (f *fixture) newStubScript(usage string) {
	f.t.Helper()
	f.newScript("toy.sh", "#!/usr/bin/env bash\n# untested: fixture\n#   usage: "+usage+"\ntool=\"toy\"\ntrue")
}

// The grammar wrapped the way report.sh's own header wraps it, over a dispatch that accepts all four.
func newWrappedGrammarStub(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.mkdirAll(f.root + "/tools/toy")
	f.write(f.root+"/tools/toy/toy.go", `package toy

func (r *run) dispatch() {
	switch r.arg(0) {
	case "alpha":
		r.alpha()
	case "beta":
		r.beta()
	case "gamma":
		r.gamma()
	case "delta":
		r.delta()
	default:
		r.refuse("usage: toy.sh {alpha <x>|beta|gamma \"<y>\"|delta} [<z>]")
	}
}
`)
	f.newScript("toy.sh", "#!/usr/bin/env bash\n# untested: fixture\n"+
		"#   usage: toy.sh {alpha <x>|beta|\n"+
		"#                  gamma \"<y>\"|delta} [<z>]\n"+
		"tool=\"toy\"\ntrue")
	return f
}

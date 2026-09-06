package ecocheck_test

import (
	"fmt"
	"strings"
	"testing"

	ecocheck "kk-flavor/tools/eco-check"
	"kk-flavor/tools/shell"
)

// The heads are bound to their constants for the reason harness_test.go's block gives. The tails
// below them are this file's own.
const (
	noCallSite = ecocheck.SubcommandWithNoCallSite
	weldedName = ecocheck.SubcommandCallSitesNotChecked
	// Every way this scan can hold a script with subcommands and name none of them says so under one
	// class prefix, and report.go ranks on that prefix. `toyIsUnread` carries the fixture script's own
	// path after it, so a case pins the file it is about and not merely that something went unread.
	unreadable       = ecocheck.UnreadDispatch
	toyIsUnread      = "/skills/toy.sh ("
	acceptsUnnamed   = ecocheck.SubcommandUsageDoesNotName
	namesUnaccepted  = ecocheck.SubcommandDispatchDoesNotAccept
	unreadArms       = "(it opens a case dispatch on $1 and no arm of it could be read)"
	noWayToADispatch = `it names no tool="<name>" to reach one through, and opens no case dispatch on $1`
)

// The tool source the stub below execs: a dispatch, found by the `usage: toy.sh {` line its refusing
// arm carries rather than by the name of the function around it.
var toyDispatch = newDispatch("toy.sh {alpha|beta}", "alpha", "beta")

// The same dispatch with a decoy switch written ahead of it. The decoy's arms are string literals of
// the same shape, so only the usage line in the refusing arm tells the two apart — which is what a
// scan taking the first switch it finds would get wrong.
var toyDecoyAndDispatch = strings.Replace(newDispatch("toy.sh {alpha}", "alpha"), dispatchFunc,
	`func (r *run) stageOf(name string) string {
	switch name {
	case "pending":
		return "p"
	case "closed":
		return "c"
	}
	return ""
}

`+dispatchFunc, 1)

func TestGoDispatchSubcommandCallSites(t *testing.T) {
	t.Run("fires on a Go-dispatched subcommand no file documents a call site for", func(t *testing.T) {
		newToolStub(t, "toy.sh {alpha|beta}", toyDispatch).reports(noCallSite + "beta")
	})

	t.Run("accepts one a document does name", func(t *testing.T) {
		f := newToolStub(t, "toy.sh {alpha|beta}", toyDispatch)
		f.write(f.root+"/kk-flavor/standards/how.md", "run `toy.sh beta` when the stage returns\n")
		f.doesNotReport(noCallSite + "beta")
	})

	t.Run("reads a grammar wrapped across comment lines", func(t *testing.T) {
		newWrappedGrammarStub(t).reports(noCallSite + "gamma")
	})

	t.Run("and agrees with the dispatch about the wrapped half", func(t *testing.T) {
		newWrappedGrammarStub(t).doesNotReport(acceptsUnnamed)
	})

	t.Run("takes labels from the dispatch and not from another switch beside it", func(t *testing.T) {
		newToolStub(t, "toy.sh {alpha}", toyDecoyAndDispatch).doesNotReport(noCallSite + "pending")
	})

	// Without this the case above passes on a run that read no dispatch at all.
	t.Run("while still taking the dispatch's own (control for the case above)", func(t *testing.T) {
		newToolStub(t, "toy.sh {alpha}", toyDecoyAndDispatch).reports(noCallSite + "alpha")
	})
}

// Either way the scan can fail to read a dispatch, it has to say which way, and then check what it
// does know rather than nothing.
func TestADispatchThatCannotBeReadIsReported(t *testing.T) {
	t.Run("fires when the tool ships no source to read a dispatch out of", func(t *testing.T) {
		f := newStubWithoutSource(t)
		f.reports(unreadable + f.root + toyIsUnread)
	})

	t.Run("and names that as the way it could not read it", func(t *testing.T) {
		newStubWithoutSource(t).reports("no source directory at")
	})

	t.Run("and still checks the subcommands the usage names", func(t *testing.T) {
		newStubWithoutSource(t).reports(noCallSite + "beta")
	})

	t.Run("fires when the source holds no switch carrying the stub's usage line", func(t *testing.T) {
		f := newStubWithUnmarkedSource(t)
		f.reports(unreadable + f.root + toyIsUnread)
	})

	t.Run("and names that as the way it could not read it", func(t *testing.T) {
		newStubWithUnmarkedSource(t).reports("no switch under")
	})

	// A stub naming no grammar over a tool holding no dispatch takes no subcommand. That is a
	// determination, not a failure.
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
		newToolStub(t, "toy.sh {alpha}", toyDispatch).reports(acceptsUnnamed + "beta")
	})

	t.Run("fires on one the usage names and the dispatch does not accept", func(t *testing.T) {
		newToolStub(t, "toy.sh {alpha|beta|gamma}", toyDispatch).reports(namesUnaccepted + "gamma")
	})

	t.Run("stays quiet when the two agree", func(t *testing.T) {
		newToolStub(t, "toy.sh {alpha|beta}", toyDispatch).doesNotReport("subcommand its usage")
	})
}

// The bound has to *report* what it dropped, never quietly check fewer than it looks like it checked.
func TestSubcommandCountIsBounded(t *testing.T) {
	// The usage names every arm the dispatch has, deliberately: a fixture where the two lists disagree
	// floods the one finding class both cases read out of, and the control below would then fail on
	// the flood rather than on the bound.
	newFloodedDispatch := func(t *testing.T) *fixture {
		var names []string
		for i := 1; i <= 300; i++ {
			names = append(names, fmt.Sprintf("s%03d", i))
		}
		usage := "toy.sh {" + strings.Join(names, "|") + "}"
		return newToolStub(t, usage, newDispatch(usage, names...))
	}

	t.Run("reports the subcommands it did not carry", func(t *testing.T) {
		newFloodedDispatch(t).reports("were NOT checked")
	})

	// Presence alone is not enough here, and it was all this pinned: the notice reached the screen
	// because it sorted ahead of the basename the findings led with, and the day those findings led
	// with a path instead, the per-rank cap dropped the one line saying the scan had stopped checking.
	t.Run("and ranks that notice above the findings it qualifies", func(t *testing.T) {
		newFloodedDispatch(t).ranksAbove("were NOT checked", noCallSite)
	})

	// Without this the case above passes on a run that checked none of them.
	t.Run("and carries the ones up to the bound (control for the case above)", func(t *testing.T) {
		newFloodedDispatch(t).reports(noCallSite + "s001")
	})
}

// A call site is written as a basename, so that is the search token; the attribution is not. Two
// scripts under one name had their dispatches read into a single set, and a call site for either then
// answered for both — the cheapest mute the scan has, since committing a second `report.sh` anywhere
// under the root silently stops every subcommand of the first from being checked.
func TestTwoScriptsUnderOneNameAreReportedNotWelded(t *testing.T) {
	newSharedScriptName := func(t *testing.T) *fixture {
		f := newToolStub(t, "toy.sh {alpha|beta}", toyDispatch)
		f.newScript("other/scripts/toy.sh", "#!/usr/bin/env bash\n# untested: fixture\ntrue")
		return f
	}

	t.Run("reports the scripts sharing a name", func(t *testing.T) {
		newSharedScriptName(t).reports(weldedName)
	})

	// The count is what says how many, because the printer bounds the line and the last path on a long
	// one can be cut; the first is in a fixed position and is asserted whole.
	t.Run("and names how many and which", func(t *testing.T) {
		f := newSharedScriptName(t)
		f.reports("2 scripts are named toy.sh (" + f.root + "/skills/other/scripts/toy.sh, ")
	})

	// The report replaces the check rather than sitting beside it. A finding that cannot be attributed
	// to one of two files is not printed against either.
	t.Run("and withholds the finding it can no longer attribute", func(t *testing.T) {
		newSharedScriptName(t).doesNotReport(noCallSite + "beta")
	})

	// Without this the case above passes on a fixture whose dispatch was never read.
	t.Run("while one script of that name is checked as before (control)", func(t *testing.T) {
		newToolStub(t, "toy.sh {alpha|beta}", toyDispatch).reports(noCallSite + "beta")
	})

	t.Run("and names that one by path, not by basename", func(t *testing.T) {
		f := newToolStub(t, "toy.sh {alpha|beta}", toyDispatch)
		f.reports(noCallSite + "beta — " + f.root + "/skills/toy.sh")
	})

	// The findings that go on being printed for a welded name. Each is about one stub's own grammar,
	// so each still fires, and each used to lead with the basename: two of them then read identically
	// and collapsed in the sort, leaving one line for two files and no way to tell which. The scan has
	// no answer here, so the finding says how many rather than naming the first and calling it fact.
	t.Run("says a dispatch finding names one of several files rather than guessing", func(t *testing.T) {
		f := newToolStub(t, "toy.sh {alpha|beta|gamma}", toyDispatch)
		f.newScript("other/scripts/toy.sh", "#!/usr/bin/env bash\n# untested: fixture\ntrue")
		f.reports(namesUnaccepted + "gamma — toy.sh (one of the 2 files of that name — which is not in the tree)")
	})

	// Without this the case above passes on a tree where the two stubs never disagreed at all.
	t.Run("while one script of that name is named by its path (control)", func(t *testing.T) {
		f := newToolStub(t, "toy.sh {alpha|beta|gamma}", toyDispatch)
		f.reports(namesUnaccepted + "gamma — " + f.root + "/skills/toy.sh")
	})
}

// Both ways this scan says it could not read a dispatch quote the directory it looked in, and the
// stub names the directory the path is built from — so its length is the reviewed tree's to choose. A
// path cut mid-segment is still a path, so a cut with nothing marking it sends a reader to a
// directory nobody has.
func TestAnUnreadableDispatchPathSaysItWasCut(t *testing.T) {
	// Past the 120-byte bound on its own, whatever TMPDIR the run has: on a machine with a long one
	// the fixture root already runs past it, on a short one only this name does.
	newLongToolName := func(t *testing.T) *fixture {
		t.Helper()
		f := newRoot(t)
		f.newScript("toy.sh", "#!/usr/bin/env bash\n# untested: fixture\n#   usage: toy.sh {alpha|beta}\n"+
			"tool=\""+strings.Repeat("z", 200)+"\"\ntrue")
		return f
	}

	t.Run("names the directory it could not read (control for the case below)", func(t *testing.T) {
		newLongToolName(t).reports("no source directory at ")
	})

	// Matched on the marker together with the text the finding puts after the path, so the assertion
	// is about where the cut is reported rather than about a "..." landing anywhere in the output.
	t.Run("and marks the path it cut rather than naming a shorter wrong one", func(t *testing.T) {
		newLongToolName(t).reports(shell.CutMarker + ") — the 2 subcommand(s)")
	})
}

// The name each of these three findings leads with is a case label or a usage alternative: the
// branch's own text, and as long as it likes. Cut only at the printer's 500-byte bound, the tail that
// goes is the path and the sentence naming the defect. The reader is left a name and no file.
//
// One case per site, because each reaches its name through a different half of the scan. Each asserts
// the marker together with the text that has to survive it.
func TestALongSubcommandNameIsCutBeforeItsAttribution(t *testing.T) {
	long := strings.Repeat("z", 200)

	t.Run("marks a cut call-site name and keeps the path after it", func(t *testing.T) {
		f := newShellScript(t, "case \"$1\" in\n  "+long+")\n    :\n    ;;\nesac")
		f.reports(shell.CutMarker + " — " + f.root + "/skills/toy.sh takes it")
	})

	t.Run("and marks one the dispatch accepts and the usage does not name", func(t *testing.T) {
		usage := "toy.sh {alpha}"
		f := newToolStub(t, usage, newDispatch(usage, "alpha", long))
		f.reports(shell.CutMarker + " — " + f.root + "/skills/toy.sh accepts it")
	})

	t.Run("and marks one the usage names and the dispatch does not accept", func(t *testing.T) {
		usage := "toy.sh {alpha|" + long + "}"
		f := newToolStub(t, usage, newDispatch(usage, "alpha"))
		f.reports(shell.CutMarker + " — " + f.root + "/skills/toy.sh names it in its usage")
	})

	// Without this the three above would pass on a checker that marked every name, cut or not.
	t.Run("and leaves a name that fits whole, with no mark on it", func(t *testing.T) {
		f := newToolStub(t, "toy.sh {alpha|beta}", toyDispatch)
		f.doesNotReport(noCallSite + "beta" + shell.CutMarker)
	})
}

// The line every fixture's dispatch opens on, and the anchor the decoy above is spliced in front of.
const dispatchFunc = "func (r *run) dispatch() {"

// The `package toy` source of a tool whose dispatch accepts exactly `names` and refuses with `usage`.
// The scan finds the switch by that usage line and never by the function around it, so `usage` is the
// caller's to state: a builder writing its own refuses under a grammar the case never set up, and the
// case then observes a disagreement it did not build.
func newDispatch(usage string, names ...string) string {
	var source strings.Builder
	source.WriteString("package toy\n\n" + dispatchFunc + "\n\tswitch r.arg(0) {\n")
	for _, name := range names {
		fmt.Fprintf(&source, "\tcase %q:\n\t\tr.run()\n", name)
	}
	fmt.Fprintf(&source, "\tdefault:\n\t\tr.refuse(%q)\n\t}\n}\n", "usage: "+usage)
	return source.String()
}

// A shell dispatch is the same dispatch in every spelling of its opening line, and an author reaches
// for whichever one they like. A scan matching one literal checks the subcommands of the spelling it
// knows and says nothing whatever about a script written any other way.
//
// The whole table, not the spellings a scenario happened to name — this is a mapping from a line to
// whether a dispatch is there, and a row left out is a row nothing observes.
func TestAShellDispatchIsReadInEverySpellingOfItsOpening(t *testing.T) {
	for _, opening := range []string{
		`case "${1:-}" in`,
		`case "$1" in`,
		`case $1 in`,
		`case "${1-}" in`,
		`case "${1}" in`,
		`case "${1:-help}" in`,
	} {
		t.Run("reads a dispatch opened with "+opening, func(t *testing.T) {
			f := newShellDispatch(t, opening)
			// The whole finding, path and subcommand together: `beta` is a name other fixtures in this
			// file produce too, and a bare substring would pass on any of their findings.
			f.reports(noCallSite + "beta — " + f.root + "/skills/toy.sh")
		})
	}
}

// The silence one line further in than the case above. The opening matched, no arm did, and the scan
// then held a dispatch it could name no subcommand of — which it reported as nothing at all, the one
// answer indistinguishable from a script that has no dispatch.
func TestADispatchWhoseArmsCannotBeReadIsReported(t *testing.T) {
	// Arm bodies on the arms' own lines. Legal shell, written all over this tree, and a form the arm
	// pattern does not read — it wants the line to end at the `)`.
	newUnreadableArms := func(t *testing.T) *fixture {
		t.Helper()
		return newShellScript(t, "case \"$1\" in\n  alpha) : ;;\n  beta) : ;;\nesac")
	}

	t.Run("fires on a dispatch it could read no arm of", func(t *testing.T) {
		f := newUnreadableArms(t)
		f.reports(unreadable + f.root + "/skills/toy.sh " + unreadArms)
	})

	// Without this the case above passes on a scan that reports it over every dispatch in the tree.
	t.Run("and stays quiet on one whose arms it did read (control)", func(t *testing.T) {
		newShellDispatch(t, `case "$1" in`).doesNotReport(unreadArms)
	})

	t.Run("and stays quiet on a script that opens no dispatch at all", func(t *testing.T) {
		newShellScript(t, "true").doesNotReport(unreadArms)
	})
}

// What the loosened opening pattern must still refuse. It decides only that a dispatch is *there*, so
// it is deliberately the loosest thing in this file — and a pattern that answers yes to every `case`
// turns two real shapes in this tree into findings: a top-level `case` over a value that is not the
// first argument, and an in-function lookup table, of which `install.sh` holds one.
func TestATopLevelCaseIsNotAlwaysADispatch(t *testing.T) {
	t.Run("reads no dispatch out of a top-level case over another value", func(t *testing.T) {
		f := newShellScript(t, "flag=\"$2\"\ncase \"$flag\" in\n  alpha)\n    :\n    ;;\nesac")
		f.doesNotReport(noCallSite + "alpha — " + f.root + "/skills/toy.sh")
	})

	t.Run("nor out of a lookup table inside a function", func(t *testing.T) {
		f := newShellScript(t, "route() {\n  case \"$1\" in\n    alpha) : ;;\n  esac\n}\ntrue")
		f.doesNotReport(unreadArms)
	})

	// Without the two above this file would pass on a pattern matching every line, and without this
	// one they would pass on a pattern matching none.
	t.Run("while a dispatch on the first argument still is one (control)", func(t *testing.T) {
		f := newShellDispatch(t, `case "$1" in`)
		f.reports(noCallSite + "alpha — " + f.root + "/skills/toy.sh")
	})
}

// The same silence reached through the stub half, which reads `tool="<name>"` in one spelling of its
// own. A stub spelling it any other way names its subcommands in a usage grammar an agent reads, and
// nothing whatever checks them: no tool to find a Go dispatch through, and no `case` of its own.
func TestAUsageGrammarWithNoDispatchBehindItIsReported(t *testing.T) {
	// `tool='toy'` — legal shell, and a spelling the tool-declaration pattern does not read.
	newUnreachableDispatch := func(t *testing.T) *fixture {
		t.Helper()
		f := newRoot(t)
		f.mkdirAll(f.root + "/tools/toy")
		f.write(f.root+"/tools/toy/toy.go", toyDispatch)
		f.newScript("toy.sh", "#!/usr/bin/env bash\n# untested: fixture\n#   usage: toy.sh {alpha|beta}\ntool='toy'\ntrue")
		return f
	}

	t.Run("fires when nothing in the script reaches a dispatch", func(t *testing.T) {
		f := newUnreachableDispatch(t)
		f.reports(unreadable + f.root + toyIsUnread)
	})

	t.Run("and names that as the way it could not read it", func(t *testing.T) {
		newUnreachableDispatch(t).reports(noWayToADispatch)
	})

	t.Run("and still checks the subcommands its usage names", func(t *testing.T) {
		newUnreachableDispatch(t).reports(noCallSite + "beta")
	})

	// The two determinations that stay quiet. Each is an answer this scan reached, not one it failed
	// to: a script whose own dispatch was read has its subcommands from there, and a script naming no
	// grammar at all takes no subcommand (check.sh and stats.sh each take a root).
	t.Run("stays quiet on a script whose own shell dispatch was read", func(t *testing.T) {
		// cadence.sh's shape: a usage grammar and a dispatch, and no tool= line at all.
		f := newShellScript(t, "#   usage: toy.sh {alpha|beta}\ncase \"$1\" in\n"+
			"  alpha)\n    :\n    ;;\n  beta)\n    :\n    ;;\nesac")
		f.doesNotReport(unreadable)
	})

	t.Run("stays quiet on a script whose usage names no subcommand", func(t *testing.T) {
		newShellScript(t, "#   usage: toy.sh [<root>]\ntrue").doesNotReport(unreadable)
	})
}

// A line saying this scan checked nothing about a file has to survive a tree that floods the report.
// report.go ranks a finding on the head of its line, and every finding in this class leads with the
// path of the file it is about — so unranked they fall to rank 5, share one budget with `dangling
// link:` and sort below every one of them. 300 crafted links then hide the line, and the report reads
// clean of it: the silence this whole class exists to end, restored one layer up. The case lives
// beside the scan rather than in report_test.go because what it pins is this class's rank, and a rank
// entry naming a class is only ever read together with the class.
func TestAnUnreadDispatchSurvivesAFlood(t *testing.T) {
	newUnreadDispatchUnderAFlood := func(t *testing.T) *fixture {
		t.Helper()
		f := newStubWithoutSource(t)
		f.floodWithLinks(f.root+"/kk-flavor/standards/flood.md", 300, "[x](nope%03d.md)")
		return f
	}

	t.Run("shows it through a flood of link findings", func(t *testing.T) {
		f := newUnreadDispatchUnderAFlood(t)
		f.reports(unreadable + f.root + toyIsUnread)
	})

	// Presence alone passes again the day the rank is dropped and the flood lands one line short of
	// the per-rank cap, so the ordering is what this pins.
	t.Run("and ranks it above that flood rather than inside it", func(t *testing.T) {
		newUnreadDispatchUnderAFlood(t).ranksAbove(unreadable, "dangling link: ")
	})
}

// One script under `skills/`, with the header every script in this tree carries.
func newShellScript(t *testing.T, body string) *fixture {
	t.Helper()
	f := newRoot(t)
	f.newScript("toy.sh", "#!/usr/bin/env bash\n# untested: fixture\n"+body)
	return f
}

// A dispatch accepting `alpha` and `beta`, opened with the given line. Nothing in the tree cites
// either, so both are findings and the case picks the one it means.
func newShellDispatch(t *testing.T, opening string) *fixture {
	t.Helper()
	return newShellScript(t, opening+"\n  alpha)\n    :\n    ;;\n  beta)\n    :\n    ;;\nesac")
}

// A stub reaching a Go tool, and that tool's source: the shape report.sh and eco-report have.
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
	// A real dispatch, refusing under a different script's name: the marker is what says which stub a
	// dispatch belongs to, and no other line in the file does.
	return newToolStub(t, "toy.sh {alpha|beta}",
		strings.Replace(toyDispatch, "usage: toy.sh {", "usage: other.sh {", 1))
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
	f.write(f.root+"/tools/toy/toy.go",
		newDispatch(`toy.sh {alpha <x>|beta|gamma "<y>"|delta} [<z>]`, "alpha", "beta", "gamma", "delta"))
	f.newScript("toy.sh", "#!/usr/bin/env bash\n# untested: fixture\n"+
		"#   usage: toy.sh {alpha <x>|beta|\n"+
		"#                  gamma \"<y>\"|delta} [<z>]\n"+
		"tool=\"toy\"\ntrue")
	return f
}

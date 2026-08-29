package ecoreport_test

// Which report an invocation acts on. The intent reaches a path here, and a report the discovery
// paths cannot see is a ship that stands open while `state` answers `no-report` and `idsd-ship
// continue` starts a fresh one over live work.

import (
	"strconv"
	"testing"
)

func TestADotNamedReportIsInvisibleToEveryDiscoveryPath(t *testing.T) {
	t.Parallel()
	// `init` refuses a leading dot rather than sanitising one, and that refusal is worth something only
	// while the listing really cannot see such a file. Both plants below are made by hand, since `init`
	// is exactly what will not create them.
	f := newShip(t, "001-the-only-ship")
	f.write(f.repo+"/.idsd/qualify-reports/.hidden-qualify-report.md", "---\nintent: 002-hidden\n---\n")
	// A directory named like a report is the other thing the listing must not take for one.
	f.mkdirAll(f.repo + "/.idsd/qualify-reports/003-a-directory-qualify-report.md")

	f.runReport("list")
	f.record("list names the one real report, and neither the dot-named file nor the directory",
		f.out == "001-the-only-ship\tresume", "said:\n"+f.out)

	// Counted, either plant makes every unnamed subcommand refuse as ambiguous — which is the second
	// way a ship stops being reachable, and the one a human meets first.
	f.runReport("state")
	f.record("and the one report still resolves without being named",
		f.status == 0 && f.out == "resume", "exit "+strconv.Itoa(f.status)+"; said '"+f.out+"'")
}

func TestASubcommandRefusesAnIntentNameThatCouldNameNoReport(t *testing.T) {
	t.Parallel()
	// The intent value reaches a path, so a name outside the slug charset has to be refused where it is
	// resolved and not only where it is created: `init` guards its own, and every other subcommand takes
	// the same value as its last argument.
	f := newShip(t, "001-real")
	for _, subcommand := range []string{"gate", "carry", "state", "close", "discard"} {
		f.runReport(subcommand, "../../escaped")
		f.assertRefused(subcommand + " refuses an intent name that could name no report")
		// The message, not the exit: without this guard each of these still refuses, further along and
		// for a reason that is not the name it was handed.
		f.assertReports("names no report", "and "+subcommand+" names the value as what it refused")
	}
	f.record("and the one real report is untouched", f.isFile(f.reportPath("001-real")), "")
}

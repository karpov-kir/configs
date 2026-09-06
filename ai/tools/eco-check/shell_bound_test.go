package ecocheck_test

// The per-file read bound, whose reason is shell.MaxFileBytes, and the other way a read comes back
// with nothing: a file the walk handed over and the open refused. Both are the same claim — a file
// nothing read must not be indistinguishable from one that held nothing.

import (
	"testing"

	ecocheck "kk-flavor/tools/eco-check"
)

// A markdown file the walk reaches and the read cannot open. Root reads a mode-000 file whatever the
// mode says, and nothing else builds this condition: every other way to make the read fail is refused
// a limb earlier by the walk's own regular-file test and never reaches the open. So the case declines
// rather than asserting against a file the checker reads happily.
func newUnreadableStandard(t *testing.T) *fixture {
	t.Helper()
	skipUnlessModeDeniesRead(t, "an unreadable file cannot be built here")
	f := newRoot(t)
	f.write(f.root+"/kk-flavor/standards/shut.md", "# Shut\n\n## One home\n")
	f.chmod(f.root+"/kk-flavor/standards/shut.md", 0o000)
	return f
}

func TestUnreadableFileIsReportedNotSilentlySkipped(t *testing.T) {
	t.Run("names a file the read could not open", func(t *testing.T) {
		newUnreadableStandard(t).reports(ecocheck.FileCouldNotBeRead)
	})

	t.Run("and says it was not checked rather than leaving it to read as empty", func(t *testing.T) {
		newUnreadableStandard(t).reports("it was NOT checked")
	})

	// It ranks with the tampered-check class, not with the references. Left at the default rank it
	// shares one budget with `dangling link:` and sorts below every one of them — and a flood of those
	// is the cheapest thing a branch can commit.
	t.Run("and reaches the screen through a flood of link findings", func(t *testing.T) {
		f := newUnreadableStandard(t)
		f.floodWithLinks(f.root+"/kk-flavor/standards/flood.md", 300, "[x](nope%03d.md)")
		f.ranksAbove(ecocheck.FileCouldNotBeRead, ecocheck.DanglingLink)
	})
}

func TestOversizeFileIsReportedNotRead(t *testing.T) {
	oversize := func(t *testing.T) *fixture {
		f := newRoot(t)
		f.writeOversize(f.root + "/kk-flavor/standards/huge.md")
		return f
	}

	t.Run("reports a file past the read bound", func(t *testing.T) {
		oversize(t).reports(ecocheck.FileTooLargeToScan)
	})

	// An unchecked file must never look like a checked one, so the finding has to say plainly that
	// nothing read it.
	t.Run("and says it was not checked rather than reading part of it", func(t *testing.T) {
		oversize(t).reports("it was NOT checked")
	})
}

// The same bound on the other read. The case above lists nothing, so it only ever reached the
// line-oriented read. The budget counts its files through a second read, and that one had no bound at
// all: a 600 MB standard listed under Read always took 636 MB resident. That is the OOM this bound
// exists to stop, reached through the one path that did not apply it.
func TestOversizeBudgetFileIsReportedNotCounted(t *testing.T) {
	listed := func(t *testing.T) *fixture {
		f := newRoot(t)
		f.writeOversize(f.root + "/kk-flavor/standards/huge.md")
		f.write(f.root+"/kk-flavor/inject.md",
			"# Flavor\n\n## Read always\n\n- [standards/huge.md](standards/huge.md)\n")
		return f
	}

	t.Run("reports a budget file past the bound", func(t *testing.T) {
		listed(t).reports(ecocheck.FileTooLargeToScan)
	})

	// Counted but unread is the shape that lies. The census line would carry a figure for a file
	// nothing opened, so the finding says which of the two happened.
	t.Run("and says it was not counted", func(t *testing.T) {
		listed(t).reports("it was NOT counted")
	})
}

// The third read of a whole file the reviewed tree wrote. The call-site pass searches every .md, .sh
// and .yaml under the root for a `<script> <subcommand>` literal, and it had no bound: a 600 MB
// standard took 1.27 GB resident — twice the file, because the bytes were copied into a string to
// search them. It runs whenever some script carries a dispatch, which every real tree has.
func TestOversizeFileIsNotReadByTheCallSiteScan(t *testing.T) {
	searched := func(t *testing.T) *fixture {
		f := newRoot(t)
		// A dispatch, or the call-site pass never runs and this case observes nothing.
		f.newScript("d.sh", "#!/usr/bin/env bash\n# untested: fixture\ncase \"${1:-}\" in\n  alpha)\n    true\n    ;;\nesac")
		f.writeOversize(f.root + "/kk-flavor/standards/huge.md")
		return f
	}

	t.Run("reports the file it did not search", func(t *testing.T) {
		searched(t).reports("no call site in it was seen")
	})

	t.Run("while the scan itself is live on that tree", func(t *testing.T) {
		searched(t).reports(noCallSite + "alpha — ")
	})
}

// The same consequence reached the other way. This pass reads whole files rather than lines, so it
// has its own open and readLines' report does not cover it: a file it could not open left every call
// site in it unseen, and the subcommands those sites answer for were reported as having none.
func TestUnreadableFileIsNotSearchedForCallSitesInSilence(t *testing.T) {
	searched := func(t *testing.T) *fixture {
		skipUnlessModeDeniesRead(t, "an unreadable file cannot be built here")
		f := newRoot(t)
		// A dispatch, or the call-site pass never runs and this case observes nothing.
		f.newScript("d.sh", "#!/usr/bin/env bash\n# untested: fixture\ncase \"${1:-}\" in\n  alpha)\n    true\n    ;;\nesac")
		f.write(f.root+"/kk-flavor/standards/shut.md", "run `d.sh alpha` at the close\n")
		f.chmod(f.root+"/kk-flavor/standards/shut.md", 0o000)
		return f
	}

	t.Run("reports the file it could not search", func(t *testing.T) {
		searched(t).reports("no call site in it was seen")
	})

	// The control, and the thing that makes the fixture worth building: the unread file holds the one
	// call site `alpha` has, so the run reports a subcommand as uncalled on evidence it never read.
	// Both findings have to be there for a reader to tell those two facts apart.
	t.Run("while still reporting the subcommand it could not find a site for", func(t *testing.T) {
		searched(t).reports(noCallSite + "alpha — ")
	})
}

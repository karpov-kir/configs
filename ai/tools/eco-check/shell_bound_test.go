package ecocheck_test

// The per-file read bound. The reviewed tree chooses how large a committed file is, and reading one
// whole put peak memory at 2.48 GB for a 64 MiB file that packs to 408 KB — half a gigabyte of it
// OOM-kills the review stage.

import "testing"

func TestOversizeFileIsReportedNotRead(t *testing.T) {
	oversize := func(t *testing.T) *fixture {
		f := newRoot(t)
		f.writeOversize(f.root + "/kk-flavor/standards/huge.md")
		return f
	}

	t.Run("reports a file past the read bound", func(t *testing.T) {
		oversize(t).reports("file too large to scan")
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
		listed(t).reports("file too large to scan")
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

	// The control. The pass is live on this fixture, so the silence above is a refusal and not a scan
	// that never ran.
	t.Run("while the scan itself is live on that tree", func(t *testing.T) {
		searched(t).reports("subcommand with no call site: alpha")
	})
}

package ecocheck_test

// The per-file read bound. The reviewed tree chooses how large a committed file is, and reading one
// whole put peak memory at 2.48 GB for a 64 MiB file that packs to 408 KB — half a gigabyte of it
// OOM-kills the review stage. The bound went in without a case, and the mutation harness said so:
// removing it killed nothing.

import "testing"

func TestOversizeFileIsReportedNotRead(t *testing.T) {
	// Just past the 8 MiB bound, and made of newlines because that is the cheap shape — a branch commits
	// almost nothing and the checker allocates a slice header per line.
	oversize := func(t *testing.T) *fixture {
		f := newRoot(t)
		body := make([]byte, (8<<20)+1)
		for i := range body {
			body[i] = '\n'
		}
		f.write(f.root+"/kk-flavor/standards/huge.md", string(body))
		return f
	}

	t.Run("reports a file past the read bound", func(t *testing.T) {
		oversize(t).reports("file too large to scan")
	})

	// The half that matters: an unchecked file must never be indistinguishable from a checked one, so
	// the finding has to say plainly that nothing read it.
	t.Run("and says it was not checked rather than reading part of it", func(t *testing.T) {
		oversize(t).reports("it was NOT checked")
	})
}

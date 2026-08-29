package ecocheck_test

import (
	"strings"
	"testing"
)

// A link target is written by the branch under review, and `..` is in its charset, so the scan used
// to stat whatever path the branch named. A dangling link is reported only when the target is
// missing, so the silence told the branch's author which files the reviewing machine holds. Both
// answers have to look the same now.
func TestATraversalLinkIsNotStatted(t *testing.T) {
	// One target that exists outside the root, one that does not. Both have to come back reported the
	// same way, with nothing in the output saying which is which.
	newProbe := func(t *testing.T) (*fixture, string, string) {
		f := newRoot(t)
		f.write(f.base+"/present.md", "# here\n")
		up := "../../../../../../../../../../../../../../../../../../../../"
		real := up + strings.TrimPrefix(f.base+"/present.md", "/")
		fake := up + strings.TrimPrefix(f.base+"/absent.md", "/")
		f.write(f.root+"/kk-flavor/standards/probe.md",
			"- [a]("+real+")\n- [b]("+fake+")\n")
		return f, real, fake
	}

	t.Run("reports a traversal to a file that is not there", func(t *testing.T) {
		f, _, fake := newProbe(t)
		f.reports("dangling link: " + f.root + "/kk-flavor/standards/probe.md -> " + fake)
	})

	// The half that closes the oracle. Without it the present target goes unreported, and that gap
	// between the two findings is the leak.
	t.Run("and reports the one that is there identically", func(t *testing.T) {
		f, real, _ := newProbe(t)
		f.reports("dangling link: " + f.root + "/kk-flavor/standards/probe.md -> " + real)
	})

	// The control. Without it, a scan that reported every link would pass here just as well as one
	// that stops asking about paths outside the tree.
	t.Run("while an in-root link still resolves", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/standards/sibling.md", "# S\n")
		f.write(f.root+"/kk-flavor/standards/probe.md", "- [a](sibling.md)\n- [b](../inject.md)\n")
		f.doesNotReport("dangling link: " + f.root + "/kk-flavor/standards/probe.md")
	})
}

// The same refusal in the other resolver, which three scans share. A `~/.kk-flavor/` ref carries
// `..` in its charset too, so it reached that same stat.
func TestATraversalHomeRefIsNotStatted(t *testing.T) {
	f := newRoot(t)
	f.write(f.base+"/present.md", "# here\n")
	up := "../../../../../../../../../../../../../../../../../../../../"
	f.write(f.root+"/kk-flavor/standards/probe.md",
		"see `~/.kk-flavor/"+up+strings.TrimPrefix(f.base+"/present.md", "/")+"`\n")
	f.reports("dangling home ref: ~/.kk-flavor/" + up)
}

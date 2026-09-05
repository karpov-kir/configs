package ecocheck_test

import (
	"path/filepath"
	"strings"
	"testing"

	ecocheck "kk-flavor/tools/eco-check"
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
		f.reports(ecocheck.DanglingLink + f.root + "/kk-flavor/standards/probe.md -> " + fake)
	})

	// The half that closes the oracle. Without it the present target goes unreported, and that gap
	// between the two findings is the leak.
	t.Run("and reports the one that is there identically", func(t *testing.T) {
		f, real, _ := newProbe(t)
		f.reports(ecocheck.DanglingLink + f.root + "/kk-flavor/standards/probe.md -> " + real)
	})

	// The control. Without it, a scan that reported every link would pass here just as well as one
	// that stops asking about paths outside the tree.
	t.Run("while an in-root link still resolves", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/standards/sibling.md", "# S\n")
		f.write(f.root+"/kk-flavor/standards/probe.md", "- [a](sibling.md)\n- [b](../inject.md)\n")
		f.doesNotReport(ecocheck.DanglingLink + f.root + "/kk-flavor/standards/probe.md")
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
	f.reports(ecocheck.DanglingHomeRef + "~/.kk-flavor/" + up)
}

// One tree, every way `check.sh` lets a caller spell its root, and the same answer required from each.
// The shape they disagreed on is a citation whose first component is the root's own directory name;
// tree.go's suffix index says why, and carries the example. Each spelling is one a caller produces:
// `.` is the first of the two candidates a bare `check.sh` tries, so it is what running the script
// from inside the tree lands on, and a trailing slash is what completing a directory name at a shell
// prompt produces.
func TestAPathRefThroughTheRootsOwnNameResolvesHoweverTheRootIsNamed(t *testing.T) {
	// The root's directory is `r`, so `r/sibling.md` is that shape here. Each needle is the whole
	// finding, echoed file included. A needle built from the head and the tail alone matches no line
	// the checker ever prints, and every case below would then pass over any behaviour at all.
	finding := func(root string) string {
		return ecocheck.DanglingPathRef + root + "/kk-flavor/standards/probe.md -> r/sibling.md"
	}
	newSpellingProbe := func(t *testing.T) *fixture {
		f := newRoot(t)
		f.write(f.root+"/sibling.md", "# S\n")
		f.write(f.root+"/kk-flavor/standards/probe.md", "see `r/sibling.md`\n")
		return f
	}

	t.Run("named absolutely", func(t *testing.T) {
		f := newSpellingProbe(t)
		f.doesNotReportWithRootNamed(f.base, f.root, finding(f.root))
	})

	t.Run("named relative to the working directory", func(t *testing.T) {
		f := newSpellingProbe(t)
		f.doesNotReportWithRootNamed(f.base, "r", finding("r"))
	})

	t.Run("named with a leading ./", func(t *testing.T) {
		f := newSpellingProbe(t)
		f.doesNotReportWithRootNamed(f.base, "./r", finding("./r"))
	})

	// The doubled separator is the point: the walk concatenates, so this root's paths are `r//…` and
	// no cut that trims one separator turns them back into the tree's own name.
	t.Run("named with a trailing slash", func(t *testing.T) {
		f := newSpellingProbe(t)
		f.doesNotReportWithRootNamed(f.base, "r/", finding("r/"))
	})

	// `.` names the tree without naming it, so nothing in the spelling carries the directory's name
	// at all. This is what a bare `check.sh` run from inside the tree resolves to.
	t.Run("and named `.` from inside the tree", func(t *testing.T) {
		f := newSpellingProbe(t)
		f.doesNotReportWithRootNamed(f.root, ".", finding("."))
	})

	// The control, and the half that matters most: a resolver that answered yes to everything would
	// pass all of the above while reporting nothing a relative-root run was called to find.
	t.Run("while a citation naming nothing still dangles under the same spelling", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/standards/probe.md", "see `r/absent.md`\n")
		f.reportsWithRootNamed(f.base, "r", ecocheck.DanglingPathRef+"r/kk-flavor/standards/probe.md -> r/absent.md")
	})

	// And the other half of the control: a bare basename is the shape prose writes most often, and it
	// resolves through a tail of the canonical name rather than the name itself. Without this, an
	// index that kept only whole canonical names would pass every case above.
	t.Run("while a bare basename still resolves under that spelling", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/sibling.sh", "#!/usr/bin/env bash\n")
		f.write(f.root+"/kk-flavor/standards/probe.md", "see `sibling.sh`\n")
		f.doesNotReportWithRootNamed(f.root, ".", ecocheck.DanglingPathRef+"./kk-flavor/standards/probe.md -> sibling.sh")
	})
}

// The other direction of that rule, and the security half of it. A citation resolves through the root's
// own name and no further up; tree.go's rootName carries what an anchor reaching higher would leak.
// Held here rather than beside the traversal cases above because it is underRoot's probe arriving
// through the index instead of through a stat.
func TestACitationNamingADirectoryAboveTheRootDoesNotResolve(t *testing.T) {
	f := newRoot(t)
	f.write(f.root+"/sibling.md", "# S\n")
	// The two names directly above the root, which the absolute spelling used to answer for.
	parent := filepath.Base(f.base)
	grandparent := filepath.Base(filepath.Dir(f.base))
	f.write(f.root+"/kk-flavor/standards/probe.md",
		"see `"+parent+"/r/sibling.md` and `"+grandparent+"/"+parent+"/r/sibling.md`\n")

	head := ecocheck.DanglingPathRef + f.root + "/kk-flavor/standards/probe.md -> "
	output := f.run()
	f.found(output, head+parent+"/r/sibling.md")
	f.found(output, head+grandparent+"/"+parent+"/r/sibling.md")
	// The control: the root's own name still resolves, so this case cannot pass by reporting everything.
	f.absent(output, head+"r/sibling.md")
}

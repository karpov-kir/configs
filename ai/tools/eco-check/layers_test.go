package ecocheck_test

import (
	"strings"
	"testing"

	ecocheck "configs/ai/tools/eco-check"
)

func (f *fixture) newStandard(name, declaration, body string) {
	f.t.Helper()
	f.write(f.root+"/kk-flavor/standards/"+name+".md", declaration+"# "+name+"\n\n## Rule\n\n"+body+"\n")
}

func citation(target string) string {
	return "See [" + target + ".md](" + target + ".md) → **Rule**."
}

func TestAStandardMustDeclareOneOfTheThreeLayers(t *testing.T) {
	t.Run("a standard with no declaration is named", func(t *testing.T) {
		f := newRoot(t)
		f.newStandard("quiet", "", "nothing to see")
		f.reports(ecocheck.StandardWithoutLayer+f.root+"/kk-flavor/standards/quiet.md", "base|craft|process")
	})

	t.Run("and a word outside the three is refused by name, not read as an absent line", func(t *testing.T) {
		f := newRoot(t)
		f.newStandard("odd", "**Layer:** foundation\n\n", "nothing to see")
		output := f.run()
		f.found(output, ecocheck.UnreadableLayer+f.root+"/kk-flavor/standards/odd.md", "`foundation`")
		f.absent(output, ecocheck.StandardWithoutLayer)
	})

	t.Run("and a declared standard is not reported", func(t *testing.T) {
		f := newRoot(t)
		f.newStandard("settled", layerLine, "nothing to see")
		f.doesNotReport(ecocheck.StandardWithoutLayer, ecocheck.UnreadableLayer)
	})
}

func TestAPathTheWalkWillNotEnterRefusesInsteadOfReportingClean(t *testing.T) {
	crossLayerPair := func(f *fixture, dir string) {
		f.mkdirAll(f.root + "/kk-flavor/standards/" + dir)
		f.write(f.root+"/kk-flavor/standards/"+dir+"/high.md",
			"**Layer:** craft\n\n# high\n\n## Rule\n\nSee [../low.md](../low.md) → **Rule**.\n")
		f.write(f.root+"/kk-flavor/standards/low.md",
			"**Layer:** base\n\n# low\n\n## Rule\n\nSee ["+dir+"/high.md]("+dir+"/high.md) → **Rule**.\n")
	}

	t.Run("a real subdirectory holding the cycle is reported, which is the control", func(t *testing.T) {
		f := newRoot(t)
		crossLayerPair(f, "architecture")
		f.reports(ecocheck.LayerCrossingCycle)
	})

	// The exploit: the same tree with `architecture` committed as a symlink. Agents open
	// `standards/architecture/core.md` normally; this scan never walks in.
	t.Run("and the same cycle behind a symlinked subdirectory refuses rather than passing", func(t *testing.T) {
		f := newRoot(t)
		crossLayerPair(f, "real")
		f.symlink(f.root+"/kk-flavor/standards/real", f.root+"/kk-flavor/standards/architecture")
		f.absent(f.refuses(ecocheck.StandardsPathHidden), ecocheck.StandardWithoutLayer)
	})

	t.Run("and a symlinked standard file refuses too", func(t *testing.T) {
		f := newRoot(t)
		f.newStandard("real", layerLine, "nothing to see")
		f.symlink(f.root+"/kk-flavor/standards/real.md", f.root+"/kk-flavor/standards/sneaky.md")
		f.refuses(ecocheck.StandardsPathHidden)
	})

	t.Run("and a standards directory that is itself a symlink refuses", func(t *testing.T) {
		f := newBareRoot(t)
		f.write(f.root+"/kk-flavor/inject.md", "# Flavor\n")
		f.mkdirAll(f.root + "/kk-flavor/real-standards")
		f.symlink(f.root+"/kk-flavor/real-standards", f.root+"/kk-flavor/standards")
		f.refuses(ecocheck.StandardsNotADirectory)
	})

	t.Run("and a standards path that is a regular file refuses too", func(t *testing.T) {
		f := newBareRoot(t)
		f.write(f.root+"/kk-flavor/inject.md", "# Flavor\n")
		f.write(f.root+"/kk-flavor/standards", "not a directory\n")
		f.refuses(ecocheck.StandardsNotADirectory)
	})

	// The evasion the refusal above does not cover, because the link is not under the standards at
	// all: what holds here is nodeNames keying one standard by the file it is rather than the
	// spelling that reached it.
	t.Run("and a cycle whose return hop goes through an alias elsewhere is still reported", func(t *testing.T) {
		f := newRoot(t)
		f.mkdirAll(f.root + "/kk-flavor/standards/architecture")
		f.write(f.root+"/kk-flavor/standards/architecture/high.md",
			"**Layer:** craft\n\n# high\n\n## Rule\n\nSee [../low.md](../low.md) → **Rule**.\n")
		f.write(f.root+"/kk-flavor/standards/low.md",
			"**Layer:** base\n\n# low\n\n## Rule\n\n"+
				"See [../aka/architecture/high.md](../aka/architecture/high.md) → **Rule**.\n")
		f.symlink(f.root+"/kk-flavor/standards", f.root+"/kk-flavor/aka")
		f.reports(ecocheck.LayerCrossingCycle)
	})

	// The same two-spellings defect with no link in it at all, which is the case a volume folding
	// `LOW.md` onto `low.md` produces.
	//
	// The case is written to pass on a case-sensitive volume too, where `LOW.md` is simply a file that
	// is not there: the citation then dangles, which is a finding of its own, and the tree still never
	// reports clean. Either way the run is not silent, which is the property under test.
	t.Run("and a cycle whose hop is spelled in another case is not read as a clean tree", func(t *testing.T) {
		f := newRoot(t)
		f.newStandard("high", "**Layer:** craft\n\n", citation("low"))
		f.newStandard("low", "**Layer:** base\n\n", "See [HIGH.md](HIGH.md) → **Rule**.")
		output := f.run()
		if !strings.Contains(output, ecocheck.LayerCrossingCycle) &&
			!strings.Contains(output, unresolved) && !strings.Contains(output, dangling) {
			t.Errorf("a cycle spelled in two cases went unreported and the citation did not dangle:\n%s", output)
		}
	})

	t.Run("and a standard named with a capitalised extension is still read as one", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/standards/rogue.MD", "# rogue\n\n## Rule\n\nnothing to see\n")
		f.reports(ecocheck.StandardWithoutLayer)
	})

	t.Run("and an ordinary standards tree refuses nothing", func(t *testing.T) {
		f := newRoot(t)
		f.newStandard("settled", layerLine, "nothing to see")
		f.doesNotReport(ecocheck.StandardsPathHidden, ecocheck.StandardsNotADirectory)
	})
}

func TestACitationCycleIsJudgedByTheLayersItRunsThrough(t *testing.T) {
	t.Run("a cycle across two layers is reported, naming the cycle", func(t *testing.T) {
		f := newRoot(t)
		f.newStandard("low", "**Layer:** base\n\n", citation("high"))
		f.newStandard("high", "**Layer:** craft\n\n", citation("low"))
		// Both files, each with the layer it declares, which is what makes the finding readable without
		// opening either. The loop closes on the file it opened on; that third name is past the
		// report's own 500-byte line bound under a temp-directory path, so the case stops at the hop
		// that crosses.
		f.reports(ecocheck.LayerCrossingCycle + f.root + "/kk-flavor/standards/high.md (craft) → " +
			f.root + "/kk-flavor/standards/low.md (base)")
	})

	// The other half of the rule, and the half the shipped tree depends on: three cycles inside one
	// layer are cross-references there today, and a check that reported those would be answered by
	// editing the standards rather than the checker.
	t.Run("and a cycle inside one layer is a cross-reference, not a finding", func(t *testing.T) {
		f := newRoot(t)
		f.newStandard("one", "**Layer:** craft\n\n", citation("two"))
		f.newStandard("two", "**Layer:** craft\n\n", citation("one"))
		f.doesNotReport(ecocheck.LayerCrossingCycle)
	})

	// Upward is not the defect. A lower file may say a higher one binds too; what knots the tree is
	// the citation coming back.
	t.Run("and a citation upward with nothing coming back is no cycle at all", func(t *testing.T) {
		f := newRoot(t)
		f.newStandard("under", "**Layer:** base\n\n", citation("over"))
		f.newStandard("over", "**Layer:** process\n\n", "nothing comes back")
		f.doesNotReport(ecocheck.LayerCrossingCycle)
	})

	// A skill declares no layer, so a loop running through one is a loop nothing here can judge —
	// cite-graph prints those unjudged. Judged anyway, every such loop would be a defect against a
	// division its files are not part of.
	t.Run("and a loop that leaves the standards is left unjudged", func(t *testing.T) {
		f := newRoot(t)
		f.newMountedSkill("kk-between")
		f.write(f.root+"/kk-flavor/skills/kk-between/SKILL.md",
			"---\nname: kk-between\ndescription: stands between two standards\n---\n\n## Rule\n\n"+
				"See [../../standards/far.md](../../standards/far.md) → **Rule**.\n")
		f.newStandard("near", "**Layer:** base\n\n",
			"See [../skills/kk-between/SKILL.md](../skills/kk-between/SKILL.md) → **Rule**.")
		f.newStandard("far", "**Layer:** craft\n\n", citation("near"))
		// The two citation findings are absent for the same reason the cycle one has to be: they say
		// both hops really resolved. Without them a fixture whose paths were wrong would pass this case
		// with no loop in it at all.
		f.absent(f.run(), ecocheck.LayerCrossingCycle, unresolved, dangling)
	})
}

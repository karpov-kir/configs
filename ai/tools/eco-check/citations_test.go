package ecocheck_test

import (
	"strings"
	"testing"

	ecocheck "kk-flavor/tools/eco-check"
	"kk-flavor/tools/shell"
)

func TestDelimitedSectionCitations(t *testing.T) {
	t.Run("fires on a citation whose section is not delimited", func(t *testing.T) {
		newCitation(t, "One home").reports(undelimited)
	})

	t.Run("accepts the bolded form", func(t *testing.T) {
		newCitation(t, "**One home**").doesNotReport(undelimited)
	})

	t.Run("accepts the backticked form, which the parser also reads exactly", func(t *testing.T) {
		newCitation(t, "`One home`").doesNotReport(undelimited)
	})

	// A shell comment cites the same way a document does, and the scan reads both.
	t.Run("fires on an undelimited citation inside a shell comment", func(t *testing.T) {
		f := newCitedTarget(t, targetSectionBody)
		f.newScript("citer.sh", "#!/usr/bin/env bash\n# untested: fixture\n# the rule is target.md → One home\ntrue")
		f.reports(undelimited)
	})
}

// A cited path is resolved with a test that follows symlinks, so what it names is whatever the
// reviewed tree pointed it at. `evil.md -> /dev/zero`, or a committed FIFO, made the read of it never
// return — the shell version hangs on the same fixture, both killed at 12s.
func TestCitationTargetMustBeARegularFile(t *testing.T) {
	// /dev/null stands in for /dev/zero: the same class of target, and the one that cannot hang this
	// suite on the day someone takes the guard back out.
	newDeviceTarget := func(t *testing.T) *fixture {
		f := newRoot(t)
		f.symlink("/dev/null", f.root+"/kk-flavor/standards/evil.md")
		f.write(f.root+"/kk-flavor/standards/citer.md",
			"see [evil.md](evil.md) → **X** for the rule\n")
		return f
	}

	t.Run("fires on a cited path that resolves to a device", func(t *testing.T) {
		newDeviceTarget(t).reports(notRegular)
	})

	t.Run("and says it was not read rather than reading part of it", func(t *testing.T) {
		newDeviceTarget(t).reports("it was NOT read")
	})
}

// The cited path names no file at all. This one is the citation's own half of the dangling-link scan:
// the path never resolves, so nothing reads the section and no other case here reaches the finding.
func TestUnresolvableCitationPaths(t *testing.T) {
	t.Run("fires on a cited path that names no file under the root", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/standards/citer.md",
			"see `standards/nowhere.md` → **Density** for the rule\n")
		f.reports(unresolved)
	})

	// The other way the resolution comes back empty: a bare name several files answer to names none of
	// them, and guessing would report a section against a file the citation never meant.
	t.Run("and on a bare name more than one file answers to", func(t *testing.T) {
		f := newRoot(t)
		f.mkdirAll(f.root + "/kk-flavor/templates")
		f.newMountedSkill("kk-drive")
		// Neither twin sits beside the citer, or the citer's own directory would answer first and the
		// bare name would never reach the ambiguity test.
		f.write(f.root+"/kk-flavor/templates/twin.md", "# One\n\n## Density\n")
		f.write(f.root+"/skills/kk-drive/twin.md", "# Two\n\n## Density\n")
		f.write(f.root+"/kk-flavor/standards/citer.md", "see `twin.md` → **Density** for the rule\n")
		f.reports(unresolved)
	})

	// The control: the same citation resolves and reports nothing once one file answers to the name.
	t.Run("and stays quiet once the cited path resolves", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/standards/real.md", "# Real\n\n## Density\n")
		f.write(f.root+"/kk-flavor/standards/citer.md", "see `real.md` → **Density** for the rule\n")
		f.doesNotReport(unresolved)
	})
}

// The section every case cites unless it says otherwise, and the cited document it sits in. Both are
// written once because six builders and cases below reach for them: two copies of a heading, one of
// them edited, is a case that goes on passing while citing something the target no longer carries.
const (
	targetSection     = "One home"
	targetSectionBody = "## " + targetSection + "\n"
)

func newCitedTarget(t *testing.T, body string) *fixture {
	t.Helper()
	f := newRoot(t)
	// The title above the body is what makes the file a markdown document rather than a fragment.
	f.write(f.root+"/kk-flavor/standards/target.md", "# Target\n\n"+body)
	return f
}

// A standard citing a section of `target.md`, where the case chooses the heading the target carries
// and the form the citation is written in.
func newCitedHeading(t *testing.T, heading, citation string) *fixture {
	t.Helper()
	f := newCitedTarget(t, "## "+heading+"\n")
	f.write(f.root+"/kk-flavor/standards/citer.md",
		"see [target.md](target.md) → "+citation+" for the rule\n")
	return f
}

func newCitation(t *testing.T, citation string) *fixture {
	t.Helper()
	return newCitedHeading(t, targetSection, citation)
}

// A cited path holding a glob. The citation scan is the one whose token can carry a metacharacter at
// all, for the reason citations.go → globInCitation gives. Both ways of writing a citation reach the
// refusal, so both are covered here.
func TestAPatternInACitedPathIsRefusedRatherThanMatched(t *testing.T) {
	// The target carries no section, so every pattern below matches a file that has no `One home`
	// heading: a resolver that still globbed would resolve the path and say `dangling section ref`
	// against it. That is why each case asserts the refusal *and* that nothing resolved — asserting
	// only the refusal would pass over a resolver answering these as find(1) does.
	newPatternProbe := func(t *testing.T, citation string) *fixture {
		t.Helper()
		f := newCitedTarget(t, "")
		f.write(f.root+"/kk-flavor/standards/citer.md", citation+"\n")
		return f
	}
	refused := func(t *testing.T, f *fixture, ref string) {
		t.Helper()
		output := f.run()
		f.found(output, ecocheck.CitationPathIsPattern+f.root+"/kk-flavor/standards/citer.md:1 -> "+ref)
		f.absent(output, dangling, unresolved)
	}

	// One case per metacharacter, because the refusal names the byte it found and each is a separate
	// way in. `\` is doubled only for Go's own literal.
	for _, ref := range []string{"standards/targ*t.md", "standards/targ?t.md", "standards/[st]arget.md", "standards/targe\\t.md"} {
		t.Run("refuses a backticked citation holding "+ref, func(t *testing.T) {
			refused(t, newPatternProbe(t, "see `"+ref+"` → **One home** for the rule"), ref)
		})
	}

	// The link form takes its token from a different arm of the parser, whose filter is `[^()]*` — so
	// refusing in only one of the two arms would leave the other resolving patterns.
	t.Run("and a markdown-link citation holding one", func(t *testing.T) {
		refused(t, newPatternProbe(t, "see [x](targ*t.md) → **One home** for the rule"), "targ*t.md")
	})

	// The control: the same citation without the metacharacter resolves, so the refusal above cannot be
	// a scan that stopped reading citations altogether.
	t.Run("while the same citation naming the file outright still resolves", func(t *testing.T) {
		f := newPatternProbe(t, "see `standards/target.md` → **One home** for the rule")
		output := f.run()
		f.absent(output, ecocheck.CitationPathIsPattern, unresolved)
		f.found(output, dangling+f.root+"/kk-flavor/standards/citer.md:1 -> standards/target.md → One home")
	})
}

// A citation wraps. A hard line break between the cited file and its section, or inside the section
// name, is what a formatter leaves behind, and a line-oriented split cannot see across one: the
// citation then resolves against nothing and the whole scan goes quiet, which is the one outcome a
// dangling-reference scan must not have. Every case here carries its own control, because a wrapped
// citation the scan never saw and a wrapped citation that resolved are the same silence.
func TestWrappedSectionCitations(t *testing.T) {
	t.Run("reports a dangling section on one line (control for the wraps below)", func(t *testing.T) {
		newWrittenCitation(t, "see [target.md](target.md) → **Nowhere at all** for the rule").reports(dangling)
	})

	t.Run("reports one wrapped between the arrow and its section", func(t *testing.T) {
		newWrittenCitation(t, "see [target.md](target.md) →\n**Nowhere at all** for the rule").reports(dangling)
	})

	t.Run("reports one wrapped between the cited file and the arrow", func(t *testing.T) {
		newWrittenCitation(t, "see [target.md](target.md)\n→ **Nowhere at all** for the rule").reports(dangling)
	})

	t.Run("reports one wrapped inside the section name", func(t *testing.T) {
		newWrittenCitation(t, "see [target.md](target.md) → **Nowhere at\nall** for the rule").reports(dangling)
	})

	// The other direction: a wrapped citation that does resolve must not become a finding, or the fix
	// above trades a silence for noise on every wrapped line in the tree.
	t.Run("resolves a wrapped citation whose section is really there", func(t *testing.T) {
		newWrittenCitation(t, "see [target.md](target.md) →\n**One home** for the rule").doesNotReport(dangling)
	})

	t.Run("and one wrapped inside the section name", func(t *testing.T) {
		newWrittenCitation(t, "see [target.md](target.md) → **One\nhome** for the rule").doesNotReport(dangling)
	})

	// A paragraph break is not a wrap. Joining across one would let a file named in one paragraph
	// answer an arrow in the next, which is a citation nobody wrote.
	t.Run("does not join a file to an arrow across a blank line", func(t *testing.T) {
		newWrittenCitation(t, "see [target.md](target.md)\n\n→ **Nowhere at all** for the rule").doesNotReport(dangling)
	})

	// The position names the line the arrow sits on, which is where a reader looks for the citation.
	t.Run("names the line the arrow sits on", func(t *testing.T) {
		newWrittenCitation(t, "padding\nsee [target.md](target.md)\n→ **Nowhere at all** for the rule").
			reports("/kk-flavor/standards/citer.md:3 -> target.md")
	})
}

// A script cites the same way a document does, and the wrap carries the comment marker onto the
// continuation line. Read as text, that marker lands inside the section name and the delimited form
// reads as undelimited.
func TestWrappedCitationInsideAShellComment(t *testing.T) {
	newCommentCitation := func(t *testing.T, section string) *fixture {
		f := newCitedTarget(t, targetSectionBody)
		f.newScript("citer.sh", "#!/usr/bin/env bash\n# untested: fixture\n# the rule is target.md →\n# **"+section+"**\ntrue")
		return f
	}

	t.Run("sees a wrapped citation across the comment marker (control)", func(t *testing.T) {
		newCommentCitation(t, "Nowhere at all").reports(dangling)
	})

	t.Run("and does not read that marker as part of the section", func(t *testing.T) {
		newCommentCitation(t, "One home").doesNotReport(undelimited)
	})
}

func newWrittenCitation(t *testing.T, body string) *fixture {
	t.Helper()
	f := newCitedTarget(t, targetSectionBody)
	f.write(f.root+"/kk-flavor/standards/citer.md", body+"\n")
	return f
}

// A heading numbered `## 7. What a suite reports` is cited by its text. Nothing could resolve that
// shape: markdownHeadings registers a heading under what it says, and reportCitation trims *trailing*
// words off a citation, so a leading token is the one thing the matcher walks away from.
func TestNumberedHeadingCitations(t *testing.T) {
	t.Run("resolves a citation naming a numbered heading by its text alone", func(t *testing.T) {
		newNumberedHeading(t, "**What a suite reports**").doesNotReport(dangling)
	})

	t.Run("and the numbered form as the heading writes it", func(t *testing.T) {
		newNumberedHeading(t, "**7. What a suite reports**").doesNotReport(dangling)
	})

	// The proof the carve-out opens no word-by-word prefix hole: a fixed affix comes off, and nothing
	// else does. Both cases name a real part of that heading's text and must still resolve nowhere —
	// with and without the number, because the numberless key is the one this carve-out adds.
	t.Run("still refuses part of a numbered heading's text", func(t *testing.T) {
		newNumberedHeading(t, "**What a suite**").reports(dangling)
	})

	t.Run("and part of it carrying the number too", func(t *testing.T) {
		newNumberedHeading(t, "**7. What a suite**").reports(dangling)
	})

	// The carve-out composes with the em-dash one rather than competing with it: a heading wearing
	// both affixes is cited by the text between them.
	t.Run("resolves a numbered heading's text without its em-dash subtitle", func(t *testing.T) {
		newCitedHeading(t, "7. What a suite reports — and to whom", "**What a suite reports**").doesNotReport(dangling)
	})
}

func newNumberedHeading(t *testing.T, citation string) *fixture {
	t.Helper()
	return newCitedHeading(t, "7. What a suite reports", citation)
}

// A citation whose head names no markdown file resolved against nothing and was dropped without a
// word, so renaming the section it names broke it with this gate green. A citation the scanner cannot
// see is worse than one it reports wrong.
func TestUncheckableCitations(t *testing.T) {
	t.Run("fires on a backticked head that names no markdown file", func(t *testing.T) {
		newBacktickedHead(t, "`kk-qualify` → **The residue** decides").reports(uncheckable)
	})

	// The control that says the fixture reaches the scan at all: the same line with a head that does
	// name a file is read, and reported as the dangling section it is.
	t.Run("while the same citation with a real file is read as usual", func(t *testing.T) {
		newBacktickedHead(t, "`target.md` → **The residue** decides").reports(dangling)
	})

	// The two forms the tree writes prose in. Both were measured across this tree before the finding
	// went in: restricting it to the mandated `**` form is what makes them quiet.
	t.Run("stays quiet on a prose arrow with a backticked right side", func(t *testing.T) {
		newBacktickedHead(t, "`test:unit` → `*.unit.test.ts` is the pairing").doesNotReport(uncheckable)
	})

	t.Run("stays quiet on a prose arrow with a bare right side", func(t *testing.T) {
		newBacktickedHead(t, "`intent` → build, then ship").doesNotReport(uncheckable)
	})

	// A bare head is not a reference written and misread, it is prose, and this tree writes 39 arrows
	// that way. Only the backticked head is worth a finding.
	t.Run("stays quiet on a bare head with nothing in the block to stand in", func(t *testing.T) {
		newBacktickedHead(t, "the placeholder is <file>.md → **Section**").doesNotReport(uncheckable)
	})
}

// The head is the one part of that finding a reader has to find again in their own file, and it is a
// run the reviewed tree chose the length of. Cut at 60 bytes with nothing saying so, it reads as the
// whole head, and the reader greps for a string nobody wrote.
func TestAnUncheckableCitationSaysWhenItsHeadWasCut(t *testing.T) {
	// Long enough to run past the bound, and carrying no dot, so the head still names no markdown file
	// and the finding under test is the one that fires.
	longHead := strings.Repeat("kk-qualify-", 10)

	t.Run("fires on a head that runs past the bound (control for the case below)", func(t *testing.T) {
		newBacktickedHead(t, "`"+longHead+"` → **The residue** decides").reports(uncheckable)
	})

	// Matched on the marker together with the text the finding puts after the head, so the assertion
	// is about where the cut is reported and not about a "..." landing anywhere in the output.
	t.Run("and marks the head it cut rather than quoting a shorter wrong one", func(t *testing.T) {
		newBacktickedHead(t, "`"+longHead+"` → **The residue** decides").reports(shell.CutMarker + "` → ")
	})

	// The other direction: a head inside the bound is quoted whole, or the marker starts saying a
	// citation was cut when the reader is looking at all of it.
	t.Run("while a head inside the bound is quoted with no marker", func(t *testing.T) {
		newBacktickedHead(t, "`kk-qualify` → **The residue** decides").doesNotReport(shell.CutMarker + "` → ")
	})
}

func newBacktickedHead(t *testing.T, body string) *fixture {
	t.Helper()
	f := newWrittenCitation(t, body)
	f.newMountedSkill("kk-qualify")
	return f
}

// The second citation into a file already named in the same sentence — `You run under `x.md` as an
// orchestrator (→ **Section**)`. Six of them are written that way in this tree, and every one of them
// resolved against nothing until the file was carried forward.
func TestCitationCarriedFromItsBlock(t *testing.T) {
	t.Run("reads a section against the file named earlier in the block", func(t *testing.T) {
		newWrittenCitation(t, "run under [target.md](target.md) as an orchestrator (→ **Nowhere at all**)").reports(dangling)
	})

	t.Run("and stays quiet when that section is really there (control)", func(t *testing.T) {
		newWrittenCitation(t, "run under [target.md](target.md) as an orchestrator (→ **One home**)").doesNotReport(dangling)
	})

	t.Run("carries it to a second arrow on the same line", func(t *testing.T) {
		newWrittenCitation(t, "`target.md` → **One home** and → **Nowhere at all** both bind").reports(dangling)
	})

	// The bound is the block, for the reason the block exists: a file named in one paragraph must not
	// answer an arrow in the next, which would be a citation nobody wrote.
	t.Run("does not carry a file across a blank line", func(t *testing.T) {
		newWrittenCitation(t, "run under [target.md](target.md) as an orchestrator\n\n(→ **Nowhere at all**)").doesNotReport(dangling)
	})

	// Prose in a block that happens to name a file is still prose. Only the mandated `**` form is read
	// as a citation, which is what keeps this from reporting the tree against itself.
	t.Run("does not carry a file onto a prose arrow", func(t *testing.T) {
		newWrittenCitation(t, "see [target.md](target.md), where intent → build, then ship").doesNotReport(dangling)
	})

	// The shortest name a file can have. It reads like a corner nobody writes, and it is not: the
	// search runs backwards over the `.md` occurrences in a block, so a name its pattern cannot match
	// is not one answer missed but a probe that fails and moves on to the next — over a paragraph of
	// them, that is the scan reading the block again per occurrence.
	t.Run("carries a one-character filename like any other", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/standards/a.md", "# A\n\n## One home\n")
		f.write(f.root+"/kk-flavor/standards/citer.md", "run under `a.md` as an orchestrator (→ **Nowhere at all**)\n")
		f.reports(dangling)
	})
}

// Four ways a section citation dangles, all reading as correct to a human and all found only here. A
// finding that says "dangling" and stops leaves the reader hunting through the cited file for a
// difference they have already failed to see once, so each names its own variant.
func TestDanglingSectionRefNamesItsVariant(t *testing.T) {
	// The one a reader can act on without opening anything: both strings are in hand where the
	// citation failed, so the finding carries the citation that resolves.
	t.Run("quotes the resolving form when only the number differs", func(t *testing.T) {
		f := newDanglingVariant(t, "## 4. Surgical changes\n", "**3. Surgical changes**")
		f.reports("cite it as target.md → **4. Surgical changes**")
	})

	t.Run("names a bolded run that is not a heading", func(t *testing.T) {
		f := newDanglingVariant(t, "- **The residue** is what is left\n", "**The residue**")
		f.reports("**The residue** is bolded text there, not a heading")
	})

	t.Run("names the file the section is in when the citation points at another", func(t *testing.T) {
		f := newDanglingVariant(t, "nothing here\n", "**The residue**")
		f.write(f.root+"/kk-flavor/standards/moved.md", "# Moved\n\n## The residue\n")
		f.reports("that heading is in ")
	})

	t.Run("and says a name nothing answers to reads like a paraphrase", func(t *testing.T) {
		f := newDanglingVariant(t, "nothing here\n", "**The residue**")
		f.reports("reads like a paraphrase of one")
	})
}

// A cited file whose body the case chooses, cited by a section the case chooses.
func newDanglingVariant(t *testing.T, body, section string) *fixture {
	t.Helper()
	f := newCitedTarget(t, body)
	f.write(f.root+"/kk-flavor/standards/citer.md",
		"see [target.md](target.md) → "+section+" for the rule\n")
	return f
}

// The note that rides a citation finding against a test harness. The rule it states is its own text,
// at citations.go → harnessCitationNote; the cost of taking that rule is the third case below.
//
// This file writes its own citations out, because no scan reads a `.go` file. A shell suite covering
// the same ground could not, which is the asymmetry that note answers.
func TestACitationInATestHarnessSaysWhatToDoAboutIt(t *testing.T) {
	t.Run("names the rule on a finding against a suite", func(t *testing.T) {
		newHarnessCitation(t, "fixture-test.sh").reports(ecocheck.HarnessCitationNote)
	})

	t.Run("and on one against a mutation list", func(t *testing.T) {
		newHarnessCitation(t, "fixture-mutate.sh").reports(ecocheck.HarnessCitationNote)
	})

	// The cost this choice takes, stated as a case: there is no escape hatch, so a harness may carry no
	// citation literal at all. The finding still fires, and that is what makes the rule bind.
	t.Run("and reports it all the same, since nothing here exempts a harness", func(t *testing.T) {
		newHarnessCitation(t, "fixture-test.sh").reports(dangling)
	})

	// The note is scoped to a harness. Every other script pays nothing for it.
	t.Run("says nothing of the sort on an ordinary script", func(t *testing.T) {
		newHarnessCitation(t, "fixture.sh").doesNotReport(ecocheck.HarnessCitationNote)
	})

	// Without this the case above passes on a fixture whose citation was never read.
	t.Run("while still reporting that script's citation (control for the case above)", func(t *testing.T) {
		newHarnessCitation(t, "fixture.sh").reports(dangling)
	})
}

// A script of the given name carrying one dangling citation, built rather than written out.
func newHarnessCitation(t *testing.T, name string) *fixture {
	t.Helper()
	f := newCitedTarget(t, targetSectionBody)
	f.newScript(name, "#!/usr/bin/env bash\n# untested: fixture\n# the rule is target.md → **Nowhere at all**\ntrue")
	return f
}

// A `#` line inside a fenced block is a code sample, and a `**run**` inside one is sample text. One
// rule reads both out, so both cases below are about the same guard: what a citation resolves against
// is a document's sections, never what it quotes.
func TestFencedContentIsNotASection(t *testing.T) {
	t.Run("a citation naming a heading that only exists inside a fence dangles", func(t *testing.T) {
		newDanglingVariant(t, "```\n## One home\n```\n", "**One home**").reports(dangling)
	})

	// Without this the case above passes on a fixture whose target was never read.
	t.Run("while the same heading outside one resolves (control)", func(t *testing.T) {
		newCitation(t, "**One home**").doesNotReport(dangling)
	})

	// And the variant a dangling citation is given must not be read out of a fence either, or the
	// finding sends its reader to a heading that is sample text.
	t.Run("and a bolded run inside a fence is not offered as the near miss", func(t *testing.T) {
		newDanglingVariant(t, "```\n- **The residue** is sample text\n```\n", "**The residue**").
			doesNotReport("bolded text there")
	})
}

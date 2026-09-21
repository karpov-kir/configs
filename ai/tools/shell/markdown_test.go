package shell

import "testing"

// The block has to close. Walking to end-of-file and accepting on the way lets a body line answer for
// a block that never closed — a file the loader cannot read frontmatter from at all, reported as a
// clean declaration. The old reader was bounded to lines 2-10, so past line 10 it reported nothing
// and the mismatch surfaced; the bound went, and end-of-file has to take its place as the terminator.
func TestFrontmatterMustClose(t *testing.T) {
	name := func(lines []string) string { return FrontmatterName(lines) }

	closed := []string{"---", "name: alpha", "---", "body", "name: beta"}
	if got := name(closed); got != "alpha" {
		t.Errorf("closed block: got %q, want alpha", got)
	}
	// Never closed: everything after line 1 is body, whatever it looks like.
	unterminated := []string{"---", "intro", "body text", "name: beta"}
	if got := name(unterminated); got != "" {
		t.Errorf("unterminated block: got %q, want empty — a body line answered as a declaration", got)
	}
	// Far enough down that the old line-2-to-10 window would also have missed it.
	deep := []string{"---", "a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "name: beta"}
	if got := name(deep); got != "" {
		t.Errorf("unterminated, past line 10: got %q, want empty", got)
	}
	// A file that does not open with a delimiter has no frontmatter at all.
	if got := name([]string{"# Title", "name: beta"}); got != "" {
		t.Errorf("no opening delimiter: got %q, want empty", got)
	}
	// An empty block closes immediately and declares nothing.
	if got := name([]string{"---", "---", "name: beta"}); got != "" {
		t.Errorf("empty block: got %q, want empty", got)
	}
	if got := name(nil); got != "" {
		t.Errorf("no lines: got %q, want empty", got)
	}
}

// The audience marker, read through the same block walk as every other declaration: a skill that
// declares itself maintainer-only is one ai/bootstrap.sh leaves unmounted without --maintainer, and
// one eco-check's mount scan then expects to find no mount for.
func TestTheMaintainerMarkerIsReadOnlyOutOfFrontmatter(t *testing.T) {
	marked := []string{"---", "name: kk-reduce", "audience: maintainer", "---", "body"}
	if !IsMaintainerAudience(marked) {
		t.Error("a declared marker went unread, so bootstrap would mount a skill the scan expects unmounted")
	}
	// Spacing and case, the way the opt-out marker beside it is read.
	if !IsMaintainerAudience([]string{"---", "Audience:   Maintainer  ", "---"}) {
		t.Error("the marker is case- and space-insensitive, and one spelling of it was refused")
	}
	if IsMaintainerAudience([]string{"---", "audience: everyone", "---"}) {
		t.Error("another audience answered as the maintainer one")
	}
	if IsMaintainerAudience([]string{"---", "name: kk-build", "---", "audience: maintainer"}) {
		t.Error("a body line answered as a declaration — that skill would silently stop being installed")
	}
	if IsMaintainerAudience([]string{"---", "audience: maintainer"}) {
		t.Error("an unterminated block is not frontmatter, and the loader cannot read one either")
	}
	if IsMaintainerAudience(nil) {
		t.Error("no lines declared a marker")
	}
}

// The misspelling is the case, not the marker: a reader asking only "is this the marker" answers no
// to `audience: maintainr` and to a skill that declared nothing, and the two mean opposite things.
func TestAnAudienceNothingReadsIsRefusedRatherThanIgnored(t *testing.T) {
	frontmatter := func(line string) []string {
		if line == "" {
			return []string{"---", "name: x", "---"}
		}
		return []string{"---", "name: x", line, "---"}
	}
	for _, c := range []struct {
		line    string
		want    string
		refused bool
	}{
		// The two that must stay silent, and they are the controls: without them an always-refusing
		// reader passes every case below.
		{"", "", false},
		{"audience: maintainer", "", false},
		{"audience:   Maintainer  ", "", false},
		// The class the marker check cannot reach.
		{"audience: maintainr", "maintainr", true},
		{"audience: everyone", "everyone", true},
		// A key with no value is a declaration too — a bound they think they set.
		{"audience:", "", true},
	} {
		value, found := UnknownAudience(frontmatter(c.line))
		if found != c.refused {
			t.Errorf("UnknownAudience(%q) found = %v, want %v", c.line, found, c.refused)
			continue
		}
		if found && value != c.want {
			t.Errorf("UnknownAudience(%q) echoed %q, want %q — it is read by whoever typed the line",
				c.line, value, c.want)
		}
	}
	// Outside the block it is prose, which is the rule the whole frontmatter scan rests on.
	if _, found := UnknownAudience([]string{"---", "name: x", "---", "audience: maintainr"}); found {
		t.Error("an audience line in the body was refused, so prose can fail an install")
	}
}

// The three forms and nothing else. The declaration is what the orchestrator tier ceiling reads, so
// a fourth word it could not place would exempt a skill from that check by being unreadable — which
// is why the parser reports a line it cannot read rather than passing it over.
func TestRunsDeclarationReadsTheThreeFormsAndRefusesAFourth(t *testing.T) {
	for _, one := range []struct {
		line     string
		mode     string
		declared bool
	}{
		{line: "**Runs:** dispatched", mode: "dispatched", declared: true},
		{line: "**Runs:** orchestrator", mode: "orchestrator", declared: true},
		{line: "**Runs:** holds — converses", mode: "holds — converses", declared: true},
		{line: "**Runs:** holds — session-context", mode: "holds — session-context", declared: true},
		{line: "**Runs:** holds — landing", mode: "holds — landing", declared: true},
		// `human` was retired when it turned out to conflate a skill that grills the human round by
		// round with one that asks once. A reader answering only "which of the three is it" would
		// have read every skill still carrying it as declaring nothing at all.
		{line: "**Runs:** human", mode: "", declared: true},
		{line: "**Runs:** holds — whenever", mode: "", declared: true},
		// A hyphen is not the em dash the tree is written with, and the difference is invisible.
		{line: "**Runs:** holds - converses", mode: "", declared: true},
		{line: "**Runs:**", mode: "", declared: true},
		{line: "The skill **Runs:** orchestrator inline", mode: "", declared: false},
		{line: "Runs: orchestrator", mode: "", declared: false},
		{line: "nothing about how it runs", mode: "", declared: false},
	} {
		mode, declared := RunsDeclaration([]string{"---", "name: x", "---", "", one.line, "", "body"})
		if mode != one.mode || declared != one.declared {
			t.Errorf("%q read as (%q, %v), wanted (%q, %v)", one.line, mode, declared, one.mode, one.declared)
		}
	}
}

// Every line is scanned, frontmatter included. The declaration belongs in the body — it is a contract
// between the skill and the model policy, where frontmatter is what the harness loads into every
// session that never invokes the skill — but a file that puts it in the block anyway has still
// declared, and reading it as silence would exempt that skill from the tier ceiling.
func TestARunsDeclarationInsideTheFrontmatterIsStillRead(t *testing.T) {
	if mode, _ := RunsDeclaration([]string{"---", "**Runs:** orchestrator", "---", "", "body"}); mode != "orchestrator" {
		t.Errorf("a declaration inside the frontmatter block went unread: %q", mode)
	}
}

// The reason a `holds` declaration names, which is what says whether a session's tier is the work it
// keeps or an offload nobody has done yet. Empty for every mode that names no reason, so a caller
// printing it never prints half of `orchestrator`.
func TestRunsHoldsReasonIsTheReasonAndNothingElse(t *testing.T) {
	for mode, want := range map[string]string{
		"holds — converses":       "converses",
		"holds — session-context": "session-context",
		"holds — landing":         "landing",
		"orchestrator":            "",
		"dispatched":              "",
		"":                        "",
	} {
		if got := RunsHoldsReason(mode); got != want {
			t.Errorf("%q gave reason %q, wanted %q", mode, got, want)
		}
	}
}

// The extension declaration's grammar. It carries the one edge nothing else can check — extension,
// sequencing and orientation all name a second skill the same way — so a line nobody can read must be
// reported as a broken declaration and never as an absent one: read as silence, the extension is
// priced free and the bill it belongs on loses it silently.
//
// Its own case rather than the tree census's, because every declaration in the shipped tree parses.
// The census cannot reach this arm at all, which is how it came to have no case.
func TestExtendsDeclarationReadsTheFormAndReportsALineItCannot(t *testing.T) {
	for _, one := range []struct {
		lines    []string
		extends  []string
		declared bool
	}{
		{lines: []string{"**Extends:** kk-build — Phase 3's loop"}, extends: []string{"kk-build"}, declared: true},
		{lines: []string{"**Extends:** idsd-finalize — `done`, on a clean gate"}, extends: []string{"idsd-finalize"}, declared: true},
		// Several contracts read as one session's delta is what `idsd-ship` does; all are returned
		// rather than the first winning silently.
		{lines: []string{"**Extends:** kk-build — the build", "**Extends:** kk-qualify — the pass"}, extends: []string{"kk-build", "kk-qualify"}, declared: true},
		// A when carrying its own em dash: the only shape where the separator could be read in two
		// places, and the one that would return the wrong target if it were. No shipped declaration
		// has one, so the tree census cannot reach this either.
		{lines: []string{"**Extends:** kk-build — Phase 3 — the loop, not Phase 2"}, extends: []string{"kk-build"}, declared: true},
		// One character is a whole when: what it has to name is a phase of this skill, which no regex
		// judges, so the grammar holds only that something is there.
		{lines: []string{"**Extends:** kk-build — 3"}, extends: []string{"kk-build"}, declared: true},
		// The `— <when>` is the half a reader acts on, so a declaration without one is a broken
		// declaration and not an absent edge. Nothing else in the file says where the read happens.
		{lines: []string{"**Extends:** kk-build"}, declared: true},
		{lines: []string{"**Extends:** kk-build —"}, declared: true},
		{lines: []string{"**Extends:** kk-build — "}, declared: true},
		// Whitespace is not a when. A tab-only clause is the one way this line can be wrong and cost
		// money: read as present, it prices a declaration nothing in the file locates.
		{lines: []string{"**Extends:** kk-build — \t\t"}, declared: true},
		// The separator is one space, an em dash, one space. A when flush against the dash is a
		// broken declaration, not a when beginning with an em dash.
		{lines: []string{"**Extends:** kk-build —Phase 3's loop"}, declared: true},
		// A carriage return a CRLF checkout leaves behind is trailing space, not part of the when.
		{lines: []string{"**Extends:** kk-build — Phase 3's loop\r"}, extends: []string{"kk-build"}, declared: true},
		// An en dash, a hyphen or a colon where the em dash belongs: near-misses of the separator are
		// refused rather than read past, the way the `**Runs:**` grammar refuses a fourth word.
		{lines: []string{"**Extends:** kk-build - Phase 3's loop"}, declared: true},
		{lines: []string{"**Extends:** kk-build – Phase 3's loop"}, declared: true},
		{lines: []string{"**Extends:** kk-build, kk-qualify — both"}, declared: true},
		{lines: []string{"**Extends:** `kk-build` — Phase 3's loop"}, declared: true},
		{lines: []string{"**Extends:** ~/.kk-flavor/skills/kk-build/SKILL.md — Phase 3's loop"}, declared: true},
		{lines: []string{"**Extends:**"}, declared: true},
		{lines: []string{"It **Extends:** kk-build — inline"}},
		{lines: []string{"Extends: kk-build — Phase 3's loop"}},
		{lines: []string{"nothing about what it extends"}},
	} {
		extends, declared := ExtendsDeclarations(one.lines)
		if declared != one.declared || len(extends) != len(one.extends) {
			t.Errorf("%q read as (%v, %v), wanted (%v, %v)", one.lines, extends, declared, one.extends, one.declared)
			continue
		}
		for at := range extends {
			if extends[at] != one.extends[at] {
				t.Errorf("%q read as %v, wanted %v", one.lines, extends, one.extends)
				break
			}
		}
	}
}

// The layer grammar: three names, and a line nobody can read reported as one rather than as an absent
// declaration. The cycle check next door reads this answer, so a fourth word passed over as silence
// would exempt that standard from the check by being unreadable — and it would arrive under the
// finding for a file that declares nothing, sending its author to write a line the file already has.
func TestLayerDeclarationReadsTheThreeAndRefusesAFourth(t *testing.T) {
	for _, one := range []struct {
		line     string
		layer    string
		declared bool
	}{
		{line: "**Layer:** base", layer: "base", declared: true},
		{line: "**Layer:** craft", layer: "craft", declared: true},
		{line: "**Layer:** process", layer: "process", declared: true},
		{line: "**Layer:** process\r", layer: "process", declared: true},
		{line: "**Layer:** foundation", layer: "", declared: true},
		// This row carries the closing anchor. `basement` opens on a real layer name, so a pattern that
		// stopped anchoring its end reads it as `base` and files the standard in an invented layer. The
		// `foundation` row shares no prefix with any layer, which leaves the anchor to this one.
		{line: "**Layer:** basement", layer: "", declared: true},
		{line: "**Layer:** Base", layer: "", declared: true},
		{line: "**Layer:**", layer: "", declared: true},
		{line: "The file **Layer:** base declares", layer: "", declared: false},
		{line: "Layer: base", layer: "", declared: false},
		{line: "nothing about a layer", layer: "", declared: false},
	} {
		layer, declared := LayerDeclaration([]string{one.line, "", "# Heading", "", "body"})
		if layer != one.layer || declared != one.declared {
			t.Errorf("%q read as (%q, %v), wanted (%q, %v)", one.line, layer, declared, one.layer, one.declared)
		}
	}
}

func TestUnknownLayerIsTheWordAsItWasTyped(t *testing.T) {
	written, found := UnknownLayer([]string{"**Layer:**   Crafts  ", "", "body"})
	if !found || written != "Crafts" {
		t.Errorf("unknown layer read as (%q, %v), wanted (\"Crafts\", true)", written, found)
	}
	for _, lines := range [][]string{{"**Layer:** craft"}, {"# Heading"}, nil} {
		if written, found := UnknownLayer(lines); found || written != "" {
			t.Errorf("%v read as an unknown layer (%q, %v)", lines, written, found)
		}
	}
}

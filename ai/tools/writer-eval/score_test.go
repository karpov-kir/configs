package writereval

import (
	"strings"
	"testing"
)

func TestNoneIsReadAsDecliningTheSite(t *testing.T) {
	for _, raw := range []string{"none", "`none`", "None.", "  none  "} {
		if r := ParseReturn(raw); !r.None {
			t.Errorf("%q was not read as declining the site", raw)
		}
	}
	if ParseReturn("/** Returns the rows. */").None {
		t.Errorf("a written block was read as declining the site")
	}
}

func TestTheAuditLinesComeOutOfTheBlock(t *testing.T) {
	r := ParseReturn("/** A closing period posts in the base currency. */\n" +
		"term: closing period — domain\n" +
		"term: base currency — domain\n" +
		"verb: posts — literal\n")
	if got := r.Text(); got != "A closing period posts in the base currency." {
		t.Errorf("block text %q, and the audit lines should have come out of it", got)
	}
	if len(r.Terms) != 2 || len(r.Verbs) != 1 {
		t.Fatalf("%d term(s) and %d verb(s), want 2 and 1", len(r.Terms), len(r.Verbs))
	}
	if r.Terms[0].Word != "closing period" || r.Terms[0].Class != "domain" {
		t.Errorf("first term %+v", r.Terms[0])
	}
}

func checkedBy(failures []Failure, check string) bool {
	for _, f := range failures {
		if f.Check == check {
			return true
		}
	}
	return false
}

// Each of Kirill's four blocks in the ledger domain, with the plain rewrite of each beside it. The
// rewrite is the control: a check that fires on both reaches clear prose, which is what the census
// caught two proposed rules doing.
func TestEachFlaggedShapeFailsAndItsPlainRewritePasses(t *testing.T) {
	cases := []struct {
		name    string
		flagged string
		check   string
		plain   string
	}{
		{"a consequence about code that does not exist",
			"/** An account can refuse a posting, so a full entry type would hide the ledger's answer. */",
			"counterfactual-consequence",
			"/** An account can refuse a posting. postingType omits the currency from the entry type. */"},
		{"a boolean written as a person answering",
			"/** A closing period posts in one currency, so a currency-blind yes still settles the period. */",
			"anthropomorphism",
			"/** A closing period posts in the base currency. Asking without a currency asks about the base one. */"},
		{"an elided verb",
			"/** A live row set drops a row as soon as the ledger does. */",
			"elided-verb",
			"/** A live row set drops a row when the ledger removes it. */"},
		{"a negated case clause",
			"/** Returns the posting row, or undefined unless exactly one row is measurable. */",
			"negated-case",
			"/** Returns the sole posting row, or undefined when no row is measurable. */"},
	}
	for _, c := range cases {
		flagged := Score(ParseReturn(c.flagged))
		if !checkedBy(flagged, c.check) {
			t.Errorf("%s: %s did not fire on %q (failures %+v)", c.name, c.check, c.flagged, flagged)
		}
		if plain := Score(ParseReturn(c.plain)); len(plain) != 0 {
			t.Errorf("%s: the plain rewrite failed %+v, and it is the control", c.name, plain)
		}
	}
}

// A verb the writer itself audited as a figure is a rewrite, whatever the regexes say. This is the
// presence half of the scoring: the writer says what its own words are and the score reads that.
func TestAVerbAuditedAsAFigureFailsTheBlock(t *testing.T) {
	r := ParseReturn("/** The period covers both halves. */\nverb: covers — figure\n")
	failures := Score(r)
	if !checkedBy(failures, "verb-audited-as-a-figure") {
		t.Errorf("a self-audited figure did not fail the block: %+v", failures)
	}
	if !checkedBy(failures, "metaphor-verb") {
		t.Errorf("covers is on the taken list and did not fire: %+v", failures)
	}
}

// The metaphor list holds only what the census measured. A verb the writing standard names but the
// reviewed set never carries stays out, or the writer is being corrected against a guess.
func TestTheMetaphorListHoldsOnlyWhatWasMeasured(t *testing.T) {
	for _, absent := range []string{"hide", "climb", "slip past", "rubber-stamp", "hedge", "understate",
		"answer", "reach"} {
		if takenVerbs[absent] {
			t.Errorf("%q is on the taken list and the census counted no case for it", absent)
		}
	}
	for _, present := range []string{"cover", "settle", "sit in", "load-bearing"} {
		if !takenVerbs[present] {
			t.Errorf("%q was measured as a figure and is missing from the taken list", present)
		}
	}
}

func TestAVerdictNeedsBothTheClassAndTheChecks(t *testing.T) {
	clean := ParseReturn("/** A closing period posts in the base currency. */")
	if v := Judge("clean", ExpectWritten, clean); !v.Passed() {
		t.Errorf("a clean written block did not pass: %+v", v)
	}
	if v := Judge("wanted none", ExpectNone, clean); v.Passed() {
		t.Errorf("a block written where none was wanted passed")
	}
	dirty := ParseReturn("/** The period covers both halves. */")
	if v := Judge("dirty", ExpectWritten, dirty); v.Passed() {
		t.Errorf("a block failing a check passed")
	}
	if v := Judge("none", ExpectNone, ParseReturn("none")); !v.Passed() {
		t.Errorf("a declined site did not pass: %+v", v)
	}
}

// A declined site is scored on its class alone. The text checks over an empty block would report a
// failure about text the writer never wrote.
func TestADeclinedSiteFailsNoTextCheck(t *testing.T) {
	if got := Score(ParseReturn("none")); len(got) != 0 {
		t.Errorf("a declined site failed %+v", got)
	}
}

func TestTheSentenceCeilingCountsTheBlocksSentences(t *testing.T) {
	long := "/** One. Two. Three. Four. */"
	if !checkedBy(Score(ParseReturn(long)), "over-the-sentence-ceiling") {
		t.Errorf("four sentences did not reach the ceiling")
	}
	if checkedBy(Score(ParseReturn("/** A summary. One note. Two notes. */")), "over-the-sentence-ceiling") {
		t.Errorf("a summary and two notes reached the ceiling, and that is the shape the rule allows")
	}
}

func TestTheScorerReadsAMultiLineBlock(t *testing.T) {
	r := ParseReturn("/**\n * Returns the sole posting row.\n * A closing period posts in the base currency.\n */")
	if got := r.Text(); !strings.HasPrefix(got, "Returns the sole posting row.") {
		t.Errorf("multi-line block read as %q", got)
	}
	if got := Score(r); len(got) != 0 {
		t.Errorf("a clean multi-line block failed %+v", got)
	}
}

// A none on a site question 1 called needed is a skipped rewrite unless two attempts are there to
// read. The writer names that site itself, so this reads presence.
func TestANoneAfterQuestionOneSaidNeededShowsItsAttempts(t *testing.T) {
	skipped := ParseReturn("question 1: needed\nnone\n")
	if got := Judge("skipped", ExpectNone, skipped); got.Passed() {
		t.Errorf("a none with no attempts passed: %+v", got)
	}
	tried := ParseReturn("question 1: needed\n" +
		"attempt 1: /** The ledger settles it. */ - metaphor verb\n" +
		"attempt 2: /** The ledger covers it. */ - metaphor verb\n" +
		"none\n")
	if tried.Attempts != 2 {
		t.Fatalf("%d attempt(s) read, want 2", tried.Attempts)
	}
	if got := Judge("tried", ExpectNone, tried); !got.Passed() {
		t.Errorf("a none after two attempts failed: %+v", got)
	}
	declined := ParseReturn("question 1: none\nnone\n")
	if got := Judge("declined", ExpectNone, declined); !got.Passed() {
		t.Errorf("a site question 1 declined needs no attempts: %+v", got)
	}
}

// The question and attempt lines come out of the block the way the audit lines do.
func TestTheQuestionAndAttemptLinesAreNotPartOfTheBlock(t *testing.T) {
	r := ParseReturn("question 1: needed\n/** A closing period posts in the base currency. */\n")
	if !r.Answered || !r.Needed {
		t.Errorf("question 1 was not read: %+v", r)
	}
	if got := r.Text(); got != "A closing period posts in the base currency." {
		t.Errorf("block text %q", got)
	}
}

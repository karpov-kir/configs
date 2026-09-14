package modelpolicy

import (
	"strings"
	"testing"
)

// Spliced out of `sample` below by the one case that needs a document without it, so `sample` is
// assembled from this rather than repeating it — the splice then cannot miss.
const tierOrder = `"tiers":{"codex":["helper","middling","frontier"],"claude":["haiku","sonnet","opus"]},`

const sample = `{"version":4,"limits":{"intents-in-flight":10},` +
	tierOrder +
	`"sessions":{"kk-build":{"codex":{"model":"frontier","effort":"high"},"claude":{"model":"opus"}}},` +
	`"workers":{` +
	`"bloat-judge":{"codex":{"model":"helper","effort":"low"},"claude":{"model":"haiku"},"rolls":3},` +
	`"build/explore":{"codex":{"model":"middling","effort":"low"},"claude":{"model":"sonnet"}}}}`

// `sample` with the order removed and nothing else touched, so the missing order is the only thing
// left to refuse: validateTiers asks whether an order exists before it asks whether any model is
// ranked. Derived rather than written out a second time, so a field added to the schema reaches both
// documents at once.
//
// Two cases read it. TestPolicyRejectsMalformedDocuments asks only that it is refused;
// TestADocumentWithNoTierOrderIsRefusedForThat asks which guard spoke.
var noTierOrder = strings.Replace(sample, tierOrder, ``, 1)

func policyForTest(t *testing.T) *Policy {
	t.Helper()
	p, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEachClientGetsItsOwnSettingsAndRolls(t *testing.T) {
	p := policyForTest(t)
	got, err := p.Resolve(Request{Client: "claude", Task: "bloat-judge"})
	if err != nil || got.Requested.Model != "haiku" || got.Requested.Effort != "" || got.Rolls != 3 {
		t.Fatalf("claude judge = %+v, %v", got, err)
	}
	if len(got.PolicyDigest) != 64 {
		t.Fatalf("digest = %q", got.PolicyDigest)
	}
	got, err = p.Resolve(Request{Client: "codex", Task: "bloat-judge"})
	if err != nil || got.Requested.Model != "helper" || got.Requested.Effort != "low" {
		t.Fatalf("codex judge = %+v, %v", got, err)
	}
}

// TopTier is where the ceiling reads "the dearest model", and only the command-level gate pins that
// today — at a level where a cheapest-first answer still needs a whole tree to expose it. Here it is
// one call. The false arm is the unknown client, the same shape TierOf's own case covers: Parse is the
// only constructor and it refuses an empty order for every client the policy dispatches to, so an
// unranked name is the one way left to ask about a client with no order behind it.
func TestTopTierNamesTheDearestModelAndRefusesAnUnknownClient(t *testing.T) {
	policy := policyForTest(t)
	for client, want := range map[string]string{"codex": "frontier", "claude": "opus"} {
		if got, ranked := policy.TopTier(client); !ranked || got != want {
			t.Errorf("%s top tier = %q, %v; want %q", client, got, ranked, want)
		}
	}
	if got, ranked := policy.TopTier("nobody"); ranked || got != "" {
		t.Errorf("an unranked client answered %q, %v; want no top tier at all", got, ranked)
	}
}

// An effort with no model used to be a lever that kept a site on its caller's model. It is refused now,
// and the whole of why is that the same file forbids what it does: a cheap coordinator is safe only
// once every site under it names its own model. Nothing shipped ever used it, the tier order cannot
// rank it, and a row outside that order is one no ceiling can judge. The case below is the refusal
// itself; TestARowNamingAnEffortAndNoModelIsRefused covers the shapes it reaches. Both read the
// sentence rather than the error, because validName refuses an empty model too and would otherwise
// answer for this guard while it is disabled.
func TestEffortWithoutAModelIsRefusedRatherThanKeptAsALever(t *testing.T) {
	raw := strings.Replace(sample, `"kk-build":{"codex":{"model":"frontier","effort":"high"}`, `"kk-build":{"codex":{"effort":"high"}`, 1)
	if raw == sample {
		t.Fatal("the fixture edit matched nothing, so this case tests the unmodified sample")
	}
	_, err := Parse([]byte(raw))
	if err == nil {
		t.Fatal("a row carrying an effort and no model parsed, so the dispatch would take its caller's model")
	}
	// Which guard spoke, not merely that one did. validName refuses an empty segment too, so with this
	// refusal disabled the document is still refused — by a sentence about whitespace and control
	// characters, over a row that holds neither. Asserting the error alone would be green either way.
	if !strings.Contains(err.Error(), "names no model") {
		t.Fatalf("refused for the wrong reason, so this case cannot tell the guard from its neighbour: %v", err)
	}
}

func TestUnassignedTaskFailsRatherThanInheriting(t *testing.T) {
	p := policyForTest(t)
	for _, request := range []Request{
		{Client: "claude", Task: "kk-code-review"},
		{Client: "claude", Task: ""},
		{Client: "unknown", Task: "bloat-judge"},
	} {
		if got, err := p.Resolve(request); err == nil {
			t.Fatalf("unassigned dispatch accepted: %+v -> %+v", request, got)
		}
	}
}

func TestPolicyRejectsMalformedDocuments(t *testing.T) {
	for name, raw := range map[string]string{
		// These three keep the version valid on purpose: Go's decoder matches keys case-insensitively
		// and takes the last of a duplicate pair, so a wrong version would fail them on the version
		// alone and prove nothing about strictness.
		"wrong key case":   strings.Replace(sample, `"version":4`, `"Version":4`, 1),
		"unknown field":    strings.Replace(sample, `"version":4`, `"version":4,"typo":true`, 1),
		"duplicate field":  strings.Replace(sample, `"version":4`, `"version":4,"version":4`, 1),
		"nested duplicate": strings.Replace(sample, `"model":"helper"`, `"model":"helper","model":"other"`, 1),
		"unknown client":   strings.Replace(sample, `"codex":{"model":"helper"`, `"other":{"model":"helper"`, 1),
		"version 1":        strings.Replace(sample, `"version":4`, `"version":1`, 1),
		"trailing value":   sample + ` {}`,
		"null":             `null`,
		"no tasks":         `{"version":4,"limits":{"intents-in-flight":10},"sessions":{},"workers":{}}`,
		"no cap":           strings.Replace(sample, `"intents-in-flight":10`, `"intents-in-flight":0`, 1),
		"one client only":  strings.Replace(sample, `"claude":{"model":"haiku"},`, ``, 1),
		"empty settings":   strings.Replace(sample, `"claude":{"model":"haiku"}`, `"claude":{}`, 1),
		"unknown effort":   strings.Replace(sample, `"effort":"low"`, `"effort":"turbo"`, 1),
		// The other row of the effort table: three effort names are codex's alone, so one on a claude
		// entry reads as a tier and sets nothing. Without this case the two halves of the set can be
		// merged into one and every case here stays green.
		"codex-only effort on claude": strings.Replace(sample, `"claude":{"model":"opus"}`, `"claude":{"model":"opus","effort":"ultra"}`, 1),
		"claude effort alone":         strings.Replace(sample, `"claude":{"model":"opus"}`, `"claude":{"effort":"high"}`, 1),
		"option-shaped model":         strings.Replace(sample, `"model":"helper"`, `"model":"--dangerously-skip-permissions"`, 1),
		"even rolls":                  strings.Replace(sample, `"rolls":3`, `"rolls":4`, 1),
		"rolls over the cap":          strings.Replace(sample, `"rolls":3`, `"rolls":31`, 1),
		"cap over the ceiling":        strings.Replace(sample, `"intents-in-flight":10`, `"intents-in-flight":999999`, 1),
		"task name with space":        strings.Replace(sample, `"bloat-judge":`, `"bloat judge":`, 1),
		// The tier order is what tells a later check which of two rows spends more, so every way it
		// can fail to answer that is refused at parse rather than read as "unranked" downstream. A
		// comparison that quietly answers "not higher" passes what it should have stopped.
		"no tier order at all":   noTierOrder,
		"a client with no tiers": strings.Replace(sample, `"claude":["haiku","sonnet","opus"]`, `"claude":[]`, 1),
		"a model no tier ranks":  strings.Replace(sample, `"claude":{"model":"sonnet"}`, `"claude":{"model":"unranked"}`, 1),
		"a client the policy does not dispatch to": strings.Replace(sample, `"tiers":{"codex"`,
			`"tiers":{"cluade":["haiku"],"codex"`, 1),
		// A fourth entry on each side, so every row's model is still ranked and the two lists are
		// still the same length: the entry's shape is the only thing left to refuse it.
		"a tier shaped like a flag": strings.NewReplacer(
			`"codex":["helper","middling","frontier"]`, `"codex":["helper","middling","frontier","spare"]`,
			`"claude":["haiku","sonnet","opus"]`, `"claude":["haiku","sonnet","opus","--dangerously-skip-permissions"]`,
		).Replace(sample),
		// Same isolation for the name guard: a fourth entry on each side, so the unranked-model,
		// duplicate and length guards all pass and only this one can fire. With a three-entry list
		// the unranked-model guard answers instead, and the case proves nothing about names.
		"a tier that is not a usable name": strings.NewReplacer(
			`"codex":["helper","middling","frontier"]`, `"codex":["helper","middling","frontier","spare"]`,
			`"claude":["haiku","sonnet","opus"]`, `"claude":["haiku","sonnet","opus","ha iku"]`,
		).Replace(sample),
		// A fourth entry on each side again, so the lengths still match and every model a row names
		// is still ranked. Shortened or lengthened on one side alone, the length guard or the
		// unranked-model guard catches this and the duplicate guard is never reached.
		"one model at two tiers": strings.NewReplacer(
			`"codex":["helper","middling","frontier"]`, `"codex":["helper","middling","frontier","spare"]`,
			`"claude":["haiku","sonnet","opus"]`, `"claude":["haiku","sonnet","sonnet","opus"]`,
		).Replace(sample),
		"tier lists of different lengths": strings.Replace(sample, `"codex":["helper","middling","frontier"]`,
			`"codex":["helper","middling"]`, 1),

		// A task name is resolved as a relative path by every reader that finds the prompt a row
		// dispatches, and `/` has to be legal because `patrol/scout` is a real key — so each way a
		// segment can leave the tree is refused here, at the one place all of those readers share.
		// The field guide reads such a file and prints a line of it onto a committed page, so a name
		// that escapes is a read primitive rather than only a broken lookup.
		"task name climbing out":           strings.Replace(sample, `"bloat-judge":`, `"../../../etc/passwd":`, 1),
		"task name with a dot-dot segment": strings.Replace(sample, `"build/explore":`, `"build/../../../secret":`, 1),
		"task name with a dot segment":     strings.Replace(sample, `"build/explore":`, `"build/./explore":`, 1),
		"task name with an empty segment":  strings.Replace(sample, `"build/explore":`, `"build//explore":`, 1),
		"task name that is absolute":       strings.Replace(sample, `"bloat-judge":`, `"/etc/passwd":`, 1),
		"negative rolls":                   strings.Replace(sample, `"rolls":3`, `"rolls":-1`, 1),
		// Each way the field naming another row's prompt can name nothing, refused at parse time rather
		// than at the dispatch that would read it.
		"prompt owner is no row": strings.Replace(sample, `"claude":{"model":"sonnet"}`,
			`"claude":{"model":"sonnet"},"worker":"absent"`, 1),
		"prompt owner is a session": strings.Replace(sample, `"claude":{"model":"sonnet"}`,
			`"claude":{"model":"sonnet"},"worker":"kk-build"`, 1),
		"row names itself as its prompt's owner": strings.Replace(sample, `"claude":{"model":"sonnet"}`,
			`"claude":{"model":"sonnet"},"worker":"build/explore"`, 1),
		"session names a prompt owner": strings.Replace(sample, `"claude":{"model":"opus"}`,
			`"claude":{"model":"opus"},"worker":"bloat-judge"`, 1),
		"prompt owner names one in turn": strings.NewReplacer(
			`"claude":{"model":"sonnet"}`, `"claude":{"model":"sonnet"},"worker":"bloat-judge"`,
			`"rolls":3`, `"rolls":3,"worker":"build/explore"`).Replace(sample),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(raw)); err == nil {
				t.Fatal("invalid policy accepted")
			}
		})
	}
}

// The digest is the judge's cache identity, so a changed assignment has to change it or a stale
// verdict is reused under the new policy.
func TestDigestTracksEveryAssignment(t *testing.T) {
	first := policyForTest(t)
	changed, err := Parse([]byte(strings.Replace(sample, `"model":"haiku"`, `"model":"sonnet"`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest() == changed.Digest() {
		t.Fatal("a changed model left the digest alone")
	}
	rerolled, err := Parse([]byte(strings.Replace(sample, `"rolls":3`, `"rolls":5`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest() == rerolled.Digest() {
		t.Fatal("a changed roll count left the digest alone")
	}
}

func TestARowNamingAnotherWorkersPromptKeepsItsOwnTier(t *testing.T) {
	raw := strings.Replace(sample, `"claude":{"model":"sonnet"}`,
		`"claude":{"model":"sonnet"},"worker":"bloat-judge"`, 1)
	p, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Resolve(Request{Client: "claude", Task: "build/explore"})
	if err != nil || got.Requested.Model != "sonnet" || got.Worker != "bloat-judge" || got.Kind != "worker" {
		t.Fatalf("a row naming another's prompt = %+v, %v", got, err)
	}
	if named := p.PromptOwners(); len(named) != 1 || named["build/explore"] != "bloat-judge" {
		t.Fatalf("PromptOwners() = %v", named)
	}
	// The rows that own their prompt say nothing, so a caller reads the field as "someone else's".
	own, err := p.Resolve(Request{Client: "claude", Task: "bloat-judge"})
	if err != nil || own.Worker != "" {
		t.Fatalf("a row owning its prompt = %+v, %v", own, err)
	}
}

// The self-reference refusal is observed only by its message: the chain refusal below it catches a
// one-row cycle too. A row naming itself declares no prompt at all, and being told it names a further
// row's prompt in turn sends its author hunting for a second row that was never there.
func TestARowNamingItselfIsToldThatRatherThanToldOfAChain(t *testing.T) {
	raw := strings.Replace(sample, `"claude":{"model":"sonnet"}`,
		`"claude":{"model":"sonnet"},"worker":"build/explore"`, 1)
	_, err := Parse([]byte(raw))
	if err == nil {
		t.Fatal("a row naming itself as the prompt it dispatches was accepted")
	}
	if !strings.Contains(err.Error(), "names itself") {
		t.Fatalf("the refusal does not say the row names itself: %v", err)
	}
}

// The rank is the whole point of the order: a caller comparing two rows needs "cheaper" and
// "nothing ranks this" to be two answers, never one.
func TestTierOfRanksCheapestFirstAndSaysWhenItCannot(t *testing.T) {
	p := policyForTest(t)
	for _, want := range []struct {
		client string
		model  string
		rank   int
	}{
		{"claude", "haiku", 0}, {"claude", "sonnet", 1}, {"claude", "opus", 2},
		{"codex", "helper", 0}, {"codex", "middling", 1}, {"codex", "frontier", 2},
	} {
		rank, ranked := p.TierOf(want.client, want.model)
		if !ranked || rank != want.rank {
			t.Errorf("TierOf(%q, %q) = %d, %v; want %d, true", want.client, want.model, rank, ranked, want.rank)
		}
	}
	// A model no list carries, and a client the policy does not price, both answer false rather than
	// the zero rank — the cheapest tier, and so the one wrong answer that would read as a pass.
	for _, absent := range []struct{ client, model string }{
		{"claude", "nonesuch"}, {"nobody", "haiku"},
	} {
		if rank, ranked := p.TierOf(absent.client, absent.model); ranked {
			t.Errorf("TierOf(%q, %q) claimed rank %d", absent.client, absent.model, rank)
		}
	}
}

// The absent-order guard cannot be isolated by a table that only asks whether a document was
// refused. Delete the guard and this document is still refused: with no order, every model its rows
// name is unranked, so the unranked-model guard picks the question up and the table stays green.
//
// So this case asks which guard spoke. A policy carrying no order at all must be refused for
// carrying no order, because that refusal is the sentence someone has to read to know what to add.
func TestADocumentWithNoTierOrderIsRefusedForThat(t *testing.T) {
	_, err := Parse([]byte(noTierOrder))
	if err == nil {
		t.Fatal("a policy with no tier order was accepted")
	}
	if !strings.Contains(err.Error(), "orders no codex models") {
		t.Errorf("refused for the wrong reason, so the absent order is not what this pins: %v", err)
	}
}

package voicecheck

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The brief's model notes are what a writer copies, so every check passes them. The third modelled the
// shape a reviewer rejected, a fact with no act behind a summary ending on a participle, and runs 18 and
// 20 wrote it again.
func TestTheBriefsModelNotesPassEveryCheck(t *testing.T) {
	brief, err := os.ReadFile("../../kk-flavor/workers/comment-writer.md")
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile("(?s)```ts\n(.*?)```").FindSubmatch(brief)
	if block == nil {
		t.Fatal("the brief holds no ts block of model notes")
	}
	lines := strings.Split(strings.TrimSuffix(string(block[1]), "\n"), "\n")
	s := voiceScanner()
	s.profile = ProfileComment
	if found := s.scanSource("models.ts", lines, nil, nil); len(found) != 0 {
		t.Errorf("%d finding(s) over the brief's model notes:\n%s", len(found), render(found))
	}
	// Each model on a declaration with a body states an act in its note.
	for n, b := range commentBlocksIn(lines, onlyAdded(nil)) {
		if b.end >= len(lines) || dataDeclaration(lines[b.end:]) {
			continue
		}
		if text := strings.Join(lines[b.start-1:b.end], "\n"); !noteStatesAnAct(text, declaredName(lines[b.end:])) {
			t.Errorf("model note %d states no act:\n%s", n+1, text)
		}
	}
}

// A note on a body states its act by a connector, a verb-first sentence, this code as subject or a name
// of code. The reviewed block stated a fact and stopped, and three sound notes at c528b1028 stated their
// act each a different way.
func TestANoteStatesItsActOrDrawsAFinding(t *testing.T) {
	for _, tc := range []struct {
		block string
		acts  bool
	}{
		{"// Tells whether the ledger grants the scheme, asked without a mode. On a ledger that ignores the mode,\n// the first posting stalls.", false},
		{"// A server that fails the request answers with no body. This branch passes such a response through unchanged.", true},
		{"// An XHR for a data URI fails there. A test that sends one is skipped by `skipOnPlatform`.", true},
		{"// Rejects a set in which no entry declares the currency. Such a set would end up empty.", true},
		{"// A ledger that ignores the mode stalls on its first posting, so the scheme is asked with no mode.", true},
	} {
		if got := noteStatesAnAct(tc.block, "claimFor"); got != tc.acts {
			t.Errorf("states an act %v, want %v:\n%s", got, tc.acts, tc.block)
		}
	}
	s := scanner{profile: ProfileComment}
	for _, tc := range []struct {
		sentence string
		fires    bool
	}{
		{"Tells whether the ledger grants a scheme, asked without a settlement mode.", true},
		{"Returns the claim, or Unknown when the ledger answers nothing.", false},
		{"Tells whether a posting settles, by expecting a refusal.", false},
	} {
		fired := false
		s.sentenceShape(tc.sentence, 0, func(check string, _, _ int) { fired = fired || check == checkTrailingAct })
		if fired != tc.fires {
			t.Errorf("trailing participle on %q: %v, want %v", tc.sentence, fired, tc.fires)
		}
	}
}

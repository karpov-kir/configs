// Counts the shapes a comment rule proposes to forbid, over a set of files the repository does not
// hold. Every rule this campaign wrote as a word test or a length test had to be re-cut once someone
// counted it. A proposed rule is counted here before it is written down.
//
// The set is named by an environment variable and never by a committed path, and the report carries
// counts and sample sentences alone. A case is named by its position in the sorted set.
package commentcensus

import (
	"regexp"
	"sort"
	"strings"

	readerjudge "kk-flavor/tools/reader-judge"
)

// Block is one comment block with its marker characters removed. OverDecl says whether a
// declaration sits under it.
type Block struct {
	Line      int
	Span      int
	Text      string
	OverDecl  bool
	Sentences []string
}

var markerHead = regexp.MustCompile(`^\s*(///|//|/\*\*|/\*|\*/|\*|#)\s?`)
var markerTail = regexp.MustCompile(`\s*\*/\s*$`)

// declaration is the shape of a line a summary can sit over. The shape is loose on purpose. A count
// of summaries is the denominator of every rule measured here, and a line this misses becomes a
// note, which understates a rule's reach.
var declaration = regexp.MustCompile(`^\s*(export\s+|default\s+|public\s+|private\s+|protected\s+|static\s+|async\s+|abstract\s+)*(function|const|let|var|class|interface|type|enum|func|def)\b|^\s*[A-Za-z_$][\w$]*\s*[:(]|^\s*[A-Za-z_$][\w$]*\s*=\s*(\(|function|async)`)

// Blocks reads a file's comment blocks and says which of them stand over a declaration.
func Blocks(lines []string) []Block {
	var out []Block
	for _, u := range readerjudge.CommentBlocks(lines) {
		var text []string
		for offset := 0; offset < u.Span; offset++ {
			raw := lines[u.Line-1+offset]
			cut := markerTail.ReplaceAllString(markerHead.ReplaceAllString(raw, ""), "")
			text = append(text, strings.TrimSpace(cut))
		}
		joined := strings.TrimSpace(strings.Join(text, " "))
		if joined == "" {
			continue
		}
		next := u.Line + u.Span
		for next <= len(lines) && strings.TrimSpace(lines[next-1]) == "" {
			next++
		}
		over := next <= len(lines) && declaration.MatchString(lines[next-1])
		out = append(out, Block{Line: u.Line, Span: u.Span, Text: joined, OverDecl: over, Sentences: Sentences(joined)})
	}
	return out
}

var sentenceEnd = regexp.MustCompile(`(?:[.!?])(?:\s+|$)`)

// Sentences splits on terminal punctuation. A doc tag line carries no sentence and drops out. An
// abbreviation splits one sentence in two, which inflates the sentence count and leaves the finding
// counts alone.
func Sentences(text string) []string {
	var out []string
	for _, piece := range sentenceEnd.Split(text, -1) {
		piece = strings.TrimSpace(piece)
		if piece == "" || strings.HasPrefix(piece, "@") {
			continue
		}
		out = append(out, piece)
	}
	return out
}

// Summary is the first sentence of a block standing over a declaration. Note is every other sentence
// in any block. The two are counted apart because a rule aimed at one reaches the other by accident.
func (b Block) Summary() (string, bool) {
	if !b.OverDecl || len(b.Sentences) == 0 {
		return "", false
	}
	return b.Sentences[0], true
}

func (b Block) Notes() []string {
	if s, ok := b.Summary(); ok && len(b.Sentences) > 0 {
		_ = s
		return b.Sentences[1:]
	}
	return b.Sentences
}

// A shape is one proposed rule. Its count is the sentences it would reach.
type Shape struct {
	Name string
	Over string // "note", "summary" or "either"
	Hits func(sentence string) string
}

func firstMatch(res []*regexp.Regexp) func(string) string {
	return func(s string) string {
		for _, re := range res {
			if m := re.FindString(s); m != "" {
				return m
			}
		}
		return ""
	}
}

func mustAll(patterns ...string) []*regexp.Regexp {
	var out []*regexp.Regexp
	for _, p := range patterns {
		out = append(out, regexp.MustCompile(p))
	}
	return out
}

// WordCount counts whitespace-separated words, which is what every length rule in this tree means by
// a word.
func WordCount(s string) int { return len(strings.Fields(s)) }

// Shapes are the rules proposed for the note and the summary, each counted on its own so a rule that
// only duplicates another can be dropped before it lands.
func Shapes() []Shape {
	return []Shape{
		{"note-connective", "note", firstMatch(mustAll(
			`(?i)\b(so|because|since|which|unless|where|while|although|whereas)\b`,
			`(?i)\bas soon as\b`))},
		{"so-clause", "note", firstMatch(mustAll(`(?i),\s+so\b`))},
		{"counterfactual-consequence", "note", firstMatch(mustAll(
			`(?i)\bso\b[^.]*\b(would|could)\b`))},
		{"anthropomorphism", "either", firstMatch(mustAll(
			`(?i)\b(a|the|its|their)\s+(yes|no)\b`,
			`(?i)\bsay(s|ing)?\s+(yes|no)\b`,
			`(?i)\banswers?\s+(yes|no)\b`))},
		{"elided-verb", "either", firstMatch(mustAll(
			`(?i)\b(as|than|like|so)\s+(the|a|an|its|their)\s+\w+\s+(does|do|did)\b`))},
		{"negated-case", "summary", firstMatch(mustAll(
			`(?i)\bor\s+[\w']+\s+(unless|except)\b`))},
		{"long-sentence-15", "either", func(s string) string {
			if WordCount(s) > 15 {
				return s
			}
			return ""
		}},
	}
}

// MetaphorVerbs are the figures of speech the writing standard already names, plus the ones a review
// read as unclear. Each is counted alone: the standard's list was never measured, and a word that
// reads as a figure in one sentence is the literal verb in another.
func MetaphorVerbs() []string {
	return []string{"settle", "hide", "cover", "reach", "sit in", "climb", "slip past",
		"rubber-stamp", "hedge", "load-bearing", "understate", "answer"}
}

var verbForms = map[string][]string{}

// MetaphorHit says which listed verb a sentence carries, in any of its inflected forms.
func MetaphorHit(sentence string) string {
	low := strings.ToLower(sentence)
	for _, stem := range MetaphorVerbs() {
		for _, form := range formsOf(stem) {
			if wordIn(low, form) {
				return stem
			}
		}
	}
	return ""
}

func formsOf(stem string) []string {
	if cached, ok := verbForms[stem]; ok {
		return cached
	}
	forms := []string{stem}
	switch {
	case strings.Contains(stem, " ") || strings.Contains(stem, "-"):
		head, rest, _ := strings.Cut(stem, map[bool]string{true: " ", false: "-"}[strings.Contains(stem, " ")])
		join := map[bool]string{true: " ", false: "-"}[strings.Contains(stem, " ")]
		for _, f := range formsOf(head) {
			forms = append(forms, f+join+rest)
		}
	case strings.HasSuffix(stem, "e"):
		forms = append(forms, stem+"s", stem+"d", strings.TrimSuffix(stem, "e")+"ing")
	default:
		forms = append(forms, stem+"s", stem+"ed", stem+"ing")
	}
	verbForms[stem] = forms
	return forms
}

func wordIn(haystack, word string) bool {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(word) + `\b`)
	return re.MatchString(haystack)
}

var identifierWord = regexp.MustCompile(`[A-Za-z_$][\w$]*`)
var hyphenPair = regexp.MustCompile(`\b([a-z]+)-([a-z]+)\b`)

// CoinedCompounds returns the hyphenated pairs in a sentence that the file's own identifiers do not
// carry, in either the hyphenated or the camelCase spelling. A compound the code spells is the
// domain's word. One the code lacks is a word the comment invented.
func CoinedCompounds(sentence string, identifiers map[string]bool) []string {
	var out []string
	for _, m := range hyphenPair.FindAllStringSubmatch(sentence, -1) {
		joined := strings.ToLower(m[1] + m[2])
		if identifiers[strings.ToLower(m[0])] || identifiers[joined] {
			continue
		}
		out = append(out, m[0])
	}
	return out
}

// Identifiers collects every identifier the file spells, lowercased, so a comment's compound can be
// asked whether the code carries it.
func Identifiers(lines []string, blocks []Block) map[string]bool {
	inComment := map[int]bool{}
	for _, b := range blocks {
		for offset := 0; offset < b.Span; offset++ {
			inComment[b.Line+offset] = true
		}
	}
	out := map[string]bool{}
	for i, line := range lines {
		if inComment[i+1] {
			continue
		}
		for _, word := range identifierWord.FindAllString(line, -1) {
			out[strings.ToLower(word)] = true
		}
	}
	return out
}

// Tally is one shape's count with a sample of what it reached, so a number is read beside the
// sentences behind it.
type Tally struct {
	Name    string
	Count   int
	Samples []string
}

const samplesKept = 8

func (t *Tally) add(sentence string) {
	t.Count++
	if len(t.Samples) < samplesKept {
		t.Samples = append(t.Samples, sentence)
	}
}

// Report is what one run of the census measured.
type Report struct {
	Files     int
	Blocks    int
	Summaries int
	Notes     int
	Shapes    []Tally
	Verbs     []Tally
	Coined    Tally
	Long      Tally
}

// Measure counts every shape over the files handed to it. A file is a name and its lines. The name
// orders the set, and the report carries counts and matched sentences alone.
func Measure(files [][]string) Report {
	shapes := Shapes()
	rep := Report{Files: len(files)}
	byName := map[string]*Tally{}
	for _, s := range shapes {
		t := &Tally{Name: s.Name}
		byName[s.Name] = t
		rep.Shapes = append(rep.Shapes, Tally{})
	}
	verbs := map[string]*Tally{}
	for _, v := range MetaphorVerbs() {
		verbs[v] = &Tally{Name: v}
	}
	coined := &Tally{Name: "coined-compound"}
	long := &Tally{Name: "note-over-2-sentences"}

	for _, lines := range files {
		blocks := Blocks(lines)
		identifiers := Identifiers(lines, blocks)
		rep.Blocks += len(blocks)
		for _, b := range blocks {
			summary, hasSummary := b.Summary()
			notes := b.Notes()
			if hasSummary {
				rep.Summaries++
			}
			rep.Notes += len(notes)
			if len(notes) > 2 {
				long.add(strings.Join(notes, " "))
			}
			for _, s := range shapes {
				consider := func(sentence string) {
					if hit := s.Hits(sentence); hit != "" {
						byName[s.Name].add(sentence)
					}
				}
				if hasSummary && (s.Over == "summary" || s.Over == "either") {
					consider(summary)
				}
				if s.Over == "note" || s.Over == "either" {
					for _, n := range notes {
						consider(n)
					}
				}
			}
			for _, sentence := range b.Sentences {
				if stem := MetaphorHit(sentence); stem != "" {
					verbs[stem].add(sentence)
				}
				for _, c := range CoinedCompounds(sentence, identifiers) {
					coined.add(c + " — " + sentence)
				}
			}
		}
	}
	rep.Shapes = rep.Shapes[:0]
	for _, s := range shapes {
		rep.Shapes = append(rep.Shapes, *byName[s.Name])
	}
	for _, v := range MetaphorVerbs() {
		rep.Verbs = append(rep.Verbs, *verbs[v])
	}
	sort.SliceStable(rep.Verbs, func(i, j int) bool { return rep.Verbs[i].Count > rep.Verbs[j].Count })
	rep.Coined = *coined
	rep.Long = *long
	return rep
}

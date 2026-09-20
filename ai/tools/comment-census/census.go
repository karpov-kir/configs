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
		// `yes` takes modifiers between the article and the word. The block that prompted this rule
		// said "a scheme-blind yes", and the adjacent form passed straight over it. The `no` form
		// stays adjacent, because `no` is the determiner in a phrase like "no row". A wider form counts
		// each such phrase as a boolean written as a person.
		{"anthropomorphism", "either", firstMatch(mustAll(
			`(?i)\b(a|an|the|its|their|his|her|our|your)\s+(?:[a-z][a-z-]*\s+){0,2}yes\b`,
			`(?i)\b(a|an|the|its|their|his|her|our|your)\s+no\b`,
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

var soClause = regexp.MustCompile(`(?i),\s+so\s+(.{0,80})`)
var backticked = regexp.MustCompile("`([^`]+)`")

// soSubjectWords is how far into a `so` clause the subject is looked for. A subject longer than this
// is a clause the pattern already refuses on its length.
const soSubjectWords = 5

// subjectPronoun opens a clause that carries the sentence's own subject forward. These are counted
// apart, because the pattern asks a consequence clause for an element by name.
var subjectPronoun = map[string]bool{"it": true, "they": true, "this": true, "that": true,
	"these": true, "those": true, "he": true, "she": true, "we": true, "you": true}

// SoSubject is what a consequence clause puts in its subject position.
type SoSubject int

const (
	// SoNoClause says the sentence carries no `so` clause.
	SoNoClause SoSubject = iota
	// SoNamesCode says the subject is an element the file spells.
	SoNamesCode
	// SoPronoun says the subject refers back to the sentence's own subject.
	SoPronoun
	// SoNamesNoElement says the subject is a noun the file never spells.
	SoNamesNoElement
)

// soClauseKind sorts a sentence's consequence clause into one of the kinds SoSubject names. A clause
// naming no element of the code is what the pattern under review drops.
func soClauseKind(sentence string, identifiers map[string]bool) (clause string, kind SoSubject) {
	clause, namesCode, found := SoClauseSubject(sentence, identifiers)
	switch {
	case !found:
		return clause, SoNoClause
	case namesCode:
		return clause, SoNamesCode
	}
	if fields := strings.Fields(strings.ToLower(clause)); len(fields) > 0 && subjectPronoun[strings.Trim(fields[0], "`,.")] {
		return clause, SoPronoun
	}
	return clause, SoNamesNoElement
}

// SoClauseSubject reads the clause after `, so` and says whether its subject is an element the file
// spells. A clause naming its element inside backticks counts as naming it.
func SoClauseSubject(sentence string, identifiers map[string]bool) (clause string, namesCode, found bool) {
	m := soClause.FindStringSubmatch(sentence)
	if m == nil {
		return "", false, false
	}
	clause = strings.TrimSpace(m[1])
	head := clause
	if fields := strings.Fields(clause); len(fields) > soSubjectWords {
		head = strings.Join(fields[:soSubjectWords], " ")
	}
	for _, quoted := range backticked.FindAllStringSubmatch(head, -1) {
		for _, word := range identifierWord.FindAllString(quoted[1], -1) {
			if identifiers[strings.ToLower(word)] {
				return clause, true, true
			}
		}
	}
	for _, word := range identifierWord.FindAllString(head, -1) {
		if identifiers[strings.ToLower(word)] && len(word) > 2 {
			return clause, true, true
		}
	}
	return clause, false, true
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
	SoNamed   Tally
	SoPronoun Tally
	SoUnnamed Tally
	SoBoth    Tally
	Restating Tally
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
	soNamed := &Tally{Name: "so-clause-naming-the-code"}
	soUnnamed := &Tally{Name: "so-clause-naming-no-element"}
	soPronoun := &Tally{Name: "so-clause-with-a-pronoun-subject"}
	soBoth := &Tally{Name: "of those, also counterfactual"}
	restating := &Tally{Name: "restates-code"}
	counterfactual := Shapes()[2]

	for _, lines := range files {
		blocks := Blocks(lines)
		identifiers := Identifiers(lines, blocks)
		for _, b := range blocks {
			summary, ok := b.Summary()
			if !ok {
				continue
			}
			if survived, hadContent := Restates(summary, DeclarationWords(lines, b)); hadContent && len(survived) == 0 {
				restating.add(summary)
			}
		}
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
			for _, n := range notes {
				switch clause, kind := soClauseKind(n, identifiers); kind {
				case SoNamesCode:
					soNamed.add(clause)
				case SoPronoun:
					soPronoun.add(clause)
				case SoNamesNoElement:
					soUnnamed.add(clause)
					if counterfactual.Hits(n) != "" {
						soBoth.add(clause)
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
	rep.SoNamed = *soNamed
	rep.SoPronoun = *soPronoun
	rep.SoBoth = *soBoth
	rep.Restating = *restating
	rep.SoUnnamed = *soUnnamed
	return rep
}

// openingVerbs are the verbs a summary opens with. The writer's strike step removes them before it
// asks what the summary adds, because "Returns" over a function that returns is the return type
// again.
var openingVerbs = map[string]bool{"check": true, "whether": true, "return": true, "list": true,
	"say": true, "give": true, "declare": true, "hold": true, "name": true, "read": true,
	"write": true, "take": true, "yield": true, "produce": true, "provide": true, "get": true,
	"set": true, "pair": true, "map": true}

// stopWords carry no content. The strike passes over them, and they count for neither side of it.
var stopWords = map[string]bool{"a": true, "an": true, "the": true, "of": true, "for": true,
	"in": true, "on": true, "to": true, "and": true, "or": true, "its": true, "it": true,
	"every": true, "each": true, "with": true, "that": true, "this": true, "from": true,
	"by": true, "is": true, "are": true, "as": true, "at": true, "be": true, "one": true,
	"given": true, "into": true, "which": true, "their": true, "them": true, "then": true,
	"where": true, "when": true, "all": true, "any": true, "no": true, "not": true, "if": true}

// stemOf folds a plural and a verb form onto one word, because the strike step counts those as the
// same word as the identifier's.
func stemOf(word string) string {
	word = strings.ToLower(strings.Trim(word, "`'\".,:;()[]{}"))
	for _, suffix := range []string{"ies", "ing", "es", "ed", "s"} {
		if len(word) > len(suffix)+2 && strings.HasSuffix(word, suffix) {
			if suffix == "ies" {
				return strings.TrimSuffix(word, "ies") + "y"
			}
			word = strings.TrimSuffix(word, suffix)
			break
		}
	}
	// A trailing `e` goes too, so that price and priced fold onto one stem. Without it the strike
	// misses every identifier whose verb form the summary spells as a noun.
	if len(word) > 3 && strings.HasSuffix(word, "e") {
		return strings.TrimSuffix(word, "e")
	}
	return word
}

// bodyWindow bounds how far past a declaration the strike step reads. The count of findings moves
// with it, and that is why restates-code only reports. The README holds the measurement.
const bodyWindow = 40

var camelBreak = regexp.MustCompile(`([a-z0-9])([A-Z])`)

// DeclarationWords collects the stems the declaration under a block spells: its identifier split at
// its camel humps, its parameter names, its return type and its body.
func DeclarationWords(lines []string, b Block) map[string]bool {
	at := b.Line + b.Span
	for at <= len(lines) && strings.TrimSpace(lines[at-1]) == "" {
		at++
	}
	out := map[string]bool{}
	depth, seenBrace := 0, false
	for i := at; i <= len(lines) && i < at+bodyWindow; i++ {
		line := lines[i-1]
		for _, word := range identifierWord.FindAllString(camelBreak.ReplaceAllString(line, "$1 $2"), -1) {
			out[stemOf(word)] = true
		}
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if strings.Contains(line, "{") {
			seenBrace = true
		}
		if seenBrace && depth <= 0 {
			break
		}
	}
	return out
}

// Restates says whether every content word of a summary is a word the declaration beneath it already
// spells. It returns the words that survived the strike, so a finding can be read beside them.
func Restates(summary string, declWords map[string]bool) (survived []string, hadContent bool) {
	for _, raw := range strings.Fields(camelBreak.ReplaceAllString(summary, "$1 $2")) {
		stem := stemOf(raw)
		if stem == "" || stopWords[stem] || openingVerbs[stem] {
			continue
		}
		hadContent = true
		if declWords[stem] {
			continue
		}
		survived = append(survived, stem)
	}
	return survived, hadContent
}

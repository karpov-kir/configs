// The register check: which sentences in a body, a reply or a rule file are written in the house
// register rather than in plain prose. It counts nothing. Each check names a shape a reader stumbles
// on, and reports the text that matched so the writer can see what to change.
//
// Deterministic, so two runs over one text print one report.
package voicecheck

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
)

const (
	// writing.md puts one idea in a sentence and keeps it under about 25 words. The check fires at 30
	// so a sentence at the rule's own edge is not a finding, and only a sentence carrying a second
	// idea is.
	maxSentenceWords = 30

	// The note pattern spends one subordinating connective on `so`, which puts a conforming note at one.
	// The check fires above that.
	flaggedConnectivesPerSentence = 2

	flaggedNegationsPerSentence = 2

	// Words after a semicolon before its tail reads as a clause, where the sentence carries several and
	// is therefore a list. One semicolon joins two clauses whatever its tail is: the tree holds eleven
	// with a tail of one to three words, and every one of them joins two sentences.
	minClauseTailWords = 4
)

// Findings and echoed text are bounded: under kk-pr this text comes off a branch somebody else wrote.
const (
	maxFindings   = 500
	maxMatchBytes = 120
	// What a body on stdin may be. A stream read to a cap and TRUNCATED would report clean over the part
	// the scan never saw, so a stream over this is refused.
	maxStdinBytes = 64 * 1024 * 1024
)

// Profile is which text the scan reads and which checks apply to it.
type Profile string

const (
	// ProfileProse reads a markdown or plain-text file whole: a PR body, a review comment, a reply.
	ProfileProse Profile = "prose"
	// ProfileInstruction reads a rule file under ai/kk-flavor/, skipping the frontmatter, fenced code
	// and section headings. A heading is a label, not a sentence, and holding one to sentence shape
	// would report every file in the tree for its own table of contents.
	ProfileInstruction Profile = "instruction"
)

// Finding is one match, printed as `<file>:<line>: <check>: <matched text>`.
type Finding struct {
	File  string
	Line  int
	Check string
	Text  string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: %s: %s", shell.CutBytesMarked(shell.Oneline(f.File), maxPathBytes),
		f.Line, f.Check, shell.CutBytesMarked(shell.Oneline(f.Text), maxMatchBytes))
}

// The checks, each named so a report can name it.
const (
	checkBold          = "bold"
	checkContrast      = "contrast"
	checkCounterfactal = "counterfactual-opener"
	checkNoSubject     = "no-subject"
	checkIntensifier   = "intensifier"
	checkPositional    = "positional"
	checkCoined        = "coined"
	checkCounterfact   = "counterfactual-consequence"
	checkAnthropo      = "anthropomorphism"
	checkElidedVerb    = "elided-verb"
	checkLongSentence  = "long-sentence"
	checkClauseDepth   = "clause-depth"
	checkDoubleNeg     = "double-negative"
	checkSemicolon     = "semicolon"
)

// AllChecks is every check name, which the suite reads to prove each one fires on its corpus.
var AllChecks = []string{checkBold, checkContrast, checkCounterfactal, checkNoSubject,
	checkIntensifier, checkPositional, checkCoined, checkCounterfact, checkAnthropo, checkElidedVerb,
	checkLongSentence, checkClauseDepth, checkDoubleNeg, checkSemicolon, checkTestsNarration}

var (
	// A bold span opening on a word or a backtick. `**` around a space is markdown that did not close.
	reBold = regexp.MustCompile(`\*\*[^*\s][^*]*\*\*`)

	// The contrast spine: the sentence is about the thing the reader did not ask about. `rather than`
	// and `instead of` are the same frame; `, never X` and `, not a X` are it with the verb dropped.
	// `instead of` only where it heads a clause. A host repository writes "accepts a typed event
	// instead of an arbitrary string", which names two real alternatives, and the frame this check is
	// after is the one that opens on the alternative in order to pre-refute it.
	reRatherThan = regexp.MustCompile(`\brather than\b`)
	reNeverNot   = regexp.MustCompile(`,\s+never\s+[a-z]|,\s+not\s+(?:a|an|the|its|his|her|their|our|your|[a-z]+s\b)`)
	// The same spine with the comma dropped and a conjunction in its place. "It is logged and not
	// believed", "a survey and no verdict" — the reader is still being told about the thing they did
	// not ask about.
	reAndNot = regexp.MustCompile(`[^,]\s+and\s+(?:not|no)\s+(?:a|an|the|its|his|her|their|our|your|[a-z]+)`)
	// What stands before the conjunction, asked whether it is already a negation.
	reNegated   = regexp.MustCompile(`(?i)\b(?:no|not|never|nothing|nobody|none)\b`)
	reInsteadOf = regexp.MustCompile(`(?:^|[.;:]\s+|,\s+)[Ii]nstead of\b`)

	// A symbol defined against another symbol. The sentence is about a thing the reader did not ask
	// about, and the reader opens the other symbol to learn what this one does.
	reDefinedAgainst = regexp.MustCompile(`(?i)\ba different question from\b|\bthe other half of what\b|\bunlike \{@link\b`)

	// The connectives a subordinate clause hangs off. A sentence is a finding at two. Past that the
	// reader holds one clause open while reading another.
	reConnective = regexp.MustCompile(`\b(?:because|so|since|where|while|which|whose|although|unless|whereas)\b`)

	// A preposition before `which` defines a term, as in "a book in which no posting declares the
	// currency". The writing rules ask for that wording, so it opens no clause the reader holds.
	rePrepositionWhich = regexp.MustCompile(`\b(?:in|of|to|for|on|at|by|under|with|from) which\b`)

	// Negation a reader has to carry. At two in a sentence the reader resolves them against each other
	// before learning what the sentence says.
	reNegation = regexp.MustCompile(`\b(?:no|not|never|neither|nor|nothing|nobody|without)\b`)

	// A semicolon with a clause after it. Four words is where the tail stops reading as a list item and
	// starts reading as a sentence of its own.
	reSemicolon = regexp.MustCompile(`;\s`)

	// A sentence opening on the wrong implementation, which the reader has to imagine before they can
	// read the right one.
	reCounterfactual = regexp.MustCompile(`^(?:Otherwise\b|Without this\b|Left\s+[a-z]|Were\s+[a-z]|Had\s+[a-z])`)
	reReadAloneAnd   = regexp.MustCompile(`^Read\b.*\balone and\b`)

	// A clause whose subject was dropped. Three shapes: a past participle heading the sentence
	// ("Counted across ..."), the same participle one clause in (", paired with its set") where no
	// verb opened the sentence, and a gerund heading it ("Reading representations out of ...").
	//
	// The gerund pattern captures the stem, because an imperative can end in `ing` too: "Bring the
	// choice to the human" is the register this check exists to reward, and opensWithGerund holds the
	// two apart by what is left when `ing` comes off.
	reParticipleOpen = regexp.MustCompile(`^(?:[A-Z][a-z]+ed|Said|Held|Left|Built|Read|Kept|Meant|Written|Drawn|Taken|Given|Shown|Made|Sent|Spelt|Brought|Found|Lost|Split)\s+` + prepositions + `\b`)
	reParticiplePhr  = regexp.MustCompile(`,\s+(?:[a-z]+ed|said|held|left|built|read|kept|meant|written|drawn|taken|given|shown|made|sent)\s+` + prepositions + `\b`)
	reGerundOpen     = regexp.MustCompile(`^([A-Z][a-z]+)ing\s+[a-z]`)

	// A sentence opening on a verb in the third person, which is the form the rule asks a summary to
	// take: "Lists …", "Returns …", "Checks whether …". Used only to suppress the participial-phrase
	// trigger below, because "Lists every entry, paired with its book" has its subject and its verb
	// and the phrase is a reduced relative clause. A plural noun opening a sentence ("Entries paired
	// with …") is suppressed with it, which is the cost of reading shape rather than grammar.
	reOpensWithVerb = regexp.MustCompile(`^[A-Z][a-z]+s\s`)

	// Words ending in `ed` that are no participle, so the participial arm stops reading them as a
	// dropped subject. The list names what the instruction tree holds: `red` 16 times, `need` 16,
	// `seed` 5, `fed` once, and `proceed`, which is the word firing a finding today. A length floor
	// was the first idea, and the measurement ended it: `fed` is three letters and a participle.
	notAParticiple = map[string]bool{
		"red": true, "bed": true, "shed": true, "sled": true, "embed": true, "need": true,
		"seed": true, "feed": true, "deed": true, "heed": true, "creed": true, "greed": true,
		"speed": true, "breed": true, "indeed": true, "exceed": true, "proceed": true, "succeed": true,
	}

	// "nothing", "nobody" and "the one X" used to make a claim feel larger than it is.
	reIntensifier = regexp.MustCompile(`(?i)\bnothing\b|\bnobody\b|\bno one\b|\bthe one\s`)

	// An adverb that raises a claim without adding to it. The word before decides. A DEFINITE
	// determiner makes it the emphatic form, as in "this run" or "the case", and all twenty-two uses
	// in the tree are that form. An indefinite determiner leaves the adverb, and the adverb inflates
	// a claim where the emphatic form picks a thing out.
	reEmphasis = regexp.MustCompile(`(?i)(\w+\s+)?\b(?:very|crucially|vitally|extremely)\b`)
	rePointing = regexp.MustCompile(`(?i)^(?:the|this|that|these|those|its|his|her|their|our|your)\s`)

	// "the token above" where the token has a name. Anchored on the noun phrase the reader is sent to
	// hunt for, so "see the note above" fires and a bare "above" inside a sentence about layout does not.
	rePositional = regexp.MustCompile(`\b(?:the|this|that|these|those|a|an)\s+(?:[A-Za-z@{}.\-]+\s+){0,3}(?:above|below)\b`)

	// An inline code span. A backticked identifier, path or literal is the code's own text quoted into
	// prose, so the register checks read past it: `Next: <the one immediate action>` is a template an
	// agent types out, and holding it to sentence shape would report the rule for stating itself.
	reInlineCode = regexp.MustCompile("`[^`]*`")
)

const prepositions = `(?:across|with|by|out|for|to|in|on|at|against|from|through|over|under|rather|into|past|beside|alongside|about|around|after|before|between|within|without|than|as)`

// sentenceSpans cuts on a sentence terminator followed by a space, and returns each sentence as a
// half-open range rather than a string. Ranges, because every check runs over the line with its inline
// code blanked and every finding echoes the line as the writer typed it: one offset has to address
// both, and blanking keeps them the same length.
//
// Written by hand because Go's regexp has no lookbehind, and a split that consumed the terminator
// would take the `.` off the sentence a check then reads for its opening word.
func sentenceSpans(text string) [][2]int {
	var out [][2]int
	add := func(from, to int) {
		for from < to && text[from] == ' ' || from < to && text[from] == '\t' {
			from++
		}
		for to > from && (text[to-1] == ' ' || text[to-1] == '\t') {
			to--
		}
		if from < to {
			out = append(out, [2]int{from, to})
		}
	}
	start := 0
	for i := 0; i < len(text)-1; i++ {
		if text[i] != '.' && text[i] != '!' && text[i] != '?' {
			continue
		}
		if text[i+1] != ' ' && text[i+1] != '\t' {
			continue
		}
		add(start, i+1)
		start = i + 1
	}
	add(start, len(text))
	return out
}

// scanner holds one run's settings so both profiles reach the same checks.
type scanner struct {
	profile Profile
	// coined is words a caller names beside the built-in ones. No run names any: a repository keeps no
	// word list. The suite does, to reach the checks with a word of its own.
	coined []string
	// kind is the body a prose text is read as, empty for none. template is its template's lines,
	// which the checks leave unread.
	kind     string
	template map[string]bool
	// inCell says the segment is one cell of a table row. A cell is a list by construction. A semicolon
	// in one separates two fields, and the same semicolon in prose joins two clauses.
	inCell bool
}

// segment is one run of text a check reads whole, with the line every byte of it came from. A markdown
// paragraph is written across several lines and read as one, so a check that ran per line would never
// see a sentence that wraps.
type segment struct {
	text   string
	lineOf []int
}

// join builds a segment out of the lines from..to, taking each line's words through strip and putting
// a single space between them. The space is a byte of the segment too, and it is attributed to the
// line it precedes, so a match starting at the seam reports the line the sentence continues onto
// rather than the one it left.
func join(lines []string, from, to int, strip func(string) string) segment {
	var seg segment
	var b strings.Builder
	for at := from; at <= to && at <= len(lines); at++ {
		words := strip(lines[at-1])
		if words == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
			seg.lineOf = append(seg.lineOf, at)
		}
		b.WriteString(words)
		for range []byte(words) {
			seg.lineOf = append(seg.lineOf, at)
		}
	}
	seg.text = b.String()
	return seg
}

// lineAt is the source line one byte of the segment came from, and lineSpan the lines a match covers.
func (seg segment) lineAt(offset int) int {
	if offset < 0 || offset >= len(seg.lineOf) {
		return 0
	}
	return seg.lineOf[offset]
}

func (seg segment) lineSpan(from, to int) (int, int) {
	first, last := seg.lineAt(from), seg.lineAt(from)
	for at := from; at < to && at < len(seg.lineOf); at++ {
		if line := seg.lineOf[at]; line > last {
			last = line
		}
	}
	return first, last
}

// scanProse reads a whole text file, a paragraph at a time. The instruction profile skips what a rule
// file uses as structure: its frontmatter, its fenced code, and its headings.
//
// A paragraph is a run of lines with no blank line in it. A list item starts a new one. A table row
// ends one and is read a cell at a time. Typed fields and wrapped files both arrive here.
func (s scanner) scanProse(file string, lines []string) []Finding {
	var found []Finding
	inFence, inFrontmatter := false, false
	paragraph := 0
	flush := func(end int) {
		if paragraph == 0 {
			return
		}
		found = append(found, s.scanSegment(file, join(lines, paragraph, end, strings.TrimSpace))...)
		paragraph = 0
	}
	for i, raw := range lines {
		at := i + 1
		line := strings.TrimSpace(raw)
		if s.profile == ProfileInstruction {
			if at == 1 && shell.IsFrontmatterDelimiter(line) {
				inFrontmatter, paragraph = true, 0
				continue
			}
			if inFrontmatter {
				inFrontmatter = !shell.IsFrontmatterDelimiter(line)
				continue
			}
			if shell.IsFenceDelimiter(line) {
				flush(at - 1)
				inFence = !inFence
				continue
			}
			if inFence || strings.HasPrefix(line, "#") {
				flush(at - 1)
				continue
			}
		}
		// A table row is data laid out in columns, and its cells carry prose a reader follows. A
		// paragraph made of them runs together into one pseudo-sentence, long and deeply clausal
		// however plain each cell is. Each cell is read on its own. The prose in a table stays under
		// the register, and the layout draws no finding of its own.
		if strings.HasPrefix(line, "|") {
			flush(at - 1)
			found = append(found, s.scanCells(file, at, line)...)
			continue
		}
		if line == "" {
			flush(at - 1)
			continue
		}
		// Items often carry no closing period, so a run of them joined into one paragraph reads as one
		// long sentence.
		if _, isItem := shell.ListMarker(line); isItem {
			flush(at - 1)
		}
		if paragraph == 0 {
			paragraph = at
		}
	}
	flush(len(lines))
	return found
}

// scanCells reads a table row's cells, each as its own segment. The delimiter row under a header
// carries dashes and colons. Every check passes over those, so it needs no case of its own.
func (s scanner) scanCells(file string, at int, row string) []Finding {
	var found []Finding
	for _, cell := range strings.Split(strings.Trim(row, "|"), "|") {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			continue
		}
		inCell := s
		inCell.inCell = true
		found = append(found, inCell.scanSegment(file, join([]string{cell}, 1, 1, strings.TrimSpace))...)
	}
	for i := range found {
		found[i].Line = at
	}
	return found
}

// participialPhrase is the first `, <past participle> <preposition>` a text carries, or nil. A regexp
// reads a word ending in `ed` and cannot tell a participle from a stem, so each match is read back
// and the words notAParticiple names are stepped over.
func participialPhrase(read string) []int {
	for _, at := range reParticiplePhr.FindAllStringIndex(read, -1) {
		fields := strings.Fields(strings.TrimLeft(read[at[0]:at[1]], ", "))
		if len(fields) > 0 && !notAParticiple[fields[0]] {
			return at
		}
	}
	return nil
}

// Three sentence shapes a reviewer read as confusing, each counted on sixty files of reviewed code
// before it became a check.
var (
	// A consequence about code that does not exist. The reader inverts it to learn what this code
	// does. 14 of 304 notes.
	reSoWould = regexp.MustCompile(`(?i)\bso\b[^.]*\b(would|could)\b`)

	// A boolean written as a person answering, at 3 of 711 sentences. The first form takes modifiers
	// between the article and the word: the sentence that prompted this rule put two there. The
	// second form stays adjacent, because that word is a determiner in the phrase "no row".
	reAnthropomorphic = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(a|an|the|its|their|his|her|our|your)\s+(?:[a-z][a-z-]*\s+){0,2}yes\b`),
		regexp.MustCompile(`(?i)\b(a|an|the|its|their|his|her|our|your)\s+no\b`),
		regexp.MustCompile(`(?i)\bsay(s|ing)?\s+(yes|no)\b`),
		regexp.MustCompile(`(?i)\banswers?\s+(yes|no)\b`),
	}

	// A verb the sentence borrows from a clause before it, which the reader supplies again. 1 of 711.
	reElidedVerb = regexp.MustCompile(`(?i)\b(as|than|like|so)\s+(the|a|an|its|their)\s+\w+\s+(does|do|did)\b`)

	reComparativeTail = regexp.MustCompile(`(?i)^\s+(more|longer|further|fewer|less|worse|better)\b`)
)

// sentenceShapes are the three checks above, in the order a segment is read for them.
var sentenceShapes = []struct {
	check    string
	patterns []*regexp.Regexp
}{
	{checkCounterfact, []*regexp.Regexp{reSoWould}},
	{checkAnthropo, reAnthropomorphic},
	{checkElidedVerb, []*regexp.Regexp{reElidedVerb}},
}

// scanSegment is every check over one segment, and the only place a check runs. One function, so the
// two profiles cannot drift into reading the same sentence differently.
func (s scanner) scanSegment(file string, seg segment) []Finding {
	if seg.text == "" {
		return nil
	}
	text := seg.text
	var found []Finding
	add := func(check string, from, to int) {
		first, _ := seg.lineSpan(from, to)
		found = append(found, Finding{File: file, Line: first, Check: check,
			Text: strings.TrimSpace(text[from:to])})
	}

	// Every check reads the segment with its inline code spans blanked. Blanking replaces each span
	// with as many spaces, so an offset into `prose` addresses the same byte of `text`, and every
	// finding is echoed out of `text` so the writer reads what they typed. A backticked word is a quoted
	// literal, and a rule that names a coined word as a tell would otherwise report itself for naming it.
	prose := reInlineCode.ReplaceAllStringFunc(text, func(span string) string {
		return strings.Repeat(" ", len(span))
	})
	// The three sentence shapes read a body and stay out of the instruction profile: a rule file writes
	// about them, and writing about one is not writing in it.
	if s.profile == ProfileProse {
		for _, shape := range sentenceShapes {
			for _, pattern := range shape.patterns {
				at := pattern.FindStringIndex(prose)
				// `no` before a comparative is the ordinary word, as in "saying no more than itself".
				// The check reported its own documentation there.
				if at == nil || reComparativeTail.MatchString(prose[at[1]:]) {
					continue
				}
				add(shape.check, at[0], at[1])
				break
			}
		}
	}
	// A coined word is a codebase's invented vocabulary, so the check belongs where code and the text
	// about a change are — not over a rule file, which is prose about writing and uses the ordinary
	// English word a codebase may happen to have coined. A machine-level conf naming one project's
	// terms would otherwise report every repository's rule files for using English.
	coinedIn := prose
	if s.profile == ProfileInstruction {
		coinedIn = ""
	}
	for _, word := range s.coinedTerms() {
		// Group 1 is only the word, and the scan resumes at its end. A scan resuming past the boundary
		// characters the pattern matched would swallow the separator — `rung rung` reporting once.
		for from := 0; from < len(coinedIn); {
			at := coinedPattern(word).FindStringSubmatchIndex(coinedIn[from:])
			if at == nil {
				break
			}
			if !strings.EqualFold(coinedIn[from+at[2]:from+at[3]], "names no") ||
				!reNamedInACall.MatchString(coinedIn[from+at[3]:]) {
				add(checkCoined, from+at[2], from+at[3])
			}
			from += at[3]
		}
		if inIdentifier := coinedInIdentifier(word); inIdentifier != nil {
			for _, at := range inIdentifier.FindAllStringIndex(coinedIn, -1) {
				add(checkCoined, at[0], at[1])
			}
		}
	}

	// Bold is markdown. A rule file is markdown, so the check is the prose profile's.
	if s.profile != ProfileInstruction {
		for _, at := range reBold.FindAllStringIndex(prose, -1) {
			add(checkBold, at[0], at[1])
		}
	}
	for _, re := range []*regexp.Regexp{reRatherThan, reNeverNot, reInsteadOf, reDefinedAgainst} {
		for _, at := range re.FindAllStringIndex(prose, -1) {
			add(checkContrast, at[0], at[1])
		}
	}
	for _, at := range reAndNot.FindAllStringIndex(prose, -1) {
		if reNegated.MatchString(prose[:at[0]]) {
			continue
		}
		add(checkContrast, at[0], at[1])
	}
	for _, at := range reIntensifier.FindAllStringIndex(prose, -1) {
		add(checkIntensifier, at[0], at[1])
	}
	for _, at := range reEmphasis.FindAllStringIndex(prose, -1) {
		if rePointing.MatchString(prose[at[0]:at[1]]) {
			continue
		}
		add(checkIntensifier, at[0], at[1])
	}
	for _, span := range sentenceSpans(text) {
		read := prose[span[0]:span[1]]
		if reCounterfactual.MatchString(read) || reReadAloneAnd.MatchString(read) {
			add(checkCounterfactal, span[0], span[1])
		}
		if reParticipleOpen.MatchString(read) || opensWithGerund(read) {
			add(checkNoSubject, span[0], span[1])
		}
		if at := participialPhrase(read); at != nil && !reOpensWithVerb.MatchString(read) {
			add(checkNoSubject, span[0]+at[0], span[0]+at[1])
		}
		if at := rePositional.FindStringIndex(read); at != nil {
			add(checkPositional, span[0]+at[0], span[0]+at[1])
		}
		// The count reads `text`, because blanking an inline code span leaves spaces behind. A backticked
		// identifier is a word on the page, and the blanked form drops it.
		if len(strings.Fields(text[span[0]:span[1]])) > maxSentenceWords {
			add(checkLongSentence, span[0], span[1])
		}
		if len(reConnective.FindAllString(rePrepositionWhich.ReplaceAllString(read, " "), -1)) >= flaggedConnectivesPerSentence {
			add(checkClauseDepth, span[0], span[1])
		}
		if len(reNegation.FindAllString(read, -1)) >= flaggedNegationsPerSentence {
			add(checkDoubleNeg, span[0], span[1])
		}
		// A sentence carrying one semicolon joins two clauses, whatever the tail's length. Several
		// semicolons are a list, and there the tail tells a list item from a clause: `owner; entry;
		// book` is three fields where three sentences would each run longer.
		if all := reSemicolon.FindAllStringIndex(read, -1); len(all) > 0 {
			tail := len(strings.Fields(read[all[0][1]:]))
			if (len(all) == 1 && !s.inCell) || tail >= minClauseTailWords {
				add(checkSemicolon, span[0], span[1])
			}
		}
	}
	return found
}

// shortStems are the verbs `ing` is part of rather than an ending on: no subject went missing in
// "Bring the choice to the human" or "String the calls together". Four letters is where the stem stops
// being a word of its own — `Br`, `S`, `Cl`, `Sw` are not verbs, `Read`, `Count` and `Narrow` are.
var shortStems = map[string]bool{"spring": true, "string": true, "during": true, "noth": true,
	"someth": true, "anyth": true, "everyth": true, "morn": true, "even": true}

// opensWithGerund says the sentence starts on a gerund with its subject dropped.
func opensWithGerund(sentence string) bool {
	match := reGerundOpen.FindStringSubmatch(sentence)
	if match == nil {
		return false
	}
	stem := strings.ToLower(match[1])
	return len(stem) >= 4 && !shortStems[stem] && !shortStems[strings.ToLower(match[0][:len(match[1])+3])]
}

// readAllCapped reads to the cap and REFUSES at it. io.LimitReader alone returns a short read with a
// nil error, so a caller cannot tell a complete stream from a cut one, and the scan reports a clean
// result over what it never saw. Reading one byte past the cap is what makes the two distinguishable.
func readAllCapped(from io.Reader, cap int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(from, cap+1))
	if err != nil {
		return nil, errors.New("could not be read")
	}
	if int64(len(body)) > cap {
		return nil, fmt.Errorf("is larger than %d bytes, and a scan over part of it would report clean over the rest", cap)
	}
	return body, nil
}

// defaultCoined are the phrases a repository coins by accident, the house idiom of naming, where plain
// English says absent or unlisted. Each conf would otherwise have to restate them, and a
// tell the lane only reads for is a tell that reopens. This one reopened three times before anyone
// measured it.
var defaultCoined = []string{"has no name", "names no", "names nothing", "a name it does not hold"}

// reNamedInACall is a thing a request or a call names literally, and the idiom said of one is plain
// English. Run 25's writer turned a request that named no scheme into a passive to get past the check.
var reNamedInACall = regexp.MustCompile(`(?i)^\s+(?:scheme|type|key|parameter|mode|format|field|argument|header|option|version|currency|file|path)s?\b`)

// coinedTerms is a caller's own words followed by the built-in phrases. It resolves here because
// scanSegment is the only place a check runs. A construction site that merged them would leave every
// other construction of a scanner short of the list, and the run would still pass.
func (s scanner) coinedTerms() []string {
	return append(append([]string{}, s.coined...), defaultCoined...)
}

// coinedPattern matches the term and anything built off it — a coined noun also catches its plural,
// and a coined verb its `-s` and `-ing` forms, because a term is coined in every shape it takes.
//
// A term of several words needs no extra pattern. QuoteMeta leaves a space alone. A paragraph's lines
// are joined with one space before a check reads them, so a phrase broken across two lines is the same
// phrase. The stem suffix lands on the last word, which is the word that inflects.
func coinedPattern(word string) *regexp.Regexp {
	// The word is captured, and its boundaries are matched rather than asserted, because Go's `\b` is
	// an ASCII word boundary: a coined term opening on a letter outside ASCII has no boundary before
	// it and `\b` never matches, so the pattern compiles and then finds nothing. A check that is
	// silently inert is worse than one that refuses.
	return regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}_])(` + regexp.QuoteMeta(word) + `\p{L}*)(?:[^\p{L}\p{N}_]|$)`)
}

// coinedInIdentifier matches the word as a camelCase segment — `readSprocket`, `SprocketReader`,
// `readSoleSprocketDeclaring`. A coined term that became an identifier is the same word the reader
// does not share, and the rename is what carries it away. Written as its own pattern because Go's
// regexp has no lookbehind, so the segment's preceding lowercase run is matched rather than asserted.
//
// The first character is taken as a rune. Taken as a byte, a coined word opening on a multi-byte one
// leaves its trailing bytes orphaned, and regexp refuses a pattern holding invalid UTF-8.
func coinedInIdentifier(word string) *regexp.Regexp {
	if word == "" || strings.ContainsAny(word, " \t") {
		return nil
	}
	first, size := utf8.DecodeRuneInString(word)
	titled := string(unicode.ToUpper(first)) + word[size:]
	return regexp.MustCompile(`\b[A-Za-z]*[a-z]` + regexp.QuoteMeta(titled) + `[A-Za-z]*\b`)
}

// voice parses the flags and runs the scan over the paths that follow them.
func voice(out console, args []string, cwd string, git repo.Git, cfg Config) int {
	profile := ProfileProse
	counts := false
	// A PR body and a ticket each have a width, and `--kind` reads the prose as one of them.
	kind := ""
flags:
	for len(args) > 0 {
		switch {
		case args[0] == "--":
			args = args[1:]
			break flags
		case strings.HasPrefix(args[0], "--kind="):
			kind = strings.TrimPrefix(args[0], "--kind=")
			if !kinds[kind] {
				return out.refuseArguments(fmt.Errorf("no kind %q — the scan did NOT run. Kinds: %s %s",
					shell.CutBytesMarked(shell.Oneline(kind), 40), KindPRBody, KindTicket))
			}
		case strings.HasPrefix(args[0], "--profile="):
			named := Profile(strings.TrimPrefix(args[0], "--profile="))
			switch named {
			case ProfileProse, ProfileInstruction:
				profile = named
			default:
				return out.refuseArguments(fmt.Errorf("no profile %q — the scan did NOT run. Profiles: prose instruction",
					shell.CutBytesMarked(shell.Oneline(string(named)), 40)))
			}
		case args[0] == "--per-file":
			counts = true
		// A path that starts with `-` goes after `--`. Read as a path here, a mistyped flag would be
		// refused as a file that cannot be read.
		case strings.HasPrefix(args[0], "-") && args[0] != "-":
			return out.refuseArguments(fmt.Errorf("no option %q — the scan did NOT run",
				shell.CutBytesMarked(shell.Oneline(args[0]), 40)))
		default:
			break flags
		}
		args = args[1:]
	}
	if kind != "" && profile != ProfileProse {
		return out.refuseArguments(fmt.Errorf("--kind reads a PR body or a ticket, which the prose profile "+
			"reads, and not the %s profile — the scan did NOT run", profile))
	}
	if len(args) == 0 {
		return out.refuseArguments(fmt.Errorf("the %s profile needs a path, or `-` for stdin — the scan did NOT run", profile))
	}

	s := scanner{profile: profile, kind: kind}
	if kind != "" {
		root := cwd
		if top, err := git.TopLevel(cwd); err == nil && top != "" {
			root = top
		}
		s.template = templateLines(root, kind)
	}
	if counts {
		return reportCounts(out, s, profile, args, cwd, cfg)
	}
	found, err := s.scanPaths(args, cwd, cfg)
	if err != nil {
		return out.refuse(err)
	}
	return reportVoice(out, profile, found, len(args))
}

// scanPaths reads the named files, or stdin for `-`. The prose profile's caller usually holds the
// text rather than a path — a PR body being drafted — so stdin is the common form there.
func (s scanner) scanPaths(args []string, cwd string, cfg Config) ([]Finding, error) {
	read := func(file string, lines []string) []Finding {
		if s.kind == "" {
			return s.scanProse(file, lines)
		}
		authored := AuthoredLines(strings.Join(lines, "\n"), s.template)
		return append(s.scanProse(file, authored), KindFindings(file, lines, authored)...)
	}
	var found []Finding
	for _, arg := range args {
		if arg == "-" {
			body, err := readAllCapped(os.Stdin, maxStdinBytes)
			if err != nil {
				return nil, fmt.Errorf("stdin %v — exit 2, the scan did NOT run", err)
			}
			found = append(found, read("-", shell.SplitLines(string(body)))...)
			continue
		}
		path := arg
		if !strings.HasPrefix(path, "/") {
			path = shell.Join(cwd, path)
		}
		// Asked before the read, so the cap bounds what is loaded and not merely what is scanned.
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("cannot read %s — exit 2, the scan did NOT run",
				shell.CutBytesMarked(shell.Oneline(arg), maxPathBytes))
		}
		if info.Size() > cfg.MaxFileBytes {
			return nil, fmt.Errorf("%s is over DENSITY_MAX_FILE_BYTES — exit 2, the scan did NOT run",
				shell.CutBytesMarked(shell.Oneline(arg), maxPathBytes))
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("cannot read %s — exit 2, the scan did NOT run",
				shell.CutBytesMarked(shell.Oneline(arg), maxPathBytes))
		}
		found = append(found, read(arg, shell.SplitLines(string(body)))...)
	}
	return found, nil
}

// reportVoice prints the findings sorted, then a denominator on stderr. The sort makes two runs over one
// text print one report, so a diff of two runs is the change.
func reportVoice(out console, profile Profile, found []Finding, files int) int {
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].File != found[j].File {
			return found[i].File < found[j].File
		}
		if found[i].Line != found[j].Line {
			return found[i].Line < found[j].Line
		}
		return found[i].Check < found[j].Check
	})
	shown := found
	if len(shown) > maxFindings {
		shown = shown[:maxFindings]
	}
	for _, f := range shown {
		fmt.Fprintln(out.stdout, f.String())
	}
	if len(found) > len(shown) {
		fmt.Fprintf(out.stdout, "… and %d further finding(s), not shown\n", len(found)-len(shown))
	}

	byCheck := map[string]int{}
	for _, f := range found {
		byCheck[f.Check]++
	}
	var parts []string
	for _, name := range AllChecks {
		if byCheck[name] > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", name, byCheck[name]))
		}
	}
	out.note("%s profile: %d finding(s)%s over %d file(s).", profile, len(found), tally(parts), files)
	if len(found) == 0 {
		out.note("clean, which says the register was read and matched nothing — not that the text was not read.")
		return exitClean
	}
	out.note("each finding is an edit the lane makes, not a count to drive down. A finding that must stand is a defect in the check, which is fixed there.")
	return exitFound
}

// reportCounts prints one line per path: the finding count, a space, and the path as it was given. A
// path with no findings gets a line too. reportVoice shows at most maxFindings. A count is one line
// however many findings it counts, so that cap does not apply here.
func reportCounts(out console, s scanner, profile Profile, args []string, cwd string, cfg Config) int {
	total := 0
	for _, arg := range args {
		found, err := s.scanPaths([]string{arg}, cwd, cfg)
		if err != nil {
			// The counts already printed stay on stdout. The caller pairs them back against the paths it
			// asked for, and the last path with a line is where the run stopped.
			return out.refuse(err)
		}
		fmt.Fprintf(out.stdout, "%d %s\n", len(found), arg)
		total += len(found)
	}
	out.note("%s profile: %d finding(s) over %d file(s).", profile, total, len(args))
	if total == 0 {
		return exitClean
	}
	return exitFound
}

func tally(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return " — " + strings.Join(parts, ", ")
}

// The voice half of the detector: which sentences in a comment, a body or a rule file are written in
// the house register rather than in plain prose. It counts nothing. Each check names a shape a reader
// stumbles on, and reports the text that matched so the writer can see what to change.
//
// The bar (bar.go) says how MANY comment lines a change set may carry. This says whether the lines it
// carries can be read. The two are separate because a set can be driven to any rate by compressing
// every sentence, which meets the number and is the defect.
//
// Deterministic, so two runs over one text print one report and a finding can be allowlisted by its
// exact matched text.
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

	"configs/ai/tools/diffscan"
	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
)

// A block over this many text lines is a wall. A file header gets more, because a header carries the
// call order and error modes a published surface owes. Text lines, not raw lines: a `/**` opening and
// a `*/` closing carry no words, and counting them would make every four-sentence block a finding.
const (
	voiceLongBlock  = 4
	voiceLongHeader = 8

	// writing.md puts one idea in a sentence and keeps it under about 25 words. The check fires at 30
	// so a sentence at the rule's own edge is not a finding, and only a sentence carrying a second
	// idea is.
	voiceLongSentence = 30

	// Two subordinating connectives in one sentence. The note pattern spends one on `so`, which puts a
	// conforming note at one. The check fires above that.
	voiceClauseDepth = 2

	// Two negations in one sentence.
	voiceNegations = 2

	// Words after a semicolon before its tail reads as a clause, where the sentence carries several and
	// is therefore a list. One semicolon joins two clauses whatever its tail is: the tree holds eleven
	// with a tail of one to three words, and every one of them joins two sentences.
	voiceClauseTail = 4
)

// Findings and echoed text are bounded the same way the default mode's are: under kk-pr this text
// comes off a branch somebody else wrote.
const (
	maxFindings   = 500
	maxMatchBytes = 120
	// What a whole diff or body on stdin may be. Held apart from DENSITY_MAX_FILE_BYTES, which is a
	// per-FILE number: a diff is many files, and `gh pr diff` over an ordinary change set is larger
	// than any file in it. Read to a cap and TRUNCATED, a scan reports clean over the half it never
	// saw — and git orders a diff by path, so padding an early file pushes a hostile one past the cut.
	maxStdinBytes = 64 * 1024 * 1024
	// The highest line number a hunk header may claim before the line is dropped. scanAdded sizes a
	// slice by it, and on the `-` arm the header comes off a diff somebody else wrote: `@@ +10000000`
	// for one added line measured 166 MB resident, and a 2^31 line number asks for tens of gigabytes.
	// A source file reaching this many lines has other problems.
	maxDiffLine = 1 << 20
)

// Profile is which text the scan reads and which checks apply to it.
type Profile string

const (
	// ProfileComment reads the comment lines a diff added to source files.
	ProfileComment Profile = "comment"
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

// The checks, each named so an allowlist entry can name it.
const (
	checkBold          = "bold"
	checkContrast      = "contrast"
	checkCounterfactal = "counterfactual-opener"
	checkNoSubject     = "no-subject"
	checkIntensifier   = "intensifier"
	checkPositional    = "positional"
	checkLongBlock     = "long-block"
	checkCoined        = "coined"
	checkCoinedIdent   = "coined-identifier"
	checkCounterfact   = "counterfactual-consequence"
	checkAnthropo      = "anthropomorphism"
	checkElidedVerb    = "elided-verb"
	checkBareIdent     = "bare-identifier"
	checkLongSentence  = "long-sentence"
	checkClauseDepth   = "clause-depth"
	checkDoubleNeg     = "double-negative"
	checkSemicolon     = "semicolon"
	checkReasonAway    = "reason-by-link"
	checkAloneForOnly  = "alone-for-only"
)

// AllChecks is every check name, for the allowlist parser to refuse an entry naming none of them.
var AllChecks = []string{checkBold, checkContrast, checkCounterfactal, checkNoSubject,
	checkIntensifier, checkPositional, checkLongBlock, checkCoined, checkCoinedIdent,
	checkCounterfact, checkAnthropo, checkElidedVerb, checkBareIdent,
	checkLongSentence, checkClauseDepth, checkDoubleNeg, checkSemicolon, checkReasonAway, checkAloneForOnly}

// reAlone is the word itself. What it follows decides whether it is the exclusivity word or the
// ordinary one, and aloneIsIdiom reads that.
var reAlone = regexp.MustCompile(`(?i)\balone\b`)

// aloneLookBack is how many words back the governing verb is looked for: `leave the actor alone`.
const aloneLookBack = 3

// aloneStandsAfter is the pronouns `alone` follows in its ordinary sense, and aloneStandsUnder the
// verbs that take it: `leave it alone`, `the header stands alone`.
var aloneStandsAfter = map[string]bool{"it": true, "them": true, "him": true, "her": true, "me": true,
	"us": true, "you": true, "i": true, "he": true, "she": true, "we": true, "they": true}

var aloneStandsUnder = map[string]bool{"leave": true, "leaves": true, "left": true, "leaving": true,
	"let": true, "lets": true, "letting": true, "stand": true, "stands": true, "standing": true,
	"stood": true, "go": true, "goes": true, "went": true, "gone": true}

// aloneIsIdiom says this `alone` is the ordinary word and not the exclusivity one. `before` is the
// text up to the word.
func aloneIsIdiom(before string) bool {
	words := strings.Fields(before)
	for at := len(words) - 1; at >= 0 && at > len(words)-1-aloneLookBack; at-- {
		word := strings.ToLower(strings.Trim(words[at], "`*_,.;:()\"'"))
		if aloneStandsUnder[word] {
			return true
		}
		if at == len(words)-1 && aloneStandsAfter[word] {
			return true
		}
	}
	return false
}

// reReasonAway is a note handing its reason to another note: `for the reason {@link X} gives`,
// `see <X> for why`, `as {@link X} explains`. The reason is written where it is read, so a block
// carrying one of these shapes is a block whose reason has to move to it.
var reReasonAway = regexp.MustCompile(`(?i)\b(for the reason|see|as)\s+(\{@link\s+[^}\n]{1,80}\}|` +
	"`[^`\n]{1,80}`" + `)\s+(gives|states|explains|for why|for the reason)\b`)

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

// stripMarker takes the comment syntax off a line, leaving the words. A `*` continuation loses its
// star, so a sentence that starts a continuation line is read as starting a sentence.
func stripMarker(raw string) string {
	line := strings.TrimLeft(raw, shell.SpaceBytes)
	for _, marker := range []string{"/**", "/*", "*/", "//", "#"} {
		if strings.HasPrefix(line, marker) {
			return strings.TrimSpace(line[len(marker):])
		}
	}
	// A lone `*` is a continuation marker only where a space or the end of the line follows, which is
	// the rule isComment applies. Stripped unconditionally, the `**Bold**` opening a starless line
	// inside a `/* */` block becomes `*Bold**`: the bold check cannot fire and the echoed text is
	// corrupt. The two have to agree, or a line counts as a comment and is then read as something else.
	if rest, marked := continuation(line); marked {
		return strings.TrimSpace(rest)
	}
	return line
}

// continuation reads a `*` or `*/` continuation marker, returning what follows it.
func continuation(line string) (string, bool) {
	rest := ""
	switch {
	case strings.HasPrefix(line, "*/"):
		rest = line[2:]
	case strings.HasPrefix(line, "*"):
		rest = line[1:]
	default:
		return line, false
	}
	if rest == "" || rest[0] == ' ' || rest[0] == '\t' {
		return rest, true
	}
	return line, false
}

// proseOf is stripMarker with the doc tags dropped, which is what a check reads. A `@param` line
// joined into the segment would put the signature in the middle of a sentence.
func proseOf(raw string) string {
	stripped := stripMarker(raw)
	if !isProseLine(stripped) {
		return ""
	}
	return stripped
}

// block is one run of adjacent comment lines, by the 1-based lines it spans.
type block struct {
	start int
	end   int
}

// present says a line is one this scan actually holds. Over a whole file every line is; over the
// sparse file scanChange reconstructs from a diff, only the added ones are, and the rest are gaps
// standing in for text nobody handed us.
//
// The distinction is load-bearing twice over, and both ways it goes wrong quietly. A gap read as a
// blank line makes every block in a diff look like a file header, because nothing but blanks stands
// above it — and a header is allowed twice a block's length, so blocks of five to eight lines pass
// unreported in the one mode that matters. A gap read as more of a `/*` run makes one reworded `/**`
// swallow every later comment in the file into a single block, because the `*/` that would have
// closed it is a line the diff never added.
type present func(int) bool

func wholeFile(int) bool { return true }

func onlyAdded(within map[int]bool) present {
	if within == nil {
		return wholeFile
	}
	return func(at int) bool { return within[at] }
}

// commentBlocks groups adjacent comment lines. Mirrors reader-judge's own grouping: inside a `/*` run
// every line belongs to the block until one carries `*/`, whatever it starts with — except that a gap
// ends the run here, since the line that would have closed it may be one the diff did not touch.
func commentBlocks(lines []string) []block { return commentBlocksIn(lines, wholeFile) }

func commentBlocksIn(lines []string, held present) []block {
	var found []block
	inBlock, inStar := false, false
	for i, raw := range lines {
		at := i + 1
		line := strings.TrimLeft(raw, shell.SpaceBytes)
		switch {
		case !held(at):
			inBlock, inStar = false, false
		case isShebang(at, line):
			// An interpreter directive, not a comment. Counted as one it joins the file header below
			// it and spends a line of that header's allowance, so a script whose header is exactly at
			// the limit reports long for saying which interpreter runs it.
			inBlock, inStar = false, false
		case inStar:
			found[len(found)-1].end = at
			if strings.Contains(line, "*/") {
				inStar, inBlock = false, false
			}
		case !isComment(line):
			inBlock = false
		case inBlock:
			found[len(found)-1].end = at
			inStar = opensStar(line)
		default:
			found = append(found, block{start: at, end: at})
			inBlock = true
			inStar = opensStar(line)
		}
	}
	return found
}

// isShebang says this is the interpreter directive a script opens with. Only on the first line: `#!`
// anywhere else is an ordinary comment that happens to start with a bang.
func isShebang(at int, line string) bool {
	return at == 1 && strings.HasPrefix(line, "#!")
}

func opensStar(line string) bool {
	return strings.HasPrefix(line, "/*") && !strings.Contains(line[2:], "*/")
}

// isFileHeader says this block opens the file: nothing but blank lines stands above it. A block two
// lines down from an import is not a header, and giving it the header's allowance would let every
// block in a file claim eight lines by sitting near the top.
//
// A line above that this scan does not hold refuses the claim rather than passing it. Over a diff we
// cannot see what stands there, and the header allowance is twice a block's, so a guess in the
// generous direction is the whole check going quiet.
func (b block) isFileHeader(lines []string, held present) bool {
	for at := 1; at < b.start; at++ {
		line := strings.TrimSpace(lines[at-1])
		if !held(at) {
			return false
		}
		// A shell script's interpreter directive stands above its header and does not displace it.
		if line == "" || isShebang(at, line) {
			continue
		}
		return false
	}
	return true
}

// toolInputIn returns the first line of this block that a check reads, or "" where none does. A
// comment is prose to a reader and input to a check at the same time, and this block is both.
//
// The scan covers the block opening the file, since a header scan reads no further. comment-strip
// keeps these same lines, so both tools agree on the text carrying a second reader.
func (b block) toolInputIn(lines []string) string {
	for at := b.start; at <= b.end && at <= len(lines); at++ {
		text := strings.TrimSpace(stripMarker(lines[at-1]))
		if toolInputLine.MatchString(text) || namedSuite.MatchString(text) {
			return shell.CutBytesMarked(shell.Oneline(text), 60)
		}
	}
	return ""
}

// The spellings a check in ai/tools/ reads out of a file's opening comment block: a usage line, a
// test declaration, and the name of the suite covering the script.
var (
	toolInputLine = regexp.MustCompile(`^(usage:|untested:)`)
	namedSuite    = regexp.MustCompile(`[A-Za-z0-9_.-]+-test\.sh`)
)

// textLines is how long a block reads: the lines carrying prose. A `/**` and its closing `*/` carry no
// words, and a doc tag line is a table of the signature rather than a sentence, so neither spends the
// allowance. Counting tag lines made a six-parameter function's `@param` list a long block, which is
// the one shape the rule has no quarrel with.
func (b block) textLines(lines []string) int {
	counted := 0
	for at := b.start; at <= b.end && at <= len(lines); at++ {
		if isProseLine(stripMarker(lines[at-1])) {
			counted++
		}
	}
	return counted
}

// isProseLine says a stripped comment line carries sentences. A line opening on a doc tag does not:
// `@param`, `@returns`, `@throws`, `@example` and the rest are the signature written out, and every
// language's doc tool spells them this way.
func isProseLine(stripped string) bool {
	return stripped != "" && !strings.HasPrefix(stripped, "@")
}

// scanner holds one run's settings so every profile reaches the same checks.
type scanner struct {
	profile Profile
	coined  []string
	// domain holds the compounds this codebase's readers know, from the conf. The coined-identifier
	// check passes over these and fires on every other compound the code spells.
	domain  []string
	allowed allowlist
	// suppressed counts what the allowlist dropped. A finding answered by an entry is still a finding
	// the text carried, and a report that said nothing about it would read as text that matched nothing.
	suppressed *int
	// record says the text opens with the note's record, which RecordFindings reads against the block
	// and the source under it. The register checks read the prose either way.
	record bool
	// inCell says the segment is one cell of a table row. A cell is a list by construction. A semicolon
	// in one separates two fields, and the same semicolon in prose joins two clauses.
	inCell bool
	// notice writes a line to the run's stderr. A file this scan declines to read has to say so, or
	// the report claims a denominator it never covered. Nil in a caller that only wants the findings.
	notice func(string)
}

func (s scanner) announce(line string) {
	if s.notice != nil {
		s.notice(line)
	}
}

// segment is one run of text a check reads whole, with the line every byte of it came from. A comment
// block and a markdown paragraph are both written across several lines and read as one, so a check
// that ran per line would never see a sentence that wraps — and a wrapped sentence is the ordinary
// case in a block the width of a screen.
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

// scanSource reads a source file's comment blocks. `within` is the set of lines this scan holds, nil
// over a whole file; it scopes the scan by deciding what a block IS, so a block never reaches a check
// carrying a line the diff did not add and nothing needs filtering afterwards.
//
// `whole` is the file as the change leaves it, and it is nil where the run cannot reach the file.
// Two checks exempt a word the code beside a block already spells, and both read `whole`. Over a
// diff, `lines` carries the added lines alone, so the declaration under an untouched block is a gap
// and the exemption goes with it. The scan's scope stays with `lines`.
func (s scanner) scanSource(file string, lines []string, within map[int]bool, whole []string) []Finding {
	var found []Finding
	held := onlyAdded(within)
	if whole == nil {
		whole = lines
	}
	identifiers := identifierWordsOf(whole)
	for _, b := range commentBlocksIn(lines, held) {
		found = append(found, s.coinedIdentifiers(file, b, lines, identifiers)...)
		found = append(found, s.bareIdentifiers(file, b, lines, declaredAt(whole, b))...)
		limit := voiceLongBlock
		header := b.isFileHeader(lines, held)
		if header {
			limit = voiceLongHeader
		}
		if n := b.textLines(lines); n > limit {
			text := fmt.Sprintf("%d text lines, over %d", n, limit)
			// A split is the remedy this finding usually gets. Here it would put a blank line through
			// the block, a header scan ends at that blank, and the lines under it reach no reader in
			// silence. Two scripts lost their binary this way, so the finding rules the split out.
			if read := b.toolInputIn(lines); header && read != "" {
				text += fmt.Sprintf(" — a check reads %q here, so shorten this block and keep it whole", read)
			}
			found = append(found, Finding{File: file, Line: b.start, Check: checkLongBlock, Text: text})
		}
		found = append(found, s.scanSegment(file, join(lines, b.start, b.end, proseOf))...)
	}
	return s.filter(found)
}

// filter drops what the allowlist answers and counts it. A finding an entry answers is still a finding
// the text carried, and a report saying nothing about it reads as text that matched nothing.
func (s scanner) filter(found []Finding) []Finding {
	kept := s.allowed.filter(found)
	if s.suppressed != nil {
		*s.suppressed += len(found) - len(kept)
	}
	return kept
}

// scanProse reads a whole text file, a paragraph at a time. The instruction profile skips what a rule
// file uses as structure: its frontmatter, its fenced code, and its headings.
//
// A paragraph is a run of lines with no blank line in it. The flavor asks for one paragraph to a line
// in a field you type into, and repository files wrap, so both shapes arrive here and both are read
// the same way.
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
		if paragraph == 0 {
			paragraph = at
		}
	}
	flush(len(lines))
	return s.filter(found)
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

// scanSegment is every check over one segment, and the only place a check runs. One function, so the
// three profiles cannot drift into reading the same sentence differently.
// Three shapes a reviewer read as confusing, each counted on sixty files of reviewed code before it
// became a check. comment-census holds the counts and the samples behind them.
//
// They read comments alone, because the counts were taken over comment blocks. A rule file writes
// about these shapes, and writing about one is not writing in it.
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

	// `no` before a comparative is the ordinary word, as in "saying no more than itself".
	reComparativeTail = regexp.MustCompile(`(?i)^\s+(more|longer|further|fewer|less|worse|better)\b`)
)

// SentenceShapes are the three comment-only checks, exported so comment-census counts the patterns
// this scan fires on. One definition, two readers.
func SentenceShapes() map[string][]*regexp.Regexp {
	return map[string][]*regexp.Regexp{
		checkCounterfact: {reSoWould},
		checkAnthropo:    reAnthropomorphic,
		checkElidedVerb:  {reElidedVerb},
	}
}

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
	// finding is echoed out of `text` so the writer reads what they typed.
	//
	// The comment profile is the exception, and only for `coined`: a coined word inside backticks
	// there is an identifier built on the term, which is the rename the refactor lane owes. In a rule
	// file or a body there are no identifiers, so a backticked word is a quoted literal — and a rule
	// that names a coined word as a tell would otherwise report itself for naming it.
	prose := reInlineCode.ReplaceAllStringFunc(text, func(span string) string {
		return strings.Repeat(" ", len(span))
	})
	// The three sentence shapes read a comment and a body, since a PR body carries the same shapes a
	// comment does. They stay out of the instruction profile: a rule file writes about them, and
	// writing about one is not writing in it.
	if s.profile == ProfileComment || s.profile == ProfileProse {
		for _, name := range []string{checkCounterfact, checkAnthropo, checkElidedVerb} {
			for _, pattern := range SentenceShapes()[name] {
				at := pattern.FindStringIndex(prose)
				// `no` before a comparative is the ordinary word, as in "saying no more than itself".
				// The check reported its own documentation there.
				if at == nil || reComparativeTail.MatchString(prose[at[1]:]) {
					continue
				}
				add(name, at[0], at[1])
				break
			}
		}
	}
	// Two shapes a reviewer sent back on 2026-09-22, both of them a comment's own sentence and neither
	// of them a register tell. `alone` after a noun is `only` doing its work in a word a reader in a
	// second language meets as "by itself" first. A reason given as a pointer at another comment leaves
	// the reason at neither block, and the link form is what makes that one readable.
	if s.profile == ProfileComment {
		for _, at := range reAlone.FindAllStringIndex(text, -1) {
			if aloneIsIdiom(text[:at[0]]) {
				continue
			}
			add(checkAloneForOnly, at[0], at[1])
		}
		for from := 0; from < len(text); {
			at := reReasonAway.FindStringIndex(text[from:])
			if at == nil {
				break
			}
			add(checkReasonAway, from+at[0], from+at[1])
			from += at[1]
		}
	}
	// A coined word is a codebase's invented vocabulary, so the check belongs where code and the text
	// about a change are — not over a rule file, which is prose about writing and uses the ordinary
	// English word a codebase may happen to have coined. A machine-level conf naming one project's
	// terms would otherwise report every repository's rule files for using English.
	coinedIn := prose
	if s.profile == ProfileComment {
		coinedIn = text
	}
	if s.profile == ProfileInstruction {
		coinedIn = ""
	}
	for _, word := range s.coinedTerms() {
		// Group 1 is the word itself; the pattern matches the characters either side of it so the
		// boundary holds outside ASCII, and those are not part of what a reader is shown.
		// Advanced to the end of the WORD rather than the end of the match. The pattern matches the
		// characters either side of the word so the boundary holds outside ASCII, and a scan that
		// resumed past them would swallow the separator — `rung rung` reporting once.
		for from := 0; from < len(coinedIn); {
			at := coinedPattern(word).FindStringSubmatchIndex(coinedIn[from:])
			if at == nil {
				break
			}
			add(checkCoined, from+at[2], from+at[3])
			from += at[3]
		}
		if inIdentifier := coinedInIdentifier(word); inIdentifier != nil {
			for _, at := range inIdentifier.FindAllStringIndex(coinedIn, -1) {
				add(checkCoined, at[0], at[1])
			}
		}
	}

	// Bold is markdown. A rule file is markdown, so the check is the comment and prose profiles'.
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
		if len(strings.Fields(text[span[0]:span[1]])) > voiceLongSentence {
			add(checkLongSentence, span[0], span[1])
		}
		if len(reConnective.FindAllString(read, -1)) >= voiceClauseDepth {
			add(checkClauseDepth, span[0], span[1])
		}
		if len(reNegation.FindAllString(read, -1)) >= voiceNegations {
			add(checkDoubleNeg, span[0], span[1])
		}
		// A sentence carrying one semicolon joins two clauses, whatever the tail's length. Several
		// semicolons are a list, and there the tail tells a list item from a clause: `owner; entry;
		// book` is three fields where three sentences would each run longer.
		if all := reSemicolon.FindAllStringIndex(read, -1); len(all) > 0 {
			tail := len(strings.Fields(read[all[0][1]:]))
			if (len(all) == 1 && !s.inCell) || tail >= voiceClauseTail {
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

// coinedTerms is a conf's own words followed by the built-in phrases. It resolves here because
// scanSegment is the only place a check runs. A construction site that merged them would leave every
// other construction of a scanner short of the list, and the run would still pass.
func (s scanner) coinedTerms() []string {
	return append(append([]string{}, s.coined...), defaultCoined...)
}

// coinedPattern matches the term and anything built off it — a coined noun also catches its plural,
// and a coined verb its `-s` and `-ing` forms, because a term is coined in every shape it takes.
//
// A term of several words needs no extra pattern. QuoteMeta leaves a space alone. A block's lines are
// joined with one space before a check reads them, so a phrase broken across two comment lines is the
// same phrase. The stem suffix lands on the last word, which is the word that inflects.
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

// ScanFile is the whole scan over one file's content, for a caller holding the bytes already. The
// suite reads the fixture through it, so what a test measures is what a run reports.
func ScanFile(profile Profile, coined []string, allowed allowlist, file, content string) []Finding {
	s := scanner{profile: profile, coined: coined, allowed: allowed}
	lines := shell.SplitLines(content)
	if profile == ProfileComment {
		return s.scanSource(file, lines, nil, lines)
	}
	return s.scanProse(file, lines)
}

// voice runs the register check, which is what bare arguments select.
func voice(out console, args []string, cwd string, git repo.Git, cfg Config) int {
	profile := ProfileComment
	counts := false
	// The writer checks one block before it writes it, and a block is source where a revision range is
	// a diff. The comment profile reads stdin as a diff, so a block piped to it holds no hunk and the
	// run reports an empty scan. Run 7 put 17 bare identifiers over 8 files past the gate that way.
	source := false
	// The writer fills the note's record before it writes the block, and `--record` says the piped text
	// opens with that record. Four blocks a reviewer sent back on 2026-09-22 each stated a fact and
	// stopped, and the register checks passed every one: a block with nothing wrong with its prose.
	record := false
flags:
	for len(args) > 0 {
		switch {
		case args[0] == "--source":
			source = true
		case args[0] == "--record":
			record = true
		case strings.HasPrefix(args[0], "--profile="):
			named := Profile(strings.TrimPrefix(args[0], "--profile="))
			switch named {
			case ProfileComment, ProfileProse, ProfileInstruction:
				profile = named
			default:
				return out.refuseArguments(fmt.Errorf("no profile %q — the scan did NOT run. Profiles: comment prose instruction",
					shell.CutBytesMarked(shell.Oneline(string(named)), 40)))
			}
		case args[0] == "--per-file":
			counts = true
		default:
			break flags
		}
		args = args[1:]
	}
	if record && !source {
		return out.refuseArguments(errors.New("--record reads a record against the block and the source " +
			"under it, which is what --source pipes — the scan did NOT run"))
	}
	if source && profile != ProfileComment {
		return out.refuseArguments(fmt.Errorf("--source reads a block with the comment profile's checks, "+
			"and the %s profile has no blocks — the scan did NOT run", profile))
	}
	if counts && profile == ProfileComment {
		return out.refuseArguments(errors.New("--per-file counts the findings in each path it is given, " +
			"and the comment profile is handed a diff — the scan did NOT run"))
	}

	// The arguments are read before anything else, and refused with the grammar. The scan would instead
	// hand the caller a git failure, which is silent about what this tool takes.
	if profile == ProfileComment && !source && len(args) > 0 && args[0] != "-" {
		if err := diffscan.RefuseNonRevisions(git, args, cwd); err != nil {
			return out.refuseArguments(err)
		}
	}

	coined, domain, allowed, conf, err := voiceConfig(cwd)
	if err != nil {
		return out.refuse(err)
	}
	// A configuration that took effect says so, every run it changes. Silent, a conf a repository
	// ships can allow every check and the run still reports `0 finding(s)` and `clean, which says the
	// register was read` — a clean voice pass over text nothing read.
	if conf != "" {
		out.note("reading %s: %d coined word(s), %d domain word(s), %d allowlist entry(ies)",
			shell.CutBytesMarked(shell.Oneline(conf), maxPathBytes), len(coined), len(domain), len(allowed))
	}
	suppressed := 0
	s := scanner{profile: profile, coined: coined, domain: domain, allowed: allowed, suppressed: &suppressed,
		record: record, notice: func(line string) { out.note("%s", line) }}
	over := scanned{conf: conf}
	if counts {
		return reportCounts(out, s, profile, args, cwd, cfg, &over)
	}

	var found []Finding
	if profile == ProfileComment && !source {
		found, err = s.scanChange(args, cwd, git, cfg, &over)
	} else {
		found, err = s.scanPaths(args, cwd, cfg, &over, source)
	}
	if err != nil {
		return out.refuse(err)
	}
	over.suppressed = suppressed
	return reportVoice(out, profile, found, over)
}

// scanChange reads the diff — git's, or one on stdin for a branch this checkout does not hold, which
// is how the negative control runs over `gh pr diff`. Blocks are runs of ADDED comment lines, so the
// scan needs no working tree: a block split by a line the diff did not touch is two blocks, which is
// what a reviewer reading the diff sees too.
func (s scanner) scanChange(args []string, cwd string, git repo.Git, cfg Config, over *scanned) ([]Finding, error) {
	fromStdin := len(args) > 0 && args[0] == "-"
	var diff []byte
	var err error
	if fromStdin {
		if diff, err = readAllCapped(os.Stdin, maxStdinBytes); err != nil {
			return nil, fmt.Errorf("the diff on stdin %v — exit 2, the scan did NOT run", err)
		}
	} else {
		if err = diffscan.RefuseNonRevisions(git, args, cwd); err != nil {
			return nil, err
		}
		if diff, err = diffscan.Diff(git, cwd, args); err != nil {
			return nil, err
		}
	}

	added := newAddedLines()
	if err := s.readDiff(added, diff); err != nil {
		return nil, err
	}

	// The untracked half runs only with no revisions and no diff on stdin, the way the default mode's
	// does: with revisions the caller named two commits, and a file in neither of them is not part of
	// what they asked about. A new file is the commonest place a new comment lands, so a voice scan
	// that skipped it would report clean over the change most worth reading.
	named, _ := diffscan.RevisionsNamed(args)
	if !fromStdin && len(named) == 0 {
		if err := s.readUntracked(added, cwd, git, cfg); err != nil {
			return nil, err
		}
	}
	over.files = len(added.order)
	over.declined = len(added.declined)
	// A diff on stdin names a branch this checkout may lack, so the file is out of reach here and the
	// declaration exemption stays off. The diff carries context lines that usually hold the
	// declaration, and reading those would close the gap. It needs diffscan to offer them.
	var whole map[string][]string
	if !fromStdin {
		whole = s.endSide(args, cwd, git, cfg, added.order)
	}
	return s.scanAdded(added, whole), nil
}

// addedLines is what a diff or an untracked walk contributed, per file, in the order the files
// arrived. Held apart from the scan so the two arms fill one structure and the scan reads it once.
type addedLines struct {
	byFile   map[string][]addedLine
	order    []string
	declined map[string]bool
}

type addedLine struct {
	at   int
	text string
}

func newAddedLines() *addedLines {
	return &addedLines{byFile: map[string][]addedLine{}, declined: map[string]bool{}}
}

// declineOnce records a file this scan will not read and says so, the first time only. A diff carries
// many lines of one file, and a notice per line would bury the report it belongs to.
func (a *addedLines) declineOnce(file string, say func()) {
	if a.declined[file] {
		return
	}
	a.declined[file] = true
	say()
}

func (a *addedLines) take(file string, at int, text string) {
	if _, seen := a.byFile[file]; !seen {
		a.order = append(a.order, file)
	}
	a.byFile[file] = append(a.byFile[file], addedLine{at: at, text: text})
}

// skip says whether this file is one the scan reads at all. A line number past the cap is dropped on
// its own: scanAdded sizes a slice by the highest one, and on the `-` arm that number comes off a diff
// somebody else wrote.
func (s scanner) skip(a *addedLines, line diffscan.AddedLine, result *diffscan.Result) bool {
	if notThisRepositorysSource(line.File) {
		return true
	}
	// Both ends. The ceiling bounds what scanAdded allocates; the floor catches a hunk header whose
	// line number overflowed the counter that walks it, which arrives negative and would otherwise
	// pass the ceiling and index a zero-length slice.
	//
	// A line refused here is ANNOUNCED and counted, never dropped quietly. Dropped, one crafted hunk
	// header takes a file out of the scan and the run still closes on "clean, which says the register
	// was read" — the tool asserting it read what it discarded.
	if line.Line < 1 || line.Line > maxDiffLine {
		a.declineOnce(line.File, func() {
			s.announce(fmt.Sprintf("skipping '%s' — its diff claims line %d, which is outside the range this scan reads; it was NOT scanned.",
				shell.CutBytesMarked(shell.Oneline(line.File), maxPathBytes), line.Line))
		})
		return true
	}
	// This scan echoes file CONTENT — up to 120 bytes of it per finding — so it takes the guard
	// dup-literals takes and not the one the default density mode takes: that one reports paths and
	// counts, and a sentence out of a `.env` printed here reaches the orchestrator's transcript and
	// any body drafted from it. The untracked arm gets the same guard from Options.
	if !diffscan.SecretNamed(line.File) {
		return false
	}
	a.declineOnce(line.File, func() {
		result.SkippedUnread++
		s.announce(diffscan.SecretSkipNotice(line.File))
	})
	return true
}

func (s scanner) readDiff(a *addedLines, diff []byte) error {
	var result diffscan.Result
	err := result.WalkDiff(diff, func(line diffscan.AddedLine) {
		if !s.skip(a, line, &result) {
			a.take(line.File, line.Line, line.Text)
		}
	})
	if err != nil {
		return fmt.Errorf("the diff could not be read to the end (%v) — exit 2, the scan did NOT run over all of it. Not a clean result.", err)
	}
	return nil
}

func (s scanner) readUntracked(a *addedLines, cwd string, git repo.Git, cfg Config) error {
	var result diffscan.Result
	options := diffscan.Options{MaxFileBytes: cfg.MaxFileBytes, SkipSecretNamed: true, Announce: s.announce}
	err := result.WalkUntracked(git, cwd, options, func(line diffscan.AddedLine) {
		if !s.skip(a, line, &result) {
			a.take(line.File, line.Line, line.Text)
		}
	})
	if err != nil {
		return errors.New("could not list untracked files — exit 2, the scan did NOT run over them.")
	}
	return nil
}

// scanAdded reads each file as a sparse one: the added lines at their own numbers, and a gap standing
// in for every line the diff did not carry. `within` is what tells a gap from a blank line the diff
// really added, which decides where a block ends and whether one is a file header.
func (s scanner) scanAdded(a *addedLines, whole map[string][]string) []Finding {
	var found []Finding
	for _, file := range a.order {
		highest := 0
		within := map[int]bool{}
		for _, line := range a.byFile[file] {
			if line.at > highest {
				highest = line.at
			}
			within[line.at] = true
		}
		lines := make([]string, highest)
		for _, line := range a.byFile[file] {
			lines[line.at-1] = line.text
		}
		found = append(found, s.scanSource(file, lines, withoutSharedRegions(lines, within), whole[file])...)
	}
	return found
}

// endSide is each changed file as the change leaves it, for the checks that ask what the code beside
// a block spells. `git diff` has three right-hand sides. No revisions and one revision both end at
// the working tree. A range ends at the revision it names. A file this run cannot reach is left out
// and falls back to the diff's own lines.
func (s scanner) endSide(args []string, cwd string, git repo.Git, cfg Config, files []string) map[string][]string {
	top, err := git.TopLevel(cwd)
	if err != nil || len(files) == 0 {
		return nil
	}
	named, _ := diffscan.RevisionsNamed(args)
	rev := rightEnd(named)
	whole := map[string][]string{}
	if rev == "" {
		for _, file := range files {
			body, err := os.ReadFile(shell.Join(top, file))
			if err != nil || int64(len(body)) > cfg.MaxFileBytes {
				continue
			}
			whole[file] = shell.SplitLines(string(body))
		}
		return whole
	}
	// A revision this checkout lacks, or a read that fails, leaves the map short. The exemption falls
	// silent for those files, the way every run behaved before this existed.
	_ = git.ContentsAt(top, rev, files, cfg.MaxFileBytes, func(path string, content []byte) {
		whole[path] = shell.SplitLines(string(content))
	})
	return whole
}

// rightEnd is the revision a diff's right-hand side names, or "" where that side is the working tree.
// `a..b` and `a...b` both end at b, and `a..` ends at HEAD the way git reads it.
func rightEnd(named []string) string {
	if len(named) > 1 {
		return named[len(named)-1]
	}
	if len(named) == 0 {
		return ""
	}
	for _, separator := range []string{"...", ".."} {
		if _, right, found := strings.Cut(named[0], separator); found {
			if right == "" {
				return "HEAD"
			}
			return right
		}
	}
	// `git diff <rev>` compares that revision against the working tree.
	return ""
}

// A region a repository holds byte-identical across several files. The two markers name it, and the
// wiring check's shared-region scan enforces it.
const (
	sharedRegionOpen  = "# --- shared:"
	sharedRegionClose = "# --- end shared:"
)

// withoutSharedRegions drops the lines inside a shared region. A finding there asks for an edit the
// shared-region scan forbids, since the same bytes sit in every file carrying the region. The text is
// one text however many copies exist, so a new file carrying a copy adds no prose to read.
func withoutSharedRegions(lines []string, within map[int]bool) map[int]bool {
	kept := map[int]bool{}
	inside := false
	for at := 1; at <= len(lines); at++ {
		text := strings.TrimSpace(lines[at-1])
		if strings.HasPrefix(text, sharedRegionClose) {
			inside = false
			continue
		}
		if strings.HasPrefix(text, sharedRegionOpen) {
			inside = true
		}
		if !inside && within[at] {
			kept[at] = true
		}
	}
	return kept
}

// scanDiff is the diff half on its own, for a caller holding the bytes.
func (s scanner) scanDiff(diff []byte) ([]Finding, error) {
	added := newAddedLines()
	if err := s.readDiff(added, diff); err != nil {
		return nil, err
	}
	return s.scanAdded(added, nil), nil
}

// scanPaths reads the named files, or stdin for `-`. The prose profile's caller usually holds the
// text rather than a path — a PR body being drafted — so stdin is the common form there.
func (s scanner) scanPaths(args []string, cwd string, cfg Config, over *scanned, source bool) ([]Finding, error) {
	read := func(file string, lines []string) []Finding {
		if source {
			if !s.record {
				return s.scanSource(file, lines, nil, lines)
			}
			found := RecordFindings(file, lines)
			_, under, _ := splitRecord(lines)
			return append(found, s.scanSource(file, under, nil, under)...)
		}
		return s.scanProse(file, lines)
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("the %s profile needs a path, or `-` for stdin — the scan did NOT run", s.profile)
	}
	var found []Finding
	for _, arg := range args {
		if arg == "-" {
			body, err := readAllCapped(os.Stdin, maxStdinBytes)
			if err != nil {
				return nil, fmt.Errorf("stdin %v — exit 2, the scan did NOT run", err)
			}
			over.files++
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
		over.files++
		found = append(found, read(arg, shell.SplitLines(string(body)))...)
	}
	return found, nil
}

// reportVoice prints the findings sorted, then a denominator on stderr. Sorted rather than in scan
// order so two runs over one tree print one report and a diff of two runs is the change.
// scanned is what a run covered, so an empty report can be told from an empty scan. diffscan's own
// header states the rule: the denominator is contract, not decoration.
type scanned struct {
	files      int
	declined   int
	suppressed int
	conf       string
}

func reportVoice(out console, profile Profile, found []Finding, over scanned) int {
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
	out.note("%s profile: %d finding(s)%s over %d file(s), %d declined unread.",
		profile, len(found), tally(parts), over.files, over.declined)
	if over.suppressed > 0 {
		out.note("%d finding(s) suppressed by the allowlist in %s.", over.suppressed, over.conf)
	}
	if over.files == 0 {
		out.note("nothing reached the scan, so this run says nothing about the text.")
		return exitClean
	}
	if len(found) == 0 {
		out.note("clean, which says the register was read and matched nothing — not that the text was not read.")
		return exitClean
	}
	out.note("each finding is an edit the lane makes, not a count to drive down. A finding that must stand goes in the allowlist with its reason.")
	return exitFound
}

// reportCounts prints one line per path: the finding count, a space, and the path as it was given. A
// path with no findings gets a line too. reportVoice, the function that prints findings, shows at most
// maxFindings, a const in this file. A count is one line however many findings it counts, so that cap
// does not apply here.
func reportCounts(out console, s scanner, profile Profile, args []string, cwd string, cfg Config, over *scanned) int {
	if len(args) == 0 {
		return out.refuse(fmt.Errorf("--per-file needs a path to count, and the %s profile got none — the scan did NOT run", profile))
	}
	total := 0
	for _, arg := range args {
		// --per-file is refused with the comment profile, so these are prose paths.
		found, err := s.scanPaths([]string{arg}, cwd, cfg, over, false)
		if err != nil {
			// The counts already printed stay on stdout. The caller pairs them back against the paths it
			// asked for, and the last path with a line is where the run stopped.
			return out.refuse(err)
		}
		fmt.Fprintf(out.stdout, "%d %s\n", len(found), arg)
		total += len(found)
	}
	over.suppressed = *s.suppressed
	out.note("%s profile: %d finding(s) over %d file(s), %d declined unread.",
		profile, total, over.files, over.declined)
	if over.suppressed > 0 {
		out.note("%d finding(s) suppressed by the allowlist in %s.", over.suppressed, over.conf)
	}
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

// A hyphenated pair in prose, and the identifier words a file spells outside its comments.
var reHyphenPair = regexp.MustCompile(`\b([a-z]+)-([a-z]+)\b`)
var reIdentifierWord = regexp.MustCompile(`[A-Za-z_$][\w$]*`)
var reCamelBreak = regexp.MustCompile(`([a-z0-9])([A-Z])`)

// identifierWordsOf collects what a file's identifiers spell, lowercased: each whole identifier, and
// each adjacent pair of its camel humps joined. A compound in prose is matched against the pairs,
// because the compound a reader meets with a hyphen sits inside an identifier as two humps.
func identifierWordsOf(lines []string) map[string]bool {
	out := map[string]bool{}
	for _, line := range lines {
		for _, word := range reIdentifierWord.FindAllString(line, -1) {
			out[strings.ToLower(word)] = true
			humps := strings.Fields(reCamelBreak.ReplaceAllString(word, "$1 $2"))
			for i := 0; i+1 < len(humps); i++ {
				out[strings.ToLower(humps[i]+humps[i+1])] = true
			}
		}
	}
	return out
}

// A hump-cased name, and the comma that would place it.
var reCamelToken = regexp.MustCompile(`\b[a-z][a-z0-9]*[A-Z][A-Za-z0-9]*\b`)
var reAppositiveTail = regexp.MustCompile(`^\s*,`)

// placeableNames are the terms of art a reader already places, spelled the way an identifier is.
// This check reported three of them in its own comment, which is what that run is for.
var placeableNames = map[string]bool{"camelcase": true, "srgb": true, "ios": true, "macos": true,
	"tvos": true, "watchos": true, "iphone": true, "ipad": true, "javascript": true, "typescript": true}

// bareIdentifiers finds a name a block uses without saying what it is. A reader who cannot place a
// name reads the sentence as being about something else. An appositive places one, and the site's own
// declaration needs none. It reached 37 of 304 notes on a sixty-file set, and its false positives are
// the terms of art in placeableNames, a list of names a reader already knows.
func (s scanner) bareIdentifiers(file string, b block, lines []string, declared map[string]bool) []Finding {
	var found []Finding
	for at := b.start; at <= b.end && at <= len(lines); at++ {
		text := proseOf(lines[at-1])
		for _, span := range reCamelToken.FindAllStringIndex(text, -1) {
			token := text[span[0]:span[1]]
			if declared[strings.ToLower(token)] || placeableNames[strings.ToLower(token)] ||
				reAppositiveTail.MatchString(text[span[1]:]) {
				continue
			}
			found = append(found, Finding{File: file, Line: at, Check: checkBareIdent, Text: token})
		}
	}
	return found
}

// coinedIdentifiers finds a hyphenated compound in a block whose camelCase join the code spells. The
// code invented the word and the prose took it, so the rename lane owns it. A compound the conf
// names as the domain's passes. comment-census's README holds the measurement that seeds it.
func (s scanner) coinedIdentifiers(file string, b block, lines []string, identifiers map[string]bool) []Finding {
	var found []Finding
	known := map[string]bool{}
	for _, word := range s.domain {
		known[strings.ToLower(word)] = true
	}
	for at := b.start; at <= b.end && at <= len(lines); at++ {
		text := proseOf(lines[at-1])
		for _, m := range reHyphenPair.FindAllStringSubmatch(text, -1) {
			if known[strings.ToLower(m[0])] || !identifiers[strings.ToLower(m[1]+m[2])] {
				continue
			}
			found = append(found, Finding{File: file, Line: at, Check: checkCoinedIdent, Text: m[0]})
		}
	}
	return found
}

// declaredAt is what the declaration under a block spells, which is the name a block may use without
// placing it. The block sits on that declaration, so its reader has the name in front of them.
//
// The lines it reads are the file as the change leaves it. A reader opens the file, so the
// declaration under a block is in front of them whether or not the change touched it.
func declaredAt(lines []string, b block) map[string]bool {
	out := map[string]bool{}
	// Comment lines are walked past as well as blank ones. Over a diff a block ends where its added
	// lines end, and the rest of that same comment then stands between the block and the declaration.
	// A walk stopping there would read the prose as the declaration and exempt every word of it.
	at := b.end + 1
	for at <= len(lines) && at <= b.end+declarationSearch {
		line := strings.TrimLeft(lines[at-1], shell.SpaceBytes)
		if line != "" && !isComment(line) && !isShebang(at, line) {
			break
		}
		at++
	}
	if at > len(lines) || at > b.end+declarationSearch {
		return out
	}
	for _, word := range reIdentifierWord.FindAllString(lines[at-1], -1) {
		out[strings.ToLower(word)] = true
	}
	return out
}

// How far under a block the declaration may sit before the line found is no longer the block's own.
// The gap holds a comment's remaining lines and the blanks around them. A bound well over the
// longest block stops a runaway walk from exempting a name off unrelated code.
const declarationSearch = 60

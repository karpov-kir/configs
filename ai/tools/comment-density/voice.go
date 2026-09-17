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
package density

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"kk-flavor/tools/diffscan"
	gitrepo "kk-flavor/tools/repo"
	"kk-flavor/tools/shell"
)

// A block over this many text lines is a wall. A file header gets more, because a header carries the
// call order and error modes a published surface owes. Text lines, not raw lines: a `/**` opening and
// a `*/` closing carry no words, and counting them would make every four-sentence block a finding.
const (
	voiceLongBlock  = 4
	voiceLongHeader = 8
)

// Findings and echoed text are bounded the same way the default mode's are: under kk-pr this text
// comes off a branch somebody else wrote.
const (
	maxFindings   = 500
	maxMatchBytes = 120
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
)

// AllChecks is every check name, for the allowlist parser to refuse an entry naming none of them.
var AllChecks = []string{checkBold, checkContrast, checkCounterfactal, checkNoSubject,
	checkIntensifier, checkPositional, checkLongBlock, checkCoined}

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
	reInsteadOf  = regexp.MustCompile(`(?:^|[.;:]\s+|,\s+)[Ii]nstead of\b`)

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

	// "nothing", "nobody" and "the one X" used to make a claim feel larger than it is.
	reIntensifier = regexp.MustCompile(`(?i)\bnothing\b|\bnobody\b|\bno one\b|\bthe one\s`)

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
	for _, marker := range []string{"/**", "/*", "*/", "//", "*", "#"} {
		if strings.HasPrefix(line, marker) {
			return strings.TrimSpace(line[len(marker):])
		}
	}
	return line
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

// commentBlocks groups adjacent comment lines. Mirrors bloat-judge's own grouping: inside a `/*` run
// every line belongs to the block until one carries `*/`, whatever it starts with.
func commentBlocks(lines []string) []block {
	var found []block
	inBlock, inStar := false, false
	for i, raw := range lines {
		line := strings.TrimLeft(raw, shell.SpaceBytes)
		switch {
		case inStar:
			found[len(found)-1].end = i + 1
			if strings.Contains(line, "*/") {
				inStar, inBlock = false, false
			}
		case !isComment(line):
			inBlock = false
		case inBlock:
			found[len(found)-1].end = i + 1
			inStar = opensStar(line)
		default:
			found = append(found, block{start: i + 1, end: i + 1})
			inBlock = true
			inStar = opensStar(line)
		}
	}
	return found
}

func opensStar(line string) bool {
	return strings.HasPrefix(line, "/*") && !strings.Contains(line[2:], "*/")
}

// isFileHeader says this block opens the file: nothing but blank lines stands above it. A block two
// lines down from an import is not a header, and giving it the header's allowance would let every
// block in a file claim eight lines by sitting near the top.
func (b block) isFileHeader(lines []string) bool {
	for at := 1; at < b.start; at++ {
		if strings.TrimSpace(lines[at-1]) != "" {
			return false
		}
	}
	return true
}

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
	allowed allowlist
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
// a single space between them. The space is a byte of the segment too, and it belongs to the line it
// follows, so a match landing on it reports the line the sentence continues onto.
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

// scanSource reads a source file's comment blocks. Findings whose lines lie outside `within` are
// dropped where `within` is non-nil, which is how the comment profile reports only what a diff added.
// A finding spanning several lines stays where any one of them was added: a sentence is the unit a
// writer edits, and half of one is not a thing to report.
func (s scanner) scanSource(file string, lines []string, within map[int]bool) []Finding {
	var found []Finding
	for _, b := range commentBlocks(lines) {
		if within != nil && !b.touches(within) {
			continue
		}
		limit := voiceLongBlock
		if b.isFileHeader(lines) {
			limit = voiceLongHeader
		}
		if n := b.textLines(lines); n > limit {
			found = append(found, Finding{File: file, Line: b.start, Check: checkLongBlock,
				Text: fmt.Sprintf("%d text lines, over %d", n, limit)})
		}
		found = append(found, s.scanSegment(file, join(lines, b.start, b.end, proseOf), within)...)
	}
	return s.allowed.filter(found)
}

func (b block) touches(within map[int]bool) bool {
	for at := b.start; at <= b.end; at++ {
		if within[at] {
			return true
		}
	}
	return false
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
		found = append(found, s.scanSegment(file, join(lines, paragraph, end, strings.TrimSpace), nil)...)
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
		if line == "" {
			flush(at - 1)
			continue
		}
		if paragraph == 0 {
			paragraph = at
		}
	}
	flush(len(lines))
	return s.allowed.filter(found)
}

// scanSegment is every check over one segment, and the only place a check runs. One function, so the
// three profiles cannot drift into reading the same sentence differently.
func (s scanner) scanSegment(file string, seg segment, within map[int]bool) []Finding {
	if seg.text == "" {
		return nil
	}
	text := seg.text
	var found []Finding
	add := func(check string, from, to int) {
		first, last := seg.lineSpan(from, to)
		if within != nil && !spans(first, last, within) {
			return
		}
		found = append(found, Finding{File: file, Line: first, Check: check,
			Text: strings.TrimSpace(text[from:to])})
	}

	// The coined check reads the segment as typed, and every other check reads it with its inline code
	// spans blanked. A coined word inside backticks is an identifier built on the coined term, and that
	// is the one thing a reader still has to have been in the room for.
	//
	// Blanking replaces each span with as many spaces, so an offset into `prose` addresses the same
	// byte of `text`. Every finding is echoed out of `text`, so the writer reads what they typed.
	prose := reInlineCode.ReplaceAllStringFunc(text, func(span string) string {
		return strings.Repeat(" ", len(span))
	})
	for _, word := range s.coined {
		for _, at := range coinedPattern(word).FindAllStringIndex(text, -1) {
			add(checkCoined, at[0], at[1])
		}
		if inIdentifier := coinedInIdentifier(word); inIdentifier != nil {
			for _, at := range inIdentifier.FindAllStringIndex(text, -1) {
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
	for _, re := range []*regexp.Regexp{reRatherThan, reNeverNot, reInsteadOf} {
		for _, at := range re.FindAllStringIndex(prose, -1) {
			add(checkContrast, at[0], at[1])
		}
	}
	for _, at := range reIntensifier.FindAllStringIndex(prose, -1) {
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
		if at := reParticiplePhr.FindStringIndex(read); at != nil && !reOpensWithVerb.MatchString(read) {
			add(checkNoSubject, span[0]+at[0], span[0]+at[1])
		}
		if at := rePositional.FindStringIndex(read); at != nil {
			add(checkPositional, span[0]+at[0], span[0]+at[1])
		}
	}
	return found
}

// spans says a finding reaches a line the diff added.
func spans(first, last int, within map[int]bool) bool {
	for at := first; at <= last; at++ {
		if within[at] {
			return true
		}
	}
	return false
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

// coinedPattern matches the word and anything built off it — `rung` also catches `rungs`, and `climb`
// catches `climbs` and `climbing`, because a coined term is coined in every form it takes.
func coinedPattern(word string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `[a-z]*\b`)
}

// coinedInIdentifier matches the word as a camelCase segment: `playedRung`, `StreamRung`,
// `readSoleRungDeclaring`. A coined term that became an identifier is the same word the reader does
// not share, and the rename is what carries it away. Written as its own pattern because Go's regexp
// has no lookbehind, so the segment's preceding lowercase run is matched rather than asserted.
func coinedInIdentifier(word string) *regexp.Regexp {
	if word == "" {
		return nil
	}
	titled := strings.ToUpper(word[:1]) + word[1:]
	return regexp.MustCompile(`\b[A-Za-z]*[a-z]` + regexp.QuoteMeta(titled) + `[A-Za-z]*\b`)
}

// ScanFile is the whole scan over one file's content, for a caller holding the bytes already. The
// suite reads the fixture through it, so what a test measures is what a run reports.
func ScanFile(profile Profile, coined []string, allowed allowlist, file, content string) []Finding {
	s := scanner{profile: profile, coined: coined, allowed: allowed}
	lines := shell.SplitLines(content)
	if profile == ProfileComment {
		return s.scanSource(file, lines, nil)
	}
	return s.scanProse(file, lines)
}

// voice runs the mode. `--voice` selects it as the first argument only, the way `--bar` does.
func voice(out console, git gitrepo.Git, args []string, cwd string, cfg Config) int {
	profile := ProfileComment
	for len(args) > 0 && strings.HasPrefix(args[0], "--profile=") {
		named := Profile(strings.TrimPrefix(args[0], "--profile="))
		switch named {
		case ProfileComment, ProfileProse, ProfileInstruction:
			profile = named
		default:
			return out.refuseArguments(fmt.Errorf("no profile %q — the scan did NOT run. Profiles: comment prose instruction",
				shell.CutBytesMarked(shell.Oneline(string(named)), 40)))
		}
		args = args[1:]
	}

	coined, allowed, err := voiceConfig(cwd)
	if err != nil {
		return out.refuse(err)
	}
	s := scanner{profile: profile, coined: coined, allowed: allowed}

	var found []Finding
	if profile == ProfileComment {
		found, err = s.scanChange(git, args, cwd, cfg)
	} else {
		found, err = s.scanPaths(args, cwd, cfg)
	}
	if err != nil {
		return out.refuse(err)
	}
	return reportVoice(out, profile, found)
}

// scanChange reads the diff — git's, or one on stdin for a branch this checkout does not hold, which
// is how the negative control runs over `gh pr diff`. Blocks are runs of ADDED comment lines, so the
// scan needs no working tree: a block split by an untouched line is two blocks, which is what a
// reviewer reading the diff sees too.
func (s scanner) scanChange(git gitrepo.Git, args []string, cwd string, cfg Config) ([]Finding, error) {
	var diff []byte
	var err error
	if len(args) > 0 && args[0] == "-" {
		if diff, err = io.ReadAll(os.Stdin); err != nil {
			return nil, fmt.Errorf("could not read the diff on stdin — exit 2, the scan did NOT run")
		}
	} else {
		if err = diffscan.RefuseNonRevisions(git, args, cwd); err != nil {
			return nil, err
		}
		if diff, err = diffscan.Diff(git, cwd, args); err != nil {
			return nil, err
		}
	}

	// Added comment lines, per file, in the order the diff carries them.
	type addedLine struct {
		at   int
		text string
	}
	byFile := map[string][]addedLine{}
	var order []string
	var result diffscan.Result
	walkErr := result.WalkDiff(diff, func(a diffscan.AddedLine) {
		if notThisRepositorysSource(a.File) || a.Line == 0 {
			return
		}
		if _, seen := byFile[a.File]; !seen {
			order = append(order, a.File)
		}
		byFile[a.File] = append(byFile[a.File], addedLine{at: a.Line, text: a.Text})
	})
	if walkErr != nil {
		return nil, fmt.Errorf("the diff could not be read to the end (%v) — exit 2, the scan did NOT run over all of it. Not a clean result.", walkErr)
	}

	// The untracked half runs only with no revisions and no diff on stdin, the way the default mode's
	// does: with revisions the caller named two commits, and a file in neither of them is not part of
	// what they asked about. A new file is the commonest place a new comment lands, so a voice scan
	// that skipped it would report clean over the change most worth reading.
	fromStdin := len(args) > 0 && args[0] == "-"
	named, _ := diffscan.RevisionsNamed(args)
	if !fromStdin && len(named) == 0 {
		untracked := func(a diffscan.AddedLine) {
			if notThisRepositorysSource(a.File) || a.Line == 0 {
				return
			}
			if _, seen := byFile[a.File]; !seen {
				order = append(order, a.File)
			}
			byFile[a.File] = append(byFile[a.File], addedLine{at: a.Line, text: a.Text})
		}
		if err := result.WalkUntracked(git, cwd, diffscan.Options{MaxFileBytes: cfg.MaxFileBytes}, untracked); err != nil {
			return nil, errors.New("could not list untracked files — exit 2, the scan did NOT run over them.")
		}
	}

	var found []Finding
	for _, file := range order {
		added := byFile[file]
		// A sparse file: the added lines at their own numbers, everything else blank. Blanks end a
		// block, which is what makes an added run its own block.
		highest := 0
		within := map[int]bool{}
		for _, a := range added {
			if a.at > highest {
				highest = a.at
			}
			within[a.at] = true
		}
		lines := make([]string, highest)
		for _, a := range added {
			lines[a.at-1] = a.text
		}
		found = append(found, s.scanSource(file, lines, within)...)
	}
	return found, nil
}

// scanPaths reads the named files, or stdin for `-`. The prose profile's caller usually holds the
// text rather than a path — a PR body being drafted — so stdin is the common form there.
func (s scanner) scanPaths(args []string, cwd string, cfg Config) ([]Finding, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("the %s profile needs a path, or `-` for stdin — the scan did NOT run", s.profile)
	}
	var found []Finding
	for _, arg := range args {
		if arg == "-" {
			body, err := io.ReadAll(os.Stdin)
			if err != nil {
				return nil, fmt.Errorf("could not read stdin — exit 2, the scan did NOT run")
			}
			found = append(found, s.scanProse("-", shell.SplitLines(string(body)))...)
			continue
		}
		path := arg
		if !strings.HasPrefix(path, "/") {
			path = shell.Join(cwd, path)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("cannot read %s — exit 2, the scan did NOT run",
				shell.CutBytesMarked(shell.Oneline(arg), maxPathBytes))
		}
		if int64(len(body)) > cfg.MaxFileBytes {
			return nil, fmt.Errorf("%s is over DENSITY_MAX_FILE_BYTES — exit 2, the scan did NOT run",
				shell.CutBytesMarked(shell.Oneline(arg), maxPathBytes))
		}
		found = append(found, s.scanProse(arg, shell.SplitLines(string(body)))...)
	}
	return found, nil
}

// reportVoice prints the findings sorted, then a denominator on stderr. Sorted rather than in scan
// order so two runs over one tree print one report and a diff of two runs is the change.
func reportVoice(out console, profile Profile, found []Finding) int {
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
	out.note("%s profile: %d finding(s)%s.", profile, len(found), tally(parts))
	if len(found) == 0 {
		out.note("clean, which says the register was read and matched nothing — not that the text was not read.")
		return exitClean
	}
	out.note("each finding is an edit the lane makes, not a count to drive down. A finding that must stand goes in the allowlist with its reason.")
	return exitFound
}

func tally(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return " — " + strings.Join(parts, ", ")
}

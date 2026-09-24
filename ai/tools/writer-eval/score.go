// Scores a comment block the writer produced, by what the block carries. Four judge kinds were built
// in this campaign and all four were dropped on measurement. The label each of them failed asked a
// reader what a block does NOT say. The two that held asked about a contradiction and a mixture,
// both present in the text. So this scorer reads the block and asks only what is there.
//
// Every check here was counted on a set of reviewed code before it was written down, and the counts
// are in comment-census's README. A check that reached clear prose is absent on purpose.
package writereval

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	census "configs/ai/tools/comment-census"
)

// Return is what the writer hands back for one site: the block it wrote, or none, and the audit
// lines it classified its own words with.
type Return struct {
	None  bool
	Block string
	Terms []Audit
	Verbs []Audit
	// Summary and Note are the two part lines. Question 1 decides the summary. Question 3 runs
	// whatever it answered, and a site is none only where both parts are. One answer for the whole
	// site declined every block a reviewer had kept. Of those declines, 37 of 37 and 15 of 15 stopped
	// at question 1, and the facts file stayed shut.
	Summary  Part
	Note     Part
	Attempts int
	// At is the line the writer put the block on. A strip offers a site, and the fact is often about a
	// declaration further down the file. Three of four blocks a reviewer sent back on 2026-09-22 sat
	// where the archive had put them, and their subjects were elsewhere.
	At int
	// Carried says the return sent the claim somewhere else in the tree. A claim about a row of data
	// goes to a field on that row, and a block is the wrong home for it however well written.
	Carried bool
	// Routed is the fates the return named, one entry per routed line, lower-cased: `for the pr body`,
	// `does not fit`, `stale` and the rest. A case asking where a fact went reads this.
	Routed []string
	// Record is the note's three slot lines as the writer gave them. The record check reads them against
	// the block, the way the pipeline's writer pipes them before it writes.
	Record []string
	// Rounds is how many times the record check sent the block back before this return.
	Rounds int
}

// Part is what one half of a block came back as.
type Part int

const (
	// PartUnsaid is a return that named no part line.
	PartUnsaid Part = iota
	// PartNone is a part the writer decided against.
	PartNone
	// PartWritten is a part the writer wrote.
	PartWritten
)

// Audit is one classified word or phrase from the writer's return.
type Audit struct {
	Word  string
	Class string
}

var auditLine = regexp.MustCompile(`(?i)^\s*(term|verb):\s*(.+?)\s+[—-]\s+(\w+)\s*$`)

// placedLine is the line the writer says its block sits on.
var placedLine = regexp.MustCompile(`(?i)^\s*at:\s*(\d+)\s*$`)

// The lines a writer returns beside its block: what it dropped and where that went. They are the
// return's own bookkeeping, and reading one as prose scored a correct `carried by` as a written
// block. Three of l07's five rolls answered correctly and one was counted.
var verdictLine = regexp.MustCompile(`(?i)^\s*(shown by the body|carried by [^:]*|stale|for the pr body|does not fit|belongs at [^:]*|invariant diverged|about this code|unverified|none):`)

// recordLine is one slot of the note's record.
var recordLine = regexp.MustCompile(`(?i)^\s*(fact|bears_on|does):\s*\S`)

var summaryLine = regexp.MustCompile(`(?i)^\s*summary:\s*(needed|none)\s*$`)
var noteLine = regexp.MustCompile(`(?i)^\s*note:\s*(written|none)\s*$`)
var attemptLine = regexp.MustCompile(`(?i)^\s*attempt \d+:`)
var blockMarker = regexp.MustCompile(`^\s*(///|//|/\*\*|/\*|\*/|\*|#)\s?`)

// ParseReturn reads the writer's answer. A line shaped as an audit entry is one, and every other
// line is the block. The word `none` alone is the writer declining the site.
func ParseReturn(raw string) Return {
	var out Return
	var body []string
	for _, line := range strings.Split(raw, "\n") {
		if m := summaryLine.FindStringSubmatch(line); m != nil {
			out.Summary = PartNone
			if strings.EqualFold(m[1], "needed") {
				out.Summary = PartWritten
			}
			continue
		}
		if m := noteLine.FindStringSubmatch(line); m != nil {
			out.Note = PartNone
			if strings.EqualFold(m[1], "written") {
				out.Note = PartWritten
			}
			continue
		}
		if m := placedLine.FindStringSubmatch(line); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				out.At = n
			}
			continue
		}
		if attemptLine.MatchString(line) {
			out.Attempts++
			continue
		}
		if recordLine.MatchString(line) {
			out.Record = append(out.Record, strings.TrimSpace(line))
			continue
		}
		if m := verdictLine.FindStringSubmatch(line); m != nil {
			fate := strings.ToLower(strings.TrimSpace(m[1]))
			if strings.HasPrefix(fate, "carried by") {
				out.Carried = true
			}
			out.Routed = append(out.Routed, fate)
			continue
		}
		if m := auditLine.FindStringSubmatch(line); m != nil {
			entry := Audit{Word: strings.Trim(m[2], "`\"'"), Class: strings.ToLower(m[3])}
			if strings.EqualFold(m[1], "term") {
				out.Terms = append(out.Terms, entry)
			} else {
				out.Verbs = append(out.Verbs, entry)
			}
			continue
		}
		body = append(body, line)
	}
	joined := strings.TrimSpace(strings.Join(body, "\n"))
	if strings.EqualFold(strings.Trim(joined, "`.*_ "), "none") {
		out.None = true
		return out
	}
	out.Block = joined
	return out
}

// CommentLines is the block's comment lines as the writer wrote them, markers kept, which is what the
// record check reads above the declaration.
func (r Return) CommentLines() []string {
	var out []string
	inSpan := false
	for _, line := range strings.Split(r.Block, "\n") {
		trimmed := strings.TrimSpace(line)
		opens := strings.HasPrefix(trimmed, "/*")
		if inSpan || opens || blockMarker.MatchString(line) {
			out = append(out, line)
		}
		if strings.Contains(trimmed, "*/") {
			inSpan = false
		} else if opens {
			inSpan = true
		}
	}
	return out
}

// Text is the block's comment lines with their markers removed, which is what every check below
// reads. A writer returns the declaration under the block as often as not, and a declaration counted
// as a sentence put seven of twenty-one plain blocks over the note's two-sentence ceiling.
func (r Return) Text() string {
	var out []string
	inSpan := false
	for _, line := range strings.Split(r.Block, "\n") {
		trimmed := strings.TrimSpace(line)
		opens := strings.HasPrefix(trimmed, "/*")
		marked := inSpan || opens || blockMarker.MatchString(line)
		if strings.Contains(trimmed, "*/") {
			inSpan = false
		} else if opens {
			inSpan = true
		}
		if !marked {
			continue
		}
		cut := blockMarker.ReplaceAllString(line, "")
		cut = strings.TrimSuffix(strings.TrimSpace(cut), "*/")
		out = append(out, strings.TrimSpace(cut))
	}
	return strings.TrimSpace(strings.Join(out, " "))
}

// takenVerbs are the metaphor verbs the census found on the reviewed set and read as figures. The
// writing standard named six more that fire zero times over sixty files. `answer` and `reach` are
// the literal verbs of a codebase that queries things. All eight stay out.
var takenVerbs = map[string]bool{"cover": true, "settle": true, "sit in": true, "load-bearing": true}

// noteSentences is the ceiling the comment rule sets for a note.
const noteSentences = 2

// Failure is one check a block did not pass. It carries the text that failed it.
type Failure struct {
	Check string
	Found string
}

// Score reads a written block and returns every check it failed. An empty result is a block that
// passes, and the caller counts those against the bar.
func Score(r Return) []Failure {
	if r.None {
		return nil
	}
	text := r.Text()
	sentences := census.Sentences(text)
	var failures []Failure
	add := func(check, found string) { failures = append(failures, Failure{check, found}) }

	for _, shape := range census.Shapes() {
		switch shape.Name {
		case "counterfactual-consequence", "anthropomorphism", "elided-verb", "negated-case":
			for _, sentence := range sentences {
				if hit := shape.Hits(sentence); hit != "" {
					add(shape.Name, hit)
				}
			}
		}
	}
	for _, sentence := range sentences {
		if stem := census.MetaphorHit(sentence); stem != "" && takenVerbs[stem] {
			add("metaphor-verb", stem)
		}
	}
	// The ceiling is the note's, and the summary sits outside it. The rule's two-sentence bound is in
	// the note paragraph, and the block's own bound is four prose lines. A check counting the block
	// put a summary and a two-sentence note over a limit neither of them breaks, and the plain half
	// went from 6% failing to 15%.
	inNote := sentences
	if r.Summary == PartWritten && len(inNote) > 0 {
		inNote = inNote[1:]
	}
	if len(inNote) > noteSentences {
		add("over-the-sentence-ceiling", fmt.Sprintf("%d note sentence(s)", len(inNote)))
	}
	for _, v := range r.Verbs {
		if v.Class == "figure" {
			add("verb-audited-as-a-figure", v.Word)
		}
	}
	for _, t := range r.Terms {
		if t.Class != "identifier" && t.Class != "plain" && t.Class != "path" {
			add("term-audited-as-none-of-the-three", t.Word+" — "+t.Class)
		}
	}
	sort.SliceStable(failures, func(i, j int) bool { return failures[i].Check < failures[j].Check })
	return failures
}

// Expected is what a labelled case says the writer should do at its site.
type Expected string

const (
	// ExpectNone is a site that earns no block.
	ExpectNone Expected = "none"
	// ExpectWritten is a site that earns one, which then has to pass every check.
	ExpectWritten Expected = "written"
)

// Verdict is one case's outcome: whether the class matched, and what the block failed.
type Verdict struct {
	Name     string
	Want     Expected
	Got      Expected
	Failures []Failure
}

// Passed says whether a case cleared the bar. The class has to match, and a written block has to
// fail no check.
//
// A carried claim satisfies a case wanting none. Both leave the site without a block, and the carried
// line says where the claim went. A case wanting carried asks for that line by name.
func (v Verdict) Passed() bool {
	if len(v.Failures) > 0 {
		return false
	}
	return v.Want == v.Got || (v.Want == ExpectNone && v.Got == ExpectCarried)
}

// Judge scores one return against what its case expected. A declined site and a rename are scored on
// their class alone. A run that checked the prose of a declined block would report a failure about
// text it never received.
func Judge(name string, want Expected, r Return) Verdict {
	got := ClassOf(r)
	v := Verdict{Name: name, Want: want, Got: got}
	if got == ExpectWritten {
		v.Failures = Score(r)
	}
	// A part the writer set out to write and then answered none for is a skipped rewrite unless the
	// attempts are there to read. The gate applies per part, and the writer names the parts itself,
	// so this reads presence.
	if got == ExpectNone && r.Summary == PartWritten && r.Attempts < 2 {
		v.Failures = append(v.Failures, Failure{"summary-dropped-without-two-attempts",
			fmt.Sprintf("%d attempt(s) shown", r.Attempts)})
	}
	if got == ExpectNone && r.Note == PartWritten && r.Attempts < 2 {
		v.Failures = append(v.Failures, Failure{"note-dropped-without-two-attempts",
			fmt.Sprintf("%d attempt(s) shown", r.Attempts)})
	}
	return v
}

// Parts is what the two part lines said, for a table that counts them. A run that reports the site
// alone cannot see a note lost to the summary's verdict, which is the defect the parts exist for.
func (r Return) Parts() (summary, note Part) { return r.Summary, r.Note }

// PartName is how a part reads in a table.
func PartName(p Part) string {
	switch p {
	case PartNone:
		return "none"
	case PartWritten:
		return "written"
	}
	return "unsaid"
}

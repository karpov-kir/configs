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
	"strings"

	census "kk-flavor/tools/comment-census"
)

// Return is what the writer hands back for one site: the block it wrote, or none, and the audit
// lines it classified its own words with.
type Return struct {
	None  bool
	Block string
	Terms []Audit
	Verbs []Audit
	// Answered and Needed carry what question 1 said, and Attempts counts the rewrites the writer
	// showed. A none on a site question 1 called needed has to show two attempts. Otherwise the gate
	// the writer runs over its own block reads as permission to skip the site.
	Answered bool
	Needed   bool
	Attempts int
}

// Audit is one classified word or phrase from the writer's return.
type Audit struct {
	Word  string
	Class string
}

var auditLine = regexp.MustCompile(`(?i)^\s*(term|verb):\s*(.+?)\s+[—-]\s+(\w+)\s*$`)
var questionLine = regexp.MustCompile(`(?i)^\s*question 1:\s*(needed|none)\s*$`)
var attemptLine = regexp.MustCompile(`(?i)^\s*attempt \d+:`)
var blockMarker = regexp.MustCompile(`^\s*(///|//|/\*\*|/\*|\*/|\*|#)\s?`)

// ParseReturn reads the writer's answer. A line shaped as an audit entry is one, and every other
// line is the block. The word `none` alone is the writer declining the site.
func ParseReturn(raw string) Return {
	var out Return
	var body []string
	for _, line := range strings.Split(raw, "\n") {
		if m := questionLine.FindStringSubmatch(line); m != nil {
			out.Answered = true
			out.Needed = strings.EqualFold(m[1], "needed")
			continue
		}
		if attemptLine.MatchString(line) {
			out.Attempts++
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

// Text is the block with its comment markers removed, which is what every check below reads.
func (r Return) Text() string {
	var out []string
	for _, line := range strings.Split(r.Block, "\n") {
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
	if len(sentences) > noteSentences+1 {
		add("over-the-sentence-ceiling", fmt.Sprintf("%d sentences", len(sentences)))
	}
	for _, v := range r.Verbs {
		if v.Class == "figure" {
			add("verb-audited-as-a-figure", v.Word)
		}
	}
	for _, t := range r.Terms {
		if t.Class != "identifier" && t.Class != "domain" && t.Class != "plain" {
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
func (v Verdict) Passed() bool { return v.Want == v.Got && len(v.Failures) == 0 }

// Judge scores one return against what its case expected. A declined site and a rename are scored on
// their class alone. A run that checked the prose of a declined block would report a failure about
// text it never received.
func Judge(name string, want Expected, r Return) Verdict {
	got := ClassOf(r)
	v := Verdict{Name: name, Want: want, Got: got}
	if got == ExpectWritten {
		v.Failures = Score(r)
	}
	// A none on a site question 1 called needed is a skipped rewrite unless the attempts are there to
	// read. The writer names that site itself, so this reads presence.
	if got == ExpectNone && r.Answered && r.Needed && r.Attempts < 2 {
		v.Failures = append(v.Failures, Failure{"none-without-two-attempts",
			fmt.Sprintf("%d attempt(s) shown", r.Attempts)})
	}
	return v
}

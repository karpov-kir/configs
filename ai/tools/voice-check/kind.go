package voicecheck

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"configs/ai/tools/shell"
)

// The kinds a prose text can be read as, beside the register the prose profile reads. A PR body and a
// ticket each have a width their reader expects, which is what `kind` adds.
const (
	KindPRBody = "pr-body"
	KindTicket = "ticket"
)

const (
	checkOverBand       = "over-band"
	checkTestsNarration = "tests-narration"
)

// kindChecks run only under `--kind`. They read a whole body, where the register checks read a comment
// block. The house corpus holds blocks, so these checks are tested on their own.
var kindChecks = []string{checkOverBand, checkTestsNarration}

// band is a kind's width, in the authored words a body may carry. Its reference median prints beside a
// finding, so an author reads the line and its source together.
type band struct {
	words  int
	median int
}

// defaultBands are the figures `human-writing.md` -> Change descriptions (PRs) states, and they move
// there. Fifty merged PRs of the host repository ran to a median of 48 authored words. Their 90th
// percentile was 110. A ticket takes the PR figure until a body of tickets is counted.
var defaultBands = map[string]band{
	KindPRBody: {words: 110, median: 48},
	KindTicket: {words: 110, median: 48},
}

// templatePaths are where a repository keeps its PR template, relative to its root.
var templatePaths = []string{
	".github/pull_request_template.md", ".github/PULL_REQUEST_TEMPLATE.md",
	"docs/pull_request_template.md", "pull_request_template.md", "PULL_REQUEST_TEMPLATE.md",
}

// templateLines is the repository's PR template, one trimmed line each. The measurement left the
// template's own lines out, since the author did not write them.
func templateLines(root string) map[string]bool {
	out := map[string]bool{}
	for _, rel := range templatePaths {
		body, err := os.ReadFile(shell.Join(root, rel))
		if err != nil {
			continue
		}
		for _, line := range shell.SplitLines(string(body)) {
			if trimmed := templateKey(line); trimmed != "" {
				out[trimmed] = true
			}
		}
		break
	}
	return out
}

// reCheckbox is a task-list box, ticked or not. A template ships its checklist unticked and a merged
// body carries it ticked, and both are the template's line.
var reCheckbox = regexp.MustCompile(`^([-*+]\s+)\[[ xX]\]`)

// templateKey is a line as the template match reads it: trimmed, with any checkbox read as unticked.
func templateKey(line string) string {
	return reCheckbox.ReplaceAllString(strings.TrimSpace(line), "$1[ ]")
}

var (
	reHTMLComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	reHeading     = regexp.MustCompile(`^\s{0,3}#{1,6}\s`)
	reStackMap    = regexp.MustCompile(`(?i)^\s*stack, base first:`)
	reAttribution = regexp.MustCompile(`(?i)generated with \[`)
	reFence       = regexp.MustCompile("^\\s*(```|~~~)")
	reWord        = regexp.MustCompile(`[\pL\pN]`)
)

// AuthoredWords counts the words of a body its author wrote. It leaves out template lines, the stack
// map, headings, HTML comments, fenced code and the tool's attribution line. The measurement behind the
// band left out the template and the stack map the same way.
func AuthoredWords(body string, template map[string]bool) int {
	body = reHTMLComment.ReplaceAllString(body, "")
	count := 0
	fenced := false
	for _, line := range shell.SplitLines(body) {
		if reFence.MatchString(line) {
			fenced = !fenced
			continue
		}
		trimmed := strings.TrimSpace(line)
		if fenced || trimmed == "" || template[templateKey(line)] || reHeading.MatchString(line) ||
			reStackMap.MatchString(line) || reAttribution.MatchString(line) {
			continue
		}
		for _, field := range strings.Fields(trimmed) {
			if reWord.MatchString(field) {
				count++
			}
		}
	}
	return count
}

var (
	reTestsHeading = regexp.MustCompile(`(?i)^\s{0,3}(#{1,6}\s*|\*\*)(tests?|testing|how (this was|it was|to) test(ed)?)\b`)
	// reRanNarration is a line saying what ran and that it passed, which CI reports itself. A manual
	// drive or a migration against real data names what CI cannot do, and stays.
	reRanNarration = regexp.MustCompile(`(?i)\b(npm|pnpm|yarn|go|cargo|make|gradle|mvn|dotnet)\s+(run\s+)?(test|check|lint|vet)\b|\b(pytest|jest|vitest|mocha|rspec)\b|\b(all|unit|integration|e2e)\s+(tests|checks|suites?)\s+(pass|passed|are green|green)\b|\btests?\s+(pass|passed|are green)\b|\bCI\s+(is\s+)?(green|passes|passed)\b|✅|✔️|✔`)
)

// KindFindings reads a body as the kind names. A body over the band is one finding with the median
// beside it, and each line of a Tests section that narrates what ran is one.
func KindFindings(file, body, kind string, bands map[string]band, template map[string]bool) []Finding {
	var out []Finding
	b := bands[kind]
	if words := AuthoredWords(body, template); words > b.words {
		out = append(out, Finding{File: file, Line: 1, Check: checkOverBand,
			Text: fmt.Sprintf("%d authored words, over %d; the median is %d", words, b.words, b.median)})
	}
	inTests := false
	for number, line := range shell.SplitLines(body) {
		if reHeading.MatchString(line) || strings.HasPrefix(strings.TrimSpace(line), "**") {
			inTests = reTestsHeading.MatchString(line)
			continue
		}
		if inTests && reRanNarration.MatchString(line) {
			out = append(out, Finding{File: file, Line: number + 1, Check: checkTestsNarration,
				Text: strings.TrimSpace(line)})
		}
	}
	return out
}

package voicecheck

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
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
// block, so the house corpus of blocks never exercises them.
var kindChecks = []string{checkOverBand, checkTestsNarration}

// band is a kind's width: the authored words a body may carry, and the reference median printed beside
// a finding so an author reads the line and what it was drawn from together.
type band struct {
	words  int
	median int
}

// defaultBands are the figures `human-writing.md` -> Change descriptions (PRs) states. Fifty merged
// PRs of the host repository ran to a median of 48 authored words and a 90th percentile of 110, with
// the template's lines and the stack map left out. A ticket takes the PR figure until a body of
// tickets is counted. A `band` line in the conf moves either.
var defaultBands = map[string]band{
	KindPRBody: {words: 110, median: 48},
	KindTicket: {words: 110, median: 48},
}

// parseBandLine reads `band <kind> <words> <median> # <reason>`. The reason records what was counted,
// and a figure with none is refused the way an allow entry with none is.
func parseBandLine(rest string, number int) (string, band, error) {
	figures, reason, hasReason := strings.Cut(rest, " # ")
	fields := strings.Fields(figures)
	if len(fields) != 3 {
		return "", band{}, fmt.Errorf("line %d is not `band <kind> <words> <median> # <reason>`", number)
	}
	if _, known := defaultBands[fields[0]]; !known {
		return "", band{}, fmt.Errorf("line %d sets a band for %q, and the kinds are %s and %s", number, fields[0],
			KindPRBody, KindTicket)
	}
	words, errWords := strconv.Atoi(fields[1])
	median, errMedian := strconv.Atoi(fields[2])
	if errWords != nil || errMedian != nil || words < 1 || median < 1 {
		return "", band{}, fmt.Errorf("line %d sets a band that is not two positive counts", number)
	}
	if !hasReason || strings.TrimSpace(reason) == "" {
		return "", band{}, fmt.Errorf("line %d sets a band with no reason after ` # `; a figure with no reason is refused", number)
	}
	return fields[0], band{words: words, median: median}, nil
}

// confBands reads the conf's `band` lines over the defaults. No conf leaves the defaults standing.
func confBands(cwd string) (map[string]band, error) {
	out := map[string]band{}
	for kind, b := range defaultBands {
		out[kind] = b
	}
	path, _, found := voiceConfPath(cwd)
	if !found {
		return out, nil
	}
	body, err := readCapped(path, maxVoiceConfBytes)
	if err != nil {
		return nil, err
	}
	for number, raw := range shell.SplitLines(body) {
		keyword, rest, _ := strings.Cut(strings.TrimSpace(raw), " ")
		if keyword != "band" {
			continue
		}
		kind, b, err := parseBandLine(strings.TrimSpace(rest), number+1)
		if err != nil {
			return nil, err
		}
		out[kind] = b
	}
	return out, nil
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

// AuthoredWords counts the words of a body its author wrote. Template lines, the stack map, headings,
// HTML comments, fenced code and the tool's attribution line are left out, the way the measurement
// behind the band left the template and the stack map out.
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

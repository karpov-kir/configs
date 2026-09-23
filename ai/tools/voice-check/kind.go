package voicecheck

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"configs/ai/tools/shell"
)

// The kinds a prose text can be read as. A PR body and a ticket each come with a template, and `kind`
// names which one, so the checks read the author's sentences and leave the template's alone.
const (
	KindPRBody = "pr-body"
	KindTicket = "ticket"
)

// kinds is the set `--kind` accepts.
var kinds = map[string]bool{KindPRBody: true, KindTicket: true}

const checkTestsNarration = "tests-narration"

// kindChecks run only under `--kind`. They read a whole body, where the register checks read a comment
// block. The house corpus holds blocks, so these checks are tested on their own.
var kindChecks = []string{checkTestsNarration}

// templateGlobs are where a repository keeps each kind's template, relative to its root.
var templateGlobs = map[string][]string{
	KindPRBody: {".github/pull_request_template.md", ".github/PULL_REQUEST_TEMPLATE.md",
		".github/PULL_REQUEST_TEMPLATE/*.md", "docs/pull_request_template.md", "pull_request_template.md",
		"PULL_REQUEST_TEMPLATE.md"},
	KindTicket: {".github/ISSUE_TEMPLATE/*.md", ".github/ISSUE_TEMPLATE/*.yml", ".github/ISSUE_TEMPLATE/*.yaml",
		".github/issue_template.md", ".github/ISSUE_TEMPLATE.md", "issue_template.md"},
}

// templateLines is the repository's template for the kind, one line each as templateKey reads it. The
// author did not write those lines, so no check reads them as the author's.
func templateLines(root, kind string) map[string]bool {
	out := map[string]bool{}
	for _, pattern := range templateGlobs[kind] {
		matches, _ := filepath.Glob(shell.Join(root, pattern))
		for _, path := range matches {
			body, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			for _, line := range shell.SplitLines(string(body)) {
				if key := templateKey(line); key != "" {
					out[key] = true
				}
			}
		}
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
)

// AuthoredLines is the body with every line its author did not write blanked: the template's lines, a
// checkbox ticked or not, headings, the stack map, HTML comments, fenced code and a generated line. The
// blanks keep each line's number, so a finding names the line the author sees.
func AuthoredLines(body string, template map[string]bool) []string {
	body = reHTMLComment.ReplaceAllStringFunc(body, func(comment string) string {
		return strings.Repeat("\n", strings.Count(comment, "\n"))
	})
	lines := shell.SplitLines(body)
	out := make([]string, len(lines))
	fenced := false
	for i, line := range lines {
		if reFence.MatchString(line) {
			fenced = !fenced
			continue
		}
		if fenced || template[templateKey(line)] || reHeading.MatchString(line) || reCheckbox.MatchString(strings.TrimSpace(line)) ||
			reStackMap.MatchString(line) || reAttribution.MatchString(line) {
			continue
		}
		out[i] = line
	}
	return out
}

var (
	reTestsHeading = regexp.MustCompile(`(?i)^\s{0,3}(#{1,6}\s*|\*\*)(tests?|testing|how (this was|it was|to) test(ed)?)\b`)
	// reRanNarration is a line saying what ran and that it passed, which CI reports itself. A manual
	// drive or a migration against real data names what CI cannot do, and stays.
	reRanNarration = regexp.MustCompile(`(?i)\b(npm|pnpm|yarn|go|cargo|make|gradle|mvn|dotnet)\s+(run\s+)?(test|check|lint|vet)\b|\b(pytest|jest|vitest|mocha|rspec)\b|\b(all|unit|integration|e2e)\s+(tests|checks|suites?)\s+(pass|passed|are green|green)\b|\btests?\s+(pass|passed|are green)\b|\bCI\s+(is\s+)?(green|passes|passed)\b|✅|✔️|✔`)
)

// KindFindings reads a body as the kind names. Each authored line of a Tests section that narrates
// what ran is one finding. The body's length is none: a sentence goes by the sentence test, and
// `human-writing.md` -> Change descriptions (PRs) says what that test is.
func KindFindings(file string, raw, authored []string) []Finding {
	var out []Finding
	inTests := false
	for number, line := range raw {
		if reHeading.MatchString(line) || strings.HasPrefix(strings.TrimSpace(line), "**") {
			inTests = reTestsHeading.MatchString(line)
			continue
		}
		if inTests && number < len(authored) && authored[number] != "" && reRanNarration.MatchString(line) {
			out = append(out, Finding{File: file, Line: number + 1, Check: checkTestsNarration,
				Text: strings.TrimSpace(line)})
		}
	}
	return out
}

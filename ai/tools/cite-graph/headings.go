package main

import (
	"regexp"
	"sort"
	"strings"

	"kk-flavor/tools/shell"
)

var headingPattern = regexp.MustCompile(`^#{2,}\s+(.+?)\s*$`)

// Whether a cited name enters a real heading, by check.sh's rule rather than a stricter one of our
// own: prose runs on past the heading it names, so a citation naming a leading run of one resolves.
// `→ **Phase 2 — Assemble Context**` enters `## Phase 2 — Assemble Context (progressive)`.
//
// Matching stricter than check.sh is worse than matching wrong: two detectors disagreeing about what
// resolves is invisible until someone reads both.
func entersAHeading(headings map[string]bool, section string) (string, bool) {
	if headings[section] {
		return section, true
	}
	// Truncate the citation to find a heading, which is check.sh's direction. Extending the citation
	// to reach a longer heading is the inverse and accepts what check.sh refuses: `→ **Caller of a
	// skill**` against `## Caller`.
	//
	// Longest run first, so a citation naming a real heading never resolves to a shorter one.
	for cut := len(section); cut > 0; cut-- {
		if cut < len(section) && section[cut] != ' ' && section[cut] != '\t' {
			continue // a word boundary, so half a word cannot satisfy a citation
		}
		run := strings.TrimRight(section[:cut], " \t")
		if run == "" {
			continue
		}
		if headings[run] {
			return run, true
		}
		if heading, ok := headingByAlias(headings, run); ok {
			return heading, true
		}
	}
	return "", false
}

// The names a heading also answers to, tried in this order. Each returns empty for a heading the
// alias does not apply to, which never matches because the run asked about is never empty.
//
//   - The run before an em dash, for a heading carrying a subtitle: `**Budget**` for `## Budget —
//     the keep test`.
//   - The text of a numbered heading: `**What a suite reports**` for `## 7. What a suite reports`.
//     eco-check resolves that by registering the numberless form, so without it the two detectors
//     disagree about what resolves.
//   - The two composed, because eco-check takes the numberless of every form it registers, the
//     em-dash prefix included: `**Trigger**` resolves there against `## 1. Trigger — how it gets
//     invoked`. Matching each alone and not the pair leaves that disagreement standing on every
//     heading that is numbered AND carries a subtitle.
var headingAliases = []func(string) string{shell.BeforeEmDash, shell.WithoutLeadingNumber, numberlessBeforeEmDash}

// The heading a run reaches through one of those aliases. Returns the full heading, never the alias:
// keyed on the alias, the edge leaves the real heading reported UNENTERED while files enter it.
//
// Sorted, because two headings in one file can share ONE alias — `## Caller — the skill's` and
// `## Caller — the orchestrator's` both reduce to `Caller` under BeforeEmDash — and a bare `range`
// over the map then returns whichever the runtime visited first. The edge keys on the heading
// returned, so the same tree reports a different section entered, and a different UNENTERED list, on
// two runs of the same commit.
//
// Only a collision inside a *single* alias reaches the map's order: headingAliases is the outer loop,
// so headings colliding under different aliases are already separated by the order that list is
// written in.
//
// Sorting picks a stable winner rather than the right one — there is no right one, since both headings
// genuinely answer to the run. A tree wanting a particular one has two headings that need telling
// apart, which is check.sh's finding to report, not this tool's to guess at.
func headingByAlias(headings map[string]bool, run string) (string, bool) {
	sorted := make([]string, 0, len(headings))
	for heading := range headings {
		sorted = append(sorted, heading)
	}
	sort.Strings(sorted)
	for _, alias := range headingAliases {
		for _, heading := range sorted {
			if alias(heading) == run {
				return heading, true
			}
		}
	}
	return "", false
}

func numberlessBeforeEmDash(heading string) string {
	before := shell.BeforeEmDash(heading)
	if before == "" {
		return ""
	}
	return shell.WithoutLeadingNumber(before)
}

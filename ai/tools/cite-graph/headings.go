package main

import (
	"regexp"
	"sort"

	"kk-flavor/tools/shell"
)

var headingPattern = regexp.MustCompile(`^#{2,}\s+(.+?)\s*$`)

// Whether a cited name enters a real heading, by check.sh's rule rather than a looser or stricter one
// of our own: this tool only ever reads the delimited `→ **Section**` form, and a delimited citation
// names its heading whole. `→ **Caller of a skill**` enters nothing where the heading is `## Caller`,
// and neither does `→ **Phase 2 — Assemble Context**` against `## Phase 2 — Assemble Context
// (progressive)`. The aliases below are the only names accepted in place of the whole one, and they
// are the forms eco-check registers for the heading itself.
//
// Disagreeing with check.sh in either direction is worse than matching wrong: two detectors
// disagreeing about what resolves is invisible until someone reads both. This used to truncate the
// citation word by word, which resolved a name that ran past its heading — the paraphrase-by-extension
// eco-check now refuses.
func entersAHeading(headings map[string]bool, section string) (string, bool) {
	// Never an empty name: every alias answers empty for a heading it does not apply to, so an empty
	// run would enter whichever heading the sort visited first.
	if section == "" {
		return "", false
	}
	if headings[section] {
		return section, true
	}
	return headingByAlias(headings, section)
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

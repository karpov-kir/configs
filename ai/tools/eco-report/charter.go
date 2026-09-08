package ecoreport

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

const charterConstraintsBound = 30

var countedConstraint = regexp.MustCompile(`^[0-9]+x\s*\|`)

// The charter holds human-owned prose, not counted agent records. This check
// validates its shape; project-wide meaning and approval remain semantic judgments.
func charterLayoutFindings(path string) []string {
	label := fmt.Sprintf("%q", path)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return []string{label + ": charter must be a readable regular file"}
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return []string{label + ": charter could not be read"}
	}
	var findings []string
	sections, count := 0, 0
	inside, comment := false, false
	fence := ""
	for i, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			marker := strings.TrimLeft(trimmed, fence[:1])
			if len(trimmed)-len(marker) >= len(fence) && strings.TrimSpace(marker) == "" {
				fence = ""
			}
			continue
		}
		line = charterVisibleLine(line, &comment)
		trimmed = strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			width := len(trimmed) - len(strings.TrimLeft(trimmed, trimmed[:1]))
			fence = trimmed[:width]
			if inside {
				findings = append(findings, fmt.Sprintf("%s:%d: use a plain constraint bullet, not a code block", label, i+1))
			}
			continue
		}
		if strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "## ") {
			inside = strings.TrimSpace(line) == "## Constraints"
			if inside {
				sections++
			}
			continue
		}
		if !inside || trimmed == "" {
			continue
		}
		text := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		if !strings.HasPrefix(line, "- ") || text == "" || countedConstraint.MatchString(text) {
			findings = append(findings, fmt.Sprintf("%s:%d: each constraint must be one plain top-level '- ' bullet, without record metadata; keep rationale in agent decisions", label, i+1))
			continue
		}
		count++
	}
	if sections == 0 {
		findings = append(findings, label+": missing ## Constraints section")
	} else if sections > 1 {
		findings = append(findings, label+": duplicate ## Constraints sections")
	}
	if count > charterConstraintsBound {
		findings = append(findings, fmt.Sprintf("%s: %d constraints exceed the maximum of %d; ask the human to consolidate or narrow them", label, count, charterConstraintsBound))
	}
	return findings
}

func charterVisibleLine(line string, comment *bool) string {
	var visible strings.Builder
	for line != "" {
		if *comment {
			end := strings.Index(line, "-->")
			if end < 0 {
				break
			}
			line, *comment = line[end+3:], false
		} else {
			start := strings.Index(line, "<!--")
			if start < 0 {
				visible.WriteString(line)
				break
			}
			visible.WriteString(line[:start])
			line, *comment = line[start+4:], true
		}
	}
	return visible.String()
}

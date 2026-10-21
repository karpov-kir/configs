package ecoreport

import (
	"errors"
	"strings"
)

func (r *run) assertDecisionSections(kind *recordKind, path string, lines []string) {
	if kind.file != "decisions.md" || len(lines) == 0 {
		return
	}
	if err := decisionSectionsError(lines); err != nil {
		r.refuse("error: " + path + ": " + err.Error() + " — nothing was written.")
	}
}

func decisionSectionsError(lines []string) error {
	candidate, decisions := -1, -1
	for index, line := range lines {
		switch {
		case line == "## Promotion candidates" && candidate == -1 && decisions == -1:
			candidate = index
		case line == "## Decisions" && candidate >= 0 && decisions == -1:
			decisions = index
		case strings.HasPrefix(strings.TrimSpace(line), "##"):
			return errors.New("decision headings out of shape: the file holds `## Promotion candidates` then `## Decisions`, once each, and no other heading of any level")
		default:
			if _, isEntry := parseRecordEntry(index, line); isEntry && candidate == -1 {
				return errors.New("decision entry outside its sections")
			}
		}
	}
	if candidate == -1 || decisions == -1 {
		return errors.New("needs ## Promotion candidates followed by ## Decisions")
	}
	return nil
}

func (r *run) recordClassify(kind *recordKind, path, text, classification string) {
	handle, lines, _ := r.openLockedRecord(path, false)
	defer handle.Close()
	r.assertDecisionSections(kind, path, lines)
	found := r.oneMatchingEntry(path, lines, text, "classified")
	entryLine := lines[found.line]
	lines = append(lines[:found.line], lines[found.line+1:]...)
	insertion := len(lines)
	if classification == "candidate" {
		insertion = decisionsHeadingLine(lines)
	}
	lines = append(lines[:insertion], append([]string{entryLine}, lines[insertion:]...)...)
	r.overwriteRecord(path, handle, lines)
	r.line("classified in %s as %s: %s", kind.file, classification, found.quoted())
}

/** Returns the index of the `## Decisions` heading, or len(lines) when the record has none. */
func decisionsHeadingLine(lines []string) int {
	for index, line := range lines {
		if line == "## Decisions" {
			return index
		}
	}
	return len(lines)
}

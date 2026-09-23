package voicecheck

import (
	"fmt"
	"regexp"
	"strings"
)

// A note's record is three slots: the fact from outside this code, the identifier here it bears on,
// and what that identifier does because of it. The block writes the record as two plain sentences.
//
// Four blocks a reviewer read on 2026-09-22 stated a fact and stopped. Each record would have had an
// empty `does`. These checks read the record, since the writer's prose is already decided.
const (
	checkRecordSlot      = "record-slot-missing"
	checkRecordElsewhere = "bears-on-elsewhere"
	checkRecordUnnamed   = "block-omits-bears-on"
	checkRecordUntied    = "does-untied"
)

// recordMarker ends the record and opens the block. Everything above it is slots, everything under
// it is the block with the declaration it sits on and that declaration's body.
const recordMarker = "---"

// recordSlots is the slots a record carries, in the order a writer fills them.
var recordSlots = []string{"fact", "bears_on", "does"}

// noDoes is what `does` holds where the declaration under the block is one line. The value beneath a
// constant, a field or an enum member is itself the tie, so there is no act to name.
const noDoes = "none"

// identifierToken is a name as source spells it, which the segment split then cuts at the humps.
var identifierToken = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

var camelHump = regexp.MustCompile(`([a-z0-9])([A-Z])`)

// recordSegments is every segment the given text's identifiers spell, lower-cased, plus each
// identifier whole. Segments match whole, so `MediaSource` meets mediaSourceClaim, an identifier
// holding it, and never meets `sourced`.
func recordSegments(text string) map[string]bool {
	out := map[string]bool{}
	for _, token := range identifierToken.FindAllString(text, -1) {
		out[strings.ToLower(token)] = true
		for _, segment := range strings.Fields(camelHump.ReplaceAllString(token, "$1 $2")) {
			for _, part := range strings.Split(segment, "_") {
				if part != "" {
					out[strings.ToLower(part)] = true
				}
			}
		}
	}
	return out
}

// splitRecord cuts the piped lines at the marker. The second return is false where the text carries
// no marker. That caller passed `--record` and piped a block with no record above it.
func splitRecord(lines []string) (record, under []string, found bool) {
	for i, line := range lines {
		if strings.TrimSpace(line) == recordMarker {
			return lines[:i], lines[i+1:], true
		}
	}
	return nil, lines, false
}

// readSlots reads the slot lines. A line naming no slot is the writer's own prose and is passed over,
// so a record may carry a comment of its own.
func readSlots(lines []string) map[string]string {
	held := map[string]string{}
	for _, line := range lines {
		name, value, cut := strings.Cut(line, ":")
		if !cut {
			continue
		}
		name = strings.ToLower(strings.TrimSpace(name))
		for _, slot := range recordSlots {
			if name == slot {
				held[slot] = strings.TrimSpace(value)
			}
		}
	}
	return held
}

// commentLead is what a block's lines open with, stripped so the block's own words can be read.
var commentLead = regexp.MustCompile(`^\s*(//+|/\*+|\*+/?|#)\s?`)

// blockAndBody cuts the text under the marker into the block the writer wrote and the code it sits
// on. The block is the run of comment lines the text opens with, and everything after it is the
// declaration and its body.
func blockAndBody(lines []string) (block, body []string) {
	at := 0
	for at < len(lines) {
		trimmed := strings.TrimSpace(lines[at])
		if trimmed == "" && len(block) == 0 {
			at++
			continue
		}
		if !commentLead.MatchString(lines[at]) {
			break
		}
		block = append(block, commentLead.ReplaceAllString(lines[at], ""))
		at++
	}
	return block, lines[at:]
}

// reDataKeyword opens a declaration that holds values and performs no act: an enum, an interface or
// a type, whatever its length.
var reDataKeyword = regexp.MustCompile(`^\s*(export\s+)?(declare\s+)?(default\s+)?(const\s+enum|enum|interface|type)\b`)

// reValueLine is a constant, a variable or a field: a name, then a type or a value.
var reValueLine = regexp.MustCompile(`^\s*(export\s+)?((const|let|var|readonly|static|private|public|protected|declare)\s+)*[A-Za-z_$][\w$]*\??\s*[:=]`)

// dataDeclaration says the declaration under the block holds values and performs no act. That is an
// enum, an interface, a type, or a constant or field whose value is data, at any length. `does` may be
// `none` there, because the value beneath is the tie. Run 9 read an opening brace as a body, and five
// writers filled `does` on data to get past it.
func dataDeclaration(body []string) bool {
	for _, line := range body {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if reDataKeyword.MatchString(line) {
			return true
		}
		return reValueLine.MatchString(line) && !strings.Contains(line, "=>") &&
			!strings.Contains(line, "function")
	}
	return false
}

// RecordFindings reads a record against the block it produced and the source the block sits on. It
// reports what the record fails. The register checks read the prose.
func RecordFindings(file string, lines []string) []Finding {
	record, under, found := splitRecord(lines)
	if !found {
		return []Finding{{file, 1, checkRecordSlot,
			"the text carries no `---` line, so it holds a block and no record"}}
	}
	held := readSlots(record)
	block, body := blockAndBody(under)
	at := len(record) + 1

	var out []Finding
	for _, slot := range recordSlots {
		if held[slot] == "" {
			out = append(out, Finding{file, 1, checkRecordSlot, slot + ":"})
		}
	}
	bearsOn := held["bears_on"]
	if bearsOn == "" {
		return out
	}

	// The site's own identifier set is the declaration and the body under it. A name from anywhere
	// else is a caller, a sibling or another file's, and the fact it carries belongs where it is read.
	site := recordSegments(strings.Join(body, "\n"))
	if !spellsTheName(bearsOn, strings.Join(body, "\n")) {
		out = append(out, Finding{file, at, checkRecordElsewhere,
			fmt.Sprintf("%s is declared nowhere under this block", bearsOn)})
	}
	if !spellsTheName(bearsOn, strings.Join(block, "\n")) {
		out = append(out, Finding{file, at, checkRecordUnnamed,
			fmt.Sprintf("the block never says %s", bearsOn)})
	}

	does := held["does"]
	switch {
	case does == "":
	case strings.EqualFold(does, noDoes):
		if !dataDeclaration(body) {
			out = append(out, Finding{file, at, checkRecordSlot,
				"does: none, and the declaration under the block has a body, which shows what the code " +
					"does and cannot show that the fact is why"})
		}
	case !namesSomethingIn(does, site):
		out = append(out, Finding{file, at, checkRecordUntied,
			fmt.Sprintf("does: %s, and the body spells none of it", does)})
	}
	return out
}

// namesSomethingIn says the phrase shares one segment with the set. One segment is enough: a tie
// reading `takes the FairPlay path` names the branch by `Fairplay`, and `path` is the English around
// it.
func namesSomethingIn(phrase string, set map[string]bool) bool {
	for segment := range recordSegments(phrase) {
		if set[segment] {
			return true
		}
	}
	return false
}

// spellsTheName says the text holds the identifier whole. `bears_on` names one declaration, and a
// block saying "format" has said one hump of getFormatClaim, the declared name, and left the rest
// unsaid.
func spellsTheName(name, text string) bool {
	wanted := identifierToken.FindAllString(name, -1)
	if len(wanted) == 0 {
		return false
	}
	held := map[string]bool{}
	for _, token := range identifierToken.FindAllString(text, -1) {
		held[strings.ToLower(token)] = true
	}
	for _, token := range wanted {
		if !held[strings.ToLower(token)] {
			return false
		}
	}
	return true
}

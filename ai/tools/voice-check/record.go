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
	checkRecordSelfNamed = "block-names-its-declaration"
	checkValueThisOpens  = "value-block-opens-on-this"
	checkValueActor      = "value-as-actor"
	// A reviewer read a note stating a fact and stopping as having no point: "Stalls and what?". Its
	// record named the act every time, and the block never did.
	checkFactNoAct = "fact-with-no-act"
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
// on. The block is the first run of comment lines, and the code after it is the declaration and body.
// Code piped ahead of the block is context. Run 10 piped a `return {` there, the check read an empty
// block, and it refused twice a name the block spelled.
func blockAndBody(lines []string) (block, body []string) {
	at := 0
	for at < len(lines) && !commentLead.MatchString(lines[at]) {
		at++
	}
	if at == len(lines) {
		return nil, lines
	}
	for at < len(lines) {
		if !commentLead.MatchString(lines[at]) {
			break
		}
		block = append(block, commentLead.ReplaceAllString(lines[at], ""))
		at++
	}
	return block, lines[at:]
}

// reValueThisOpens is a block on a value declaration opening with the declaration as its subject.
var reValueThisOpens = regexp.MustCompile(`^\s*This (member|row|constant|field|enum|type)\b`)

// reValueActor is a value said to keep, drop or reject, which only code does.
var reValueActor = regexp.MustCompile(`(?i)\bthis (member|row|constant|field|enum|type|value)\s+(keeps|drops|rejects)\b`)

// declarationKeywords open a declaration before its name. statementKeywords open a line that declares
// no name.
var declarationKeywords = map[string]bool{"export": true, "default": true, "async": true, "function": true,
	"const": true, "let": true, "var": true, "class": true, "interface": true, "type": true, "enum": true,
	"readonly": true, "static": true, "private": true, "public": true, "protected": true, "declare": true,
	"abstract": true, "get": true, "set": true, "func": true, "def": true}
var statementKeywords = map[string]bool{"if": true, "else": true, "for": true, "while": true, "switch": true,
	"return": true, "case": true, "await": true, "throw": true, "try": true, "do": true, "new": true}

// reComputedName is the last name inside a computed key, such as Deferred in `[Clearing.Deferred]:`.
var reComputedName = regexp.MustCompile(`^\[[\w$.]*?([\w$]+)\]`)

// declaredName is the name the first code line under the block declares, or "" where that line is a
// statement.
func declaredName(body []string) string {
	for _, line := range body {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if m := reComputedName.FindStringSubmatch(trimmed); m != nil {
			return m[1]
		}
		for _, token := range identifierToken.FindAllString(trimmed, -1) {
			switch {
			case statementKeywords[token]:
				return ""
			case !declarationKeywords[token]:
				return token
			}
		}
		return ""
	}
	return ""
}

// reDataKeyword opens a declaration that holds values and performs no act: an enum, an interface or
// a type, whatever its length.
var reDataKeyword = regexp.MustCompile(`^\s*(export\s+)?(declare\s+)?(default\s+)?(const\s+enum|enum|interface|type)\b`)

// reValueLine is a constant, a variable or a field: a name, then a type or a value. A computed key,
// `[Clearing.Deferred]: undefined,`, is a field too. Run 13 read one as a declaration with a body, and
// nine catalogue rows written with `does: none` could not be kept.
var reValueLine = regexp.MustCompile(`^\s*(export\s+)?((const|let|var|readonly|static|private|public|protected|declare)\s+)*([A-Za-z_$][\w$]*|\[[\w$.]+\])\??\s*[:=]`)

// dataDeclaration says the declaration under the block holds values and performs no act. That is an
// enum, an interface, a type, or a constant or field whose value is data, at any length. Run 9 read an
// opening brace as a body, and five writers filled `does` on data to get past it.
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
	// A summary-only block's record says so in its summary and note lines, and reads clean. Run 26's
	// loop writer met a slot finding on one, then checked it with --source and without its record.
	if summaryOnly(record) {
		return nil
	}

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
	// A reviewer's eye jumped to each name of the declaration under a block in run 13 to check it was
	// the current one. A name another interface qualifies, such as `LedgerBook.SETTLED` over `SETTLED`,
	// is that interface's.
	blockText := strings.Join(block, "\n")
	own := declaredName(body) == bearsOn
	switch {
	case own && regexp.MustCompile(`(^|[^\w$.])`+regexp.QuoteMeta(bearsOn)+`($|[^\w$])`).MatchString(blockText):
		out = append(out, Finding{file, at, checkRecordSelfNamed,
			fmt.Sprintf("the block names %s, the declaration it sits on; say the domain thing or a role noun, or open on the verb", bearsOn)})
	case !own && !spellsTheName(bearsOn, blockText):
		out = append(out, Finding{file, at, checkRecordUnnamed,
			fmt.Sprintf("the block never says %s", bearsOn)})
	}

	// The reviewer of 2026-09-29 cut "This member" from a block, and read a constant said to keep part of
	// a type as the value acting.
	if dataDeclaration(body) && reValueThisOpens.MatchString(blockText) {
		out = append(out, Finding{file, at, checkValueThisOpens,
			"the block opens on this member, this row or this constant; open on the verb, as Names or Marks"})
	}
	if m := reValueActor.FindString(blockText); m != "" {
		out = append(out, Finding{file, at, checkValueActor,
			fmt.Sprintf("%q makes a value the actor; say what the code does to the thing, or who is asked", m)})
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
	if does != "" && !strings.EqualFold(does, noDoes) && !dataDeclaration(body) && !noteStatesAnAct(blockText, declaredName(body)) {
		out = append(out, Finding{file, at, checkFactNoAct,
			fmt.Sprintf("the record's act is %q, and the note states the fact alone; say what this code does about it, and what that gives its caller", does)})
	}
	return out
}

var (
	// reSummaryOpen opens a summary, the sentence saying what a declaration does: the note is the rest.
	reSummaryOpen = regexp.MustCompile(`^(?:Tells|Returns|Lists|Checks|Throws)\b`)
	// An act shows in a note by a connector to the fact, a sentence opening on its verb, or this code as
	// a subject. "The" or "its" before a noun for code counts only at a sentence's start. Mid-sentence,
	// as in "when the request carries a mode", the noun is part of a fact.
	reActConnector = regexp.MustCompile(`(?i)\b(?:so|because|therefore|which is why|that is why|for that reason|since)\b`)
	// `or so` hedges a claim and joins no fact to an act.
	reHedgeSo = regexp.MustCompile(`(?i)\bor so\b`)
	// A capitalised word ending in s is a plural subject where a relative or a verb follows it.
	reActVerbFirst = regexp.MustCompile(`^(?:[A-Z][a-z]+s)\s+(\w+)`)
	rePluralNext   = regexp.MustCompile(`^(?:that|which|who|whose|of|in|on|with|from|for|are|were|have|had|do|did|can|may|must|will|would|should|could)$`)
	// A sentence naming code, a caller or a file, tells what that code does: the rule lets a note name a
	// caller's act.
	reActNamesCode = regexp.MustCompile("`[^`]+`|\\b[a-z]+[A-Z]\\w*\\b")
	reActThisCode  = regexp.MustCompile(`\b[Tt]his\s+(?:code|function|method|call|check|helper|class|wrapper|copy|branch|loop|guard|test|module|hook|filter|declaration|constructor|getter|setter|handler|callback|case|row|entry|walk|probe|request)\b|^(?:The|Its)\s+(?:code|function|method|call|check|helper|class|wrapper|copy|branch|loop|guard|test|module|hook|filter|declaration|constructor|getter|setter|handler|callback|case|row|entry|walk|probe|request)\b|\b[Tt]his\s+(?:name|value|key|result|string|id)\b`)
	reDeclaredWord = func(name string) *regexp.Regexp {
		return regexp.MustCompile(`(?:^|[^\w$])` + regexp.QuoteMeta(name) + `(?:[^\w$]|$)`)
	}
)

// noteStatesAnAct says a block's note, its sentences after any summary, states what the code does.
func noteStatesAnAct(block, declared string) bool {
	var words []string
	for _, line := range strings.Split(block, "\n") {
		if text := strings.TrimSpace(commentLineText(line)); text != "" {
			words = append(words, text)
		}
	}
	sentences := regexp.MustCompile(`[.!?]\s+`).Split(strings.Join(words, " "), -1)
	if len(sentences) > 0 && reSummaryOpen.MatchString(sentences[0]) {
		sentences = sentences[1:]
	}
	if len(sentences) == 0 || strings.TrimSpace(strings.Join(sentences, "")) == "" {
		return true
	}
	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if reActConnector.MatchString(reHedgeSo.ReplaceAllString(sentence, "")) || verbFirst(sentence) || reActThisCode.MatchString(sentence) ||
			reActNamesCode.MatchString(sentence) || (declared != "" && reDeclaredWord(declared).MatchString(sentence)) {
			return true
		}
	}
	return false
}

// verbFirst says a sentence opens on a verb in the third person: a capitalised word ending in s that
// no relative, preposition or plural verb follows.
func verbFirst(sentence string) bool {
	m := reActVerbFirst.FindStringSubmatch(sentence)
	return m != nil && !strings.HasPrefix(m[0], "This ") && !strings.HasPrefix(m[0], "Its ") && !rePluralNext.MatchString(m[1])
}

// commentLineText is a comment line with its markers taken off.
func commentLineText(line string) string {
	return regexp.MustCompile(`^\s*(?:/\*\*|\*/|\*|//|#)\s?`).ReplaceAllString(line, "")
}

// namesSomethingIn says the phrase shares one segment with the set. One segment is enough: a tie
// reading `takes the Deferred path` names the branch by `Deferred`, and `path` is the English around
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

// summaryOnly says the record is a summary-only block's: `note: none` with `summary: written`, and no
// fact.
func summaryOnly(record []string) bool {
	parts := map[string]string{}
	for _, line := range record {
		if name, value, cut := strings.Cut(line, ":"); cut {
			parts[strings.ToLower(strings.TrimSpace(name))] = strings.ToLower(strings.TrimSpace(value))
		}
	}
	return parts["note"] == "none" && parts["summary"] == "written" && (parts["fact"] == "" || parts["fact"] == "none")
}

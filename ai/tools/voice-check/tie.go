package voicecheck

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Within one file a connector, or a "this <noun>" subject, stands in at most three blocks. A reviewer
// read 72 blocks on 2026-09-29, 27 tied with "so" and 25 with "this function". He counted each apart.

// The bound reads the file the writer holds at write time, and the keep criteria leave it out.
const checkTieRepeated = "tie-opening-repeated"

// tieBound is how many other blocks of one file may already use a connector or a subject.
const tieBound = 2

// reConnector is a connector that claims the act follows from the fact.
var reConnector = regexp.MustCompile(`\b(which is why|that is why|for that reason|therefore|because|so)\b`)

// reThisSubject is a declaration or a role noun said as "this <noun>".
var reThisSubject = regexp.MustCompile(`\bthis (function|method|member|property|call|branch|check|lookup|filter|probe|guard|helper|match|copy|test|loop|constant|field|row|enum|type)\b`)

// tieOpenings is each connector and each "this <noun>" subject in the text, lower-cased.
func tieOpenings(text string) map[string]bool {
	text = strings.ToLower(text)
	out := map[string]bool{}
	for _, m := range reConnector.FindAllString(text, -1) {
		out[m] = true
	}
	for _, m := range reThisSubject.FindAllString(text, -1) {
		out[m] = true
	}
	return out
}

// fileBlocks is each run of comment lines in a file, with the comment leads stripped.
func fileBlocks(lines []string) []string {
	var out []string
	var held []string
	for _, line := range append(lines, "") {
		if commentLead.MatchString(line) {
			held = append(held, commentLead.ReplaceAllString(line, ""))
			continue
		}
		if len(held) > 0 {
			out = append(out, strings.Join(held, "\n"))
			held = nil
		}
	}
	return out
}

// TieFindings reports each tie opening of the block that stands in two other blocks of the file
// already. A block the file already holds word for word is the block itself, and it is not counted.
func TieFindings(file string, block []string, fileLines []string) []Finding {
	own := strings.TrimSpace(strings.Join(block, "\n"))
	counts := map[string]int{}
	for _, other := range fileBlocks(fileLines) {
		if strings.TrimSpace(other) == own {
			continue
		}
		for opening := range tieOpenings(other) {
			counts[opening]++
		}
	}
	var repeated []string
	for opening := range tieOpenings(own) {
		if counts[opening] >= tieBound {
			repeated = append(repeated, opening)
		}
	}
	sort.Strings(repeated)
	var out []Finding
	for _, opening := range repeated {
		out = append(out, Finding{file, 1, checkTieRepeated,
			fmt.Sprintf("%q opens the tie in %d other blocks of this file; vary the connector or the subject", opening, counts[opening])})
	}
	return out
}

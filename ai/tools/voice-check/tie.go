package voicecheck

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// A reviewer read 72 blocks on 2026-09-29, 27 tied with "so" and 25 with "this function", and counted
// the connector and the subject apart.

const checkTieRepeated = "tie-opening-repeated"

const maxBlocksPerTieOpening = 2

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
	for block, rest := blockAndBody(lines); block != nil; block, rest = blockAndBody(rest) {
		out = append(out, strings.Join(block, "\n"))
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
		if counts[opening] >= maxBlocksPerTieOpening {
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

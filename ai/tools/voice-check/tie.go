package voicecheck

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Within one file no tie opening, a connector with its subject, stands in more than two blocks. A
// reviewer read 72 blocks on 2026-09-29, 27 of them tied with "so" and 25 with "this function", and
// the fixed form written 72 times was what he read. The bound reads the file the writer holds at write
// time. The keep criteria leave it out, so a kept block stays kept whatever a neighbour says.
const checkTieRepeated = "tie-opening-repeated"

// tieBound is how many other blocks of one file may already open a tie the same way.
const tieBound = 2

// reConnector is a connector that claims the act follows from the fact. `therefore` may follow its
// subject, as in "this check therefore tells", and every other connector opens its clause.
var reConnector = regexp.MustCompile(`\b(which is why|that is why|for that reason|therefore|because|so)\b`)

var reTieWord = regexp.MustCompile(`[a-z0-9_$]+`)

// tieOpenings is each connector in the text with the two words of its subject, lower-cased: "so this
// function", or "therefore this check" for a subject standing before the connector.
func tieOpenings(text string) map[string]bool {
	text = strings.ToLower(text)
	out := map[string]bool{}
	for _, at := range reConnector.FindAllStringSubmatchIndex(text, -1) {
		connector := text[at[2]:at[3]]
		before := strings.TrimRight(text[:at[0]], " \t\n")
		var subject []string
		if connector == "therefore" && before != "" && !strings.HasSuffix(before, ",") && !strings.HasSuffix(before, ".") {
			words := reTieWord.FindAllString(before[strings.LastIndexAny(before, ".,;:")+1:], -1)
			if len(words) > 2 {
				words = words[len(words)-2:]
			}
			subject = words
		} else {
			words := reTieWord.FindAllString(text[at[1]:], 2)
			subject = words
		}
		out[strings.TrimSpace(connector+" "+strings.Join(subject, " "))] = true
	}
	return out
}

// TieFindings reports each tie opening of the block that stands in two blocks above its declaration
// already. The bound runs in file order. The third and later
// block with one opening answer for the repeat, and that choice is fixed. A declaration the file does not hold yet puts the block under every block.
func TieFindings(file string, block, body, fileLines []string) []Finding {
	at := len(fileLines) + 1
	for _, line := range body {
		if strings.TrimSpace(line) == "" {
			continue
		}
		for n, held := range fileLines {
			if strings.TrimSpace(held) == strings.TrimSpace(line) {
				at = n + 1
				break
			}
		}
		break
	}
	return tieFindingsAbove(file, block, fileLines, at)
}

// fileBlocks is each run of comment lines in a file with its first line, the comment leads stripped.
func fileBlocks(lines []string) (blocks []string, starts []int) {
	var held []string
	start := 0
	for n, line := range append(lines, "") {
		if commentLead.MatchString(line) {
			if len(held) == 0 {
				start = n + 1
			}
			held = append(held, commentLead.ReplaceAllString(line, ""))
			continue
		}
		if len(held) > 0 {
			blocks = append(blocks, strings.Join(held, "\n"))
			starts = append(starts, start)
			held = nil
		}
	}
	return blocks, starts
}

// tieFindingsAbove counts the tie openings of each block starting before line `at`. It reports each
// opening of the block that two of those blocks use already.
func tieFindingsAbove(file string, block, fileLines []string, at int) []Finding {
	own := strings.TrimSpace(strings.Join(block, "\n"))
	counts := map[string]int{}
	blocks, starts := fileBlocks(fileLines)
	for i, other := range blocks {
		if starts[i] >= at || strings.TrimSpace(other) == own {
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
			fmt.Sprintf("%q opens the tie in %d blocks above this one; vary the connector or the subject", opening, counts[opening])})
	}
	return out
}

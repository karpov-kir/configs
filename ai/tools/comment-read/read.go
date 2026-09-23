// Package commentread measures the blind reader that reads one comment block with the line under it
// and answers what that line does because of the block, or `cannot say`.
package commentread

import (
	"fmt"
	"regexp"
	"strings"
)

// Answer is one reader's return. CannotSay is the finding a writer gets back, and Said is the
// reader's sentence either way.
type Answer struct {
	CannotSay bool
	Said      string
	// Parsed is false where the return matched neither line the brief asks for. Such a roll is a
	// reader that did not answer, and it counts toward no verdict.
	Parsed bool
}

var readsLine = regexp.MustCompile(`(?im)^\s*reads:\s*(.+?)\s*$`)
var cannotLine = regexp.MustCompile(`(?im)^\s*cannot say\b:?\s*(.*?)\s*$`)

// Parse reads a reader's return. A return carrying both lines is read as `cannot say`, the finding
// that costs a rewrite, since a reader unsure enough to write it has not restated the block.
func Parse(raw string) Answer {
	if m := cannotLine.FindStringSubmatch(raw); m != nil {
		return Answer{CannotSay: true, Said: m[1], Parsed: true}
	}
	if m := readsLine.FindStringSubmatch(raw); m != nil {
		return Answer{Said: m[1], Parsed: true}
	}
	return Answer{}
}

// Site is what the reader is given: a block and the line of code under it.
type Site struct {
	Name  string
	Block string
	Line  string
}

// Prompt puts the brief above the site. The block and the line are fenced apart so a reader cannot
// take a word of the code for a word of the note.
func Prompt(brief string, s Site) string {
	return fmt.Sprintf("%s\n\n--- block\n%s\n--- line under it\n%s\n", strings.TrimSpace(brief),
		strings.TrimSpace(s.Block), strings.TrimSpace(s.Line))
}

// ParseSites reads the labelled file: each site opens on `=== <name>`, its block follows, and a
// `--- line` marker puts the line of code under it.
func ParseSites(raw string) ([]Site, error) {
	var out []Site
	for _, chunk := range strings.Split(raw, "\n=== ")[1:] {
		name, body, _ := strings.Cut(chunk, "\n")
		block, line, found := strings.Cut(body, "\n--- line\n")
		if !found {
			return nil, fmt.Errorf("site %q holds no `--- line` marker", strings.TrimSpace(name))
		}
		site := Site{Name: strings.TrimSpace(name), Block: strings.TrimSpace(block), Line: strings.TrimSpace(line)}
		if site.Block == "" || site.Line == "" || strings.Contains(site.Line, "\n") {
			return nil, fmt.Errorf("site %q needs a block and exactly one line under it", site.Name)
		}
		out = append(out, site)
	}
	return out, nil
}

// Verdict is the majority over rolls that answered. A tie is `cannot say`, the side that sends the
// block back, because the bar asks the reader to fail every flagged block and a tie has not passed it.
func Verdict(answers []Answer) (cannot bool, counted int) {
	yes := 0
	for _, a := range answers {
		if !a.Parsed {
			continue
		}
		counted++
		if a.CannotSay {
			yes++
		}
	}
	return counted > 0 && yes*2 >= counted, counted
}

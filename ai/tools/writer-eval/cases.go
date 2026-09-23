package writereval

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Case is one labelled site: the code as the writer sees it, the facts file the strip left beside
// it, and the class the label expects. The code carries no comment block, because the strip removes
// every block before the writer reads a file.
type Case struct {
	Name   string
	Expect Expected
	Why    string
	Label  string
	Code   string
	Facts  string
	// Callers is the call sites a grep over the repository finds, each under the path of the file it
	// sits in. Question 1 reads an exported symbol's callers, and question 3's consequence names what
	// a caller does to the site's result. A writer shown no callers reads every caller as hypothetical
	// and leaves the consequence out, which is what a run did on 2026-09-21 at a site with two.
	Callers string
	// Tests is what the change set's tests hold about this site. Question 3 greps them for a fact
	// before it keeps a claim. A `carried by <test>` label needs them. Without them the writer greps
	// an empty set and keeps the claim, which the text in front of it asks for.
	Tests string
	// WantSummary and WantNote are what a case expects of each part, where it cares. An empty field
	// leaves the case scored on the site alone.
	WantSummary string
	WantNote    string
	// Keeps is the wording the block has to carry: one group per `keeps:` line, alternatives inside a
	// group, and every group has to be met. One group asks for one thing. Run 7 read a block that
	// named a caller's act and left the site unnamed, and the case there asked for the caller alone
	// and passed it on every roll.
	Keeps [][]string
	// Site is the line the strip offered, and 1 where a case leaves it unsaid. Lands is the identifier
	// the block has to sit on. The two differ often: a strip offers the declaration the archive
	// recorded, and the fact is about something further down the file. Three of four blocks a reviewer
	// sent back on 2026-09-22 sat where the archive had put them.
	Site  int
	Lands string
	// Bars is the wording the block may not carry, as alternatives of which none may appear. It holds a
	// shape a review already rejected at this site. A rule the writer reads can be rewritten, and a
	// rewrite that reverses the reason for an earlier fix would let the rejected shape back without
	// anything noticing. The case is what notices.
	Bars []string
	// Floor overrides the share of rolls this case has to clear, in whole percent. A case no step
	// decides is the writer judging, and three mechanisms have failed to reach the ones carrying it.
	//
	// It is a share and no longer a count. Floors were counts written against five rolls, so each of
	// them grew three times weaker the day the roll count went to fifteen, and the table stayed quiet.
	Floor int
	// Returns is the fates the return has to name, and Withholds the fates it may not. A fact that fits
	// no note is routed somewhere, and where it went is the thing a case about routing scores. Run 7
	// routed 704 words of facts about the world into a PR body, where the change is described.
	Returns   []string
	Withholds []string
}

// ExpectCarried is a site whose claim belongs somewhere else in the tree: a field on the data row it
// describes, a test, a lint rule. The writer returns where it goes and writes no block.
const ExpectCarried Expected = "carried"

// ExpectRename is a site whose own identifier carries a coined compound. The writer returns the
// rename and leaves the word out of its prose, so the refactor lane can take it.
const ExpectRename Expected = "rename"

var expectedClasses = map[string]Expected{
	string(ExpectNone):    ExpectNone,
	string(ExpectWritten): ExpectWritten,
	string(ExpectRename):  ExpectRename,
	string(ExpectCarried): ExpectCarried,
}

const codeSection = "--- code"
const factsSection = "--- facts"
const testsSection = "--- tests"
const callersSection = "--- callers"

// splitSections cuts a case body at its marker lines. The first piece belongs to the section the
// caller names, and every marker after it opens the next.
func splitSections(first, body string) map[string]string {
	out := map[string]string{}
	name, held := first, []string(nil)
	for _, line := range strings.Split(body, "\n") {
		if trimmed := strings.TrimRight(line, " \t"); strings.HasPrefix(trimmed, "--- ") {
			out[name] = strings.Join(held, "\n")
			name, held = trimmed, nil
			continue
		}
		held = append(held, line)
	}
	out[name] = strings.Join(held, "\n")
	return out
}

// ParseCase reads one case file. A header line names a field, and the two sections carry the text.
func ParseCase(name, raw string) (Case, error) {
	c := Case{Name: name}
	head, rest, found := strings.Cut(raw, codeSection+"\n")
	if !found {
		return c, fmt.Errorf("%s holds no %q section", name, codeSection)
	}
	sections := splitSections(codeSection, rest)
	for marker := range sections {
		if marker != codeSection && marker != factsSection && marker != testsSection && marker != callersSection {
			return c, fmt.Errorf("%s names an unknown section %q", name, marker)
		}
	}
	if _, found := sections[factsSection]; !found {
		return c, fmt.Errorf("%s holds no %q section", name, factsSection)
	}
	c.Code = strings.TrimSpace(sections[codeSection])
	c.Facts = strings.TrimSpace(sections[factsSection])
	c.Tests = strings.TrimSpace(sections[testsSection])
	c.Callers = strings.TrimSpace(sections[callersSection])
	if c.Code == "" {
		return c, fmt.Errorf("%s shows the writer no code", name)
	}
	for _, line := range strings.Split(strings.TrimSpace(head), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return c, fmt.Errorf("%s has a header line that names no field: %q", name, line)
		}
		value = strings.TrimSpace(value)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "expect":
			class, known := expectedClasses[strings.ToLower(value)]
			if !known {
				return c, fmt.Errorf("%s expects %q, and the classes are none, written, rename and carried",
					name, value)
			}
			c.Expect = class
		case "why":
			c.Why = value
		case "labelled":
			c.Label = value
		case "summary":
			c.WantSummary = strings.ToLower(value)
		case "note":
			c.WantNote = strings.ToLower(value)
		case "keeps":
			var group []string
			for _, wording := range strings.Split(value, "|") {
				if trimmed := strings.TrimSpace(wording); trimmed != "" {
					group = append(group, strings.ToLower(trimmed))
				}
			}
			if len(group) == 0 {
				return c, fmt.Errorf("%s names no wording to keep", name)
			}
			c.Keeps = append(c.Keeps, group)
		case "site":
			at, err := strconv.Atoi(value)
			if err != nil || at < 1 {
				return c, fmt.Errorf("%s names a site of %q, which is not a line", name, value)
			}
			c.Site = at
		case "lands":
			c.Lands = value
		case "bars":
			for _, wording := range strings.Split(value, "|") {
				if trimmed := strings.TrimSpace(wording); trimmed != "" {
					c.Bars = append(c.Bars, strings.ToLower(trimmed))
				}
			}
			if len(c.Bars) == 0 {
				return c, fmt.Errorf("%s names no wording to bar", name)
			}
		case "returns", "withholds":
			var fates []string
			for _, fate := range strings.Split(value, "|") {
				if trimmed := strings.ToLower(strings.TrimSpace(fate)); trimmed != "" {
					fates = append(fates, trimmed)
				}
			}
			if len(fates) == 0 {
				return c, fmt.Errorf("%s names no fate under %s", name, key)
			}
			if strings.EqualFold(strings.TrimSpace(key), "returns") {
				c.Returns = append(c.Returns, fates...)
			} else {
				c.Withholds = append(c.Withholds, fates...)
			}
		case "floor":
			share, err := strconv.Atoi(strings.TrimSuffix(value, "%"))
			if err != nil || share < 1 || share > 100 {
				return c, fmt.Errorf("%s names a floor of %q, which is not a share between 1 and 100", name, value)
			}
			c.Floor = share
		default:
			return c, fmt.Errorf("%s names an unknown field %q", name, key)
		}
	}
	if c.Expect == "" {
		return c, fmt.Errorf("%s names no expected class", name)
	}
	if c.Site == 0 {
		c.Site = 1
	}
	if c.Lands != "" && !strings.Contains(c.Code, c.Lands) {
		return c, fmt.Errorf("%s asks the block to land on %q, which the code does not hold", name, c.Lands)
	}
	if c.Why == "" {
		return c, fmt.Errorf("%s says no reason, and a case nobody can read is a case nobody can re-cut", name)
	}
	return c, nil
}

// LoadCases reads every case in a directory, in name order so a run reports them the same way twice.
func LoadCases(dir string) ([]Case, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".case") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, fmt.Errorf("%s holds no case file", dir)
	}
	var cases []Case
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		c, err := ParseCase(strings.TrimSuffix(name, ".case"), string(raw))
		if err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	return cases, nil
}

var renameLine = strings.ToLower("rename:")

// ClassOf reads which class a return landed in. A rename is named by its own line, because the
// writer answering a coined identifier has a third thing to say beyond a block and none.
func ClassOf(r Return) Expected {
	for _, line := range strings.Split(r.Block, "\n") {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), renameLine) {
			return ExpectRename
		}
	}
	if r.Carried && r.Block == "" {
		return ExpectCarried
	}
	if r.None {
		return ExpectNone
	}
	return ExpectWritten
}

// JudgeCase scores a return against a case, including what the case expects of each part. Where a
// case sets neither part expectation, it is scored on the site alone, as it was before the split.
func JudgeCase(c Case, r Return) Verdict {
	v := Judge(c.Name, c.Expect, r)
	want := func(field, label string, got Part) {
		if field == "" {
			return
		}
		if PartName(got) != field {
			v.Failures = append(v.Failures, Failure{label + "-part", "wanted " + field + ", got " + PartName(got)})
		}
	}
	want(c.WantSummary, "summary", r.Summary)
	want(c.WantNote, "note", r.Note)
	// A block the writer never wrote fails on its part already, and reporting the wording too would
	// count one miss twice.
	if r.Block != "" {
		text := strings.ToLower(r.Text())
		for _, group := range c.Keeps {
			kept := false
			for _, wording := range group {
				kept = kept || strings.Contains(text, wording)
			}
			if !kept {
				v.Failures = append(v.Failures, Failure{"dropped-the-obligation",
					"the block keeps none of: " + strings.Join(group, ", ")})
			}
		}
	}
	// A block the writer never wrote fails on its part already.
	if c.Lands != "" && r.Block != "" {
		if at := DeclaredAt(c.Code, c.Lands); at == 0 {
			v.Failures = append(v.Failures, Failure{"case-names-no-such-declaration", c.Lands})
		} else if r.At != at {
			v.Failures = append(v.Failures, Failure{"wrote-it-at-the-wrong-declaration",
				fmt.Sprintf("line %d, and %s is declared on line %d", r.At, c.Lands, at)})
		}
	}
	for _, wording := range c.Bars {
		if r.Block != "" && strings.Contains(strings.ToLower(r.Text()), wording) {
			v.Failures = append(v.Failures, Failure{"wrote-the-barred-shape", "the block carries " + wording})
		}
	}
	routed := func(fate string) bool {
		for _, got := range r.Routed {
			if strings.HasPrefix(got, fate) {
				return true
			}
		}
		return false
	}
	for _, fate := range c.Returns {
		if !routed(fate) {
			v.Failures = append(v.Failures, Failure{"routed-it-elsewhere", "no " + fate + " line"})
		}
	}
	for _, fate := range c.Withholds {
		if routed(fate) {
			v.Failures = append(v.Failures, Failure{"routed-to-a-withheld-fate", fate})
		}
	}
	return v
}

// DeclaredAt is the line of the first declaration holding `what`, or zero where the code holds none.
// A case names the declaration its block belongs on, because an edit anywhere earlier in the fixture
// moves a line number.
func DeclaredAt(code, what string) int {
	for n, line := range strings.Split(code, "\n") {
		if strings.Contains(line, what) {
			return n + 1
		}
	}
	return 0
}

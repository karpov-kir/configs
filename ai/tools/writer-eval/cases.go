package writereval

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	// Tests is what the change set's tests hold about this site. Question 3 greps them for a fact
	// before it keeps a claim. A `carried by <test>` label needs them. Without them the writer greps
	// an empty set and keeps the claim, which the text in front of it asks for.
	Tests string
	// WantSummary and WantNote are what a case expects of each part, where it cares. Empty means the
	// case is scored on the site alone.
	WantSummary string
	WantNote    string
}

// ExpectRename is a site whose own identifier carries a coined compound. The writer returns the
// rename and leaves the word out of its prose, so the refactor lane can take it.
const ExpectRename Expected = "rename"

var expectedClasses = map[string]Expected{
	string(ExpectNone):    ExpectNone,
	string(ExpectWritten): ExpectWritten,
	string(ExpectRename):  ExpectRename,
}

const codeSection = "--- code"
const factsSection = "--- facts"
const testsSection = "--- tests"

// ParseCase reads one case file. A header line names a field, and the two sections carry the text.
func ParseCase(name, raw string) (Case, error) {
	c := Case{Name: name}
	head, rest, found := strings.Cut(raw, codeSection+"\n")
	if !found {
		return c, fmt.Errorf("%s holds no %q section", name, codeSection)
	}
	code, facts, found := strings.Cut(rest, factsSection+"\n")
	if !found {
		return c, fmt.Errorf("%s holds no %q section", name, factsSection)
	}
	facts, tests, _ := strings.Cut(facts, testsSection+"\n")
	c.Code = strings.TrimSpace(code)
	c.Facts = strings.TrimSpace(facts)
	c.Tests = strings.TrimSpace(tests)
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
				return c, fmt.Errorf("%s expects %q, and the classes are none, written and rename", name, value)
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
		default:
			return c, fmt.Errorf("%s names an unknown field %q", name, key)
		}
	}
	if c.Expect == "" {
		return c, fmt.Errorf("%s names no expected class", name)
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
	if r.None {
		return ExpectNone
	}
	return ExpectWritten
}

// JudgeCase scores a return against a case, including what the case expects of each part. A case
// that names no part expectation is scored on the site alone, the way it was before the parts split.
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
	return v
}

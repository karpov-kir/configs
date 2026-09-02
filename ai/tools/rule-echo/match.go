package main

// What two bolded rules are to each other. The words that discriminate one rule from another, the
// files a rule's citations name, and the verdict a pair earns from the two. The walk that finds the
// rules and the report that prints them are main.go beside this file.

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The discriminating words a span needs before a match means anything, and the same floor the
// dependency test below applies to what is left after the names are removed. One constant because the
// two are compared against each other: drifted apart, a pair could clear the match and fail the test
// for a reason no reader could see.
const minDiscriminatingWords = 4

// Words carrying no discrimination. A rule pair sharing only these shares nothing.
var common = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "of": true, "to": true, "in": true,
	"is": true, "it": true, "that": true, "this": true, "for": true, "on": true, "as": true,
	"with": true, "by": true, "not": true, "never": true, "you": true, "your": true, "its": true,
	"be": true, "are": true, "was": true, "one": true, "at": true, "from": true, "what": true,
	"which": true, "when": true, "where": true, "who": true, "so": true, "but": true, "than": true,
}

var wordPattern = regexp.MustCompile(`[a-z0-9][a-z0-9-]*`)

func keyOf(text string) map[string]bool {
	key := map[string]bool{}
	for _, w := range wordPattern.FindAllString(strings.ToLower(text), -1) {
		if !common[w] && len(w) > 2 {
			key[w] = true
		}
	}
	return key
}

var backticked = regexp.MustCompile("`[^`]*`")

// The words a span carries only because it names something in backticks — a path, a command, a file.
// Two consumers declaring the same dependency at their own point of use share all of this vocabulary
// without stating the same rule, and cutting either leaves that file not naming what it runs.
func namedWords(text string) map[string]bool {
	named := map[string]bool{}
	for _, quoted := range backticked.FindAllString(text, -1) {
		for w := range keyOf(quoted) {
			named[w] = true
		}
	}
	return named
}

var mdLinkTarget = regexp.MustCompile(`\]\(([^)]*)\)`)

// The markdown files one line points at, written the way the line wrote them. A rule that cites the
// file owning it is a pointer, and a pointer cannot avoid the vocabulary of what it points at — you
// cannot cite a rule about resolved scratch roots without saying "resolved scratch root". namedWords
// cannot strip that overlap, because the rule's subject is not its name. Left unread, the tree's own
// compliance is reported as its duplication. That is the failure the `→` filter in collect already
// prevents, and the filter reaches only a citation written before the bold text, never the ones after.
//
// Both forms this tree cites in: a backticked path, and a markdown link target.
func citedTargets(line string) []string {
	var targets []string
	add := func(target string) {
		target = strings.TrimSpace(target)
		if strings.HasSuffix(target, ".md") {
			targets = append(targets, target)
		}
	}
	for _, quoted := range backticked.FindAllString(line, -1) {
		add(strings.Trim(quoted, "`"))
	}
	for _, m := range mdLinkTarget.FindAllStringSubmatch(line, -1) {
		add(m[1])
	}
	return targets
}

// Which walked file a citation names. Built from the tree the walk actually read, because that file
// set is the only thing that can answer it: this tree is walked as `ai/skills/x/SKILL.md` and cited
// as `~/.claude/skills/x/SKILL.md`, so neither the whole path nor the base name decides it.
//
// `cite-graph`'s nameResolver answers the same question over the same tree, and `tails` below holds
// the rule both copies turn on. A loose answer costs more here than it does there: a path the tree
// does not carry, resolved to a real file anyway, exempts a genuine restatement and the run reports a
// duplicated rule as compliant. Two copies of one fact, agreeing because both get read rather than by
// construction.
type citationResolver struct {
	byBase map[string][]string
}

func newCitationResolver(files []string) *citationResolver {
	byBase := map[string][]string{}
	for _, f := range files {
		base := filepath.Base(f)
		byBase[base] = append(byBase[base], f)
	}
	return &citationResolver{byBase: byBase}
}

// The one walked file this target names, or "" when the tree holds no such file or more than one
// answers to it. Ambiguity resolves to nothing rather than to a guess: the exemption a wrong guess
// buys is silence over a restatement, and the pair it refuses is merely printed.
func (r *citationResolver) fileNamed(target, from string) string {
	if !strings.ContainsRune(target, '/') {
		// A bare name carries no path to match on, so it names a file only when one file has it.
		return only(r.byBase[target])
	}
	// The forms one cited path is written in: relative to the citing file, and with each mount
	// prefix off, because a citation names `~/.claude/skills/x/SKILL.md` for a file this walk reached
	// as `<root>/skills/x/SKILL.md`.
	cleaned := strings.TrimPrefix(strings.TrimPrefix(target, "~/"), "./")
	forms := []string{
		filepath.Join(filepath.Dir(from), target),
		cleaned,
		strings.TrimPrefix(cleaned, ".kk-flavor/"),
		strings.TrimPrefix(cleaned, ".claude/"),
	}
	return only(r.tails(forms))
}

// The walked files any written form of the path is the tail of. The whole form has to be the tail:
// matching less would let a path the tree does not carry answer to a file that merely ends the same
// way, which is the collision this resolver exists to refuse.
func (r *citationResolver) tails(forms []string) []string {
	seen := map[string]bool{}
	var tails []string
	for _, form := range forms {
		for _, candidate := range r.byBase[filepath.Base(form)] {
			if seen[candidate] || !(candidate == form || strings.HasSuffix(candidate, "/"+form)) {
				continue
			}
			seen[candidate] = true
			tails = append(tails, candidate)
		}
	}
	sort.Strings(tails)
	return tails
}

// The single element, or "" for none and for more than one.
func only(paths []string) string {
	if len(paths) != 1 {
		return ""
	}
	return paths[0]
}

// The overlap that survives once the words both spans owe to a name they both cite are removed.
func overlapBeyond(a, b, named map[string]bool) int {
	n := 0
	for w := range a {
		if b[w] && !named[w] {
			n++
		}
	}
	return n
}

func overlap(a, b map[string]bool) int {
	n := 0
	for w := range a {
		if b[w] {
			n++
		}
	}
	return n
}

// What one candidate pair turns out to be.
type verdict int

const (
	unrelated verdict = iota
	// The same rule stated in two files: what this tool exists to find, and what fails the run.
	restatement
	// Two files naming the same thing — a path, a command — and agreeing on nothing else. Reported
	// apart from a restatement because the answer is already known: see namedWords. The `→` filter in
	// collect is this same case caught earlier, when the citation is written after an arrow rather than
	// as prose.
	sharedName
	// One of the two cites the file the other is in: a pointer to the rule's home, which is what
	// `ecosystem.md` → **One home** asks for rather than what it forbids. See citedTargets.
	citesOwner
)

func classify(a, b span) (v verdict, shared, beyond int) {
	shared = overlap(a.key, b.key)
	smaller := len(a.key)
	if len(b.key) < smaller {
		smaller = len(b.key)
	}
	// Most of the shorter rule's discriminating words, so a long rule cannot drag in every short one
	// that happens to share vocabulary with part of it.
	if shared < minDiscriminatingWords || shared*100 < smaller*70 {
		return unrelated, shared, 0
	}
	// Before the name test, because a citation outranks it: the pair shares the rule's subject, and
	// namedWords strips names, not subjects. `beyond` is left at zero — it measures how much survives
	// the names, and no such measurement decided this verdict. Nothing prints it here, so the zero
	// reaches no reader.
	if a.cites[b.file] || b.cites[a.file] {
		return citesOwner, shared, 0
	}
	named := namedWords(a.text)
	for w := range namedWords(b.text) {
		named[w] = true
	}
	beyond = overlapBeyond(a.key, b.key, named)
	if beyond < minDiscriminatingWords {
		return sharedName, shared, beyond
	}
	return restatement, shared, beyond
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Both rows are real spans out of this tree. The first is not a restatement: two consumers naming the
// same dependency at their own point of use is what `One home` asks for, not what it bars. The second
// is a genuine restatement and has to stay one, or the test below would pass by silencing everything.
func TestAPairSharingOnlyACitedNameIsNotARestatement(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want verdict
	}{
		{
			"the same dependency declared by two consumers",
			"The pass is `~/.claude/skills/kk-qualify/SKILL.md`",
			"The pass is `~/.claude/skills/kk-qualify/SKILL.md`",
			sharedName,
		},
		{
			"the same rule reworded in two files",
			"A licence you received goes into every spawn prompt you build, verbatim",
			"The licence goes in every spawn prompt's emphasis slot",
			restatement,
		},
		{
			// The blind spot the split could open: prose that states a rule and happens to name a file.
			// It stays a restatement because what the two share survives the name being removed — five
			// words of agreement that owe the path nothing.
			"a rule that names a file but agrees beyond it",
			"Never edit a generated file by hand, regenerate it from `~/.claude/skills/kk-qualify/SKILL.md`",
			"Never edit a generated file by hand, regenerate it instead",
			restatement,
		},
		{"two rules sharing nothing", "the first rule about fences", "an unrelated rule about ledgers", unrelated},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, shared, beyond := classify(spanOf(c.a), spanOf(c.b))
			if got != c.want {
				t.Fatalf("classify = %v, want %v (%d shared, %d beyond the name)", got, c.want, shared, beyond)
			}
		})
	}
}

// A pair that shares only a cited name must not fail the run: the whole point of setting it apart is
// that a reader has already answered it, and an exit 1 asks them again every time.
func TestOnlyARestatementFailsTheRun(t *testing.T) {
	naming, _, _ := classify(
		spanOf("The pass is `~/.claude/skills/kk-qualify/SKILL.md`"),
		spanOf("The pass is `~/.claude/skills/kk-qualify/SKILL.md`"))
	if naming == restatement {
		t.Fatal("a shared name is being counted as a restatement, so a clean tree would exit 1")
	}
}

func spanOf(text string) span {
	return span{text: text, key: keyOf(text)}
}

// The two rules every citation case below is built from: a rule, and a second rule that restates it.
// Both are real lines out of this tree, and uncited they are a restatement — which is what makes an
// exemption below mean the citation bought it.
const ownerRule = "`.idsd/` in this suite means the resolved scratch root, not a path in the repo."
const pointerRule = "The intent path below, and every `.idsd/` path in this file, hangs off the resolved scratch root rather than the repo root"

// Every citation case walks a real tree instead of building spans by hand. The line that hands a
// line's citations to the spans on it lives inside the walk, and a case that assembles its own spans
// passes unchanged with that line deleted — the tool then reports the compliant pointer it was built
// to exempt, and nothing in the suite says so.
func scanTree(t *testing.T, files map[string]string) scan {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	found, err := collect(dir)
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func classifyTree(t *testing.T, files map[string]string) verdict {
	t.Helper()
	found := scanTree(t, files)
	// Two, or the pair under test is not the pair being classified and every assertion below is over
	// something other than what it names.
	if len(found.spans) != 2 {
		t.Fatalf("the fixture produced %d bolded rules, want exactly 2: %+v", len(found.spans), found.spans)
	}
	v, _, _ := classify(found.spans[0], found.spans[1])
	return v
}

// A tree carrying the pair: the rule at `ownerPath`, the rule restating it at `pointerPath`, and the
// restating line citing `cites` — empty for none.
func pairAt(ownerPath, pointerPath, cites string) map[string]string {
	line := "**" + pointerRule + "**"
	if cites != "" {
		line += " (`" + cites + "` → **Report**)"
	}
	return map[string]string{
		ownerPath:   "# owner\n**" + ownerRule + "**\n",
		pointerPath: "# pointer\n" + line + "\n",
	}
}

const ownerFile = "skills/idsd-qualify/SKILL.md"
const pointerFile = "skills/idsd-build/SKILL.md"

func pairCiting(cites string) map[string]string {
	return pairAt(ownerFile, pointerFile, cites)
}

// The real pair this exists for: a rule in one file that cites the file owning it. citedTargets says
// why such a pair scores as a restatement on vocabulary alone. The first half here is the negative
// control: without the citation this same pair IS a restatement, so a pass below means the citation
// decided it rather than the two texts having been harmless all along.
func TestAPointerToTheRulesOwnerIsNotARestatement(t *testing.T) {
	if v := classifyTree(t, pairCiting("")); v != restatement {
		t.Fatalf("uncited, the pair classifies as %v — the fixture is not a restatement, so this case proves nothing", v)
	}
	if v := classifyTree(t, pairCiting("~/.claude/skills/idsd-qualify/SKILL.md")); v != citesOwner {
		t.Fatalf("a rule citing the file that owns it classified as %v, want citesOwner — a compliant pointer fails the run", v)
	}
}

// The forms a citation is written in here, each resolved against the tree that was walked. A tree is
// walked as `<root>/skills/x/SKILL.md` and cited as `~/.claude/skills/x/SKILL.md`, so a citation that
// resolves by string shape alone resolves to nothing — and a citation that stops counting takes the
// exemption with it, silently.
func TestEveryWrittenFormOfACitationResolves(t *testing.T) {
	forms := []string{
		"~/.claude/skills/idsd-qualify/SKILL.md",
		"skills/idsd-qualify/SKILL.md",
		"idsd-qualify/SKILL.md",
		"../idsd-qualify/SKILL.md",
	}
	for _, form := range forms {
		if v := classifyTree(t, pairCiting(form)); v != citesOwner {
			t.Errorf("a citation written as %q classified as %v, want citesOwner", form, v)
		}
	}
}

// A citation resolves against the files the walk actually read, never against the shape of the path.
// Every skill's file is named `SKILL.md`, so a match on a trailing segment or two rests entirely on
// the parent: `~/vendor/delta/SKILL.md` then answers to the tree's own `skills/delta/SKILL.md`, and a
// citation naming a different file exempts the restatement. The cited path need not even exist — a
// fabricated one silences the pair just as well.
func TestACitationNamingAnotherFileDoesNotExemptTheRestatement(t *testing.T) {
	cases := []struct {
		name, owner, cites, alsoInTree string
	}{
		// The owner sits at `skills/delta/`, so the cited path's last two segments are exactly the
		// owner's and only the whole tail tells the two apart.
		{"a real file the cited path does name", "skills/delta/SKILL.md", "~/vendor/delta/SKILL.md", "vendor/delta/SKILL.md"},
		{"the same path, with no such file in the tree", "skills/delta/SKILL.md", "~/vendor/delta/SKILL.md", ""},
		// The owner's basename is the tree's only one, so nothing but the tail refuses this: matched
		// on the last segment, a path the tree carries nothing of answers to the real file.
		{"a fabricated path whose basename is unique in the tree", "standards/writing.md", "~/made/up/writing.md", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := pairAt(c.owner, "skills/alpha/SKILL.md", c.cites)
			if c.alsoInTree != "" {
				files[c.alsoInTree] = "# the file that citation really does name\n"
			}
			if v := classifyTree(t, files); v != restatement {
				t.Fatalf("classified as %v, want restatement — citing %q exempted a rule owned by %s", v, c.cites, c.owner)
			}
		})
	}
}

// A bare name is how most of this tree's citations are written — `ecosystem.md` → **One home** names
// no path at all — and one file answering to it is what makes it resolvable. The refusal case below
// asserts only the other side: with the whole branch returning nothing, every bare citation stops
// exempting and each pointer written that way is reported as the duplication it is not.
func TestABareNameOnlyOneFileAnswersToResolves(t *testing.T) {
	if v := classifyTree(t, pairAt("standards/writing.md", "skills/alpha/SKILL.md", "writing.md")); v != citesOwner {
		t.Fatalf("a bare name only one file in the tree carries classified as %v, want citesOwner", v)
	}
}

// One file reachable through two written forms is one match, not two. A citation written relative to
// a file at the walk's own root is exactly that: joined against the citing file's directory it equals
// the file, and taken as written it is that file's tail. Counted twice the citation reads as
// ambiguous and a compliant pointer is reported as a restatement. Every other fixture here puts its
// citing file in a subdirectory, where the two forms cannot coincide.
func TestOneFileReachedByTwoWrittenFormsIsOneMatch(t *testing.T) {
	if v := classifyTree(t, pairAt("standards/writing.md", "alpha.md", "standards/writing.md")); v != citesOwner {
		t.Fatalf("a file that two written forms of one citation both name classified as %v, want citesOwner", v)
	}
}

// Three files answer to a bare `SKILL.md`, so nothing about it says which was meant. The owner is the
// first of them in the walk's order, so a resolver that took the first match rather than refusing
// would exempt this pair — which is what makes a pass here mean the ambiguity was refused, and not
// that the guess happened to miss.
func TestAnAmbiguousCitationExemptsNothing(t *testing.T) {
	files := pairAt("skills/aaa/SKILL.md", "skills/zzz/SKILL.md", "SKILL.md")
	files["skills/mmm/SKILL.md"] = "# a third file answering to the same bare name\n"
	if v := classifyTree(t, files); v != restatement {
		t.Fatalf("classified as %v, want restatement — a bare name three files answer to was resolved to one of them", v)
	}
}

// A citation exempts the rule it was written for, not every rule beside it. Handed the whole line, one
// legitimate pointer covers each of its line-mates: a duplicated rule parked on a line that already
// carries a cross-reference is exempted, and comes back reported as compliant.
func TestACitationDoesNotExemptTheOtherRulesOnItsLine(t *testing.T) {
	const carried = "a citation resolves against the file set this walk actually read"
	const beside = "an ambiguous name is refused rather than resolved to some guess"
	const cite = " (`~/.kk-flavor/standards/writing.md` → **One home**)"
	// Both orders, because the scope has two edges and one fixture can only see one of them: a
	// citation written before its line-mate leaks backwards over the span's start, and one written
	// after leaks forwards over its end. Asserted with one order only, either edge can be removed
	// with every case still green.
	cases := []struct{ name, line string }{
		{"written before the rule beside it", "**" + carried + "**" + cite + " and also **" + beside + "**"},
		{"written after the rule beside it", "**" + beside + "** and also **" + carried + "**" + cite},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			found := scanTree(t, map[string]string{
				"standards/writing.md":  "# owner\n**" + carried + "**\n\n**" + beside + "**\n",
				"skills/alpha/SKILL.md": "# two rules, one line, one citation\n" + c.line + "\n",
			})
			if len(found.spans) != 4 {
				t.Fatalf("the fixture produced %d bolded rules, want exactly 4: %+v", len(found.spans), found.spans)
			}
			at := func(dir, text string) span {
				t.Helper()
				for _, s := range found.spans {
					if strings.Contains(s.file, dir) && s.text == text {
						return s
					}
				}
				t.Fatalf("no span for %q under %s", text, dir)
				return span{}
			}
			// The control: the rule the citation was written for is still exempt, so a pass on the
			// assertion below is the scoping and not a citation nothing read.
			if v, _, _ := classify(at("skills", carried), at("standards", carried)); v != citesOwner {
				t.Fatalf("the rule carrying the citation classified as %v, want citesOwner", v)
			}
			if v, _, _ := classify(at("skills", beside), at("standards", beside)); v != restatement {
				t.Fatalf("the rule beside the citation classified as %v, want restatement — one pointer exempted its line-mate", v)
			}
		})
	}
}

// Both ways this tree writes a citation, and nothing else. A backticked span is as often a command as
// a path, and reading one as a citation exempts pairs at random.
func TestCitedTargetsReadsBothFormsAndNothingElse(t *testing.T) {
	cases := []struct {
		name, line string
		want       []string
	}{
		{"backticked path", "see `~/.claude/skills/idsd-qualify/SKILL.md` → **Report**", []string{"~/.claude/skills/idsd-qualify/SKILL.md"}},
		{"markdown link", "([ecosystem.md](ecosystem.md) → **One home**)", []string{"ecosystem.md"}},
		{"relative link", "[testing.md](../testing.md)", []string{"../testing.md"}},
		{"a backticked command", "run `report.sh root` first", nil},
		{"a backticked path of another kind", "the scanner is `scripts/dup-literals.sh`", nil},
		{"no citation at all", "plain prose naming nothing", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := citedTargets(c.line); strings.Join(got, "|") != strings.Join(c.want, "|") {
				t.Errorf("citedTargets(%q) = %q, want %q", c.line, got, c.want)
			}
		})
	}
}

// Cases for the handoff gate. Every check is paired with its negative control. The clean draft
// passes, and each case is that same draft broken in exactly one way. A gate that had stopped
// reading the file would otherwise satisfy every refusal case and fail none of them.

// No case here forks git. The gate asks a repository four things: that the directory is a work tree,
// whether the base commit resolves, whether the tree is dirty, and how `repo-key` abbreviates it. A
// `repotest.Fake` answers all four. That a real git answers them the way these cases assume is
// `repo/exec_test.go`'s.

// What stays on disk is the directory itself and a HEAD under its git dir. The gate resolves the
// path a draft has to name, and `repo-key` reads that HEAD to refuse a path that is no git dir.

// One repository serves the whole suite, so these cases run in sequence. Two cases in parallel would
// disagree about whether the tree is dirty.

// One guard is named here, with no case behind it. "could not resolve" sits behind a successful
// IsDir, so reaching it needs the directory to disappear between two statements. No fixture worth
// building does that.

// No case here reads outside the module. The case that ran this gate over the shipped handoff
// template is `ai/tools/shipped_handoff_template_test.go`, in the package `ai/gate.sh` forces. Go
// keys its test cache on the module, and a case here would have answered `ok (cached)` over a
// template that changed.
package handoffcheck

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"configs/ai/tools/repo/repotest"
	"configs/ai/tools/shell"
)

// The fixture clone's own directory name, and the abbreviation `repo-key` answers for it — which is
// the only repository prefix a title may carry in this suite. Both written out rather than asked of
// `repo-key`, so a case comparing the two is not comparing that package with itself.
//
// Distinctive, not "repo": the gate refuses a draft naming the repository by basename, and a fixture
// called "repo" would make that case pass on the word "repo" appearing anywhere.
const (
	fixtureName   = "handoff-fixture"
	fixtureAbbrev = "HF"
)

// The base commit every fixture holds: twelve hex, the length a session writes an abbreviated SHA
// down at. It is distinct from the `0123456789ab` a case hands a repository that lacks it.
const fixtureSHA = "9f2a1c0b7de4"

// The single repository every case runs against, its resolved path, and the port answering for it.
// TestMain builds it once. The dirty-tree pair is the only case that changes what a later case
// reads, and it puts the tree back.
var (
	fixtureRepo string
	fixturePath string
	fixtureGit  *repotest.Fake
)

func TestMain(m *testing.M) {
	base, err := os.MkdirTemp("", "handoff-check")
	if err != nil {
		fmt.Fprintln(os.Stderr, "handoff-check: no temporary directory, so nothing was tested:", err)
		os.Exit(2)
	}
	defer os.RemoveAll(base)

	fixtureRepo = filepath.Join(base, fixtureName)
	if fixtureRepo, fixturePath, fixtureGit, err = newRepo(fixtureRepo); err != nil {
		fmt.Fprintln(os.Stderr, "handoff-check: no repository fixture, so nothing was tested:", err)
		os.RemoveAll(base)
		os.Exit(2)
	}

	code := m.Run()
	os.RemoveAll(base)
	os.Exit(code)
}

// newRepo is a directory the gate can be pointed at and the port that answers for it. That is a work
// tree holding one commit, a clean tree, and a shared git dir whose parent name is what `repo-key`
// abbreviates.

// Two things are real on disk and both have to be. The gate resolves the path a draft must name, and
// `repo-key` refuses a git dir with no HEAD in it. The case for a repository lacking an abbreviation
// is arranged that way.

// os.MkdirTemp hands back a symlinked path on macOS. A draft quoting the path as created then fails
// to match what the gate compares against.
func newRepo(dir string) (repoDir, resolved string, git *repotest.Fake, err error) {
	git = repotest.New(dir)
	if err = os.MkdirAll(git.Git, 0o755); err != nil {
		return "", "", nil, err
	}
	if err = os.WriteFile(filepath.Join(git.Git, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		return "", "", nil, err
	}
	if resolved, err = filepath.EvalSymlinks(dir); err != nil {
		return "", "", nil, err
	}
	return dir, resolved, git.Commit(fixtureSHA, nil), nil
}

// draftWith is the draft every case mutates. Each slot holds the shortest thing that is genuinely
// filled, so a case that breaks one slot is measuring that slot alone.
type draft struct {
	title   string
	task    string
	facts   string
	lead    string
	scope   string
	traps   string
	start   string
	licence string
	extra   string
}

func cleanDraft() draft {
	return draft{
		title:   "Cut the mutation run down",
		task:    "Make the mutation gate finish inside ten minutes. Done when it does.",
		facts:   "7306s of serial work, 921s wall on 8 performance cores, median mutant 13.6s.",
		lead:    "I believe sharding by package is the biggest win. Verify that before building anything.",
		scope:   "The flaky resolver test — raise it, do not bundle it.",
		traps:   "resolve.sh reuses a stale binary unless ECO_TOOLS_BUILD=1 is set.",
		start:   "Base commit " + fixtureSHA + " in " + fixturePath + ". Another session holds ai/tools/eco-check/; rebase onto its work.",
		licence: "> Take the bigger change where it is the better one.",
	}
}

func (d draft) text() string {
	body := ""
	if d.title != "" {
		body = "# " + d.title + "\n"
	}
	for _, slot := range []struct{ name, value string }{
		{"The task", d.task},
		{"Measured facts", d.facts},
		{"The lead", d.lead},
		{"Out of scope", d.scope},
		{"Traps", d.traps},
		{"Where it starts", d.start},
		{"Licence", d.licence},
	} {
		if slot.name == "Traps" && d.traps == dropped {
			continue
		}
		body += "\n## " + slot.name + "\n" + slot.value + "\n"
	}
	return body + d.extra + "\n"
}

// The value that removes a heading rather than emptying it, so "missing section" and "empty section"
// are two different cases and not one written twice.
const dropped = "\x00drop"

// gate writes the draft to a file and runs the gate over it against the fixture repository, answering
// with the combined output the shell caller would have seen and the exit code.
func gate(t *testing.T, d draft) (string, int) {
	t.Helper()
	return gateOver(t, d.text(), fixtureRepo, fixtureGit)
}

func gateOver(t *testing.T, body, repo string, git *repotest.Fake) (string, int) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "case.md")
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatalf("writing the case draft: %v — nothing was tested", err)
	}
	return gateFile(file, repo, git)
}

func gateFile(file, repo string, git *repotest.Fake) (string, int) {
	var out, errOut bytes.Buffer
	code := Run("handoff-check.sh", file, repo, git, &out, &errOut)
	return out.String() + errOut.String(), code
}

func expect(t *testing.T, name, got string, code, want int, contains, absent []string) {
	t.Helper()
	if code != want {
		t.Errorf("%s: exit %d, wanted %d — output: %s", name, code, want, got)
	}
	for _, text := range contains {
		if !strings.Contains(got, text) {
			t.Errorf("%s: wanted %q in: %s", name, text, got)
		}
	}
	for _, text := range absent {
		if strings.Contains(got, text) {
			t.Errorf("%s: did not want %q in: %s", name, text, got)
		}
	}
}

// The ways it cannot run. Each exits 2 and prints no finding, so a caller reading 2 as clean is the
// failure these guard.
func TestRefusesToRun(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run("handoff-check.sh", "", fixtureRepo, fixtureGit, &out, &errOut)
	expect(t, "no draft argument", out.String()+errOut.String(), code, 2, []string{"usage:"}, nil)

	got, code := gateFile(filepath.Join(t.TempDir(), "absent.md"), fixtureRepo, fixtureGit)
	expect(t, "a draft that is not there", got, code, 2, []string{"no such file"}, nil)

	empty := filepath.Join(t.TempDir(), "empty.md")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatalf("writing the empty draft: %v — nothing was tested", err)
	}
	got, code = gateFile(empty, fixtureRepo, fixtureGit)
	expect(t, "an empty draft", got, code, 2, []string{"nothing was checked"}, nil)

	// A filled draft, not the empty one above. The draft guards run first, so an empty draft exits 2
	// before the repository is looked at, and these two cases would pass with the repository guards gone.
	filled := filepath.Join(t.TempDir(), "titleonly.md")
	if err := os.WriteFile(filled, []byte("# a draft the gate gets past\n"), 0o644); err != nil {
		t.Fatalf("writing the filled draft: %v — nothing was tested", err)
	}
	// The refusal is arranged, not found: a temporary directory that happened to sit inside a clone
	// would pass this case for the wrong reason.
	outside := t.TempDir()
	noRepository := repotest.New(outside)
	noRepository.Fail["GitDir"] = errors.New("fatal: not a git repository")
	got, code = gateFile(filled, outside, noRepository)
	expect(t, "a repo that is not a work tree", got, code, 2, []string{"not a git work tree"}, nil)

	got, code = gateFile(filled, filled, fixtureGit)
	expect(t, "a repo that is a file", got, code, 2, []string{"not a directory"}, nil)
}

// The one guard whose fixture can fail to deny. Probed, never assumed: root reads a mode-000 file
// happily, and asserting there would go red for a reason that has nothing to do with the gate.
func TestRefusesAnUnreadableDraft(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unreadable.md")
	if err := os.WriteFile(path, []byte("# a draft nobody may read\n"), 0o000); err != nil {
		t.Fatalf("writing the unreadable draft: %v — nothing was tested", err)
	}
	if _, err := os.ReadFile(path); err == nil {
		t.Skip("mode 000 does not deny this process, so the fixture proves nothing")
	}
	got, code := gateFile(path, fixtureRepo, fixtureGit)
	expect(t, "a draft this process cannot read", got, code, 2, []string{"cannot read"}, nil)
}

// The control. If this stops passing, no case below proves anything.
func TestCompleteDraftPasses(t *testing.T) {
	got, code := gate(t, cleanDraft())
	expect(t, "a complete draft", got, code, 0, nil, []string{"missing section"})
}

// The dirty-tree advisory: a note, not a finding, so a clean draft over a dirty repo still exits 0.
func TestDirtyTreeIsANoteAndNotAFinding(t *testing.T) {
	fixtureGit.StatusLines = []string{"?? untracked.txt"}
	got, code := gate(t, cleanDraft())
	expect(t, "a dirty repository", got, code, 0, []string{"does not travel"}, nil)

	// Put back, because every other case reads this same repository and asserts the note is absent.
	fixtureGit.StatusLines = nil
	got, code = gate(t, cleanDraft())
	expect(t, "a clean repository", got, code, 0, nil, []string{"does not travel"})
}

func TestStructure(t *testing.T) {
	for _, tc := range []struct {
		name             string
		mutate           func(*draft)
		want             int
		contains, absent []string
	}{
		{
			name:     "a missing section",
			mutate:   func(d *draft) { d.traps = dropped },
			want:     1,
			contains: []string{"missing section: Traps"},
		},
		{
			name:     "an empty section",
			mutate:   func(d *draft) { d.traps = "" },
			want:     1,
			contains: []string{"empty section: Traps"},
		},
		{
			name:     "a leftover template comment",
			mutate:   func(d *draft) { d.extra = "\n<!-- still to write -->" },
			want:     1,
			contains: []string{"template comment left"},
		},
		{
			name:     "a duplicated section",
			mutate:   func(d *draft) { d.extra = "\n## Traps\na second copy" },
			want:     1,
			contains: []string{"duplicate section: Traps"},
		},
		{
			name:     "an eighth section",
			mutate:   func(d *draft) { d.extra = "\n## Appendix\nwhatever else I felt like adding" },
			want:     1,
			contains: []string{"unknown section: Appendix"},
		},
		{
			// No prefix, so the line IS the work half and the finding names the line. Pinned against the
			// half-naming case below: a title with no prefix has no second slot to point the author at.
			name:     "the template title placeholder",
			mutate:   func(d *draft) { d.title = "<one imperative line: the work>" },
			want:     1,
			contains: []string{"the title line is still the template placeholder"},
			absent:   []string{"work half"},
		},
		{
			// The control for the anchored placeholder test: an angle bracket inside a real title is
			// not the placeholder, and refusing it would send the agent to reword a sound line.
			name:   "a real title holding an angle bracket",
			mutate: func(d *draft) { d.title = "Cut the mutation run to <10 minutes" },
			want:   0,
		},
		{
			name:     "the template's repository prefix left unfilled",
			mutate:   func(d *draft) { d.title = "[<repo abbrev>] Cut the mutation run down" },
			want:     1,
			contains: []string{"repository prefix is still the template placeholder"},
		},
		{
			name:   "a filled repository prefix",
			mutate: func(d *draft) { d.title = "[" + fixtureAbbrev + "] Cut the mutation run down" },
			want:   0,
		},
		{
			// The half the whole-line test used to hide: a filled prefix in front of an unfilled work
			// half reads as a done line, and the work slot nobody wrote went unreported. The finding
			// names that half rather than the line, because the line's other slot is filled.
			name:     "a filled prefix in front of an unfilled work half",
			mutate:   func(d *draft) { d.title = "[" + fixtureAbbrev + "] <one imperative line: the work>" },
			want:     1,
			contains: []string{"the title's work half is still the template placeholder"},
		},
		{
			// The control for the placeholder test's closing anchor, on the prefix half: `<10min` opens
			// with an angle bracket and is not the template's slot, so it is refused for naming the
			// wrong repository and never for being unfilled.
			name:     "a prefix that opens with an angle bracket is not the placeholder",
			mutate:   func(d *draft) { d.title = "[<10min] Cut the mutation run down" },
			want:     1,
			contains: []string{"the title opens with [<10min]"},
			absent:   []string{"still the template placeholder"},
		},
		{
			name:   "a filled prefix on a title holding an angle bracket",
			mutate: func(d *draft) { d.title = "[" + fixtureAbbrev + "] Cut the mutation run to <10 minutes" },
			want:   0,
		},
		{
			// The drift the prefix exists to remove: two sessions in one repository wrote two different
			// names there, and nothing held either against what the tool the template names would print.
			name:     "an opening bracket naming a repository this is not",
			mutate:   func(d *draft) { d.title = "[issue-tracker] Cut the mutation run down" },
			want:     1,
			contains: []string{"the title opens with [issue-tracker]", "abbreviates to " + fixtureAbbrev},
			absent:   []string{"still the template placeholder"},
		},
		{
			// The same refusal, reached by an author who wrote a bracketed word as prose. The finding
			// may not tell them their repository prefix is wrong: they wrote no prefix. It says what it
			// saw — an opening bracket, which is the slot — and names both ways out.
			name:     "an opening bracket that was never meant as a repository",
			mutate:   func(d *draft) { d.title = "[flaky] resolver test — cut it from the run" },
			want:     1,
			contains: []string{"the title opens with [flaky]", "off the start of the line"},
			absent:   []string{"repository prefix"},
		},
		{
			// A prefix carrying a control byte reaches the finding as text. The bytes are the draft's,
			// and a raw escape would be re-interpreted by the terminal the human reads the finding in.
			name:     "an opening bracket holding a control byte",
			mutate:   func(d *draft) { d.title = "[conf\x1b[31migs] Cut the mutation run down" },
			want:     1,
			contains: []string{"the title opens with [conf [31migs]"},
			absent:   []string{"\x1b"},
		},
		{
			// The bracket has to open the line. A title holding one further along is a title with no
			// repository prefix, and reading its first words as one would refuse a sound line for
			// naming a repository nobody wrote down.
			name:   "a bracket further along a title with no prefix",
			mutate: func(d *draft) { d.title = "Cut the [flaky] resolver test out of the run" },
			want:   0,
		},
		{
			// Only the FIRST `]` cuts, so a second bracketed word stays in the work half where its
			// author put it.
			name:   "a second bracketed word after a filled prefix",
			mutate: func(d *draft) { d.title = "[" + fixtureAbbrev + "] [flaky] resolver test — cut it from the run" },
			want:   0,
		},
		{
			// The extreme of the defect this check exists to refuse: a title that is only the prefix.
			// The work half is absent rather than a placeholder, and the line used to pass whole
			// because the split wanted a space it never found.
			name:     "a title that is only a filled prefix",
			mutate:   func(d *draft) { d.title = "[" + fixtureAbbrev + "]" },
			want:     1,
			contains: []string{"the title's work half is still the template placeholder"},
		},
		{
			// A CRLF draft. `\r` is a space byte, so an unfilled half ended in one and never matched
			// its own closing `>`; the title check passed a line in which nothing had been written.
			name:     "a placeholder title on a CRLF line",
			mutate:   func(d *draft) { d.title = "<one imperative line: the work>\r" },
			want:     1,
			contains: []string{"the title line is still the template placeholder"},
		},
		{
			// The space after the bracket is the work half's to lose, so a name written tight against
			// it is still the name standing in the slot and is still weighed.
			name:     "an opening bracketed word with no space after it",
			mutate:   func(d *draft) { d.title = "[issue-tracker]Cut the mutation run down" },
			want:     1,
			contains: []string{"the title opens with [issue-tracker]"},
		},
		{
			name:     "two title lines",
			mutate:   func(d *draft) { d.extra = "\n# A second title" },
			want:     1,
			contains: []string{"more than one title line"},
		},
		{
			name:     "no title line",
			mutate:   func(d *draft) { d.title = "" },
			want:     1,
			contains: []string{"no title line"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := cleanDraft()
			tc.mutate(&d)
			got, code := gate(t, d)
			expect(t, tc.name, got, code, tc.want, tc.contains, tc.absent)
		})
	}
}

// `None` is how a slot says there is nothing, and three slots may never say it.
func TestNone(t *testing.T) {
	for _, tc := range []struct {
		name             string
		mutate           func(*draft)
		want             int
		contains, absent []string
	}{
		{
			name:     "None with a reason",
			mutate:   func(d *draft) { d.traps = "None known — this repo has no false-green traps." },
			want:     0,
			contains: []string{"declared None: Traps"},
		},
		{
			name:     "a bare None",
			mutate:   func(d *draft) { d.traps = "None" },
			want:     1,
			contains: []string{"None with no reason: Traps"},
		},
		{
			name:     "None in a slot that refuses it",
			mutate:   func(d *draft) { d.start = "None — no base needed." },
			want:     1,
			contains: []string{"None refused in: Where it starts"},
		},
		{
			// The word, not a word starting with it. Without the boundary a slot opening "Nonetheless"
			// is read as an empty one and its real content never measured.
			name:     "a slot opening with a longer word",
			mutate:   func(d *draft) { d.traps = "Nonetheless the resolver reuses a stale binary." },
			want:     0,
			absent:   []string{"declared None", "None with no reason"},
			contains: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := cleanDraft()
			tc.mutate(&d)
			got, code := gate(t, d)
			expect(t, tc.name, got, code, tc.want, tc.contains, tc.absent)
		})
	}
}

// The chip carries a working directory and a pasted prompt does not, so the draft has to name the
// repository. The clean draft names it by absolute path, which is the only form that counts.
func TestBaseAndRepository(t *testing.T) {
	for _, tc := range []struct {
		name             string
		mutate           func(*draft)
		want             int
		contains, absent []string
	}{
		{
			name:     "a base commit that resolves to nothing",
			mutate:   func(d *draft) { d.start = "Base commit 0123456789ab in " + fixturePath + ". Nobody else is live." },
			want:     1,
			contains: []string{"base commit does not resolve"},
		},
		{
			name:     "no commit named at all",
			mutate:   func(d *draft) { d.start = "Start from the tip of main in " + fixturePath + "; nobody else is live." },
			want:     1,
			contains: []string{"no base commit named"},
		},
		{
			name: "the repository named by basename alone",
			mutate: func(d *draft) {
				d.start = "Base commit " + fixtureSHA + " in the handoff-fixture checkout; nobody else is live."
			},
			want:     1,
			contains: []string{"no repository named in: Where it starts"},
		},
		{
			name:     "a slot naming the commit but no repository",
			mutate:   func(d *draft) { d.start = "Base commit " + fixtureSHA + ", and nobody else is live here." },
			want:     1,
			contains: []string{"no repository named in: Where it starts"},
		},
		{
			// The bound on the hex scan, and why it is there: a temporary directory named
			// `build1691946027` puts a ten-digit run inside a word. Unbounded, the draft below is
			// answered with "base commit does not resolve", quoting digits nobody wrote as a commit.
			name:     "a hex run glued to the middle of a word is not a commit",
			mutate:   func(d *draft) { d.start = "Start from the tip of build1691946027 in " + fixturePath + "." },
			want:     1,
			contains: []string{"no base commit named"},
			absent:   []string{"does not resolve"},
		},
		{
			// Its control: the same run standing as its own token is read as a candidate, so the bound
			// cannot be satisfied by a scan that stopped finding commits at all.
			name:     "a hex run standing alone is tried as a commit",
			mutate:   func(d *draft) { d.start = "Start from 1691946027 in " + fixturePath + "." },
			want:     1,
			contains: []string{"base commit does not resolve"},
		},
		{
			// Fenced content is read, not merely counted. A "Where it starts" written inside a fence
			// names the repository and the commit as plainly as an unfenced one, and both scans see it.
			name: "a fenced Where it starts",
			mutate: func(d *draft) {
				d.start = "```\nBase commit " + fixtureSHA + " in " + fixturePath + ". Nobody else is live.\n```"
			},
			want:   0,
			absent: []string{"no repository named", "no base commit named"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := cleanDraft()
			tc.mutate(&d)
			got, code := gate(t, d)
			expect(t, tc.name, got, code, tc.want, tc.contains, tc.absent)
		})
	}
}

// The substance floor, pinned on both sides of its boundary. Without the passing half, raising the
// floor to refuse every real draft would still look like a working gate.
func TestSubstanceFloor(t *testing.T) {
	d := cleanDraft()
	d.traps = "Stale binary unless the flag"
	got, code := gate(t, d)
	expect(t, "a five-word slot", got, code, 0, nil, nil)

	d = cleanDraft()
	d.traps = "Stale binary unless flag"
	got, code = gate(t, d)
	expect(t, "a four-word slot", got, code, 1, []string{"barely filled: Traps — 4 word(s)"}, nil)

	// Structure, placeholder, base commit, repository, blockquote and reachback all clear here. Only
	// the floor stands between this draft and a chip nobody could act on, so this measures the floor
	// alone rather than passing on some other finding.
	got, code = gate(t, draft{
		title:   "Cut the mutation run down",
		task:    "Finish it.",
		facts:   "Some.",
		lead:    "Probably fine.",
		scope:   "Stuff.",
		traps:   "Careful.",
		start:   "Base commit " + fixtureSHA + " in " + fixturePath + ".",
		licence: "> proceed properly",
	})
	expect(t, "a draft answering every slot in a word or two", got, code, 1,
		[]string{"barely filled: The task"}, []string{"no repository named"})

	// Licence is the one slot the floor may not touch: the words are the human's, quoted verbatim, and
	// padding them to clear a word count is the one repair the template forbids.
	d = cleanDraft()
	d.licence = "> Nothing else loosens."
	got, code = gate(t, d)
	expect(t, "a four-word licence quote", got, code, 0, nil, nil)

	// A fenced block is content. Measured facts is where command output goes, and a slot holding only
	// a fence used to come back as empty, which sent the author to fix the wrong thing.
	d = cleanDraft()
	d.facts = "```\n7306s serial, 921s wall, 8 cores\n```"
	got, code = gate(t, d)
	expect(t, "a section filled only by a fenced block", got, code, 0, nil, []string{"empty section: Measured facts"})
}

func TestLicenceMustBeQuoted(t *testing.T) {
	d := cleanDraft()
	d.licence = "He said to take the bigger change."
	got, code := gate(t, d)
	expect(t, "a paraphrased licence", got, code, 1, []string{"the licence is not quoted"}, nil)
}

func TestReachback(t *testing.T) {
	for _, tc := range []struct {
		name             string
		mutate           func(*draft)
		want             int
		contains, absent []string
	}{
		{
			name:     "a phrase reaching back into this conversation",
			mutate:   func(d *draft) { d.scope = "The flaky resolver test, as discussed — do not bundle it." },
			want:     1,
			contains: []string{"as discussed"},
		},
		{
			// A blockquote in Licence carries the words a human wrote, so a dangling phrase in one is
			// theirs.
			name:   "the same phrase inside the licence blockquote",
			mutate: func(d *draft) { d.licence = "> Fix it as discussed, and take the bigger change." },
			want:   0,
		},
		{
			// The exemption belongs to Licence only. Anywhere else a ">" is the drafting agent quoting
			// something it found, and exempting those would let any line dodge the scan by growing one.
			name:     "a reachback inside a blockquote outside Licence",
			mutate:   func(d *draft) { d.scope = "> The flaky resolver test, as discussed — do not bundle it." },
			want:     1,
			contains: []string{"as discussed"},
		},
		{
			name:   "the same phrase inside a fence",
			mutate: func(d *draft) { d.extra = "\n```\nas discussed\n```" },
			want:   0,
		},
		{
			// Word boundaries, both sides. Without them "as discussed" matches inside "was discussed"
			// and the finding quotes the author a phrase that appears nowhere in the draft.
			name:   "a dangling phrase sitting inside a longer word",
			mutate: func(d *draft) { d.scope = "The retry policy was discussed with the team; raise it, do not bundle it." },
			want:   0,
		},
		{
			// The template names "the file above" beside "as discussed" as a phrase that strands the
			// receiver.
			name:     "a reachback naming a thing above",
			mutate:   func(d *draft) { d.scope = "Skip the file above; raise it, do not bundle it." },
			want:     1,
			contains: []string{"the file above"},
		},
		{
			// The noun list after "the" is closed on purpose: a handoff about filesystem work says this.
			name:   "a real sentence ending in a directory above",
			mutate: func(d *draft) { d.scope = "Leave the directory above the checkout alone; raise it separately." },
			want:   0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := cleanDraft()
			tc.mutate(&d)
			got, code := gate(t, d)
			expect(t, tc.name, got, code, tc.want, tc.contains, tc.absent)
		})
	}
}

// A repository whose directory name holds the bytes a shell or an awk `-v` would have expanded. The
// gate must find that path in the draft exactly as written and report nothing else: the name is the
// only thing that could redden a draft otherwise correct for this repository.
func TestRepositoryNameHoldingEscapes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), `evil\nSHA\t--injected-token`)
	repo, path, git, err := newRepo(dir)
	if err != nil {
		t.Skipf("could not build a repository named %q, so this proves nothing: %v", dir, err)
	}
	d := cleanDraft()
	d.start = "Base commit " + fixtureSHA + " in " + path + ". Nobody else is live."
	got, code := gateOver(t, d.text(), repo, git)
	expect(t, "a correct draft in a repository whose name holds escapes", got, code, 0,
		nil, []string{"--injected-token", "no repository named"})
}

// Nothing reaches the terminal carrying an escape, whichever line it rode out on. The bytes come from
// the path the caller named and from the draft's own text, and a raw `ESC [ 2 K` erases the line it
// prints on while `ESC [ n A` first walks up over the ones above — `base commit does not resolve` is
// the last line printed, so an escape there reaches every finding already on screen, and kk-handoff
// tells the drafting agent to fix what a finding names and never to argue with one.
//
// Every printing site is driven, because the guard belongs to the printer rather than to whichever
// message someone remembered: an earlier version escaped two sites and left three raw, and the case
// that covered it was green because its fixture left the tree clean and the base resolvable.
func TestNoLineLeavesTheGateCarryingAControlByte(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "evil\x1b[31mname")
	repo, path, git, err := newRepo(dir)
	if err != nil {
		t.Skipf("could not build a repository named %q, so this proves nothing: %v", dir, err)
	}
	// Almost never fires: APFS and ext4 both carry a raw 0x1b through mkdir and realpath.
	if !strings.Contains(path, "\x1b") {
		t.Skipf("the filesystem did not keep the escape in %q, so this proves nothing", path)
	}
	// Dirty, so `dirtyNote` speaks on every case in this block. No other case reads this repository.
	git.StatusLines = []string{"?? untracked.txt"}
	for _, tc := range []struct {
		name     string
		mutate   func(*draft)
		contains []string
	}{
		{
			// The path through `resolveBase` and `dirtyNote`, the two that printed it raw, plus the
			// `no repository named` finding that quotes it as the repair.
			name:   "a draft naming no repository, over a dirty tree with no base that resolves",
			mutate: func(d *draft) { d.start = "Base commit 0123456789ab and nobody else is live here." },
			contains: []string{
				"no repository named in: Where it starts",
				"base commit does not resolve",
				"does not travel",
			},
		},
		{
			// The path through the title finding, which needs the draft to have named the repository.
			name: "a draft naming the repository but opening with the wrong bracketed word",
			mutate: func(d *draft) {
				d.title = "[issue-tracker] Cut the mutation run down"
				d.start = "Base commit " + fixtureSHA + " in " + path + ". Nobody else is live."
			},
			contains: []string{"the title opens with [issue-tracker]", "does not travel"},
		},
		{
			// The draft's own bytes, on the two findings that quote a heading back.
			name:     "a draft whose headings carry the escape",
			mutate:   func(d *draft) { d.extra = "\n## Append\x1b[2K\nwhatever else I felt like adding" },
			contains: []string{"unknown section:", "does not travel"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := cleanDraft()
			d.start = "Base commit " + fixtureSHA + " in " + path + ". Nobody else is live."
			tc.mutate(&d)
			got, code := gateOver(t, d.text(), repo, git)
			expect(t, tc.name, got, code, 1, tc.contains, []string{"\x1b"})
		})
	}
}

// What a message quotes is bounded where it stands, and the line it becomes is bounded again. Both
// bounds exist so the words carrying the repair survive: cut only at the line, a long name would push
// `use [X]` — the one repair the message offers — off the end.
func TestAFindingIsBoundedWhereItQuotesTheDraft(t *testing.T) {
	long := strings.Repeat("z", 400)

	d := cleanDraft()
	d.title = "[" + long + "] Cut the mutation run down"
	got, code := gateOver(t, d.text(), fixtureRepo, fixtureGit)
	expect(t, "a very long opening bracketed word", got, code, 1,
		[]string{shell.CutMarker, "use [" + fixtureAbbrev + "]"}, nil)

	d = cleanDraft()
	d.extra = "\n## " + strings.Repeat("y", 900) + "\nwhatever else I felt like adding"
	got, code = gateOver(t, d.text(), fixtureRepo, fixtureGit)
	expect(t, "a heading longer than the line bound", got, code, 1, []string{shell.CutMarker}, nil)
	for _, line := range shell.SplitLines(got) {
		if len(line) > lineWidthCap {
			t.Errorf("a line left the gate at %d bytes, past the %d-byte bound: %.80s", len(line), lineWidthCap, line)
		}
	}

	// The path is the other field standing before the repair, and the tree rather than the draft
	// chooses how long it is. Long enough that the line bound alone would take `use [DR]` off the end:
	// cut only at 500, the repair is what the path pushes past it.
	//
	// The leaf is not called `repo`: that is fallbackName, which abbreviates to the same `R` a clone
	// named `repo` does, so the assertion could not tell the repository it built from the degenerate
	// answer a name with nothing in it falls back to.
	deep := filepath.Join(t.TempDir(), strings.Repeat("d", 150), strings.Repeat("e", 150), strings.Repeat("f", 150), "deep-repo")
	repo, path, git, err := newRepo(deep)
	if err != nil {
		t.Skipf("could not build a repository at %q, so this proves nothing: %v", deep, err)
	}
	d = cleanDraft()
	d.title = "[issue-tracker] Cut the mutation run down"
	d.start = "Base commit " + fixtureSHA + " in " + path + ". Nobody else is live."
	got, code = gateOver(t, d.text(), repo, git)
	expect(t, "a repository path longer than its bound", got, code, 1,
		[]string{shell.CutMarker, "use [DR]"}, nil)
}

// The name in hand belongs to the repository this process was pointed at, which is the draft's own
// only once the draft has named it. Run from one checkout over a correct draft about another, the
// comparison would tell a correct author to break a correct title.
func TestAPrefixGoesUnweighedWhereTheDraftNamesNoRepository(t *testing.T) {
	other, _, otherGit, err := newRepo(filepath.Join(t.TempDir(), "alpha"))
	if err != nil {
		t.Fatalf("building the second repository: %v — nothing was tested", err)
	}
	d := cleanDraft()
	d.title = "[" + fixtureAbbrev + "] Cut the mutation run down"
	got, code := gateOver(t, d.text(), other, otherGit)
	expect(t, "a correct title weighed from another checkout", got, code, 1,
		[]string{"no repository named in: Where it starts"}, []string{"the title opens with"})
}

// The prefix is held against an abbreviation only where there is one. A directory the gate is told
// is a work tree but that `repo-key` cannot name leaves the prefix unread, and the draft survives a
// comparison the gate could not make.

// The repository answers every other question and still yields no abbreviation. Its shared git dir
// is a directory with no HEAD in it, which is what `repo-key` refuses on.

// The case arranges that state instead of looking for it. It used to probe whether its temporary
// directory sat inside a clone and skip where it did, and a skip proves no point about the guard.
func TestAPrefixIsUnreadWhereTheRepositoryHasNoAbbreviation(t *testing.T) {
	dir, path, git, err := newRepo(filepath.Join(t.TempDir(), "unnameable"))
	if err != nil {
		t.Fatalf("building the repository: %v — nothing was tested", err)
	}
	git.Common = filepath.Join(dir, "shared")
	if err := os.MkdirAll(git.Common, 0o755); err != nil {
		t.Fatalf("building a shared git dir with no HEAD: %v — nothing was tested", err)
	}
	d := cleanDraft()
	d.title = "[issue-tracker] Cut the mutation run down"
	d.start = "Base commit " + fixtureSHA + " in " + path + ". Nobody else is live."
	got, code := gateOver(t, d.text(), dir, git)
	expect(t, "a prefix over a repository with no abbreviation", got, code, 0,
		nil, []string{"the title opens with", "prefix is", "still the template placeholder"})
}

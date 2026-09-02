// Cases for the handoff gate. Every check is paired with its negative control: the clean draft passes,
// and each case is that same draft broken in exactly one way. Without the pair, a gate that had
// stopped reading the file would satisfy every refusal case and fail nothing.
//
// The fixture is a real git repository with a real commit, because the two things the gate reaches
// outside the draft for are whether the base commit resolves and whether the tree is dirty. Stubbing
// git would prove only that the stub ran. One repository serves the whole suite, so nothing here runs
// in parallel: two cases would otherwise disagree about whether the tree is dirty.
//
// One guard has no case and is named rather than quietly absent. "could not resolve" sits behind a
// successful IsDir, so reaching it needs the directory to disappear between two statements, and no
// fixture worth building does that.
package handoffcheck

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The one repository every case runs against, its resolved path, and its commit. Built once in
// TestMain: `git init` plus a commit is the expensive part of a case, and nothing here changes what a
// later case reads except the dirty-tree pair, which puts the tree back.
var (
	fixtureRepo string
	fixturePath string
	fixtureSHA  string
)

func TestMain(m *testing.M) {
	base, err := os.MkdirTemp("", "handoff-check")
	if err != nil {
		fmt.Fprintln(os.Stderr, "handoff-check: no temporary directory, so nothing was tested:", err)
		os.Exit(2)
	}
	defer os.RemoveAll(base)

	// Named distinctively, not "repo": the gate refuses a draft naming the repository by basename, and
	// a fixture called "repo" would make that case pass on the word "repo" appearing anywhere.
	fixtureRepo = filepath.Join(base, "handoff-fixture")
	if fixtureRepo, fixturePath, fixtureSHA, err = newRepo(fixtureRepo); err != nil {
		fmt.Fprintln(os.Stderr, "handoff-check: no git fixture, so nothing was tested:", err)
		os.RemoveAll(base)
		os.Exit(2)
	}

	code := m.Run()
	os.RemoveAll(base)
	os.Exit(code)
}

// newRepo builds a git repository holding one empty commit, and answers with the path the gate will
// resolve for itself. os.MkdirTemp hands back a symlinked path on macOS, so a draft quoting the path
// as created would not match what the gate compares against.
func newRepo(dir string) (repo, resolved, sha string, err error) {
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return "", "", "", err
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "base"},
	} {
		if err = exec.Command("git", append([]string{"-C", dir}, args...)...).Run(); err != nil {
			return "", "", "", err
		}
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", "", "", err
	}
	resolved, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return "", "", "", err
	}
	return dir, resolved, strings.TrimSpace(string(out))[:12], nil
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
	return gateOver(t, d.text(), fixtureRepo)
}

func gateOver(t *testing.T, body, repo string) (string, int) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "case.md")
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatalf("writing the case draft: %v — nothing was tested", err)
	}
	return gateFile(file, repo)
}

func gateFile(file, repo string) (string, int) {
	var out, errOut bytes.Buffer
	code := Run("handoff-check.sh", file, repo, &out, &errOut)
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
	code := Run("handoff-check.sh", "", fixtureRepo, &out, &errOut)
	expect(t, "no draft argument", out.String()+errOut.String(), code, 2, []string{"usage:"}, nil)

	got, code := gateFile(filepath.Join(t.TempDir(), "absent.md"), fixtureRepo)
	expect(t, "a draft that is not there", got, code, 2, []string{"no such file"}, nil)

	empty := filepath.Join(t.TempDir(), "empty.md")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatalf("writing the empty draft: %v — nothing was tested", err)
	}
	got, code = gateFile(empty, fixtureRepo)
	expect(t, "an empty draft", got, code, 2, []string{"nothing was checked"}, nil)

	// A filled draft, not the empty one above. The draft guards run first, so an empty draft exits 2
	// before the repository is looked at, and these two cases would pass with the repository guards gone.
	filled := filepath.Join(t.TempDir(), "titleonly.md")
	if err := os.WriteFile(filled, []byte("# a draft the gate gets past\n"), 0o644); err != nil {
		t.Fatalf("writing the filled draft: %v — nothing was tested", err)
	}
	got, code = gateFile(filled, t.TempDir())
	expect(t, "a repo that is not a work tree", got, code, 2, []string{"not a git work tree"}, nil)

	got, code = gateFile(filled, filled)
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
	got, code := gateFile(path, fixtureRepo)
	expect(t, "a draft this process cannot read", got, code, 2, []string{"cannot read"}, nil)
}

// The control. If this stops passing, no case below proves anything.
func TestCompleteDraftPasses(t *testing.T) {
	got, code := gate(t, cleanDraft())
	expect(t, "a complete draft", got, code, 0, nil, []string{"missing section"})
}

// The dirty-tree advisory: a note, not a finding, so a clean draft over a dirty repo still exits 0.
func TestDirtyTreeIsANoteAndNotAFinding(t *testing.T) {
	untracked := filepath.Join(fixtureRepo, "untracked.txt")
	if err := os.WriteFile(untracked, []byte("x\n"), 0o644); err != nil {
		t.Fatalf("dirtying the fixture: %v — nothing was tested", err)
	}
	got, code := gate(t, cleanDraft())
	expect(t, "a dirty repository", got, code, 0, []string{"does not travel"}, nil)

	if err := os.Remove(untracked); err != nil {
		t.Fatalf("cleaning the fixture: %v — the case below cannot measure", err)
	}
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
			name:     "the template title placeholder",
			mutate:   func(d *draft) { d.title = "<one imperative line: the work>" },
			want:     1,
			contains: []string{"title line is still the template placeholder"},
		},
		{
			// The control for the anchored placeholder test: an angle bracket inside a real title is
			// not the placeholder, and refusing it would send the agent to reword a sound line.
			name:   "a real title holding an angle bracket",
			mutate: func(d *draft) { d.title = "Cut the mutation run to <10 minutes" },
			want:   0,
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
	repo, path, sha, err := newRepo(dir)
	if err != nil {
		t.Skipf("could not build a repository named %q, so this proves nothing: %v", dir, err)
	}
	d := cleanDraft()
	d.start = "Base commit " + sha + " in " + path + ". Nobody else is live."
	got, code := gateOver(t, d.text(), repo)
	expect(t, "a correct draft in a repository whose name holds escapes", got, code, 0,
		nil, []string{"--injected-token", "no repository named"})
}

// The template and the gate each hold the seven headings, and nothing else compares them. Rename one
// in either and every future draft is refused, with the drift surfacing only at the next real handoff.
// Running the gate over the shipped template is that comparison: the leftover comments prove the scan
// reached the slots, and neither drift finding may appear.
func TestShippedTemplateMatchesTheHeadingsTheGateRequires(t *testing.T) {
	template := filepath.Join("..", "..", "skills", "kk-handoff", "handoff-prompt.md")
	if _, err := os.Stat(template); err != nil {
		t.Fatalf("cannot reach %s, so the drift case did not run: %v", template, err)
	}
	got, code := gateFile(template, fixtureRepo)
	expect(t, "the shipped template", got, code, 1,
		[]string{"template comment left"}, []string{"missing section:", "unknown section:"})
}

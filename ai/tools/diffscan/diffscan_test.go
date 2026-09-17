package diffscan

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kk-flavor/tools/repo/repotest"
)

const crlfBody = "// one\r\n// two\r\nx := 1\r\n"
const lfBody = "// one\n// two\nx := 1\n"

func countedLines(t *testing.T, walk func(*Result, func(AddedLine)) error) (map[string]int, Result) {
	t.Helper()
	var result Result
	seen := map[string]int{}
	if err := walk(&result, func(added AddedLine) {
		if added.Text != "" {
			seen[added.File]++
		}
	}); err != nil {
		t.Fatalf("the walk failed, so this case measured nothing: %v", err)
	}
	return seen, result
}

// A repository whose untracked files are on disk as well as in the listing. bodyToScan opens each one,
// so a case that only listed them would be measuring the open failing.
func untrackedRepo(t *testing.T, files map[string]string) (*repotest.Fake, string) {
	t.Helper()
	root := t.TempDir()
	fake := repotest.New(root)
	for name, body := range files {
		writeFile(t, filepath.Join(root, name), body)
		fake.AddUntracked(name)
	}
	return fake, root
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("could not make the directory for the fixture %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("could not write the fixture %s: %v", path, err)
	}
}

func TestTheUntrackedArmReadsCRLFLikeTheDiffArm(t *testing.T) {
	fake, root := untrackedRepo(t, map[string]string{"crlf.go": crlfBody, "lf.go": lfBody})

	seen, result := countedLines(t, func(r *Result, visit func(AddedLine)) error {
		return r.WalkUntracked(fake, root, Options{MaxFileBytes: 1 << 20}, visit)
	})

	if seen["lf.go"] != 3 {
		t.Fatalf("the LF control yielded %d countable lines, wanted 3 — the fixture is wrong, not the code", seen["lf.go"])
	}
	if seen["crlf.go"] != 3 {
		t.Errorf("the CRLF file yielded %d countable lines against the control's 3, and %d line(s) were counted binary — "+
			"a \\r is a line ending, not a control byte", seen["crlf.go"], result.BinaryLines)
	}
	if result.BinaryLines != 0 {
		t.Errorf("%d line(s) were dropped as binary over text files, so the run covered less than Reached claims", result.BinaryLines)
	}
}

// The other half of "agree": the same three lines through the diff arm. Without it the case above
// could be satisfied by changing both arms in the wrong direction.
func TestTheDiffArmReadsCRLF(t *testing.T) {
	diff := "diff --git a/crlf.go b/crlf.go\n--- a/crlf.go\n+++ b/crlf.go\n@@ -0,0 +1,3 @@\n" +
		"+// one\r\n+// two\r\n+x := 1\r\n"
	seen, result := countedLines(t, func(r *Result, visit func(AddedLine)) error {
		return r.WalkDiff([]byte(diff), visit)
	})
	if seen["crlf.go"] != 3 {
		t.Errorf("the diff arm yielded %d countable lines, wanted 3 (%d counted binary)", seen["crlf.go"], result.BinaryLines)
	}
}

func TestARealControlByteIsStillBinary(t *testing.T) {
	fake, root := untrackedRepo(t, map[string]string{"esc.go": "// fine\n// bad\x1bhere\n"})

	seen, result := countedLines(t, func(r *Result, visit func(AddedLine)) error {
		return r.WalkUntracked(fake, root, Options{MaxFileBytes: 1 << 20}, visit)
	})
	if seen["esc.go"] != 1 {
		t.Errorf("%d line(s) reached the visitor, wanted only the clean one", seen["esc.go"])
	}
	if result.BinaryLines != 1 {
		t.Errorf("%d line(s) counted binary, wanted the one holding an ESC", result.BinaryLines)
	}
}

// The port names an untracked file from the working tree ROOT, never from the directory the scan was
// asked about. Opened against that directory instead, every file in a run from a subdirectory fails to
// open and lands in SkippedUnread: the scan covers nothing and still exits clean, which is the one
// outcome the denominator exists to make impossible.
func TestAnUntrackedFileIsScannedFromASubdirectory(t *testing.T) {
	root := t.TempDir()
	fake := repotest.New(root)
	fake.AddUntracked("pkg/a.go")
	writeFile(t, filepath.Join(root, "pkg", "a.go"), lfBody)

	seen, result := countedLines(t, func(r *Result, visit func(AddedLine)) error {
		return r.WalkUntracked(fake, filepath.Join(root, "pkg"), Options{MaxFileBytes: 1 << 20}, visit)
	})
	if seen["pkg/a.go"] != 3 {
		t.Errorf("the file yielded %d countable lines to a scan run from pkg/, wanted 3", seen["pkg/a.go"])
	}
	if result.SkippedUnread != 0 {
		t.Errorf("%d file(s) were declined unread, so a run from a subdirectory scanned nothing at all "+
			"and still exited clean", result.SkippedUnread)
	}
}

func TestRevisionsNamedSeparatesThemFromPathspecs(t *testing.T) {
	for _, row := range []struct {
		name            string
		args            []string
		named, pathspec []string
	}{
		{"nothing at all", nil, nil, nil},
		{"revisions only", []string{"HEAD", "origin/main"}, []string{"HEAD", "origin/main"}, nil},
		{"pathspecs only, which names no revision", []string{"--", "src/"}, []string{}, []string{"src/"}},
		{"both halves", []string{"HEAD", "--", "a.go", "b.go"}, []string{"HEAD"}, []string{"a.go", "b.go"}},
		{"a bare separator", []string{"--"}, []string{}, []string{}},
	} {
		t.Run(row.name, func(t *testing.T) {
			named, pathspec := RevisionsNamed(row.args)
			if !slices.Equal(named, row.named) {
				t.Errorf("revisions = %v, wanted %v", named, row.named)
			}
			if !slices.Equal(pathspec, row.pathspec) {
				t.Errorf("pathspecs = %v, wanted %v", pathspec, row.pathspec)
			}
		})
	}
}

// What this package hands the port, which nothing the port hands back reflects: a patch is answered
// verbatim, so a case reading the answer could not tell a dropped pathspec from a kept one.
type patchAsked struct {
	*repotest.Fake
	revisions, pathspec []string
}

func (p *patchAsked) Patch(dir string, revisions, pathspec []string) ([]byte, error) {
	p.revisions, p.pathspec = revisions, pathspec
	return p.Fake.Patch(dir, revisions, pathspec)
}

// The whole point of the split. That `git diff HEAD -- a.go` then shows a staged change and leaves the
// rest of the tree out is git's own behaviour, and `repo/exec_test.go` drives Patch against a real
// repository for it; what belongs here is which revisions and pathspecs this package asks with.
func TestAPathspecScanStillDefaultsToHead(t *testing.T) {
	asked := &patchAsked{Fake: repotest.New("/repo")}

	if _, err := Diff(asked, "/repo", nil); err != nil {
		t.Fatalf("the bare form failed: %v", err)
	}
	if !slices.Equal(asked.revisions, []string{"HEAD"}) {
		t.Errorf("the bare form asked for %v rather than HEAD, so it diffed against the index. That "+
			"reports a clean tree over real work and exits 0", asked.revisions)
	}

	if _, err := Diff(asked, "/repo", []string{"--", "a.go"}); err != nil {
		t.Fatalf("the pathspec form failed: %v", err)
	}
	if !slices.Equal(asked.revisions, []string{"HEAD"}) {
		t.Errorf("`-- a.go` asked for %v, so it diffed against the index rather than HEAD. That reports "+
			"a clean tree over real work and exits 0", asked.revisions)
	}
	if !slices.Equal(asked.pathspec, []string{"a.go"}) {
		t.Errorf("`-- a.go` reached the port as the pathspec %v, wanted [a.go] — the port supplies the "+
			"separator itself, and the scope the caller asked for is otherwise silently ignored", asked.pathspec)
	}
}

// An argument that is BOTH a path on disk and a revision is the whole reason the scan asks git before
// refusing one. Refused, a legal invocation is turned away; accepted without asking, `git diff <path>`
// diffs against the INDEX and the scan runs over a change set nobody asked about.
func TestAPathIsRefusedOnlyWhereItNamesNoRevision(t *testing.T) {
	root := t.TempDir()
	fake := repotest.New(root)
	fake.Commit("main", nil)
	writeFile(t, filepath.Join(root, "main", "keep.go"), lfBody)
	writeFile(t, filepath.Join(root, "notes.txt"), "")

	if err := RefuseNonRevisions(fake, []string{"main"}, root); err != nil {
		t.Errorf("`main` was refused though it names a branch, so an invocation git would have "+
			"answered is turned away: %v", err)
	}
	if err := RefuseNonRevisions(fake, []string{"notes.txt"}, root); err == nil {
		t.Error("`notes.txt` was accepted though it names no revision — that scans the diff against the " +
			"INDEX, which is a different change set and exits 0 over a real hit")
	}
}

func TestAnUntrackedSecretNamedFileIsNeverRead(t *testing.T) {
	const secret = "AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMIK7MDENGbPxRfiCYEXAMPLEKEY"
	fake, root := untrackedRepo(t, map[string]string{".env": secret + "\n", "plain.go": lfBody})

	walk := func(skipSecretNamed bool) (map[string][]string, []string, Result) {
		t.Helper()
		var result Result
		var announced []string
		delivered := map[string][]string{}
		opts := Options{
			MaxFileBytes:    1 << 20,
			SkipSecretNamed: skipSecretNamed,
			Announce:        func(line string) { announced = append(announced, line) },
		}
		if err := result.WalkUntracked(fake, root, opts, func(added AddedLine) {
			delivered[added.File] = append(delivered[added.File], added.Text)
		}); err != nil {
			t.Fatalf("the walk failed, so this case measured nothing: %v", err)
		}
		return delivered, announced, result
	}

	if control, _, _ := walk(false); !slices.Contains(control[".env"], secret) {
		t.Fatalf("the option-off control delivered %d line(s) of .env and none of them the secret, so the "+
			"fixture is wrong rather than the guard right", len(control[".env"]))
	}

	delivered, announced, result := walk(true)
	if len(delivered[".env"]) != 0 {
		t.Errorf("%d line(s) of .env reached the visitor, so the secret is on its way into whatever the "+
			"caller prints: %q", len(delivered[".env"]), delivered[".env"])
	}
	if len(delivered["plain.go"]) == 0 {
		t.Errorf("plain.go was declined too, so the guard refuses more than the names it is for")
	}
	if result.Reached != 1 {
		t.Errorf("Reached = %d, wanted 1 — the declined file was counted as covered, or plain.go was missed",
			result.Reached)
	}
	if result.SkippedUnread != 1 {
		t.Errorf("SkippedUnread = %d, wanted 1 — a decline outside the tally lets the summary claim a "+
			"denominator it never covered", result.SkippedUnread)
	}
	if len(announced) != 1 || !strings.Contains(announced[0], "its name marks it as secret-bearing") {
		t.Errorf("the skip was announced as %q, wanted one notice naming the reason", announced)
	}
}

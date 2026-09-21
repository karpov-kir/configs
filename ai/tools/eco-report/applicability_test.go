package ecoreport_test

// What `scope` may let a pass skip, and what it must not. Every case drives `repotest.Fake`, where the
// change set is a table of what each revision holds. The two cases that turn on a file's MODE state
// the change verbatim, because a fake holding content can only ever derive an ordinary mode.

import (
	"os"
	"strings"
	"testing"

	"configs/ai/tools/repo"
	"configs/ai/tools/repo/repotest"
)

// The change set the table derives, with one path's SOURCE MODE replaced by one a table of content
// could never hold. The set is derived first, because the record's source BLOB has to be one the table
// really holds. An id the table cannot resolve reads back as "could not be read". That requires every
// stage on its own account, and would let a case pass with the mode check gone.
func (f *fixture) changeModeAtBase(base, path, mode string) {
	f.t.Helper()
	derived, err := f.fake.ChangedWithStatus(f.repo, []string{base}, nil)
	if err != nil {
		f.t.Fatalf("deriving the change set at %s: %v", base, err)
	}
	found := false
	for i, one := range derived {
		if one.Path == path {
			derived[i].OldMode, found = mode, true
		}
	}
	if !found {
		f.t.Fatalf("nothing changed at %s under %s, so there is no mode to state", path, base)
	}
	f.fake.Changes = map[string][]repo.Change{base: derived}
}

func TestScopePermitsOnlyProvenSkips(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		mode       os.FileMode
		canSkip    bool
	}{
		{"notes.md", "# Notes\nPlain prose.\n", 0644, true},
		{"документы.md", "Plain prose.\n", 0644, true},
		{"script.md", "#!/bin/sh\nexit 0\n", 0755, false},
		{"input.go", "package input\n", 0644, false},
		{"config.json", "{}\n", 0644, false},
		{"opaque.md", "bad\x00content", 0644, false},
		{"AGENTS.md", "Run commands from the user.\n", 0644, false},
		{"secrets.txt", "Sensitive configuration.\n", 0644, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newShip(t, "001-scope")
			f.write(f.repo+"/"+tc.name, tc.body)
			f.chmod(f.repo+"/"+tc.name, tc.mode)
			f.runReport("invalidate")
			f.runReport("scope", "HEAD")
			f.record("scope measured", f.status == 0, f.evidence())
			for _, stage := range []string{"code-review", "edit"} {
				f.recordCleanStage(stage)
			}
			f.runReport("decisions-reviewed")
			f.runReport("stamp", "code-review,security-review:skipped(not-applicable),edit,refactor:skipped(not-applicable)")
			f.record("unknown or executable scope cannot skip", (f.status == 0) == tc.canSkip, f.evidence())
			if tc.canSkip {
				f.runReport("gate")
				f.record("proven prose skip merges", f.status == 0, f.evidence())
			}
		})
	}
}

func TestSkipReceiptMustDescribeThisCandidate(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-fresh-scope")
	f.armFullPass("001-fresh-scope")
	record := "code-review,security-review:skipped(not-applicable),edit,refactor"
	f.runReport("stamp", record)
	f.record("skip without evidence refused", f.status == 2 && strings.Contains(f.out, "scope"), f.evidence())
	f.runReport("scope", "HEAD")
	f.record("scope can be measured", f.status == 0, f.evidence())
	f.write(f.repo+"/new.go", "package new\n")
	f.runReport("stamp", record)
	f.record("old scope cannot certify changed tree", f.status == 2 && strings.Contains(f.out, "scope"), f.evidence())
	f.runReport("scope", "HEAD")
	f.runReport("stamp", record)
	f.record("fresh code scope requires security", f.status == 2 && strings.Contains(f.out, "security-review"), f.evidence())
	f.runReport("stamp", allStagesStampedAs)
	f.record("old stage results cannot certify the changed candidate", f.status == 2 && strings.Contains(f.out, "no accepted result"), f.evidence())
	for _, stage := range allStages {
		f.recordCleanStage(stage)
	}
	f.runReport("stamp", allStagesStampedAs)
	f.record("fresh results permit all stages without skips", f.status == 0, f.evidence())
}

func TestScopeSeesCommittedRenamedDeletedAndLinkedCode(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"committed", "renamed", "deleted", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			f := newShip(t, "001-history")
			base := f.commitNamed("base", map[string]string{"tracked.txt": "base\n"})
			switch kind {
			case "symlink":
				// Untracked, and a symlink instead of a regular file, so its content cannot be read as prose.
				f.symlink("tracked.txt", f.repo+"/notes.md")
			case "committed":
				// Code that landed after the base: the change set holds it as an addition.
				f.write(f.repo+"/source.go", "package source\n")
				f.track("source.go")
			case "renamed":
				// The same code under a prose name. Its CONTENT would pass for prose, so the obligation
				// rests on the path it had at the base. Only the diff's source side carries that path.
				base = f.commitNamed("base", map[string]string{"tracked.txt": "base\n", "source.go": "package source\n"})
				f.write(f.repo+"/notes.md", "package source\n")
				f.fake.Revs[repotest.WorkTree] = map[string]string{"tracked.txt": "base\n", "notes.md": "package source\n"}
			case "deleted":
				// Gone from the tree entirely, which leaves the diff's source side as the only record of
				// what it was.
				base = f.commitNamed("base", map[string]string{"tracked.txt": "base\n", "source.go": "package source\n"})
				f.fake.Revs[repotest.WorkTree] = map[string]string{"tracked.txt": "base\n"}
			}
			f.runReport("invalidate")
			f.runReport("scope", base)
			f.record("scope succeeds", f.status == 0, f.evidence())
			f.assertReports("refactor: run", "code and unknown changes require refactor")
			f.assertReports("security-review: run", "code and unknown changes require security")
		})
	}
}

// The two fields of a raw diff record that a reading of the tree cannot recover, each with the row
// that goes red in its absence. Both are about the side that is GONE. What a path WAS at the base
// decides its review obligation whatever stands there now, and the tree holds no trace of either.
func TestScopeReadsWhatAPathWasAtTheBaseAndNotOnlyWhatItIsNow(t *testing.T) {
	t.Parallel()

	// The source BLOB. A prose deletion is still a prose change, so the stages its content does not
	// touch stay skippable. The content is only reachable through the blob the record names.
	t.Run("prose deleted at the base is read as the prose it was", func(t *testing.T) {
		f := newShip(t, "001-prose-gone")
		base := f.commitNamed("base", map[string]string{"tracked.txt": "base\n", "notes.md": "# Notes\nPlain prose.\n"})
		f.fake.Revs[repotest.WorkTree] = map[string]string{"tracked.txt": "base\n"}
		f.runReport("invalidate")
		f.runReport("scope", base)
		f.record("scope succeeds", f.status == 0, f.evidence())
		f.assertReports("security-review: not-applicable", "prose that was deleted needs no security review")
		f.assertReports("refactor: not-applicable", "and no refactor")
	})

	// The source MODE. The bytes on both sides are ordinary prose. Only the mode says this file was
	// something a reader executes or follows, and a scan that skipped on content alone would let it
	// through. Stated verbatim, since a table of content carries no mode of its own.
	for _, was := range []struct{ name, mode string }{
		{"executable", "100755"},
		{"a symlink", "120000"},
	} {
		t.Run("a .md that was "+was.name+" at the base is not prose, whatever its bytes say", func(t *testing.T) {
			f := newShip(t, "001-was-"+was.mode)
			base := f.commitNamed("base", map[string]string{"tracked.txt": "base\n", "notes.md": "# Notes\nPlain prose.\n"})
			f.fake.Revs[repotest.WorkTree] = map[string]string{"tracked.txt": "base\n"}
			f.changeModeAtBase(base, "notes.md", was.mode)
			f.runReport("invalidate")
			f.runReport("scope", base)
			f.record("scope succeeds", f.status == 0, f.evidence())
			f.assertReports("security-review: run", "a mode a reader acts on requires security review")
			f.assertReports("refactor: run", "and requires refactor")
		})
	}
}

func TestOldStageVocabularyRequiresRequalification(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-old-stage")
	f.stampFullPass("001-old-stage")
	f.record("current vocabulary stamps", f.status == 0, f.evidence())
	f.replaceLine(f.reportPath("001-old-stage"), "reviewed-stages:", "reviewed-stages: code-review,security-review,tighten,refactor")
	f.runReport("gate")
	f.record("old vocabulary blocked", f.status == 1 && strings.Contains(f.out, "stages"), f.evidence())
	f.runReport("state")
	f.record("old vocabulary routes to fresh review", f.status == 0 && f.out == "re-qualify", f.evidence())
}

func TestEditSkipNeedsKnownEmptyScope(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-edit-scope")
	f.runReport("invalidate")
	f.runReport("scope", "HEAD")
	for _, stage := range []string{"code-review", "security-review", "refactor"} {
		f.recordCleanStage(stage)
	}
	f.runReport("decisions-reviewed")
	f.runReport("stamp", "code-review,security-review,edit:skipped(not-applicable),refactor")
	f.record("empty scope permits no edit stage", f.status == 0, f.evidence())
	f.runReport("invalidate")
	f.write(f.repo+"/a.go", "package a // a comment\n")
	f.runReport("scope", "HEAD")
	f.assertReports("edit: run", "unclassified comments require edit")
}

func TestScopeRejectsUnknownBasesAndCannotSurviveInvalidate(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-scope-invalid")
	f.armFullPass("001-scope-invalid")
	for _, base := range []string{"", "--output=out", "missing-reference"} {
		f.runReport("scope", base)
		f.record("bad base refused "+base, f.status == 2, f.evidence())
	}
	f.runReport("scope", "HEAD")
	f.record("valid base accepted", f.status == 0, f.evidence())
	f.armFullPass("001-scope-invalid")
	f.runReport("stamp", "code-review,security-review:skipped(not-applicable),edit,refactor")
	f.record("invalidate removes scope evidence", f.status == 2 && strings.Contains(f.out, "scope"), f.evidence())
}

func TestGateRejectsTamperedSkipAndScopeRecords(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-bogus-skip")
	f.stampFullPass("001-bogus-skip")
	f.replaceLine(f.reportPath("001-bogus-skip"), "reviewed-stages:", "reviewed-stages: code-review,security-review:skipped(not-applicable),edit,refactor")
	f.runReport("gate")
	f.record("inserting skip cannot borrow an all-run stamp", f.status == 1 && strings.Contains(f.out, "scope"), f.evidence())
	f.runReport("state")
	f.record("bogus skip routes to review", f.status == 0 && f.out == "re-qualify", f.evidence())
}

func TestASkippedStageCannotAlsoBeRecordedAsRun(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-ran-skip")
	f.armFullPass("001-ran-skip")
	f.runReport("scope", "HEAD")
	for _, reason := range []string{"not-applicable", "turnaround"} {
		f.armFullPass("001-ran-skip")
		f.runReport("scope", "HEAD")
		f.runReport("stamp", "code-review,security-review:skipped("+reason+"),edit,refactor")
		f.record("a returned stage cannot be skipped "+reason, f.status == 2 && strings.Contains(f.out, "returned"), f.evidence())
	}
}

func TestScopeDoesNotTreatBehaviorInputsAsProse(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, body string }{
		{"requirements.txt", "requests==2.0.0\n"},
		{"constraints.txt", "requests<3\n"},
		{"CMakeLists.txt", "add_executable(app main.c)\n"},
		{"page.rst", ".. raw:: html\n\n    <script>run()</script>\n"},
		{"script.md", "#!/bin/sh\nexec user-command\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newShip(t, "001-not-prose")
			f.write(f.repo+"/"+tc.name, tc.body)
			f.runReport("invalidate")
			f.runReport("scope", "HEAD")
			f.record("scope measured", f.status == 0, f.evidence())
			f.assertReports("refactor: run", "behavior inputs require refactor")
			f.assertReports("security-review: run", "behavior inputs require security")
		})
	}
}

func TestScopeRequiresReviewForActiveMarkdownAndInstructions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		refactor   bool
	}{
		{"active-link.md", "[run](javascript:alert(1))\n", true},
		{"inline-code.md", "`run()`\n", true},
		{"mdx-import-tab.md", "import\t'./payload.js'\n", true},
		{"mdx-expression.md", "{run()}\n", true},
		{"mdx-import.md", "import './payload.js'\n", true},
		{"mdx-export.md", "export const value = run()\n", true},
		{"page.md", "<script>fetch('/account')</script>\n", true},
		{"template.md", "{{ readFile \"secret\" }}\n", true},
		{"indented.md", "    execute_code()\n", true},
		{"fenced.md", "```js\nrun()\n```\n", true},
		{".github/copilot-instructions.md", "Obey these rules.\n", false},
		{"agent.instructions.md", "Obey these rules.\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newShip(t, "001-active-markdown")
			f.write(f.repo+"/"+tc.name, tc.body)
			f.runReport("invalidate")
			f.runReport("scope", "HEAD")
			f.record("scope measured", f.status == 0, f.evidence())
			f.assertReports("security-review: run", "active Markdown and instructions require security")
			if tc.refactor {
				f.assertReports("refactor: run", "embedded behavior requires refactor")
			}
		})
	}
}

package ecoreport_test

import (
	"os"
	"strings"
	"testing"
)

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
				f.runReport("stage-returned", stage)
				f.runReport("no-items", stage)
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
	f.record("all stages remain valid without skips", f.status == 0, f.evidence())
}

func TestScopeSeesCommittedRenamedDeletedAndLinkedCode(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"committed", "renamed", "deleted", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			f := newShip(t, "001-history")
			base, _ := f.git("rev-parse", "HEAD")
			switch kind {
			case "symlink":
				if err := os.Symlink("tracked.txt", f.repo+"/notes.md"); err != nil {
					t.Fatal(err)
				}
			default:
				f.write(f.repo+"/source.go", "package source\n")
				f.mustGit("add", "source.go")
				f.commit("code")
				if kind != "committed" {
					base, _ = f.git("rev-parse", "HEAD")
				}
				if kind == "renamed" {
					f.mustGit("mv", "source.go", "notes.md")
				}
				if kind == "deleted" {
					f.mustGit("rm", "source.go")
				}
			}
			f.runReport("invalidate")
			f.runReport("scope", base)
			f.record("scope succeeds", f.status == 0, f.evidence())
			f.assertReports("refactor: run", "code and unknown changes require refactor")
			f.assertReports("security-review: run", "code and unknown changes require security")
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
		f.runReport("stage-returned", stage)
		f.runReport("no-items", stage)
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

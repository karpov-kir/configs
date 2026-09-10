package ecoreport_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestDecisionClassificationPreservesEntryAndSection(t *testing.T) {
	t.Parallel()
	for _, local := range []bool{false, true} {
		t.Run(fmt.Sprint(local), func(t *testing.T) {
			f := newShip(t, "001-sections")
			kind := "project-decisions"
			args := []string{"record"}
			path := recordFile(f, kind)
			if local {
				kind = "local-decisions"
				args = append(args, "--intent", "001-sections")
				path = localRecordFile(f, "001-sections", kind)
			}
			f.write(path, "# Decisions\n\n## Promotion candidates\n\n## Decisions\n\n7x | 2020-01-01 | unique 🍦 entry\n1x | 2021-01-01 | untouched\n")
			f.runReport(append(args, "classify", kind, "🍦", "candidate")...)
			content := f.read(path)
			candidate, decisions, _ := strings.Cut(content, "## Decisions")
			f.record("classification preserves entry count date and text", f.status == 0 && strings.Contains(candidate, "7x | 2020-01-01 | unique 🍦 entry") && !strings.Contains(decisions, "🍦"), f.evidence()+content)
			f.runReport(append(args, "append", kind, "unique 🍦 entry")...)
			content = f.read(path)
			candidate, decisions, _ = strings.Cut(content, "## Decisions")
			f.record("a duplicate append bumps the candidate in place", f.status == 0 && strings.Contains(candidate, "8x | "+today()+" | unique 🍦 entry") && !strings.Contains(decisions, "🍦"), f.evidence()+content)
			f.runReport(append(args, "revise", kind, "🍦", "revised 🍦 entry")...)
			content = f.read(path)
			candidate, _, _ = strings.Cut(content, "## Decisions")
			f.record("revision preserves candidate classification", f.status == 0 && strings.Contains(candidate, "8x | "+today()+" | revised 🍦 entry"), f.evidence()+content)
			f.runReport(append(args, "classify", kind, "🍦", "decision")...)
			content = f.read(path)
			candidate, decisions, _ = strings.Cut(content, "## Decisions")
			f.record("classification can return an entry to decisions", f.status == 0 && !strings.Contains(candidate, "🍦") && strings.Contains(decisions, "8x | "+today()+" | revised 🍦 entry"), f.evidence()+content)
		})
	}
}

func TestDecisionRecordRejectsMalformedSections(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		"# Decisions\n1x | 2020-01-01 | selected\n",
		"## Decisions\n## Promotion candidates\n1x | 2020-01-01 | selected\n",
		"## Promotion candidates\n## Decisions\n## Decisions\n1x | 2020-01-01 | selected\n",
		"## Promotion candidates \n## Decisions\n1x | 2020-01-01 | selected\n",
	} {
		for _, operation := range []string{"append", "bump", "revise", "evict", "admit", "classify"} {
			f := newRepo(t)
			path := recordFile(f, "project-decisions")
			f.write(path, body)
			args := []string{"record", operation, "project-decisions", "selected"}
			if operation == "revise" || operation == "admit" {
				args = append(args, "replacement")
			}
			if operation == "classify" {
				args = append(args, "candidate")
			}
			f.runReport(args...)
			f.record(operation+" refuses malformed sections unchanged", f.status == 2 && f.read(path) == body, f.evidence()+f.read(path))
		}
	}
}

func TestDecisionCapIncludesPendingCandidates(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	path := recordFile(f, "project-decisions")
	body := "## Promotion candidates\n"
	for index := range 100 {
		body += fmt.Sprintf("%dx | 2020-01-01 | candidate %d\n", index+1, index)
	}
	body += "\n## Decisions\n"
	f.write(path, body)
	f.runReport("record", "append", "project-decisions", "new entry")
	f.record("candidates consume cap without automatic eviction or promotion", f.status == 2 && strings.Contains(f.out, "100 entries") && f.read(path) == body, f.evidence())
	f.runReport("record", "admit", "project-decisions", "candidate 99", "replacement")
	candidate, decisions, _ := strings.Cut(f.read(path), "## Decisions")
	f.record("explicit admission preserves the replaced entry section", f.status == 0 && strings.Contains(candidate, "1x | "+today()+" | replacement") && !strings.Contains(decisions, "replacement"), f.evidence()+f.read(path))
}

func TestClassifyOnlyAcceptsDecisionKindsAndKnownClassifications(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"project-playbook", "project-language", "constraints", "local-playbook", "local-language"} {
		f := newRepo(t)
		f.runReport("record", "classify", kind, "selector", "candidate")
		f.assertRefused("classification refuses " + kind)
	}
	f := newRepo(t)
	f.runReport("record", "classify", "project-decisions", "selector", "other")
	f.assertRefused("classification refuses an unknown target section")
	f.record("rejected classification creates no record", !f.exists(f.scratch()), f.evidence())
}

func TestRecordRejectsEveryLinkedParent(t *testing.T) {
	t.Parallel()
	for _, component := range []string{"for-agents", "intents", "intents/001-sections", "intents/001-sections/for-agents"} {
		t.Run(component, func(t *testing.T) {
			f := newRepo(t)
			outside := f.base + "/outside"
			f.mkdirAll(outside)
			path := f.scratch() + "/" + component
			parent := path[:strings.LastIndex(path, "/")]
			f.mkdirAll(parent)
			f.symlink(outside, path)
			args := []string{"record"}
			kind := "project-decisions"
			if component != "for-agents" {
				args = append(args, "--intent", "001-sections")
				kind = "local-decisions"
			}
			f.runReport(append(args, "append", kind, "must stay inside")...)
			f.record("a linked parent refuses without writing outside", f.status == 2 && len(f.find(outside)) == 1, f.evidence()+joinLines(f.find(outside)))
		})
	}
}

func TestLocalRecordCapReportsItsOwnBoundWithoutChoosingAnEviction(t *testing.T) {
	t.Parallel()
	f := newShip(t, "001-cap")
	path := localRecordFile(f, "001-cap", "local-decisions")
	body := "## Promotion candidates\n"
	for index := range 26 {
		body += fmt.Sprintf("1x | 2020-01-01 | pending %d\n", index)
	}
	body += "\n## Decisions\n"
	f.write(path, body)
	f.runReport("record", "--intent", "001-cap", "bump", "local-decisions", "pending 25")
	f.record("local over-cap note uses 25 while preserving all candidates", f.status == 0 && strings.Contains(f.out, "over its cap of 25") && strings.Count(f.read(path), "x | ") == 26 && strings.HasSuffix(f.read(path), "## Decisions\n"), f.evidence()+f.read(path))
	f.record("over-cap advice leaves the choice to the judge", strings.Contains(f.out, "Where it names nothing, the record stays over its cap") && strings.Contains(f.out, "Exit 2 is not an answer") && !strings.Contains(f.out, "pending 7"), f.evidence())
	f.record("over-cap advice retains promotion before eviction", strings.Contains(f.out, "promote what must not be lost"), f.evidence())
	f.runReport("record", "--intent", "001-cap", "evict", "local-decisions", "pending 0")
	f.record("one explicit eviction restores the local cap", f.status == 0 && strings.Count(f.read(path), "x | ") == 25, f.evidence())
	f.runReport("record", "--intent", "001-cap", "append", "local-decisions", "new decision")
	f.record("full local refusal quotes its own cap", f.status == 2 && strings.Contains(f.out, "25 entries, its cap") && !strings.Contains(f.read(path), "new decision"), f.evidence())
}

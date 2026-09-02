package ecoreport_test

// The template is read once per report and never written, so what these cases pin is what a drifted
// or planted one would put into every report scaffolded from it.

import (
	"testing"
)

func TestADriftedTemplateIsRefusedBeforeAnyReportIsScaffolded(t *testing.T) {
	t.Parallel()
	// The fixture first: an untouched copy must initialize. Without this, a copy the tool cannot read
	// at all would refuse every case below, and each refusal would pass for the wrong reason.
	intact := newShip(t, "001-intact-copy")
	intact.record("the copied skill dir initializes a report from its own template",
		intact.status == 0 && intact.isFile(intact.reportPath("001-intact-copy")), "")

	// The drift that gates clean: a placeholder outside the unstamped set puts a real-looking
	// reviewed-tree in every new report, which `gate` reads as a completed review of whatever tree matches.
	drifted := newRepo(t)
	drifted.runReport("check-ignore")
	drifted.replaceLine(drifted.templatePath(), "reviewed-tree:",
		"reviewed-tree: 1111111111111111111111111111111111111111")
	drifted.runReport("init", "001-drifted-placeholder")
	drifted.assertRefused("init refuses a template whose reviewed-tree placeholder reads as a completed review")
	drifted.assertReports("would gate clean", "and says every new report would gate clean")
	drifted.record("and scaffolded no report from the drifted template",
		!drifted.exists(drifted.reportPath("001-drifted-placeholder")), "")

	// A field the template stopped carrying: init stamps `intent:`, and gate and state read the other
	// three, so a report scaffolded without one answers from a line that is not there. Every field
	// assertTemplateStampable requires has a row here, or a field added to that list arrives with
	// nothing standing behind it.
	for _, missingField := range []string{"intent", "reviewed-tree", "reviewed-worktree", "reviewed-stages"} {
		stripped := newRepo(t)
		stripped.runReport("check-ignore")
		stripped.dropLines(stripped.templatePath(), missingField+":")
		stripped.runReport("init", "001-missing-"+missingField)
		stripped.assertRefused("init refuses a template with no '" + missingField + ":' line")
		stripped.assertReports("has no '"+missingField+":' line", "and names the missing '"+missingField+":' line")
		stripped.record("and scaffolded no report while '"+missingField+":' is missing",
			!stripped.exists(stripped.reportPath("001-missing-"+missingField)), "")
	}

	// The template is read and never written, so what the symlink guard stops is content from outside
	// the repo becoming a new report's frontmatter. A committed link arrives through someone else's
	// branch, and a forged `reviewed-tree` is what it would carry in.
	linked := newRepo(t)
	linked.runReport("check-ignore")
	linked.mkdirAll(linked.base + "/foreign")
	// A plausible template, placeholders included, so every other check on it passes and the symlink
	// guard is the only thing left between the link and a scaffolded report. A stub here would be
	// refused for its missing `intent:` line instead, and three of the four assertions below could no
	// longer fail.
	linked.write(linked.base+"/foreign/outside.md",
		"---\nintent: 002-attacker\nreviewed-tree: <hash>\nreviewed-stages: <stages>\n---\n\n# Decide\n\nSMUGGLED\n")
	linked.remove(linked.templatePath())
	linked.symlink(linked.base+"/foreign/outside.md", linked.templatePath())
	linked.runReport("init", "001-linked-template")
	linked.assertRefused("init refuses a template that is a symlink")
	linked.assertReports("is a symlink", "and says it will not read the template through one")
	linked.record("and scaffolded no report from the linked template",
		!linked.exists(linked.reportPath("001-linked-template")), "")
	// The write that could already have landed is the report, not the link's target: init only ever
	// reads the template. So this assertion names the harm, content from outside the repo reaching
	// .idsd/, rather than the shape of the refusal.
	smuggled := linked.filesContaining(linked.scratch(), "SMUGGLED")
	linked.record("and no content from outside the repo reached .idsd/", len(smuggled) == 0, joinLines(smuggled))

	// A template that is gone. Without this refusal the `intent:` guard fires on the failure to open
	// the file, and the message stops naming the real cause, so the message is what pins this one.
	absent := newRepo(t)
	absent.runReport("check-ignore")
	absent.remove(absent.templatePath())
	absent.runReport("init", "001-absent-template")
	absent.assertRefused("init refuses when the template is missing")
	absent.assertReports("template not found", "and names the missing template as the cause")
	absent.record("and scaffolded no report without a template",
		!absent.exists(absent.reportPath("001-absent-template")), "")
}

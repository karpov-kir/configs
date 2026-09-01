package ecocheck

import (
	"regexp"
	"strings"

	"kk-flavor/tools/shell"
)

// Direction inside the skill layer. The rule's home is ecosystem.md → **Family direction**: the any-repo
// family never names the workflow family, or anything it owns. This is its enforcement.
//
// scanDirection beside this file guards the tier above — the shared layer never naming a skill — and
// stops there, so nothing checked this one until a generic skill's own description had drifted into
// naming a workflow sibling and the directory that workflow keeps its state in.
//
// Why it matters more than tidiness: a description is what the model reads to choose a skill. One that
// discriminates itself against a sibling reads as a dangling reference in every tree that does not
// mount that sibling, which is most of them — worse than not discriminating at all.

// The two families ecosystem.md → **Conventions a new file joins** describes, as the prefixes that
// name them. Written out rather than derived, because no property of the names says which family works
// in any repo and which belongs to one workflow — the direction is the whole rule, so it has to be
// declared. scanFamilyDirection refuses a mounted skill outside both, so adding a third family fails
// here rather than going quietly unscanned.
const (
	anyRepoFamily  = "kk-"
	workflowFamily = "idsd-"
)

// The one skill allowed to name the other family, by the exception ecosystem.md → **Family direction** grants. Held
// as a name rather than sniffed out of the prose, so the exception is a decision recorded in one place
// instead of a phrase any file could start matching by accident.
const familyRouter = "kk-foreman"

// What the router's own file must cite to claim its exception. ecosystem.md says the exception is
// claimed in the skill's own file, and keying the exception on the name alone cannot tell a claimed one
// from a silent one — the router could stop explaining why it names the other family and nothing here
// would fail. Asserted through the tree's own citation form rather than a phrase, so it stays true when
// the wording is reworded.
const routerClaimCitation = "ecosystem.md → **Family direction**"

// The citation as it is really written: the canonical form ecosystem.md → **Conventions a new file
// joins** fixes puts the path in backticks, so the closing one sits between the filename and the arrow.
// Matching the bare string instead fails on every correctly-written citation in the tree.
//
// The section is the rule's OWN, and that is why the claim is checkable at all: **One home** is cited
// all over the tree for unrelated reasons, so requiring that one let any incidental citation stand in
// for a claim the router never made.
var routerClaimPattern = regexp.MustCompile("ecosystem\\.md`? → \\*\\*Family direction\\*\\*")

// What the workflow family keeps its state in: the family prefix as a dotted directory. Derived, so a
// renamed family does not leave this scan looking for a directory nobody writes any more.
func workflowStateDir() string { return "." + strings.TrimSuffix(workflowFamily, "-") }

func (c *checker) scanFamilyDirection() {
	workflowName := regexp.MustCompile(`\b` + regexp.QuoteMeta(workflowFamily) + `[A-Za-z0-9._-]*`)
	// Word-boundary on the left only: the state dir already starts with a dot, and `\b` before one
	// never matches.
	stateDir := regexp.MustCompile(regexp.QuoteMeta(workflowStateDir()) + `\b`)

	for _, name := range c.laneNames() {
		switch {
		case strings.HasPrefix(name, workflowFamily):
			// The permitted direction: a workflow skill may name and cite an any-repo one.
			continue
		case !strings.HasPrefix(name, anyRepoFamily):
			c.add("skill '" + name + "' is in neither declared family (" + anyRepoFamily + ", " +
				workflowFamily + ") — nothing checks its citation direction (ecosystem.md → **Family direction**)")
			continue
		case name == familyRouter:
			c.assertRouterClaimsItsException(name)
			continue
		}
		c.reportFamilyLeaks(name, workflowName, stateDir)
	}
}

// One any-repo skill's whole directory, prose and scripts alike. Not just SKILL.md: a script's comment
// steers the agent reading it exactly as the skill file does, and a reference file is read on the same
// trigger.
//
// Fences are not skipped, matching scanDirection's reasoning — a name inside one steers its reader too,
// and a description is never fenced anyway.
func (c *checker) reportFamilyLeaks(name string, workflowName, stateDir *regexp.Regexp) {
	dir := shell.Join(c.root.Skills(), name)
	found := 0
	for _, file := range c.filesNamed(dir, "*.md", "*.sh") {
		lines, err := c.readLines(file)
		if err != nil {
			continue
		}
		safeFile := shell.Oneline(file)
		for _, pattern := range []*regexp.Regexp{workflowName, stateDir} {
			for _, hit := range grepNumbered(lines, pattern) {
				found++
				if found <= findingCap {
					c.add("any-repo skill names the workflow family: " + safeFile + ":" +
						shell.Oneline(hit.String()) +
						" — name the capability, not the skill or directory that has it (ecosystem.md → **Family direction**)")
				} else if found == findingCap+1 {
					c.reportBoundReached("any-repo skill names the workflow family", safeFile)
				}
			}
		}
	}
}

// The router's exception is real only while its file says so. Without this, `familyRouter` is a blanket
// pass: every finding in that skill is suppressed and nothing requires it to explain itself, which is
// the opposite of what ecosystem.md asks for.
func (c *checker) assertRouterClaimsItsException(name string) {
	path := shell.Join(shell.Join(c.root.Skills(), name), "SKILL.md")
	lines, err := c.readLines(path)
	if err != nil {
		c.add("router " + shell.Oneline(path) + " could not be read, so its family exception is unverified" +
			" — every cross-family finding in it is suppressed on the strength of a file nothing opened")
		return
	}
	for _, line := range lines {
		if routerClaimPattern.MatchString(line) {
			return
		}
	}
	c.add("router " + shell.Oneline(path) + " names the workflow family but claims no exception" +
		" — cite " + routerClaimCitation + " in it, or stop naming that family")
}

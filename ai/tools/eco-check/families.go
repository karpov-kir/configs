package ecocheck

import (
	"regexp"
	"strings"

	"kk-flavor/tools/shell"
)

// Direction inside the lane trees. The rule's home is ecosystem.md → **Family direction**: the any-repo
// family never names the workflow family, or anything it owns. This is its enforcement.
//
// What a leak costs: a description is what the model reads to choose a skill. One that discriminates
// itself against a sibling reads as a dangling reference in every tree that does not mount that
// sibling, which is most of them — worse than not discriminating at all.

// The two families ecosystem.md → **Conventions a new file joins** describes, as the prefixes that
// name them. Written out rather than derived, because no property of the names says which family works
// in any repo and which belongs to one workflow — the direction is the whole rule, so it has to be
// declared. scanFamilyDirection refuses a mounted skill outside both, so adding a third family fails
// here rather than going quietly unscanned.
const (
	anyRepoFamily  = "kk-"
	workflowFamily = "idsd-"
)

// The quote on the first head is what tells it from the other `skill…` heads: the tree's own
// directory name follows it.
const (
	skillInNeitherFamily       = "skill '"
	anyRepoNamesWorkflowFamily = "any-repo lane names the workflow family"
	familyRouterFinding        = "router "
)

// The one skill allowed to name the other family, by the exception ecosystem.md → **Family direction** grants. Held
// as a name rather than sniffed out of the prose, so the exception is a decision recorded in one place
// instead of a phrase any file could start matching by accident.
const familyRouter = "kk-foreman"

// What the router's own file must cite to claim its exception, since ecosystem.md says the exception is
// claimed in the skill's own file. A citation rather than a phrase, so it survives the prose around it
// being reworded.
const routerClaimCitation = "ecosystem.md → **Family direction**"

// The citation as it is really written: the canonical form ecosystem.md → **Conventions a new file
// joins** fixes puts the path in backticks, so the closing one sits between the filename and the arrow.
// Matching the bare string instead fails on every correctly-written citation in the tree.
//
// The section is the rule's OWN, and that is why the claim is checkable at all: **One home** is cited
// all over the tree for unrelated reasons, so requiring that one let any incidental citation stand in
// for a claim the router never made.
var routerClaimPattern = regexp.MustCompile("ecosystem\\.md`? → \\*\\*Family direction\\*\\*")

// Derived, so a renamed family does not leave this scan looking for a directory nobody writes any more.
func workflowStateDir() string { return "." + strings.TrimSuffix(workflowFamily, "-") }

// The one directory of the worker tree the workflow family owns. Everything else under `workers/` is
// any-repo by default, so a worker added tomorrow is scanned rather than unclassified: there is no
// third case here for a neither-family finding to catch.
func workflowWorkerDir() string { return strings.TrimSuffix(workflowFamily, "-") }

// The files a lane steers its reader with, in either tree: prose, and the scripts whose comments steer
// just as surely. One list rather than the pair at each call site, so the mutation that proves scripts
// are really read has one anchor to narrow.
var laneProseAndScripts = []string{"*.md", "*.sh"}

func (c *checker) scanFamilyDirection() {
	workflowName := regexp.MustCompile(`\b` + regexp.QuoteMeta(workflowFamily) + `[A-Za-z0-9._-]*`)
	// Word-boundary on the left only: the state dir already starts with a dot, and `\b` before one
	// never matches.
	stateDir := regexp.MustCompile(regexp.QuoteMeta(workflowStateDir()) + `\b`)

	for _, name := range c.laneNames() {
		switch {
		case strings.HasPrefix(name, workflowFamily):
			continue
		case !strings.HasPrefix(name, anyRepoFamily):
			c.add(skillInNeitherFamily + name + "' is in neither declared family (" + anyRepoFamily + ", " +
				workflowFamily + ") — nothing checks its citation direction (ecosystem.md → **Family direction**)")
			continue
		case name == familyRouter:
			c.assertRouterClaimsItsException(name)
			continue
		}
		c.reportSkillFamilyLeaks(name, workflowName, stateDir)
	}
	c.reportWorkerFamilyLeaks(workflowName, stateDir)
}

// The same direction across the worker tree. Without it the whole layer is unscanned: a worker is
// dispatched by whichever skill needs it, so an any-repo one naming an `idsd-*` skill or the `.idsd/`
// directory reads as a dangling reference in every repository that mounts no workflow skill.
//
// No router exception here: the one skill that routes between families has a door, and a worker has
// nothing to route.
func (c *checker) reportWorkerFamilyLeaks(workflowName, stateDir *regexp.Regexp) {
	var anyRepo []string
	for _, tree := range c.workerLaneTrees() {
		owned := shell.Join(tree, workflowWorkerDir())
		for _, file := range c.filesNamed(tree, laneProseAndScripts...) {
			if file == owned || strings.HasPrefix(file, owned+"/") {
				continue
			}
			anyRepo = append(anyRepo, file)
		}
	}
	c.reportFamilyLeaksIn(anyRepo, workflowName, stateDir)
}

func (c *checker) reportSkillFamilyLeaks(name string, workflowName, stateDir *regexp.Regexp) {
	c.reportFamilyLeaksIn(c.filesNamed(shell.Join(c.root.Skills(), name), laneProseAndScripts...), workflowName, stateDir)
}

// One call's any-repo files, from whichever tree they came: one skill's directory, or every any-repo
// worker at once. The bound is per call, so one leaking skill cannot exhaust the next skill's
// allowance. The worker tree is a single call, so its prompts share one allowance between them.
//
// Fences are not skipped, matching scanDirection's reasoning — a name inside one steers its reader too,
// and a description is never fenced anyway.
func (c *checker) reportFamilyLeaksIn(files []string, workflowName, stateDir *regexp.Regexp) {
	found := 0
	for _, file := range files {
		lines, err := c.readLines(file)
		if err != nil {
			continue
		}
		safeFile := shell.Oneline(file)
		for _, pattern := range []*regexp.Regexp{workflowName, stateDir} {
			for _, hit := range grepNumbered(lines, pattern) {
				c.addBounded(&found, anyRepoNamesWorkflowFamily, safeFile, func() string {
					return safeFile + ":" + shell.Oneline(hit.String()) +
						" — name the capability, not the skill or directory that has it (ecosystem.md → **Family direction**)"
				})
			}
		}
	}
}

// The router's exception is real only while its file says so. Without this, `familyRouter` is a blanket
// pass: every finding in that skill is suppressed and nothing requires it to explain itself, which is
// the opposite of what ecosystem.md asks for.
func (c *checker) assertRouterClaimsItsException(name string) {
	path := c.skillFilePath(name)
	lines, err := c.readLines(path)
	if err != nil {
		c.add(familyRouterFinding + shell.Oneline(path) + " could not be read, so its family exception is unverified" +
			" — every cross-family finding in it is suppressed on the strength of a file nothing opened")
		return
	}
	for _, line := range lines {
		if routerClaimPattern.MatchString(line) {
			return
		}
	}
	c.add(familyRouterFinding + shell.Oneline(path) + " names the workflow family but claims no exception" +
		" — cite " + routerClaimCitation + " in it, or stop naming that family")
}

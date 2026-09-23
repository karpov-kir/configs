package ecocheck

import (
	"io"
	"strings"

	"configs/ai/tools/shell"
)

const (
	skillDirWithoutSkillFile = "skill dir without SKILL.md: "
	skillNameDirMismatch     = "skill name/dir mismatch"
	skillWithoutDescription  = "skill without a description: "
	audienceNothingReads     = "audience nothing reads"
	codexPolicyRefused       = "Codex invocation policy refused: "
	codexPolicyMismatch      = "Codex invocation policy mismatch: "
	stageNothingCanInvoke    = "stage nothing can invoke: "
)

// Each defect here makes a skill unreachable rather than merely mis-linked: the loader finds a skill
// by its directory, invokes it by its frontmatter `name`, and routes to it by its `description`.
func (c *checker) scanSkillDirectories() {
	for _, entry := range c.walkTree(c.root.Skills()).entries {
		if !entry.mode.IsDir() || shell.DirName(entry.path) != c.root.Skills() {
			continue
		}
		if !c.holdsRegularFile(shell.Join(entry.path, "SKILL.md")) {
			c.add(skillDirWithoutSkillFile + shell.Oneline(entry.path))
		}
	}
	for _, file := range c.filesNamed(c.root.Skills(), "SKILL.md") {
		lines, err := c.readLines(file)
		// A file nothing read declares nothing, and both findings below would state what it declares.
		// Unread, they came out as `declares ''` and `without a description` — two positive claims
		// about frontmatter that is very likely fine, aimed at the one reader who would go and check.
		// readLines has already named the file; that finding is the true one.
		if err != nil {
			continue
		}
		if c.root.Agent() == "codex" {
			c.checkCodexInvocation(file, lines)
		}
		declared := shell.FrontmatterName(lines)
		if declared != shell.BaseName(shell.DirName(file)) {
			c.add(skillNameDirMismatch + ": " + shell.Oneline(file) + " declares '" + shell.Oneline(declared) + "'")
		}
		if shell.FrontmatterDescription(lines) == "" {
			c.add(skillWithoutDescription + shell.Oneline(file))
		}
		// Reported rather than read as an absent marker. `audience: maintainer` is the one value both
		// readers know; anything else installs the skill for everyone while the human who typed it
		// believes they marked it, and nothing on a correct-looking machine says otherwise. The
		// install refuses the same line for itself — an external machine has no copy of this check.
		if value, found := shell.UnknownAudience(lines); found {
			c.add(audienceNothingReads + ": " + shell.Oneline(file) + " declares '" + shell.Oneline(value) +
				"' — the only value is `audience: maintainer`, and this one leaves the skill installed for everyone")
		}
	}
}

// A skill another skill extends is a stage, and a stage its own orchestrator cannot invoke stalls.
//
// `disable-model-invocation` is for the skill ecosystem.md → **Conventions a new file joins** describes
// as one the human always initiates. A skill named on an `**Extends:**` line is the opposite: its
// caller is another skill, reaching it mid-pipeline with nobody at the keyboard. Claude Code refuses
// the marked skill to every model caller and adds "Do not replicate this skill's workflow by other
// means" — which bans the read-the-file route the extending skill instructs, so the stage's only
// remaining move is to stop and wait for a human to type the slash command. That was observed on
// 2026-09-16: a ship reached its landing stage twice and stopped both times.
//
// The edge is read off `**Extends:**` rather than off prose. ecosystem.md → **Three kinds, two homes**
// names three ways one skill can mention another and only extension runs it, so the declaration is the
// one signal that distinguishes a stage from a neighbour a description merely points at.
func (c *checker) scanStageInvocability() {
	extendedBy := map[string]string{}
	for _, file := range c.filesNamed(c.root.Skills(), "SKILL.md") {
		lines, err := c.readLines(file)
		if err != nil {
			continue
		}
		caller := shell.BaseName(shell.DirName(file))
		extends, _ := shell.ExtendsDeclarations(lines)
		for _, stage := range extends {
			// First caller wins, and the files arrive byte-sorted, so the name in the finding is stable
			// across runs. Naming one is enough: the finding is about the stage, and a second caller
			// changes nothing about what the reader has to do to it.
			if _, named := extendedBy[stage]; !named {
				extendedBy[stage] = caller
			}
		}
	}
	for _, name := range c.skillDirNames() {
		caller, isStage := extendedBy[name]
		if !isStage {
			continue
		}
		file := c.skillFilePath(name)
		lines, err := c.readLines(file)
		if err != nil || !shell.IsOptedOutOfModelInvocation(lines) {
			continue
		}
		c.add(stageNothingCanInvoke + shell.Oneline(file) + " — " + shell.Oneline(caller) +
			" extends it, so a skill invokes it with nobody at the keyboard, and disable-model-invocation" +
			" leaves it waiting for a human to type /" + shell.Oneline(name))
	}
}

// Every skill's description loads in every session too: the same tier, held to the same bar, and the
// only part of a skill no file in the router lists.
func (c *checker) reportDescriptionCensus(out io.Writer) {
	descriptionWords := 0
	routedSkills := 0
	skillTotal := 0
	for _, name := range c.skillDirNames() {
		file := c.skillFilePath(name)
		if !c.holdsRegularFile(file) {
			continue
		}
		skillTotal++
		lines, err := c.readLines(file)
		// Unread, this file looks exactly like a routed skill whose description says nothing: it
		// counted toward the figure's denominator and contributed zero words to the figure. readLines
		// has already named it at rank 1, so what is left to get right here is not counting it as a
		// description this run measured. The total above still counts it — the skill is in the tree,
		// which is what that number says; only the claim to have read its description goes.
		if err != nil || (c.root.Agent() == "claude" && shell.IsOptedOutOfModelInvocation(lines)) {
			continue
		}
		routedSkills++
		descriptionWords += len(shell.SplitFields(shell.FrontmatterDescription(lines)))
	}
	writeLinef(out, "always-loaded: %d words of skill description across %d of %d skills",
		descriptionWords, routedSkills, skillTotal)
}

func (c *checker) checkCodexInvocation(file string, lines []string) {
	sidecar := shell.Join(shell.DirName(file), "agents/openai.yaml")
	optedOut := false
	if c.holdsSomething(sidecar) {
		if !c.root.Contains(sidecar) {
			c.add(codexPolicyRefused + shell.Oneline(sidecar))
			return
		}
		policy, err := c.readLines(sidecar)
		if err != nil {
			return
		}
		inPolicy := false
		for _, line := range policy {
			if strings.TrimSpace(line) == "policy:" {
				inPolicy = true
				continue
			}
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && !strings.HasPrefix(line, "#") {
				inPolicy = false
			}
			if inPolicy && strings.TrimSpace(line) == "allow_implicit_invocation: false" {
				optedOut = true
			}
		}
	}
	if optedOut != shell.IsOptedOutOfModelInvocation(lines) {
		c.add(codexPolicyMismatch + shell.Oneline(file) + " — disable-model-invocation must match agents/openai.yaml policy.allow_implicit_invocation: false")
	}
}

package projectsetup

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"kk-flavor/tools/flavor"
	"kk-flavor/tools/installer"
	"kk-flavor/tools/shell"
)

// A project already ignoring the whole agent directory, and the line that does it. Reported rather
// than appended to: that rule covers this install's mounts and the project's own client settings
// alike, so what to do about it is a decision for the human whose repository it is.
//
// The forms git honours for a directory at the repository root: with or without a leading slash, with
// or without the trailing one.
func (run *invocation) broadAgentRule() string {
	body, err := os.ReadFile(run.ignoreFile)
	if err != nil {
		return ""
	}
	pattern := regexp.MustCompile(`^[ \t]*/?` + regexp.QuoteMeta(run.agentDirectory) + `/?[ \t]*$`)
	for number, line := range shell.SplitLines(string(body)) {
		if pattern.MatchString(line) {
			return fmt.Sprintf("%d", number+1)
		}
	}
	return ""
}

// Everything that has to be true of the project's own files before the first write. Asked together and
// ahead of the mounts, because a run that mounted twenty skills and then refused the instruction file
// has left a project half installed.
func (run *invocation) projectFilesWritable() bool {
	// A nonempty AGENTS.override.md shadows AGENTS.md, so a run that wrote the instructions anyway would
	// leave the project installed and nothing loading them.
	override := run.project + "/AGENTS.override.md"
	if run.agent == codexAgent && isNonEmptyFile(override) {
		run.mounting.Refuse(override + " shadows AGENTS.md — merge its instructions before installing")
		return false
	}
	for _, file := range []string{run.instructionsFile, run.claudeFile} {
		if !shell.PathExists(file) && !shell.IsSymlink(file) {
			continue
		}
		if !run.mounting.RegionWritable(file) {
			return false
		}
		// Half a fence means something edited inside the region or truncated the file, so the span a
		// write would rewrite is no longer the span that was written. Refused here rather than at the
		// write, because by then the mounts are in and the project is half installed.
		if run.mounting.HasBrokenRegion(file, flavor.RegionOpen, flavor.RegionClose) {
			run.mounting.Refuse(file + " holds an incomplete kk-flavor region — nothing was installed")
			return false
		}
	}
	return true
}

func isNonEmptyFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

// The two parents a skill mount lands under. A symlink at either would send every link this run writes
// somewhere the project never named.
func (run *invocation) projectSkillsWritable(mounting *installer.Run, project string) bool {
	for _, parent := range []string{project + "/" + run.agentDirectory, project + "/" + run.agentDirectory + "/skills"} {
		if shell.IsSymlink(parent) {
			mounting.Refuse(parent + " is a symlink — move it aside before restoring project skills")
			return false
		}
	}
	return true
}

func (run *invocation) writeIgnoreRules(broadRule string) {
	switch {
	case broadRule != "":
		run.mounting.Refuse(run.ignoreFile + " already ignores " + run.agentDirectory +
			"/ wholesale (line " + broadRule + ") — that covers this install and the project's own " +
			"settings alike, so no rule was added; decide it yourself")
	case shell.PathExists(run.ignoreFile):
		run.mounting.WriteRegion(run.ignoreFile, run.ignoreOpen, run.ignoreClose, run.ignoreBody())
	default:
		run.createIgnoreFile()
	}
}

// The branch for a project with no .gitignore at all. The region writer handles the file that exists;
// this is the one path that creates one, and it makes the check the region writer would otherwise have
// made.
func (run *invocation) createIgnoreFile() {
	if run.isDryRun {
		run.mounting.Say("  would create " + run.ignoreFile + " with the skill ignore rules")
		return
	}
	// A dangling symlink answers "not there" to the check at the call site, and a write follows it — so
	// without this the branch rewrites whatever the link names, anywhere this run's user can write. A
	// repository is not trusted input.
	if shell.IsSymlink(run.ignoreFile) {
		run.mounting.Refuse(run.ignoreFile +
			" is a symlink, and this writes the file itself — repoint or remove it, then re-run")
		return
	}
	body := run.ignoreOpen + "\n" + run.ignoreBody() + "\n" + run.ignoreClose + "\n"
	if err := os.WriteFile(run.ignoreFile, []byte(body), 0o644); err != nil {
		run.mounting.Refuse("could not create " + run.ignoreFile +
			" — the skill mounts are not ignored and will show up in this project's history")
		return
	}
	run.mounting.Say("  created  " + run.ignoreFile + " with the skill ignore rules")
}

// One instruction file's region, creating the file when the project has none.
func (run *invocation) writeInstructions(file, body string) bool {
	if !shell.PathExists(file) && !shell.IsSymlink(file) {
		if run.isDryRun {
			run.mounting.Say("  would create " + file + " and add the kk-flavor region")
			return true
		}
		// The region writer refuses a file that is not there, deliberately — a typo in a path would
		// otherwise produce a plausible-looking new file in someone's repository — so a project that has
		// never been configured is handed an empty one here first.
		if err := os.WriteFile(file, nil, 0o644); err != nil {
			run.mounting.Refuse("could not create " + file)
			return false
		}
	}
	return run.mounting.WriteRegion(file, flavor.RegionOpen, flavor.RegionClose, body)
}

// The hook body, spelled exactly as it is written into a repository. It reaches this installer through
// the bucket rather than through an absolute path into a checkout, so a hook survives the checkout
// moving — and it is compared byte for byte when deciding whether a hook is one this wrote, so the
// spelling is load-bearing rather than cosmetic.
func hookBody() string {
	return strings.Join([]string{
		"#!/usr/bin/env bash",
		"# kk-flavor project skills",
		`exec bash "$HOME/.kk-flavor/../project-skills.sh" --sync .`,
	}, "\n") + "\n"
}

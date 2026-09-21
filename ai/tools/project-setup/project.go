package projectsetup

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"configs/ai/tools/flavor"
	"configs/ai/tools/installer"
	"configs/ai/tools/shell"
)

// The number of the line where a project already ignores the whole agent directory, or "" for none.
// The rule is reported and left alone: it covers this install's mounts and the project's own client
// settings alike. The human whose repository it is decides what to do. The pattern takes the four
// forms git honours at the repository root, with the leading and trailing slashes each optional.
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
	// A nonempty AGENTS.override.md shadows AGENTS.md. A run that wrote the instructions anyway leaves
	// the project installed with no client loading them.
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
		// Half a fence means something edited inside the region or truncated the file. The span a write
		// rewrites is then no longer the span that was written. The refusal comes here, ahead of the
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
func (run *invocation) skillsMountWritable(mounting *installer.Run, project string) bool {
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

// The branch for a project with no .gitignore at all. The region writer handles a file that exists,
// and this is the single path that creates one. It makes the check the region writer would otherwise
// have made.
func (run *invocation) createIgnoreFile() {
	if run.isDryRun {
		run.mounting.Say("  would create " + run.ignoreFile + " with the skill ignore rules")
		return
	}
	// A dangling symlink answers "not there" to the check at the call site, and a write follows it.
	// Absent this, the branch rewrites whatever the link names, anywhere this run's user can write. A
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
		// The region writer refuses a file that is not there, deliberately: a typo in a path would
		// otherwise produce a plausible-looking new file in someone's repository. A project configured
		// for the first time is handed an empty one here.
		if err := os.WriteFile(file, nil, 0o644); err != nil {
			run.mounting.Refuse("could not create " + file)
			return false
		}
	}
	return run.mounting.WriteRegion(file, flavor.RegionOpen, flavor.RegionClose, body)
}

// The hook body, spelled exactly as it is written into a repository. The body reaches this installer
// through the bucket and holds no absolute path into a checkout, so a hook survives the checkout
// moving. It is also compared byte for byte when deciding whether a hook is one this wrote, so the
// spelling here is what an installed hook is held against.
func hookBody() string {
	return strings.Join([]string{
		"#!/usr/bin/env bash",
		"# kk-flavor project skills",
		// `~/.kk-flavor` is a link INTO a checkout, so `..` beside it means that checkout's ai/ directory,
		// and only `cd -P` resolves it that way. A logical cleanup against the name it was reached by
		// would instead mean the home, where no stub has ever been. shell.RealPath is the same rule on
		// the Go side, and the physical path is what lets the refusal name a checkout a human can act on.
		`flavor="$(CDPATH= cd -P -- "$HOME/.kk-flavor/.." 2>/dev/null && pwd -P)"`,
		`[ -n "$flavor" ] && [ -r "$flavor/project-skills.sh" ] || {`,
		// The refusal is the whole of what this adds over `exec`. The hook fires in somebody else's
		// repository, on every checkout, and the thing that can go missing sits outside that repository.
		// With the path handed straight to bash, an absent stub reads as `<hook>: No such file or
		// directory` under the project's own name. That sends the reader to the wrong repository.
		`  printf 'kk-flavor: %s holds no project-skills.sh, so project skills were not restored. That checkout is incomplete; this repository is not at fault.\n' "${flavor:-$HOME/.kk-flavor/..}" >&2`,
		// Non-zero because the hook did not do its job. Git ignores a post-checkout hook's status, so the
		// line is free, and a truthful status that goes unread still beats one that lies.
		`  exit 1`,
		`}`,
		`exec bash "$flavor/project-skills.sh" --sync .`,
	}, "\n") + "\n"
}

// Whether a hook already in a repository is one this installer wrote, and so may replace or remove.
// Every spelling this has ever written, newest first. supersededHookBody, the const, stays because a
// hook is compared byte for byte. Drop it, and an install refuses to touch the older hook while an
// uninstall leaves it firing on every checkout. It goes once no machine still carries that spelling.
func isOurHookBody(body string) bool {
	for _, known := range []string{hookBody(), supersededHookBody} {
		if body == known {
			return true
		}
	}
	return false
}

// The body written before the hook learned to name the checkout behind ~/.kk-flavor.
const supersededHookBody = "#!/usr/bin/env bash\n# kk-flavor project skills\n" +
	`exec bash "$HOME/.kk-flavor/../project-skills.sh" --sync .` + "\n"

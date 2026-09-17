package aibootstrap

import (
	"os"
	"strings"

	"configs/ai/tools/machine"
	"configs/ai/tools/shell"
)

// The rtk step: take out the file this repository used to write, then let rtk configure itself.
//
// Both halves are the owner tier's. rtk is personal tooling, so a colleague's machine is never told
// about a leftover this repository never wrote there, and never has rtk initialised on it.
func (run *invocation) configureRtk() {
	run.mounting.Say("rtk")
	run.removeLeftoverRtkDocument()
	run.initialiseRtk()
}

// ~/.claude/RTK.md is a copy an earlier bootstrap left behind, and what the next `rtk init -g` puts
// back. Claude's alone: the Codex profile keeps its own RTK.md and that one is rtk's to write.
func (run *invocation) removeLeftoverRtkDocument() {
	if run.agent == codexAgent {
		run.mounting.Say("  ok       Claude RTK cleanup does not apply to Codex")
		return
	}
	if !run.isOwner {
		run.mounting.Say("  skipped  rtk is the owner tier's")
		return
	}
	leftover := run.Home + "/.claude/RTK.md"
	switch {
	case !shell.PathExists(leftover) && !shell.IsSymlink(leftover):
		run.mounting.Say("  ok       no leftover " + leftover)
	// A directory is not a shape this ever wrote there, so it holds something else and removing it
	// would be the data loss this whole installer promises not to be.
	case !shell.IsSymlink(leftover) && shell.IsDir(leftover):
		run.mounting.Refuse(leftover + " is a directory, and this script only ever wrote a file there — " +
			"move it aside yourself")
	case run.isDryRun:
		run.mounting.Say("  would remove the leftover " + leftover)
	default:
		// Remove, never a delete that follows the link: a machine set up from an older README by hand
		// holds a symlink here, and what it points at is not this run's to take.
		if err := os.Remove(leftover); err != nil {
			run.mounting.Refuse("could not remove the leftover " + leftover)
			return
		}
		run.mounting.Say("  removed  " + leftover)
	}
}

// rtk writes into the client's own configuration, so it is handed the client's native arguments rather
// than one form for both: Claude takes a global hook with its settings patched, Codex takes its own
// `--codex --global` and writes an RTK.md into the profile.
func (run *invocation) rtkArguments() []string {
	if run.agent == codexAgent {
		return []string{"init", "--codex", "--global"}
	}
	return []string{"init", "--agent", claudeAgent, "--global", "--hook-only", "--auto-patch"}
}

func (run *invocation) initialiseRtk() {
	if !run.isOwner || run.isRtkSkipped {
		run.mounting.Say("  skipped  RTK initialization")
		return
	}
	// An instruction file this run refused to write is one rtk would be configured against for nothing:
	// the machine would describe tooling whose instructions never arrived.
	if !run.areInstructionsReady {
		run.mounting.Say("  skipped  RTK initialization: instructions were refused")
		return
	}
	// A Codex profile that already holds an RTK.md keeps it — it is the human's own document, and rtk
	// would write over it. Checked through the region writer's own guard, so a symlink or an unwritable
	// file is refused here rather than discovered by rtk halfway through.
	existing := run.CodexHome + "/RTK.md"
	needsInitialisation := true
	if run.agent == codexAgent && (shell.PathExists(existing) || shell.IsSymlink(existing)) {
		if !run.mounting.RegionWritable(existing) {
			return
		}
		needsInitialisation = false
	}
	if run.isDryRun {
		if needsInitialisation {
			run.mounting.Say("  would run rtk " + strings.Join(run.rtkArguments(), " "))
		}
		if run.agent == codexAgent {
			run.mounting.Say("  owner instructions already describe RTK usage")
		}
		return
	}
	if !needsInitialisation {
		run.mounting.Say("  kept     " + existing)
		return
	}
	if !run.Machine.HasCommand("rtk") {
		run.mounting.Refuse("rtk is not on PATH — install it or use --skip-rtk")
		return
	}
	if run.agent == codexAgent {
		run.initialiseCodexRtk()
		return
	}
	if run.Machine.Run(machine.Command{Name: "rtk", Args: run.rtkArguments(), Loud: true}) != 0 {
		run.mounting.Refuse("rtk init failed for " + run.agent)
	}
}

// Codex's rtk init writes a whole profile, so it is pointed at a staging directory and only the one
// file this install wants is copied out. Run against the real profile it would also rewrite AGENTS.md,
// which is the file the owner tier has just made its own copy of.
//
// The copy does not overwrite: by the time it runs, the profile either has no RTK.md — the case that
// got here — or gained one while rtk was running, and that one is not this run's to replace.
func (run *invocation) initialiseCodexRtk() {
	staging, err := os.MkdirTemp(run.Home, ".kk-flavor-rtk.")
	if err != nil {
		run.mounting.Refuse("could not create RTK staging directory")
		return
	}
	defer os.RemoveAll(staging)

	status := run.Machine.Run(machine.Command{
		Name: "rtk", Args: run.rtkArguments(), Env: []string{"CODEX_HOME=" + staging}, Loud: true,
	})
	if status != 0 || !shell.IsRegularFile(staging+"/RTK.md") {
		run.mounting.Refuse("rtk init failed for " + run.agent)
		return
	}
	if err = os.MkdirAll(run.CodexHome, 0o755); err != nil {
		run.mounting.Refuse("could not install Codex RTK instructions")
		return
	}
	if shell.PathExists(run.CodexHome+"/RTK.md") || shell.IsSymlink(run.CodexHome+"/RTK.md") {
		return
	}
	if err = copyFile(staging+"/RTK.md", run.CodexHome+"/RTK.md"); err != nil {
		run.mounting.Refuse("could not install Codex RTK instructions")
	}
}

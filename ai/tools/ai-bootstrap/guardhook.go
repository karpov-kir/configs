package aibootstrap

import (
	"encoding/json"
	"errors"
	"os"
	"slices"

	"configs/ai/tools/shell"
)

// guardMatcher names the Agent tool by both names Claude Code has given it.
const guardMatcher = "Agent|Task"

// guardCommand is the dispatch guard the hook runs, by its path under the mount.
func (run *invocation) guardCommand() string {
	return run.Home + "/.kk-flavor/scripts/agent-guard.sh"
}

func (run *invocation) claudeSettings() string {
	return run.Home + "/.claude/settings.json"
}

// registerAgentGuard adds the dispatch guard to Claude's settings as a PreToolUse hook on the Agent
// tool, and keeps every other setting and hook. The guard holds a dispatch to the narrowest agent type
// with its tools. Codex runs every worker with the same tools, so a guard there would hold it to
// nothing.
func (run *invocation) registerAgentGuard() {
	if run.agent != claudeAgent || !run.isOwner {
		return
	}
	run.mounting.Say("dispatch guard")
	settings, ok := run.readClaudeSettings()
	if !ok {
		return
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	groups, _ := hooks["PreToolUse"].([]any)
	if slices.ContainsFunc(groups, run.isGuardGroup) {
		run.mounting.Say("  ok       " + run.claudeSettings() + " runs the dispatch guard")
		return
	}
	if run.isDryRun {
		run.mounting.Say("  would add the dispatch guard hook to " + run.claudeSettings())
		return
	}
	hooks["PreToolUse"] = append(groups, map[string]any{
		"matcher": guardMatcher,
		"hooks":   []any{map[string]any{"type": "command", "command": run.guardCommand()}},
	})
	settings["hooks"] = hooks
	if run.writeClaudeSettings(settings) {
		run.mounting.Say("  added    the dispatch guard hook to " + run.claudeSettings())
	}
}

// removeAgentGuard takes the dispatch guard's hook out of Claude's settings and leaves the rest.
func (run *invocation) removeAgentGuard() {
	if run.agent != claudeAgent || !run.isOwner || !shell.PathExists(run.claudeSettings()) {
		return
	}
	run.mounting.Say("dispatch guard")
	settings, ok := run.readClaudeSettings()
	if !ok {
		return
	}
	hooks, _ := settings["hooks"].(map[string]any)
	groups, _ := hooks["PreToolUse"].([]any)
	kept := slices.DeleteFunc(slices.Clone(groups), run.isGuardGroup)
	if len(kept) == len(groups) {
		run.mounting.Say("  ok       " + run.claudeSettings() + " holds no dispatch guard")
		return
	}
	if run.isDryRun {
		run.mounting.Say("  would remove the dispatch guard hook from " + run.claudeSettings())
		return
	}
	if len(kept) == 0 {
		delete(hooks, "PreToolUse")
	} else {
		hooks["PreToolUse"] = kept
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	}
	if run.writeClaudeSettings(settings) {
		run.mounting.Say("  removed  the dispatch guard hook from " + run.claudeSettings())
	}
}

// isGuardGroup says a PreToolUse group runs the dispatch guard, whatever else it holds.
func (run *invocation) isGuardGroup(group any) bool {
	entries, _ := group.(map[string]any)["hooks"].([]any)
	return slices.ContainsFunc(entries, func(entry any) bool {
		hook, _ := entry.(map[string]any)
		return hook["command"] == run.guardCommand()
	})
}

// readClaudeSettings reads the settings, or an empty set where the file is not there. A symlink or a
// file that is not a JSON object is refused: writing through the one, or over the other, would change a
// file this run cannot read.
func (run *invocation) readClaudeSettings() (map[string]any, bool) {
	path := run.claudeSettings()
	if shell.IsSymlink(path) {
		run.mounting.Refuse(path + " is a symlink, so the dispatch guard was not registered — add it by hand")
		return nil, false
	}
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, true
	}
	settings := map[string]any{}
	if err != nil || json.Unmarshal(body, &settings) != nil {
		run.mounting.Refuse("could not read " + path + " as JSON, so the dispatch guard was not registered")
		return nil, false
	}
	return settings, true
}

// writeClaudeSettings writes the settings beside the file and renames them over it, keeping its mode.
// Claude reads the file at start, and a half-written one would hold no settings at all.
func (run *invocation) writeClaudeSettings(settings map[string]any) bool {
	path := run.claudeSettings()
	body, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		run.mounting.Refuse("could not write " + path)
		return false
	}
	mode := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.MkdirAll(shell.DirName(path), 0o755); err != nil {
		run.mounting.Refuse("could not create " + shell.DirName(path))
		return false
	}
	staged := path + ".kk-flavor-staged"
	if err := os.WriteFile(staged, append(body, '\n'), mode); err != nil || os.Rename(staged, path) != nil {
		_ = os.Remove(staged)
		run.mounting.Refuse("could not write " + path)
		return false
	}
	return true
}

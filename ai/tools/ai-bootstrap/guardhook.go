package aibootstrap

import (
	"bytes"
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
	return run.Home + "/.kk-flavor/scripts/agent-guard-hook.sh"
}

func (run *invocation) claudeSettings() string {
	return run.Home + "/.claude/settings.json"
}

// registerAgentGuard adds the dispatch guard to Claude's settings as a PreToolUse hook on the Agent
// tool, and keeps every other setting and hook. The guard holds a dispatch to the narrowest agent type
// with its tools. Codex runs every worker with the same tools, and it takes no guard.
func (run *invocation) registerAgentGuard() {
	if run.agent != claudeAgent || !run.isOwner {
		return
	}
	run.mounting.Say("dispatch guard")
	run.addGuardHook(run.claudeSettings(), guardMatcher)
}

// addGuardHook adds the guard as a PreToolUse group to the hooks file at path, beside what it holds.
func (run *invocation) addGuardHook(path, matcher string) bool {
	settings, ok := run.readHookSettings(path)
	if !ok {
		return false
	}
	hooks, groups, ok := run.preToolUse(path, settings)
	if !ok {
		return false
	}
	if slices.ContainsFunc(groups, run.isGuardGroup) {
		run.mounting.Say("  ok       " + path + " runs the dispatch guard")
		return true
	}
	if run.isDryRun {
		run.mounting.Say("  would add the dispatch guard hook to " + path)
		return true
	}
	hooks["PreToolUse"] = append(groups, map[string]any{
		"matcher": matcher,
		"hooks":   []any{map[string]any{"type": "command", "command": run.guardCommand()}},
	})
	settings["hooks"] = hooks
	if !run.writeHookSettings(path, settings) {
		return false
	}
	run.mounting.Say("  added    the dispatch guard hook to " + path)
	return true
}

// removeAgentGuard takes the dispatch guard's hook out of Claude's settings and leaves the rest.
func (run *invocation) removeAgentGuard() {
	if run.agent != claudeAgent || !run.isOwner {
		return
	}
	run.mounting.Say("dispatch guard")
	run.removeGuardHook(run.claudeSettings())
}

// removeGuardHook takes the guard's group out of the hooks file at path and leaves the rest.
func (run *invocation) removeGuardHook(path string) {
	if !shell.PathExists(path) {
		return
	}
	settings, ok := run.readHookSettings(path)
	if !ok {
		return
	}
	hooks, groups, ok := run.preToolUse(path, settings)
	if !ok {
		return
	}
	kept := slices.DeleteFunc(slices.Clone(groups), run.isGuardGroup)
	if len(kept) == len(groups) {
		run.mounting.Say("  ok       " + path + " holds no dispatch guard")
		return
	}
	if run.isDryRun {
		run.mounting.Say("  would remove the dispatch guard hook from " + path)
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
	if run.writeHookSettings(path, settings) {
		run.mounting.Say("  removed  the dispatch guard hook from " + path)
	}
}

// preToolUse is the settings' hooks and their PreToolUse groups, each made where absent. A value of
// another shape is the owner's, and the run refuses it and leaves it as it stands.
func (run *invocation) preToolUse(path string, settings map[string]any) (map[string]any, []any, bool) {
	hooks := map[string]any{}
	if value, found := settings["hooks"]; found {
		object, isObject := value.(map[string]any)
		if !isObject {
			run.mounting.Refuse(path + " holds hooks that are not an object, so the dispatch guard was left out")
			return nil, nil, false
		}
		hooks = object
	}
	var groups []any
	if value, found := hooks["PreToolUse"]; found {
		list, isList := value.([]any)
		if !isList {
			run.mounting.Refuse(path + " holds PreToolUse hooks that are not a list, so the dispatch guard was left out")
			return nil, nil, false
		}
		groups = list
	}
	return hooks, groups, true
}

// isGuardGroup says a PreToolUse group runs the dispatch guard, whatever else it holds.
func (run *invocation) isGuardGroup(group any) bool {
	object, _ := group.(map[string]any)
	entries, _ := object["hooks"].([]any)
	return slices.ContainsFunc(entries, func(entry any) bool {
		hook, _ := entry.(map[string]any)
		return hook["command"] == run.guardCommand()
	})
}

// readHookSettings reads a hooks file, or an empty set where the file is not there. A symlink or a
// file that is not a JSON object is refused: writing through the one, or over the other, would change a
// file this run cannot read.
func (run *invocation) readHookSettings(path string) (map[string]any, bool) {
	if shell.IsSymlink(path) {
		run.mounting.Refuse(path + " is a symlink, so the dispatch guard was not registered — add it by hand")
		return nil, false
	}
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, true
	}
	settings := map[string]any{}
	decoder := json.NewDecoder(bytes.NewReader(body))
	// A number stays as it was written, where a float would round a large one.
	decoder.UseNumber()
	if err != nil || decoder.Decode(&settings) != nil {
		run.mounting.Refuse("could not read " + path + " as JSON, so the dispatch guard was not registered")
		return nil, false
	}
	return settings, true
}

// writeHookSettings writes the settings beside the file and renames them over it, keeping its mode. A
// client reads the file at start, and a half-written one would hold no settings at all.
func (run *invocation) writeHookSettings(path string, settings map[string]any) bool {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	// A hook command holding `&&` or `>` stays readable.
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(settings); err != nil {
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
	if err := os.WriteFile(staged, encoded.Bytes(), mode); err != nil || os.Rename(staged, path) != nil {
		_ = os.Remove(staged)
		run.mounting.Refuse("could not write " + path)
		return false
	}
	return true
}

// sayCodexTakesNoGuard says on every Codex install why the dispatch guard is left out. Codex's
// spawn_agent takes task_name, message, fork_turns, model and reasoning_effort, and a bare spawn ran
// past the guard when a Codex session verified it (ai/tools/agent-guard/README.md).
func (run *invocation) sayCodexTakesNoGuard() {
	if run.agent != codexAgent || !run.isOwner {
		return
	}
	run.mounting.Say("dispatch guard")
	run.mounting.Say("  skipped  Codex exposes no role on spawn_agent, so there is no lighter worker to hold a spawn to")
}

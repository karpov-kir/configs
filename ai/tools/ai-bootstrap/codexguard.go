package aibootstrap

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"configs/ai/tools/shell"
)

// The region bootstrap appends to Codex's config.toml, and the marked line it adds to the owner's own
// [features] table. The open marker records how the file ended, so an uninstall gives back its bytes.
const (
	codexRegionOpen        = "# >>> kk-flavor: the light worker; bootstrap writes this region >>>"
	codexRegionOpenNewline = "# >>> kk-flavor: the light worker; bootstrap writes this region, and added the newline above >>>"
	codexRegionOpenCreated = "# >>> kk-flavor: the light worker; bootstrap writes this region, and created this file >>>"
	codexRegionClose       = "# <<< kk-flavor <<<"
	codexHooksLine         = "hooks = true # kk-flavor: the dispatch guard"
	codexRoleLayer         = "kk-flavor-light-worker.toml"
	// Codex runs a PreToolUse hook for spawn_agent under this tool name.
	codexGuardMatcher = "Agent"
)

var (
	reFeaturesHeader = regexp.MustCompile(`^\s*\[\s*features\s*\]\s*(#.*)?$`)
	reFeaturesOther  = regexp.MustCompile(`^\s*features\s*[.=]`)
	reAgentsOwned    = regexp.MustCompile(`light-worker|^\s*agents\s*=`)
	reTableHeader    = regexp.MustCompile(`^\s*\[`)
	reHooksKey       = regexp.MustCompile(`^\s*(hooks|"hooks"|'hooks')\s*=\s*([^\s#]+)`)
	rePluginTable    = regexp.MustCompile(`^\s*\[\s*plugins\.("[^"]+"|'[^']+')\s*\]\s*(#.*)?$`)
	reServerTable    = regexp.MustCompile(`^\s*\[\s*mcp_servers\.([A-Za-z0-9_-]+|"[^"]+"|'[^']+')\s*\]\s*(#.*)?$`)
	reServersTable   = regexp.MustCompile(`^\s*\[\s*mcp_servers\s*\]\s*(#.*)?$`)
	reInlineServer   = regexp.MustCompile(`^\s*([A-Za-z0-9_-]+|"[^"]+"|'[^']+')\s*=\s*\{`)
)

func (run *invocation) codexConfig() string { return run.CodexHome + "/config.toml" }
func (run *invocation) codexHooks() string  { return run.CodexHome + "/hooks.json" }

// codexRegion is the role declaration bootstrap appends to config.toml, with a [features] table where
// the owner's config has none.
func codexRegion(open string, withFeatures bool) []string {
	region := []string{open}
	if withFeatures {
		region = append(region, "[features]", codexHooksLine, "")
	}
	return append(region,
		"[agents.light-worker]",
		`description = "A worker with no plugin and no MCP server: the shell, file reads and edits. Spawn it for a task needing nothing else; a spawn naming no role needs a Needs: line."`,
		`config_file = "`+codexRoleLayer+`"`,
		codexRegionClose, "")
}

// registerCodexLightWorker gives Codex a light-worker role and the dispatch guard. A Codex worker
// inherits every plugin and MCP server and opened at a median 29.8k tokens over 18 spawns, where a
// Claude worker holding six tools opens near 5k. The role's layer switches off each plugin and server
// the config declares. The guard, in hooks.json, holds a spawn with no role to a Needs: line.
func (run *invocation) registerCodexLightWorker() {
	if run.agent != codexAgent || !run.isOwner {
		return
	}
	run.mounting.Say("light worker and dispatch guard")
	path := run.codexConfig()
	body, ok := run.readCodexConfig(path)
	if !ok {
		return
	}
	owners, refusal := withoutCodexRegion(body)
	if refusal != "" {
		run.mounting.Refuse(path + ": " + refusal + ", so the light worker was left as it stands")
		return
	}
	next, refusal := withCodexRole(owners, body != "" || shell.PathExists(path))
	if refusal != "" {
		run.mounting.Refuse(path + ": " + refusal + ", so the light worker was not declared — add it by hand")
		return
	}
	layer := codexLayer(owners)
	layerPath := run.CodexHome + "/" + codexRoleLayer
	held, _ := os.ReadFile(layerPath)
	switch {
	case next == body && string(held) == layer:
		run.mounting.Say("  ok       " + path + " declares the light worker")
	case run.isDryRun:
		run.mounting.Say("  would declare the light worker in " + path)
	default:
		if body != "" && next != body && !run.backUp(path, body) {
			return
		}
		if !writeStaged(layerPath, layer) || !writeStaged(path, next) {
			run.mounting.Refuse("could not write " + path + " or " + layerPath)
			return
		}
		run.mounting.Say("  declared the light worker in " + path)
	}
	run.addGuardHook(run.codexHooks(), codexGuardMatcher)
}

// removeCodexLightWorker takes out the region, the marked [features] line, the role's layer and the
// guard's hook, and leaves the owner's own lines as they stand.
func (run *invocation) removeCodexLightWorker() {
	if run.agent != codexAgent || !run.isOwner {
		return
	}
	run.mounting.Say("light worker and dispatch guard")
	run.removeGuardHook(run.codexHooks())
	path := run.codexConfig()
	body, ok := run.readCodexConfig(path)
	if !ok || body == "" {
		return
	}
	owners, refusal := withoutCodexRegion(body)
	switch {
	case refusal != "":
		run.mounting.Refuse(path + ": " + refusal + ", so the light worker was left in place — remove it by hand")
	case owners == body:
		run.mounting.Say("  ok       " + path + " declares no light worker")
	case run.isDryRun:
		run.mounting.Say("  would remove the light worker from " + path)
	case strings.Contains(body, codexRegionOpenCreated) && owners == "":
		_ = os.Remove(path)
		_ = os.Remove(run.CodexHome + "/" + codexRoleLayer)
		run.mounting.Say("  removed  " + path + ", which bootstrap created")
	case !writeStaged(path, owners):
		run.mounting.Refuse("could not write " + path)
	default:
		_ = os.Remove(run.CodexHome + "/" + codexRoleLayer)
		run.mounting.Say("  removed  the light worker from " + path)
	}
}

// readCodexConfig reads the config, or "" where it is not there. A symlink or a file with Windows line
// endings is refused, since the region's markers are matched by line.
func (run *invocation) readCodexConfig(path string) (string, bool) {
	if shell.IsSymlink(path) {
		run.mounting.Refuse(path + " is a symlink, so the light worker was left out — add it by hand")
		return "", false
	}
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", true
	}
	if err != nil || strings.Contains(string(body), "\r") {
		run.mounting.Refuse("could not read " + path + " as a file of plain lines, so the light worker was left out")
		return "", false
	}
	return string(body), true
}

// backUp copies the config beside itself before the first change, in its own mode.
func (run *invocation) backUp(path, body string) bool {
	mode := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	backup := fmt.Sprintf("%s.kk-flavor-backup-%d", path, time.Now().UnixNano())
	if err := os.WriteFile(backup, []byte(body), mode); err != nil {
		run.mounting.Refuse("could not back up " + path + ", so it was left as it was")
		return false
	}
	run.mounting.Say("  backup   " + backup)
	return true
}

// withoutCodexRegion is the config with bootstrap's region and marked line taken out, ending as the
// owner's file ended. A line inside the region that bootstrap did not write is the owner's, or one Codex
// appended there, and the run refuses and keeps it.
func withoutCodexRegion(text string) (string, string) {
	lines := strings.Split(text, "\n")
	open := -1
	for at, line := range lines {
		if line == codexRegionOpen || line == codexRegionOpenNewline || line == codexRegionOpenCreated {
			open = at
		}
	}
	var kept []string
	for _, line := range lines {
		if line != codexHooksLine {
			kept = append(kept, line)
		}
	}
	if open < 0 {
		return strings.Join(kept, "\n"), ""
	}
	region := codexRegion(lines[open], false)
	if open+1 < len(lines) && lines[open+1] == "[features]" {
		region = codexRegion(lines[open], true)
	}
	if strings.Join(lines[open:], "\n") != strings.Join(region, "\n") {
		return "", "the kk-flavor region holds a line bootstrap did not write, or one follows it"
	}
	owners := strings.Replace(strings.Join(lines[:open], "\n"), codexHooksLine+"\n", "", 1)
	switch lines[open] {
	case codexRegionOpenCreated:
		return "", ""
	case codexRegionOpenNewline:
		return owners, ""
	}
	return owners + "\n", ""
}

// withCodexRole adds the marked hooks line to the owner's [features] table and appends the region. The
// config it edits holds one [features] header and features in that table alone. It declares the light
// worker nowhere else and holds a multi-line string nowhere, since a header could hide in one.
func withCodexRole(owners string, existed bool) (string, string) {
	if strings.Contains(owners, `"""`) || strings.Contains(owners, `'''`) {
		return "", "it holds a multi-line string"
	}
	lines := strings.Split(owners, "\n")
	features := -1
	for at, line := range lines {
		switch {
		case reFeaturesOther.MatchString(line):
			return "", "it sets features outside a [features] table"
		case reAgentsOwned.MatchString(line):
			return "", "it already names a light worker or sets agents inline"
		case reFeaturesHeader.MatchString(line):
			if features >= 0 {
				return "", "it holds two [features] headers"
			}
			features = at
		}
	}
	add := codexHooksLine
	if features >= 0 {
		for at := features + 1; at < len(lines) && !reTableHeader.MatchString(lines[at]); at++ {
			if m := reHooksKey.FindStringSubmatch(lines[at]); m != nil {
				if m[2] != "true" {
					return "", "it sets features.hooks off"
				}
				add = ""
			}
		}
		if add != "" {
			lines = append(lines[:features+1], append([]string{add}, lines[features+1:]...)...)
		}
	}
	text := strings.Join(lines, "\n")
	open := codexRegionOpen
	switch {
	case !existed:
		open, text = codexRegionOpenCreated, ""
	case !strings.HasSuffix(text, "\n"):
		open, text = codexRegionOpenNewline, text+"\n"
	}
	return text + strings.Join(codexRegion(open, features < 0), "\n"), ""
}

// codexLayer is the light worker's config layer: every plugin and MCP server the owner's config
// declares, switched off, each under the key the owner wrote. It is generated, so a plugin the owner
// adds later is switched off on the next bootstrap.
func codexLayer(owners string) string {
	out := []string{"# Generated by bootstrap for the light-worker role: each plugin and MCP server config.toml declares, off.", ""}
	inServers := false
	for _, line := range strings.Split(owners, "\n") {
		switch m := rePluginTable.FindStringSubmatch(line); {
		case m != nil:
			out = append(out, "[plugins."+m[1]+"]", "enabled = false", "")
		case reServerTable.MatchString(line):
			out = append(out, "[mcp_servers."+reServerTable.FindStringSubmatch(line)[1]+"]", "enabled = false", "")
		}
		if reTableHeader.MatchString(line) {
			inServers = reServersTable.MatchString(line)
			continue
		}
		if m := reInlineServer.FindStringSubmatch(line); inServers && m != nil {
			out = append(out, "[mcp_servers."+m[1]+"]", "enabled = false", "")
		}
	}
	return strings.Join(out, "\n")
}

// writeStaged writes beside the file and renames over it, keeping the old file's mode.
func writeStaged(path, body string) bool {
	mode := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	staged := path + ".kk-flavor-staged"
	if err := os.WriteFile(staged, []byte(body), mode); err != nil || os.Rename(staged, path) != nil {
		_ = os.Remove(staged)
		return false
	}
	return true
}

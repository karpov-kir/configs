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

// The region bootstrap owns in Codex's config.toml, and the mark on the one line it adds to the owner's
// own [features] table. Both come out on uninstall, and every other line stays as the owner wrote it.
const (
	codexRegionOpen  = "# >>> kk-flavor: the light worker and the dispatch guard; bootstrap writes this region >>>"
	codexRegionClose = "# <<< kk-flavor <<<"
	codexHooksLine   = "hooks = true # kk-flavor: the dispatch guard"
	codexRoleLayer   = "kk-flavor-light-worker.toml"
)

var (
	rePluginTable = regexp.MustCompile(`^\s*\[plugins\."([^"]+)"\]\s*$`)
	reServerTable = regexp.MustCompile(`^\s*\[mcp_servers\.([A-Za-z0-9_-]+)\]\s*$`)
	reTableHeader = regexp.MustCompile(`^\s*\[`)
	reHooksKey    = regexp.MustCompile(`^\s*hooks\s*=\s*(\S+)`)
)

func (run *invocation) codexConfig() string { return run.CodexHome + "/config.toml" }

// registerCodexLightWorker gives Codex a light-worker role and the dispatch guard. A Codex worker
// inherits every plugin and MCP server and opened at a median 29.8k tokens over 18 spawns, where a
// Claude worker holding six tools opens near 5k. The role's layer switches off each plugin and server
// the config declares. The guard holds a spawn with no role to a Needs: line.
func (run *invocation) registerCodexLightWorker() {
	if run.agent != codexAgent || !run.isOwner {
		return
	}
	run.mounting.Say("light worker and dispatch guard")
	path := run.codexConfig()
	if shell.IsSymlink(path) {
		run.mounting.Refuse(path + " is a symlink, so the light worker was not registered — add it by hand")
		return
	}
	body, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		run.mounting.Refuse("could not read " + path)
		return
	}
	owners := withoutCodexRegion(string(body))
	next, refusal := run.withCodexGuard(owners)
	if refusal != "" {
		run.mounting.Refuse(refusal)
		return
	}
	layer := codexLayer(owners)
	layerPath := run.CodexHome + "/" + codexRoleLayer
	held, _ := os.ReadFile(layerPath)
	if next == string(body) && string(held) == layer {
		run.mounting.Say("  ok       " + path + " declares the light worker and the dispatch guard")
		return
	}
	if run.isDryRun {
		run.mounting.Say("  would declare the light worker and the dispatch guard in " + path)
		return
	}
	if len(body) > 0 && next != string(body) {
		backup := fmt.Sprintf("%s.kk-flavor-backup-%d", path, time.Now().Unix())
		if err := os.WriteFile(backup, body, 0o600); err != nil {
			run.mounting.Refuse("could not back up " + path + ", so it was left as it was")
			return
		}
		run.mounting.Say("  backup   " + backup)
	}
	if !writeStaged(layerPath, layer) || !writeStaged(path, next) {
		run.mounting.Refuse("could not write " + path + " or " + layerPath)
		return
	}
	run.mounting.Say("  declared the light worker and the dispatch guard in " + path)
}

// removeCodexLightWorker takes out the region, the marked [features] line and the role's layer, and
// leaves the owner's own lines as they stand.
func (run *invocation) removeCodexLightWorker() {
	if run.agent != codexAgent || !run.isOwner {
		return
	}
	path := run.codexConfig()
	body, err := os.ReadFile(path)
	if err != nil || shell.IsSymlink(path) {
		return
	}
	run.mounting.Say("light worker and dispatch guard")
	next := withoutCodexRegion(string(body))
	if next == string(body) {
		run.mounting.Say("  ok       " + path + " holds no light worker")
		return
	}
	if run.isDryRun {
		run.mounting.Say("  would remove the light worker and the dispatch guard from " + path)
		return
	}
	if !writeStaged(path, next) {
		run.mounting.Refuse("could not write " + path)
		return
	}
	_ = os.Remove(run.CodexHome + "/" + codexRoleLayer)
	run.mounting.Say("  removed  the light worker and the dispatch guard from " + path)
}

// withoutCodexRegion is the config with bootstrap's region and its marked [features] line taken out.
func withoutCodexRegion(text string) string {
	var kept []string
	inside := false
	for _, line := range strings.Split(text, "\n") {
		switch {
		case line == codexRegionOpen:
			inside = true
		case line == codexRegionClose:
			inside = false
		case !inside && line != codexHooksLine:
			kept = append(kept, line)
		}
	}
	return strings.TrimRight(strings.Join(kept, "\n"), "\n") + "\n"
}

// withCodexGuard adds the [features] switch and the region to the owner's config. A hooks switch the
// owner set off is the owner's, and the run refuses and leaves hooks off.
func (run *invocation) withCodexGuard(owners string) (string, string) {
	lines := strings.Split(strings.TrimRight(owners, "\n"), "\n")
	features := -1
	for at, line := range lines {
		if strings.TrimSpace(line) == "[features]" {
			features = at
		}
	}
	region := []string{"", codexRegionOpen}
	if features >= 0 {
		for at := features + 1; at < len(lines) && !reTableHeader.MatchString(lines[at]); at++ {
			if m := reHooksKey.FindStringSubmatch(lines[at]); m != nil {
				if m[1] != "true" {
					return "", run.codexConfig() + " sets features.hooks off, so the dispatch guard was left out"
				}
				features = -2
				break
			}
		}
		if features >= 0 {
			lines = append(lines[:features+1], append([]string{codexHooksLine}, lines[features+1:]...)...)
		}
	} else {
		region = append(region, "[features]", "hooks = true", "")
	}
	region = append(region,
		"[agents.light-worker]",
		`description = "A worker with no plugin and no MCP server: the shell, file reads and edits. Spawn it for a task needing nothing else; a spawn naming no role needs a Needs: line."`,
		`config_file = "`+codexRoleLayer+`"`,
		"",
		"[[hooks.PreToolUse]]",
		`matcher = "spawn_agent"`,
		"",
		"[[hooks.PreToolUse.hooks]]",
		`type = "command"`,
		`command = "`+run.guardCommand()+`"`,
		codexRegionClose)
	return strings.Join(append(lines, region...), "\n") + "\n", ""
}

// codexLayer is the light worker's config layer: every plugin and MCP server the owner's config
// declares, switched off. It is generated, so a plugin the owner adds later is switched off on the next
// bootstrap.
func codexLayer(owners string) string {
	out := []string{"# Generated by bootstrap for the light-worker role: each plugin and MCP server config.toml declares, off.", ""}
	for _, line := range strings.Split(owners, "\n") {
		if m := rePluginTable.FindStringSubmatch(line); m != nil {
			out = append(out, fmt.Sprintf("[plugins.%q]", m[1]), "enabled = false", "")
		}
		if m := reServerTable.FindStringSubmatch(line); m != nil {
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

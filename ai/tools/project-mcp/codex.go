package projectmcp

import (
	"fmt"
	"regexp"
	"strings"

	"configs/ai/tools/mcp"
)

// The fences around the region this tool owns in a project's `.codex/config.toml`. Everything between
// them is written by this tool and compared byte for byte on the next run; everything outside is the
// project's, and is never rewritten.
const (
	regionOpen  = "# kk-flavor-mcp:begin"
	regionClose = "# kk-flavor-mcp:end"
)

// An ordinary server table and nothing else: `[mcp_servers.<name>]`, optionally with sub-tables, and
// at most a trailing comment.
var serverTable = regexp.MustCompile(`^\s*\[mcp_servers\.([a-zA-Z0-9_-]+)(?:\.[a-zA-Z0-9_-]+)*\]\s*(?:#.*)?$`)

// A TOML escape. A file using them can spell `mcp_servers` in a way the line scan below would not
// recognise, so such a file is left for an explicit edit.
var tomlEscape = regexp.MustCompile(`\\[uU]`)

// Codex's project file is TOML, which this tool does not parse. It owns one fenced region and reads
// the rest only well enough to know it is not being asked to redefine something.
//
// Recognizing ordinary server tables only is the whole approach. Other spellings could redefine the
// parent table or a managed server, so a file using them is left for an explicit edit rather than
// guessed at — guessing wrong here silently detaches a server the human still has in their config.
func mergeCodex(text string, servers []projectServer, isUninstall bool) (string, error) {
	region, err := codexRegion(servers)
	if err != nil {
		return "", err
	}
	if strings.Contains(text, "'''") || strings.Contains(text, `"""`) {
		return "", fmt.Errorf("multiline TOML strings require an explicit MCP merge")
	}

	outside := text
	start, end := strings.Index(text, regionOpen), strings.Index(text, regionClose)
	if start >= 0 || end >= 0 {
		if err = checkRegionFences(text, start, end); err != nil {
			return "", err
		}
		if owned := text[start:end+len(regionClose)] + "\n"; owned != region {
			return "", fmt.Errorf("project MCP region was edited; preserve or remove it explicitly")
		}
		// A setting under the closing fence belongs to the last table above it, which is a managed
		// server — so removing the region would silently re-home it on whatever table comes next.
		suffix := text[end+len(regionClose):]
		if following := firstSetting(suffix); following != "" && !strings.HasPrefix(strings.TrimLeft(following, " \t"), "[") {
			return "", fmt.Errorf("settings after the MCP region belong to a managed server; " +
				"move them before removing it")
		}
		outside = text[:start] + strings.TrimPrefix(suffix, "\n")
	}
	if isUninstall {
		return outside, nil
	}
	if err = refuseConflictingTables(outside, servers); err != nil {
		return "", err
	}
	if start >= 0 {
		return text, nil
	}
	if outside != "" && !strings.HasSuffix(outside, "\n") {
		outside += "\n"
	}
	return outside + region, nil
}

// The region as this tool writes it, which is also what an untouched one has to equal.
func codexRegion(servers []projectServer) (string, error) {
	blocks := make([]string, 0, len(servers))
	for _, server := range servers {
		args, err := mcp.EncodeJSON(server.Args)
		if err != nil {
			return "", err
		}
		// A JSON array of strings is a TOML array of basic strings, so the arguments go across with one
		// encoder and no TOML writer of our own.
		blocks = append(blocks, fmt.Sprintf("[mcp_servers.%s]\ncommand = \"sh\"\nargs = %s\n", server.Name, args))
	}
	return regionOpen + "\n" + strings.Join(blocks, "\n") + regionClose + "\n", nil
}

// Both fences, each on a whole line of its own, in that order, and once each. Half a region, a
// repeated one, or one sharing a line with something else is a file this tool cannot tell its own
// writing from — so it says so rather than picking an interpretation.
func checkRegionFences(text string, start, end int) error {
	if start < 0 || end < start ||
		strings.Index(text[start+len(regionOpen):], regionOpen) >= 0 ||
		strings.Index(text[end+len(regionClose):], regionClose) >= 0 {
		return fmt.Errorf("incomplete or repeated project MCP markers")
	}
	after := text[end+len(regionClose):]
	if (start > 0 && text[start-1] != '\n') || (end > 0 && text[end-1] != '\n') ||
		(after != "" && after[0] != '\n') {
		return fmt.Errorf("project MCP markers must occupy whole lines")
	}
	return nil
}

// The first line that sets or opens something, comments and blank lines skipped.
func firstSetting(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			continue
		}
		return line
	}
	return ""
}

// A server this tool manages, defined outside its own region, is a definition it would shadow or be
// shadowed by. Which of the two depends on TOML's own rules, so neither is guessed at.
func refuseConflictingTables(outside string, servers []projectServer) error {
	managed := map[string]bool{}
	for _, server := range servers {
		managed[server.Name] = true
	}
	for _, line := range strings.Split(outside, "\n") {
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			continue
		}
		if tomlEscape.MatchString(line) {
			return fmt.Errorf("escaped TOML requires an explicit MCP merge")
		}
		if !strings.Contains(line, "mcp_servers") {
			continue
		}
		match := serverTable.FindStringSubmatch(line)
		if match == nil || managed[match[1]] {
			return fmt.Errorf("existing MCP TOML conflicts or uses ambiguous syntax; merge it explicitly")
		}
	}
	return nil
}

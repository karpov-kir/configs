// The MCP server declarations both installers read: `ai/mcp.jsonc`, and the gitignored
// `ai/mcp.private.jsonc` beside it.
package mcp

import "strings"

// The token a declaration writes where the checkout's `ai/` directory belongs.
const ConfigsToken = "@CONFIGS@"

// StripComments blanks every line whose first non-blank characters are `//`, leaving the line itself
// in place.
//
// Anchored at the line start: blanking from any `//` onwards truncates a URL. It blanks rather than
// deletes, so a parser's line numbers still point at the line the human is looking at.
func StripComments(text string) string {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		if strings.HasPrefix(strings.TrimLeft(line, " \t\v\f\r"), "//") {
			lines[index] = ""
		}
	}
	return strings.Join(lines, "\n")
}

// SubstituteConfigsDir replaces every occurrence of the token with dir — every one, because a
// declaration carries one per stdio server.
func SubstituteConfigsDir(text, dir string) string {
	return strings.ReplaceAll(text, ConfigsToken, dir)
}

// ConfigsDirIsSubstitutable reports whether dir can be substituted into a JSON string at all.
func ConfigsDirIsSubstitutable(dir string) bool {
	return !strings.ContainsAny(dir, `"\`)
}

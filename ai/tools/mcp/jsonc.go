// The MCP server declarations both installers read: `ai/mcp.jsonc`, and the gitignored
// `ai/mcp.private.jsonc` beside it. This package owns the three things that happen to that text
// before anything parses it — the comment stripping, the `@CONFIGS@` substitution, and the refusal of
// a checkout directory that cannot be substituted into valid JSON — plus the ordered object the two
// installers hand on to a client.
//
// `ai/mcp-sync.sh` registers those servers in a client's user scope; `ai/project-mcp.sh` maps the
// public ones into one project's own client files. Both read the same file, so both read it here.
package mcp

import "strings"

// The token a declaration writes where the checkout's `ai/` directory belongs. What the client CLI is
// handed is a literal string and it expands nothing, so a `@CONFIGS@` that survives registration is a
// server whose command does not exist.
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
//
// A plain replacement and not a regular expression: the replacement is a filesystem path, and every
// metacharacter a pattern language reserves — `&`, `$`, `\`, and whichever character is chosen as a
// delimiter — is one a path may hold. None of them is special here.
func SubstituteConfigsDir(text, dir string) string {
	return strings.ReplaceAll(text, ConfigsToken, dir)
}

// ConfigsDirIsSubstitutable reports whether dir can be substituted into a JSON string at all.
//
// A `"` and a `\` are refused rather than escaped, because the mangling is silent. A `\` lands inside
// the JSON string as an escape, so the entry still parses and the command names a different path:
// `/opt/a\b` reaches the CLI as a backspace. A crafted `"` closes the string early, so the command is
// no longer mcp-env.sh and the rest of the directory becomes further keys, an `env` one being enough.
// Either way what goes missing is the environment stripping, so the caller names the directory and
// stops.
func ConfigsDirIsSubstitutable(dir string) bool {
	return !strings.ContainsAny(dir, `"\`)
}

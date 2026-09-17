package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestALineStartingWithACommentMarkerIsBlanked(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		text string
		want string
	}{
		{name: "at the line start", text: "// a comment\n{\"a\": 1}\n", want: "\n{\"a\": 1}\n"},
		{name: "behind leading whitespace, which does not save it", text: "  \t// indented\n{\"a\": 1}\n",
			want: "\n{\"a\": 1}\n"},
		{name: "after JSON on the same line, where it is left alone", text: "{\"a\": 1} // trailing\n",
			want: "{\"a\": 1} // trailing\n"},
		{name: "inside a value, where blanking it would truncate the URL",
			text: "{\n  \"url\": \"https://example.com/mcp\"\n}\n", want: "{\n  \"url\": \"https://example.com/mcp\"\n}\n"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			if got := StripComments(scenario.text); got != scenario.want {
				t.Errorf("a comment %s was not handled as the declaration's grammar says\n  got: %q\n want: %q",
					scenario.name, got, scenario.want)
			}
		})
	}
}

// The control that makes the row above a measurement: the stripping leaves that file unparseable on
// purpose, so the grammar is "a comment owns its whole line" rather than "a comment is anything after
// two slashes".
func TestATrailingCommentIsLeftForTheParserToRefuse(t *testing.T) {
	t.Parallel()
	stripped := StripComments("{\"a\": 1} // trailing\n")
	var value any
	if err := json.Unmarshal([]byte(stripped), &value); err == nil {
		t.Errorf("a trailing comment parsed as JSON, so the stripping guessed at it instead of leaving it "+
			"to be refused — a declaration would then mean whatever the guess made of it\n  %q", stripped)
	}
}

// Blanked and not deleted, so a parse error's line number still points at the line the human is
// looking at.
func TestStrippingKeepsEveryLineWhereItWas(t *testing.T) {
	t.Parallel()
	text := "// one\n{\n  // two\n  \"a\": 1\n}\n"
	before, after := strings.Count(text, "\n"), strings.Count(StripComments(text), "\n")
	if before != after {
		t.Errorf("stripping moved the lines: %d before, %d after. Every parse error a human reads after "+
			"this points at the wrong line of their file.", before, after)
	}
}

func TestTheTokenBecomesTheDirectory(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		text string
		dir  string
		want string
	}{
		{name: "in a command", text: `{"command": "@CONFIGS@/mcp-env.sh"}`, dir: "/opt/kk",
			want: `{"command": "/opt/kk/mcp-env.sh"}`},
		// The declaration carries one per stdio server, so the first is never the only one.
		{name: "at every occurrence, not just the first", text: "@CONFIGS@/x @CONFIGS@/y", dir: "/a",
			want: "/a/x /a/y"},
		{name: "nowhere, leaving text without the token untouched", text: `{"url": "https://example.com/mcp"}`,
			dir: "/opt/kk", want: `{"url": "https://example.com/mcp"}`},
		// Why this is a plain replacement and not a pattern: every delimiter a pattern language could
		// take is a character a path may hold, and `&` is a replacement's own back-reference in sed.
		// None of them is special here.
		{name: "in a path holding a pattern language's own metacharacters", text: "@CONFIGS@/mcp-env.sh",
			dir: `/tmp/a&b$c*d e`, want: `/tmp/a&b$c*d e/mcp-env.sh`},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			if got := SubstituteConfigsDir(scenario.text, scenario.dir); got != scenario.want {
				t.Errorf("the checkout directory did not land literally\n  got: %q\n want: %q\n"+
					"what the client is handed is a literal string it expands nothing in, so a command "+
					"spelled differently here is a server that does not start", got, scenario.want)
			}
		})
	}
}

func TestADirectoryIsSubstitutableUnlessItCanCloseOrEscapeAJSONString(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		dir  string
		want bool
	}{
		{dir: "/Users/kk/configs/ai", want: true},
		{dir: "/tmp/with space/ai", want: true},
		{dir: `/tmp/a&b$c*d/ai`, want: true},
		{dir: "/tmp/ünïcodé/ai", want: true},
		{dir: `/tmp/a"b/ai`, want: false},
		{dir: `/tmp/a\b/ai`, want: false},
	} {
		t.Run(scenario.dir, func(t *testing.T) {
			t.Parallel()
			if got := ConfigsDirIsSubstitutable(scenario.dir); got != scenario.want {
				t.Errorf("%q: substitutable = %v, want %v. A directory wrongly accepted here reaches the "+
					"client as a different command; one wrongly refused stops a sync that would have been fine.",
					scenario.dir, got, scenario.want)
			}
		})
	}
}

// Why those two are refused rather than escaped: both manglings are silent, and both parse.
//
// A backslash lands inside the JSON string as an escape, so the entry still parses and names a
// DIFFERENT path — `/opt/a\b` reaches the CLI as a backspace.
func TestABackslashInTheDirectoryYieldsJSONThatParsesAndNamesAnotherPath(t *testing.T) {
	t.Parallel()
	substituted := SubstituteConfigsDir(`{"command": "@CONFIGS@/mcp-env.sh"}`, `/tmp/a\b`)
	var entry struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(substituted), &entry); err != nil {
		t.Fatalf("the backslash case no longer parses, so it no longer shows what makes this silent: %v", err)
	}
	if entry.Command == `/tmp/a\b/mcp-env.sh` {
		t.Errorf("the backslash survived as itself, so this case no longer demonstrates the mangling the "+
			"refusal exists for\n  %q", entry.Command)
	}
}

// A crafted quote closes the string early, and the rest of the directory becomes further keys — an
// `env` one being enough, since the environment stripping is exactly what the wrapper this command
// names exists to do.
func TestACraftedQuoteInTheDirectoryGrowsTheEnvKeyTheWrapperExistsToRemove(t *testing.T) {
	t.Parallel()
	injected := SubstituteConfigsDir(`{"command": "@CONFIGS@/mcp-env.sh"}`,
		`/tmp/x", "env": {"LEAK": "1"}, "ignored": "`)
	object, err := ParseObject([]byte(injected))
	if err != nil {
		t.Fatalf("the injection case no longer parses, so it no longer shows what makes this silent: %v", err)
	}
	if got := strings.Join(object.Keys(), ","); got != "command,env,ignored" {
		t.Errorf("the injected entry's keys are %q, want \"command,env,ignored\". This case is what says "+
			"the refusal is about a credential reaching an unpinned package, not about tidy paths.", got)
	}
}

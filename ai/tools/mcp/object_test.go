package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAnObjectKeepsItsKeysInDocumentOrder(t *testing.T) {
	t.Parallel()
	source := `{"zebra":1,"alpha":{"b":2,"a":1},"middle":["x"]}`
	object, err := ParseObject([]byte(source))
	if err != nil {
		t.Fatalf("parsing %s: %v", source, err)
	}
	if got := strings.Join(object.Keys(), ","); got != "zebra,alpha,middle" {
		t.Errorf("keys are %q, want \"zebra,alpha,middle\" — Go's own map would answer them sorted", got)
	}
}

// Set must leave a replaced value's key where it is, which is how a duplicate key keeps its first
// position.
func TestSettingAnExistingKeyLeavesItWhereItWas(t *testing.T) {
	t.Parallel()
	object, err := ParseObject([]byte(`{"one":1,"two":2,"three":3}`))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	object.Set("two", json.RawMessage(`22`))
	object.Set("four", json.RawMessage(`4`))
	if got := strings.Join(object.Keys(), ","); got != "one,two,three,four" {
		t.Errorf("keys are %q, want \"one,two,three,four\" — a replaced value moved its key, and a new one "+
			"must land at the end", got)
	}
}

// A value is kept as its own bytes — non-ASCII and the characters Go's default encoder escapes
// included — so a field `mcp-sync` reads arrives exactly as the declaration wrote it.
func TestAValueComesBackByteForByte(t *testing.T) {
	t.Parallel()
	const kept = `{"text":"a && b <c> ünïcodé","n":1.5e10}`
	object, err := ParseObject([]byte(`{"kept":` + kept + `}`))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	if raw, _ := object.Get("kept"); string(raw) != kept {
		t.Errorf("a value was rewritten\n  got: %s\n want: %s", raw, kept)
	}
}

func TestSomethingThatIsNotAnObjectIsRefused(t *testing.T) {
	t.Parallel()
	// A document holding a second object is TestAnythingAfterTheObjectIsRefusedWhetherOrNotItIsJSON's
	// whole subject, so it is not repeated here.
	for _, source := range []string{`[1,2]`, `"text"`, ``, `{`} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			if _, err := ParseObject([]byte(source)); err == nil {
				t.Errorf("%q was read as an object, so a malformed config would be merged into rather than "+
					"refused", source)
			}
		})
	}
}

// An earlier check decoded a second value, which fires only when what follows parses. `THIS IS NOT
// JSONC` appended to `ai/mcp.jsonc` was accepted, the file read as the leading object, and every
// suite over it stayed green.

// Anything after the closing brace makes the document more than one object, valid JSON included.
func TestAnythingAfterTheObjectIsRefusedWhetherOrNotItIsJSON(t *testing.T) {
	for _, trailing := range []string{
		`{"one":1} {"two":2}`,
		`{"one":1} THIS IS NOT JSONC`,
		`{"one":1} 7`,
		`{"one":1} ]`,
	} {
		if _, err := ParseObject([]byte(trailing)); err == nil {
			t.Errorf("ParseObject(%q) accepted a document holding more than one object", trailing)
		}
	}
	// The control. Whitespace and a trailing newline are what a well-formed file ends with, and a
	// refusal there would refuse every file this tool reads.
	for _, clean := range []string{`{"one":1}`, "{\"one\":1}\n", "  {\"one\":1}  \n\n"} {
		if _, err := ParseObject([]byte(clean)); err != nil {
			t.Errorf("ParseObject(%q) refused a well-formed object: %v", clean, err)
		}
	}
}

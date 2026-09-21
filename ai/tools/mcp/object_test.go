package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAnObjectRoundTripsInDocumentOrder(t *testing.T) {
	t.Parallel()
	source := `{"zebra":1,"alpha":{"b":2,"a":1},"middle":["x"]}`
	object, err := ParseObject([]byte(source))
	if err != nil {
		t.Fatalf("parsing %s: %v", source, err)
	}
	raw, err := object.MarshalJSON()
	if err != nil {
		t.Fatalf("writing it back: %v", err)
	}
	if string(raw) != source {
		t.Errorf("a round trip reordered or reshaped the document\n  got: %s\n want: %s\n"+
			"Go's own map would answer these keys sorted, which is a diff across every line of a file "+
			"this tool was asked to add one entry to", raw, source)
	}
}

// Set must leave a replaced value's key where it is. A key that moves sends the server to the end of
// a project's config, where it reads as a removal plus an addition.
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

func TestDeletingAKeyLeavesTheRestInOrder(t *testing.T) {
	t.Parallel()
	object, err := ParseObject([]byte(`{"one":1,"two":2,"three":3}`))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	object.Delete("two")
	object.Delete("absent")
	raw, err := object.MarshalJSON()
	if err != nil {
		t.Fatalf("writing it back: %v", err)
	}
	if string(raw) != `{"one":1,"three":3}` {
		t.Errorf("after deleting one key the object is %s, want {\"one\":1,\"three\":3}", raw)
	}
}

// A project's config holds other tools' entries, so what this does not touch has to come back byte
// for byte — non-ASCII and the characters Go's default encoder escapes included.
func TestValuesThisToolDoesNotTouchComeBackByteForByte(t *testing.T) {
	t.Parallel()
	source := `{"kept":{"text":"a && b <c> ünïcodé","n":1.5e10}}`
	object, err := ParseObject([]byte(source))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	raw, err := object.MarshalJSON()
	if err != nil {
		t.Fatalf("writing it back: %v", err)
	}
	if string(raw) != source {
		t.Errorf("an untouched value was rewritten\n  got: %s\n want: %s", raw, source)
	}
}

func TestEncodingLeavesTheLaunchersOwnCharactersAlone(t *testing.T) {
	t.Parallel()
	encoded, err := EncodeJSON([]string{`a && b`, `<x>`})
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	// The assertion looks for any backslash, because each escape has more than one spelling. Every
	// character in this fixture is one Go's default encoder rewrites, and none of them needs escaping
	// in JSON.
	if strings.ContainsRune(string(encoded), '\\') {
		t.Errorf("the launcher's own characters were written as escapes: %s\nThe file is compared byte for "+
			"byte against what a reinstall would write, so an escaping nobody asked for reads as a file "+
			"somebody edited, and the region is then refused.", encoded)
	}
	if string(encoded) != `["a && b","<x>"]` {
		t.Errorf("encoding changed the values themselves\n  got: %s\n want: %s", encoded, `["a && b","<x>"]`)
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

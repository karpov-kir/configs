package shell_test

// The first tests this package has had. Oneline is the guard every attacker-chosen name in both tools
// passes through on its way to a message, so the cases below are written against what it promises a
// reader — one physical line, and nothing that drives the terminal — rather than against how it walks
// the bytes.

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"kk-flavor/tools/shell"
)

func TestOnelineReplacesEveryC0ByteAndDel(t *testing.T) {
	for b := 0; b < 0x20; b++ {
		in := "a" + string(rune(b)) + "b"
		if got := shell.Oneline(in); got != "a b" {
			t.Errorf("Oneline(%q) = %q, want %q", in, got, "a b")
		}
	}
	if got := shell.Oneline("a\u007fb"); got != "a b" {
		t.Errorf("Oneline of DEL = %q, want %q", got, "a b")
	}
}

// UTF-8 puts the C1 controls at 0xc2 0x80-0x9f, two bytes both above 0x7f, so a scan testing one byte
// at a time never sees them. U+009B is CSI, `ESC [` as one character: the escape the C0 case bars,
// arriving by a second door. The third door is the raw byte, below.
func TestOnelineReplacesTheEncodedC1Range(t *testing.T) {
	for r := rune(0x80); r <= 0x9f; r++ {
		in := "a" + string(r) + "b"
		got := shell.Oneline(in)
		if got != "a b" {
			t.Errorf("Oneline(U+%04X) = %q, want %q", r, got, "a b")
		}
		if !utf8.ValidString(got) {
			t.Errorf("Oneline(U+%04X) returned invalid UTF-8: %q", r, got)
		}
	}
}

// The three spellings of the same terminal command — ESC [ 2 K, U+009B encoded as UTF-8, and the raw
// 0x9b byte an 8-bit terminal reads as CSI directly. The introducer is looked for byte by byte,
// because reaching the terminal is a fact about bytes: IndexByte for 0x9b catches the raw form and
// the second byte of the encoded one at once.
func TestOnelineNeutralisesACsiSequenceThatWouldEraseTheLineAboveIt(t *testing.T) {
	for _, erase := range []string{"\x1b[2K", "\u009b2K", string([]byte{0x9b}) + "2K"} {
		got := shell.Oneline("refused: evil" + erase + ".md")
		if strings.IndexByte(got, 0x1b) >= 0 || strings.IndexByte(got, 0x9b) >= 0 {
			t.Errorf("Oneline(%q) = %q, which still carries the introducer", erase, got)
		}
		if strings.Contains(got, "\n") {
			t.Errorf("Oneline(%q) = %q, which spans more than one line", erase, got)
		}
	}
}

// APFS refuses a raw 0x9b in a filename, but not in a file's content, and eco-check echoes
// content-derived text through Oneline — a cited section in citations.go, a matched rule ID in
// rule-ids.go. So a repository file can put the single-byte CSI into a message printed to a terminal,
// and 0x85 (NEL) into one that then spans two lines.
func TestOnelineReplacesARawC1ByteCarryingNoCharacter(t *testing.T) {
	for b := 0x80; b <= 0x9f; b++ {
		in := "a" + string([]byte{byte(b)}) + "b"
		got := shell.Oneline(in)
		if got != "a b" {
			t.Errorf("Oneline of the raw byte 0x%02x = %q, want %q", b, got, "a b")
		}
		if !utf8.ValidString(got) {
			t.Errorf("Oneline of the raw byte 0x%02x returned invalid UTF-8: %q", b, got)
		}
	}
}

// The C1 range doubles as the UTF-8 continuation range, so a guard written on byte value rather than
// on what the byte decodes to would shred every character below. Each one carries at least one byte
// in 0x80–0x9f, which is what makes them the evidence and not decoration.
func TestOnelineLeavesMultiByteCharactersWhole(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"an accented letter, second byte 0x89", "É"},
		{"a CJK character, middle byte 0x97", "日"},
		{"an emoji, four bytes carrying 0x9f and 0x8d", "\U0001F366"},
		{"a four-byte symbol, three of its bytes in the range", "\U0001D11E"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			carriesTheRange := false
			for i := 0; i < len(c.text); i++ {
				if c.text[i] >= 0x80 && c.text[i] <= 0x9f {
					carriesTheRange = true
				}
			}
			if !carriesTheRange {
				t.Fatalf("%q carries no byte in 0x80-0x9f, so it proves nothing about the guard", c.text)
			}
			in := "cited: " + c.text + ".md"
			if got := shell.Oneline(in); got != in {
				t.Fatalf("Oneline(%q) = %q, want it unchanged", in, got)
			}
		})
	}
}

// The decoder has to keep its place across a multi-byte character to space the raw byte beside it: a
// walk that lost a byte here would either eat part of 日 or leave the CSI standing.
func TestOnelineSpacesARawC1ByteBetweenTwoMultiByteCharacters(t *testing.T) {
	in := "日" + string([]byte{0x9b}) + "本"
	if got := shell.Oneline(in); got != "日 本" {
		t.Fatalf("Oneline(%q) = %q, want %q", in, got, "日 本")
	}
}

// A truncated multi-byte sequence orphans its continuation bytes, and an orphan in the C1 range is a
// byte an 8-bit terminal acts on with no character behind it, so it goes the way of any other. The
// lead byte stays: 0xe2 is outside the range, and eating it would rewrite a byte this function was
// only asked to make printable.
func TestOnelineSpacesAnOrphanedContinuationByteInTheC1Range(t *testing.T) {
	in := "name" + string([]byte{0xe2, 0x86}) // the first two bytes of →
	want := "name" + string([]byte{0xe2}) + " "
	if got := shell.Oneline(in); got != want {
		t.Fatalf("Oneline(%q) = %q, want %q", in, got, want)
	}
}

// The bound is chosen, not missed: a bidi override neither splits a line nor drives the terminal, and
// mapping it to a space would corrupt a legitimate right-to-left name. A caller needing that needs a
// different guard, not a wider one here.
func TestOnelineLeavesBidiOverridesAndOrdinaryTextAlone(t *testing.T) {
	for _, keep := range []string{
		"\u202eRIGHT-TO-LEFT OVERRIDE",
		"\u00a0", // NBSP is 0xc2 0xa0 — one byte past the C1 range, and not a control
		"→ … — é 日本語",
		"plain/path/to/a.md",
	} {
		if got := shell.Oneline(keep); got != keep {
			t.Errorf("Oneline(%q) = %q, want it unchanged", keep, got)
		}
	}
}

// A 0xc2 with nothing after it is a truncated sequence, not a C1 control. Eating it would rewrite
// bytes this function was only asked to make printable, and shorten a name the caller still has to
// recognise.
func TestOnelineLeavesATruncatedLeadByteAlone(t *testing.T) {
	in := "name" + string([]byte{0xc2})
	if got := shell.Oneline(in); got != in {
		t.Errorf("Oneline(%q) = %q, want it unchanged", in, got)
	}
}

// sameValues is reflect.DeepEqual minus the one thing these tables must not assert. Empty comes back
// as nil from SplitLines and as an allocated empty slice from SplitFields and SortUnique (text.go,
// above SplitLines), a difference no caller can read, so a case pinning it would freeze a contract
// the package leaves open. Length and contents are what a caller does read, and comparing those is
// what lets "no words" and "nothing in, nothing out" assert something instead of returning early.
func sameValues(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// Each function below documents itself as a shell tool under LC_ALL=C, and that claim is the contract
// these cases hold it to. The expectations were taken from the real tools once — `LC_ALL=C sort -u`
// puts A and B before a and b; `LC_ALL=C wc -w` reads "a\u00a0b" as one word and "a\tb\vc\fd" as
// four; `LC_ALL=C cut -c1-4` over two `→` returns four bytes and splits the second rune — and written
// down here rather than re-derived by spawning those tools, because a fork per case is what made the
// shell suite this package replaced too slow to run.

func TestCutBytesCutsBytesAndNotRunes(t *testing.T) {
	cases := []struct {
		name string
		text string
		n    int
		want string
	}{
		{"shorter than the bound is untouched", "abc", 10, "abc"},
		{"exactly the bound is untouched", "abc", 3, "abc"},
		{"longer is cut to the bound", "abcdef", 3, "abc"},
		{"a zero bound cuts everything", "abc", 0, ""},
		{"the empty string survives any bound", "", 5, ""},
		// `cut -c1-4` over "→→" returns 4 bytes, splitting the second rune. Bytes are the point: the
		// bound is on what a terminal is asked to print, not on what a locale calls a character.
		{"a multi-byte rune is split at the byte bound", "→→", 4, "→\xe2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shell.CutBytes(c.text, c.n); got != c.want {
				t.Fatalf("CutBytes(%q, %d) = %q, want %q", c.text, c.n, got, c.want)
			}
		})
	}
}

func TestIsSpaceByteIsTheCLocaleSpaceClass(t *testing.T) {
	for b := 0; b < 0x100; b++ {
		want := strings.IndexByte(" \t\n\v\f\r", byte(b)) >= 0
		if got := shell.IsSpaceByte(byte(b)); got != want {
			t.Errorf("IsSpaceByte(0x%02x) = %v, want %v", b, got, want)
		}
	}
	// The cutset and the predicate have to agree, or a scan that trims whitespace and a scan that
	// tests for it come to different answers about the same byte.
	for i := 0; i < len(shell.SpaceBytes); i++ {
		if !shell.IsSpaceByte(shell.SpaceBytes[i]) {
			t.Errorf("SpaceBytes carries 0x%02x, which IsSpaceByte rejects", shell.SpaceBytes[i])
		}
	}
}

func TestSplitLinesCountsLinesTheWayALineOrientedToolDoes(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string
	}{
		{"no text is no lines", "", nil},
		{"a line without its newline", "a", []string{"a"}},
		{"a trailing newline adds no empty record", "a\n", []string{"a"}},
		// "Bytes come back untouched" covers the CR of a CRLF file: only the final \n is a terminator,
		// so the line keeps its \r, which is what every line-oriented tool hands over.
		{"a CRLF line keeps its carriage return", "a\r\n", []string{"a\r"}},
		{"two lines", "a\nb", []string{"a", "b"}},
		{"two lines, terminated", "a\nb\n", []string{"a", "b"}},
		{"a blank line between two is kept", "a\n\nb\n", []string{"a", "", "b"}},
		// A file holding one newline holds one empty line, which is what `wc -l` counts. Only the
		// final terminator is dropped, so a second newline is a line of its own.
		{"one newline is one empty line", "\n", []string{""}},
		{"two newlines are two empty lines", "\n\n", []string{"", ""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shell.SplitLines(c.text); !sameValues(got, c.want) {
				t.Fatalf("SplitLines(%q) = %q, want %q", c.text, got, c.want)
			}
		})
	}
}

// SplitFields is what decides eco-stats' noteWordCap, the guard refusing an over-long note before it
// reaches a committed ledger, so what counts as one word here bounds what reaches that file.
func TestSplitFieldsCountsWordsTheWayWcDoes(t *testing.T) {
	cases := []struct {
		name string
		line string
		want []string
	}{
		{"no text is no words", "", nil},
		{"only whitespace is no words", " \t\n ", nil},
		{"runs of spaces at either end produce no empty fields", "  a  b  ", []string{"a", "b"}},
		{"every byte of the C-locale space class separates", "a\tb\vc\fd", []string{"a", "b", "c", "d"}},
		{"multi-byte words survive whole", "→ 日本語 é", []string{"→", "日本語", "é"}},
		// wc -w reads this as one word: the separator set is ASCII, so a non-breaking space joins
		// rather than splits. A note written with one is one word, not two.
		{"a non-breaking space does not separate", "a\u00a0b", []string{"a\u00a0b"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shell.SplitFields(c.line); !sameValues(got, c.want) {
				t.Fatalf("SplitFields(%q) = %q, want %q", c.line, got, c.want)
			}
		})
	}
}

// Oneline runs before SplitFields wherever a note is counted, so a control byte becomes a separator
// rather than staying inside a word. It makes the count honest — a word count over text nobody can
// see the shape of is not a count of anything — and it can only push a note over the cap, never
// under, so the guard it feeds still fails safe.
func TestOnelineMakesAControlByteSeparateWords(t *testing.T) {
	joined := shell.SplitFields("one\u009btwo")
	if len(joined) != 1 {
		t.Fatalf("SplitFields alone found %d words in a CSI-joined pair, want 1: %q", len(joined), joined)
	}
	split := shell.SplitFields(shell.Oneline("one\u009btwo"))
	if len(split) != 2 {
		t.Fatalf("after Oneline, SplitFields found %d words, want 2: %q", len(split), split)
	}
}

func TestSortUniqueSortsByByteAndDropsDuplicates(t *testing.T) {
	cases := []struct {
		name   string
		values []string
		want   []string
	}{
		{"nothing in, nothing out", nil, nil},
		// LC_ALL=C is byte order, so every capital sorts ahead of every lowercase letter.
		{"byte order, not locale order", []string{"b", "A", "a", "B"}, []string{"A", "B", "a", "b"}},
		{"duplicates collapse to one", []string{"a", "a", "a"}, []string{"a"}},
		{"already sorted and unique is unchanged", []string{"a", "b"}, []string{"a", "b"}},
		{"the empty string is a value like any other", []string{"b", "", "b"}, []string{"", "b"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shell.SortUnique(c.values); !sameValues(got, c.want) {
				t.Fatalf("SortUnique(%q) = %q, want %q", c.values, got, c.want)
			}
		})
	}
}

// "input left alone" is a promise the signature does not carry: sorting in place would reorder a
// slice the caller still holds, and every caller here passes one it built itself.
func TestSortUniqueLeavesItsInputAlone(t *testing.T) {
	values := []string{"c", "a", "b", "a"}
	before := append([]string(nil), values...)
	shell.SortUnique(values)
	if !reflect.DeepEqual(values, before) {
		t.Fatalf("SortUnique reordered its input to %q, want %q", values, before)
	}
}

package shell

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// Anything attacker-chosen that reaches a message goes through Oneline first: one physical line per
// message is what makes a ranking, or a grep for a refusal, mean anything. Every control byte goes,
// not only the two that split a line — an ESC sequence erases the real line printed above it.
//
// C1 as well as C0, and in both of the spellings a terminal reads. UTF-8 puts U+0080–U+009F at 0xc2
// followed by 0x80–0x9f, two bytes both above 0x7f, so a scan testing one byte at a time structurally
// cannot see them; a terminal in UTF-8 mode usually does not act on that encoded form, but "usually"
// is not the bar for a name chosen by the tree under review. The same argument convicts the raw 0x9b
// byte harder still, so guarding the encoded form alone had it backwards: raw 0x9b is CSI, `ESC [` as
// one byte, and a terminal in 8-bit mode acts on it with no "usually" about it. Raw 0x85 is NEL,
// which breaks the one line this function exists to keep whole.
//
// The rule that admits both spellings: a C1 byte carrying no character becomes a space, one carrying
// a character does not. The range doubles as UTF-8 continuation bytes — 0x97 sits inside 日, and
// three of the four bytes of U+1D11E are in it — so mapping by byte value would shred every accented
// letter, CJK character and emoji in exchange for blunting a hazard, which is a worse trade than the
// hazard. Decoding first tells the two apart, and a byte that decodes to nothing is carrying nothing,
// so spacing it costs no text. That reaches an orphaned continuation byte and a lone surrogate too:
// there is no character in either to keep.
//
// It does not follow that the output is safe on a terminal in 8-bit mode. That terminal never
// decodes, so it acts on the 0x97 inside 日 as readily, and the only guard against that is the one
// just refused. This takes the whole of the hazard that costs no text and stops.
//
// The bound stops at C1 by choice rather than by oversight. What this function exists to stop is text
// that splits a line or drives the terminal; a bidi override does neither, and mapping it to a space
// would corrupt a legitimate right-to-left name in exchange for blunting a hazard that changes how
// text reads rather than what the terminal does. A caller needing that needs a different guard, not a
// wider one here.
func Oneline(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	for i := 0; i < len(text); {
		char, width := utf8.DecodeRuneInString(text[i:])
		// A byte that decodes to nothing stands for itself, so a raw 0x9b is judged as the CSI an
		// 8-bit terminal reads rather than as the replacement rune Go hands back for it.
		if char == utf8.RuneError && width == 1 {
			char = rune(text[i])
		}
		if isControlChar(char) {
			out.WriteByte(' ')
		} else {
			out.WriteString(text[i : i+width])
		}
		i += width
	}
	return out.String()
}

// isControlChar is the set Oneline maps to a space: C0, DEL, and the C1 range above them. U+00A0 is
// NBSP rather than a control, which is why the range is bounded above at U+009F.
func isControlChar(char rune) bool {
	return char < 0x20 || char == 0x7f || (char >= 0x80 && char <= 0x9f)
}

// CutBytes is `cut -c1-n` under LC_ALL=C, which is bytes. Used where a message quotes a name the
// tree chose, so the bound has to be on what is printed, not on what a locale calls a character.
//
// n is non-negative. A negative bound panics on the slice, and nothing guards it, by choice: every
// caller passes a compile-time constant, so the input that reaches the panic does not exist. A guard
// has to pick between clamping to empty and returning the whole string, and that pick is a contract
// for a caller nobody has. Stating the bound instead leaves the choice to whoever first passes a
// computed n.
func CutBytes(text string, n int) string {
	if len(text) <= n {
		return text
	}
	return text[:n]
}

// SpaceBytes is the C-locale `[[:space:]]` as a cutset, for the strings.Trim family. IsSpaceByte is
// the same set as a predicate, and reads it, so a scan that trims whitespace and a scan that tests
// for it cannot come to different answers about a byte.
const SpaceBytes = " \t\n\v\f\r"

// IsSpaceByte is the C-locale `[[:space:]]`, which is what every scan was compared against.
func IsSpaceByte(b byte) bool {
	return strings.IndexByte(SpaceBytes, b) >= 0
}

// Nothing has two shapes across the three functions below, and neither shape is promised. SplitLines
// returns nil for empty input; SplitFields and SortUnique return an allocated empty slice, one
// inherited from strings.FieldsFunc and one from make. Every caller reads the result with len or
// range, which cannot tell the two apart, so the difference has never been observable — and picking
// one would freeze a contract nobody asked for, the same trade CutBytes leaves open above. It is
// stated here instead. The cases hold these functions to length and contents rather than to
// reflect.DeepEqual, so no test freezes what this note leaves open.

// SplitLines is one file as the line-oriented tools saw it: split on \n, with no empty record for a
// trailing newline. Bytes come back untouched — Go has no notion of a binary file, so the `-a` the
// shell version needed on every grep is gone with the hazard it guarded.
func SplitLines(text string) []string {
	if text == "" {
		return nil
	}
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return []string{""}
	}
	return strings.Split(text, "\n")
}

// SplitFields is awk's `split($0, f, /[[:space:]]+/)` minus the empty fields it produces at either
// end, which no caller could have used: an empty field carries no token. It is also `wc -w`, which
// counts exactly these runs.
func SplitFields(line string) []string {
	return strings.FieldsFunc(line, func(r rune) bool {
		return r < 0x80 && IsSpaceByte(byte(r))
	})
}

// SortUnique is `sort -u` under LC_ALL=C: byte order, duplicates dropped, input left alone.
func SortUnique(values []string) []string {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	unique := make([]string, 0, len(sorted))
	for i, value := range sorted {
		if i == 0 || value != sorted[i-1] {
			unique = append(unique, value)
		}
	}
	return unique
}

// asciiLower is awk's tolower under LC_ALL=C, which touches ASCII and nothing else.
func asciiLower(text string) string {
	out := []byte(text)
	for i, b := range out {
		if b >= 'A' && b <= 'Z' {
			out[i] = b + 'a' - 'A'
		}
	}
	return string(out)
}

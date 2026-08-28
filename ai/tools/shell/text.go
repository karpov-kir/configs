package shell

import (
	"sort"
	"strings"
)

// Anything attacker-chosen that reaches a message goes through Oneline first: one physical line per
// message is what makes a ranking, or a grep for a refusal, mean anything. Every control byte goes,
// not only the two that split a line — an ESC sequence erases the real line printed above it.
//
// C1 as well as C0. UTF-8 puts U+0080–U+009F at 0xc2 followed by 0x80–0x9f, two bytes both above
// 0x7f, so a scan testing one byte at a time structurally cannot see them. U+009B is CSI — `ESC [`
// as a single character — and it reached every message this function is the guard for. The exposure
// is smaller than a bare ESC, because a terminal in UTF-8 mode usually does not act on an encoded
// U+009B, but "usually" is not the bar for a name chosen by the tree under review.
//
// The bound stops there, and it stops there by choice rather than by oversight. What this function
// exists to stop is text that splits a line or drives the terminal; a lone surrogate and a bidi
// override do neither. Both are left alone, and mapping bidi to space would corrupt a legitimate
// right-to-left name in exchange for blunting a hazard that changes how text reads rather than what
// the terminal does. A caller needing that needs a different guard, not a wider one here.
func Oneline(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	for i := 0; i < len(text); i++ {
		b := text[i]
		switch {
		case b < 0x20 || b == 0x7f:
			out.WriteByte(' ')
		// 0xc2 0xa0 is NBSP, not a control, so the second byte is bounded above at 0x9f. A trailing
		// 0xc2 with nothing after it is left as it is: eating it would rewrite bytes this function was
		// only asked to make printable.
		case b == 0xc2 && i+1 < len(text) && text[i+1] >= 0x80 && text[i+1] <= 0x9f:
			out.WriteByte(' ')
			i++
		default:
			out.WriteByte(b)
		}
	}
	return out.String()
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

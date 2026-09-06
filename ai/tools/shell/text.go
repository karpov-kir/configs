package shell

import (
	"slices"
	"strings"
	"unicode/utf8"
)

// Anything attacker-chosen that reaches a message goes through Oneline first: one physical line per
// message is what makes a ranking, or a grep for a refusal, mean anything. Every control byte goes,
// not only the two that split a line — an ESC sequence erases the real line printed above it.
//
// C1 as well as C0, and in both of the spellings a terminal reads. UTF-8 puts U+0080–U+009F at 0xc2
// followed by 0x80–0x9f, two bytes both above 0x7f, so a scan testing one byte at a time structurally
// cannot see them; a UTF-8 terminal usually will not act on that encoded form, but "usually" is not
// the bar for a name the tree under review chose. Raw 0x9b needs no "usually" at all: it is CSI, `ESC
// [` as one byte, and a terminal in 8-bit mode acts on it. Raw 0x85 is NEL, which breaks the one line
// this function exists to keep whole.
//
// The rule that admits both spellings: a C1 byte carrying no character becomes a space, one carrying
// a character does not. The range doubles as UTF-8 continuation bytes — 0x97 sits inside 日, and
// three of the four bytes of U+1D11E are in it — so mapping by byte value would shred every accented
// letter, CJK character and emoji in exchange for blunting a hazard, which is the worse trade.
// Decoding first tells the two apart, and a byte that decodes to nothing is carrying nothing,
// so spacing it costs no text. That reaches an orphaned continuation byte and a lone surrogate too:
// there is no character in either to keep.
//
// None of which makes the output safe on a terminal in 8-bit mode. That terminal never decodes, so it
// acts on the 0x97 inside 日 as readily, and the only guard against that is the one just refused. This
// takes the whole of the hazard that costs no text and stops.
//
// The bound stops at C1. What this function stops is text that splits a line or drives the terminal; a
// bidi override does neither, and mapping it to a space would corrupt a legitimate right-to-left name
// to blunt a hazard that changes how text reads rather than what the terminal does. A caller needing
// that needs a different guard, not a wider one here.
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

// CutBytes is `cut -c1-n` under LC_ALL=C, which is bytes. It bounds a message quoting a name the tree
// chose, so the bound has to be on what is printed, not on what a locale calls a character.
//
// Callers want CutBytesMarked below, which is this cut plus a marker saying it happened; no message in
// these tools reaches CutBytes directly any more. It stays exported for the harness rather than for a
// caller: go-mutate proves the marker is load-bearing by mutating each of those call sites back to
// this function, and a mutant that will not compile proves nothing.
//
// n is non-negative. A negative bound panics on the slice, and nothing guards it, by choice: every
// caller passes a compile-time constant, so the input that reaches the panic does not exist. A guard
// has to pick between clamping to empty and returning the whole string, and that pick is a contract
// for a caller nobody has. Stating the bound instead leaves the choice to whoever first passes a
// computed n.
//
// This cuts bytes and Oneline cuts runes, and the two disagreeing is the reason for the one departure
// from `cut` below. A message is cut only after Oneline has run over it: Oneline decodes, and keeps
// a multi-byte character because it carries a character rather than a control. `cut` would then
// slice that character in half and hand the leading bytes on — and U+16C0 encodes as e1 9b 80, so a
// cut between its second and third byte leaves 0x9b standing. That is CSI, the single byte Oneline
// exists to keep out of a message. Running second, the bound was undoing the guard.
//
// So a rune the bound falls inside is dropped rather than halved. The bound is still on bytes and the
// result is still at most n of them; what changes is only that a sequence this cut broke is not
// printed. A byte the text already carried is left exactly where it is, the way Oneline leaves a
// truncated lead byte alone — this repairs nothing, it only declines to break something.
func CutBytes(text string, n int) string {
	if len(text) <= n {
		return text
	}
	cut := n
	// Back to the start of the rune the bound landed inside, at most UTFMax-1 bytes away. Only that
	// rune is examined, and only when it really does run past the bound.
	for start := cut - 1; start >= 0 && cut-start < utf8.UTFMax; start-- {
		if !utf8.RuneStart(text[start]) {
			continue
		}
		if _, width := utf8.DecodeRuneInString(text[start:]); start+width > cut {
			cut = start
		}
		break
	}
	return text[:cut]
}

// CutMarker is what CutBytesMarked ends a cut result with. Three ASCII dots rather than an ellipsis
// character, because a message reaches the terminal through Oneline and the marker has to be the one
// part of it that carries no byte Oneline exists to strip.
const CutMarker = "..."

// CutBytesMarked is CutBytes with the cut made visible. Same bound, same refusal to halve a rune; the
// difference is that a result shorter than the input says so.
//
// Which matters wherever the text is a refusal's reason. A reason cut at the bound stops being a
// reason and starts being a shorter wrong one: `permission denied` arrives as `permission de`, and a
// mount message cut at its colon arrives ending in `: `, which reads as "there was no reason" rather
// than as "the reason did not fit". A refusal that names three possibilities and then drops which one
// it was is the same defect as a scan that stops early and says nothing.
//
// The marker is inside the bound, not added past it: the result is at most n bytes, as CutBytes
// promises. n is therefore at least len(CutMarker), and CutBytes's note on an unguarded bound holds
// here too.
func CutBytesMarked(text string, n int) string {
	if len(text) <= n {
		return text
	}
	return CutBytes(text, n-len(CutMarker)) + CutMarker
}

// SpaceBytes is the C-locale `[[:space:]]` as a cutset, for the strings.Trim family. IsSpaceByte is
// the same set as a predicate, and reads it, so a scan that trims whitespace and a scan that tests
// for it cannot come to different answers about a byte.
const SpaceBytes = " \t\n\v\f\r"

func IsSpaceByte(b byte) bool {
	return strings.IndexByte(SpaceBytes, b) >= 0
}

// Nothing has two shapes across the three functions below, and neither shape is promised. SplitLines
// and SortUnique return nil for empty input; SplitFields returns the allocated empty slice it
// inherits from strings.FieldsFunc. Every caller reads the result with len or range, which cannot
// tell the two apart, so the difference has never been observable — and picking one would freeze a
// contract nobody asked for, the same trade CutBytes leaves open above. It is stated here instead.
// The cases hold these functions to length and contents rather than to reflect.DeepEqual, so no test
// freezes what this note leaves open.

// SplitLines splits on \n, with no empty record for a trailing newline. Bytes come back untouched:
// there is no binary-file notion here to make a line vanish.
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
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	return slices.Compact(sorted)
}

// AsciiLower is awk's tolower under LC_ALL=C, which touches ASCII and nothing else. strings.ToLower
// would fold non-ASCII too, so the text under review would decide whether a scan matched.
func AsciiLower(text string) string {
	out := []byte(text)
	for i, b := range out {
		if b >= 'A' && b <= 'Z' {
			out[i] = b + 'a' - 'A'
		}
	}
	return string(out)
}

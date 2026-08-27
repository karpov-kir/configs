package shell

import (
	"sort"
	"strings"
)

// Anything attacker-chosen that reaches a message goes through Oneline first: one physical line per
// message is what makes a ranking, or a grep for a refusal, mean anything. Every control byte goes,
// not only the two that split a line — an ESC sequence erases the real line printed above it.
func Oneline(text string) string {
	out := []byte(text)
	for i, b := range out {
		if b < 0x20 || b == 0x7f {
			out[i] = ' '
		}
	}
	return string(out)
}

// CutBytes is `cut -c1-n` under LC_ALL=C, which is bytes. Used where a message quotes a name the
// tree chose, so the bound has to be on what is printed, not on what a locale calls a character.
func CutBytes(text string, n int) string {
	if len(text) <= n {
		return text
	}
	return text[:n]
}

// IsSpaceByte is the C-locale `[[:space:]]`, which is what every scan was compared against.
func IsSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\v' || b == '\f' || b == '\r'
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

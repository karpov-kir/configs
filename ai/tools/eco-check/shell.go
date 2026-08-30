package ecocheck

import (
	"fmt"
	"os"
	"strings"

	"kk-flavor/tools/shell"
)

// What this checker needs of the shell version's primitives beyond the ones `kk-flavor/tools/shell`
// holds for both ports: a bounded file read, the two `wc` figures, and the comparison form a heading
// is matched in. Their exact edges — which space `s/^ //` removes, which bytes `wc -w` splits on —
// are the contract several scans read a finding out of.

// Comparison form for a heading or a cited section name: lowercased, markdown emphasis dropped,
// whitespace runs flattened, one leading and one trailing space removed.
func plainText(text string) string {
	out := make([]byte, 0, len(text))
	wasSpace := false
	for i := 0; i < len(text); i++ {
		b := text[i]
		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}
		if b == '`' || b == '*' || b == '_' {
			continue
		}
		if shell.IsSpaceByte(b) {
			if !wasSpace {
				out = append(out, ' ')
			}
			wasSpace = true
			continue
		}
		wasSpace = false
		out = append(out, b)
	}
	return strings.TrimSuffix(strings.TrimPrefix(string(out), " "), " ")
}

func isAlnumByte(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// The bound and its reason are shell.MaxFileBytes. Over it the file is reported and not read, which
// is what keeps an unchecked file distinguishable from a checked one.
const maxFileBytes = shell.MaxFileBytes

// The one wording every bounded read here reports itself with, ending on what this reader did not do.
// Three scans hit the bound and each says something different about the consequence; only that half
// differs, and a second copy of the rest would drift from the bound it quotes.
func tooLargeToScan(name string, size int64, consequence string) string {
	return fmt.Sprintf("file too large to scan: %s is %d bytes, over the %d-byte bound — %s",
		name, size, maxFileBytes, consequence)
}

// A file the walk handed over and the read then refused. Both bounds above already turn "nothing was
// read" into a finding rather than an empty result; a file this process cannot open is the other way
// that happens, and it was the silent one. Every scan skips such a file, which is the right call — the
// reviewed tree chooses what is unreadable, so stopping would be a switch a branch could throw — but
// skipping it *quietly* made three files at mode 000 read as 41 dangling-section findings against the
// files that cited them, a census line 34 words short under an unchanged denominator, and nothing on
// stderr. Reported, the skip is still a skip and the run can no longer exit 0 over it.
func couldNotRead(name, consequence string) string {
	return fmt.Sprintf("file could not be read: %s — %s", name, consequence)
}

func (c *checker) readLines(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxFileBytes {
		c.add(tooLargeToScan(shell.Oneline(path), info.Size(), "it was NOT checked"))
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// The error still comes back, so every caller goes on skipping the file exactly as before.
		// Several scans read one file, and the findings deduplicate, so this is one line per file.
		c.add(couldNotRead(shell.Oneline(path), "it was NOT checked"))
		return nil, err
	}
	return shell.SplitLines(string(data)), nil
}

func countNonEmptyLines(text string) int {
	count := 0
	for _, line := range shell.SplitLines(text) {
		if line != "" {
			count++
		}
	}
	return count
}

// `wc -l` counts newline bytes, `wc -w` counts runs of non-whitespace. Both figures ride the
// exit-0 census line, so they are counted per file and summed by the caller: concatenating first
// would glue the last word of a file with no final newline onto the first word of the next.
//
// Bounded by maxFileBytes for the reason readLines is, and reporting the same way. Reported and not
// counted, never truncated — a file whose words went uncounted must not read as one that had none.
func (c *checker) countLinesAndWords(path string) (lines, words int) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, 0
	}
	if info.Size() > maxFileBytes {
		c.add(tooLargeToScan(shell.Oneline(path), info.Size(), "it was NOT counted"))
		return 0, 0
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0
	}
	inWord := false
	for i := 0; i < len(data); i++ {
		b := data[i]
		if b == '\n' {
			lines++
		}
		if shell.IsSpaceByte(b) {
			inWord = false
			continue
		}
		if !inWord {
			words++
			inWord = true
		}
	}
	return lines, words
}

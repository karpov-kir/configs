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

// The reviewed tree chooses this file's size, so the read is bounded: a committed 64 MiB of newlines
// is 408 KB packed and took 2.48 GB of resident memory, against the 7.5 MB the streaming shell version
// used, and half a gigabyte of it OOM-kills the review stage outright. The bound is far above any real
// instruction file — the largest in this tree is under 60 KB — so hitting it is a statement about the
// branch, not about the tree growing.
// Over the bound the file is **reported and not read**. Truncating it instead would leave an unchecked
// file indistinguishable from a checked one, which is the failure `check.sh` names three times over.
const maxFileBytes = 8 << 20

func (c *checker) readLines(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxFileBytes {
		c.add(fmt.Sprintf("file too large to scan: %s is %d bytes, over the %d-byte bound — it was NOT checked",
			shell.Oneline(path), info.Size(), maxFileBytes))
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
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
func countLinesAndWords(path string) (lines, words int) {
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

package ecoreport

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"kk-flavor/tools/shell"
)

// The primitives this tool holds itself, ported from the shell version: what `kk-flavor/tools/shell`
// does not already share with every tool here, plus the directory walk two refusals count through. Each
// is here because a refusal in this tool turns on its exact edges — what `rm -f` refuses, what `mv` does
// across devices, which question `-r` and `-x` really asked, and what `find -type f` does and does not
// call a file.

// `${value#"${value%%[![:space:]]*}"}` — leading whitespace, and nothing else, removed.
func trimLeadingSpace(value string) string {
	for i := 0; i < len(value); i++ {
		if !shell.IsSpaceByte(value[i]) {
			return value[i:]
		}
	}
	return ""
}

// `${value%%[[:space:]]*}` — everything up to the first whitespace byte.
func firstField(value string) string {
	for i := 0; i < len(value); i++ {
		if shell.IsSpaceByte(value[i]) {
			return value[:i]
		}
	}
	return value
}

// The slug charset, `[0-9A-Za-z._-]`. It is what stops a slug `../`-escaping a path it indexes, so
// it is a whitelist and stays one: `/` outside the set is the whole point.
func isSlugByte(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' ||
		b == '.' || b == '_' || b == '-'
}

func isSlugCharset(value string) bool {
	for i := 0; i < len(value); i++ {
		if !isSlugByte(value[i]) {
			return false
		}
	}
	return true
}

func isNonEmptyFile(path string) bool { // -s
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

// -r and -x as access(2) answers them, which is the question the shell's own test builtins asked.
// `shell` deliberately holds no readability test, because there are two answers and they differ:
// ecoroot's containment test opens the file, and these must not, since root reads anything and the
// suite skips its permission cases on exactly that answer.
func isReadable(path string) bool { return syscall.Access(path, 0x4) == nil }

func isExecutable(path string) bool { return syscall.Access(path, 0x1) == nil }

// `rm -f`: a missing path is success, a directory is not. os.Remove takes an empty directory
// happily, and `init` reads a failure here as "something is in the way".
func rmFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return nil
	}
	if info.IsDir() {
		return &os.PathError{Op: "remove", Path: path, Err: syscall.EISDIR}
	}
	return os.Remove(path)
}

// `mv`. os.Rename alone is not mv: rename(2) refuses across devices.
//
// No caller reaches the fallback today: all three stage their temp file in the destination's own
// directory. Keep it that way. The fallback follows a symlink at the destination and truncates before
// writing, where the rename would have replaced it atomically — so a fourth caller staging somewhere
// else is what falls into it.
func moveFile(from, to string) error {
	err := os.Rename(from, to)
	if err == nil || !isCrossDevice(err) {
		return err
	}
	info, err := os.Lstat(from)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	if err := os.WriteFile(to, content, info.Mode().Perm()); err != nil {
		return err
	}
	return os.Remove(from)
}

func isCrossDevice(err error) bool {
	var errno syscall.Errno
	if pathErr, ok := err.(*os.LinkError); ok {
		if errno, ok = pathErr.Err.(syscall.Errno); ok {
			return errno == syscall.EXDEV
		}
	}
	return false
}

// `cp`, for the one copy in the tool: template → staged report. The mode comes from the source, as
// cp's own open(2) does, so the report lands with the template's permissions and not 0600.
func copyFile(from, to string) error {
	info, err := os.Stat(from)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, content, info.Mode().Perm())
}

// `rmdir a b c 2>/dev/null || true` — each removed only if empty, every failure discarded.
func rmdirIfEmpty(paths ...string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}

// What awk wrote back: every record followed by ORS, so a file whose last line had no newline gains one.
func joinRecords(lines []string) []byte {
	var out strings.Builder
	for _, line := range lines {
		out.WriteString(line)
		out.WriteString("\n")
	}
	return []byte(out.String())
}

// `printf '%s\n' "$text" | wc -l`, which counts the newline printf adds — so a one-line value is 1
// and an empty one is 1 too. Every caller has already established the value is non-empty.
func countPrintedLines(text string) int {
	return strings.Count(text, "\n") + 1
}

// `sed 's/^/  /'` over a captured block, which is how a refusal quotes a list back.
func indentLines(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func writeAll(w io.Writer, text string) {
	_, _ = io.WriteString(w, text)
}

// The POSIX cksum CRC, which is what the stage markers hold. Reimplemented rather than shelled out to,
// because during any swap a marker written by one version of this tool is read by the other: a different
// digest there would read as "the report has moved" and free a stamp the pass never earned.
func cksum(content []byte) (uint32, int) {
	var crc uint32
	for _, b := range content {
		crc = crc<<8 ^ cksumTable[byte(crc>>24)^b]
	}
	for length := len(content); length != 0; length >>= 8 {
		crc = crc<<8 ^ cksumTable[byte(crc>>24)^byte(length)]
	}
	return ^crc, len(content)
}

// The CRC-32 table cksum reads, over the POSIX polynomial. Built once: it is the same 256 entries for
// every marker, and a marker is written and read on every stage return.
var cksumTable = newCksumTable()

func newCksumTable() [256]uint32 {
	var table [256]uint32
	for i := range table {
		crc := uint32(i) << 24
		for range 8 {
			if crc&0x80000000 != 0 {
				crc = crc<<1 ^ 0x04C11DB7
			} else {
				crc <<= 1
			}
		}
		table[i] = crc
	}
	return table
}

// How many non-directory entries a directory holds, and up to sampleBound of their paths relative to
// it. The count and the sample are separate returns because the sample is bounded: reporting len(sample)
// as the count understates a directory of a hundred files as twenty.
//
// Symlinks count and are never followed. The link itself is something that would be lost, and what is at
// the other end is not this directory's to weigh. Counting only regular files gave a directory holding
// just `charter.md -> ../docs/charter.md` a count of zero, which both callers read as "nothing here to
// lose" — reconcileTreeIdsdDir's RemoveAll deleted the link in silence, and assertPromotionTargetIsClear
// would have promoted over it, while a symlink at either directory itself was loudly refused.
func filesUnder(dir string) (count int, sample []string, err error) {
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		count++
		if len(sample) < sampleBound {
			sample = append(sample, strings.TrimPrefix(path, dir+"/"))
		}
		return nil
	})
	return count, sample, err
}

// How many paths a refusal names before it stops.
const sampleBound = 20

// What the refusal adds when the sample is short of the count, so the two never read as disagreeing.
func sampleTail(count, shown int) string {
	if count <= shown {
		return ""
	}
	return " (and " + strconv.Itoa(count-shown) + " more)"
}

package commentpass

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const baseLedger = `// Posts entries.
export function post(entries: Entry[]): void {
  entries.forEach(write);
}

// A ledger closes a book at midnight.
export const CLOSE_HOUR = 0;
`

const headLedger = `// Posts entries.
export function post(entries: Entry[]): void {
  entries.sort(byDate).forEach(write);
}

// A ledger closes a book at midnight.
export const CLOSE_HOUR = 0;

export function reopen(book: Book): void {
  book.open = true;
}
`

// fixture is a repository whose head changes post's body and adds reopen, beside an untouched constant.
func fixture(t *testing.T) (dir, base string) {
	t.Helper()
	dir = t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	write(t, dir, ".prettierrc", `{"printWidth": 120}`)
	write(t, dir, "ledger.ts", baseLedger)
	git("add", "-A")
	git("commit", "-qm", "base")
	base = git("rev-parse", "HEAD")
	write(t, dir, "ledger.ts", headLedger)
	return dir, base
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// run passes the fixture with a caller that answers `reply` and counts its calls.
func run(t *testing.T, dir, base, reply string, calls *int, extra ...string) (int, string) {
	t.Helper()
	page := filepath.Join(t.TempDir(), "page.md")
	write(t, filepath.Dir(page), "page.md", "# Comments\nWrite a comment only where it is needed.\n")
	state := t.TempDir()
	env := map[string]string{"HOME": t.TempDir(), "XDG_STATE_HOME": state}
	if s, ok := os.LookupEnv("TEST_STATE"); ok {
		env["XDG_STATE_HOME"] = s
	}
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	caller := func(string) Caller {
		return func(system, user string) (string, error) {
			*calls++
			return reply, nil
		}
	}
	var out, errOut strings.Builder
	code := Run("comment-pass.sh", append([]string{"--base=" + base, "--page=" + page}, extra...), dir, lookup, caller, &out, &errOut)
	return code, out.String() + errOut.String()
}

// The change's material is the comment on the changed function and the function it added. The
// constant's comment, which the change left alone, is context.
func TestTheMaterialIsWhatTheChangeTouched(t *testing.T) {
	dir, base := fixture(t)
	calls := 0
	code, text := run(t, dir, base, "", &calls, "--list")
	if code != 0 || !strings.Contains(text, "ledger.ts: 1 comment(s), 1 place(s)") ||
		!strings.Contains(text, "c1 lines 1-1, on line 2") || !strings.Contains(text, "p1 line 9") || calls != 0 {
		t.Fatalf("exit %d, %d call(s):\n%s", code, calls, text)
	}
}

// One call writes the file: a rewrite replaces the block, an add goes above its declaration at its
// indent, and the untouched comment stands.
func TestOneCallRewritesAndAdds(t *testing.T) {
	dir, base := fixture(t)
	calls := 0
	reply := "c1 rewrite: the order is what the caller relies on\n// Posts entries oldest first, so a later entry never precedes the one it corrects.\n" +
		"p1 add: reopening clears a closed state the caller cannot see\n// Reopens the book, which lets a correction post after the close.\n"
	code, text := run(t, dir, base, reply, &calls)
	got, _ := os.ReadFile(filepath.Join(dir, "ledger.ts"))
	want := "// Posts entries oldest first, so a later entry never precedes the one it corrects.\nexport function post(" +
		"entries: Entry[]): void {\n  entries.sort(byDate).forEach(write);\n}\n\n// A ledger closes a book at midnight.\n" +
		"export const CLOSE_HOUR = 0;\n\n// Reopens the book, which lets a correction post after the close.\nexport function reopen("
	if code != 0 || calls != 1 || !strings.HasPrefix(string(got), want) {
		t.Fatalf("exit %d, %d call(s):\n%s\n%s", code, calls, text, got)
	}
}

// A reply that leaves an id unanswered, answers one it was not offered, or gives a verb its kind does
// not take fails the file, which stays as it was.
func TestAMalformedReplyFailsTheFile(t *testing.T) {
	for _, reply := range []string{
		"c1 keep: fine\n",
		"c1 keep: fine\np1 skip: fine\nc9 keep: fine\n",
		"c1 add: wrong\np1 skip: fine\n",
		"c1 rewrite: no text\np1 skip: fine\n",
	} {
		dir, base := fixture(t)
		calls := 0
		code, text := run(t, dir, base, reply, &calls)
		got, _ := os.ReadFile(filepath.Join(dir, "ledger.ts"))
		if code != 1 || string(got) != headLedger || !strings.Contains(text, "does not parse") {
			t.Errorf("%q: exit %d\n%s", reply, code, text)
		}
	}
}

// The gate refuses a block over four prose lines, a line over the width and markdown, with the text
// shown, and leaves the file as it was.
func TestTheGateRefusesWithTheTextShown(t *testing.T) {
	for _, comment := range []string{
		"// one\n// two\n// three\n// four\n// five\n",
		"// " + strings.Repeat("word ", 30) + "\n",
		"// **bold** words\n",
	} {
		dir, base := fixture(t)
		calls := 0
		code, text := run(t, dir, base, "c1 rewrite: r\n"+comment+"p1 skip: none needed\n", &calls)
		got, _ := os.ReadFile(filepath.Join(dir, "ledger.ts"))
		if code != 1 || string(got) != headLedger || !strings.Contains(text, "the gate refused") {
			t.Errorf("%q: exit %d\n%s", comment, code, text)
		}
	}
}

// A rerun with the page, the file and the notes unchanged calls no model and applies the last reply.
func TestARerunOfAnUnchangedFileReusesItsReply(t *testing.T) {
	dir, base := fixture(t)
	state := t.TempDir()
	t.Setenv("TEST_STATE", state)
	calls := 0
	if code, text := run(t, dir, base, "c1 keep: fine\np1 skip: fine\n", &calls); code != 0 || calls != 1 {
		t.Fatalf("first: exit %d, %d call(s)\n%s", code, calls, text)
	}
	if code, text := run(t, dir, base, "c1 keep: fine\np1 skip: fine\n", &calls); code != 0 || calls != 1 || !strings.Contains(text, "reply kept") {
		t.Fatalf("rerun: exit %d, %d call(s)\n%s", code, calls, text)
	}
	notes := filepath.Join(t.TempDir(), "notes.txt")
	write(t, filepath.Dir(notes), "notes.txt", "ledger.ts:1 the summary says nothing the name does not\n")
	if code, text := run(t, dir, base, "c1 keep: fine\np1 skip: fine\n", &calls, "--notes="+notes); code != 0 || calls != 2 {
		t.Fatalf("a new note: exit %d, %d call(s)\n%s", code, calls, text)
	}
}

// A reviewer's note on the file goes into its call.
func TestTheNotesReachTheFilesCall(t *testing.T) {
	lines := strings.Split(headLedger, "\n")
	m := findMaterial("ledger.ts", lines, map[int]bool{3: true})
	prompt := userPrompt("ledger.ts", lines, m, []string{"ledger.ts:1 the summary says nothing the name does not"})
	if !strings.Contains(prompt, "The reviewer's notes on this file") || !strings.Contains(prompt, "ledger.ts:1 the summary") {
		t.Fatalf("prompt:\n%s", prompt)
	}
}

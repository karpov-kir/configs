package score

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const trackedConfig = `# What a score has to beat to stay.
default        cut <= 7
reply          cut <= 7
outward-text   cut <= 7
report         cut <= 7
code-comment   cut <= 7
instruction    cut <= 7
always-loaded  cut <= 7
record-entry   cut <= 7
`

type run struct {
	t      *testing.T
	env    Env
	stdout strings.Builder
	stderr strings.Builder
	code   int
}

func newRun(t *testing.T, tracked string, override *string) *run {
	t.Helper()
	dir := t.TempDir()
	config := filepath.Join(dir, "thresholds.conf")
	if err := os.WriteFile(config, []byte(tracked), 0o644); err != nil {
		t.Fatalf("could not write the fixture config: %v", err)
	}
	env := Env{ConfigPath: config, OverridePath: filepath.Join(dir, "override.conf")}
	if override != nil {
		if err := os.WriteFile(env.OverridePath, []byte(*override), 0o644); err != nil {
			t.Fatalf("could not write the fixture override: %v", err)
		}
	}
	return &run{t: t, env: env}
}

func (r *run) do(stdin string, args ...string) {
	r.doReading(strings.NewReader(stdin), args...)
}

func (r *run) doReading(stdin io.Reader, args ...string) {
	r.stdout.Reset()
	r.stderr.Reset()
	r.code = Run("score.sh", args, r.env, stdin, &r.stdout, &r.stderr)
}

func (r *run) out() string { return r.stdout.String() + r.stderr.String() }

func (r *run) expectCode(want int) {
	r.t.Helper()
	if r.code != want {
		r.t.Errorf("exit %d, wanted %d — output: %s", r.code, want, r.out())
	}
}

func (r *run) expectOut(want string) {
	r.t.Helper()
	if !strings.Contains(r.out(), want) {
		r.t.Errorf("wanted %q in: %s", want, r.out())
	}
}

func (r *run) expectNotOut(unwanted string) {
	r.t.Helper()
	if strings.Contains(r.out(), unwanted) {
		r.t.Errorf("%q appears in: %s", unwanted, r.out())
	}
}

// Substring is the wrong test for a bare number: the failure path prints the config's path, and a
// temp directory carrying that digit passes a substring test with the guard removed.
func (r *run) expectStdoutExactly(want string) {
	r.t.Helper()
	if r.stdout.String() != want {
		r.t.Errorf("stdout is %q, wanted exactly %q", r.stdout.String(), want)
	}
}

// A control byte in the output is not cosmetic: it rewrites lines already printed. Newline is the only
// one this report uses.
func (r *run) expectNoControl() {
	r.t.Helper()
	for _, char := range r.out() {
		if char == '\n' {
			continue
		}
		if char < 0x20 || char == 0x7f {
			r.t.Errorf("a control byte (%q) survived into: %q", char, r.out())
			return
		}
	}
}

// The C1 range the rune scan above structurally cannot see. Raw 0x9b is CSI — `ESC [` as one byte —
// which a terminal in 8-bit mode acts on, and it decodes to no character, so a scan over runes reads
// it as U+FFFD and finds nothing. The test for it is therefore on bytes.
func (r *run) expectNoRawCSI() {
	r.t.Helper()
	if i := strings.IndexByte(r.out(), 0x9b); i >= 0 {
		r.t.Errorf("a raw CSI byte survived at offset %d into: %q", i, r.out())
	}
}

// Cases for the repeated-literal detector. Two must not be weakened.
//
// "a path argument is refused with exit 2, never scanned": `git diff <path>` is legal and diffs
// against the index, so a path quietly accepted scans the wrong change set and exits 0 —
// indistinguishable from a clean tree, and the whole point of the tool is lost silently.
//
// "an untracked file whose name marks it as secret-bearing is never read": this echoes 60 bytes of
// every duplicate, so the untracked arm is a route from a secret into the transcript, the qualify
// report, and any PR comment drafted from either. Two .env files sharing one API token is the ordinary
// case — the token is over the length floor, appears twice, and would print.
package duplicates

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var seedRepo string

func TestMain(m *testing.M) {
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	base, err := os.MkdirTemp("", "dup-seed")
	if err != nil {
		panic("dup tests: no temp dir, so nothing was tested: " + err.Error())
	}
	defer os.RemoveAll(base)
	os.Setenv("HOME", filepath.Join(base, "home"))
	os.MkdirAll(os.Getenv("HOME"), 0o755)
	seedRepo = filepath.Join(base, "seed")
	if err := buildSeed(seedRepo); err != nil {
		panic("dup tests: could not build the seed repository, so nothing was tested: " + err.Error())
	}
	os.Exit(m.Run())
}

func buildSeed(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"init", "-q"}, {"config", "user.email", "t@t"},
		{"config", "user.name", "t"}, {"config", "commit.gpgsign", "false"},
	} {
		if err := git(dir, args...); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		return err
	}
	if err := git(dir, "add", "seed.txt"); err != nil {
		return err
	}
	return git(dir, "commit", "-qm", "base")
}

func git(dir string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	return cmd.Run()
}

type repo struct {
	t      *testing.T
	dir    string
	stdout strings.Builder
	stderr strings.Builder
	code   int
}

func newRepo(t *testing.T) *repo {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	if err := copyTree(seedRepo, dir); err != nil {
		t.Fatalf("could not build a fixture repo: %v — stopping, since every case reads one", err)
	}
	return &repo{t: t, dir: dir}
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, info.Mode().Perm())
	})
}

func (r *repo) write(name, body string) {
	r.t.Helper()
	full := filepath.Join(r.dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		r.t.Fatalf("write %s: %v", name, err)
	}
}

func (r *repo) commit(message string) {
	r.t.Helper()
	if err := git(r.dir, "add", "-A"); err != nil {
		r.t.Fatalf("stage: %v", err)
	}
	if err := git(r.dir, "commit", "-qm", message); err != nil {
		r.t.Fatalf("commit: %v", err)
	}
}

func (r *repo) run(args ...string) {
	r.runWith(Config{MinLength: defaultMinLength, MaxFileBytes: defaultMaxFileBytes}, args...)
}

func (r *repo) runWith(cfg Config, args ...string) {
	r.stdout.Reset()
	r.stderr.Reset()
	r.code = Run("dup-literals.sh", args, r.dir, cfg, &r.stdout, &r.stderr)
}

func (r *repo) expectCode(want int) {
	r.t.Helper()
	if r.code != want {
		r.t.Errorf("exit %d, wanted %d\nstdout: %s\nstderr: %s", r.code, want, r.stdout.String(), r.stderr.String())
	}
}

func (r *repo) expectStdoutHas(want string) {
	r.t.Helper()
	if !strings.Contains(r.stdout.String(), want) {
		r.t.Errorf("wanted %q on stdout, got: %s", want, r.stdout.String())
	}
}

func (r *repo) expectStdoutLacks(unwanted string) {
	r.t.Helper()
	if strings.Contains(r.stdout.String(), unwanted) {
		r.t.Errorf("%q appears on stdout: %s", unwanted, r.stdout.String())
	}
}

func (r *repo) expectNoStdout() {
	r.t.Helper()
	if r.stdout.Len() != 0 {
		r.t.Errorf("expected nothing on stdout, got: %s", r.stdout.String())
	}
}

func (r *repo) expectStderrHas(want string) {
	r.t.Helper()
	if !strings.Contains(r.stderr.String(), want) {
		r.t.Errorf("wanted %q on stderr, got: %s", want, r.stderr.String())
	}
}

// A run of one character, of the length a case is about. Every literal below comes from here.
func repeated(char rune, n int) string { return strings.Repeat(string(char), n) }

// --- an unchanged tree --------------------------------------------------------------------------

func TestAnUnchangedTree(t *testing.T) {
	r := newRepo(t)
	r.run("HEAD")
	r.expectCode(0)
	r.expectNoStdout()
	r.expectStderrHas("0 file(s) reached the scan")
	// The denominator is what tells "nothing repeated" from "nothing was read".
	r.expectStderrHas("says nothing about the change set")
}

// --- the arguments it refuses -------------------------------------------------------------------

func TestARevisionIsNotAPath(t *testing.T) {
	t.Run("a path exits 2 and is named as a path", func(t *testing.T) {
		r := newRepo(t)
		r.write("seen.go", "x\n")
		r.commit("add seen")
		r.run("seen.go")
		r.expectCode(2)
		r.expectStderrHas("is a path, not a git-diff revision")
		r.expectStderrHas("the scan did NOT run")
		r.expectNoStdout()
	})

	t.Run("an option exits 2 and is named as an option", func(t *testing.T) {
		r := newRepo(t)
		r.run("--output=/dev/null")
		r.expectCode(2)
		r.expectStderrHas("is an option, not a git-diff revision")
		r.expectNoStdout()
	})

	t.Run("a revision git cannot resolve exits 2 as git's rejection", func(t *testing.T) {
		r := newRepo(t)
		r.run("no-such-rev")
		r.expectCode(2)
		r.expectStderrHas("git rejected these arguments")
		r.expectStderrHas("Not a clean result")
		r.expectNoStdout()
	})
}

// --- what counts as a duplicate ------------------------------------------------------------------

func TestARepeatedLineIsFound(t *testing.T) {
	r := newRepo(t)
	r.write("base.go", "package fixture\n")
	r.commit("base")
	long := repeated('a', 120)
	r.write("base.go", long+"\n"+long+"\n")
	r.run("HEAD")
	r.expectCode(1)
	r.expectStdoutHas("2x")
	r.expectStdoutHas("120 chars")
}

func TestARepeatedTokenInsideDifferingLinesIsFound(t *testing.T) {
	r := newRepo(t)
	r.write("base.go", "package fixture\n")
	r.commit("base")
	token := repeated('k', 130)
	r.write("base.go", "first = \""+token+"\"\nsecond = \""+token+"\"\n")
	r.run("HEAD")
	r.expectCode(1)
	r.expectStdoutHas("2x token")
	r.expectStdoutHas("130 chars")
}

// The floor, both sides: one under it is not a finding and one at it is.
func TestTheLengthFloor(t *testing.T) {
	t.Run("under the floor is not reported", func(t *testing.T) {
		r := newRepo(t)
		r.write("base.go", "package fixture\n")
		r.commit("base")
		short := repeated('a', 99)
		r.write("base.go", short+"\n"+short+"\n")
		r.run("HEAD")
		r.expectCode(0)
		r.expectNoStdout()
		// Read all the same, so this run is not one that read nothing.
		r.expectStderrHas("1 file(s) reached the scan")
	})

	t.Run("at the floor is reported", func(t *testing.T) {
		r := newRepo(t)
		r.write("base.go", "package fixture\n")
		r.commit("base")
		exact := repeated('a', 100)
		r.write("base.go", exact+"\n"+exact+"\n")
		r.run("HEAD")
		r.expectCode(1)
		r.expectStdoutHas("100 chars")
	})

	t.Run("the floor is configurable", func(t *testing.T) {
		r := newRepo(t)
		r.write("base.go", "package fixture\n")
		r.commit("base")
		short := repeated('a', 20)
		r.write("base.go", short+"\n"+short+"\n")
		r.runWith(Config{MinLength: 10, MaxFileBytes: defaultMaxFileBytes}, "HEAD")
		r.expectCode(1)
		r.expectStdoutHas("20 chars")
	})
}

// A literal appearing once is not a duplicate, however long.
func TestASingleOccurrenceIsNotADuplicate(t *testing.T) {
	r := newRepo(t)
	r.write("base.go", "package fixture\n")
	r.commit("base")
	r.write("base.go", repeated('a', 200)+"\n")
	r.run("HEAD")
	r.expectCode(0)
	r.expectNoStdout()
}

// --- the diff shape ------------------------------------------------------------------------------

// `diff --git` is the anchor, never `+++` alone. TWO plus signs in the source: the diff prefixes every
// added line with one, so `++ b/decoy.go` is what arrives as `+++ b/decoy.go` and could be mistaken
// for a real header. Written with three it arrives as `++++ ` and matches nothing, which is a fixture
// that exercises the anchor's absence rather than the anchor.
func TestAnAddedLineShapedLikeADiffHeaderDoesNotReassignTheFile(t *testing.T) {
	r := newRepo(t)
	r.write("real.go", "package fixture\n")
	r.commit("base")
	long := repeated('z', 120)
	r.write("real.go", "++ b/decoy.go\n"+long+"\n"+long+"\n")
	r.run("HEAD")
	r.expectCode(1)
	// The duplicate is real; what must not have happened is the decoy being read as a header, which
	// would drop every line after it out of the scan entirely.
	r.expectStdoutHas("2x")
}

// --- the untracked arm ----------------------------------------------------------------------------

func TestUntrackedFilesAreScannedOnlyWithNoRevision(t *testing.T) {
	r := newRepo(t)
	long := repeated('u', 120)
	r.write("fresh.go", long+"\n"+long+"\n")

	r.run()
	r.expectCode(1)
	r.expectStdoutHas("2x")

	// With revisions the caller named two commits, and a file in neither is not what they asked about.
	r.run("HEAD")
	r.expectCode(0)
	r.expectNoStdout()
}

// The load-bearing one. This tool echoes 60 bytes of every duplicate, so a file whose NAME marks it as
// secret-bearing is never read — and the skip is announced and counted, never silent.
// The uppercase spellings are cases in their own right, not tidiness: a name is not a keyword, and on
// a case-insensitive filesystem `.ENV` IS `.env`. The modern key types are here for the same reason —
// `id_ed25519` is what ssh-keygen writes by default, so a list stopping at `id_rsa` covers the key
// nobody generates any more and misses the one everybody has.
func TestAnUntrackedSecretNamedFileIsNeverRead(t *testing.T) {
	secret := repeated('S', 130)
	for _, name := range []string{
		".env", ".env.local", "config/.env.production", "id_rsa", "server.pem", "app.key",
		"my-credentials.txt", "secrets.yaml",
		".ENV", "Server.PEM", "ID_RSA",
		"id_ecdsa", "id_ed25519", "id_ed25519.bak",
		".netrc", ".npmrc", "AuthKey_A1B2C3D4E5.p8",
	} {
		t.Run(name+" is skipped unread", func(t *testing.T) {
			r := newRepo(t)
			r.write(name, secret+"\n"+secret+"\n")
			r.run()
			// The secret is over the floor and appears twice, so a scan that read it would print it.
			r.expectStdoutLacks(secret[:60])
			r.expectStderrHas("its name marks it as secret-bearing")
			r.expectStderrHas("1 file(s) skipped unread")
		})
	}
}

// And the control: an ordinary untracked file with the same content IS read, or the case above would
// pass against a scanner that skipped everything.
func TestAnOrdinaryUntrackedFileWithTheSameContentIsRead(t *testing.T) {
	r := newRepo(t)
	body := repeated('S', 130)
	r.write("ordinary.txt", body+"\n"+body+"\n")
	r.run()
	r.expectCode(1)
	r.expectStdoutHas("2x")
}

func TestAnUntrackedBinaryFileIsSkippedAndCounted(t *testing.T) {
	r := newRepo(t)
	long := repeated('b', 130)
	r.write("blob.bin", "\x00"+long+"\n"+long+"\n")
	r.run()
	r.expectCode(0)
	r.expectStderrHas("1 file(s) skipped unread")
}

func TestAnUntrackedFileOverTheByteCapIsSkippedAndCounted(t *testing.T) {
	r := newRepo(t)
	long := repeated('c', 130)
	r.write("big.txt", long+"\n"+long+"\n")
	r.runWith(Config{MinLength: defaultMinLength, MaxFileBytes: 32})
	r.expectCode(0)
	r.expectStderrHas("1 file(s) skipped unread")
}

// --- the report itself -----------------------------------------------------------------------------

// A suppressed duplicate is announced, never dropped, and exactly the cap is printed above the
// announcement.
func TestPastTheDisplayCap(t *testing.T) {
	r := newRepo(t)
	r.write("base.go", "package fixture\n")
	r.commit("base")
	var body strings.Builder
	for i := 0; i < maxShown+1; i++ {
		line := fmt.Sprintf("%03d", i) + repeated('q', 120)
		body.WriteString(line + "\n" + line + "\n")
	}
	r.write("base.go", body.String())
	r.run("HEAD")
	r.expectCode(1)
	r.expectStdoutHas("… and 1 further duplicate(s), not shown")
	if shown := strings.Count(r.stdout.String(), " chars): "); shown != maxShown {
		t.Errorf("printed %d duplicates above the announcement, wanted exactly the cap %d", shown, maxShown)
	}
}

// Two runs over one tree print one report, or a diff of two is unreadable and the display cap takes a
// different 200 each time.
func TestTheReportIsOrdered(t *testing.T) {
	r := newRepo(t)
	r.write("base.go", "package fixture\n")
	r.commit("base")
	var body strings.Builder
	for _, tag := range []string{"zeta", "alpha", "mid"} {
		line := tag + repeated('w', 120)
		body.WriteString(line + "\n" + line + "\n")
	}
	r.write("base.go", body.String())
	r.run("HEAD")
	first := r.stdout.String()
	r.run("HEAD")
	if second := r.stdout.String(); first != second {
		t.Errorf("two runs over one tree printed different reports:\n%s\nand\n%s", first, second)
	}
}

// A threshold that does not parse is a scan that did not run, never one against the default.
func TestAThresholdThatDoesNotParseRefuses(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"DUP_MIN_LEN", "junk"},
		{"DUP_MIN_LEN", "0"},
		{"DUP_MAX_FILE_BYTES", "big"},
	} {
		t.Run(tc.key+"="+tc.value+" is refused", func(t *testing.T) {
			_, err := ConfigFromEnv(func(key string) (string, bool) {
				if key == tc.key {
					return tc.value, true
				}
				return "", false
			})
			if err == nil {
				t.Fatalf("%s=%q was accepted, so a scan would run against a floor nobody chose", tc.key, tc.value)
			}
			if !strings.Contains(err.Error(), "did NOT run") {
				t.Errorf("the refusal does not say the scan did not run: %v", err)
			}
		})
	}
	cfg, err := ConfigFromEnv(func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatalf("an empty environment was refused: %v", err)
	}
	if cfg.MinLength != 100 || cfg.MaxFileBytes != 262144 {
		t.Errorf("defaults are %+v, wanted length 100 and cap 262144", cfg)
	}
}

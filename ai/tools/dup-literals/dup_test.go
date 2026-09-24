// Cases for the repeated-literal detector. Two must not be weakened. "A path argument is refused with
// exit 2, never scanned": `git diff <path>` is legal and diffs against the index, so a path quietly
// accepted scans the wrong change set and exits 0, indistinguishable from a clean tree. "An untracked
// file whose name marks it as secret-bearing is never read": this echoes 60 bytes of every duplicate,
// so the untracked arm is a route from a secret into the transcript and any PR comment drafted from
// it — two .env files sharing one API token is the ordinary case, and the token would print.
package duplicates

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"configs/ai/tools/flavorconfig"
	"configs/ai/tools/shell"
)

func TestAnUnchangedTree(t *testing.T) {
	f := newFixture(t)
	f.run("HEAD")
	f.ExpectCode(f.code, 0)
	f.ExpectNoOut()
	f.ExpectErr("0 file(s) reached the scan")
	// The denominator is what tells "nothing repeated" from "nothing was read".
	f.ExpectErr("says nothing about the change set")
}

func TestARevisionIsNotAPath(t *testing.T) {
	t.Run("a path exits 2 and is named as a path", func(t *testing.T) {
		f := newFixture(t)
		// On disk and naming no revision, which is the pair the refusal turns on: an argument that is
		// both is a legal invocation and passes.
		f.onDisk("seen.go", "x\n")
		f.run("seen.go")
		f.ExpectCode(f.code, 2)
		f.ExpectErr("is a path, not a git-diff revision")
		f.ExpectErr("the scan did NOT run")
		// The grammar goes with an argument refusal, so the caller is told what this tool does take.
		f.ExpectErr(usage)
		f.ExpectNoOut()
	})

	t.Run("an option exits 2 and is named as an option", func(t *testing.T) {
		f := newFixture(t)
		f.run("--output=/dev/null")
		f.ExpectCode(f.code, 2)
		f.ExpectErr("is an option, not a git-diff revision")
		f.ExpectErr(usage)
		f.ExpectNoOut()
	})

	// The refusal is git's, so the case arranges git's refusal and never a name it happens to reject.
	// That real git turns `no-such-rev` down is git's own behaviour, and `repo/exec_test.go` holds it
	// against a real repository. What belongs here is that this tool answers a refused diff with exit
	// 2, with git's words, and without the grammar.
	t.Run("a revision git cannot resolve exits 2 as git's rejection", func(t *testing.T) {
		f := newFixture(t)
		f.git.Fail["Patch"] = errors.New("fatal: bad revision 'no-such-rev'")
		f.run("no-such-rev")
		f.ExpectCode(f.code, 2)
		f.ExpectErr("git rejected these arguments")
		f.ExpectErr("Not a clean result")
		// A revision this repository does not carry is a sound invocation. Answering it with the grammar
		// would send the caller to fix an argument that was already the right shape.
		f.ExpectNotErr(usage)
		f.ExpectNoOut()
	})
}

func TestARepeatedTokenInsideDifferingLinesIsFound(t *testing.T) {
	f := newFixture(t)
	token := repeated('k', 130)
	f.added("base.go", "first = \""+token+"\"", "second = \""+token+"\"")
	f.run("HEAD")
	f.ExpectCode(f.code, 1)
	f.ExpectOut("2x token")
	f.ExpectOut("130 chars")
}

func TestTheLengthFloor(t *testing.T) {
	t.Run("under the floor is not reported", func(t *testing.T) {
		f := newFixture(t)
		short := repeated('a', 99)
		f.added("base.go", short, short)
		f.run("HEAD")
		f.ExpectCode(f.code, 0)
		f.ExpectNoOut()
		// Read all the same, so this run is not one that read nothing.
		f.ExpectErr("1 file(s) reached the scan")
	})

	t.Run("at the floor is reported", func(t *testing.T) {
		f := newFixture(t)
		exact := repeated('a', 100)
		f.added("base.go", exact, exact)
		f.run("HEAD")
		f.ExpectCode(f.code, 1)
		f.ExpectOut("100 chars")
	})

	t.Run("the floor is configurable", func(t *testing.T) {
		f := newFixture(t)
		short := repeated('a', 20)
		f.added("base.go", short, short)
		f.runWith(Config{MinLength: 10, MaxFileBytes: defaultMaxFileBytes}, "HEAD")
		f.ExpectCode(f.code, 1)
		f.ExpectOut("20 chars")
	})
}

func TestASingleOccurrenceIsNotADuplicate(t *testing.T) {
	f := newFixture(t)
	f.added("base.go", repeated('a', 200))
	f.run("HEAD")
	f.ExpectCode(f.code, 0)
	f.ExpectNoOut()
}

// `diff --git` is the anchor, never `+++` alone. TWO plus signs in the source: the diff prefixes every
// added line with one, so `++ b/decoy.go` is what arrives as `+++ b/decoy.go` and could be mistaken
// for a real header. Written with three it arrives as `++++ ` and matches nothing, which is a fixture
// that exercises the anchor's absence rather than the anchor.
func TestAnAddedLineShapedLikeADiffHeaderDoesNotReassignTheFile(t *testing.T) {
	f := newFixture(t)
	long := repeated('z', 120)
	f.added("real.go", "++ b/decoy.go", long, long)
	f.run("HEAD")
	f.ExpectCode(f.code, 1)
	f.ExpectOut("2x")
}

func TestUntrackedFilesAreScannedOnlyWithNoRevision(t *testing.T) {
	f := newFixture(t)
	long := repeated('u', 120)
	f.untracked("fresh.go", long+"\n"+long+"\n")

	f.run()
	f.ExpectCode(f.code, 1)
	f.ExpectOut("2x")

	f.run("HEAD")
	f.ExpectCode(f.code, 0)
	f.ExpectNoOut()
}

// This tool echoes 60 bytes of every duplicate, so a file marked by NAME as secret-bearing is never
// read. The skip is announced and counted, which keeps it visible to a reader.

// Which names are secret-bearing is `diffscan.SecretNamed`'s list, and every row of it is driven
// where it lives, in `diffscan/diffscan_test.go`. A run of this whole pipeline per name pays for the
// same answer a second time. This case holds only that the untracked arm asks that list at all.

// Two guards stand on this path, and this case cannot say which of them fired. `diffscan.Options`'
// SkipSecretNamed declines the file before it is opened, and `count` declines its lines after.
// Measured: either guard alone keeps this case green, and only both off reddens it. That is
// deliberate for a path that would otherwise put a credential in a report. Do not read a green run
// as either guard working, and keep both even where one looks redundant.

// The case this file exists for.
func TestAnUntrackedSecretNamedFileIsNeverRead(t *testing.T) {
	secret := repeated('S', 130)
	f := newFixture(t)
	f.untracked(".env", secret+"\n"+secret+"\n")
	f.run()
	// The secret is over the floor and appears twice, so it is what this tool echoes from any file it reads.
	f.ExpectNotOut(secret[:60])
	f.ExpectErr("its name marks it as secret-bearing")
	f.ExpectErr("1 file(s) skipped unread")
}

// The skip announcement names a file somebody else put in the tree, and it is the one path in this
// scan that reaches a terminal without a report's own sanitising in front of it. `.env*` matches
// whatever follows the prefix, so the name that triggers the announcement is the attacker's to choose.
func TestTheSkipAnnouncementIsSanitised(t *testing.T) {
	secret := repeated('S', 130)

	// Raw, the escape clears the reader's screen or rewrites the lines above it, and a reader who
	// cannot trust the report stops reading it — including on the run that skipped a real secret.
	// ESC alone: a lone 0x9b, the 8-bit CSI, is not valid UTF-8 and APFS refuses to name a file with
	// it, so the fixture cannot carry that byte. Oneline maps the whole 0x80-0x9f band the same way.
	t.Run("an escape in the name does not reach stderr", func(t *testing.T) {
		f := newFixture(t)
		f.untracked(".env\x1b[2J", secret+"\n"+secret+"\n")
		f.run()
		f.ExpectErr("its name marks it as secret-bearing")
		if strings.Contains(f.Err.String(), "\x1b") {
			t.Errorf("the announcement carries a raw escape: %q", f.Err.String())
		}
	})

	// Cut without the marker, an overlong name is a shorter different name, and the reader looking for
	// the file that was skipped does not find it.
	t.Run("an overlong name is cut and says so", func(t *testing.T) {
		f := newFixture(t)
		name := strings.Repeat("d", 200) + "/.env"
		f.untracked(name, secret+"\n"+secret+"\n")
		f.run()
		f.ExpectErr("its name marks it as secret-bearing")
		f.ExpectErr(shell.CutMarker)
		if strings.Contains(f.Err.String(), name) {
			t.Errorf("the whole %d-byte name reached stderr uncut: %q", len(name), f.Err.String())
		}
	})
}

// The same route through the other arm. What makes the guard necessary is that this tool echoes 60
// bytes of every duplicate, which is true of a TRACKED file's added lines as readily as an untracked
// file's — and the untracked guard is the only one that existed. Two tracked `.env` files each
// gaining the same `TOKEN=` line is the two-files-one-token case, and it printed the token.
func TestATrackedSecretNamedFileIsNeverScannedEither(t *testing.T) {
	secret := repeated('S', 130)
	f := newFixture(t)
	f.added(".env", "TOKEN="+secret)
	f.added(".env.staging", "TOKEN="+secret)

	f.run("HEAD")
	f.ExpectCode(f.code, 0)
	f.ExpectNotOut(secret[:60])
	f.ExpectErr("its name marks it as secret-bearing")
	f.ExpectErr("2 file(s) skipped unread")
}

// The control the case above needs: the same duplicate in tracked files NOT named as secret-bearing
// is still reported. Without it the guard passes against a scanner that stopped reading the diff.
func TestATrackedOrdinaryFileWithTheSameDuplicateIsStillReported(t *testing.T) {
	secret := repeated('S', 130)
	f := newFixture(t)
	f.added("one.go", "TOKEN="+secret)
	f.added("two.go", "TOKEN="+secret)

	f.run("HEAD")
	f.ExpectCode(f.code, 1)
	f.ExpectOut("2x token")
}

// And the control: an ordinary untracked file with the same content IS read, or the case above would
// pass against a scanner that skipped everything.
func TestAnOrdinaryUntrackedFileWithTheSameContentIsRead(t *testing.T) {
	f := newFixture(t)
	body := repeated('S', 130)
	f.untracked("ordinary.txt", body+"\n"+body+"\n")
	f.run()
	f.ExpectCode(f.code, 1)
	f.ExpectOut("2x")
}

// A symlink is declined, not followed. Stat answers about the target, so without Lstat an
// innocuously named link reads as a regular file and whatever it points at is scanned and echoed
// under the link's name — the secret check above only ever sees the LINK's name, and the target can
// sit outside the repository entirely. A real link and a real target, because the decision is
// `os.Lstat`'s and the listing cannot carry it.
func TestAnUntrackedSymlinkIsSkippedAndCounted(t *testing.T) {
	f := newFixture(t)
	body := repeated('L', 130)
	outside := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(outside, []byte(body+"\n"+body+"\n"), 0o600); err != nil {
		t.Fatalf("could not write the link target, so nothing was tested: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(f.dir, "notes.txt")); err != nil {
		t.Skipf("this filesystem does not do symlinks, so this case says nothing: %v", err)
	}
	f.git.AddUntracked("notes.txt")
	f.run()
	f.ExpectCode(f.code, 0)
	// The body is over the floor and appears twice, so a scan that followed the link would print it.
	f.ExpectNotOut(body[:60])
	f.ExpectErr("1 file(s) skipped unread")
}

func TestAnUntrackedBinaryFileIsSkippedAndCounted(t *testing.T) {
	f := newFixture(t)
	long := repeated('b', 130)
	f.untracked("blob.bin", "\x00"+long+"\n"+long+"\n")
	f.run()
	f.ExpectCode(f.code, 0)
	f.ExpectErr("1 file(s) skipped unread")
}

func TestAnUntrackedFileOverTheByteCapIsSkippedAndCounted(t *testing.T) {
	f := newFixture(t)
	long := repeated('c', 130)
	f.untracked("big.txt", long+"\n"+long+"\n")
	f.runWith(Config{MinLength: defaultMinLength, MaxFileBytes: 32})
	f.ExpectCode(f.code, 0)
	f.ExpectErr("1 file(s) skipped unread")
}

func TestPastTheDisplayCap(t *testing.T) {
	const wantCap = 200
	if maxShown != wantCap {
		t.Fatalf("the display cap is %d, and this case pins %d. Changing the cap is a change to what "+
			"the report promises, so update this number deliberately rather than reading it from the "+
			"constant — a case that reads it can no longer tell you the cap moved.", maxShown, wantCap)
	}

	f := newFixture(t)
	var lines []string
	for i := 0; i < wantCap+1; i++ {
		line := fmt.Sprintf("%03d", i) + repeated('q', 120)
		lines = append(lines, line, line)
	}
	f.added("base.go", lines...)
	f.run("HEAD")
	f.ExpectCode(f.code, 1)
	f.ExpectOut("… and 1 further duplicate(s), not shown")
	if shown := strings.Count(f.Out.String(), " chars): "); shown != wantCap {
		t.Errorf("printed %d duplicates above the announcement, wanted exactly the cap %d", shown, wantCap)
	}
}

func TestTheReportIsOrdered(t *testing.T) {
	f := newFixture(t)
	var lines []string
	for _, tag := range []string{"zeta", "alpha", "mid"} {
		line := tag + repeated('w', 120)
		lines = append(lines, line, line)
	}
	f.added("base.go", lines...)
	f.run("HEAD")
	first := f.Out.String()
	f.run("HEAD")
	if second := f.Out.String(); first != second {
		t.Errorf("two runs over one tree printed different reports:\n%s\nand\n%s", first, second)
	}
}

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

// This case reads the file through `flavorconfig`. The shipped numbers equal the constants in code, and
// a case built on ConfigFromEnv passes whether or not it reads the file. No other place in the
// repository opens this file, and a typo in it shows up here first.
func TestTheShippedConfigParsesAndMatchesTheBuiltInDefaults(t *testing.T) {
	flavor, err := filepath.Abs("../../kk-flavor")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	if err := os.Symlink(flavor, filepath.Join(home, ".kk-flavor")); err != nil {
		t.Fatal(err)
	}
	settings, err := flavorconfig.Read(flavorconfig.Path(home, configName), configKeys)
	if err != nil {
		t.Fatalf("the shipped dup-literals.conf does not parse: %v", err)
	}
	if len(settings) != len(configKeys) {
		t.Fatalf("the shipped dup-literals.conf sets %v, and this tool reads %v", settings, configKeys)
	}
	if settings["min-length"] != fmt.Sprint(defaultMinLength) || settings["max-file-bytes"] != fmt.Sprint(defaultMaxFileBytes) {
		t.Fatalf("the shipped config holds %v and the built-in defaults are %d/%d — a run that finds no configs would scan against different thresholds",
			settings, defaultMinLength, defaultMaxFileBytes)
	}
}

func TestTheEnvironmentWinsOverTheShippedConfigAndABrokenOneRefuses(t *testing.T) {
	home := t.TempDir()
	configs := filepath.Join(home, ".kk-flavor", "configs")
	if err := os.MkdirAll(configs, 0o700); err != nil {
		t.Fatal(err)
	}
	lookup := func(extra map[string]string) func(string) (string, bool) {
		return func(asked string) (string, bool) {
			if asked == "HOME" {
				return home, true
			}
			value, ok := extra[asked]
			return value, ok
		}
	}
	path := filepath.Join(configs, "dup-literals.conf")
	if err := os.WriteFile(path, []byte("min-length 40\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := ConfigFromEnv(lookup(nil))
	if err != nil || cfg.MinLength != 40 {
		t.Fatalf("got %d %v, want the shipped file's 40", cfg.MinLength, err)
	}
	if cfg.MaxFileBytes != defaultMaxFileBytes {
		t.Fatalf("a key the file leaves out took %d rather than the built-in default", cfg.MaxFileBytes)
	}
	cfg, err = ConfigFromEnv(lookup(map[string]string{"DUP_MIN_LEN": "70"}))
	if err != nil || cfg.MinLength != 70 {
		t.Fatalf("got %d %v, want the environment's 70 over the file's 40", cfg.MinLength, err)
	}

	if err := os.WriteFile(path, []byte("minlength 40\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ConfigFromEnv(lookup(nil)); err == nil {
		t.Fatal("a shipped config the tool cannot read was replaced by the built-in defaults in silence")
	}

	// A key `flavorconfig` accepts and this tool then rejects. The refusal names the file, since that
	// is what the human has to edit. The environment variable was never set.
	if err := os.WriteFile(path, []byte("min-length nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = ConfigFromEnv(lookup(nil))
	if err == nil {
		t.Fatal("a non-numeric length was accepted from the shipped config")
	}
	if !strings.Contains(err.Error(), path) || strings.Contains(err.Error(), "DUP_MIN_LEN is") {
		t.Fatalf("the refusal blames the environment for a value the file set: %v", err)
	}
}

// Cases for the repeated-literal detector. Two must not be weakened. "A path argument is refused with
// exit 2, never scanned": `git diff <path>` is legal and diffs against the index, so a path quietly
// accepted scans the wrong change set and exits 0, indistinguishable from a clean tree. "An untracked
// file whose name marks it as secret-bearing is never read": this echoes 60 bytes of every duplicate,
// so the untracked arm is a route from a secret into the transcript and any PR comment drafted from
// it — two .env files sharing one API token is the ordinary case, and the token would print.
package duplicates

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kk-flavor/tools/shell"
)

func TestAnUnchangedTree(t *testing.T) {
	r := newRepo(t)
	r.run("HEAD")
	r.expectCode(0)
	r.expectNoStdout()
	r.expectStderrHas("0 file(s) reached the scan")
	// The denominator is what tells "nothing repeated" from "nothing was read".
	r.expectStderrHas("says nothing about the change set")
}

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

func TestASingleOccurrenceIsNotADuplicate(t *testing.T) {
	r := newRepo(t)
	r.write("base.go", "package fixture\n")
	r.commit("base")
	r.write("base.go", repeated('a', 200)+"\n")
	r.run("HEAD")
	r.expectCode(0)
	r.expectNoStdout()
}

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
	r.expectStdoutHas("2x")
}

func TestUntrackedFilesAreScannedOnlyWithNoRevision(t *testing.T) {
	r := newRepo(t)
	long := repeated('u', 120)
	r.write("fresh.go", long+"\n"+long+"\n")

	r.run()
	r.expectCode(1)
	r.expectStdoutHas("2x")

	r.run("HEAD")
	r.expectCode(0)
	r.expectNoStdout()
}

// The load-bearing one. This tool echoes 60 bytes of every duplicate, so a file whose NAME marks it
// as secret-bearing is never read, and the skip is announced and counted rather than silent. The
// uppercase and modern-key rows are cases, not tidiness: on a case-insensitive filesystem `.ENV` IS
// `.env`, and a list stopping at `id_rsa` misses the `id_ed25519` ssh-keygen writes by default.
func TestAnUntrackedSecretNamedFileIsNeverRead(t *testing.T) {
	secret := repeated('S', 130)
	for _, name := range []string{
		".env", ".env.local", "config/.env.production", "id_rsa", "server.pem", "app.key",
		"my-credentials.txt", "secrets.yaml",
		".ENV", "Server.PEM", "ID_RSA",
		"id_ecdsa", "id_ed25519", "id_ed25519.bak",
		".netrc", ".npmrc", "AuthKey_A1B2C3D4E5.p8",
		// The suffix spelling of the same convention. Every name above it is a prefix form, so
		// without these the table proves only that `.env*` matches `.env*`.
		"production.env", "staging.env", "env/prod.env", "PRODUCTION.ENV",
		".pgpass", ".htpasswd", ".pypirc", ".dockercfg", "deploy.ppk", "api.token",
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
		r := newRepo(t)
		r.write(".env\x1b[2J", secret+"\n"+secret+"\n")
		r.run()
		r.expectStderrHas("its name marks it as secret-bearing")
		if strings.Contains(r.stderr.String(), "\x1b") {
			t.Errorf("the announcement carries a raw escape: %q", r.stderr.String())
		}
	})

	// Cut without the marker, an overlong name is a shorter different name, and the reader looking for
	// the file that was skipped does not find it.
	t.Run("an overlong name is cut and says so", func(t *testing.T) {
		r := newRepo(t)
		name := strings.Repeat("d", 200) + "/.env"
		r.write(name, secret+"\n"+secret+"\n")
		r.run()
		r.expectStderrHas("its name marks it as secret-bearing")
		r.expectStderrHas(shell.CutMarker)
		if strings.Contains(r.stderr.String(), name) {
			t.Errorf("the whole %d-byte name reached stderr uncut: %q", len(name), r.stderr.String())
		}
	})
}

// The same route through the other arm. What makes the guard necessary is that this tool echoes 60
// bytes of every duplicate, which is true of a TRACKED file's added lines as readily as an untracked
// file's — and the untracked guard is the only one that existed. Two tracked `.env` files each
// gaining the same `TOKEN=` line is the two-files-one-token case, and it printed the token.
func TestATrackedSecretNamedFileIsNeverScannedEither(t *testing.T) {
	secret := repeated('S', 130)
	r := newRepo(t)
	r.write(".env", "placeholder\n")
	r.write(".env.staging", "placeholder\n")
	r.commit("base")
	r.write(".env", "placeholder\nTOKEN="+secret+"\n")
	r.write(".env.staging", "placeholder\nTOKEN="+secret+"\n")

	r.run("HEAD")
	r.expectCode(0)
	r.expectStdoutLacks(secret[:60])
	r.expectStderrHas("its name marks it as secret-bearing")
	r.expectStderrHas("2 file(s) skipped unread")
}

// The control the case above needs: the same duplicate in tracked files NOT named as secret-bearing
// is still reported. Without it the guard passes against a scanner that stopped reading the diff.
func TestATrackedOrdinaryFileWithTheSameDuplicateIsStillReported(t *testing.T) {
	secret := repeated('S', 130)
	r := newRepo(t)
	r.write("one.go", "placeholder\n")
	r.write("two.go", "placeholder\n")
	r.commit("base")
	r.write("one.go", "placeholder\nTOKEN="+secret+"\n")
	r.write("two.go", "placeholder\nTOKEN="+secret+"\n")

	r.run("HEAD")
	r.expectCode(1)
	r.expectStdoutHas("2x token")
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

// A symlink is declined, not followed. Stat answers about the target, so without Lstat an
// innocuously named link reads as a regular file and whatever it points at is scanned and echoed
// under the link's name — the secret check above only ever sees the LINK's name, and the target can
// sit outside the repository entirely.
func TestAnUntrackedSymlinkIsSkippedAndCounted(t *testing.T) {
	r := newRepo(t)
	body := repeated('L', 130)
	outside := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(outside, []byte(body+"\n"+body+"\n"), 0o600); err != nil {
		t.Fatalf("could not write the link target, so nothing was tested: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(r.dir, "notes.txt")); err != nil {
		t.Skipf("this filesystem does not do symlinks, so this case says nothing: %v", err)
	}
	r.run()
	r.expectCode(0)
	// The body is over the floor and appears twice, so a scan that followed the link would print it.
	r.expectStdoutLacks(body[:60])
	r.expectStderrHas("1 file(s) skipped unread")
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

func TestPastTheDisplayCap(t *testing.T) {
	const wantCap = 200
	if maxShown != wantCap {
		t.Fatalf("the display cap is %d, and this case pins %d. Changing the cap is a change to what "+
			"the report promises, so update this number deliberately rather than reading it from the "+
			"constant — a case that reads it can no longer tell you the cap moved.", maxShown, wantCap)
	}

	r := newRepo(t)
	r.write("base.go", "package fixture\n")
	r.commit("base")
	var body strings.Builder
	for i := 0; i < wantCap+1; i++ {
		line := fmt.Sprintf("%03d", i) + repeated('q', 120)
		body.WriteString(line + "\n" + line + "\n")
	}
	r.write("base.go", body.String())
	r.run("HEAD")
	r.expectCode(1)
	r.expectStdoutHas("… and 1 further duplicate(s), not shown")
	if shown := strings.Count(r.stdout.String(), " chars): "); shown != wantCap {
		t.Errorf("printed %d duplicates above the announcement, wanted exactly the cap %d", shown, wantCap)
	}
}

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

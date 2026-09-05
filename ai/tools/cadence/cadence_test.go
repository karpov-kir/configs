// Cases for the offer cadence. The one that must not be weakened is "a stamp later than today is
// undetermined, not a 'not due'": exits 1 and 2 both end in "no offer made", so a wrong one there
// suppresses a periodic pass for as long as the bad stamp sits in the file and looks identical from
// the outside. The undetermined cases therefore assert the code AND that no verdict line came back
// with it.
//
// The colon in the "not due:" those cases look for is load-bearing at both ends: every "not due:"
// message contains "due:" as a substring, so the due cases cannot assert "due:" and must assert the
// absence of the longer one instead — and the undetermined message deliberately ends "this is not a
// 'not due'", so searching for the bare phrase finds the disclaimer and passes a broken run.
//
// The audit record is resolved through git, so all but the usage cases run inside a throwaway
// repository built by copying one seed rather than by running `git init` per case: the seed costs six
// processes and a copy costs none.
//
// Dates come from Go's own calendar arithmetic against a FIXED clock, never from re-deriving the
// package's day count — a fixture that reimplemented it would agree with itself rather than with the
// code. The clock is fixed so that "the seventh day" is a property of the code and not of the hour
// the suite happened to run at.
package cadence

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 9, 3, 14, 30, 0, 0, time.UTC)

func clock() time.Time { return fixedNow }

func today() string { return fixedNow.Format(dateLayout) }

// A negative offset is in the future.
func daysAgo(n int) string { return fixedNow.AddDate(0, 0, -n).Format(dateLayout) }

var seedRepo string

func TestMain(m *testing.M) {
	// The developer's own git config must not reach these fixtures: a global core.hooksPath or a
	// template dir would change what `git init` produces underneath every case.
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	base, err := os.MkdirTemp("", "cadence-seed")
	if err != nil {
		panic("cadence tests: no temp dir, so nothing was tested: " + err.Error())
	}
	defer os.RemoveAll(base)
	os.Setenv("HOME", filepath.Join(base, "home"))
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "config"))
	os.MkdirAll(os.Getenv("HOME"), 0o755)

	seedRepo = filepath.Join(base, "seed")
	if err := buildSeed(seedRepo); err != nil {
		// Panic rather than a silent skip: every fixture below is a copy of this, so a suite that
		// carried on would report passes over repositories that do not exist.
		panic("cadence tests: could not build the seed repository, so nothing was tested: " + err.Error())
	}
	os.Exit(m.Run())
}

func buildSeed(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	steps := [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"config", "commit.gpgsign", "false"},
	}
	for _, args := range steps {
		if err := runGit(dir, args...); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		return err
	}
	if err := runGit(dir, "add", "seed.txt"); err != nil {
		return err
	}
	if err := runGit(dir, "commit", "-qm", "base"); err != nil {
		return err
	}
	// Checked by its effect: with no commit there is no HEAD, and `git worktree add` needs one.
	return runGit(dir, "rev-parse", "--verify", "-q", "HEAD")
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}

// --- fixtures --------------------------------------------------------------------------------

type fixture struct {
	repo   string // the working tree the caller stands in
	state  string // where the record belongs
	stdout bytes.Buffer
	stderr bytes.Buffer
	code   int
}

// newRepo copies the seed. No git process runs here, which is the whole reason the seed exists.
func newRepo(t *testing.T) *fixture {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := copyTree(seedRepo, repo); err != nil {
		t.Fatalf("could not build a fixture repo: %v — stopping, since the case reads one", err)
	}
	return &fixture{repo: repo, state: filepath.Join(repo, ".git", recordName)}
}

// newNeutral is a directory inside no repository, for the cases that must reach none.
func newNeutral(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	// The precondition the "outside any repository" cases rest on, asserted rather than assumed:
	// inside a repository that case would pass for the wrong reason.
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	if err := cmd.Run(); err == nil {
		t.Fatalf("the fixture root %s resolves to a repository, so this case would read the wrong one", dir)
	}
	return &fixture{repo: dir}
}

func (f *fixture) run(args ...string) {
	f.stdout.Reset()
	f.stderr.Reset()
	f.code = Run("cadence.sh", args, f.repo, clock, &f.stdout, &f.stderr)
}

// runFrom drives the same invocation from a directory below the working tree.
func (f *fixture) runFrom(dir string, args ...string) {
	f.stdout.Reset()
	f.stderr.Reset()
	f.code = Run("cadence.sh", args, dir, clock, &f.stdout, &f.stderr)
}

// both channels, for the cases that only care that a phrase appeared somewhere.
func (f *fixture) out() string { return f.stdout.String() + f.stderr.String() }

// record writes a stamp, then reads it back: a write that produced nothing would leave the code on
// its never-offered path, which is a `due`, and the due cases would pass without the record they are
// named for.
func (f *fixture) record(t *testing.T, stamp string) {
	t.Helper()
	if err := os.WriteFile(f.state, []byte(stamp+"\n"), 0o644); err != nil {
		t.Fatalf("could not record %q in %s: %v", stamp, f.state, err)
	}
	got, err := os.ReadFile(f.state)
	if err != nil || strings.TrimRight(string(got), "\n") != stamp {
		t.Fatalf("record %s did not read back as %q — stopping, since the case reads it", f.state, stamp)
	}
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
		switch {
		case info.IsDir():
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		case info.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		default:
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, body, info.Mode().Perm())
		}
	})
}

// --- assertions ------------------------------------------------------------------------------

func (f *fixture) expectCode(t *testing.T, want int) {
	t.Helper()
	if f.code != want {
		t.Errorf("exit %d, wanted %d — output: %s", f.code, want, f.out())
	}
}

func (f *fixture) expectOut(t *testing.T, want string) {
	t.Helper()
	if !strings.Contains(f.out(), want) {
		t.Errorf("wanted %q in: %s", want, f.out())
	}
}

func (f *fixture) expectNotOut(t *testing.T, unwanted string) {
	t.Helper()
	if strings.Contains(f.out(), unwanted) {
		t.Errorf("%q appears in: %s", unwanted, f.out())
	}
}

// Exit 2 means nothing was determined, so anything left on stdout is what a caller capturing the
// verdict reads as one.
func (f *fixture) expectNoStdout(t *testing.T) {
	t.Helper()
	if f.stdout.Len() != 0 {
		t.Errorf("expected nothing on stdout, got: %s", f.stdout.String())
	}
}

// --- the four ways it cannot run ---------------------------------------------------------------

// All four exit 2, and 2 is "undetermined", never "not due". A caller that reached the usage path and
// read it as a verdict would skip the pass it was asking about.
func TestUsage(t *testing.T) {
	t.Run("no arguments exit 2", func(t *testing.T) {
		f := newNeutral(t)
		f.run()
		f.expectCode(t, 2)
		f.expectOut(t, "usage: cadence.sh audit {due|asked}")
		// The grammar alone presents `due` and `asked` as two spellings of one query, and a caller
		// probing it rewrites the record it was asking about. The warning is the only thing at the
		// point of the mistake that says so, so it is part of the contract rather than decoration.
		f.expectOut(t, "'asked' OVERWRITES it with today's date")
	})

	t.Run("an unknown topic exits 2 and prints the usage", func(t *testing.T) {
		f := newNeutral(t)
		f.run("bogus", "due")
		f.expectCode(t, 2)
		f.expectOut(t, "usage:")
	})

	// The action is dispatched behind the topic, so these two need the repository the record is
	// resolved through: run from anywhere else, the missing repository answers first and the usage
	// never prints.
	t.Run("a topic with no action exits 2 and prints the usage", func(t *testing.T) {
		f := newRepo(t)
		f.run("audit")
		f.expectCode(t, 2)
		f.expectOut(t, "usage:")
	})

	t.Run("an unknown action exits 2", func(t *testing.T) {
		f := newRepo(t)
		f.run("audit", "bogus")
		f.expectCode(t, 2)
	})

	t.Run("the usage leaves nothing on stdout for a caller to read as a verdict", func(t *testing.T) {
		f := newNeutral(t)
		f.run()
		f.expectNoStdout(t)
	})
}

// --- the repository the record belongs to -------------------------------------------------------

func TestOutsideAnyRepository(t *testing.T) {
	f := newNeutral(t)
	f.run("audit", "due")
	f.expectCode(t, 2)
	f.expectOut(t, "not inside a git repository")
	f.expectNotOut(t, "not due:")
	f.expectNoStdout(t)
}

// --- the record's own three answers -------------------------------------------------------------

// Never offered is due: the first run in a fresh checkout.
func TestNeverOfferedIsDue(t *testing.T) {
	f := newRepo(t)
	f.run("audit", "due")
	f.expectCode(t, 0)
	f.expectOut(t, "no audit has ever been offered")
	f.expectNotOut(t, "not due:")
}

// Recording, and the record's effect. The write is asserted through the next `due` as well as through
// the file, because a record written somewhere the reader does not look would satisfy neither.
func TestRecordingAnOffer(t *testing.T) {
	f := newRepo(t)
	f.run("audit", "asked")
	f.expectCode(t, 0)
	f.expectOut(t, "recorded the audit offer on "+today())

	body, err := os.ReadFile(filepath.Join(f.repo, ".git", recordName))
	if err != nil {
		t.Fatalf("the record did not land in the repository's git dir: %v", err)
	}
	if got := strings.TrimRight(string(body), "\n"); got != today() {
		t.Errorf("the record holds %q, wanted %q", got, today())
	}
	// `report.sh discard` wipes a throwaway .idsd/, so a cadence kept there could never come due.
	if _, err := os.Stat(filepath.Join(f.repo, ".idsd")); !os.IsNotExist(err) {
		t.Errorf("something was written under .idsd, where a discard would wipe it")
	}

	f.run("audit", "due")
	f.expectCode(t, 1)
	f.expectOut(t, "not due:")
	f.expectOut(t, "0 days ago")
}

// The interval, both sides of it.
func TestTheInterval(t *testing.T) {
	t.Run("six days after the last offer is not due", func(t *testing.T) {
		f := newRepo(t)
		f.record(t, daysAgo(6))
		f.run("audit", "due")
		f.expectCode(t, 1)
		// The elapsed count is the package's arithmetic, not this suite's.
		f.expectOut(t, "6 days ago")
		f.expectOut(t, "interval 7 days")
	})

	t.Run("the seventh day is due", func(t *testing.T) {
		f := newRepo(t)
		f.record(t, daysAgo(7))
		f.run("audit", "due")
		f.expectCode(t, 0)
		f.expectOut(t, "7 days ago")
		f.expectNotOut(t, "not due:")
	})

	t.Run("past the interval is due", func(t *testing.T) {
		f := newRepo(t)
		f.record(t, daysAgo(8))
		f.run("audit", "due")
		f.expectCode(t, 0)
		f.expectNotOut(t, "not due:")
	})
}

// A record that grew a second line must still resolve rather than fall through to undetermined.
func TestATrailingLineStillResolves(t *testing.T) {
	t.Run("a second line is ignored", func(t *testing.T) {
		f := newRepo(t)
		if err := os.WriteFile(f.state, []byte(daysAgo(30)+"\nleftover second line\n"), 0o644); err != nil {
			t.Fatalf("could not write the fixture record: %v", err)
		}
		f.run("audit", "due")
		f.expectCode(t, 0)
		f.expectOut(t, "30 days ago")
	})

	// A record written where lines end CRLF — a Windows editor, a checkout with autocrlf. The date is
	// the line's content and the \r is its ending, so a \r carried into the stamp makes an eleven-byte
	// string the shape check refuses, and the offer is then undetermined forever.
	t.Run("a CRLF line ending is not part of the stamp", func(t *testing.T) {
		f := newRepo(t)
		if err := os.WriteFile(f.state, []byte(daysAgo(30)+"\r\n"), 0o644); err != nil {
			t.Fatalf("could not write the fixture record: %v", err)
		}
		f.run("audit", "due")
		f.expectCode(t, 0)
		f.expectOut(t, "30 days ago")
		f.expectNotOut(t, "which is no YYYY-MM-DD date")
	})
}

// --- everything that is undetermined, and never a 'not due' -------------------------------------

// A stamp later than today: a clock change, a bad edit, a merge from a machine ahead. Reading it as a
// small negative elapsed would print "not due" and hold the offer off indefinitely.
func TestAFutureStampIsUndetermined(t *testing.T) {
	f := newRepo(t)
	f.record(t, daysAgo(-1))
	f.run("audit", "due")
	f.expectCode(t, 2)
	f.expectOut(t, "later than today")
	f.expectNotOut(t, "not due:")
	// The message says outright which of the two it is.
	f.expectOut(t, "this is not a 'not due'")
	f.expectNoStdout(t)
}

func TestARecordThatIsNoDate(t *testing.T) {
	// Each of these reaches the refusal through a different guard, and `why` names it in the subtest,
	// so a red case says which one it was the only case for.
	cases := []struct {
		name  string
		stamp string
		why   string
	}{
		{"a record holding a non-date", "not-a-date", "the shape check"},
		{"an empty record", "", "the shape check, on a record that exists and says nothing"},
		// Shaped like a date and impossible. The shape check alone accepts it, so the calendar is the
		// only thing between it and a day number computed from a month that does not exist.
		{"a month out of range", "2026-13-01", "the calendar's month range"},
		// The same defect in the day rather than the month, dated to the past so that reading it as
		// arithmetic lands BEFORE today. A month of 13 rolls into the following January, which the
		// future-stamp guard refuses on its own — so that case cannot show this check firing and this
		// one can.
		{"a day out of range", "2025-01-32", "the calendar's day range"},
		// Right fields, wrong widths. Everything downstream of the shape check reads this as the first
		// of January: a one-character month field parses as 1 and the truncated day as 5, both inside
		// the ranges the calendar checks. The shape check is the only thing between it and a day
		// number computed from a string that is not the date it looks like.
		{"a date with unpadded fields", "2025-1-15", "the shape check's fixed widths"},
		// Longer than a date, and the only case reaching the length guard from ABOVE — every other stamp
		// here is ten bytes or fewer. The guard is not redundant beside the per-byte loop under it: that
		// loop indexes the LAYOUT at the input's own offsets, so without the length test an eleven-byte
		// record walks off the end of `2006-01-02` and the tool panics where it was meant to refuse.
		{"a stamp longer than a date", "2026-01-021", "the shape check's length, before any byte is indexed"},
		// A separator where a date has one and this record does not. Nothing else here puts a wrong byte
		// at position 4.
		{"a wrong separator between the fields", "2026:01-02", "the shape check's separators"},
	}
	for _, tc := range cases {
		t.Run(tc.name+" is undetermined, by "+tc.why, func(t *testing.T) {
			f := newRepo(t)
			f.record(t, tc.stamp)
			f.run("audit", "due")
			f.expectCode(t, 2)
			f.expectOut(t, "which is no YYYY-MM-DD date")
			f.expectNotOut(t, "not due:")
			f.expectNoStdout(t)
		})
	}
}

// A record that exists and cannot be read. A directory in its place is the portable way to reach that
// branch; the alternative, an unreadable file, is readable anyway when the suite runs as root.
func TestARecordThatCannotBeRead(t *testing.T) {
	f := newRepo(t)
	if err := os.Mkdir(f.state, 0o755); err != nil {
		t.Fatalf("could not put a directory at the record's path: %v", err)
	}
	f.run("audit", "due")
	f.expectCode(t, 2)
	f.expectOut(t, "could not be read")
	f.expectNotOut(t, "not due:")

	// The write failing is the other half; writeRefused carries why the caller has to be told.
	f.run("audit", "asked")
	f.expectCode(t, 2)
	f.expectOut(t, "was NOT recorded")
	f.expectOut(t, "the next run will offer again")
}

// --- which repository the record belongs to -----------------------------------------------------

// Run from a subdirectory, which is where an unabsolutised `--git-common-dir` invents a .git of its
// own. recordPath carries why.
func TestRecordingFromASubdirectory(t *testing.T) {
	f := newRepo(t)
	deep := filepath.Join(f.repo, "deep", "deeper")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("could not build the subdirectory: %v", err)
	}

	f.runFrom(deep, "audit", "asked")
	f.expectCode(t, 0)
	if _, err := os.Stat(filepath.Join(f.repo, ".git", recordName)); err != nil {
		t.Errorf("the record did not land in the repository's git dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(deep, ".git")); !os.IsNotExist(err) {
		t.Errorf("a git dir was invented beside the caller at %s", deep)
	}

	f.runFrom(deep, "audit", "due")
	f.expectCode(t, 1)
}

// A linked worktree shares the repository, so it shares the record. `--git-path` would answer this
// worktree's own git dir, the date written from the main tree would be invisible here, and the offer
// would repeat in every worktree.
func TestALinkedWorktreeSeesTheMainTreesRecord(t *testing.T) {
	f := newRepo(t)
	linked := filepath.Join(t.TempDir(), "linked")
	if err := runGit(f.repo, "worktree", "add", linked, "-b", "other"); err != nil {
		t.Fatalf("git worktree add failed, so the case could not run: %v", err)
	}

	f.run("audit", "asked")
	f.expectCode(t, 0)

	f.runFrom(linked, "audit", "due")
	f.expectCode(t, 1)
	f.expectOut(t, "not due:")
}

// The record is per repository, not per machine. One shared date would silence the audit everywhere
// after the first offer anywhere.
func TestASecondRepositoryHasItsOwnRecord(t *testing.T) {
	first := newRepo(t)
	first.run("audit", "asked")
	first.expectCode(t, 0)

	second := newRepo(t)
	second.run("audit", "due")
	second.expectCode(t, 0)
	second.expectOut(t, "no audit has ever been offered")
}

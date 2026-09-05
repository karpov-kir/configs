// Offer cadence — idsd-ship offers a periodic pass at most once per interval; this package owns the
// interval and where its date is kept.
//
//	usage: cadence.sh audit due     0 = offer one, 1 = not yet, 2 = undetermined (never "not due")
//	       cadence.sh audit asked   record that the offer was made today, whatever the human answered
//
// The audit date goes under `.git/`, never in `.idsd/` — `report.sh discard` wipes a throwaway
// `.idsd/`, and a cadence the ship itself deletes can never come due.
//
// Every message names the caller by the `self` Run is given rather than by a constant, because the
// stub execs this binary with `-a "$0"`. It is a parameter and not a read of os.Args[0] so the suite
// drives the real messages: under `go test` that global holds the test binary's own name, and every
// usage assertion would be about that.
package cadence

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const intervalDays = 7

// A second topic would be a second record name.
const auditTopic = "audit"

const recordName = "idsd-audit-offer"

const dateLayout = "2006-01-02"

// Exit codes, on `~/.kk-flavor/scripts/score.sh`'s vocabulary. 1 is a verdict — the pass is not due
// yet — and 2 is the absence of one. Nothing here ever returns 1 for a question it could not answer:
// exits 1 and 2 both end in "no offer made", so a caller that cannot tell them apart suppresses the
// pass for as long as the bad record sits there, and it looks identical from the outside.
const (
	exitDue          = 0
	exitNotDue       = 1
	exitUndetermined = 2
)

// Clock is the seam the boundary cases drive. Production passes time.Now; a test passes a fixed
// instant so "the seventh day" is a property of the code rather than of when the suite ran.
type Clock func() time.Time

// Run executes one invocation and returns its exit code. `cwd` is the directory the caller stood in,
// which is what the repository is resolved from — passed rather than read from the process so the
// suite can drive many fixtures without chdir'ing a shared process.
func Run(self string, args []string, cwd string, now Clock, stdout, stderr io.Writer) int {
	topic := ""
	if len(args) > 0 {
		topic = args[0]
	}
	action := ""
	if len(args) > 1 {
		action = args[1]
	}

	// Dispatched on the topic first, so an unknown one is refused before anything resolves a
	// repository.
	if topic != auditTopic {
		return usage(self, stderr)
	}

	// The record is per *repository*, so it hangs off the shared git dir rather than the per-worktree
	// one: a date written from the main tree has to be visible in a linked worktree, or the offer
	// repeats in every one of them.
	//
	// `ai/tools/eco-report/root.go` resolves the idsd scratch root by the same rule, and the
	// duplication is deliberate — that resolver answers a different question about a different
	// directory, and a shared helper would make each one's failure the other's. Change one and read
	// the other.
	state, err := recordPath(cwd)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", self, err)
		return exitUndetermined
	}

	run := offer{self: self, state: state, now: now, stdout: stdout, stderr: stderr}
	switch action {
	case "due":
		return run.due()
	case "asked":
		return run.asked()
	default:
		return usage(self, stderr)
	}
}

// offer is one invocation's fixed context. Held together because both arms need all of it, and a
// `due` reading one record while `asked` writes another is the one inconsistency the two must not be
// able to express.
type offer struct {
	self   string
	state  string
	now    Clock
	stdout io.Writer
	stderr io.Writer
}

// `due` and `asked` read as two spellings of one query, and only one of them is: `asked` overwrites
// the record with no undo, so a caller probing the grammar rewrites the state it was asking about.
// The warning therefore comes before the grammar, not after it, and both go to stderr — a caller
// capturing stdout for a verdict must find nothing there.
func usage(self string, stderr io.Writer) int {
	fmt.Fprintf(stderr, "%s: 'due' only reads the record; 'asked' OVERWRITES it with today's date, and nothing undoes that.\n", self)
	fmt.Fprintf(stderr, "usage: %s audit {due|asked}\n", self)
	return exitUndetermined
}

// The disclaimer is part of the message rather than a comment about it. It is all a reader with only
// the output has to tell this from a "not due".
func (o offer) undetermined(format string, args ...any) int {
	fmt.Fprintf(o.stderr, "undetermined: %s — nothing was determined; this is not a 'not due'.\n",
		fmt.Sprintf(format, args...))
	return exitUndetermined
}

func (o offer) due() int {
	recorded, err := readStamp(o.state)
	switch {
	case errors.Is(err, os.ErrNotExist):
		// The first run in a fresh checkout, and it has to be an offer rather than a silence.
		fmt.Fprintf(o.stdout, "due: no %s has ever been offered (no %s).\n", auditTopic, o.state)
		return exitDue
	case err != nil:
		return o.undetermined("%s exists but could not be read", o.state)
	}

	stamped, ok := parseDate(recorded)
	if !ok {
		return o.undetermined("%s holds '%s', which is no YYYY-MM-DD date", o.state, recorded)
	}

	today := o.now().Format(dateLayout)
	todayDate, ok := parseDate(today)
	if !ok {
		// Unreachable: the string was produced by Format one line above. Kept as a refusal rather
		// than a panic so that a future change to how "today" is obtained cannot turn into a day
		// number computed from something that is not a date.
		return o.undetermined("the clock produced '%s', which is no YYYY-MM-DD date", today)
	}

	elapsed := int(todayDate.Sub(stamped).Hours() / 24)
	if elapsed < 0 {
		// A clock change, a bad edit, a merge from a machine ahead. Reading this as a small negative
		// elapsed would print "not due" and hold the offer off indefinitely.
		return o.undetermined("the last %s offer is recorded as %s, which is later than today", auditTopic, recorded)
	}

	// `>=` is what makes the seventh day an offer; `>` would move every cadence in the tree out by
	// one day, invisibly.
	if elapsed >= intervalDays {
		fmt.Fprintf(o.stdout, "due: %s last offered %s, %d days ago (interval %d days).\n", auditTopic, recorded, elapsed, intervalDays)
		return exitDue
	}
	fmt.Fprintf(o.stdout, "not due: %s last offered %s, %d days ago (interval %d days).\n", auditTopic, recorded, elapsed, intervalDays)
	return exitNotDue
}

func (o offer) asked() int {
	today := o.now().Format(dateLayout)
	if err := os.MkdirAll(filepath.Dir(o.state), 0o755); err != nil {
		return o.writeRefused()
	}
	if err := os.WriteFile(o.state, []byte(today+"\n"), 0o644); err != nil {
		return o.writeRefused()
	}
	fmt.Fprintf(o.stdout, "recorded the %s offer on %s.\n", auditTopic, today)
	// Not exitDue: this arm answers no question about whether a pass is owed.
	return 0
}

// The write failing is the half a caller cannot see: the offer was NOT recorded, so the next run must
// offer again, and saying so is what stops the caller believing the date is on disk.
func (o offer) writeRefused() int {
	fmt.Fprintf(o.stderr, "%s: could not write %s — the %s offer was NOT recorded, so the next run will offer again.\n", o.self, o.state, auditTopic)
	return exitUndetermined
}

// The stamp is the first line. A record that grew a second — an editor, a merge — still resolves.
func readStamp(state string) (string, error) {
	body, err := os.ReadFile(state)
	if err != nil {
		return "", err
	}
	first, _, _ := strings.Cut(string(body), "\n")
	return strings.TrimRight(first, "\r"), nil
}

// The shape check is not redundant beside time.Parse. Parse alone would accept a value this script's
// grammar never meant — the pattern is the only thing between `2026-1-15` and a date read as the
// first of January, whose month and day both sit inside the ranges Parse checks.
func parseDate(text string) (time.Time, bool) {
	if len(text) != len(dateLayout) {
		return time.Time{}, false
	}
	for i, char := range []byte(text) {
		want := dateLayout[i]
		if want == '-' {
			if char != '-' {
				return time.Time{}, false
			}
			continue
		}
		if char < '0' || char > '9' {
			return time.Time{}, false
		}
	}
	// UTC, so that a day count is a count of calendar days rather than of 24-hour periods: parsed in
	// a zone with daylight saving, two dates a week apart differ by 167 or 169 hours and the integer
	// division lands on 6 or 7 depending on the time of year.
	parsed, err := time.ParseInLocation(dateLayout, text, time.UTC)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

// recordPath answers where this repository's record lives, or why it could not be determined.
//
// git is asked rather than the layout being walked here. `--git-common-dir` knows about linked
// worktrees, alternates and `$GIT_DIR`, and reimplementing that would put a second, quietly diverging
// answer in the tree — one whose failure mode is writing the record somewhere no other caller looks,
// which is invisible until the offer repeats forever.
func recordPath(cwd string) (string, error) {
	root, err := git(cwd, "rev-parse", "--show-toplevel")
	if err != nil || root == "" {
		return "", errors.New("not inside a git repository, so there is no per-repo record — nothing was determined.")
	}
	// Asked from the root and absolutized against it, because `--git-common-dir` answers relative to
	// the caller's cwd in an ordinary repo: left relative, the record is written to a `.git` the
	// caller's own subdirectory does not have, created on the spot and invisible to everyone else.
	gitDir, err := git(root, "rev-parse", "--git-common-dir")
	if err != nil || gitDir == "" {
		return "", errors.New("could not resolve the repository's shared git dir — nothing was determined.")
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(root, gitDir)
	}
	return filepath.Join(gitDir, recordName), nil
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	// git's stderr is dropped: every failure here becomes one of the two refusals above, which say
	// what was not determined. git's own wording would name a cause the caller cannot act on.
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

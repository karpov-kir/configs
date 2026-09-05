// Apply `~/.kk-flavor/standards/writing.md` → **Score what survives**: hold the per-lane thresholds,
// and cut a scored list against one. It never produces a score — "how much this reader needs it" is a
// judgment, so the caller scores and this decides what that score buys.
//
//	usage: score.sh threshold <lane>
//	       score.sh cut [--kept-all <why>] <lane> <what a 10 is here>
//	           reads `<score><TAB><label>` lines on stdin; exit 3 if nothing fell below the line
//
// Prints to stdout. Exit 2 means it did not run; exit 3 means it ran and refuses the result. A caller
// that cannot tell those apart treats a live refusal as a broken tool and moves on.
//
// Thresholds come from `../thresholds.conf` relative to the stub this was invoked as, which is tracked,
// overlaid per lane by an untracked machine-local file. An override in effect is always announced: a
// bar moved locally produces a verdict no other machine reproduces, and silence about it is the hazard.
//
// The refusals here are the enforcement, not convenience: read the comment at one before removing it.
package score

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"kk-flavor/tools/shell"
)

const maxScore = 10

const (
	exitOK        = 0
	exitDidNotRun = 2
	exitRefusesIt = 3
)

// Env is what the machine can move, held as data so the suite drives every path without touching
// process state.
type Env struct {
	ConfigPath string
	// OverridePath is the untracked machine-local overlay. Empty means this machine has no place for
	// one, which is not the same as having one that is absent.
	OverridePath string
}

type table struct {
	order []string
	level map[string]int
}

func (t *table) names() string {
	var b strings.Builder
	for _, name := range t.order {
		b.WriteString(" " + name)
	}
	return b.String()
}

// session is one invocation's context. Held together because both arms and both refusals need all of
// it, and a refusal naming a different `self` than the report it replaces is the one inconsistency
// they must not be able to express.
type session struct {
	self   string
	env    Env
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func (s session) fail(format string, a ...any) int {
	fmt.Fprintf(s.stderr, "%s: %s\n", s.self, fmt.Sprintf(format, a...))
	return exitDidNotRun
}

func (s session) refuse(format string, a ...any) int {
	fmt.Fprintf(s.stderr, "%s: %s\n", s.self, fmt.Sprintf(format, a...))
	return exitRefusesIt
}

func Run(self string, args []string, env Env, stdin io.Reader, stdout, stderr io.Writer) int {
	s := session{self: self, env: env, stdin: stdin, stdout: stdout, stderr: stderr}
	if len(args) < 2 {
		return s.fail("usage: %s threshold <lane> | %s cut <lane> <what a 10 is here>", self, self)
	}
	switch args[0] {
	case "threshold":
		return s.threshold(args[1:])
	case "cut":
		return s.cut(args[1:])
	default:
		return s.fail("unknown command '%s' — threshold or cut", shell.Oneline(args[0]))
	}
}

func (s session) threshold(rest []string) int {
	if len(rest) != 1 {
		return s.fail("threshold takes one lane")
	}
	level, note, err := resolve(rest[0], s.env)
	if err != nil {
		return s.fail("%s", err)
	}
	// stderr, never stdout: this mode's stdout is the number, read straight back by its caller.
	if note != "" {
		fmt.Fprintf(s.stderr, "%s: %s\n", s.self, note)
	}
	fmt.Fprintf(s.stdout, "%d\n", level)
	return exitOK
}

func (s session) cut(rest []string) int {
	keptAllWhy := ""
	if len(rest) > 0 && rest[0] == "--kept-all" {
		if len(rest) < 2 {
			return s.fail("--kept-all needs the reason nothing fell below the line, in your own words")
		}
		// Neutralised before it is judged, not after. A reason of control bytes clears a TrimSpace and
		// then prints as nothing, so the refusal that exists to force written words is answered by an
		// empty line — the refusal defeated while still reading as enforced.
		keptAllWhy = shell.Oneline(rest[1])
		if strings.TrimSpace(keptAllWhy) == "" {
			return s.fail("--kept-all needs the reason nothing fell below the line, in your own words")
		}
		rest = rest[2:]
	}
	if len(rest) < 2 {
		return s.fail("cut needs the anchor: what a 10 is for this artifact, in your own words")
	}
	laneName := rest[0]
	// Neutralised before it is judged, for the reason above, and because this one also prints into the
	// report body: a carriage return in it overwrites the bar line printed three lines earlier, and a
	// bar the report did not judge against is the one claim its reader cannot check.
	anchor := shell.Oneline(strings.Join(rest[1:], " "))
	// Whitespace, not just absence: `cut prose ""` satisfies an argument count and anchors nothing,
	// which is the refusal above defeated while still reading as enforced.
	if strings.TrimSpace(anchor) == "" {
		return s.fail("the anchor is blank — write what a 10 is for this artifact before any score is read")
	}
	level, note, err := resolve(laneName, s.env)
	if err != nil {
		return s.fail("%s", err)
	}

	fmt.Fprintf(s.stdout, "lane %s, cutting at or below %d\n", laneName, level)
	// In the report body here, not on stderr: the bar a verdict was judged against belongs beside
	// the verdict, and stderr is exactly what a caller piping this report to a file loses.
	if note != "" {
		fmt.Fprintf(s.stdout, "%s\n", note)
	}
	fmt.Fprintf(s.stdout, "10 here means: %s\n\n", anchor)

	kept, gone := 0, 0
	scanner := bufio.NewScanner(s.stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		rawScore, label, hasTab := strings.Cut(line, "\t")
		if !hasTab {
			return s.fail("no tab in '%s' — each line is '<score><TAB><label>'", shell.Oneline(line))
		}
		// A label is text from the artifact under review, printed back into a report its caller
		// reads. A control character rewrites that report rather than appearing in it: `\r`
		// overwrites the verdict on the line and `\v` opens a second one, so an item this cut
		// renders as one it kept while the counts still say it was cut.
		rawScore = shell.Oneline(rawScore)
		label = shell.Oneline(label)
		value, err := parseScore(rawScore)
		if err != nil {
			return s.fail("%s", err)
		}
		if value <= level {
			fmt.Fprintf(s.stdout, "CUT   %2d  %s\n", value, label)
			gone++
		} else {
			fmt.Fprintf(s.stdout, "keep  %2d  %s\n", value, label)
			kept++
		}
	}
	// A read that died mid-list is not an end of list. Reported rather than swallowed: stdin dying
	// after three items would otherwise print `2 kept, 1 cut` at exit 0 — the shape a whole scored
	// list takes, over a list that stopped.
	if err := scanner.Err(); err != nil {
		return s.fail("the scored list stopped mid-read (%v) — what arrived is not the whole list", err)
	}
	// Nothing arrived at all — stdin empty or unreadable, which read the same to a caller. `0 kept,
	// 0 cut` at exit 0 looks exactly like a run that scored a list and cut none of it, so this is
	// the one report this must never produce. Exit 2, never 3: 3 refuses a result, and there is no
	// result here. `--kept-all` cannot excuse it — that flag answers a list read and survived.
	if kept+gone == 0 {
		return s.fail("nothing was scored — no '<score><TAB><label>' line reached stdin. Feed the list in")
	}
	fmt.Fprintf(s.stdout, "\n%d kept, %d cut\n", kept, gone)

	// Everything clearing the bar is what scoring against no anchor looks like: the scale never
	// gets used, every item lands mid-band, and the run reads as a pass. The anchor refusal cannot
	// catch it, because the anchor is a free string written before the scores exist.
	//
	// A tight artifact really can cut nothing, so this is refusable — but only by writing down why.
	if gone == 0 && kept > 0 {
		if keptAllWhy == "" {
			return s.refuse("nothing scored at or below %d. Re-score against the anchor, or re-run with --kept-all '<why nothing fell below it>'", level)
		}
		fmt.Fprintf(s.stdout, "nothing cut, accepted: %s\n", keptAllWhy)
	}
	return exitOK
}

// resolve answers the lane's level, plus the note to print when an override moved it.
func resolve(want string, env Env) (int, string, error) {
	ruled, err := readTable(env.ConfigPath, nil)
	if err != nil {
		return 0, "", err
	}
	level, known := ruled.level[want]
	if !known {
		return 0, "", fmt.Errorf("no lane '%s' in %s — it lists:%s", shell.Oneline(want), env.ConfigPath, ruled.names())
	}

	// Absent is the common case, and the only one that falls back silently. A path that exists but is
	// not a readable file is refused: a directory, a fifo, a dangling symlink, or a mode denying us.
	// Falling back there would restore the tracked bar while its owner believed their tuning was live,
	// which is the same silent-default hole a malformed line is refused for.
	if env.OverridePath == "" {
		return level, "", nil
	}
	if _, err := os.Lstat(env.OverridePath); err != nil {
		return level, "", nil
	}
	info, err := os.Stat(env.OverridePath)
	if err != nil || !info.Mode().IsRegular() || !readable(env.OverridePath) {
		// Neutralised rather than refused, because the path carries `$XDG_CONFIG_HOME`, which its owner
		// may legitimately hold.
		return 0, "", fmt.Errorf("%s is not a readable file. Fix or remove it; skipping it would restore the tracked bar without saying so", shell.Oneline(env.OverridePath))
	}
	// The override may only move a lane the tracked config already rules. What that buys is a loud
	// typo: `instructions` for `instruction` would otherwise tune nothing, silently.
	overlay, err := readTable(env.OverridePath, ruled)
	if err != nil {
		return 0, "", err
	}
	// A lane the override does not name keeps the tracked number: the overlay is per lane, so tuning
	// one bar never silently detaches the rest from the file that rules them.
	moved, named := overlay.level[want]
	if !named {
		return level, "", nil
	}
	note := fmt.Sprintf("lane %s overridden by %s: %d ruled, %d in effect", want, env.OverridePath, level, moved)
	return moved, shell.Oneline(note), nil
}

func readable(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

// readTable parses one thresholds file. `allow`, when non-nil, is the table the tracked config rules;
// a lane outside it is refused.
func readTable(path string, allow *table) (*table, error) {
	if path == "" {
		return nil, fmt.Errorf("the tracked threshold config could not be located: this was invoked as a " +
			"bare name, which names no directory to find it from. Invoke it by a path — the stub does")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no readable threshold config at %s", path)
	}
	out := &table{level: map[string]int{}}
	for _, raw := range strings.Split(string(body), "\n") {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		name := fields[0]
		// A lane name is data, and every message prints it back. Raw, a control byte in it overwrites
		// the line already on the reader's terminal, and an `\033]` sequence reaches the terminal
		// itself. Refused rather than neutralised, because no lane a config can legitimately rule
		// carries one, and refusing keeps every message downstream clean by construction.
		if shell.Oneline(name) != name {
			return nil, fmt.Errorf("%s: the lane name '%s' carries a control character", path, shell.Oneline(name))
		}
		if len(fields) != 4 || fields[1] != "cut" || fields[2] != "<=" {
			return nil, fmt.Errorf("%s: cannot read the line naming '%s' — the form is '<lane> cut <= <n>'", path, shell.Oneline(name))
		}
		level, err := parseDigits(fields[3])
		if err != nil {
			return nil, fmt.Errorf("%s: '%s' has a non-numeric level", path, shell.Oneline(name))
		}
		if level > maxScore {
			return nil, fmt.Errorf("%s: '%s' is %d, over the 0-%d scale", path, shell.Oneline(name), level, maxScore)
		}
		if allow != nil {
			if _, ruled := allow.level[name]; !ruled {
				return nil, fmt.Errorf("%s: '%s' is not a lane the tracked config rules — an override moves a lane, never adds one", path, shell.Oneline(name))
			}
		}
		if _, seen := out.level[name]; !seen {
			out.order = append(out.order, name)
		}
		out.level[name] = level
	}
	return out, nil
}

// parseScore reads one item's score. Digits only — `strconv.Atoi` accepts a leading sign, and `-3`
// is not a point on a 0-10 scale.
func parseScore(text string) (int, error) {
	value, err := parseDigits(text)
	if err != nil {
		return 0, fmt.Errorf("'%s' is not a score 0-%d", text, maxScore)
	}
	if value > maxScore {
		return 0, fmt.Errorf("'%s' is over the 0-%d scale", text, maxScore)
	}
	return value, nil
}

func parseDigits(text string) (int, error) {
	if text == "" {
		return 0, errors.New("empty")
	}
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return 0, errors.New("not all digits")
		}
	}
	return strconv.Atoi(text)
}

// ConfigPaths answers where the two thresholds files live, given the path this was invoked as. The
// tracked one sits beside the scripts directory the stub is in; the override is machine-local and
// outside the repository, so tuning a bar is never a dirty working tree.
func ConfigPaths(argv0 string, lookup func(string) (string, bool)) Env {
	var env Env
	// The tracked config is derived from argv0, so argv0 has to actually name a location. Bare
	// `score.sh` — this tool found on PATH — has no directory component, and `filepath.Dir` answers "."
	// for it, which resolves against whatever directory the process stands in. This tool runs from
	// inside the tree under review, so that tree would supply the bar its own change set is cut
	// against: the same outcome the XDG rule below refuses, reached by the other input. A trailing
	// slash misresolves the same way, one directory too deep.
	//
	// Left empty rather than guessed at. readTable then refuses by name, which is the honest answer —
	// a fallback here would be the tool choosing a threshold file nobody named.
	//
	// `ai/tools/eco-stats/ledger.go`'s ownDirectory holds this same shape for the same reason.
	if strings.Contains(argv0, "/") && !strings.HasSuffix(argv0, "/") {
		here, err := filepath.Abs(filepath.Dir(argv0))
		if err != nil {
			here = filepath.Dir(argv0)
		}
		env.ConfigPath = filepath.Join(here, "..", "thresholds.conf")
	}
	// A config home that is not absolute is treated as unset, which is what the XDG spec itself says to
	// do with one; `ai/tools/eco-report/root.go` holds the same rule for the same reason. Taken as
	// given, it resolves against whatever directory the process stands in — and this tool is run from
	// inside the tree under review, so a checkout shipping `cfg/kk-flavor/thresholds.conf` would choose
	// the bar its own change set is cut against. `filepath.IsAbs("")` is false, so absent and
	// non-absolute fall together and neither needs its own test.
	base, _ := lookup("XDG_CONFIG_HOME")
	if !filepath.IsAbs(base) {
		home, _ := lookup("HOME")
		if !filepath.IsAbs(home) {
			return env
		}
		base = filepath.Join(home, ".config")
	}
	env.OverridePath = filepath.Join(base, "kk-flavor", "thresholds.conf")
	return env
}

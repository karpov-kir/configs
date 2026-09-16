// The pre-commit gate: every check this repo gates on, run only where the change could have moved it.
//
//	usage: gate.sh [--full] [--mutants] [--units] [--why <unit>] [--check-path <name>]
//	       (no flag)     the fast path — run what is stale, skip what is not, defer the mutation harnesses
//	       --full        run everything from cold, ignoring and then refreshing every cached verdict
//	       --mutants     settle the deferred mutation units, and nothing else
//	       --units       print the unit table with each unit's freshness, and stop
//	       --why         print the input files one unit is keyed on, and stop
//	       --check-path  say whether a name is one the gate can safely build a command from, and stop
//
// Skipping is sound, not a sample, because every check here is a pure function of a declared set of
// input files plus the toolchain: the same bytes through the same compiler give the same verdict. A
// unit whose inputs hash to what they hashed on the last green run has a verdict that is already
// known, so skipping it asserts nothing that was not measured. A unit whose inputs moved by a byte is
// run. Nothing here samples, times out or guesses.
//
// A unit's inputs are discovered, with one exception a suite may state for itself. A shell suite that
// builds its own copies of the tools it names — a stub installer, a stub runner — reads as driving the
// real ones, because the two are spelt the same, and takes the whole tool tree on that. `# go-tools:
// none — <why>` in the suite is how it says otherwise; `gate/units.go` states what the gate does with
// it, and refuses the line where the suite's own text contradicts it.
//
// What it may never do, and how each is prevented:
//   - Report a pass for a unit it did not run and has no recorded verdict for. A cache miss runs.
//   - Resolve a unit to an empty input set. That is a rename or a typo silently narrowing the gate, so
//     it exits 2 and names the unit, the way run-tests.sh exits 2 when discovery finds no suites.
//   - Finish having resolved nothing at all. Also exit 2.
//   - Skip something quietly. Every run prints one line per unit, and the deferred mutation units get
//     their own block with the command that settles them.
//
// Go rather than shell, because the cost on this class of machine is process spawns rather than CPU,
// and keying 60-odd units on their declared inputs is a library call here. What is left spawning is
// the work itself — git's file list, the two mutation harnesses' listings, and each unit's own command.
//
// This is a fast path beside the full sweep, never instead of it: .github/workflows/gates.yml still
// runs every command from cold on every push, and `--full` is the same sweep on demand.
package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kk-flavor/tools/shell"
)

type Env struct {
	// Root is the repository the gate runs over. GATE_ROOT.
	Root string
	// Cache is where verdict records live. GATE_CACHE. Empty means the repository's own git dir.
	Cache string
	// UnitsFile replaces the discovered table with one read from a file — id, kind, inputs, command,
	// tab-separated. GATE_UNITS_FILE. It is how the suite reaches the run loop, the cache and every
	// refusal in seconds rather than by running the real suites, which are the very thing this exists
	// not to run.
	UnitsFile string
	// SelfDigest goes into every key, and is the digest of the code deciding the verdicts. A verdict is
	// a statement about these bytes under this toolchain, so a change to the deciding code must not be
	// answered out of the previous one's cache.
	SelfDigest string
}

type mode int

const (
	modeFast mode = iota
	modeFull
	modeMutants
	modeUnits
	modeWhy
	modeCheckPath
	modeHelp
)

type unit struct {
	id     string
	kind   string // "check" or "mutation"
	inputs []string
	cmd    string
	stem   string
	// blindToGoTests marks a unit that cannot observe the module's `_test.go` files, so hashing them
	// into its key only retires a cached verdict that is still good. Two kinds of unit qualify, for
	// two different reasons, and each has to be argued rather than assumed:
	//
	//   - the shell suites that exec a tool `resolve.sh` built, because `go build` does not compile a
	//     test file into a binary;
	//   - `wiring`, because eco-check skips them by name when it reads Go sources for subcommand
	//     dispatches (`eco-check/subcommands.go`), and everything else it walks is `*.sh` or SKILL.md.
	//
	// Left off — the conservative value — everywhere else. Narrowing a key wrongly is the direction
	// that reports a pass nobody earned.
	blindToGoTests bool
	// prerequisite is what THIS MACHINE has to provide for the command to measure everything it claims
	// to, as key material. Per-unit rather than a field in g.stamp, which sits in every key: there, a
	// provider CLI appearing would retire every verdict in the table, units that ask no model included.
	prerequisite string
	// prerequisiteShortfall says, for the unit's own line, what this machine does not provide. Printed
	// on a cache hit too, where nothing runs — true there only because `prerequisite` is in the key.
	prerequisiteShortfall string
}

type gate struct {
	env            Env
	root           string
	cache          string
	stamp          string
	out, errOut    io.Writer
	units          []unit
	manifest       []manifestLine
	scratch        string
	goMutateBinary string
}

type manifestLine struct {
	hash string
	path string
}

func (m manifestLine) String() string { return m.hash + "  " + m.path }

// Run executes one invocation and returns its exit code. 0 is a clean gate, 1 is a finding, and 2 is
// "this did not run" — never a result.
func Run(args []string, env Env, out, errOut io.Writer) int {
	g := &gate{env: env, out: out, errOut: errOut}
	return g.run(args)
}

const usageLine = "usage: gate.sh [--full] [--mutants] [--units] [--why <unit>] [--check-path <name>]"

// A refusal raised while parsing the arguments, the one class a caller can fix from the flag list — so
// it carries that list. One raised after parsing is not one the flag list answers.
func refuseInvocation(errOut io.Writer, reason string) int {
	refuse(errOut, reason)
	return refuse(errOut, usageLine)
}

func (g *gate) fail(format string, a ...any) int {
	return refuse(g.errOut, fmt.Sprintf(format, a...))
}

func refuse(errOut io.Writer, reason string) int {
	fmt.Fprintf(errOut, "gate.sh: %s\n", shell.Oneline(reason))
	return 2
}

func (g *gate) run(args []string) int {
	started := time.Now()

	selected, whyUnit, checkPath, code := parseArgs(args, g.errOut)
	if code != 0 {
		return code
	}

	if selected == modeHelp {
		fmt.Fprintln(g.out, usageLine)
		return 0
	}

	// Driven on its own so a suite can exercise the refusal without writing a hostile filename into
	// this checkout — which is the only other way to reach it, and not a thing to leave lying in a
	// repository.
	if selected == modeCheckPath {
		if err := safeToken("path", checkPath); err != nil {
			return g.fail("%s", err)
		}
		fmt.Fprintf(g.out, "gate.sh: '%s' is a name the gate can safely build a command from\n", checkPath)
		return 0
	}

	if code := g.resolveMachine(); code != 0 {
		return code
	}
	scratch, err := os.MkdirTemp("", "eco-gate")
	if err != nil {
		return g.fail("could not create a scratch directory — nothing ran")
	}
	g.scratch = scratch
	defer os.RemoveAll(scratch)

	if code := g.buildUnits(); code != 0 {
		return code
	}
	if len(g.units) == 0 {
		return g.fail("no units resolved at all — read this as the gate broken, never as a clean run")
	}
	if code := g.assignStems(); code != 0 {
		return code
	}
	if code := g.buildManifest(); code != 0 {
		return code
	}

	switch selected {
	case modeUnits:
		return g.printUnits()
	case modeWhy:
		return g.printWhy(whyUnit)
	}
	return g.runUnits(selected, started)
}

func parseArgs(args []string, errOut io.Writer) (selected mode, why, path string, code int) {
	selected = modeFast
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--full":
			selected = modeFull
		case "--mutants":
			selected = modeMutants
		case "--units":
			selected = modeUnits
		case "--check-path":
			i++
			if i >= len(args) {
				return selected, why, path, refuseInvocation(errOut, "--check-path needs a path")
			}
			path, selected = args[i], modeCheckPath
		case "--why":
			i++
			if i >= len(args) {
				return selected, why, path, refuseInvocation(errOut, "--why needs a unit id — run --units for the list")
			}
			why, selected = args[i], modeWhy
		case "-h", "--help":
			// A mode, not an early `return 0`. Returning zero here says "these arguments parsed", which
			// is what run() reads it as — so help printed and then the whole gate ran, writing verdict
			// records for a caller who asked what the flags were.
			return modeHelp, why, path, 0
		default:
			return selected, why, path, refuseInvocation(errOut, fmt.Sprintf("unknown argument '%s'", args[i]))
		}
	}
	return selected, why, path, 0
}

// A path or key that goes into a command string this later runs through a shell. Anything outside this
// set — a space, a semicolon, a quote, a leading dash — stops being a filename and starts being
// syntax: a zero-byte `ai/a;true;#-test.sh` runs as `ai/run-tests.sh -s ai/a` then `true`, so the unit
// exits 0 and the gate writes a green record for a suite that never ran. The file's contents are
// empty, so nothing reviewing contents would see it; the executable part is the name.
//
// Refused rather than escaped, and refused at discovery rather than at use, so the gate fails closed
// the way its other refusals do and says which name it cannot handle.
// Single quotes, the one form a POSIX shell reads literally throughout. Written out rather than
// assumed safe: safeToken and the quoting are two defences, and an injection needs both to fail.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func safeToken(what, value string) error {
	if value == "" {
		return fmt.Errorf("an empty %s names no file, so the gate refuses to build a command from it — nothing ran", what)
	}
	if strings.HasPrefix(value, "-") {
		return fmt.Errorf("%s '%s' begins with a dash, which the command it goes into would read as an option — nothing ran", what, value)
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		ok := c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' ||
			c == '.' || c == '_' || c == '/' || c == '-'
		if !ok {
			return fmt.Errorf("%s '%s' holds a byte the gate cannot safely put in a command, so it refuses to build one — nothing ran", what, value)
		}
	}
	return nil
}

func (g *gate) resolveMachine() int {
	root := g.env.Root
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return g.fail("could not resolve the root '%s' — nothing ran", root)
	}
	physical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return g.fail("could not resolve the root '%s' — nothing ran", root)
	}
	g.root = physical

	if _, err := exec.LookPath("go"); err != nil {
		return g.fail("no go on this machine, so the Go half cannot be built or run — nothing ran")
	}
	common, err := g.capture("git", "rev-parse", "--git-common-dir")
	if err != nil || common == "" {
		return g.fail("%s is not a git repository, so there is nothing to scope a change against — nothing ran", g.root)
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(g.root, common)
	}
	// The store is the CLONE's: --git-common-dir is shared by every worktree of a checkout, so three
	// worktrees gating at once write one directory. That sharing is the intended semantics, and the
	// reason belongs here because this line is where a reader meets it. A verdict record is an empty
	// file whose NAME is its key, and keyMaterial in keys.go hashes the unit id, its command, the
	// toolchain stamp and the hashes of its declared inputs — never g.root, and never an absolute path,
	// since every command and every manifest path is relative to the repository.
	// TestNoKeyMaterialNamesTheWorktreeItWasBuiltIn holds that over the real table, so it is a checked
	// property rather than a claim. A record another worktree wrote is therefore found only by a
	// worktree computing the same key, meaning it holds the same inputs under the same toolchain, which
	// is the gate's own premise. A cross-worktree hit is the same event as a same-worktree one, and
	// neither is a stale green. The record holds no content, so there is nothing in it to catch
	// half-written. The `<stem>.inputs` sidecars beside the records carry no key at all;
	// `gotest.inputs` is the only one anything reads back, and changedSinceGreen covers it.
	//
	// Keying the store per worktree would end all of that: every new worktree would gate from cold,
	// which is the sweep this fast path exists to avoid. GATE_CACHE is how a run that must not share
	// says so — a suite pointing at its own fixture, or a session isolating itself by hand.
	g.cache = g.env.Cache
	if g.cache == "" {
		g.cache = filepath.Join(common, "eco-gate")
	}
	if err := os.MkdirAll(g.cache, 0o755); err != nil {
		return g.fail("could not create the cache at %s — nothing ran", g.cache)
	}
	g.sweepLeakedSidecars()

	digest := g.env.SelfDigest
	if digest == "" {
		// The running binary is the deciding code. Refused rather than defaulted: an empty digest is a
		// key component that never changes, and every verdict keyed on it would survive any edit to it.
		self, err := os.Executable()
		if err == nil {
			digest, err = hashFile(self)
		}
		if err != nil || digest == "" {
			return g.fail("could not hash the gate binary, so a verdict could not be keyed to the code deciding it — nothing ran")
		}
	}
	goVersion, _ := g.capture("go", "version")
	gitVersion, _ := g.capture("git", "--version")
	nodeVersion, err := g.capture("node", "--version")
	if err != nil {
		nodeVersion = "unavailable"
	}
	g.stamp = fmt.Sprintf("%s | %s | node %s | gate %s", goVersion, gitVersion, nodeVersion, digest)
	return 0
}

// A verdict key is a sha256 rendered as hex, so every record's name ends in a tail this long. Held as
// a number because the sweep below tells a leaked temp from a record by tail length and nothing else;
// `TestAVerdictKeyIsAsLongAsTheSweepThinks` holds it against the function that produces one.
const verdictKeyLength = 64

// How long a temp must have sat before the sweep takes it. THE POINT OF THE BOUND IS THE GUARD, not
// tidiness: the store is the clone's, so a sibling worktree may have a temp in flight this second, and
// deleting that one makes its sidecar vanish and the run that owns it write nothing. Publishing takes
// milliseconds, so nothing an hour old is still being written, and a leak simply waits an hour.
const leakedSidecarAge = time.Hour

// Temps `writeSidecar` left behind. A run killed between creating one and renaming it onto its path
// leaks it, and the store is shared by every worktree of the clone, so they arrive from every killed
// run in every sibling session — and killing a gate run is routine here. Each one is inert, since
// nothing reads a name but `<stem>.inputs` and `<stem>.<key>`; a directory full of them is what makes
// a later stale-verdict investigation unreadable.
//
// Silent and best-effort. A sweep that cannot read the directory, or cannot remove an entry, leaves
// clutter and decides no verdict, so there is nothing here to report or to stop a run for.
func (g *gate) sweepLeakedSidecars() {
	entries, err := os.ReadDir(g.cache)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !leakedSidecarName(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil || time.Since(info.ModTime()) < leakedSidecarAge {
			continue
		}
		os.Remove(filepath.Join(g.cache, entry.Name()))
	}
}

// `<stem>.inputs.<random>`, the shape os.CreateTemp leaves. The tail is measured rather than merely
// found, because a verdict record is `<stem>.<key>` and a units-file table may name a unit whose stem
// ends in `.inputs` — its record is then spelt exactly like a temp. CreateTemp's tail is a short
// decimal and a key is always verdictKeyLength, so the length is what tells a leak from a green.
func leakedSidecarName(name string) bool {
	at := strings.LastIndex(name, sidecarSuffix+".")
	if at < 0 {
		return false
	}
	tail := name[at+len(sidecarSuffix)+1:]
	return tail != "" && len(tail) < verdictKeyLength
}

func (g *gate) captureLiteralPathspecs(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.root
	cmd.Env = append(os.Environ(), "GIT_LITERAL_PATHSPECS=1")
	out, err := cmd.Output()
	return strings.TrimRight(string(out), "\n"), err
}

func (g *gate) capture(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = g.root
	out, err := cmd.Output()
	return strings.TrimRight(string(out), "\n"), err
}

func hashFile(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func hashString(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// The record's filename, which is not the id. An id carries bytes a path segment may not: `:` in every
// `mutants:go:…`, `+` where a unit covers more than one suite, and possibly a `/` — which would name a
// directory the cache does not have, so every write for that unit would fail and `--mutants` would
// report a pass having recorded nothing. Mutation ids hold no `/` today, but a units-file table may.
func recordStem(id string) string {
	var b strings.Builder
	for i := 0; i < len(id); i++ {
		c := id[i]
		ok := c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' ||
			c == '.' || c == '_' || c == '-'
		if ok {
			b.WriteByte(c)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

// Two units that share a cache record share a verdict: running either would report the other fresh
// over inputs nothing had read. Asked about stems rather than ids, because the stem is the record's
// name — identical ids always flatten to one stem, so an id check could never fire on its own.
func (g *gate) assignStems() int {
	byStem := map[string][]string{}
	for i := range g.units {
		stem := recordStem(g.units[i].id)
		g.units[i].stem = stem
		byStem[stem] = append(byStem[stem], g.units[i].id)
	}
	var clashes []string
	for stem, ids := range byStem {
		if len(ids) > 1 {
			clashes = append(clashes, stem)
		}
	}
	if len(clashes) == 0 {
		return 0
	}
	sort.Strings(clashes)
	fmt.Fprintln(g.errOut, "gate.sh: these units share one cache record, so a verdict could not say which of them it belongs to — nothing ran")
	for _, stem := range clashes {
		ids := shell.SortUnique(byStem[stem])
		if len(ids) == 1 {
			fmt.Fprintf(g.errOut, "    %s — carried by two units under one id\n", ids[0])
		} else {
			fmt.Fprintf(g.errOut, "    %s — different ids, one record name\n", strings.Join(ids, " "))
		}
	}
	return 2
}

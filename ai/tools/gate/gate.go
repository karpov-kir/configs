// The pre-commit gate: every check this repo gates on, run only where the change could have moved it.
//
//	usage: gate.sh [--full] [--mutants] [--units] [--why <unit>] [--check-path <path>]
//	       (no flag)  the fast path — run what is stale, skip what is not, defer the mutation harnesses
//	       --full     run everything from cold, ignoring and then refreshing every cached verdict
//	       --mutants  settle the deferred mutation units, and nothing else
//	       --units    print the unit table with each unit's freshness, and stop
//	       --why      print the input files one unit is keyed on, and stop
//
// Skipping is sound, not a sample, because every check here is a pure function of a declared set of
// input files plus the toolchain: the same bytes through the same compiler give the same verdict. A
// unit whose inputs hash to what they hashed on the last green run has a verdict that is already
// known, so skipping it asserts nothing that was not measured. A unit whose inputs moved by a byte is
// run. Nothing here samples, times out or guesses.
//
// What it may never do, and how each is prevented:
//   - Report a pass for a unit it did not run and has no recorded verdict for. A cache miss runs.
//   - Resolve a unit to an empty input set. That is a rename or a typo silently narrowing the gate, so
//     it exits 2 and names the unit, the way run-tests.sh exits 2 when discovery finds no suites.
//   - Finish having resolved nothing at all. Also exit 2.
//   - Skip something quietly. Every run prints one line per unit, and the deferred mutation units get
//     their own block with the command that settles them.
//
// Go rather than shell, and this is where the shell version spent itself: the cost on this class of
// machine is process spawns, not CPU, and keying 60-odd units meant a `shasum` and an `awk` each on
// top of an `xargs shasum` over every declared input. All of that is a library call here. What is left
// spawning is the work itself — git's file list, the two mutation harnesses' listings, and each unit's
// own command.
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
)

// Env is what a caller — or a suite — can move. Each field is a seam the shell version had as an
// environment variable, and each exists for the same reason: without them a suite could only test the
// gate by writing fixtures into the checkout the gate is gating.
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
)

// A unit: one check, its declared inputs, and the command that settles it.
type unit struct {
	id     string
	kind   string // "check" or "mutation"
	inputs []string
	cmd    string
	stem   string
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

// One hashed input file, in the form the keys are built from.
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

func (g *gate) fail(format string, a ...any) int {
	fmt.Fprintf(g.errOut, "gate.sh: %s\n", fmt.Sprintf(format, a...))
	return 2
}

func (g *gate) run(args []string) int {
	started := time.Now()

	selected, whyUnit, checkPath, code := parseArgs(args, g.errOut)
	if code != 0 {
		return code
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

func parseArgs(args []string, errOut io.Writer) (mode, string, string, int) {
	selected := modeFast
	why, path := "", ""
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
				fmt.Fprintln(errOut, "gate.sh: --check-path needs a path")
				return selected, why, path, 2
			}
			path, selected = args[i], modeCheckPath
		case "--why":
			i++
			if i >= len(args) {
				fmt.Fprintln(errOut, "gate.sh: --why needs a unit id — run --units for the list")
				return selected, why, path, 2
			}
			why, selected = args[i], modeWhy
		case "-h", "--help":
			fmt.Fprintln(errOut, "usage: gate.sh [--full] [--mutants] [--units] [--why <unit>]")
			return selected, why, path, 0
		default:
			fmt.Fprintf(errOut, "gate.sh: unknown argument '%s'\n", args[i])
			return selected, why, path, 2
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

// The toolchain and the deciding code, which go into every key together.
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
		return g.fail("this is not a git repository, so there is nothing to scope a change against — nothing ran")
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(g.root, common)
	}
	g.cache = g.env.Cache
	if g.cache == "" {
		g.cache = filepath.Join(common, "eco-gate")
	}
	if err := os.MkdirAll(g.cache, 0o755); err != nil {
		return g.fail("could not create the cache at %s — nothing ran", g.cache)
	}

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
	g.stamp = fmt.Sprintf("%s | %s | gate %s", goVersion, gitVersion, digest)
	return 0
}

// One child process, in the root, with its output captured.
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

// The record's filename, which is not the id. Ids are package-qualified — `mutants:go:eco-check/
// shell.go` — and a `/` in one names a directory the cache does not have, so every write for a go
// mutation unit would fail and `--mutants` would report a pass having recorded nothing.
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
		ids := uniqueSorted(byStem[stem])
		if len(ids) == 1 {
			fmt.Fprintf(g.errOut, "    %s — carried by two units under one id\n", ids[0])
		} else {
			fmt.Fprintf(g.errOut, "    %s — different ids, one record name\n", strings.Join(ids, " "))
		}
	}
	return 2
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

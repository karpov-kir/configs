// Cases for `ai/mcp-env.sh` — the wrapper every stdio MCP server is launched through, and the only
// thing standing between an unpinned `npx` package and every credential exported in the shell that
// started the client.
//
// The script stays shell and cannot become anything else: the MCP client launches it from a path
// written into a config, on a machine that may have nothing built. A stub would exit 2 there and turn
// every server into a startup failure, and where a toolchain IS present it would run `go build`
// inside the very unstripped environment this wrapper exists to keep unreviewed code away from.
//
// So the exec stays, and only the exec: this is the one subject in the module measured a process at a
// time, because what it measures is what a child saw after `env -i`, and nothing in Go can answer that
// about a bash script without running it. Cases share a launch wherever two of them read the same
// child.
//
// They live in this package because `ai/mcp-env.sh` sits outside the module, and Go keys a package's
// test cache on the module: a package of their own under `ai/tools/` would answer `ok (cached)` over a
// wrapper that had changed underneath the run — and for a child process it could not do better, since
// nothing the child opens reaches the cache at all.
package tools_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The wrapper under test, beside the rest of `ai/`.
const wrapper = repoRoot + "/ai/mcp-env.sh"

// The names the wrapper may pass, written out here rather than asked of it.
//
// Every other case derives its expectation from the script under test, so on their own they stay
// green while the allow-list grows: adding GITHUB_TOKEN to it leaves all of them passing and hands
// the token to an unpinned `npx` package. This literal is what goes red on that edit, so widening the
// wrapper means editing this list in the same commit, where a reviewer reads the new name next to the
// old ones.
var pinnedAllowList = []string{
	"PATH", "HOME", "USER", "LOGNAME",
	"TMPDIR", "TMP", "TEMP",
	"LANG", "LC_ALL", "LC_CTYPE",
	"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME",
	"MISE_DATA_DIR", "MISE_CONFIG_DIR", "MISE_CACHE_DIR",
	"NODE_EXTRA_CA_CERTS", "SSL_CERT_FILE", "SSL_CERT_DIR",
	"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy",
}

// Three names no allow-list entry resembles, carrying values nothing else on this machine prints.
var sentinels = []string{
	"FAKE_API_KEY=sentinel-alpha",
	"GH_TOKEN=sentinel-bravo",
	"AWS_SECRET_ACCESS_KEY=sentinel-charlie",
}

const sentinelMark = "sentinel-"

// The path to the wrapper, refused loudly where it cannot be run: every case below is a launch of it,
// so a script this suite cannot execute makes all of them fail for a reason that has nothing to do
// with the guard they name.
func wrapperPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(wrapper)
	if err != nil {
		t.Fatalf("resolving %s: %v — nothing was measured", wrapper, err)
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("%s is not an executable file (%v) — nothing was measured, and every case in this "+
			"package would fail for that reason rather than for its own", path, err)
	}
	return path
}

// The environment a shell's `env NAME=VALUE … mcp-env.sh …` builds: this process's own, plus the
// assignments. Passed whole rather than merged inside launch, because one case needs a name TAKEN OUT
// of it and a merge can only put names in.
func launchingEnv(assignments ...string) []string {
	return append(os.Environ(), assignments...)
}

// One launch, with the wrapper started in exactly the environment it is given.
func launch(t *testing.T, environment []string, args ...string) (output string, code int) {
	t.Helper()
	command := exec.Command(wrapperPath(t), args...)
	command.Env = environment
	said, err := command.CombinedOutput()
	if err == nil {
		return string(said), 0
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("could not run the wrapper: %v\n%s — nothing was measured", err, said)
	}
	return string(said), exit.ExitCode()
}

// What the child saw. `env` as the command throughout and never a shell: bash would add PWD, SHLVL
// and `_` of its own, and the set comparison below would then be measuring bash rather than the
// allow-list.
func childEnvIn(t *testing.T, environment []string) string {
	t.Helper()
	out, code := launch(t, environment, "env")
	if code != 0 {
		t.Fatalf("the wrapper exited %d instead of launching `env`:\n%s", code, out)
	}
	return out
}

func childEnv(t *testing.T, assignments ...string) string {
	t.Helper()
	return childEnvIn(t, launchingEnv(assignments...))
}

// --- the sentinels ---
//
// The control that makes the rest mean anything. "No secret reached the child" passes just as well
// when the child printed nothing at all, so the same sentinels are measured WITHOUT the wrapper
// first, and that run has to find them.
func TestNoSecretInTheLaunchingEnvironmentReachesTheChild(t *testing.T) {
	t.Parallel()
	direct := exec.Command("env")
	direct.Env = launchingEnv(sentinels...)
	said, err := direct.Output()
	if err != nil {
		t.Fatalf("running `env` without the wrapper: %v — nothing was measured", err)
	}
	if found := strings.Count(string(said), sentinelMark); found != len(sentinels) {
		t.Fatalf("the sentinels are not in the launching environment: %d of %d found. Without them "+
			"there, the case below would pass against a wrapper that does nothing at all.",
			found, len(sentinels))
	}

	through := childEnv(t, sentinels...)
	if found := strings.Count(through, sentinelMark); found != 0 {
		t.Errorf("%d sentinel value(s) reached the child. Every credential exported in the shell that "+
			"started the client would reach an unpinned `npx` package the same way.\n%s", found, through)
	}
	// The control on the same run: a wrapper that died before exec'ing anything also prints no
	// sentinel, and a zero above would read as the stripping working.
	if strings.TrimSpace(through) == "" {
		t.Errorf("the child printed nothing, so the line above measured silence rather than absence")
	}
}

// --- the allow-list, every row of it ---
func TestTheAllowListIsExactlyTheNamesPinnedHere(t *testing.T) {
	t.Parallel()
	promised := promisedNames(t)
	if len(promised) == 0 {
		t.Fatal("the help names no allow-list at all, so the comparison below reads nothing")
	}
	if got, want := strings.Join(promised, " "), strings.Join(sorted(pinnedAllowList), " "); got != want {
		t.Errorf("the wrapper passes\n  %s\nand this file pins\n  %s\nA name added to the wrapper widens "+
			"what an unreviewed release can read, so it is added here in the same commit, where a reviewer "+
			"reads it next to the old ones.", got, want)
	}
}

// The names the help prints, set to markers and asked for back. Anything the child holds that the help
// does not name is a leak the sentinels would have missed; anything named that does not arrive is a
// server that may not start.
func TestEveryNameTheHelpPromisesArrivesAndNothingElseDoes(t *testing.T) {
	t.Parallel()
	promised := promisedNames(t)
	var marked []string
	for _, name := range promised {
		switch {
		// Left at their real values: the wrapper needs PATH to exec anything, and a marker HOME would
		// only be a marker.
		case name == "PATH" || name == "HOME":
		// A locale name has to be a locale, or every child writes a setlocale warning to stderr and this
		// suite's output grows a line no case put there.
		case name == "LANG" || strings.HasPrefix(name, "LC_"):
			marked = append(marked, name+"=C")
		default:
			marked = append(marked, name+"=marker-"+name)
		}
	}

	var observed []string
	for _, line := range strings.Split(strings.TrimSuffix(childEnv(t, marked...), "\n"), "\n") {
		if name, _, held := strings.Cut(line, "="); held {
			observed = append(observed, name)
		}
	}
	if got, want := strings.Join(sorted(observed), " "), strings.Join(promised, " "); got != want {
		t.Errorf("the child holds\n  %s\nand the help promises\n  %s\nA name the child holds that the help "+
			"does not name is a leak; one named that does not arrive is a server that may not start.", got, want)
	}
}

// --- unset, set-empty, set: three different things ---
//
// An empty TMPDIR forwarded as `TMPDIR=` makes npx unpack into a path that is the empty string
// instead of falling back to /tmp, which is why this tells them apart by name rather than by value.
func TestASetVariableArrivesWithItsValueAndAnUnsetOneStaysUnset(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		set  string
		want string
	}{
		{name: "a set variable arrives with its value", set: "LC_CTYPE=en_US.UTF-8", want: "LC_CTYPE=en_US.UTF-8"},
		{name: "a set-but-empty variable arrives, still empty", set: "LC_CTYPE=", want: "LC_CTYPE="},
		// A value from outside the system, round-tripped. The wrapper passes NAMES, so the value is
		// whatever the launching environment holds and it must not be touched.
		{name: "a non-ASCII value survives untouched", set: "LC_CTYPE=ünïcodé-Ω-Ω", want: "LC_CTYPE=ünïcodé-Ω-Ω"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			if got := line(childEnv(t, scenario.set), "LC_CTYPE="); got != scenario.want {
				t.Errorf("%s: the child holds %q, want %q", scenario.name, got, scenario.want)
			}
		})
	}
}

// Counted off one captured run, with its controls on the same run: a wrapper that died before
// exec'ing anything also prints no LC_CTYPE, and an absence would read as the guard working.
func TestAnUnsetVariableStaysUnsetRatherThanArrivingEmpty(t *testing.T) {
	t.Parallel()
	probe := childEnvIn(t, withoutName(os.Environ(), "LC_CTYPE"))
	if line(probe, "HOME=") == "" {
		t.Fatalf("the child did not run on that one, so an absence below would be silence rather than a "+
			"variable left unset:\n%s", probe)
	}
	if got := line(probe, "LC_CTYPE="); got != "" {
		t.Errorf("an unset LC_CTYPE arrived as %q. Forwarded as an empty value, a variable npx reads for "+
			"a path makes it write to the empty string rather than fall back.", got)
	}
	// And the other control: this is not the child dropping every LC_CTYPE it is given.
	if got := line(childEnv(t, "LC_CTYPE=whatever"), "LC_CTYPE="); got != "LC_CTYPE=whatever" {
		t.Errorf("the child drops LC_CTYPE whatever it is given (%q), so the case above says nothing about "+
			"unset in particular", got)
	}
}

// --- the exec ---
func TestTheArgumentsReachTheCommandIntact(t *testing.T) {
	t.Parallel()
	out, code := launch(t, launchingEnv(), "printf", "%s|", "a b", "c$d", "e'f", `"g"`)
	if code != 0 {
		t.Fatalf("the wrapper exited %d\n%s", code, out)
	}
	if want := `a b|c$d|e'f|"g"|`; out != want {
		t.Errorf("the command was handed\n  %q\nand it was given\n  %q\nAn argument split or expanded here "+
			"is a server launched with something the human never wrote.", out, want)
	}
}

func TestTheCommandsExitStatusIsTheWrappers(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		want int
	}{
		{name: "a command that fails", want: 7},
		// The control: a command that succeeds must not be reported as a failure either.
		{name: "control: a command that succeeds", want: 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			out, code := launch(t, launchingEnv(), "sh", "-c", fmt.Sprintf("exit %d", scenario.want))
			if code != scenario.want {
				t.Errorf("a command exiting %d came back as %d — the client reads this status to decide "+
					"whether the server started\n%s", scenario.want, code, out)
			}
		})
	}
}

// --- the two arms that launch nothing ---
//
// Asserted on wording as well as status. Every refusal here exits 2, so the code says one happened and
// never which.
func TestNoCommandExitsTwoRatherThanLaunchingSomething(t *testing.T) {
	t.Parallel()
	out, code := launch(t, launchingEnv())
	if code != 2 {
		t.Errorf("exit %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "nothing was launched") {
		t.Errorf("the refusal does not say that nothing was launched, which is the whole of what a caller "+
			"needs to know\n%s", out)
	}
}

// The help arm prints a line range out of the wrapper's own header, so two line numbers stand in for a
// claim about that file's content, and a line added above the range silently makes them the wrong
// lines. Both ends are pinned by content instead: the header line the range has to start at, and the
// first line past it, which has to stay out.
//
// Both needles are read out of the wrapper, never written out here. A needle spelled literally is a
// claim about prose, and prose gets reworded: the moment the wrapper's wording moves, the literal
// matches nothing, "the help lacks it" becomes true of every possible output, and the case passes
// forever without reaching its subject. Read from the file, a needle follows the rewording, and the
// controls below fail when it reads as empty.
func TestHelpPrintsTheHeaderItPromisesAndStopsThere(t *testing.T) {
	t.Parallel()
	out, code := launch(t, launchingEnv(), "--help")
	if code != 0 {
		t.Fatalf("--help exited %d\n%s", code, out)
	}
	if short, _ := launch(t, launchingEnv(), "-h"); short != out {
		t.Errorf("-h and --help are not the same arm\n   -h: %q\n--help: %q", short, out)
	}

	opening := headerProse(t, 1)
	if opening == "" {
		t.Fatal("the header's opening line was not found, so the case below compares nothing")
	}
	if !strings.Contains(out, opening) {
		t.Errorf("the help does not start at the header's opening line %q — a line added above the printed "+
			"range pushes the last fact out of it, and nothing says so\n%s", opening, out)
	}

	// The help prints two of those lines, so the third is the first one it must not reach.
	past := headerProse(t, 3)
	if past == "" {
		t.Fatal("the line past the printed range was not found, so the case below compares nothing")
	}
	if strings.Contains(out, past) {
		t.Errorf("the help reaches %q, which is a note to a maintainer and not help\n%s", past, out)
	}
}

// The names the wrapper's own help says it passes, sorted.
func promisedNames(t *testing.T) []string {
	t.Helper()
	out, code := launch(t, launchingEnv(), "--help")
	if code != 0 {
		t.Fatalf("--help exited %d\n%s", code, out)
	}
	for _, text := range strings.Split(out, "\n") {
		if names, held := strings.CutPrefix(text, "passes only: "); held {
			return sorted(strings.Fields(names))
		}
	}
	return nil
}

// The nth non-blank comment line of the wrapper's header, past the shebang.
var headerLine = regexp.MustCompile(`^# ?(.+)$`)

func headerProse(t *testing.T, nth int) string {
	t.Helper()
	body, err := os.ReadFile(wrapperPath(t))
	if err != nil {
		t.Fatalf("reading the wrapper: %v", err)
	}
	seen := 0
	for _, text := range strings.Split(string(body), "\n")[1:] {
		match := headerLine.FindStringSubmatch(text)
		if match == nil {
			continue
		}
		seen++
		if seen == nth {
			return match[1]
		}
	}
	return ""
}

// The whole line beginning with prefix, empty where the child holds none.
func line(text, prefix string) string {
	for _, one := range strings.Split(text, "\n") {
		if strings.HasPrefix(one, prefix) {
			return one
		}
	}
	return ""
}

// An environment with one name taken out, which is what a caller means by "unset" — not set to empty.
func withoutName(environment []string, name string) []string {
	var kept []string
	for _, entry := range environment {
		if held, _, _ := strings.Cut(entry, "="); held != name {
			kept = append(kept, entry)
		}
	}
	return kept
}

func sorted(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

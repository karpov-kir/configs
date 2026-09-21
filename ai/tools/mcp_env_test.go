// Cases for `ai/mcp-env.sh`, the wrapper every stdio MCP server is launched through. It stands between
// an unpinned `npx` package and every credential exported in the shell that started the client.

// The script stays shell and cannot become anything else. The MCP client launches it from a path
// written into a config, on a machine that may have no toolchain built. A stub would exit 2 there and
// turn every server into a startup failure. Where a toolchain IS present, a stub would run `go build`
// inside the very unstripped environment this wrapper keeps unreviewed code away from.

// So the exec stays, and only the exec. This subject is measured a process at a time. What it measures
// is what a child saw after `env -i`, and only running the script can answer that. Cases share a launch
// wherever two of them read the same child.

// They live in this package, and not in one of their own, for the reason shipped_tree_test.go's cases
// do. One thing no placement fixes: these cases run the wrapper as a child process, and the files a
// child opens stay out of the runner's cache.
package tools_test

import (
	"configs/ai/tools/runtest"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
)

// The wrapper under test, beside the rest of `ai/`.
const wrapper = repoRoot + "/ai/mcp-env.sh"

// Every other case derives its expectation from the script under test, so on their own they stay green
// as the allowlist grows. Adding GITHUB_TOKEN to it leaves all of them passing and hands the token to
// an unpinned `npx` package.

// This literal is what goes red on that edit. The list is edited in the same commit as the wrapper,
// where a reviewer reads the new name next to the old ones.

// The names the wrapper may pass, pinned here and never asked of it.
var pinnedAllowList = []string{
	"PATH", "HOME", "USER", "LOGNAME",
	"TMPDIR", "TMP", "TEMP",
	"LANG", "LC_ALL", "LC_CTYPE",
	"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME",
	"MISE_DATA_DIR", "MISE_CONFIG_DIR", "MISE_CACHE_DIR",
	"NODE_EXTRA_CA_CERTS", "SSL_CERT_FILE", "SSL_CERT_DIR",
	"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy",
}

// Three names unlike any allowlist entry, carrying values that only these cases ever set.
var sentinels = []string{
	"FAKE_API_KEY=sentinel-alpha",
	"GH_TOKEN=sentinel-bravo",
	"AWS_SECRET_ACCESS_KEY=sentinel-charlie",
}

const sentinelMark = "sentinel-"

// Returns the environment a shell's `env NAME=VALUE … mcp-env.sh …` builds: this process's own, plus
// the assignments. It is passed whole, because one case needs a name TAKEN OUT of it and a merge
// inside launch could only put names in.
func launchingEnv(assignments ...string) []string {
	return append(os.Environ(), assignments...)
}

// One launch, with the wrapper started in exactly the environment it is given. runnableScript, the
// helper this calls, lives in stub_reach_test.go: a script the suite cannot execute makes every case
// here fail for a reason unrelated to the guard it names.
func launch(t *testing.T, environment []string, args ...string) (output string, code int) {
	t.Helper()
	command := exec.Command(runtest.Runnable(t, wrapper), args...)
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

// Returns what the child saw. The command is `env` throughout and never a shell, because bash adds
// PWD, SHLVL and `_` of its own. The name comparison in
// TestEveryNameTheHelpPromisesArrivesAndNothingElseDoes would then measure bash instead of the
// allowlist.
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

// "No secret reached the child" passes just as well over a child that printed silence, so the control
// is on the same run: the child has to have printed something.
func TestNoSecretInTheLaunchingEnvironmentReachesTheChild(t *testing.T) {
	t.Parallel()
	// That the sentinels reach the launching environment at all was proved once, by hand, with a bare
	// `env` run over the same slice. os/exec hands that slice to the child verbatim, so re-running it
	// every time measures the standard library.
	through := childEnv(t, sentinels...)
	if found := strings.Count(through, sentinelMark); found != 0 {
		t.Errorf("%d sentinel value(s) reached the child. Every credential exported in the shell that "+
			"started the client would reach an unpinned `npx` package the same way.\n%s", found, through)
	}
	// The control on the same run: a wrapper that died before exec'ing anything also prints no
	// sentinel, and a zero from strings.Count would read as the stripping working.
	if strings.TrimSpace(through) == "" {
		t.Errorf("the child printed nothing, so the line above measured silence rather than absence")
	}
}

// --- the allowlist, every row of it ---
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

// The names the help prints, set to markers and asked for back. A name the child holds that the help
// leaves out is a leak the sentinels would have missed. A name promised that never arrives is a server
// that may fail to start.
func TestEveryNameTheHelpPromisesArrivesAndNothingElseDoes(t *testing.T) {
	t.Parallel()
	promised := promisedNames(t)
	var marked []string
	for _, name := range promised {
		switch {
		// PATH and HOME keep their real values: the wrapper needs PATH to exec anything, and a marker
		// HOME would only be a marker.
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

// An empty TMPDIR forwarded as `TMPDIR=` makes npx unpack into a path that is the empty string
// instead of falling back to /tmp. These cases tell the three apart by name, and never by value.
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

// The case reads one captured run, with its controls on the same run. A wrapper that died before
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
	// The row "a set variable arrives with its value" hands the child an LC_CTYPE and reads it back.
	// That row is what rules out a child dropping every LC_CTYPE it is given.
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

// That a command which succeeds is not reported as a failure is every other case here: childEnvIn,
// the helper they call, fails the case on any status but 0.
func TestTheCommandsExitStatusIsTheWrappers(t *testing.T) {
	t.Parallel()
	const want = 7
	out, code := launch(t, launchingEnv(), "sh", "-c", fmt.Sprintf("exit %d", want))
	if code != want {
		t.Errorf("a command exiting %d came back as %d — the client reads this status to decide "+
			"whether the server started\n%s", want, code, out)
	}
}

// --- the two arms that launch no command ---

// Every refusal here exits 2, so the code says only that a refusal happened. The wording is asserted
// as well, and it is what tells the two arms apart.
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
// claim about that file's content. A line inserted into the header ahead of the range silently makes
// them the wrong lines. Both ends are pinned by content: the header line the range has to start at,
// and the first line past it, which has to stay out.

// Both needles are read out of the wrapper, and never written out here. A needle spelled literally is
// a claim about prose, and prose gets reworded. The moment the wrapper's wording moves, a literal
// needle stops matching, and "the help lacks it" becomes true of every possible output. The case then
// passes forever while its subject goes unread.

// A needle read from the file follows the rewording, and the guards on the two needles fail when one
// reads as empty.
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

// The names the wrapper's own help says it passes, sorted. Two cases read them, and the help is one
// answer whoever asks, so the wrapper is launched for it once.
func promisedNames(t *testing.T) []string {
	t.Helper()
	promisedOnce.Do(func() {
		out, code := launch(t, launchingEnv(), "--help")
		if code != 0 {
			promisedRefusal = fmt.Sprintf("--help exited %d\n%s", code, out)
			return
		}
		for _, text := range strings.Split(out, "\n") {
			if names, held := strings.CutPrefix(text, "passes only: "); held {
				promised = sorted(strings.Fields(names))
			}
		}
	})
	if promisedRefusal != "" {
		t.Fatal(promisedRefusal)
	}
	return promised
}

var (
	promisedOnce    sync.Once
	promised        []string
	promisedRefusal string
)

// The nth non-blank comment line of the wrapper's header, past the shebang.
var headerLine = regexp.MustCompile(`^# ?(.+)$`)

func headerProse(t *testing.T, nth int) string {
	t.Helper()
	seen := 0
	for _, text := range strings.Split(runtest.ReadFile(t, wrapper), "\n")[1:] {
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

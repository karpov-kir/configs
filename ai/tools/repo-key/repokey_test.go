package repokey

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// The developer's own git config must not reach these fixtures.
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	os.Exit(m.Run())
}

// The directory name is the caller's because half these cases turn on what it is called.
func newRepo(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	run(t, dir, "init", "-q")
	run(t, dir, "config", "user.email", "t@t")
	run(t, dir, "config", "user.name", "t")
	run(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	run(t, dir, "add", "f.txt")
	run(t, dir, "commit", "-qm", "base")
	return dir
}

// A fixture that is a git directory and nothing more: `.git/HEAD`, the one file FromSharedGitDir
// probes for. It costs no subprocess where newRepo costs seven, and the cases that use it turn on what
// a key is MADE of rather than on git finding the clone. What keeps that from being a coverage loss is
// TestBothEntryPointsAgree, which holds resolveKey and FromSharedGitDir to one answer over a real
// repository — so the git-discovery half stays exercised where it belongs, once, instead of in every
// case that only needed a name to key.
func newBareRepo(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", gitDir, err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("write HEAD in %s: %v", gitDir, err)
	}
	return dir
}

// The key of a bare fixture, through the entry point that asks git nothing.
func bareKey(t *testing.T, dir string) string {
	t.Helper()
	k, err := FromSharedGitDir(filepath.Join(dir, ".git"))
	if err != nil {
		t.Fatalf("keying %s: %v", dir, err)
	}
	return k
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimRight(string(out), "\n")
}

func key(t *testing.T, dir string) string {
	t.Helper()
	k, err := resolveKey(dir)
	if err != nil {
		t.Fatalf("keying %s: %v", dir, err)
	}
	return k
}

// A worktree's own directory is NOT the clone, and `--show-toplevel` cannot tell the difference; only
// the shared git dir can.
func TestEveryWorktreeOfOneCloneKeysTheSame(t *testing.T) {
	t.Parallel()
	main := newRepo(t, "project")
	first := filepath.Join(filepath.Dir(main), "wt-one")
	second := filepath.Join(filepath.Dir(main), "wt-two")
	run(t, main, "worktree", "add", "-q", "-b", "one", first)
	run(t, main, "worktree", "add", "-q", "-b", "two", second)

	want := key(t, main)
	if got := key(t, first); got != want {
		t.Fatalf("worktree one keyed %s, the clone keys %s — each worktree would get a directory of its own", got, want)
	}
	if got := key(t, second); got != want {
		t.Fatalf("worktree two keyed %s, the clone keys %s — each worktree would get a directory of its own", got, want)
	}
	if !strings.HasPrefix(want, "project-") {
		t.Fatalf("key %s does not name the clone's directory, so nobody can read which repo it belongs to", want)
	}
}

// What rules out the remote URL. These two share a remote and differ only in where they sit, which is
// exactly the pair a URL-derived key collapses into one directory.
func TestTwoClonesOfOneRemoteKeyApart(t *testing.T) {
	t.Parallel()
	origin := newRepo(t, "origin")
	// Same basename, different parents: remote URL and directory name both identical, so only the
	// realpath can tell them apart.
	first := filepath.Join(t.TempDir(), "project")
	second := filepath.Join(t.TempDir(), "project")
	cloneInto(t, origin, first)
	cloneInto(t, origin, second)

	if filepath.Base(first) != filepath.Base(second) {
		t.Fatal("the two clones do not share a basename, so the digest could go untested")
	}

	if run(t, first, "remote", "get-url", "origin") != run(t, second, "remote", "get-url", "origin") {
		t.Fatal("the two clones do not share a remote, so this case would prove nothing about keying by URL")
	}
	if key(t, first) == key(t, second) {
		t.Fatalf("both clones keyed %s — they would write into one directory and each hold half the work", key(t, first))
	}
}

// Reaching one clone by a path that goes through a symlink is still that clone. Without the realpath,
// the same repository answers to two keys depending on how the caller happened to walk in.
func TestASymlinkedRouteToOneCloneKeysOnce(t *testing.T) {
	t.Parallel()
	direct := newRepo(t, "project")
	link := filepath.Join(t.TempDir(), "via-link")
	if err := os.Symlink(filepath.Dir(direct), link); err != nil {
		t.Skipf("cannot create a symlink here: %v", err)
	}
	if got, want := key(t, filepath.Join(link, "project")), key(t, direct); got != want {
		t.Fatalf("the symlinked route keyed %s and the direct one %s — one clone, two directories", got, want)
	}
}

// A path that resolves to nothing refuses. Falling back to the unresolved string would key one clone
// two ways, which is the failure the realpath is here to prevent.
func TestAnUnresolvablePathRefusesRatherThanFallingBack(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "gone", ".git")
	got, err := FromSharedGitDir(missing)
	if err == nil {
		t.Fatalf("keyed %s as %q instead of refusing", missing, got)
	}
	if got != "" {
		t.Fatalf("refused and still returned %q — a caller reading the value would use it", got)
	}
}

// A directory inside no repository refuses, rather than keying whatever git said on the way out.
func TestOutsideARepositoryItRefuses(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output(); err == nil {
		t.Skip("the temp dir sits inside a repository, so this case would prove nothing")
	}
	if got, err := resolveKey(dir); err == nil {
		t.Fatalf("keyed a directory in no repository as %q", got)
	}
}

// The two entry points are one algorithm: the command resolves git itself, a Go caller hands over the
// path it already holds. A drift between them is a clone with two names no single consumer can see.
func TestBothEntryPointsAgree(t *testing.T) {
	t.Parallel()
	dir := newRepo(t, "project")
	shared := run(t, dir, "rev-parse", "--git-common-dir")
	if !filepath.IsAbs(shared) {
		shared = filepath.Join(dir, shared)
	}
	direct, err := FromSharedGitDir(filepath.Clean(shared))
	if err != nil {
		t.Fatalf("keying the shared git dir: %v", err)
	}
	if got := key(t, dir); got != direct {
		t.Fatalf("resolveKey keyed %s and FromSharedGitDir %s — one clone, two names", got, direct)
	}
}

func cloneInto(t *testing.T, origin, target string) {
	t.Helper()
	out, err := exec.Command("git", "clone", "-q", origin, target).CombinedOutput()
	if err != nil {
		t.Fatalf("clone into %s: %v\n%s", target, err, out)
	}
	run(t, target, "config", "user.email", "t@t")
	run(t, target, "config", "user.name", "t")
}

// Not parallel: its subtests call t.Setenv, which Go bars under a parallel
// parent, and the fixtures here are built with the ambient environment that those subtests then
// change. Sequential, its body runs before any parallel case resumes, so nothing else is live while
// GIT_DIR points elsewhere.
//
// git reads its location from the environment before it looks at the directory it was run in, so an
// inherited variable answers for a clone the caller never named — a hook in a linked worktree runs with
// one set, and that is how a key for clone A ends up naming clone B's directories.
func TestNoInheritedVariableChoosesTheRepository(t *testing.T) {
	// Both fixtures are built before any variable is set: `git init` itself obeys them, so building
	// inside the loop would make the second case's own repository land wherever the first case pointed.
	mine := newRepo(t, "mine")
	other := newRepo(t, "other")
	want := key(t, mine)

	// The list is exactly what withoutGitLocation strips, so dropping any entry from it turns this red.
	// Keep the two in step: a variable stripped without a case here is a guard nothing can fail.
	for _, name := range []string{"GIT_DIR", "GIT_COMMON_DIR"} {
		// A subtest so t.Setenv unsets it again afterwards; set in the loop body it would still be live
		// for the next variable, and one case would then be proving another's point.
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, filepath.Join(other, ".git"))
			if got := key(t, mine); got != want {
				t.Errorf("with %s pointing at the other clone, %s keyed %s instead of %s — the caller's own repository was not the one answered for", name, mine, got, want)
			}
		})
	}
}

// What a key and a name are allowed to contain. Both are spliced into a path and a command line,
// and not every character a directory name can carry survives that.
var safeCharacters = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func TestAKeyIsSafeToSpliceIntoAPathOrACommand(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"a b", "-rf", "x$(id)", "a\tb", "a*b", ".hidden", "--", "%"} {
		dir := newBareRepo(t, name)
		got := bareKey(t, dir)
		if !safeCharacters.MatchString(got) {
			t.Errorf("directory %q keyed %q, which is not confined to characters that survive a path or a command line", name, got)
		}
		if strings.HasPrefix(got, "-") {
			t.Errorf("directory %q keyed %q, which reads as an option wherever the key reaches a command", name, got)
		}
		if got == "" {
			t.Errorf("directory %q keyed the empty string", name)
		}
	}
}

// Two clones whose names differ only in characters the safe half flattens still key apart: the digest
// carries the uniqueness, so nothing is lost by reducing the readable half.
func TestFlatteningTheNameNeverCollidesTwoClones(t *testing.T) {
	t.Parallel()
	first, second := newBareRepo(t, "a b"), newBareRepo(t, "a-b")
	if bareKey(t, first) == bareKey(t, second) {
		t.Fatalf("both clones keyed %s — flattening the readable half lost the distinction the digest should have kept", bareKey(t, first))
	}
}

// An ordinary name passes through untouched. Without this, tightening the safe half could silently
// rename every existing directory that a consumer has already created on disk.
func TestAnOrdinaryNameIsUnchanged(t *testing.T) {
	t.Parallel()
	dir := newBareRepo(t, "project-tracker")
	if got := bareKey(t, dir); !strings.HasPrefix(got, "project-tracker-") {
		t.Fatalf("keyed %s — an ordinary directory name must survive verbatim, or existing directories are renamed under their users", got)
	}
}

func TestAPathThatIsNotAGitDirRefuses(t *testing.T) {
	t.Parallel()
	dir := newBareRepo(t, "project")
	if got, err := FromSharedGitDir(dir); err == nil {
		t.Fatalf("the worktree root keyed as %q instead of refusing — a caller passing the wrong path gets a plausible answer", got)
	}
	if got, err := FromSharedGitDir(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf("the real git dir refused (%v), so the check rejects what it must accept", err)
	} else if got == "" {
		t.Fatal("the real git dir keyed empty")
	}
}

// A refusal echoes the path it was handed, and that path is whatever a directory is named. Control
// bytes in it drive the terminal an agent reads the result on: CSI 2 K then a carriage return erases
// the line and goes back to column 0, so what follows overwrites the error with anything the name chose.
//
// Every refusal that echoes a path is exercised, not just the first one reached. Each is a separate
// interpolation, so a case hitting one leaves the others free to be unwrapped with nothing turning red.
func TestARefusalCarriesNoControlBytesFromThePathItEchoes(t *testing.T) {
	t.Parallel()
	hostile := []string{"evil\x1b[2K\rALL CLEAR", "two\nlines", "bell\a", "csi\u009bm"}
	for _, name := range hostile {
		parent := t.TempDir()
		absent := filepath.Join(parent, name)
		notGit := filepath.Join(parent, "real-"+name)
		if err := os.MkdirAll(notGit, 0o755); err != nil {
			t.Fatalf("building the fixture for %q: %v", name, err)
		}
		refusals := map[string]error{}
		_, refusals["a path that resolves to nothing"] = FromSharedGitDir(absent)
		_, refusals["a directory that is not a git dir"] = FromSharedGitDir(notGit)
		_, refusals["a directory git will not answer for"] = resolveKey(notGit)

		for where, err := range refusals {
			if err == nil {
				t.Fatalf("%s did not refuse for %q, so that interpolation goes untested", where, name)
			}
			for _, b := range []byte(err.Error()) {
				if b < 0x20 || b == 0x7f {
					t.Errorf("%s: the refusal for %q carries byte %#x, which drives the terminal rather than printing", where, name, b)
					break
				}
			}
		}
	}
}

// The abbreviation is taken from the key's own readable half, not from a second reading of the same
// directory. Two derivations of one clone's name drift the moment either is touched, and sessions
// titled from the drifted one stop grouping with their siblings.
func TestTheAbbreviationIsTheKeysReadableHalfAbbreviated(t *testing.T) {
	t.Parallel()
	dir := newBareRepo(t, "issue-tracker")
	abbrev, err := abbrevFromSharedGitDir(filepath.Join(dir, ".git"))
	if err != nil {
		t.Fatalf("abbreviating %s: %v", dir, err)
	}
	key := bareKey(t, dir)
	if len(key) <= digestLength+1 {
		t.Fatalf("the clone keyed %q, which is too short to hold a readable half and a %d-character digest", key, digestLength)
	}
	name := key[:len(key)-digestLength-1]
	if want := abbrevOf(name); abbrev != want {
		t.Fatalf("the clone keys %q and abbreviates to %q, which is not %q — the two readings have drifted", key, abbrev, want)
	}
}

// Every row of the rule: the initial of each run, a run carrying a digit kept whole, and the cap on
// what reaches a title.
func TestTheAbbreviationTable(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, want string }{
		{"project-tracker-cache-cleanup", "PTCC"},
		{"issue-tracker", "IT"},
		{"github-action-deploy-k8s", "GADK8s"},
		{"billing-k8s", "BK8s"},
		{"configs", "C"},
		{"dashboard", "D"},
		{"my_repo", "MR"},
		{"open.api.spec", "OAS"},
		// A run is digit-carrying wherever the digit sits, so this one keeps its spelling as `k8s` does.
		{"2fa-tool", "2faT"},
		// A space separates two runs like any other non-alphanumeric byte.
		{"a b", "AB"},
		// A rune whose low byte is itself alphanumeric — `byte('\u0663')` is `'c'`. Only the range test
		// inside shell.IsAlnumRune keeps it a separator; without it `run[:1]` cuts the rune in two and
		// answers an invalid-UTF-8 byte. safeName never lets one this far, so this is the only case
		// that can see the guard at all.
		{"\u0663abc", "A"},
		// Nothing alphanumeric to take an initial from: `___` is left whole by safeName.
		{"___", "R"},
		// Eight runs is one more than a title takes.
		{"a-b-c-d-e-f-g-h", "ABCDEFG"},
		// A digit-carrying run spends the whole budget on its own.
		{"kubernetes123456-deploy", "Kuberne"},
	} {
		if got := abbrevOf(c.name); got != c.want {
			t.Errorf("%q abbreviates to %q, want %q", c.name, got, c.want)
		}
	}
}

// What the abbreviation is for: every session in one clone titles itself the same word, whichever
// worktree it stands in. `--show-toplevel` hands each worktree a name of its own, which is the
// inconsistency the tool exists to remove.
func TestEveryWorktreeOfOneCloneAbbreviatesTheSame(t *testing.T) {
	t.Parallel()
	main := newRepo(t, "issue-tracker")
	worktree := filepath.Join(filepath.Dir(main), "wt-one")
	run(t, main, "worktree", "add", "-q", "-b", "one", worktree)

	for _, where := range []string{main, worktree} {
		got, err := ResolveAbbrev(where)
		if err != nil {
			t.Fatalf("abbreviating %s: %v", where, err)
		}
		if got != "IT" {
			t.Errorf("%s abbreviates to %q, not the abbreviation of the clone's own directory name", where, got)
		}
	}
}

// An abbreviation refuses wherever a key does. A prefix is read by a human and spliced into a title,
// so a plausible one for a directory nobody meant is worse than none at all.
func TestAnAbbreviationRefusesWhereAKeyWould(t *testing.T) {
	t.Parallel()
	dir := newBareRepo(t, "project")
	got, err := abbrevFromSharedGitDir(dir)
	if err == nil {
		t.Fatalf("the worktree root abbreviated to %q instead of refusing — a caller passing the wrong path gets a plausible answer", got)
	}
	if got != "" {
		t.Fatalf("refused and still returned %q — a caller reading the value would use it", got)
	}
}

// The command's argument table: which answer each invocation selects, and which are refused. A
// refusal carries the usage line only where the invocation itself was malformed — a path that will not
// resolve came from a caller with nothing to fix in their command line.
func TestTheCommandsArgumentTable(t *testing.T) {
	t.Parallel()
	dir := newRepo(t, "project")
	key, err := resolveKey(dir)
	if err != nil {
		t.Fatalf("keying %s: %v", dir, err)
	}

	for _, c := range []struct {
		what   string
		args   []string
		status int
		want   string
	}{
		{"a path alone prints the key", []string{dir}, 0, key},
		{"--abbrev before the path prints the abbreviation", []string{"--abbrev", dir}, 0, "P"},
		{"two paths are refused", []string{dir, dir}, 2, usage},
		{"--abbrev with two paths is refused", []string{"--abbrev", dir, dir}, 2, usage},
		{"a flag after the path is refused", []string{dir, "--abbrev"}, 2, usage},
		{"an unreadable path refuses without the usage line", []string{filepath.Join(dir, "nowhere")}, 2, ""},
		{"a dash-leading argument is a path, not a flag", []string{"-rf"}, 2, ""},
	} {
		var out, errOut bytes.Buffer
		if status := Run(c.args, &out, &errOut); status != c.status {
			t.Errorf("%s: exited %d, want %d\nstdout: %s\nstderr: %s", c.what, status, c.status, out.String(), errOut.String())
			continue
		}
		if c.status == 0 {
			if got := strings.TrimRight(out.String(), "\n"); got != c.want {
				t.Errorf("%s: printed %q, want %q", c.what, got, c.want)
			}
			continue
		}
		if out.Len() != 0 {
			t.Errorf("%s: refused and still wrote %q to stdout, which a caller reads as an answer", c.what, out.String())
		}
		if c.want != "" && !strings.Contains(errOut.String(), c.want) {
			t.Errorf("%s: refused with %q, which does not carry %q", c.what, errOut.String(), c.want)
		}
		if errOut.Len() == 0 {
			t.Errorf("%s: refused in silence, and a tool whose contract is to refuse loudly must say why", c.what)
		}
		if c.want == "" && strings.Contains(errOut.String(), usage) {
			t.Errorf("%s: answered a resolution failure with the usage line, which sends the caller to fix a sound invocation", c.what)
		}
	}
}

// The invocation with no path at all — the one branch the argument table cannot reach, since every
// row of it supplies a path.
//
// Not parallel: t.Chdir is barred under a parallel test, and a process-wide chdir would be live under
// any case running beside it. Sequential, its body runs before any parallel case resumes, exactly as
// TestNoInheritedVariableChoosesTheRepository relies on for the environment.
func TestWithNoPathItAnswersForTheWorkingDirectory(t *testing.T) {
	dir := newRepo(t, "project")
	key, err := resolveKey(dir)
	if err != nil {
		t.Fatalf("keying %s: %v", dir, err)
	}
	t.Chdir(dir)

	for _, c := range []struct {
		what string
		args []string
		want string
	}{
		{"no argument at all", nil, key},
		{"--abbrev and nothing else", []string{"--abbrev"}, "P"},
	} {
		var out, errOut bytes.Buffer
		if status := Run(c.args, &out, &errOut); status != 0 {
			t.Errorf("%s: exited %d\n%s", c.what, status, errOut.String())
			continue
		}
		if got := strings.TrimRight(out.String(), "\n"); got != c.want {
			t.Errorf("%s: printed %q, want %q", c.what, got, c.want)
		}
	}
}

// The abbreviation is spliced into a session title and, through the stub, into whatever command line
// a caller builds around it. Asserted through its own entry point rather than through the key: an
// abbreviation re-derived on its own stops being covered by the key's table, and nothing turns red
// when it does.
func TestAnAbbreviationIsSafeToSpliceIntoAPathOrACommand(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"a b", "-rf", "x$(id)", "a\tb", "a*b", ".hidden", "--", "%"} {
		dir := newBareRepo(t, name)
		got, err := abbrevFromSharedGitDir(filepath.Join(dir, ".git"))
		if err != nil {
			t.Fatalf("abbreviating %s: %v", dir, err)
		}
		if !safeCharacters.MatchString(got) {
			t.Errorf("directory %q abbreviated to %q, which is not confined to characters that survive a path or a command line", name, got)
		}
		if strings.HasPrefix(got, "-") {
			t.Errorf("directory %q abbreviated to %q, which reads as an option wherever the abbreviation reaches a command", name, got)
		}
	}
}

// The eight-name case above proves the reduction on the shapes a person types. This one proves the
// class, because the class is what safeName promises and safeName no longer spells it out: the
// alphanumeric half is shell.IsAlnumRune now, and widening that one predicate in a general-purpose
// text package would put a `;` or a backtick into a key — spliced into a path and into the `git
// worktree add` line a human runs — with nothing in this package saying so. A directory name may
// hold every byte but NUL and `/`, so every one of them is a case.
func TestSafeNameAdmitsNoByteOutsideTheClassItPromises(t *testing.T) {
	t.Parallel()
	for i := 1; i < 0x100; i++ {
		if byte(i) == '/' {
			continue
		}
		b := string(byte(i))
		for _, name := range []string{b, "a" + b + "z", b + "tail", "head" + b, "a" + b + "1"} {
			got := safeName(name)
			if !safeCharacters.MatchString(got) {
				t.Errorf("directory %q reduced to %q, which is not confined to characters that survive a path or a command line", name, got)
			}
			if strings.HasPrefix(got, "-") {
				t.Errorf("directory %q reduced to %q, which reads as an option wherever the name reaches a command", name, got)
			}
			if strings.HasPrefix(got, ".") {
				t.Errorf("directory %q reduced to %q, which names a hidden directory", name, got)
			}
		}
	}
	// A rune the range loop yields as several bytes, and a byte sequence that is not UTF-8 at all.
	// `\u0663` and `\u0130` are the discriminating ones: `byte(r)` truncates them onto `'c'` and `'0'`,
	// so only the range test inside shell.IsAlnumRune keeps them out of the name — the rest of this list
	// truncates onto bytes that are not alphanumeric and cannot see that half at all.
	for _, name := range []string{"café", "日本", "\U0001F600", "\xff\xfe", "\xed\xa0\x80", "..", "\u0663abc", "\u0130stanbul-tools"} {
		if got := safeName(name); !safeCharacters.MatchString(got) || strings.HasPrefix(got, "-") || strings.HasPrefix(got, ".") {
			t.Errorf("directory %q reduced to %q", name, got)
		}
	}
}

// The abbreviation is spliced into a session title and compared against a written-down prefix, and
// initialsOf takes `run[:1]` off a run isSeparator bounded — so a run holding anything but an ASCII
// alphanumeric would both widen the answer and cut a multi-byte rune in half.
func TestAnAbbreviationAdmitsNoByteOutsideTheClassItPromises(t *testing.T) {
	t.Parallel()
	alnumOnly := regexp.MustCompile(`^[A-Za-z0-9]*$`)
	for i := 1; i < 0x100; i++ {
		if byte(i) == '/' {
			continue
		}
		b := string(byte(i))
		for _, name := range []string{b, "a" + b + "z", "k8s" + b, b + "v2"} {
			if got := abbrevOf(safeName(name)); !alnumOnly.MatchString(got) {
				t.Errorf("directory %q abbreviated to %q, which is not confined to characters that survive a path or a command line", name, got)
			}
		}
	}
	for _, name := range []string{"café", "日本", "\xff\xfe", "a;rm -rf /", "$(id)"} {
		if got := abbrevOf(safeName(name)); !alnumOnly.MatchString(got) {
			t.Errorf("directory %q abbreviated to %q", name, got)
		}
	}
}

package repokey

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"configs/ai/tools/repo"
	"configs/ai/tools/repo/repotest"
)

// No case here forks git. What this package does with a shared git dir is the whole of it, and where
// that path comes from is `repo.Exec`'s answer. `repo/exec_test.go` holds that answer to a real
// repository, including the property these cases used to build a linked worktree for: a worktree's
// common dir is its clone's.

// A fixture is therefore a directory holding `.git/HEAD`, the single file FromSharedGitDir probes
// for. The `repotest.Fake` that built it answers where the entry point asks git, naming `Root/.git`
// for whatever directory it is asked about. A real git gives that same answer from anywhere inside
// that clone, linked worktrees included.
func newBareRepo(t *testing.T, name string) *repotest.Fake {
	t.Helper()
	git := repotest.New(filepath.Join(t.TempDir(), name))
	if err := git.OnDisk(); err != nil {
		t.Fatalf("building the fixture repository %q: %v", name, err)
	}
	return git
}

// The key of a bare fixture, through the entry point that asks git nothing.
func bareKey(t *testing.T, git *repotest.Fake) string {
	t.Helper()
	k, err := FromSharedGitDir(git.Git)
	if err != nil {
		t.Fatalf("keying %s: %v", git.Root, err)
	}
	return k
}

func key(t *testing.T, git *repotest.Fake) string {
	t.Helper()
	k, err := resolveKey(git, git.Root)
	if err != nil {
		t.Fatalf("keying %s: %v", git.Root, err)
	}
	return k
}

// `repo/exec_test.go` shows a real linked worktree answering its clone's common dir, and the pair
// covers what one real-repository case used to.

// A worktree's own directory is not the clone, and `--show-toplevel` cannot tell the difference. Only
// the shared git dir can. Held here as: whatever directory this is asked about, the key comes off the
// common dir alone.
func TestEveryWorktreeOfOneCloneKeysTheSame(t *testing.T) {
	t.Parallel()
	git := newBareRepo(t, "project")
	clone := git.Root

	want := key(t, git)
	for _, where := range []string{clone, filepath.Join(filepath.Dir(clone), "wt-one"), "/somewhere/else"} {
		got, err := resolveKey(git, where)
		if err != nil {
			t.Fatalf("keying from %s: %v", where, err)
		}
		if got != want {
			t.Fatalf("asked from %s the key is %s, and from the clone itself %s — each worktree would "+
				"get a directory of its own", where, got, want)
		}
	}
	if !strings.HasPrefix(want, "project-") {
		t.Fatalf("key %s does not name the clone's directory, so nobody can read which repo it belongs to", want)
	}
}

// What rules out the directory name, and with it the remote URL two clones of one repository share.
// Same basename, different parents: only the realpath tells these apart.
func TestTwoClonesOfOneRemoteKeyApart(t *testing.T) {
	t.Parallel()
	first := newBareRepo(t, "project")
	second := newBareRepo(t, "project")

	if filepath.Base(first.Root) != filepath.Base(second.Root) {
		t.Fatal("the two clones do not share a basename, so the digest could go untested")
	}
	if bareKey(t, first) == bareKey(t, second) {
		t.Fatalf("both clones keyed %s — they would write into one directory and each hold half the work", bareKey(t, first))
	}
}

// Reaching one clone by a path that goes through a symlink is still that clone. Without the realpath,
// the same repository answers to two keys depending on how the caller happened to walk in.
func TestASymlinkedRouteToOneCloneKeysOnce(t *testing.T) {
	t.Parallel()
	direct := newBareRepo(t, "project")
	link := filepath.Join(t.TempDir(), "via-link")
	if err := os.Symlink(filepath.Dir(direct.Root), link); err != nil {
		t.Skipf("cannot create a symlink here: %v", err)
	}
	viaLink, err := FromSharedGitDir(filepath.Join(link, "project", ".git"))
	if err != nil {
		t.Fatalf("keying through the symlink: %v", err)
	}
	if want := bareKey(t, direct); viaLink != want {
		t.Fatalf("the symlinked route keyed %s and the direct one %s — one clone, two directories", viaLink, want)
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

// A directory git cannot answer for refuses, and never keys whatever came back on the way out.
func TestOutsideARepositoryItRefuses(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	git := repotest.New(dir)
	git.Fail["CommonDir"] = errors.New("not a git repository")
	if got, err := resolveKey(git, dir); err == nil {
		t.Fatalf("keyed a directory in no repository as %q", got)
	}
}

// The two entry points are one algorithm: the command asks git where the shared dir is, and a Go
// caller hands over the path it already holds. Where they diverge, one clone gets two keys, and no
// single consumer can see both.
func TestBothEntryPointsAgree(t *testing.T) {
	t.Parallel()
	git := newBareRepo(t, "project")
	direct, err := FromSharedGitDir(git.Git)
	if err != nil {
		t.Fatalf("keying the shared git dir: %v", err)
	}
	if got := key(t, git); got != direct {
		t.Fatalf("resolveKey keyed %s and FromSharedGitDir %s — one clone, two names", got, direct)
	}
}

// git reads its location from the environment before it looks at the directory it was run in, so an
// inherited variable answers for a clone the caller never named — a hook in a linked worktree runs with
// one set, and that is how a key for clone A ends up naming clone B's directories. This command is the
// reason `repo.WithoutGitLocation` exists, so what is held here is that the command asks through it.
// That the stripping works is `repo/exec_test.go`'s, against a real git.
func TestTheCommandStripsTheVariablesThatRelocateGit(t *testing.T) {
	t.Parallel()
	stray := []string{"GIT_DIR=/elsewhere/.git", "GIT_COMMON_DIR=/elsewhere/.git", "PATH=/usr/bin"}
	kept := repo.WithoutGitLocation(stray)
	if len(kept) != 1 || kept[0] != "PATH=/usr/bin" {
		t.Fatalf("the port kept %v of %v — this command would key the repository a caller's environment "+
			"named rather than the one it was asked about", kept, stray)
	}
	if _, ok := CommandGit().(repo.Exec); !ok {
		t.Fatalf("CommandGit no longer hands back the exec adapter, so nothing here says which "+
			"environment the command's own git runs under: %T", CommandGit())
	}
}

// What a key and a name are allowed to contain. Both are spliced into a path and a command line,
// and not every character a directory name can carry survives that.
var safeCharacters = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// The abbreviation is weighed beside the key, inside the loop that already walks the same clone. It
// is asserted through its own entry point. An abbreviation re-derived on its own stops being covered
// by the key's table, and the suite stays green when it breaks. A separate loop would build the same
// eight directories a second time and prove what this pass already proves.
func TestAKeyAndItsAbbreviationAreSafeToSpliceIntoAPathOrACommand(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"a b", "-rf", "x$(id)", "a\tb", "a*b", ".hidden", "--", "%"} {
		git := newBareRepo(t, name)
		got := bareKey(t, git)
		if !safeCharacters.MatchString(got) {
			t.Errorf("directory %q keyed %q, which is not confined to characters that survive a path or a command line", name, got)
		}
		if strings.HasPrefix(got, "-") {
			t.Errorf("directory %q keyed %q, which reads as an option wherever the key reaches a command", name, got)
		}
		if got == "" {
			t.Errorf("directory %q keyed the empty string", name)
		}

		abbrev, err := abbrevFromSharedGitDir(git.Git)
		if err != nil {
			t.Fatalf("abbreviating %s: %v", git.Root, err)
		}
		if !safeCharacters.MatchString(abbrev) {
			t.Errorf("directory %q abbreviated to %q, which is not confined to characters that survive a path or a command line", name, abbrev)
		}
		if strings.HasPrefix(abbrev, "-") {
			t.Errorf("directory %q abbreviated to %q, which reads as an option wherever the abbreviation reaches a command", name, abbrev)
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
	git := newBareRepo(t, "player-testing")
	if got := bareKey(t, git); !strings.HasPrefix(got, "player-testing-") {
		t.Fatalf("keyed %s — an ordinary directory name must survive verbatim, or existing directories are renamed under their users", got)
	}
}

func TestAPathThatIsNotAGitDirRefuses(t *testing.T) {
	t.Parallel()
	git := newBareRepo(t, "project")
	if got, err := FromSharedGitDir(git.Root); err == nil {
		t.Fatalf("the worktree root keyed as %q instead of refusing — a caller passing the wrong path gets a plausible answer", got)
	}
	if got, err := FromSharedGitDir(git.Git); err != nil {
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
		refusingGit := repotest.New(notGit)
		refusingGit.Fail["CommonDir"] = errors.New("not a git repository")
		_, refusals["a directory git will not answer for"] = resolveKey(refusingGit, notGit)

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
	git := newBareRepo(t, "issue-tracker")
	abbrev, err := abbrevFromSharedGitDir(git.Git)
	if err != nil {
		t.Fatalf("abbreviating %s: %v", git.Root, err)
	}
	key := bareKey(t, git)
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
		{"player-testing-codec-compatibility", "PTCC"},
		{"issue-tracker", "IT"},
		{"github-action-deploy-k8s", "GADK8s"},
		{"bitmovin-k8s", "BK8s"},
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
	git := newBareRepo(t, "issue-tracker")
	worktree := filepath.Join(filepath.Dir(git.Root), "wt-one")

	for _, where := range []string{git.Root, worktree} {
		got, err := ResolveAbbrev(git, where)
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
	git := newBareRepo(t, "project")
	got, err := abbrevFromSharedGitDir(git.Root)
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
	git := newBareRepo(t, "project")
	dir := git.Root
	key, err := resolveKey(git, dir)
	if err != nil {
		t.Fatalf("keying %s: %v", dir, err)
	}
	// The single row that has to refuse on resolution, and not on its arguments. The fake answers the
	// clone's git dir whatever it is asked about, and a row naming an unreadable path would still key.
	// A port that cannot answer at all is what that row needs.
	refusing := repotest.New(dir)
	refusing.Fail["CommonDir"] = errors.New("not a git repository")

	for _, c := range []struct {
		what   string
		args   []string
		git    repo.Git
		status int
		want   string
	}{
		{"a path alone prints the key", []string{dir}, git, 0, key},
		{"--abbrev before the path prints the abbreviation", []string{"--abbrev", dir}, git, 0, "P"},
		{"two paths are refused", []string{dir, dir}, git, 2, usage},
		{"--abbrev with two paths is refused", []string{"--abbrev", dir, dir}, git, 2, usage},
		{"a flag after the path is refused", []string{dir, "--abbrev"}, git, 2, usage},
		{"an unreadable path refuses without the usage line", []string{filepath.Join(dir, "nowhere")}, refusing, 2, ""},
		{"a dash-leading argument is a path, not a flag", []string{"-rf"}, refusing, 2, ""},
	} {
		var out, errOut bytes.Buffer
		if status := Run(c.args, c.git, &out, &errOut); status != c.status {
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
// row of it supplies a path. What it turns on is that "." reaches the port as the root, so the fake
// records what it was asked.
func TestWithNoPathItAnswersForTheWorkingDirectory(t *testing.T) {
	t.Parallel()
	git := newBareRepo(t, "project")
	key, err := resolveKey(git, git.Root)
	if err != nil {
		t.Fatalf("keying %s: %v", git.Root, err)
	}

	for _, c := range []struct {
		what string
		args []string
		want string
	}{
		{"no argument at all", nil, key},
		{"--abbrev and nothing else", []string{"--abbrev"}, "P"},
	} {
		var out, errOut bytes.Buffer
		if status := Run(c.args, git, &out, &errOut); status != 0 {
			t.Errorf("%s: exited %d\n%s", c.what, status, errOut.String())
			continue
		}
		if got := strings.TrimRight(out.String(), "\n"); got != c.want {
			t.Errorf("%s: printed %q, want %q", c.what, got, c.want)
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

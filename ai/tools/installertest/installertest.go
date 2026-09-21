// A fixture for an installer writes the same things the installer does, files, directories and
// symlinks under a home and a checkout. One of those writes once followed a live symlink out of the
// case's own tree and overwrote real config files in the working tree. Every suite grew the same
// guard afterwards, and four copies of a guard are four chances for one to guard something slightly
// different.

// The bound is the production one, by construction. `installer.tree` refuses a write whose nearest
// existing parent falls outside its write root. Tree refuses a fixture write by calling the same two
// functions, shell.NearestExistingParent and shell.IsWithin.

// The copies this replaces walked up with filepath.EvalSymlinks instead, and that stops at a regular
// file, a name under which no write can land. A fixture stopping there judged a write by a boundary
// the code under test lacks.

// Every refusal here is a GUARD. It says the case was about to write somewhere it never meant to,
// and it fails the case on the spot. A result would let the write happen and the assertion pass.

// Package installertest is the tree the installer suites build their fixtures in, and the read-backs
// they make against it afterwards. Both halves live here because the suites sharing the tree shared
// byte-identical copies of the assertions too. What a run printed is ai/tools/runtest's, since a
// suite with no installer in it reads a run back the same way.
package installertest

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"configs/ai/tools/machine"
	"configs/ai/tools/machine/fake"
	"configs/ai/tools/shell"
)

// Tree is a case's own directory: everything a fixture writes into it, and everything a case asks of
// it afterwards. It refuses any write that would land outside.
type Tree struct {
	t    *testing.T
	base string
}

// New is a tree over a temp directory of the case's own.
func New(t *testing.T) *Tree {
	t.Helper()
	return NewIn(t, t.TempDir())
}

// NewIn is a tree over a directory the caller already has. That fits a suite whose fixture root is
// built before the tree is: a checkout it was pointed at, or a temp directory it laid out itself.
// The directory has to exist, because the bound is its resolved path, and an unresolvable one
// contains no path.

// The path is resolved physically, because t.TempDir hands back /var/folders/… on macOS and /var is
// itself a symlink to /private/var. An unresolved bound refuses every write made through a resolved
// path, and a guard that always fires is a guard somebody deletes.
func NewIn(t *testing.T, dir string) *Tree {
	t.Helper()
	return &Tree{t: t, base: Physical(t, dir)}
}

// Base is the tree this one bounds. A case hands the same tree to the run under test as its own
// write root. A run that escapes and a fixture that escapes are then caught by the same line.
func (w *Tree) Base() string { return w.base }

// The parent is what gets checked, because each writer creates the missing directories under it.
// That ancestor is the deepest point a symlink can still redirect. The check leaves the LAST
// component unexamined, and that is why every writer here asks RefuseExistingSymlink as well.

// ContainedParent refuses a path whose nearest existing parent is outside this tree.
func (w *Tree) ContainedParent(path string) {
	w.t.Helper()
	parent := shell.NearestExistingParent(path)
	if parent == "" {
		w.t.Fatalf("refusing to write %s — no directory above it resolves\n"+
			"this is the containment guard, not a failing case", path)
	}
	if !shell.IsWithin(parent, w.base) {
		w.t.Fatalf("refusing to write %s — its nearest existing parent resolves to %s, outside %s\n"+
			"this is the containment guard, not a failing case", path, parent, w.base)
	}
}

// RefuseExistingSymlink turns away a write at a name that is already a link. A write follows a
// symlink, and the links these cases produce point into a checkout. A run leaves $home/.zshrc
// pointing at $repo/zsh/.zshrc, and a fixture write at that path afterwards lands in the real file.
// That is the single door ContainedParent leaves open.
func (w *Tree) RefuseExistingSymlink(path string) {
	w.t.Helper()
	if value, err := os.Readlink(path); err == nil {
		w.t.Fatalf("refusing to write %s — it already exists as a symlink to %s\n"+
			"this is the containment guard, not a failing case", path, value)
	}
}

func (w *Tree) MkdirAll(dir string) {
	w.t.Helper()
	w.ContainedParent(dir)
	// A contained parent leaves dir itself unexamined, and MkdirAll follows a symlink there the same
	// way a file write does.
	w.RefuseExistingSymlink(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		w.t.Fatalf("the fixture could not create %s: %v", dir, err)
	}
}

func (w *Tree) Write(path, body string) {
	w.t.Helper()
	w.WriteMode(path, body, 0o644)
}

// WriteMode is Write with the mode named, for a fixture whose subject is the mode. Such a fixture
// is a launcher the run has to find executable, or a hook git has to be able to fire. The mode is
// set afterwards as well, because os.WriteFile leaves an existing file's own mode alone.
func (w *Tree) WriteMode(path, body string, mode os.FileMode) {
	w.t.Helper()
	w.ContainedParent(path)
	w.RefuseExistingSymlink(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		w.t.Fatalf("the fixture could not create the parent of %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		w.t.Fatalf("the fixture could not write %s: %v", path, err)
	}
	if err := os.Chmod(path, mode); err != nil {
		w.t.Fatalf("the fixture could not set the mode of %s: %v", path, err)
	}
}

// Symlink refuses a name already taken by a link instead of forcing past it. `ln -s X Y` with Y
// already a symlink to a directory creates the link INSIDE Y, and that is how a stray link ends up
// in a checkout. Every fixture link is meant to be the first thing at its path.
func (w *Tree) Symlink(source, target string) {
	w.t.Helper()
	w.ContainedParent(target)
	w.RefuseExistingSymlink(target)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		w.t.Fatalf("the fixture could not create the parent of %s: %v", target, err)
	}
	if err := os.Symlink(source, target); err != nil {
		w.t.Fatalf("the fixture could not link %s at %s: %v", target, source, err)
	}
}

// Hardlink is a second name for one file, so both ends are bound. An out-of-tree source would give an
// in-tree name for a file outside the tree, and every later write through that name lands outside it.
func (w *Tree) Hardlink(source, target string) {
	w.t.Helper()
	w.ContainedParent(source)
	w.ContainedParent(target)
	w.RefuseExistingSymlink(target)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		w.t.Fatalf("the fixture could not create the parent of %s: %v", target, err)
	}
	if err := os.Link(source, target); err != nil {
		w.t.Fatalf("the fixture could not hard link %s at %s: %v", target, source, err)
	}
}

func (w *Tree) RemoveAll(path string) {
	w.t.Helper()
	w.ContainedParent(path)
	if err := os.RemoveAll(path); err != nil {
		w.t.Fatalf("the fixture could not remove %s: %v", path, err)
	}
}

// RemoveLink drops a fixture link so another can take its place. It goes through the containment
// guard, because RefuseExistingSymlink, one of its checks, deliberately leaves an existing symlink
// alone. That refusal is a rule, and reaching for os directly would work around it.
func (w *Tree) RemoveLink(path string) {
	w.t.Helper()
	w.ContainedParent(path)
	if err := os.Remove(path); err != nil {
		w.t.Fatalf("the fixture could not drop %s: %v", path, err)
	}
}

// NewSkill writes a skill in the shape an installer discovers: a SKILL.md under
// <root>/kk-flavor/skills/<name>/. audience is the frontmatter line the tier cases turn on, empty for
// a skill every install takes.
func (w *Tree) NewSkill(root, name, audience string) {
	w.t.Helper()
	w.Write(root+"/kk-flavor/skills/"+name+"/SKILL.md",
		"---\nname: "+name+"\ndescription: a skill\n"+audience+"---\n")
}

// CloseToNewFiles leaves a directory this process cannot create a file in. The tree probes it, since
// root ignores the mode bits and a filesystem can drop them. Where the probe still writes, the case
// is skipped with that reason: a case asserting against a writable directory passes for a reason
// other than its name. ~/.kk-flavor/standards/testing.md asks that of a fixture that denies access.
func (w *Tree) CloseToNewFiles(directory string) {
	w.t.Helper()
	w.ContainedParent(directory)
	if err := os.Chmod(directory, 0o555); err != nil {
		w.t.Fatalf("the fixture could not close %s: %v", directory, err)
	}
	w.t.Cleanup(func() { os.Chmod(directory, 0o755) })
	probe := filepath.Join(directory, ".can-this-process-still-write")
	if file, err := os.Create(probe); err == nil {
		file.Close()
		os.Remove(probe)
		w.t.Skip("this process writes into a directory with no write bit — CAP_DAC_OVERRIDE, root, or a " +
			"filesystem that drops the bit — so the refusal this case names could not be built")
	}
}

// Physical resolves a directory the way every bound here is resolved. It is exported for a case with
// a second tree of its own to resolve, such as a linked worktree or a checkout the run is pointed
// at.
func Physical(t *testing.T, dir string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("the case's own temp directory does not resolve, so nothing could be contained: %v", err)
	}
	return resolved
}

// --- what a case asks of the tree afterwards ------------------------------------------------------

func (w *Tree) Read(path string) string {
	w.t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		w.t.Fatalf("the case could not read %s: %v", path, err)
	}
	return string(body)
}

// Body, Exists and IsFile ANSWER. Read and the Expect family decide for the case that a missing
// file is a failure. That is the wrong call wherever absence is one of the outcomes a case is
// comparing, and these three hand the question back.

// Body is a file's contents, and whether there was a file to read. The second value is what separates
// an empty file from a missing one, which "" alone cannot.
func (w *Tree) Body(path string) (string, bool) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(body), true
}

// Exists is whether anything is at the path. Lstat, so a symlink that resolves nowhere still counts:
// it is an entry, and it is what a half-done removal leaves behind.
func (w *Tree) Exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// IsFile is whether the path is something a reader can read as a file. Stat, so a symlink to a
// regular file counts — what is at the name matters here, not how it was reached. That is the
// deliberate difference from Exists.
func (w *Tree) IsFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// ExpectNoBreach fails the case where the run reached for a path outside this tree. It is a guard
// and never a result. The run under test is bounded to the same tree the fixture writes in, so a
// breach says the case was about to write somewhere it never meant to.
func (w *Tree) ExpectNoBreach(breaches []string) {
	w.t.Helper()
	if len(breaches) > 0 {
		w.t.Fatalf("the run went for a path outside %s — %s\n"+
			"this is the containment guard, not a failing case", w.base, strings.Join(breaches, "; "))
	}
}

func (w *Tree) ExpectLinkTo(target, want string) {
	w.t.Helper()
	value, err := os.Readlink(target)
	if err != nil {
		w.t.Errorf("%s is not a symlink, so it was never mounted: %v", target, err)
		return
	}
	if value != want {
		w.t.Errorf("%s -> %s, wanted %s", target, value, want)
	}
}

// ExpectSymlink is a link at the target, wherever it points — for a case whose whole subject is that
// a link survived a run that removes some of them.
func (w *Tree) ExpectSymlink(target string) {
	w.t.Helper()
	if _, err := os.Readlink(target); err != nil {
		w.t.Errorf("%s is not a symlink any more, so the run took it: %v", target, err)
	}
}

func (w *Tree) ExpectNotSymlink(path string) {
	w.t.Helper()
	if value, err := os.Readlink(path); err == nil {
		w.t.Errorf("%s is a symlink to %s, so what was there was replaced", path, value)
	}
}

// ExpectAbsent is an empty path, with no entry of any kind. Lstat, because Stat follows the link and
// answers "not there" for one that dangles. That is the shape a half-done removal leaves behind, and
// these cases assert about it.
func (w *Tree) ExpectAbsent(path string) {
	w.t.Helper()
	if info, err := os.Lstat(path); err == nil {
		w.t.Errorf("%s is still there (%s)", path, info.Mode())
	}
}

func (w *Tree) ExpectDir(path string) {
	w.t.Helper()
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		w.t.Errorf("%s is not a directory, so the run never created it: %v", path, err)
	}
}

func (w *Tree) ExpectFileBody(path, want string) {
	w.t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		w.t.Errorf("%s could not be read, so what was in it did not survive: %v", path, err)
		return
	}
	if string(got) != want {
		w.t.Errorf("%s holds %q, wanted %q", path, got, want)
	}
}

func (w *Tree) ExpectFileContains(path, want string) {
	w.t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		w.t.Errorf("%s could not be read: %v", path, err)
		return
	}
	if !strings.Contains(string(got), want) {
		w.t.Errorf("%s does not carry %q. It holds:\n%s", path, want, got)
	}
}

// A file this cannot read fails the case. "The region was removed" and "the file is gone" are two
// different outcomes, and an uninstall that deleted the human's file outright used to pass here.
func (w *Tree) ExpectFileLacks(path, unwanted string) {
	w.t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		w.t.Errorf("%s could not be read (%v), so this case cannot tell a trimmed file from a deleted one",
			path, err)
		return
	}
	if strings.Contains(string(got), unwanted) {
		w.t.Errorf("%s still carries %q. It holds:\n%s", path, unwanted, got)
	}
}

// Mounted is every symlink directly under a directory, by name.
func (w *Tree) Mounted(directory string) []string {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if _, err := os.Readlink(filepath.Join(directory, entry.Name())); err == nil {
			names = append(names, entry.Name())
		}
	}
	return names
}

// --- brew, as a working machine -------------------------------------------------------------------

// BrewMachine is the shared fake machine with brew's own behaviour on top: a `list` answers non-zero
// until the matching `install` has run. Canned codes could not express that, and installed-first is
// the branch these installers spend most of their lines on.

// Every record is keyed "<kind> <name>", because a formula and a cask of the same name are different
// packages to brew. `brew list --formula ghostty` answers non-zero for an installed cask, and a run
// without the flag reinstalls it on every pass.
type BrewMachine struct {
	*fake.Machine
	// Installed is what this machine already has. A case sets an entry to drive the installed-first
	// branch.
	Installed map[string]bool
	// The Failing map holds the packages whose install answers non-zero.
	Failing map[string]bool
	// Installs is every install this run asked for, in order.
	Installs []string
}

func NewBrewMachine() *BrewMachine {
	brew := &BrewMachine{Machine: fake.New(), Installed: map[string]bool{}, Failing: map[string]bool{}}
	brew.Answering("brew", brew.answer)
	return brew
}

func (b *BrewMachine) answer(command machine.Command) int {
	if len(command.Args) < 2 {
		return 1
	}
	key := brewKey(command.Args)
	if command.Args[0] == "list" {
		if b.Installed[key] {
			return 0
		}
		return 1
	}
	b.Installs = append(b.Installs, key)
	if b.Failing[key] {
		return 1
	}
	b.Installed[key] = true
	return 0
}

// The `--cask` flag on the command line is what separates a cask from a formula, and it is the same
// flag machine.PackageArguments writes.
func brewKey(args []string) string {
	kind := "formula"
	for _, arg := range args {
		if arg == "--cask" {
			kind = "cask"
		}
	}
	return kind + " " + args[len(args)-1]
}

// Without takes commands off this machine, which is what drives every "X is not installed" refusal.
func (b *BrewMachine) Without(names ...string) {
	for _, name := range names {
		b.Present[name] = false
	}
}

// --- reading a usage line -------------------------------------------------------------------------

// The flags a usage line names. `--agent=claude|codex` selects between two values, so both of its
// spellings come back.
var flagPattern = regexp.MustCompile(`--[a-z][a-z-]*(?:=[a-z|]+)?`)

// FlagsIn is every flag a usage line names, sorted and deduplicated, for the case holding a parser
// against its own printed grammar.
func FlagsIn(line string) []string {
	var found []string
	for _, match := range flagPattern.FindAllString(line, -1) {
		if name, choices, isSelector := strings.Cut(match, "="); isSelector {
			for _, choice := range strings.Split(choices, "|") {
				found = append(found, name+"="+choice)
			}
			continue
		}
		found = append(found, match)
	}
	slices.Sort(found)
	return slices.Compact(found)
}

package shell_test

import (
	"os"
	"path/filepath"
	"testing"

	"configs/ai/tools/shell"
)

// A checkout reached through the bucket link, which is the shape every stub is invoked in. The
// installers mount `~/.kk-flavor` at a checkout's `ai/kk-flavor`, and the sibling scripts sit one
// level above it. Answers the base, so a case can build the path it means and the answer it wants
// from the same directory.
func checkoutThroughABucket(t *testing.T) string {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("the case's own temp directory does not resolve: %v", err)
	}
	if err = os.MkdirAll(base+"/checkout/ai/kk-flavor", 0o755); err != nil {
		t.Fatalf("building the fixture checkout: %v", err)
	}
	if err = os.MkdirAll(base+"/home", 0o755); err != nil {
		t.Fatalf("building the fixture home: %v", err)
	}
	if err = os.WriteFile(base+"/checkout/ai/project-skills.sh", nil, 0o644); err != nil {
		t.Fatalf("building the fixture stub: %v", err)
	}
	if err = os.Symlink(base+"/checkout/ai/kk-flavor", base+"/home/.kk-flavor"); err != nil {
		t.Fatalf("linking the fixture bucket: %v", err)
	}
	return base
}

// The path a repository's post-checkout hook runs: through the bucket link and back out of it with
// `..`. The bucket is a link into a checkout, so that `..` means the checkout's `ai/` alone.

// The same `..` cleaned against the name it was reached by names the home directory instead, where
// no stub has ever been. A resolution that cleans before it follows answers a path that is absent,
// and the hook then fails on every checkout.

// Both spellings, because argv[0] is whatever the human or the hook typed.
func TestOwnDirectoryFollowsASymlinkBeforeItResolvesADotDot(t *testing.T) {
	base := checkoutThroughABucket(t)
	t.Chdir(base + "/home")

	for _, invocation := range []string{
		base + "/home/.kk-flavor/../project-skills.sh",
		".kk-flavor/../project-skills.sh",
	} {
		got, err := shell.OwnDirectory(invocation)
		if err != nil {
			t.Errorf("OwnDirectory(%q): %v", invocation, err)
			continue
		}
		if want := base + "/checkout/ai"; got != want {
			t.Errorf("OwnDirectory(%q) = %q, want %q", invocation, got, want)
		}
	}
}

// RealPath is realpath(1): absolute, and symlinks followed. Every caller records or compares the
// answer, so a relative spelling that survives is a bug. An install registry entry reading `.` names
// a different directory for every later reader. A guard holding `.` against an absolute home never
// matches the home it is guarding.

// The `.kk-flavor/..` row is what separates this from filepath.Abs followed by
// filepath.EvalSymlinks. Abs cleans that `..` away before anything follows the link, and it answers
// a directory that exists and is the wrong one.
func TestRealPathIsAbsoluteAndPhysicalWhateverTheCallerTyped(t *testing.T) {
	base := checkoutThroughABucket(t)
	t.Chdir(base + "/home")

	for _, c := range []struct {
		path, want string
	}{
		{".", base + "/home"},
		{"./", base + "/home"},
		{"..", base},
		{"../checkout/ai", base + "/checkout/ai"},
		{".kk-flavor", base + "/checkout/ai/kk-flavor"},
		{".kk-flavor/..", base + "/checkout/ai"},
		{base + "/home/.kk-flavor", base + "/checkout/ai/kk-flavor"},
	} {
		got, err := shell.RealPath(c.path)
		if err != nil {
			t.Errorf("RealPath(%q): %v", c.path, err)
			continue
		}
		if got != c.want {
			t.Errorf("RealPath(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// DirName and BaseName split one path, so one input list holds both: this table exists to catch a
// row where one of them answers dirname(1) and the other does not. Every expected value is
// dirname(1)'s or basename(1)'s own output, so a row that looks wrong is not.
//
// A trailing slash arrives from argv[0], which eco-stats and eco-report both resolve their own
// directory out of. Repeated slashes arrive from shell.Join, whenever a root is spelled with a
// trailing one. The repeated-slash rows pin the second trim: dirname stops at `a`, never `a/`. The
// non-ASCII row is here because both functions split bytes, not runes.
func TestDirNameAndBaseNameAreDirnameAndBasename(t *testing.T) {
	for _, c := range []struct {
		path, dir, base string
	}{
		{"", ".", ""},
		{"/", "/", "/"},
		{"//", "/", "/"},
		{".", ".", "."},
		{"a", ".", "a"},
		{"a/", ".", "a"},
		{"a//", ".", "a"},
		{"/a", "/", "a"},
		{"/a/", "/", "a"},
		{"/a//", "/", "a"},
		{"//a", "/", "a"},
		{"a/b", "a", "b"},
		{"a/b/", "a", "b"},
		{"/a/b", "/a", "b"},
		{"/a/b/", "/a", "b"},
		{"a//b", "a", "b"},
		{"//a/b", "//a", "b"},
		{"///a///b///", "///a", "b"},
		{"./a/", ".", "a"},
		{"../", ".", ".."},
		{"a/ünïcode/", "a", "ünïcode"},
	} {
		if got := shell.DirName(c.path); got != c.dir {
			t.Errorf("DirName(%q) = %q, want %q", c.path, got, c.dir)
		}
		if got := shell.BaseName(c.path); got != c.base {
			t.Errorf("BaseName(%q) = %q, want %q", c.path, got, c.base)
		}
	}
}

// Fnmatch's element arms. `*` and a literal byte are the two the ecosystem's own patterns use, so
// they are the two every other suite exercises; `?`, a bracket expression and a backslash escape are
// reached by no caller in this tree and by no case anywhere in the module. The claim they make is
// fnmatch's, so they are held to fnmatch's answers rather than to their own — every row below was
// diffed against a C fnmatch(3) and agrees with it, bar the two the bracket note names.
func TestFnmatchElementArms(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"*.sh", "run.sh", true},
		{"*.sh", "run.md", false},

		{"?.sh", "a.sh", true},
		{"?.sh", "ab.sh", false},
		{"?", "", false},
		{"a?c", "abc", true},

		{"[abc].sh", "b.sh", true},
		{"[abc].sh", "d.sh", false},
		{"[a-z].sh", "q.sh", true},
		{"[a-z].sh", "Q.sh", false},
		{"[!a-z].sh", "Q.sh", true},
		{"[!a-z].sh", "q.sh", false},
		{"[^a-z].sh", "Q.sh", true},
		{"[]].sh", "].sh", true},
		{"[a-].sh", "-.sh", true},

		// The rows fnmatch does not settle. POSIX leaves an unterminated bracket undefined and the
		// implementations split on it — glibc reads the `[` as a literal, BSD libc matches nothing —
		// and a pattern ending in a lone `\` is the same kind of corner. path.go picks the literal
		// reading for both, so these pin a choice of ours rather than fnmatch's answer, and a change
		// to that reading has to come here to land.
		{"[abc", "[abc", true},
		{"[", "[", true},
		{`a\`, `a\`, true},
		{`a\`, "ab", false},

		{`\*.sh`, "*.sh", true},
		{`\*.sh`, "x.sh", false},
		{`a\\b`, `a\b`, true},

		{"*", "anything", true},
		{"", "", true},
		{"", "x", false},
	}
	for _, c := range cases {
		if got := shell.Fnmatch(c.pattern, c.name); got != c.want {
			t.Errorf("Fnmatch(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
		}
	}
}

// A `*` backtracks, and the bound is that it never matches a `/`-free name across more than the name.
func TestFnmatchStarBacktracks(t *testing.T) {
	for _, c := range []struct {
		pattern, name string
		want          bool
	}{
		{"*a*b", "xaybzb", true},
		{"*a*b", "xayb", true},
		{"*a*b", "ab", true},
		{"*a*b", "ba", false},
		{"a*", "a", true},
		{"*a", "a", true},
	} {
		if got := shell.Fnmatch(c.pattern, c.name); got != c.want {
			t.Errorf("Fnmatch(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
		}
	}
}

// CanonicalDir answers RealPath's question about a directory, so it has to answer it the same way.

// The `..` row is the whole case. A bucket link into a checkout, stepped back out of with `..`,
// names the checkout's `ai/` and that alone. The same `..` cleaned against the name the path was
// reached by names the home directory, which is a real directory and the wrong one.

// A containment test comparing against the wrong real directory refuses what it should admit.
func TestCanonicalDirFollowsASymlinkBeforeItResolvesADotDot(t *testing.T) {
	base := checkoutThroughABucket(t)
	t.Chdir(base + "/home")

	for _, c := range []struct {
		path, want string
	}{
		{".kk-flavor/..", base + "/checkout/ai"},
		{base + "/home/.kk-flavor/..", base + "/checkout/ai"},
		{".kk-flavor", base + "/checkout/ai/kk-flavor"},
		{".", base + "/home"},
		{base + "/checkout/ai/project-skills.sh", ""},
		{base + "/home/nothing-here", ""},
	} {
		if got := shell.CanonicalDir(c.path); got != c.want {
			t.Errorf("CanonicalDir(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// IsWithin is the single containment test these tools make, and every copy of it answered the
// equality case differently until they were folded together. Both sides arrive already resolved, so
// this compares and never resolves.

// The empty rows are the bug a single copy prevents. A resolver that could not answer hands back the
// empty string. `strings.HasPrefix(path, ""+"/")` is then true for EVERY absolute path, and a guard
// written that way admits the whole filesystem at exactly the moment it knows least.
func TestIsWithinHoldsTheRootItselfAndRefusesWhatOnlySharesAPrefix(t *testing.T) {
	for _, c := range []struct {
		path, root string
		want       bool
	}{
		{"/a/b", "/a", true},
		{"/a/b/c", "/a", true},
		{"/a", "/a", true},
		{"/ab", "/a", false},
		{"/a-b", "/a", false},
		{"/a", "/a/b", false},
		{"/b", "/a", false},
		{"/a/b", "/", true},
		{"/", "/", true},
		{"/a", "", false},
		{"", "/a", false},
		{"", "", false},
	} {
		if got := shell.IsWithin(c.path, c.root); got != c.want {
			t.Errorf("IsWithin(%q, %q) = %v, want %v", c.path, c.root, got, c.want)
		}
	}
}

// The ancestor a write is judged by: the deepest point over a path that a symlink could still
// redirect. The names under it exist nowhere yet, so none of them can redirect a write.

// A DIRECTORY, and never merely a name that resolves. A regular file resolves perfectly well, and no
// write can land under one. A walk stopping there judges the write by such a path, which is how a
// test fixture guarding the same bound as the code can guard a different one.
func TestNearestExistingParentClimbsToADirectoryAndResolvesIt(t *testing.T) {
	base := checkoutThroughABucket(t)

	for _, c := range []struct {
		path, want string
	}{
		{base + "/home/anything", base + "/home"},
		{base + "/home/one/two/three", base + "/home"},
		{base + "/home/.kk-flavor/skills/a", base + "/checkout/ai/kk-flavor"},
		{base + "/checkout/ai/project-skills.sh/below", base + "/checkout/ai"},
		{base + "/checkout/ai/project-skills.sh", base + "/checkout/ai"},
		{base + "/home/.kk-flavor/../anything", base + "/checkout/ai"},
	} {
		if got := shell.NearestExistingParent(c.path); got != c.want {
			t.Errorf("NearestExistingParent(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

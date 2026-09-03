package shell_test

import (
	"testing"

	"kk-flavor/tools/shell"
)

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
		{"///", "/", "/"},
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
		{"/a/b//", "/a", "b"},
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

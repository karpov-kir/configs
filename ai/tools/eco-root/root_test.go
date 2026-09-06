package ecoroot_test

// Resolving the checkout. Neither tool's own suite reaches the candidate list or the refusal below:
// every fixture there names its root outright.

import (
	"os"
	"testing"

	ecoroot "kk-flavor/tools/eco-root"
)

func TestANamedRootIsTakenExactlyAsItWasSpelled(t *testing.T) {
	t.Run("resolves a directory holding both kk-flavor and skills", func(t *testing.T) {
		_, dir := newCheckout(t, "")
		root, ok := ecoroot.New(dir)
		if !ok {
			t.Fatalf("New(%q) refused a checkout holding both directories", dir)
		}
		assertEquals(t, "named", root.Named(), dir)
		assertEquals(t, "flavor", root.Flavor(), dir+"/kk-flavor")
		assertEquals(t, "skills", root.Skills(), dir+"/skills")
	})

	t.Run("keeps a redundant path as the caller wrote it, never cleaned", func(t *testing.T) {
		base, _ := newCheckout(t, "ai")
		t.Chdir(base)
		root, ok := ecoroot.New("ai/../ai")
		if !ok {
			t.Fatal(`New("ai/../ai") refused a checkout it can reach`)
		}
		// filepath.Join would answer "ai/kk-flavor" here, and the tool would then print a path the
		// caller never named.
		assertEquals(t, "flavor", root.Flavor(), "ai/../ai/kk-flavor")
	})
}

func TestARootMissingEitherDirectoryIsRefused(t *testing.T) {
	for _, missing := range []string{"kk-flavor", "skills"} {
		t.Run("refuses a directory with no "+missing, func(t *testing.T) {
			_, dir := newCheckout(t, "")
			if err := os.RemoveAll(dir + "/" + missing); err != nil {
				t.Fatal(err)
			}
			// Refused rather than half-resolved: a tool that measured this tree would report a
			// figure for a checkout that is not one, and a reader cannot tell that from a real zero.
			if _, ok := ecoroot.New(dir); ok {
				t.Errorf("New accepted a directory holding no %s/", missing)
			}
		})
	}
}

func TestABareInvocationTriesTheTwoCandidatesInOrder(t *testing.T) {
	t.Run("finds the checkout in the working directory", func(t *testing.T) {
		_, dir := newCheckout(t, "")
		t.Chdir(dir)
		root, ok := ecoroot.New("")
		if !ok {
			t.Fatal("New(\"\") found no checkout in a working directory that is one")
		}
		assertEquals(t, "named", root.Named(), ".")
	})

	t.Run("falls through to ./ai when the working directory is not one", func(t *testing.T) {
		base, _ := newCheckout(t, "ai")
		t.Chdir(base)
		root, ok := ecoroot.New("")
		if !ok {
			t.Fatal("New(\"\") found no checkout under ./ai")
		}
		assertEquals(t, "named", root.Named(), "./ai")
	})

	t.Run("refuses when neither candidate holds both directories", func(t *testing.T) {
		t.Chdir(t.TempDir())
		if _, ok := ecoroot.New(""); ok {
			t.Error("New(\"\") accepted a working directory holding no checkout")
		}
	})
}

// under is where beneath base to build the checkout, so a case can put one at ./ai and leave base itself
// holding no checkout at all; base is what such a case makes its working directory.
func newCheckout(t *testing.T, under string) (base, checkout string) {
	t.Helper()
	base = t.TempDir()
	checkout = base
	if under != "" {
		checkout = base + "/" + under
	}
	for _, name := range []string{"kk-flavor", "skills"} {
		if err := os.MkdirAll(checkout+"/"+name, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return base, checkout
}

func assertEquals(t *testing.T, what, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", what, got, want)
	}
}

// The always-loaded tier's membership. The two heading lines are the whole question: a link on the closing `## Read on trigger` heading is in
// the next tier, not this one.
func TestOnlyTheLinksBetweenTheHeadingsAreAlwaysLoaded(t *testing.T) {
	lines := []string{
		"# Flavor",
		"",
		"## Read always [heading-link](heading.md)",
		"",
		"- [core](standards/core.md)",
		"- [writing](standards/writing.md)",
		"",
		"## Read on trigger [closing-link](closing.md)",
		"",
		"- [code style](standards/code-style.md)",
	}

	got := ecoroot.ReadAlwaysTargets(lines)
	want := []string{"standards/core.md", "standards/writing.md"}
	if len(got) != len(want) {
		t.Fatalf("ReadAlwaysTargets = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("target %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestARouterWithNoReadAlwaysHeadingListsNothing(t *testing.T) {
	lines := []string{"# Flavor", "", "- [core](standards/core.md)"}
	if got := ecoroot.ReadAlwaysTargets(lines); len(got) != 0 {
		t.Errorf("ReadAlwaysTargets = %v, want none", got)
	}
}

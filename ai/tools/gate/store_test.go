// The verdict store under several worktrees of one clone, which is how it is actually used: the store
// is the clone's (`gate.go`, at g.cache), the builds are one worktree each, and three of them gating at
// once write one directory.
//
// Two claims are held here, and they pull in opposite directions. A record is keyed on content alone,
// so a worktree may answer out of another's — that is the gate's premise, not a leak, and the case
// two below fail if a key ever starts carrying where the run happened. The `<stem>.inputs` sidecars
// carry no key, so every worktree writes the same paths, and the two cases after those say what makes
// reading one safe and what would stop it being safe.
package gate

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// A record another worktree wrote is a filename collision on a content hash, which is exactly what the
// gate means by fresh. Nothing about the run's location may reach the key, or a cold worktree gates
// from scratch every time.
func TestAVerdictRecordedInOneWorktreeIsFreshInAnother(t *testing.T) {
	const table = "one\tcheck\twatched.txt\t"

	first := newFixture(t)
	first.write("watched.txt", "one\n")
	first.table(table + marker("ran.log", 0))
	first.run()
	first.expectCode(0)
	first.expectOut("ran ok")

	// A second checkout of the same content, with its own root and its own units file, sharing the one
	// store — the shape `git worktree add` leaves behind.
	second := newFixture(t)
	second.cache = first.cache
	second.write("watched.txt", "one\n")
	second.table(table + marker("ran.log", 0))
	second.run()
	second.expectCode(0)
	second.expectOut("fresh")
	if got := second.runCount("ran.log"); got != 0 {
		t.Errorf("the second worktree ran the command %d time(s) over content the first already "+
			"proved green. Something in the key moved with the worktree, so every new worktree gates "+
			"from cold — output:\n%s", got, second.out())
	}

	// The control, and without it the case above passes just as well over a gate that calls everything
	// fresh. Move one byte in the second worktree and its record is a different file, which nobody has
	// written.
	second.write("watched.txt", "two\n")
	second.run()
	second.expectCode(0)
	second.expectOut("ran ok")
	if got := second.runCount("ran.log"); got != 1 {
		t.Errorf("the second worktree ran the command %d time(s) after changing its own input, "+
			"wanted 1 — a hit here would be a stale green rather than a shared verdict", got)
	}
	if got := first.runCount("ran.log"); got != 1 {
		t.Errorf("the first worktree's command ran %d time(s); each fixture counts runs in its own "+
			"root, so anything but 1 means the two roots are not separate", got)
	}
}

// The property the shared store rests on, asserted over the real table rather than argued in a
// comment. An absolute path in key material breaks the sharing twice over: the key moves with the
// checkout, so no worktree ever answers out of another's, and an input git never lists matches no
// manifest line, so the unit is keyed on one file fewer and answers a cached pass over it.
func TestNoKeyMaterialNamesTheWorktreeItWasBuiltIn(t *testing.T) {
	g, _, _ := discoveredOverThisRepo(t)
	for _, u := range g.units {
		if strings.Contains(u.cmd, g.root) {
			t.Errorf("%s runs `%s`, which spells out this checkout. Its key then differs in every "+
				"worktree, and the store they share serves none of them", u.id, u.cmd)
		}
		for _, in := range u.inputs {
			if filepath.IsAbs(in) {
				t.Errorf("%s declares the absolute input %s. `git ls-files` answers in repository-"+
					"relative paths, so nothing in the manifest matches it and the unit is keyed on "+
					"one file fewer than it declares", u.id, in)
			}
		}
	}

	// The mutation units, which are the risk: the harness prints resolved paths ABSOLUTE, and
	// groupMutants trimming the root off them is the one place a root reaches an input at all. Driven
	// off the fixture listing rather than real discovery, which would `go build` the harness and write
	// a binary into the tree.
	groups := grouped(t)
	fromListing := false
	for _, group := range groups {
		for _, in := range group.inputs {
			if strings.HasSuffix(in, ".go") {
				fromListing = true
			}
			if filepath.IsAbs(in) {
				t.Errorf("%s declares the absolute input %s — the harness's resolved path reached the "+
					"key with the root still on it", group.id, in)
			}
		}
		for _, file := range group.files {
			if filepath.IsAbs(file) {
				t.Errorf("%s hands the harness the absolute path %s, so its command names this "+
					"checkout and its key moves with it", group.id, file)
			}
		}
	}
	// The control. A group's inputs also hold suite directories and the harness's own tree, neither of
	// which goes through the trim — without this, the loop above passes having read only those.
	if !fromListing {
		t.Fatalf("no group among %d holds a path from the listing's resolved column, so nothing above "+
			"read a trimmed path", len(groups))
	}
}

// What makes reading a keyless file out of a shared store safe: a reader in another worktree sees a
// whole record or the previous whole record, never the middle of a write.
func TestASidecarIsPublishedWholeOrNotAtAll(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gotest.inputs")

	short := renderLines([]manifestLine{{hash: "aaa", path: "watched/one.txt"}})
	var many []manifestLine
	for i := 0; i < 3000; i++ {
		many = append(many, manifestLine{hash: strings.Repeat("c", 64), path: "watched/many.txt"})
	}
	long := renderLines(many)
	writeSidecar(path, short)

	stop := make(chan struct{})
	torn := make(chan string, 1)
	var readers sync.WaitGroup
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			body, err := os.ReadFile(path)
			if err != nil {
				select {
				case torn <- "the path held no file at all":
				default:
				}
				return
			}
			if seen := string(body); seen != short && seen != long {
				select {
				case torn <- seen:
				default:
				}
				return
			}
		}
	}()

	for i := 0; i < 150; i++ {
		writeSidecar(path, long)
		writeSidecar(path, short)
	}
	close(stop)
	readers.Wait()

	select {
	case seen := <-torn:
		t.Fatalf("a concurrent reader saw %d bytes that are neither record. changedSinceGreen would "+
			"have compared against it, and the case below shows a body that lost its tail answering "+
			"`unchanged` over content that moved. Publish the sidecar by rename", len(seen))
	default:
	}
}

// Publishing by rename leaks a temp whenever a run is killed between creating one and renaming it, and
// the store is the clone's, so they arrive from every killed run in every worktree. The sweep takes
// them — and must not take the one a sibling worktree is writing this second, which is the whole
// reason it is bounded by age rather than run unconditionally.
func TestAKilledRunsLeftoverSidecarIsSweptAndALiveOneIsNot(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.table("one\tcheck\twatched.txt\t" + marker("ran.log", 0))
	if err := os.MkdirAll(f.cache, 0o755); err != nil {
		t.Fatalf("building the store: %v", err)
	}

	leaked := filepath.Join(f.cache, "one"+sidecarSuffix+".2241093")
	inFlight := filepath.Join(f.cache, "one"+sidecarSuffix+".9930517")
	sidecar := filepath.Join(f.cache, "one"+sidecarSuffix)
	// A unit whose stem ends in `.inputs` — a units-file table may name one — has a verdict record
	// spelt exactly like a temp. Only the tail's length tells the two apart, so this is the case that
	// says the sweep measures one.
	record := filepath.Join(f.cache, "one"+sidecarSuffix+"."+strings.Repeat("ab", 32))
	for _, path := range []string{leaked, inFlight, sidecar, record} {
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatalf("planting %s: %v", path, err)
		}
	}
	// Everything but the in-flight temp is old, so age alone never explains a survivor below.
	old := time.Now().Add(-24 * time.Hour)
	for _, path := range []string{leaked, sidecar, record} {
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatalf("ageing %s: %v", path, err)
		}
	}

	f.run()
	f.expectCode(0)

	if _, err := os.Stat(leaked); err == nil {
		t.Error("a temp left by a killed run a day ago is still in the store, so every killed run in " +
			"every worktree of this clone adds one and nothing ever takes it away")
	}
	for _, path := range []string{inFlight, sidecar, record} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("the sweep deleted %s. A temp being written this second belongs to another "+
				"worktree's run, and a verdict record spelt like one is a green nobody re-earned",
				filepath.Base(path))
		}
	}
}

// The sweep tells a leaked temp from a verdict record by the length of the tail alone, so that number
// is right only while it is what the key-maker produces. Read off the real function, never restated.
func TestAVerdictKeyIsAsLongAsTheSweepThinks(t *testing.T) {
	if got := len(hashString("any unit's key material")); got != verdictKeyLength {
		t.Fatalf("a key is %d characters and the sweep believes %d, so a record whose stem ends in "+
			"`%s` now reads as a leaked temp and is deleted while still green",
			got, verdictKeyLength, sidecarSuffix)
	}
}

// Why the whole record matters, stated where it can be checked rather than argued. A record that lost
// its tail is a well-formed record of a smaller set, and nothing in the body says which it is.
func TestASidecarMissingALineCannotBeToldFromAMatch(t *testing.T) {
	cache := t.TempDir()
	sidecar := filepath.Join(cache, "gotest.inputs")
	one := manifestLine{hash: "aaa", path: "watched/one.txt"}
	two := manifestLine{hash: "bbb", path: "watched/two.txt"}

	// The live tree: two.txt has been deleted since the green was recorded.
	live := &gate{cache: cache, manifest: []manifestLine{one}}

	writeSidecar(sidecar, renderLines([]manifestLine{one, two}))
	if !live.changedSinceGreen([]string{"watched"}) {
		t.Fatal("the whole record, which still names a file the tree no longer has, answered " +
			"`unchanged`. Everything below is then about a comparison that does not compare")
	}

	// The same record with its last line gone — the shape a truncate-then-write leaves a reader that
	// arrives mid-write, and the shape a reader cannot refuse.
	writeSidecar(sidecar, renderLines([]manifestLine{one}))
	if live.changedSinceGreen([]string{"watched"}) {
		t.Fatal("a record that lost a line answered `changed`, so this file no longer says why the " +
			"sidecar must be published whole. Check what now tells the two apart before deleting this")
	}

	// The tears a reader does survive, and why the direction is usually the safe one: a cut inside a
	// line leaves either no `hash  path` separator or a path matching nothing under the set asked
	// about, so the line is dropped either way and the digests disagree.
	for _, body := range []string{"", "aaa  watched/one.txt\nbbb  watc", "aaa  watched/one.txt\nbb"} {
		writeSidecar(sidecar, body)
		full := &gate{cache: cache, manifest: []manifestLine{one, two}}
		if !full.changedSinceGreen([]string{"watched"}) {
			t.Errorf("a body cut to %q answered `unchanged` over a tree holding both files", body)
		}
	}
}

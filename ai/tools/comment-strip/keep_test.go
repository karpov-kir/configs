package commentstrip

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rulesHome is a home directory holding the rules a block is written under.
func rulesHome(t *testing.T, style string) string {
	t.Helper()
	home := t.TempDir()
	for _, path := range rulePaths {
		full := filepath.Join(home, ".kk-flavor", path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(style+path), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	return home
}

const keptSource = "import { keys } from './keys';\n\n" +
	"// A ledger build answers `canPost` for its own scheme alone, so `claimFor` asks it once per scheme.\n" +
	"export function claimFor(scheme: string): boolean {\n" +
	"  return keys.canPost(scheme);\n" +
	"}\n"

const keptRecord = "fact: a ledger build answers canPost for its own scheme alone\n" +
	"bears_on: claimFor\n" +
	"does: returns keys.canPost(scheme)\n"

// archiveWritten archives the fixture's block as run11 wrote it.
func archiveWritten(t *testing.T, f *fixture, archive string) {
	t.Helper()
	record := filepath.Join(f.dir, "record.txt")
	if err := os.WriteFile(record, []byte(keptRecord), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	if code := Strip("comment-strip.sh", []string{"--archive=" + archive, "--written=run11", f.path, "4", record},
		f.dir, noRepository, &out, &errOut); code != exitClean {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
}

// A block the lane wrote stands byte for byte in the next run while its record holds, and the strip
// offers no site for it. Run 12 reworded about fifty blocks run 11 had written correctly.
func TestABlockWhoseRecordHoldsStandsAsWritten(t *testing.T) {
	rulesHome(t, "rules one ")
	f := newFixture(t, "f.ts", keptSource)
	archive := filepath.Join(f.dir, "archive")
	archiveWritten(t, f, archive)
	said := f.run("--archive=" + archive)
	if said.code != exitClean || f.body() != keptSource || !strings.Contains(said.stderr, "kept as run11 wrote it") {
		t.Fatalf("exit %d, stderr %s, file:\n%s", said.code, said.stderr, f.body())
	}
}

// A changed rule, a changed body, a contradiction and a review sending the block back each reopen it.
func TestAKeptBlockReopensOnARuleABodyAContradictionOrAReview(t *testing.T) {
	cases := map[string]func(t *testing.T, f *fixture, archive string) []string{
		"a changed rule": func(t *testing.T, f *fixture, archive string) []string {
			rulesHome(t, "rules two ")
			return nil
		},
		"a changed body": func(t *testing.T, f *fixture, archive string) []string {
			f.write(strings.Replace(keptSource, "keys.canPost(scheme)", "keys.canPost(scheme) === true", 1))
			return nil
		},
		"a contradiction": func(t *testing.T, f *fixture, archive string) []string {
			var out, errOut strings.Builder
			block := "// A ledger build answers `canPost` for its own scheme alone, so `claimFor` asks it once per scheme.\n"
			if code := Strip("comment-strip.sh", []string{"--archive=" + archive, "--contradict=run12", f.path,
				recordID(block), "canPost answers for every scheme"}, f.dir, noRepository, &out, &errOut); code != exitClean {
				t.Fatalf("exit %d: %s", code, errOut.String())
			}
			return nil
		},
		"a review": func(t *testing.T, f *fixture, archive string) []string {
			return []string{"--lines=3"}
		},
	}
	for name, reopen := range cases {
		t.Run(name, func(t *testing.T) {
			rulesHome(t, "rules one ")
			f := newFixture(t, "f.ts", keptSource)
			archive := filepath.Join(f.dir, "archive")
			archiveWritten(t, f, archive)
			extra := reopen(t, f, archive)
			said := f.run(append([]string{"--archive=" + archive}, extra...)...)
			if said.code != exitCut || strings.Contains(f.body(), "// A ledger build") {
				t.Fatalf("the block stood: exit %d, stderr %s", said.code, said.stderr)
			}
		})
	}
}

// A kept block's older claims stay unread. The strip would offer them at the declaration the kept
// block stands on, and a writer would write a second block there.
func TestAKeptBlockLeavesItsOlderRecordUnoffered(t *testing.T) {
	rulesHome(t, "rules one ")
	f := newFixture(t, "f.ts", strings.Replace(keptSource, "// A ledger build", "// An older block. A ledger build", 1))
	archive := filepath.Join(f.dir, "archive")
	f.cut("--archive=" + archive)
	// The writer wrote the block run 11 archived.
	f.write(keptSource)
	if err := os.RemoveAll(f.facts); err != nil {
		t.Fatal(err)
	}
	archiveWritten(t, f, archive)
	if said := f.run("--archive=" + archive); said.code != exitClean || said.stdout != "" {
		t.Fatalf("exit %d, sites %q: %s", said.code, said.stdout, said.stderr)
	}
}

// A file header and the block under it sit on the same declaration. Archived with a key without the
// block, one replaced the other, and the next run wrote the replaced one again.
func TestAHeaderAndTheBlockUnderItBothStand(t *testing.T) {
	rulesHome(t, "rules one ")
	source := "// `claimFor` answers for one scheme of the ledger at a time.\n\n" + strings.TrimPrefix(keptSource, "import { keys } from './keys';\n\n")
	source = "import { keys } from './keys';\n" + source
	f := newFixture(t, "f.ts", source)
	archive := filepath.Join(f.dir, "archive")
	record := filepath.Join(f.dir, "record.txt")
	if err := os.WriteFile(record, []byte(keptRecord), 0o644); err != nil {
		t.Fatal(err)
	}
	// The header is named by its own first line, and the block under it by its declaration.
	for _, line := range []string{"2", "5"} {
		var out, errOut strings.Builder
		if code := Strip("comment-strip.sh", []string{"--archive=" + archive, "--written=run13", f.path, line, record},
			f.dir, noRepository, &out, &errOut); code != exitClean {
			t.Fatalf("line %s: exit %d: %s", line, code, errOut.String())
		}
	}
	said := f.run("--archive=" + archive)
	if said.code != exitClean || strings.Count(said.stderr, "kept as run13") != 2 {
		t.Fatalf("exit %d: %s", said.code, said.stderr)
	}
}

package commentstrip

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A site a writer declined is offered no more while the rules, its code and its claims hold, and is
// offered again when any of the three changes. A `--lines` strip, a review sending the site back, offers
// it whatever was declined. Run 22 offered 82 sites run 20 had declined on an unchanged tree.
func TestADeclinedSiteStaysDeclinedWhileWhatItWasDecidedOnHolds(t *testing.T) {
	home := rulesHome(t, "rules one ")
	f := newFixture(t, "f.ts", keptSource)
	archive := filepath.Join(f.dir, "archive")
	// The first strip archives the block's claim and leaves the site with no block.
	if said := f.run("--archive=" + archive); said.code != exitCut {
		t.Fatalf("the first strip took nothing: %d %s", said.code, said.stderr)
	}
	offer, err := os.ReadFile(filepath.Join(f.facts, "1.facts"))
	if err != nil {
		t.Fatal(err)
	}
	all, own, err := OfferedRecords(archive, f.path, string(offer))
	if err != nil || len(all) != 1 || len(own) != 1 {
		t.Fatalf("the offer maps to %v (own %v), %v", all, own, err)
	}
	stripped := f.body()
	if n, err := Decide(archive, "run20", f.dir, f.path, all, true); err != nil || n != 1 {
		t.Fatalf("decide recorded %d, %v", n, err)
	}
	// Each run writes a facts directory of its own.
	strip := func(options ...string) outcome {
		os.RemoveAll(f.facts)
		return f.run(append([]string{"--archive=" + archive}, options...)...)
	}
	offered := func() bool { return strings.Contains(strip().stdout, "1.facts") }
	if offered() {
		t.Fatal("a declined site on unchanged rules, code and claims was offered again")
	}
	if said := strip(); !strings.Contains(said.stderr, "declined as run20"+DeclinedLine) {
		t.Errorf("the strip did not say the site stays declined: %s", said.stderr)
	}
	if said := strip("--lines=3"); said.code != exitCut {
		t.Errorf("a review sending the site back was refused it: %d %s", said.code, said.stderr)
	}
	f.write(stripped)
	// The code under the declaration changes.
	f.write(strings.Replace(stripped, "keys.canPost(scheme)", "keys.canPost(scheme) === true", 1))
	if !offered() {
		t.Error("a declined site whose code changed was not offered again")
	}
	f.write(stripped)
	if offered() {
		t.Fatal("the site was offered again with its code restored")
	}
	// The rules change.
	if err := os.WriteFile(filepath.Join(home, ".kk-flavor", rulePaths[0]), []byte("rules two "+rulePaths[0]), 0o644); err != nil {
		t.Fatal(err)
	}
	if !offered() {
		t.Error("a declined site was not offered again under other rules")
	}
}

// A block written settles its site's own claims and takes their decline away.
func TestABlockWrittenClearsItsOwnClaimsDecline(t *testing.T) {
	rulesHome(t, "rules one ")
	f := newFixture(t, "f.ts", keptSource)
	archive := filepath.Join(f.dir, "archive")
	f.run("--archive=" + archive)
	offer, _ := os.ReadFile(filepath.Join(f.facts, "1.facts"))
	all, own, _ := OfferedRecords(archive, f.path, string(offer))
	if _, err := Decide(archive, "run20", f.dir, f.path, all, true); err != nil {
		t.Fatal(err)
	}
	if n, err := Decide(archive, "run21", f.dir, f.path, own, false); err != nil || n != 1 {
		t.Fatalf("a written block cleared %d decline(s), %v", n, err)
	}
	os.RemoveAll(f.facts)
	if said := f.run("--archive=" + archive); !strings.Contains(said.stdout, "1.facts") {
		t.Errorf("the site was not offered after its decline was cleared: %s", said.stderr)
	}
}

// A run before the strip recorded what each site carried maps a facts file by its claims' words. The
// review's sentence a loop round adds and a contradiction line are no claim, and the match holds.
func TestOfferedRecordsReadsALoopRoundsFactsByTheirClaims(t *testing.T) {
	rulesHome(t, "rules one ")
	f := newFixture(t, "f.ts", keptSource)
	archive := filepath.Join(f.dir, "archive")
	f.run("--archive=" + archive)
	offer, _ := os.ReadFile(filepath.Join(f.facts, "1.facts"))
	looped := string(offer) + "contradicted: run21 the claim\n\n# code review:\nthe claim holds\n"
	if all, own, err := OfferedRecords(archive, f.path, looped); err != nil || len(all) != 1 || len(own) != 1 {
		t.Fatalf("a loop round's facts map to %v (own %v), %v", all, own, err)
	}
}

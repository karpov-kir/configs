// Cases for the counter that decides when `go-mutate` reporting "I did not measure" stops being
// weather and becomes a red job.
//
// Every scenario gets a directory of its own and the count file inside it is named TO Run rather than
// found by it, so nothing here depends on a path the tool picked.
package nomeasure

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// One run of the counter: what it decided, and what it said.
type outcome struct {
	status int
	note   string
}

func decide(t *testing.T, status, countFile string) outcome {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Run([]string{status, countFile}, &out, &errOut)
	return outcome{status: code, note: out.String() + errOut.String()}
}

// The path of a count file in a directory of this case's own. The file itself is not created — an
// absent count file is the ordinary first run.
func freshPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "count")
}

func seed(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("seeding %s: %v", path, err)
	}
}

// What the count file holds, or "absent". A string rather than an int, so a case can assert on a
// value that is not a count at all.
func stored(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		return "absent"
	}
	return string(body)
}

func wantStatus(t *testing.T, got outcome, want int, what string) {
	t.Helper()
	if got.status != want {
		t.Errorf("%s: exit %d, wanted %d\nit said: %s", what, got.status, want, got.note)
	}
}

func wantStored(t *testing.T, path, want, what string) {
	t.Helper()
	if held := stored(t, path); held != want {
		t.Errorf("%s: the count file holds %q, wanted %q", what, held, want)
	}
}

func wantSaid(t *testing.T, got outcome, needle, what string) {
	t.Helper()
	if !strings.Contains(got.note, needle) {
		t.Errorf("%s: nothing in what it said held %q\nit said: %s", what, needle, got.note)
	}
}

// The behaviour the whole file exists for. Driven as one chain against one count file, because what
// escalates is the history and not any single run.
func TestThreeDidNotMeasureRunsRunningEscalate(t *testing.T) {
	chain := freshPath(t)
	if held := stored(t, chain); held != "absent" {
		t.Fatalf("control: the chain must start with no stored count at all, found %q", held)
	}

	first := decide(t, "2", chain)
	wantStatus(t, first, 1, "the first did-not-measure warns rather than failing")
	wantStored(t, chain, "1", "and records one")
	if first.note == "" {
		t.Fatal("control: it said nothing, so the wording cases below would compare against an empty note")
	}
	wantSaid(t, first, "(1 consecutive)", "and the note counts the run")

	second := decide(t, "2", chain)
	wantStatus(t, second, 1, "the second still warns")
	wantStored(t, chain, "2", "and records two")

	third := decide(t, "2", chain)
	wantStatus(t, third, 3, "the third escalates")
	wantStored(t, chain, "3", "and records three")
	wantSaid(t, third, "3 consecutive runs", "and the note says the count is what changed the verdict")
	wantSaid(t, third, "no longer machine load", "and that this is no longer machine load")

	fourth := decide(t, "2", chain)
	wantStatus(t, fourth, 3, "a fourth keeps escalating rather than starting over")
	wantStored(t, chain, "4", "and the count keeps climbing")
}

// Both of the harness's measured statuses, because the counter tracks whether it reached the guards
// at all. Exit 1 is a guard that did not redden, which is a finding about the code and proof that the
// harness got there.
func TestAMeasuredRunClearsTheCount(t *testing.T) {
	measured := freshPath(t)
	seed(t, measured, "2")

	cleared := decide(t, "0", measured)
	wantStatus(t, cleared, 0, "a measured run reports measured")
	wantStored(t, measured, "0", "and clears the count")
	wantSaid(t, cleared, "back to 0", "and says the count was cleared")

	again := decide(t, "2", measured)
	wantStatus(t, again, 1, "so the next did-not-measure starts the chain again")
	wantStored(t, measured, "1", "control: from one, not from where it left off")

	failing := freshPath(t)
	seed(t, failing, "2")
	found := decide(t, "1", failing)
	wantStatus(t, found, 0, "a measured-but-failing run also reports measured")
	wantStored(t, failing, "0", "and also clears the count")
}

// A status the harness never defines clears the count too, and cannot buy a silent pass: the caller
// exits on the harness's own status, so the job goes red on it.
func TestAStatusTheHarnessNeverDefinesClearsTheCount(t *testing.T) {
	missing := freshPath(t)
	seed(t, missing, "2")
	gone := decide(t, "127", missing)
	wantStatus(t, gone, 0, "a 127 from a harness that is not there reports measured")
	wantStored(t, missing, "0", "and clears the count")
}

// Reading garbage as a number is how a corrupt cache entry holds the count under the threshold
// forever, which is the escalation quietly switched off. A row per shape: a word, a number with a
// tail, a negative, a padded one, an empty file, and three that are all digits and still not counts.
//
// "1234567890" is the row the length bound exists for, and the only one that reaches it: the two
// longer values fail Atoi on their own, so a run with the bound removed still resets on those and
// this case would pass over the guard being gone.
func TestACountThatDoesNotParseIsNoHistory(t *testing.T) {
	for _, garbage := range []string{"banana", "12abc", "-1", "  2  ", "", "99999999999999999999", "9999999999999999999", "1234567890"} {
		t.Run(strconv.Quote(garbage), func(t *testing.T) {
			spoiled := freshPath(t)
			seed(t, spoiled, garbage)
			if held := stored(t, spoiled); held != garbage {
				t.Fatalf("control: the file holds %q, so this case is not driving %q at all", held, garbage)
			}
			got := decide(t, "2", spoiled)
			wantStatus(t, got, 1, "a stored "+strconv.Quote(garbage)+" reads as no history, so this run is the first")
			wantStored(t, spoiled, "1", "and it leaves the file holding a count of one")
		})
	}
}

// A single run over garbage looks correct whatever the tool does with it. What a chain shows is
// whether the escalation behind it still fires.
func TestGarbageCannotSuppressTheEscalationBehindIt(t *testing.T) {
	for _, garbage := range []string{"banana", "9999999999999999999", "1234567890"} {
		t.Run(strconv.Quote(garbage), func(t *testing.T) {
			poisoned := freshPath(t)
			seed(t, poisoned, garbage)
			decide(t, "2", poisoned)
			decide(t, "2", poisoned)
			got := decide(t, "2", poisoned)
			wantStatus(t, got, 3, "three runs after a stored "+strconv.Quote(garbage)+" must still escalate")
		})
	}
}

func TestAStoredCountAlreadyPastTheThresholdEscalates(t *testing.T) {
	past := freshPath(t)
	seed(t, past, "9")
	got := decide(t, "2", past)
	wantStatus(t, got, 3, "a stored count already past the threshold escalates")
	wantStored(t, past, "10", "and still counts up")
}

func TestTheCountFilePathComesFromOutsideThisTool(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "día d'été", "counts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("making %s: %v", dir, err)
	}
	outside := filepath.Join(dir, "count")
	got := decide(t, "2", outside)
	wantStatus(t, got, 1, "the run reports on a path with a space and non-ASCII in it")
	wantStored(t, outside, "1", "and the path is written, not mangled")
}

func TestItWritesTheNamedFileAndNothingElseBesideIt(t *testing.T) {
	contained := freshPath(t)
	decide(t, "2", contained)
	entries, err := os.ReadDir(filepath.Dir(contained))
	if err != nil {
		t.Fatalf("reading the count file's directory: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 1 || names[0] != "count" {
		t.Errorf("the run left %v beside the count file, wanted only [count]", names)
	}
}

// Each arm is asserted on the count file as well as the status: a refusal that writes has already
// done the thing it refused, and the exit code alone would not show it.
func TestTheArmsThatDecideNothing(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run(nil, &out, &errOut); code != 2 {
		t.Errorf("no arguments: exit %d, wanted 2", code)
	}
	if !strings.Contains(errOut.String(), "needs the harness's exit status") {
		t.Errorf("no arguments: it did not name what it needed\nit said: %s", errOut.String())
	}

	out.Reset()
	errOut.Reset()
	if code := Run([]string{"2"}, &out, &errOut); code != 2 {
		t.Errorf("a status with no count file: exit %d, wanted 2 rather than picking a path", code)
	}

	bad := freshPath(t)
	seed(t, bad, "1")
	notANumber := decide(t, "banana", bad)
	wantStatus(t, notANumber, 2, "a status that is not a number")
	wantSaid(t, notANumber, "is no exit status", "and it says so")
	wantStored(t, bad, "1", "and it leaves the stored count untouched")
}

// A directory that does not exist, not a permission bit: CI runs as root often enough that a mode of
// 000 denies nothing there, and the case would then assert against a file the process writes happily.
func TestACountFileThatWillNotTakeTheWriteDecidesNothing(t *testing.T) {
	unwritable := filepath.Join(t.TempDir(), "no-such-directory", "count")

	got := decide(t, "2", unwritable)
	wantStatus(t, got, 2, "a count file that will not take the write exits 2, not a warn")
	wantSaid(t, got, "the count is unchanged", "and it says the count is unchanged")
	wantStored(t, unwritable, "absent", "control: and it really did not create anything")

	reset := decide(t, "0", unwritable)
	wantStatus(t, reset, 2, "a reset that cannot be written is not reported as measured")
}

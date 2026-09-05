// The gate runs the shell suites in one lane and everything else in another. These cases pin both
// halves of that, because each is load-bearing in a different direction:
//
//   - the two lanes really do overlap, or the change bought nothing and the report is a lie about it;
//   - two `shell:` units never overlap, because those suites build temp HOMEs and link into them, and
//     the one time containment failed there it overwrote real config files.
//
// And the report stays in declared order however the lanes finish, so two runs over one tree still
// produce the same bytes.
package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A command that stamps its own start and end into one shared log, so the ORDER of events across
// lanes is observable. Timing is not asserted anywhere here: a loaded machine makes durations
// meaningless, while "did B start before A ended" survives any load.
func stamped(id string, sleep string) string {
	return "printf 'S:" + id + " ' >> lanes.log; sleep " + sleep + "; printf 'E:" + id + " ' >> lanes.log"
}

func (f *fixture) laneLog() string {
	f.t.Helper()
	body, err := os.ReadFile(filepath.Join(f.root, "lanes.log"))
	if err != nil {
		f.t.Fatalf("no lane log was written, so nothing here observed anything: %v", err)
	}
	return string(body)
}

// Whether one event comes before another in the log.
func before(log, first, second string) bool {
	i, j := strings.Index(log, first), strings.Index(log, second)
	return i >= 0 && j >= 0 && i < j
}

func TestTheShellLaneRunsBesideTheRestAndNeverBesideItself(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.table(
		"shell:one\tcheck\twatched.txt\t"+stamped("one", "2"),
		"shell:two\tcheck\twatched.txt\t"+stamped("two", "2"),
		"other\tcheck\twatched.txt\t"+stamped("other", "2"),
	)
	f.run()
	f.expectCode(0)
	log := f.laneLog()

	// The control on the instrument. Without all six events the orderings below are vacuously true.
	for _, event := range []string{"S:one", "E:one", "S:two", "E:two", "S:other", "E:other"} {
		if !strings.Contains(log, event) {
			t.Fatalf("the log is missing %s, so this case is asserting over an incomplete run: %q", event, log)
		}
	}

	if !before(log, "E:one", "S:two") {
		t.Errorf("two `shell:` units overlapped. They build temp HOMEs and link into them, and nothing "+
			"proves two can share a machine — the log reads %q", log)
	}
	if !before(log, "S:other", "E:one") {
		t.Errorf("the non-shell unit did not start until the shell lane had finished, so the two lanes "+
			"ran one after the other and the concurrency bought nothing — the log reads %q", log)
	}
}

func TestTheReportKeepsDeclaredOrderWhicheverLaneFinishesFirst(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	// The slow one is declared first and finishes last; the quick one is declared last and finishes
	// first. Printed in completion order the report would invert, and two runs over one tree would
	// disagree with each other.
	f.table(
		"shell:slow\tcheck\twatched.txt\t"+stamped("slow", "2"),
		"quick\tcheck\twatched.txt\t"+stamped("quick", "0"),
	)
	f.run()
	f.expectCode(0)

	if !before(f.laneLog(), "E:quick", "E:slow") {
		t.Fatalf("the quick unit did not finish first, so this case never exercised out-of-order "+
			"completion: %q", f.laneLog())
	}
	report := f.out()
	if !before(report, "shell:slow", "quick") {
		t.Errorf("the report lists the units in completion order, not declared order, so two runs over "+
			"one tree print different bytes:\n%s", report)
	}
}

// A unit that settles without executing — fresh, deferred, no inputs — is decided in the serial pass
// before any lane starts. It must still print in its declared place among units that did run.
func TestASettledUnitKeepsItsPlaceAmongTheOnesThatRan(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.table(
		"shell:runner\tcheck\twatched.txt\t"+stamped("runner", "1"),
		"gone\tcheck\tnot-a-file.txt\ttrue",
		"tail\tcheck\twatched.txt\t"+stamped("tail", "0"),
	)
	f.run()
	// A unit resolving to no input file exits 2 whatever else passed, which is the point of it.
	f.expectCode(2)
	report := f.out()
	if !before(report, "shell:runner", "gone") || !before(report, "gone", "tail") {
		t.Errorf("the NO INPUTS unit did not print between the two that ran, so a settled unit is no "+
			"longer in declared order:\n%s", report)
	}
}

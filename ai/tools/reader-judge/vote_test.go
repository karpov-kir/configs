// Cases for the majority rule, and for reading one roll's answer.
package readerjudge

import (
	"sync"
	"testing"
	"time"
)

func TestParseVerdictAcceptsNumbersAndNone(t *testing.T) {
	gone, err := ParseVerdict(" 3, 1,3\n", 3)
	if err != nil || len(gone) != 2 || gone[0] != 1 || gone[1] != 3 {
		t.Fatalf("got %v %v, want [1 3]", gone, err)
	}
	if gone, err := ParseVerdict("None\n", 3); err != nil || gone != nil {
		t.Fatalf("none parsed as %v %v", gone, err)
	}
}

func TestParseVerdictRefusesAnAnswerThatIsEmpty(t *testing.T) {
	if _, err := ParseVerdict("   \n", 3); err == nil {
		t.Fatal("an empty answer was accepted as none")
	}
}

func TestParseVerdictRefusesProseAndOutOfRange(t *testing.T) {
	if _, err := ParseVerdict("I would delete 2 because it restates the code", 3); err == nil {
		t.Fatal("prose with a number in it was accepted")
	}
	if _, err := ParseVerdict("4", 3); err == nil {
		t.Fatal("a unit past the end was accepted")
	}
	if _, err := ParseVerdict("0", 3); err == nil {
		t.Fatal("unit 0 was accepted")
	}
}

func TestVotingRefusesIfAnyRollExplains(t *testing.T) {
	if _, err := Voting(rollsAnswering("1", "I think 1 goes", "1"), 3)("p", viewOf("a", "b", "c")); err == nil {
		t.Fatal("a prose roll was outvoted instead of refused")
	}
}

// A roll that names a unit nobody offered has lost the plot exactly as a roll that explains has, and
// fails the vote the same way. The gap the old line-count bound left is widest in a source file, whose
// units are its comment blocks: a 500-line file with 40 of them accepted 501.
func TestVotingRefusesARollNamingAUnitThatWasNeverOffered(t *testing.T) {
	view := viewOf("a", "b")
	if _, err := Voting(rollsAnswering("1", "3", "1"), 3)("p", view); err == nil {
		t.Fatal("a unit number past the last unit was tallied instead of refused")
	}
	if got := unitsInView(view); got != 2 {
		t.Fatalf("the view offers %d units, not 2 — the bound is reading something else", got)
	}
}

// The rule is MORE THAN half, and the count is Voting's own parameter, so it has to hold away from
// the 3 production passes today. Four, because an odd count cannot tell the rule from a bare half:
// no whole number of rolls is exactly half of 3 or of 9, and a vote reading `>=` there answers the
// same as one reading `>`. At four it does not — four rolls need three.
func TestTheMajorityRuleNeedsMoreThanHalfTheRolls(t *testing.T) {
	for _, c := range []struct {
		name    string
		replies []string
		want    string
	}{
		{"three of four carries a unit", []string{"1", "1", "1", "2"}, "1"},
		{"a bare half of four does not", []string{"1", "1", "2", "3"}, "none"},
	} {
		t.Run(c.name, func(t *testing.T) {
			call, calls := counting(rollsAnswering(c.replies...))
			got, err := Voting(call, 4)("p", viewOf("a", "b", "c"))
			if err != nil {
				t.Fatalf("vote refused: %v", err)
			}
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
			if calls() != 4 {
				t.Fatalf("%d call(s), want 4 — a roll was held back", calls())
			}
		})
	}
}

// A roll that never answered fails the whole vote, exactly as a roll that explains does. The rolls
// that did answer are a majority of a smaller vote than the one the caller asked for, and reading a
// verdict out of them reports the deadline the model hit as a judgement it made.
func TestVotingRefusesWhenARollFails(t *testing.T) {
	var mu sync.Mutex
	rolled := 0
	call := func(string, string) (string, error) {
		mu.Lock()
		rolled++
		first := rolled == 1
		mu.Unlock()
		if first {
			return "", &RollTimedOut{Deadline: defaultRollDeadline}
		}
		return "1", nil
	}
	if _, err := Voting(call, 3)("p", viewOf("a", "b")); err == nil {
		t.Fatal("a roll that never answered was outvoted instead of failing the vote")
	}
}

// A fenced block is one unit over four lines, and blank lines are no unit at all, so counting lines
// would answer 7 here where the vote may only offer 2.
func TestUnitsInViewCountsUnitsAndNotLines(t *testing.T) {
	view := viewOf("intro", "", "```", "one", "two", "```", "")
	if got := unitsInView(view); got != 2 {
		t.Fatalf("got %d units, want 2\n%s", got, view)
	}
}

// The rolls go out together, which is the half of it a call count cannot see. Each one blocks until
// all three have arrived, so a vote that rolled any of them in a later wave never reaches the third
// and this ends on the timeout instead of the reply.
func TestTheRollsGoOutTogether(t *testing.T) {
	var arrived sync.WaitGroup
	arrived.Add(3)
	call := func(string, string) (string, error) {
		arrived.Done()
		arrived.Wait()
		return "1", nil
	}
	done := make(chan string, 1)
	go func() {
		reply, err := Voting(call, 3)("p", viewOf("a", "b"))
		if err != nil {
			done <- "refused: " + err.Error()
			return
		}
		done <- reply
	}()
	select {
	case reply := <-done:
		if reply != "1" {
			t.Fatalf("got %q, want 1", reply)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the rolls went out in more than one wave — the last never started while the others waited")
	}
}

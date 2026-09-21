// Cases for the machine-wide lock. What must hold: two gates never run at once, a lock no live gate
// holds never wedges the machine, and the time a gate spends queued is not charged to its budget.
//
// The third is the reason the other two exist. Two gates at once measured 102s against a budget of
// 100s on a tree whose own runs read 51s to 83s, and a bound that reddens over another session's
// build is one every session learns to re-run without reading.
package gate

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A poll far below any real wait, so a case measuring the queue is measuring the queue.
const testPoll = 5 * time.Millisecond

func TestASecondGateWaitsForTheFirstRatherThanRacingIt(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	first, waited, err := takeLock(home, testPoll)
	if err != nil {
		t.Fatalf("the first gate could not take the lock: %v — nothing was measured", err)
	}
	if waited > time.Second {
		t.Fatalf("an unheld lock made the first gate wait %s, so the queue below proves nothing", waited)
	}

	queued := make(chan time.Duration, 1)
	go func() {
		second, waitedFor, err := takeLock(home, testPoll)
		if err != nil {
			queued <- -1
			return
		}
		second.release()
		queued <- waitedFor
	}()

	// Long enough that a second gate which ignored the lock would be well past taking it.
	held := 150 * time.Millisecond
	time.Sleep(held)
	select {
	case got := <-queued:
		t.Fatalf("a second gate took the lock after %s while the first still held it, so both would "+
			"run at once and each would charge the other's work to its own budget", got)
	default:
	}

	first.release()
	got := <-queued
	if got < held {
		t.Errorf("the second gate reports a wait of %s, under the %s the first held the lock — the "+
			"figure the gate subtracts from its budget is smaller than the time it actually queued",
			got, held)
	}
}

func TestALockNoLiveGateHoldsIsTakenRatherThanWaitedOut(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	dir := filepath.Join(home, lockName)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("laying out %s: %v — nothing was measured", dir, err)
	}
	// A pid no process holds. Every reaped pid reads this way, and a gate killed outright leaves one.
	if err := os.WriteFile(filepath.Join(dir, "pid"), []byte("999999\n"), 0o644); err != nil {
		t.Fatalf("writing the dead holder's pid: %v — nothing was measured", err)
	}

	held, waited, err := takeLock(home, testPoll)
	if err != nil {
		t.Fatalf("a lock held by a dead gate was not taken: %v — every gate on this machine now queues "+
			"behind it", err)
	}
	defer held.release()
	// Taken on the pid alone. Waiting out the abandoned bound would wedge the machine for five minutes
	// over a gate that is already gone.
	if waited > time.Second {
		t.Errorf("breaking a dead gate's lock took %s, so the bound was waited out rather than the pid "+
			"being read", waited)
	}
}

func TestALockALiveGateHoldsIsNotBrokenOnAge(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	dir := filepath.Join(home, lockName)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("laying out %s: %v — nothing was measured", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pid"),
		[]byte(strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
		t.Fatalf("writing this process's pid: %v — nothing was measured", err)
	}
	// Older than the abandoned bound. A gate that waits on a cold module fetch reaches this state
	// honestly, and breaking its lock puts two gates back on the machine.
	old := time.Now().Add(-2 * lockAbandonedAfter)
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatalf("ageing %s: %v — nothing was measured", dir, err)
	}

	if abandoned(dir) {
		t.Error("a lock held by this very process reads as abandoned once it is past the bound, so a " +
			"gate that runs long has its lock taken from under it")
	}
}

func TestALockThatWillNotComeAwayIsRefused(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	dir := filepath.Join(home, lockName)
	// A lock directory holding something os.Remove will not take, and no pid, aged past the bound so a
	// waiter tries to break it. Without the refusal the loop retries that removal forever.
	if err := os.MkdirAll(filepath.Join(dir, "leftover"), 0o755); err != nil {
		t.Fatalf("laying out %s: %v — nothing was measured", dir, err)
	}
	old := time.Now().Add(-2 * lockAbandonedAfter)
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatalf("ageing %s: %v — nothing was measured", dir, err)
	}

	done := make(chan error, 1)
	go func() {
		_, _, err := takeLock(home, testPoll)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a lock holding a stray file was reported as taken")
		}
		if !strings.Contains(err.Error(), "will not come away") {
			t.Errorf("the refusal says %q, which does not name the cause a reader has to act on", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("taking a lock that cannot be removed has not returned, so the gate spins here rather " +
			"than refusing")
	}
}

func TestOnlyTheHolderGivesUpTheLock(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	held, _, err := takeLock(home, testPoll)
	if err != nil {
		t.Fatalf("taking the lock: %v — nothing was measured", err)
	}
	// The lock as it stands after a break: the path holds a directory this process no longer owns.
	// release must leave it, or the gate that broke it would be running beside a third.
	stranger := 999999
	if err := os.WriteFile(filepath.Join(held.dir, "pid"),
		[]byte(strconv.Itoa(stranger)+"\n"), 0o644); err != nil {
		t.Fatalf("writing the new holder's pid: %v — nothing was measured", err)
	}

	held.release()

	after, err := os.ReadFile(filepath.Join(held.dir, "pid"))
	if err != nil || strings.TrimSpace(string(after)) != strconv.Itoa(stranger) {
		t.Errorf("release removed a lock this process no longer held (%v, %q), so the gate that took it "+
			"next has nothing keeping a third off the machine", err, after)
	}
}

// The budget is a claim about a cold run with the machine to itself. A gate that queued has not spent
// that time on this tree, so the report has to say the wait happened and the bound must not count it.
//
// The claim is read off the reported wall clock rather than by setting a budget between the two
// figures. A budget that tight is a race with the machine: the first shape of this case gave a
// trivial check one second and went red under a second gate, which is the defect this whole file is
// about, written into the case meant to prove it fixed.
func TestTheWaitForTheLockIsReportedAndLeftOutOfTheBudget(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.table("quick\ttrue")
	// Well above anything this check can take, so nothing here turns on how busy the machine is.
	f.budget = 600

	queued := 3 * time.Second
	dir := filepath.Join(f.lockDir, lockName)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("laying out %s: %v — nothing was measured", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pid"),
		[]byte(strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
		t.Fatalf("writing the holder's pid: %v — nothing was measured", err)
	}
	go func() {
		time.Sleep(queued)
		_ = os.Remove(filepath.Join(dir, "pid"))
		_ = os.Remove(dir)
	}()

	f.run()

	f.expectCode(0)
	if said := f.errOut.String(); !strings.Contains(said, "waited") {
		t.Errorf("the run queued behind another gate and never said so, so a reader reading the wall "+
			"clock cannot tell a slow suite from a busy machine\nstderr: %s", said)
	}
	reported := reportedWallClock(t, f.out.String())
	if reported >= queued {
		t.Errorf("the gate reports %s of wall clock after queueing %s, so the time it spent waiting for "+
			"another gate is being charged to this tree's budget", reported, queued)
	}
}

// The wall clock out of the summary line, which is the figure the budget is checked against and the
// figure a reader takes the run's cost from.
func reportedWallClock(t *testing.T, report string) time.Duration {
	t.Helper()
	found := wallClock.FindStringSubmatch(report)
	if found == nil {
		t.Fatalf("no wall clock in the report, so there is no figure to read\n%s", report)
	}
	seconds, err := strconv.Atoi(found[1])
	if err != nil {
		t.Fatalf("the report says %q seconds, which is not a number\n%s", found[1], report)
	}
	return time.Duration(seconds) * time.Second
}

var wallClock = regexp.MustCompile(`(\d+)s wall clock`)

package ecoreport_test

// The four shared records — one file per clone, written by every agent in every worktree. The subject
// is the write nobody can see fail: a lost entry leaves a well-formed file, so the first case proves
// the loss is observable before asserting it does not happen.

import (
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestConcurrentRecordWritesAllSurvive(t *testing.T) {
	t.Parallel()
	const writers = 24

	// The negative control, first: the hand-run read-modify-write these subcommands replace. Both
	// writers read the same bytes and then both write, which is what two agents in two worktrees do
	// when nothing serialises them. If this does not lose an entry, the case below proves nothing.
	lost := newRepo(t)
	lost.mkdirAll(lost.scratch())
	path := recordFile(lost, "project-decisions")
	lost.write(path, "1x | 2026-01-01 | seed\n")
	var readers, writes sync.WaitGroup
	readers.Add(2)
	writes.Add(2)
	for i := range 2 {
		go func() {
			defer writes.Done()
			held, _ := os.ReadFile(path)
			readers.Done()
			readers.Wait() // both have read; now both write, as two unsynchronised agents do
			_ = os.WriteFile(path, append(held, []byte("1x | 2026-01-01 | hand "+strconv.Itoa(i)+"\n")...), 0o600)
		}()
	}
	writes.Wait()
	after := lost.read(path)
	lost.record("negative control: an unserialised read-modify-write loses one of two entries",
		strings.Contains(after, "hand 0") != strings.Contains(after, "hand 1"), after)

	// The same shape through the tool. Each goroutine is its own invocation opening its own descriptor,
	// so they contend for the flock exactly as separate processes do.
	f := newRepo(t)
	f.runReport("record", "append", "project-decisions", "seed")
	var running sync.WaitGroup
	for i := range writers {
		running.Add(1)
		go func() {
			defer running.Done()
			f.invoke(f.repo, io.Discard, io.Discard, []string{"record", "append", "project-decisions", "writer " + strconv.Itoa(i)})
		}()
	}
	running.Wait()

	content := f.read(recordFile(f, "project-decisions"))
	missing := []string{}
	for i := range writers {
		if !strings.Contains(content, "| writer "+strconv.Itoa(i)+"\n") {
			missing = append(missing, strconv.Itoa(i))
		}
	}
	f.record("every concurrent append lands",
		len(missing) == 0, "missing writers: "+strings.Join(missing, ",")+"\n"+content)
	f.record("and the entry that was there before them all is still there",
		strings.Contains(content, "| seed\n"), content)
	f.record("and no line is a splice of two entries",
		strings.Count(content, "x | ") == writers+1, content)
}

func TestARecordWriteWaitsForTheLockRatherThanRacingIt(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.runReport("record", "append", "project-decisions", "already here")
	path := recordFile(f, "project-decisions")

	// Held SHARED, deliberately. A writer taking an exclusive lock must wait for it; one that took a
	// shared lock of its own would sail straight past. That is the difference this case exists to
	// see, and a test holding an exclusive lock could not see it — both would block on that.
	handle, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("could not open the record: %v", err)
	}
	if err := syscall.Flock(int(handle.Fd()), syscall.LOCK_SH); err != nil {
		t.Fatalf("could not lock the record: %v", err)
	}

	// The competing write announces which goroutine it is before starting, so the dump below can name
	// THIS write. The package runs its cases in parallel and several of them have a write in flight.
	writer := make(chan string)
	landed := make(chan struct{})
	go func() {
		writer <- ownGoroutineID()
		f.invoke(f.repo, io.Discard, io.Discard, []string{"record", "append", "project-decisions", "waited for the lock"})
		close(landed)
	}()

	parked, blocked := awaitTheWriteParkedOnTheLock(<-writer, landed)
	held := f.read(path)
	evidence := "the write ran to completion instead of waiting:\n" + held
	if blocked {
		evidence = parked + "\n" + held
	}
	f.record("a write waits while another holds the record",
		blocked && !strings.Contains(held, "waited for the lock"), evidence)

	if err := syscall.Flock(int(handle.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatalf("could not release the lock: %v", err)
	}
	handle.Close()
	select {
	case <-landed:
	case <-time.After(30 * time.Second):
		t.Fatal("the waiting write never completed after the lock was released")
	}
	f.record("and lands as soon as the lock is released",
		strings.Contains(f.read(path), "| waited for the lock\n"), f.read(path))
}

// The competing write, seen parked inside flock(2) on the record. This is the event the case above
// waits for, and nothing earlier will do: a write that has not been scheduled yet has appended nothing
// either, and from outside that is the same file as one the lock is holding back. The old case read
// that silence after a second as proof the lock held, and under a reproduction of concurrent-gate load
// it read it that way over a lock deliberately broken to exclude nothing.
//
// Parked on two separate dumps, not one. A flock that is about to return is also momentarily inside
// flock, and a single snapshot cannot tell it from one that will never return while this case holds the
// record. A busy machine spreads the two dumps further apart, so it makes a returning call harder to
// catch, not easier: the gap here erodes towards a red and never towards a pass.
//
// Reports false the moment the write finishes instead — which is what a lock excluding nothing does,
// and the only other end this loop has. Nothing here is bounded by the clock. A starved machine merely
// sends the loop round more times. The suite timeout the gate gives every package (budgetSeconds, the
// const in gate/gate.go) is what ends a run whose write never arrives at all.
func awaitTheWriteParkedOnTheLock(id string, landed <-chan struct{}) (string, bool) {
	confirmed := false
	for {
		select {
		case <-landed:
			return "", false
		default:
		}
		stack, parked := goroutineParkedInFlock(id)
		if parked && confirmed {
			return stack, true
		}
		confirmed = parked
		// A poll interval, not a bound: no assertion reads it, and every value reaches the same verdict.
		// It decides how promptly the loop notices, and nothing else.
		time.Sleep(time.Millisecond)
	}
}

// The dump of one goroutine, when that goroutine is inside flock(2). Blocks are separated by a blank
// line and each opens with `goroutine <id> [<state>]:`, so the id picks out exactly one of them.
func goroutineParkedInFlock(id string) (string, bool) {
	for _, block := range strings.Split(allGoroutines(), "\n\n") {
		if strings.HasPrefix(block, "goroutine "+id+" [") && strings.Contains(block, "syscall.Flock") {
			return block, true
		}
	}
	return "", false
}

func allGoroutines() string {
	buf := make([]byte, 1<<16)
	for {
		if n := runtime.Stack(buf, true); n < len(buf) {
			return string(buf[:n])
		}
		buf = make([]byte, 2*len(buf))
	}
}

// The calling goroutine's id, read off the header line runtime.Stack opens with: `goroutine 35
// [running]:`. Panics rather than returning nothing on a header it cannot read, because an empty id
// matches no block and the case waiting on one would hang until the suite timeout instead.
func ownGoroutineID() string {
	var buf [64]byte
	header := string(buf[:runtime.Stack(buf[:], false)])
	fields := strings.Fields(header)
	if len(fields) < 2 || fields[0] != "goroutine" {
		panic("runtime.Stack no longer opens with a goroutine id: " + header)
	}
	return fields[1]
}

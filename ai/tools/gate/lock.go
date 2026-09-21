// One gate at a time on a machine, and the budget measured on the run rather than the wait.
//
// The bound this gate enforces is a claim about a cold run with the machine to itself. That
// condition was written down and left to whoever remembered it. On a laptop carrying nine sessions
// it is never met by accident: two gates at once read 102s against a budget of 100s, on a tree whose
// own runs read 51s to 83s. A red that says only "another gate was running" teaches every session to
// re-run it and stop reading it, which costs more than the bound was ever worth.
//
// So the second gate waits for the first, and the clock starts when it gets the machine. The lock is
// machine-wide and lives outside any checkout, because the two gates are usually two worktrees.
package gate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	lockName = "kk-flavor-gate.lock"
	// How long a lock with no pid in it may sit before a waiter takes it for abandoned. A lock whose
	// pid is there and dead is taken at once, so this covers one window: a gate killed between making
	// the directory and writing its pid.
	lockAbandonedAfter = 5 * time.Minute
	// How long a gate queues before it refuses. Several gates can legitimately be in line, each up to
	// the budget, so this is well above it. A wait longer than this is a machine nobody is watching.
	lockWaitLimit = 15 * time.Minute
)

// heldLock is this process's own lock, and the pid that says so. release removes the directory only
// while the pid inside it is still this one: after a break the path holds somebody else's lock, and
// removing that is what would put two gates back on one machine.
type heldLock struct {
	dir string
	pid int
}

func (h *heldLock) release() {
	if h == nil {
		return
	}
	if held, err := os.ReadFile(filepath.Join(h.dir, "pid")); err != nil ||
		strings.TrimSpace(string(held)) != strconv.Itoa(h.pid) {
		return
	}
	_ = os.Remove(filepath.Join(h.dir, "pid"))
	_ = os.Remove(h.dir)
}

// lockHome is the directory the lock is made in. A caller naming one gets it, which is how the suite
// keeps its own cases off the machine's real lock. Otherwise the user's cache directory, which is
// per-user and outside every checkout, and the temp directory where there is no cache directory.
func lockHome(given string) string {
	if given != "" {
		return given
	}
	if cache, err := os.UserCacheDir(); err == nil {
		return cache
	}
	return os.TempDir()
}

// alive says whether a pid names a running process. Signal 0 delivers nothing and reports whether it
// could have. A pid this finds alive may still be a different process that inherited the number, and
// the cost of that is one gate waiting behind a stranger rather than two gates racing.
func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return !errors.Is(syscall.Kill(pid, syscall.Signal(0)), syscall.ESRCH)
}

// abandoned reads the lock and says whether a waiter may break it. A pid that is there and dead says
// so outright. A lock with no readable pid is the window between the directory being made and the
// pid being written, so that one is judged on age alone.
func abandoned(dir string) bool {
	held, err := os.ReadFile(filepath.Join(dir, "pid"))
	if err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(held))); err == nil {
			return !alive(pid)
		}
	}
	info, err := os.Stat(dir)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) > lockAbandonedAfter
}

// takeLock returns the lock, and how long this run queued for it. The wait is returned rather than
// printed, because what a caller does with it is the caller's: the gate reports it and leaves it out
// of the budget.
//
// announce is called once, when this run finds the lock held, and names who holds it. A silent queue
// is a gate that looks hung. The pid is what tells a running gate from a wedged one, and the deadline
// states the wait's own end before it begins.
//
// A lock that will not come away is a refusal. os.Remove takes an empty directory alone, so a lock
// holding a file this package never put there would otherwise spin here for the life of the process.
func takeLock(home string, poll time.Duration, announce func(string)) (*heldLock, time.Duration, error) {
	dir := filepath.Join(home, lockName)
	began := time.Now()
	said := false
	for {
		err := os.Mkdir(dir, 0o755)
		if err == nil {
			held := &heldLock{dir: dir, pid: os.Getpid()}
			// The pid goes in at once, so a waiter reading this lock finds an owner. A gate killed between
			// the line above and this one leaves it ownerless, which abandoned judges on age.
			_ = os.WriteFile(filepath.Join(dir, "pid"), []byte(strconv.Itoa(held.pid)+"\n"), 0o644)
			return held, time.Since(began), nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, time.Since(began), fmt.Errorf("cannot create %s: %v", dir, err)
		}
		if !said {
			said = true
			if announce != nil {
				announce(fmt.Sprintf("waiting for the gate already running on this machine (%s), "+
					"and giving up after %s", holderOf(dir), lockWaitLimit))
			}
		}
		if abandoned(dir) {
			_ = os.Remove(filepath.Join(dir, "pid"))
			if err := os.Remove(dir); err != nil {
				return nil, time.Since(began), fmt.Errorf(
					"%s is held by no live gate and will not come away (%v) — remove it by hand", dir, err)
			}
			continue
		}
		if time.Since(began) > lockWaitLimit {
			return nil, time.Since(began), fmt.Errorf(
				"waited %s for the gate running under %s (%s) and it has not finished",
				time.Since(began).Round(time.Second), dir, holderOf(dir))
		}
		time.Sleep(poll)
	}
}

// holderOf names who a reader would have to go and look at: the pid inside the lock, and how long the
// lock has been there. A lock carrying neither says so in those words. An empty phrase in a refusal
// sends the reader to this source file to find out what was meant.
func holderOf(dir string) string {
	pid := "no pid written yet"
	if held, err := os.ReadFile(filepath.Join(dir, "pid")); err == nil {
		pid = "pid " + strings.TrimSpace(string(held))
	}
	info, err := os.Stat(dir)
	if err != nil {
		return pid
	}
	return fmt.Sprintf("%s, held for %s", pid, time.Since(info.ModTime()).Round(time.Second))
}

// One gate at a time on a machine. The budget is measured on the run, and the wait is left out.

// The bound is a claim about a cold run with the machine to itself. The plan wrote that condition
// down and left it to whoever remembered it. A laptop carrying nine sessions meets it by accident
// only. Two gates at once read 102s against a budget of 100s, on a tree whose own runs read 51s.

// A red saying "another gate was running" teaches every session to re-run the gate and stop reading
// it. That costs more than the bound is worth. So the second gate waits for the first, and the clock
// starts when it gets the machine. The lock sits outside any checkout, since the two gates are
// usually two worktrees.
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
	// How long a pidless lock may sit before a waiter takes it for abandoned. A lock carrying a dead pid
	// is taken at once. The bound covers one window: a gate killed between making the directory and
	// writing its pid into it.
	lockAbandonedAfter = 5 * time.Minute
	// How long a gate queues before it refuses. Several gates can be in line, each up to the budget, so
	// the figure sits well over it. A longer wait means an unattended machine.
	lockWaitLimit = 15 * time.Minute
)

// heldLock is this process's own lock, and the pid that says so. release removes the directory while
// the pid inside it is still this one. After a break the path holds another gate's lock, and removing
// that puts two gates back on one machine.
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

// lockHome is the directory the lock is made in. A caller naming one gets it, and the suite names its
// own to keep its cases off the machine's real lock. The default is the user's cache directory. It is
// per-user and sits outside every checkout, and the temp directory stands in where it is unavailable.
func lockHome(given string) string {
	if given != "" {
		return given
	}
	if cache, err := os.UserCacheDir(); err == nil {
		return cache
	}
	return os.TempDir()
}

// alive says whether a pid names a running process. Signal 0 delivers no signal and reports whether it
// could have. A pid this finds alive may be a different process that inherited the number. The cost
// there is one gate waiting behind a stranger, against two gates racing.
func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return !errors.Is(syscall.Kill(pid, syscall.Signal(0)), syscall.ESRCH)
}

// abandoned reads the lock and says whether a waiter may break it. A dead pid says so outright. A lock
// holding an unreadable pid sits in the window between the directory being made and the pid being
// written, and age alone judges that one.
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

// takeLock, the entry point here, returns the lock and how long this run queued for it. The wait comes back as a value, and
// what a caller does with it is the caller's. The gate reports it and leaves it out of the budget.

// announce is called once, when this run finds the lock held, and names who holds it. A silent queue
// is a gate that looks hung. The pid tells a running gate from a wedged one, and the deadline states
// the wait's own end before it begins.

// A lock that will not come away is a refusal. os.Remove takes an empty directory alone. A lock
// holding a stray file would spin here for the life of the process.
func takeLock(home string, poll time.Duration, announce func(string)) (*heldLock, time.Duration, error) {
	dir := filepath.Join(home, lockName)
	began := time.Now()
	said := false
	for {
		err := os.Mkdir(dir, 0o755)
		if err == nil {
			held := &heldLock{dir: dir, pid: os.Getpid()}
			// The pid goes in at once, so a waiter reading this lock finds an owner. A gate killed between
			// taking the lock and this write leaves it ownerless, and abandoned judges that on age.
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

// holderOf names who a reader would go and look at: the pid inside the lock, and how long the lock has
// been there. A lock carrying neither says so in those words. An empty phrase in a refusal sends the
// reader to this source file to find out what was meant.
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

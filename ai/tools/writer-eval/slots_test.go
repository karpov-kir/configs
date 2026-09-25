package writereval

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// A table's calls share one account, and so do two tables run at once. Two runs on 2026-09-22 came back
// `error x15` on most calls, which read like a rule that broke everything. Measured on 2026-09-25, 72
// calls took 366s at 4 workers, 195s at 8, 117s at 16 and 81s at 32, with no error at any count, and a
// call's mean time rose from 20s to 29s as the account queued them. So each run starts 16 workers, and
// every call of every run on the machine takes one of 32 slots. Two runs then share the account's
// capacity and never pass it.
const (
	defaultWorkers = 16
	slotsEnv       = "WRITER_EVAL_SLOTS"
	defaultSlots   = 32
)

// machineSlots is how many calls every run on this machine may have in flight together.
func machineSlots() int {
	if at, err := strconv.Atoi(os.Getenv(slotsEnv)); err == nil && at > 0 {
		return at
	}
	return defaultSlots
}

// slotDir holds one lock file per slot, in the user's cache.
func slotDir() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "kk-flavor", "writer-eval-slots")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "kk-flavor", "writer-eval-slots")
}

// takeSlot holds one machine-wide slot until the returned release runs. A lock the kernel holds is
// released with the process, so a killed run frees its slots.
func takeSlot() (func(), error) {
	dir := slotDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	for {
		for n := 0; n < machineSlots(); n++ {
			file, err := os.OpenFile(filepath.Join(dir, fmt.Sprintf("slot-%d", n)), os.O_CREATE|os.O_RDWR, 0o644)
			if err != nil {
				return nil, err
			}
			if syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) == nil {
				return func() {
					syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
					file.Close()
				}, nil
			}
			file.Close()
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// Slots bound the calls in flight across every run: with two slots, a third taker waits for a release.
func TestSlotsBoundTheCallsInFlightAcrossRuns(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv(slotsEnv, "2")
	first, err := takeSlot()
	if err != nil {
		t.Fatal(err)
	}
	second, err := takeSlot()
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan func(), 1)
	go func() {
		third, _ := takeSlot()
		got <- third
	}()
	select {
	case <-got:
		t.Fatal("a third call took a slot while two were held")
	case <-time.After(500 * time.Millisecond):
	}
	first()
	select {
	case third := <-got:
		third()
	case <-time.After(2 * time.Second):
		t.Fatal("a released slot was never taken")
	}
	second()
}

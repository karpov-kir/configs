package aibootstrap

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// The private-name guard was retired: the owner judged a list of names a fragile guard, and review is
// what caught the pull request it was built after. Its install pointed the configs checkout's git hooks
// at ai/hooks and created an empty list under the config directory. This undoes both on a machine that
// ran it.

// retiredHooksPath is where the retired guard pointed the checkout's hooks.
const retiredHooksPath = "ai/hooks"

// removeRetiredPrivateNameGuard unsets the hooks path the guard set and removes its list where the list
// holds no entry. A list its owner filled is kept and named, since its content is the owner's.
func (run *invocation) removeRetiredPrivateNameGuard() {
	if !run.isOwner {
		return
	}
	checkout := filepath.Dir(run.Repo)
	list := filepath.Join(run.ConfigHome, "kk-flavor", "private-names.txt")
	set, _ := exec.Command("git", "-C", checkout, "config", "--get", "core.hooksPath").Output()
	hooked := strings.TrimSpace(string(set)) == retiredHooksPath
	listed := fileExists(list)
	if !hooked && !listed {
		return
	}
	run.mounting.Say("retired private-name guard")
	if hooked {
		switch {
		case run.isDryRun:
			run.mounting.Say("  would unset core.hooksPath in " + checkout)
		case exec.Command("git", "-C", checkout, "config", "--unset", "core.hooksPath").Run() != nil:
			run.mounting.Refuse("could not unset core.hooksPath in " + checkout)
		default:
			run.mounting.Say("  unset    core.hooksPath in " + checkout + ", which pointed at the retired hooks")
		}
	}
	if listed {
		entries := listEntries(list)
		switch {
		case entries > 0:
			run.mounting.Say("  kept     " + list + ": it holds " + strconv.Itoa(entries) + " entr(ies), which are its owner's to remove")
		case run.isDryRun:
			run.mounting.Say("  would remove " + list + ", which holds no entry")
		case os.Remove(list) != nil:
			run.mounting.Refuse("could not remove " + list)
		default:
			run.mounting.Say("  removed  " + list + ", which held no entry")
		}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// listEntries is how many entries the list holds, comments and blank lines aside.
func listEntries(path string) int {
	file, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer file.Close()
	n := 0
	lines := bufio.NewScanner(file)
	for lines.Scan() {
		if line := strings.TrimSpace(lines.Text()); line != "" && !strings.HasPrefix(line, "#") {
			n++
		}
	}
	return n
}

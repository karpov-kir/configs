package aibootstrap

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"configs/ai/tools/shell"
	voicecheck "configs/ai/tools/voice-check"
)

// hooksPath is where the configs checkout keeps its git hooks, relative to the checkout.
const hooksPath = "ai/hooks"

// privateNamesList is the owner's private-name list, under the config directory and outside every
// repository.
func (run *invocation) privateNamesList() string {
	return filepath.Join(run.ConfigHome, "kk-flavor", "private-names.txt")
}

// guardPrivateNames creates the owner's private-name list where none stands and points this checkout's
// git hooks at the ones that read it. The configs repository is public, and a pull request once carried
// a private codebase's names that no check read. The register scan and the gate read the list on their
// own; the hooks reach the commit message the gate runs before and a push of commits made elsewhere.
func (run *invocation) guardPrivateNames() {
	if !run.isOwner {
		return
	}
	run.mounting.Say("private names")
	list := run.privateNamesList()
	switch {
	case shell.PathExists(list):
		run.mounting.Say("  ok       " + list + " holds " + strconv.Itoa(countEntries(list)) + " entr(ies)")
	case run.isDryRun:
		run.mounting.Say("  would create " + list + ", empty")
	default:
		if err := os.MkdirAll(filepath.Dir(list), 0o755); err != nil {
			run.mounting.Refuse("could not create " + filepath.Dir(list))
			return
		}
		if err := os.WriteFile(list, []byte(voicecheck.PrivateNamesHeader), 0o600); err != nil {
			run.mounting.Refuse("could not write " + list)
			return
		}
		run.mounting.Say("  created  " + list + ", empty: its owner adds the names no public text may hold")
	}
	checkout := filepath.Dir(run.Repo)
	if exec.Command("git", "-C", checkout, "rev-parse", "--is-inside-work-tree").Run() != nil {
		run.mounting.Say("  ok       " + checkout + " is no git checkout, so it takes no hooks")
		return
	}
	current, _ := exec.Command("git", "-C", checkout, "config", "--get", "core.hooksPath").Output()
	switch set := strings.TrimSpace(string(current)); {
	case set == hooksPath:
		run.mounting.Say("  ok       " + checkout + " runs the commit-msg and pre-push hooks in " + hooksPath)
	case set != "":
		run.mounting.Refuse(checkout + "'s core.hooksPath is " + set + ", so the private-name hooks in " + hooksPath +
			" do not run; point it at " + hooksPath + " or call them from there")
	case run.isDryRun:
		run.mounting.Say("  would set core.hooksPath to " + hooksPath + " in " + checkout)
	default:
		if err := exec.Command("git", "-C", checkout, "config", "core.hooksPath", hooksPath).Run(); err != nil {
			run.mounting.Refuse("could not set core.hooksPath in " + checkout)
			return
		}
		run.mounting.Say("  set      core.hooksPath to " + hooksPath + " in " + checkout + ", so commit-msg and pre-push read the list")
	}
}

// countEntries is how many entries the list holds, comments and blank lines aside.
func countEntries(path string) int {
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

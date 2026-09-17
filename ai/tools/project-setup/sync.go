package projectsetup

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// The post-checkout entry point: restore the skill links one worktree should have, for whichever
// clients this clone was installed for. It never installs dependencies or instructions — a checkout is
// not an install, and a hook that rewrote a project's tracked files on every branch switch would be
// unusable.
//
//	usage: project-skills.sh --sync <worktree>
const syncStubPath = "project-skills.sh"

// SyncOptions is everything the sync needs that it must not go looking for itself.
type SyncOptions struct {
	Self string
	Args []string
	Repo string
	Home string
	// ConfigHome is where the install registry lives. Unread by the sync and taken anyway, because the
	// machinery reads it and a run built without it would reach the developer's own.
	ConfigHome string
	Git        Git
	Out        io.Writer
	Err        io.Writer
	WriteRoot  string
}

// Sync executes one `--sync` invocation and answers its exit code.
func Sync(options SyncOptions) int {
	if len(options.Args) != 2 || options.Args[0] != "--sync" {
		fmt.Fprintf(options.Err, "%s: name one worktree to sync\n", options.Self)
		fmt.Fprintln(options.Err, "usage: "+syncStubPath+" --sync <worktree>")
		return exitBadUsage
	}
	run := &invocation{
		Options: Options{
			Self: options.Self, Repo: options.Repo, Home: options.Home, ConfigHome: options.ConfigHome,
			Git: options.Git, Out: options.Out, Err: options.Err, WriteRoot: options.WriteRoot,
		},
		arguments: arguments{project: options.Args[1]},
		targets:   newTargets(claudeAgent, options.Args[1]),
	}
	run.mounting = run.newMountingRun(options.Out, false)

	paths, isRepository := run.gitPaths(run.project)
	if !isRepository {
		fmt.Fprintf(options.Err, "%s is not a Git worktree\n", run.project)
		return 1
	}
	// The worktree root rather than whatever directory inside it the hook was run from — git runs a
	// post-checkout hook at the root, but a human running this by hand need not be standing there.
	root, err := options.Git.TopLevel(run.project)
	if err != nil {
		fmt.Fprintf(options.Err, "%s is not a Git worktree\n", run.project)
		return 1
	}
	run.arguments.project = root
	run.targets = newTargets(claudeAgent, root)

	code := exitDone
	for _, agent := range []string{claudeAgent, codexAgent} {
		tier, isInstalled := readTier(paths, agent)
		if !isInstalled {
			continue
		}
		if tier == unreadableTier {
			fmt.Fprintf(options.Err, "Invalid project skill setup: %s\n", paths.state+"/"+agent)
			return 1
		}
		if !run.syncTree(paths, root, agent, tier == maintainerTier) {
			code = 1
		}
	}
	return code
}

// What one client's state file says the tier was. Three answers rather than two: a file holding
// anything but the two words this writes is a file somebody edited or a write that was interrupted,
// and guessing a tier from it would mount a set nobody chose.
type tier int

const (
	plainTier tier = iota
	maintainerTier
	unreadableTier
)

func readTier(paths gitPaths, agent string) (tier, bool) {
	body, err := os.ReadFile(paths.state + "/" + agent)
	if err != nil {
		return plainTier, false
	}
	switch strings.TrimSpace(string(body)) {
	case "true":
		return maintainerTier, true
	case "false":
		return plainTier, true
	default:
		return unreadableTier, true
	}
}

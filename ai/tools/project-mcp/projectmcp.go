// Configure the public MCP servers in one project's own client files, and never in user settings.
//
//	usage: project-mcp.sh --agent=claude|codex [--dry-run] [--uninstall] <project>
//
// The servers come from `ai/mcp.jsonc`, the committed half of the declaration. `ai/mcp.private.jsonc`
// is never read here: a project file is committed and reviewed, so anything carrying a credential or
// naming an internal host belongs only in the user-scope sync.
//
// What lands in the project is not what the user scope registers. A user-scope entry names this
// checkout by absolute path; a project file is shared with everyone who clones the project, so each
// server is rewritten to reach the launcher through `$HOME/.kk-flavor` — the one path every install
// of this flavor has.
package projectmcp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
)

// Exit codes on the tools' shared vocabulary. 2 is a grammar this tool did not understand, so nothing
// was attempted; 1 is a project or a config file it will not write.
const (
	exitDone     = 0
	exitRefused  = 1
	exitBadUsage = 2
)

const claudeAgent = "claude"

const codexAgent = "codex"

var agents = []string{claudeAgent, codexAgent}

// Where each client keeps a project's own server list.
var configFiles = map[string]string{
	claudeAgent: ".mcp.json",
	codexAgent:  ".codex/config.toml",
}

// Every message a human reads from this tool, whichever client it was asked about.
const label = "project MCP"

// Run executes one invocation and returns its exit code.
//
// `configsDir` is the directory holding `mcp.jsonc` — the tool's own, resolved from the path the stub
// was invoked by. `home` is the home directory the refusals below compare against; taken as a value
// rather than read from the environment so no case in the suite can reach the owner's own.
func Run(self string, args []string, configsDir, home string, git repo.Git, stdout, stderr io.Writer) int {
	run, code, hasStopped := parseArguments(self, args, stdout, stderr)
	if hasStopped {
		return code
	}
	run.configsDir, run.home, run.git = configsDir, home, git
	if err := run.do(stdout); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", label, err)
		return exitRefused
	}
	return exitDone
}

// One invocation's fixed context. Held together because every step needs the project and the agent,
// and a run reading one project while writing another's file is the inconsistency the two must not be
// able to express.
type invocation struct {
	agent       string
	project     string
	isDryRun    bool
	isUninstall bool

	configsDir string
	home       string
	git        repo.Git
}

// The third value says whether the run stops here — a refused invocation and a printed help both do,
// and they exit differently. Read as a code alone, exit 0 from the help arm is indistinguishable from
// "parsed fine, carry on", which is how an empty invocation reaches the filesystem.
func parseArguments(self string, args []string, stdout, stderr io.Writer) (invocation, int, bool) {
	run := invocation{}
	for _, arg := range args {
		switch {
		case arg == "-h" || arg == "--help":
			fmt.Fprintln(stdout, usage(self))
			return run, exitDone, true
		case strings.HasPrefix(arg, "--agent=") && isKnownAgent(strings.TrimPrefix(arg, "--agent=")):
			run.agent = strings.TrimPrefix(arg, "--agent=")
		case arg == "--dry-run":
			run.isDryRun = true
		case arg == "--uninstall":
			run.isUninstall = true
		case strings.HasPrefix(arg, "-"):
			return run, badUsage(self, stderr, "unknown option %s", arg), true
		case run.project != "":
			return run, badUsage(self, stderr, "select one project"), true
		default:
			run.project = arg
		}
	}
	if run.agent == "" || run.project == "" {
		return run, badUsage(self, stderr,
			"--agent=%s and an existing project directory are required", selector()), true
	}
	if info, err := os.Stat(run.project); err != nil || !info.IsDir() {
		return run, badUsage(self, stderr,
			"--agent=%s and an existing project directory are required", selector()), true
	}
	return run, exitDone, false
}

// A refusal goes to stderr, because a caller capturing stdout is reading the line naming the file
// this tool checked. The help arm is the exception and prints where a human asked for it.
func badUsage(self string, stderr io.Writer, format string, args ...any) int {
	fmt.Fprintf(stderr, "%s: %s\n", label, fmt.Sprintf(format, args...))
	fmt.Fprintln(stderr, usage(self))
	return exitBadUsage
}

func usage(self string) string {
	return fmt.Sprintf("usage: %s --agent=%s [--dry-run] [--uninstall] <project>", self, selector())
}

func selector() string {
	return strings.Join(agents, "|")
}

func isKnownAgent(agent string) bool {
	for _, known := range agents {
		if agent == known {
			return true
		}
	}
	return false
}

// The whole of one run: resolve the project, work out what the file should hold, and write it.
func (run *invocation) do(stdout io.Writer) error {
	// Absolute as well as symlink-free, because the refusals below compare this against paths that
	// already are: `.` is what a human standing in their home types, and a relative spelling reaching
	// the home guard matches nothing it is guarding.
	project, err := shell.RealPath(run.project)
	if err != nil {
		return err
	}
	run.project = project
	if err = run.refuseHome(); err != nil {
		return err
	}
	if err = run.refuseCodexDirectoryLink(); err != nil {
		return err
	}

	file := filepath.Join(run.project, filepath.FromSlash(configFiles[run.agent]))
	if !run.isUninstall {
		if err = run.refuseUntrackedConfig(file); err != nil {
			return err
		}
	}
	servers, err := readPublicServers(run.configsDir)
	if err != nil {
		return err
	}
	previous, err := readConfig(file)
	if err != nil {
		return err
	}
	text, err := run.merge(previous, servers)
	if err != nil {
		return err
	}
	if !run.isDryRun {
		if err = writeConfig(file, text, previous); err != nil {
			return err
		}
	}
	verb := "checked"
	if run.isDryRun {
		verb = "would update"
	}
	fmt.Fprintf(stdout, "%s: %s %s\n", label, verb, file)
	return nil
}

func (run *invocation) merge(previous string, servers []projectServer) (string, error) {
	if run.agent == claudeAgent {
		return mergeClaude(previous, servers, run.isUninstall)
	}
	return mergeCodex(previous, servers, run.isUninstall)
}

// The home directory is where the user-scope sync writes, and this tool writes project files. Given
// one as the project it would put a project's server list into the human's home, where every client
// reads it as a project it is standing in.
func (run *invocation) refuseHome() error {
	if run.home == "" {
		return nil
	}
	home, err := shell.RealPath(run.home)
	if err != nil {
		return nil
	}
	if run.project == home {
		return fmt.Errorf("the home directory is not a project target")
	}
	return nil
}

// Codex's config sits one directory down, so this tool may have to create `.codex/`. A link there
// points that write outside the project — at another project's configuration, or at anything else the
// link names — so a `.codex` that is not a real directory stops the run.
func (run *invocation) refuseCodexDirectoryLink() error {
	if run.agent != codexAgent {
		return nil
	}
	directory := filepath.Join(run.project, ".codex")
	info, err := os.Lstat(directory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf(".codex must be a real directory, not a link")
	}
	return nil
}

// A project's MCP configuration is meant to be committed and reviewed — that is the whole difference
// between it and the user-scope registry. Written into a path the project ignores, it reaches nobody
// else and silently differs from what every other clone has.
//
// The ignore rules are the project's, so they are reported rather than rewritten. Uninstalling skips
// this: removing servers from a file the project ignores is still worth doing.
func (run *invocation) refuseUntrackedConfig(file string) error {
	if _, err := run.git.TopLevel(run.project); err != nil {
		return nil
	}
	relative, err := filepath.Rel(run.project, file)
	if err != nil {
		return err
	}
	relative = filepath.ToSlash(relative)
	ignored, err := run.git.Ignored(run.project, []string{relative})
	if err != nil {
		return fmt.Errorf("cannot check whether Git will track %s", relative)
	}
	if ignored[relative] {
		return fmt.Errorf("%s is ignored by Git; adjust the project ignore rules to include this "+
			"configuration, then rerun and review it before committing", relative)
	}
	return nil
}

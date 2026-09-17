package projectsetup

// Where one client keeps what this installs inside a project, and what the ignore region around it is
// fenced with. Resolved once at the top of a run, because every step after it reads several of these
// and a step deriving its own would be a second answer to drift from this one.
//
// The two clients share the project's instruction files and have a skills directory and an ignore
// region each — which is what lets one be installed and removed while the other stays.
type targets struct {
	agentDirectory      string
	otherAgentDirectory string
	skillsMount         string
	// instructionsFile is the shared one both clients load; claudeFile imports it rather than carrying a
	// second copy, so a project's two clients cannot drift apart.
	instructionsFile string
	claudeFile       string
	ignoreFile       string
	ignoreOpen       string
	ignoreClose      string
}

// What the Claude file holds: an import of the shared instructions and nothing else of this run's.
const claudeImport = "@AGENTS.md"

func newTargets(agent, project string) targets {
	built := targets{
		agentDirectory:      ".claude",
		otherAgentDirectory: ".agents",
		// A fence per client, so an uninstall takes its own rules and leaves the other's. One shared
		// region would make the first uninstall unhide the second client's mounts.
		ignoreOpen:  "# kk-flavor:begin",
		ignoreClose: "# kk-flavor:end",
	}
	if agent == codexAgent {
		built.agentDirectory = ".agents"
		built.otherAgentDirectory = ".claude"
		built.ignoreOpen = "# kk-flavor-codex:begin"
		built.ignoreClose = "# kk-flavor-codex:end"
	}
	built.skillsMount = project + "/" + built.agentDirectory + "/skills"
	built.instructionsFile = project + "/AGENTS.md"
	built.claudeFile = project + "/CLAUDE.md"
	built.ignoreFile = project + "/.gitignore"
	return built
}

// The rules that hide this client's mounts. Two prefixes rather than the whole skills directory: a
// project's own skills live there too, and ignoring the directory would hide those from its history.
func (t targets) ignoreBody() string {
	return t.agentDirectory + "/skills/kk-*\n" + t.agentDirectory + "/skills/idsd-*"
}

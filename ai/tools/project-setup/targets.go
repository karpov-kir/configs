package projectsetup

// Where one client keeps what this installs, and how the ignore region around it is fenced. A run
// resolves these once at the top: every later step reads several of them, and a step deriving its own
// would be a second answer that can disagree. The clients share the project's instruction files and
// hold a skills directory and an ignore region each, which is what lets one go and the other stay.
type targets struct {
	agentDirectory      string
	otherAgentDirectory string
	skillsMount         string
	// instructionsFile is the shared file both clients load. claudeFile imports it and holds no second
	// copy, so the two clients cannot disagree about the instructions.
	instructionsFile string
	claudeFile       string
	ignoreFile       string
	ignoreOpen       string
	ignoreClose      string
}

// What the Claude file holds of this run's: an import of the shared instructions, and that alone.
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

// The rules that hide this client's mounts. Two prefixes cover them, since a project's own skills
// live in that directory too and ignoring it whole would hide those from the project's history.
func (t targets) ignoreBody() string {
	return t.agentDirectory + "/skills/kk-*\n" + t.agentDirectory + "/skills/idsd-*"
}

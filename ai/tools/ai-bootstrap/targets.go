package aibootstrap

import (
	"os"

	"configs/ai/tools/shell"
)

// Where one client keeps what this installs, resolved once at the top of a run. Every step after
// that reads both paths, and a step deriving its own could disagree with this one.
//
// Codex's paths follow OpenAI's own discovery: skills under ~/.agents/skills, instructions in the
// profile CODEX_HOME names. Claude's are its own two.
type targets struct {
	skillsMount     string
	instructionFile string
	// Every directory a skill of this checkout's could be mounted in, for the uninstall's question of
	// whether another client still needs the bucket.
	allSkillMounts []string
}

func newTargets(agent string, options Options) targets {
	built := targets{
		skillsMount:     options.Home + "/.claude/skills",
		instructionFile: options.Home + "/.claude/CLAUDE.md",
		allSkillMounts: []string{
			options.Home + "/.claude/skills",
			options.Home + "/.agents/skills",
		},
	}
	if agent == codexAgent {
		built.skillsMount = options.Home + "/.agents/skills"
		built.instructionFile = options.CodexHome + "/AGENTS.md"
	}
	return built
}

// Whether two paths refer to the same directory even when one path is a symlink.
func isSameDirectory(one, other string) bool {
	first, err := os.Stat(one)
	if err != nil {
		return false
	}
	second, err := os.Stat(other)
	if err != nil {
		return false
	}
	return os.SameFile(first, second)
}

// Every symlink directly under a directory, by full path. Empty where the directory is missing or
// unreadable. That is what a machine with no mounts of this kind looks like.
func mountedLinks(directory string) []string {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil
	}
	var links []string
	for _, entry := range entries {
		path := shell.Join(directory, entry.Name())
		if shell.IsSymlink(path) {
			links = append(links, path)
		}
	}
	return links
}

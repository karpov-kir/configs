package installer

import (
	"os"
	"strings"

	"configs/ai/tools/shell"
)

// Which projects this machine installed the agent tree into. One fact, written by the project
// installer and read by both uninstallers — the project one to drop its own entry, the machine-wide
// one to say whether removing `~/.kk-flavor` would pull the bucket out from under a project that is
// still using it.
//
// It does not know what a mount is. It prunes on one condition — the recorded directory is gone —
// because that is the only staleness a registry can detect on its own. "Recorded but no longer holds
// the mounts" is the caller's filter over what LiveInstalls answers, since only the caller knows what
// its own install put there.

// RegistryFile is where the record lives: the path ecosystem.md → Conventions a new file joins sets
// for a machine-local file, and the one ai/tools/bloat-judge/deadline.go already reads.
func (r *Run) RegistryFile() string {
	return r.configHome + "/kk-flavor/installs"
}

// LiveInstalls is the recorded projects that still exist, having rewritten the file to drop the ones
// that do not.
//
// Pruning and reading are one call rather than two on purpose: a caller able to read without pruning
// is a caller able to get the stale answer. Here that answer says "another project still needs the
// bucket" about a directory the human deleted months ago, and uninstall then refuses to finish over
// something that is not true any more.
//
// A missing registry is not an error: it is a machine that has installed into no project.
func (r *Run) LiveInstalls() []string {
	file := r.RegistryFile()
	content, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var live []string
	var kept []string
	for _, line := range shell.SplitLines(string(content)) {
		// A blank line or a comment is not an entry, so it is not answered — but it is kept, because a
		// human who opens this file to see what is in it may well annotate it, and eating their note
		// while pruning dead projects is a poor answer to a question they did not ask.
		trimmed := strings.TrimLeft(line, shell.SpaceBytes)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			kept = append(kept, line)
			continue
		}
		if !shell.IsDir(line) {
			continue
		}
		kept = append(kept, line)
		live = append(live, line)
	}

	// Rewritten only when it would change, so a read on a healthy registry writes nothing at all and a
	// dry run never has to be special-cased here.
	if rewritten := joinLines(kept); string(rewritten) != string(content) && !r.dryRun {
		r.tree.replaceFile(file, rewritten)
	}
	return live
}

// RecordInstall adds project to the registry, answering false only where the write was refused.
// Idempotent: a second install into the same directory leaves one line.
func (r *Run) RecordInstall(project string) bool {
	file := r.RegistryFile()
	if contains(r.LiveInstalls(), project) {
		r.Say("  ok       " + project + " is already recorded in " + file)
		return true
	}
	directory := shell.DirName(file)
	return r.apply(change{
		would: "would record " + project + " in " + file,
		did:   "recorded " + project + " in " + file,
		write: func() string {
			if !shell.IsDir(directory) {
				if err := r.tree.mkdirAll(directory); err != nil {
					return "could not create " + directory + ", so " + project +
						" was not recorded — uninstall will not know about it: " + err.Error()
				}
			}
			if err := r.tree.appendLine(file, project); err != nil {
				return "could not record " + project + " in " + file +
					" — uninstall will not know about it: " + err.Error()
			}
			return ""
		},
	})
}

// ForgetInstall drops project from the registry, answering false only where the write was refused.
// Absent is success: an uninstall run twice has nothing to do the second time.
func (r *Run) ForgetInstall(project string) bool {
	file := r.RegistryFile()
	if !shell.IsRegularFile(file) {
		r.Say("  ok       nothing recorded to forget")
		return true
	}
	if !contains(r.LiveInstalls(), project) {
		r.Say("  ok       " + project + " was not recorded")
		return true
	}
	return r.apply(change{
		would: "would forget " + project + " in " + file,
		did:   "forgot   " + project + " in " + file,
		write: func() string {
			// Read from the file rather than from LiveInstalls, whose answer is projects only:
			// rewriting from that would drop the annotations the read above deliberately preserves.
			content, err := os.ReadFile(file)
			if err != nil {
				return "could not read " + file + " — " + project + " is still recorded: " + err.Error()
			}
			var kept []string
			for _, line := range shell.SplitLines(string(content)) {
				if line != project {
					kept = append(kept, line)
				}
			}
			if err := r.tree.replaceFile(file, joinLines(kept)); err != nil {
				return "could not rewrite " + file + " — " + project + " is still recorded"
			}
			return ""
		},
	})
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

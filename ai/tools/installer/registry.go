package installer

import (
	"os"
	"strings"

	"configs/ai/tools/shell"
)

// Which projects this machine installed the agent tree into. The project installer writes the
// record. Both uninstallers read it. The project one drops its own entry. The machine-wide one says
// whether removing `~/.kk-flavor` would pull the bucket out from under a project still using it.

// It does not know what a mount is. It prunes on one condition, a recorded directory that is gone,
// because that is the only staleness a registry can detect on its own. "Recorded but no longer holds
// the mounts" is the caller's filter over what LiveInstalls answers, since the caller alone knows
// what its own install put there.

// RegistryFile is where the record lives. ai/kk-flavor/standards/ecosystem.md puts a machine-local
// file under `${XDG_CONFIG_HOME:-~/.config}/kk-flavor/`, and ai/tools/reader-judge/deadline.go
// already reads one from there.
func (r *Run) RegistryFile() string {
	return r.configHome + "/kk-flavor/installs"
}

// The prune and the read are one call on purpose. A caller able to read without pruning is a caller
// able to get the stale answer. That answer says "another project still needs the bucket" about a
// directory the human deleted months ago, and uninstall then refuses to finish over something
// untrue.

// A missing registry is no error. It is a machine that has installed into no project.

// LiveInstalls is the recorded projects that still exist. It rewrites the file first, dropping every
// project that has gone.
func (r *Run) LiveInstalls() []string {
	file := r.RegistryFile()
	content, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var live []string
	var kept []string
	for _, line := range shell.SplitLines(string(content)) {
		// A blank line or a comment is no entry, so it goes unanswered. It is kept, because a human who
		// opens this file to see what is in it may well annotate it. A note eaten during a prune is a
		// poor answer to a question they did not ask.
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

	// The file is rewritten only when it would change, so a read on a healthy registry leaves it
	// alone and a dry run never has to be special-cased here.
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
// Absent is success, and an uninstall run twice finds its work already done.
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
			// The lines come from the file, because LiveInstalls answers projects alone. A rewrite from
			// that answer would drop the annotations LiveInstalls deliberately preserves.
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

package ecoreport

import (
	"os"
	"strings"
)

// The remaining decision marker and the closed stage-entry vocabulary.

const stageNames = string(resultCodeReview) + " " + string(resultSecurityReview) + " " + string(resultEdit) + " " + string(resultRefactor)

func stageList() []string { return strings.Fields(stageNames) }

// A marker never written while the caller prints "recorded" is a stage the stamp waves through.
func (r *run) writeStageMarker(stage, value string) {
	if value == "" {
		r.errLines("error: an empty marker cannot be recorded — " + stage + " is NOT marked")
		r.exit(2)
	}
	// 0700/0600, not the umask's answer. These markers are what `stamp` reads instead of re-checking a
	// stage, so a mode any other local account can write is a merge precondition anyone on the machine
	// can forge. Matches the scratch record tree, which is 0700 for the same reason.
	if err := os.MkdirAll(r.stageReturnsDir, 0o700); err != nil {
		r.errLines("error: could not create " + r.stageReturnsDir + " (" + err.Error() + ") — " + stage + " is NOT marked")
		r.exit(2)
	}
	marker := r.stageReturnsDir + "/" + stage
	if err := os.WriteFile(marker, []byte(value+"\n"), 0o600); err != nil {
		r.errLines("error: could not write " + marker + " (" + err.Error() + ") — " + stage + " is NOT marked")
		r.exit(2)
	}
}

func (r *run) hasPassMarker(stage string) bool {
	info, err := os.Stat(r.stageReturnsDir + "/" + stage)
	return err == nil && info.Mode().IsRegular()
}

// The stage record's grammar. Only `turnaround` reaches turnaroundTrims, so any other word for a turnaround trim
// would stamp a trimmed pass as untrimmed — which is why the vocabulary is closed rather than pattern-matched.
func validateStampEntries(entries string) []string {
	var problems []string
	seen := map[string]int{}
	for _, entry := range strings.Split(entries, ",") {
		stage, ok := stageOfEntry(entry)
		if !ok {
			problems = append(problems, "malformed entry: "+entry)
			continue
		}
		seen[stage]++
	}
	for _, stage := range stageList() {
		switch {
		case seen[stage] == 0:
			problems = append(problems, "missing stage: "+stage)
		case seen[stage] > 1:
			problems = append(problems, "duplicate stage: "+stage)
		}
	}
	return problems
}

func stageOfEntry(entry string) (string, bool) {
	switch entry {
	case "code-review":
		return "code-review", true
	case "refactor", "refactor:partial(turnaround)", "refactor:partial(cap)", "refactor:skipped(not-applicable)":
		// partial = the loop ended non-compliant, which is a record of what ran, not a trim.
		return "refactor", true
	}
	for _, stage := range []string{"security-review", "edit"} {
		if entry == stage || entry == stage+":skipped(turnaround)" || entry == stage+":skipped(not-applicable)" {
			return stage, true
		}
	}
	return "", false
}

package ecoreport

import (
	"fmt"
	"os"
	"strings"

	"kk-flavor/tools/shell"
)

// The stage markers, and the grammar `stamp` reads its stage record in. A marker holds the report's
// checksum as that stage returned, and the stamp demands the report have moved since — that is what
// makes "this stage's findings reached the report" a fact rather than a claim.

// Every pipeline stage, in pipeline order. One list, read by the vocabulary check below, by the usage
// line it prints, and by `stamp`'s required-set check — so a stage added to the pipeline cannot be
// accepted by one of the three and missed by the others.
const stageNames = "code-review security-review tighten refactor retro"

func stageList() []string { return strings.Fields(stageNames) }

func (r *run) assertValidStage(stage, subcommand string) {
	for _, known := range stageList() {
		if stage == known {
			return
		}
	}
	r.refuse("usage: report.sh " + subcommand + " <" + strings.Join(stageList(), "|") + ">")
}

// The report as cksum(1) saw it: "<crc> <bytes>". Empty means the report could not be read, and every
// caller treats that as "not checksummed" rather than as a value.
func (r *run) reportChecksum() string {
	content, err := os.ReadFile(r.report)
	if err != nil {
		return ""
	}
	crc, size := cksum(content)
	return fmt.Sprintf("%d %d", crc, size)
}

// A marker never written while the caller prints "recorded" is a stage the stamp waves through.
func (r *run) writeStageMarker(stage, value string) {
	if value == "" {
		r.errLines("error: the report could not be checksummed — " + stage + " is NOT marked")
		r.exit(2)
	}
	if err := os.MkdirAll(r.stageReturnsDir, 0o777); err != nil {
		r.errLines("error: could not write " + r.stageReturnsDir + "/" + stage + " — " + stage + " is NOT marked")
		r.exit(2)
	}
	if err := os.WriteFile(r.stageReturnsDir+"/"+stage, []byte(value+"\n"), 0o666); err != nil {
		r.errLines("error: could not write " + r.stageReturnsDir + "/" + stage + " — " + stage + " is NOT marked")
		r.exit(2)
	}
}

// The stage marked returned against the report as it now stands — items still unwritten. Only one may
// stand at a time (see `stage-returned`); a `no-items` marker holds a word, never a checksum.
func (r *run) outstandingStage() string {
	current := r.reportChecksum()
	if current == "" {
		return ""
	}
	entries, err := os.ReadDir(r.stageReturnsDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !shell.IsRegularFile(r.stageReturnsDir+"/"+name) {
			continue
		}
		if r.markerValue(name) == current {
			return name
		}
	}
	return ""
}

func (r *run) markerValue(stage string) string {
	content, err := os.ReadFile(r.stageReturnsDir + "/" + stage)
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(content), "\n")
}

func (r *run) stageWasMarkedReturned(stage string) bool {
	return shell.IsRegularFile(r.stageReturnsDir + "/" + stage)
}

// Why this stage cannot be stamped as having run, or empty when it can.
func (r *run) stageBlockReason(stage string) string {
	if !r.stageWasMarkedReturned(stage) {
		return "ran but was never marked returned (report.sh stage-returned " + stage + ")"
	}
	recorded := r.markerValue(stage)
	if recorded == noItemsMarker {
		return ""
	}
	if recorded == r.reportChecksum() {
		return "returned but the report is unchanged since — record its items, or report.sh no-items " + stage
	}
	return ""
}

// The stage record's grammar. Only `fast` reaches fastTrims, so any other word for a turnaround trim
// would stamp a trimmed pass as full — which is why the vocabulary is closed rather than pattern-matched.
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
	// The required-set check, in the order the shell version's awk walked its `split()` array: an
	// artifact of that awk's hash, kept so the two versions' refusals are byte-identical. It carries
	// no meaning — pipeline order would read better the day report.sh is gone.
	stages := stageList()
	for _, i := range []int{1, 2, 3, 4, 0} {
		switch stage := stages[i]; {
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
	case "refactor", "refactor:partial(fast)", "refactor:partial(cap)":
		// partial = the loop ended non-compliant, which is a record of what ran, not a trim.
		return "refactor", true
	}
	for _, stage := range []string{"security-review", "tighten", "retro"} {
		if entry == stage || entry == stage+":skipped(fast)" || entry == stage+":skipped(not-applicable)" {
			return stage, true
		}
	}
	return "", false
}

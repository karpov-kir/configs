// What a `go-mutate` exit status does to the count of consecutive runs that measured nothing, and
// whether that count is past the point where "the runner was loaded" is still the likelier
// explanation.
//
//	usage: nomeasure <harness-exit-status> <count-file>
//	       0  it measured — the count is cleared, and the caller exits on the harness's own status
//	       1  it did not measure, and the count is still under the threshold — warn and pass
//	       3  it did not measure <threshold> runs running — fail, and say why
//	       2  nothing was decided and nothing was written: a bad argument, or a count file that would
//	          not take the write
//
// A gate that warns and passes on every did-not-measure is an honest chain ending in a green tick
// forever, over guards nothing has ever proved. The count is what turns that chain into something a
// run can act on.
//
// Exit 2 is the harness saying a loaded machine killed mutants on its watchdog, not that a guard
// failed to redden. Every other status clears the count: 0 and 1 are it reporting on the guards
// themselves, and a failing guard proves it reached them, which is the only thing this counts. A
// status the harness never defines, a 127 from a harness that is not there, clears the count too. It
// cannot buy a silent pass that way, because the caller exits on that status and the job goes red.
//
// Nothing is written but the count file, and its directory is not created: the caller names the path,
// and a tool that makes up the parents is writing somewhere nobody named.
package nomeasure

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const escalateAt = 3

const harnessDidNotMeasure = 2

const (
	exitCleared        = 0
	exitUnderThreshold = 1
	exitDidNotDecide   = 2
	exitEscalated      = 3
)

const harness = "go-mutate"

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 || args[0] == "" || args[1] == "" {
		fmt.Fprintf(stderr, "nomeasure: needs the harness's exit status and a count file — nothing was decided.\n")
		fmt.Fprintf(stderr, "usage: nomeasure <harness-exit-status> <count-file>\n")
		return exitDidNotDecide
	}
	rawStatus, countFile := args[0], args[1]

	if !allDigits(rawStatus) {
		fmt.Fprintf(stderr, "nomeasure: '%s' is no exit status — nothing was decided, and the count was left as it was.\n", rawStatus)
		return exitDidNotDecide
	}

	if rawStatus != strconv.Itoa(harnessDidNotMeasure) {
		if !wasRecorded(countFile, 0, stderr) {
			return exitDidNotDecide
		}
		fmt.Fprintf(stdout, "%s reached the guards on this run (exit %s), so the did-not-measure count is back to 0.\n", harness, rawStatus)
		return exitCleared
	}

	count := storedCount(countFile) + 1
	if !wasRecorded(countFile, count, stderr) {
		return exitDidNotDecide
	}

	if count >= escalateAt {
		fmt.Fprintf(stdout, "%s has not measured on %d consecutive runs. That is no longer machine load — nothing is proving these guards, and this stops passing until one run measures.\n", harness, count)
		return exitEscalated
	}
	fmt.Fprintf(stdout, "%s did not measure every guard on this runner (%d consecutive). Nothing was proved for those, and this is not a pass for them.\n", harness, count)
	return exitUnderThreshold
}

// The history the count file holds, or zero where it holds anything this tool will not read as one.
// A count that does not parse starts the history over rather than carrying on from the garbage. Ten
// digits or more is refused by LENGTH, not by arithmetic: a stored 1234567890 parses cleanly, sits
// above the threshold, and would fail the job for good over a number no run ever wrote.
func storedCount(countFile string) int {
	body, err := os.ReadFile(countFile)
	if err != nil {
		return 0
	}
	stored := string(body)
	if !allDigits(stored) || len(stored) >= 10 {
		return 0
	}
	n, err := strconv.Atoi(stored)
	if err != nil {
		return 0
	}
	return n
}

func wasRecorded(countFile string, count int, stderr io.Writer) bool {
	if err := os.WriteFile(countFile, []byte(strconv.Itoa(count)), 0o644); err != nil {
		fmt.Fprintf(stderr, "nomeasure: could not write %s — the count is unchanged, so the next run reads stale history and this one proved nothing.\n", countFile)
		return false
	}
	return true
}

// Non-empty and every byte an ASCII digit. Not strconv.Atoi, which accepts a leading sign and would
// read "-1" as a status and "+2" as a count.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	return strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) < 0
}

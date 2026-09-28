// Runs the comment lane's stages over a change set, one command per stage, so a run writes no script
// of its own. Runs 11 to 13 rebuilt each stage by hand from the skill's prose, and each rebuild
// carried a bug once. It calls no model: the runner still dispatches the writers.
//
//	usage: comment-run.sh seed --run-dir=<dir> --archive=<dir> --range=<base>..<head> [--heads=<sha,...>] [--contradictions=<tsv>]
//	usage: comment-run.sh prompts --run-dir=<dir> --workers=<n>
//	usage: comment-run.sh archive-written --run=<run> --archive=<dir> <writer return>...
//	usage: comment-run.sh taint --ledger=<file> <transcript>... [--ledger=<file> <transcript>...]
//	usage: comment-run.sh keep-test --archive=<dir> <path>...
//	usage: comment-run.sh loop --run-dir=<dir> --archive=<dir> --run=<run> [--contradict=<sentence>] <path>:<line> <review sentence>...
//
// Exit 0 is a clean run, 1 a run that reported findings, and 2 a stage that did not run.
package commentrun

import (
	"fmt"
	"io"
	"strings"

	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
)

// The grammar. It carries the stub's name where argv[0] would carry the binary's.
const usage = "usage: comment-run.sh seed --run-dir=<dir> --archive=<dir> --range=<base>..<head> [--heads=<sha,...>] [--contradictions=<tsv>]\n" +
	"       comment-run.sh prompts --run-dir=<dir> --workers=<n>\n" +
	"       comment-run.sh archive-written --run=<run> --archive=<dir> <writer return>...\n" +
	"       comment-run.sh taint --ledger=<file> <transcript>... [--ledger=<file> <transcript>...]\n" +
	"       comment-run.sh keep-test --archive=<dir> <path>...\n" +
	"       comment-run.sh loop --run-dir=<dir> --archive=<dir> --run=<run> [--contradict=<sentence>] <path>:<line> <review sentence>..."

const (
	exitClean     = 0
	exitFindings  = 1
	exitDidNotRun = 2
)

// stage is one command, given the options and the operands after its name.
type stage func(r *runner, opts options, operands []string) int

var stages = map[string]stage{
	"seed":            seed,
	"prompts":         prompts,
	"archive-written": archiveWritten,
	"taint":           taint,
	"keep-test":       keepTest,
	"loop":            loop,
}

// runner holds the directory a stage stands in and the streams it writes to.
type runner struct {
	self   string
	cwd    string
	git    repo.Git
	stdout io.Writer
	stderr io.Writer
}

// refuse says why the stage did not run and states the grammar, and exits 2.
func (r *runner) refuse(format string, a ...any) int {
	fmt.Fprintf(r.stderr, "%s: %s — the stage did NOT run\n", r.self, fmt.Sprintf(format, a...))
	fmt.Fprintf(r.stderr, "%s\n", usage)
	return exitDidNotRun
}

// Run runs the stage its first argument names.
func Run(self string, args []string, cwd string, git repo.Git, stdout, stderr io.Writer) int {
	r := &runner{self: self, cwd: cwd, git: git, stdout: stdout, stderr: stderr}
	if len(args) == 0 {
		return r.refuse("%s", "name a stage: seed, prompts, archive-written, taint, keep-test or loop")
	}
	run, known := stages[args[0]]
	if !known {
		return r.refuse("no stage %q", shell.Echoable(args[0]))
	}
	opts, operands := parse(args[1:])
	return run(r, opts, operands)
}

// options are a stage's `--name=value` arguments. A name given twice keeps every value in order, and
// order keeps the operands after each: taint reads a ledger for the transcripts after it.
type options struct {
	values map[string][]string
	// order is every option and operand as given, for a stage that pairs them.
	order []string
}

func (o options) one(name string) string {
	if v := o.values[name]; len(v) > 0 {
		return v[len(v)-1]
	}
	return ""
}

func parse(args []string) (options, []string) {
	o := options{values: map[string][]string{}}
	var operands []string
	for _, arg := range args {
		o.order = append(o.order, arg)
		if name, value, found := strings.Cut(strings.TrimPrefix(arg, "--"), "="); strings.HasPrefix(arg, "--") && found {
			o.values[name] = append(o.values[name], value)
			continue
		}
		operands = append(operands, arg)
	}
	return o, operands
}

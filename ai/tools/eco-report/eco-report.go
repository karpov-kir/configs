// Package ecoreport is the qualify report tool — the deterministic gates the skills must not execute
// by hand. The mechanism lives here; the contract it serves (repo modes, what goes in the report,
// never commit it) is `~/.claude/skills/idsd-qualify/SKILL.md` → **Report**. idsd-ship calls it too
// (gate/state/promote/discard). One report per intent, at
// .idsd/qualify-reports/<intent>-qualify-report.md, so two ships never share a file.
//
// It is a library with a thin command beside it, for the reason ecocheck is: the suite that proves it
// drives it once per case, and a process spawn per case is the cost that makes a mutation run take
// hours. Nothing here writes to os.Stdout or calls os.Exit — every path reports through the writers
// the Invocation carries and returns the code the command exits on — and nothing here holds state
// between calls, so two runs in one process cannot see each other's caches.
//
// Two seams stay external, and must: `~/.kk-flavor/scripts/tree-fingerprint.sh` owns the
// tree-fingerprint recipe (get it half right here, with a throwaway index but no throwaway object
// store, and every untracked file's content lands in the human's own .git/objects for good), and
// `todo-gate.sh` owns the open-item scan. Both are invoked, never reimplemented.
//
// This tool deletes files (discard) and writes to the git index (promote). Every refusal below is
// load-bearing: read the comment before removing one.
//
// Subcommands:
//
//	init "<intent>" [--force]  scaffold .idsd/ + the report from the template, stamping its intent
//	                 line. Refuses over an existing report unless --force, which first prints the open
//	                 `- [ ]` it is about to discard. Refuses a symlink either way
//	root             print the resolved scratch directory — the in-tree .idsd/ in committed mode, and
//	                 outside the working tree in throwaway mode. The only way a skill learns it;
//	                 joining `.idsd/` onto the repo root is what made the location per-worktree
//	repo-mode        print committed|throwaway — is .idsd/ tracked in git?
//	invalidate       clear reviewed-tree/reviewed-stages and drop the stage markers at pass start, so no
//	                 stamp outlives its tree; stamp refuses until this pass has run it
//	stage-returned <stage>  mark a stage returned, recording the report as it then stood; stamp refuses until
//	                 the report has changed since, so a stage's items cannot be left unrecorded. One stage at
//	                 a time — refused while another stage's mark still has nothing recorded against it
//	no-items <stage> mark a stage already marked returned as having surfaced nothing, the one way to clear
//	                 its marker without editing the report
//	stamp "<stages>" compute the tree fingerprint (throwaway index) and record reviewed-tree +
//	                 reviewed-stages, one entry per pipeline stage. Any `(fast)` marks the pass
//	                 not-full. Run `stamp` bare for the grammar; that usage text is the authority on it
//	gate             done-blocker: stale tree OR turnaround-trimmed stages (both human-overridable)
//	                 OR any open `- [ ]` (never overridable) → non-zero + reasons
//	carry            print prior open `- [ ]` (with their section) so re-qualify loses none
//	check-ignore     keep qualify-reports/ out of the fingerprint, by the mechanism that fits the repo mode
//	promote          throwaway → committed: stop excluding .idsd/, ignore qualify-reports/ via .gitignore, stage
//	discard          throwaway only: remove this ship's local scratch (report, intent file, stage
//	                 markers), and the whole .idsd/ + its local exclusion when nothing else remains.
//	                 Another intent, or an authored charter/constitution/language/playbook, is
//	                 "something" — those are the human's, not this ship's scratch
//	state            print the `continue` routing token:
//	                 no-report|resume|re-qualify|decide|finalize|ready|done
//	list             one line per open ship, `<intent><TAB><state>`, for routing with several in flight
//	close [--force]  retire one landed ship's report and stage markers. Refuses while an open `- [ ]`
//	                 stands, since nothing else keeps a copy
//
// Every subcommand that reads a report takes the intent as its LAST argument, optional while only one
// report is open. Several open and none named is refused, never guessed: resolving to the wrong report
// stamps one intent's review onto another's, and the stamp is what the merge gate trusts.
package ecoreport

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"kk-flavor/tools/shell"
)

// Invocation is one run of the tool. The three fields the shell version read from its process — the
// working directory git is asked about, argv[0] the skill dir is derived from, and $HOME the
// fingerprint script hangs off — are explicit here so a test can drive a fixture without touching
// anything process-global. Empty means "take it from this process", which is what the command does.
type Invocation struct {
	Args []string
	Dir  string
	Self string
	Home string
	// Where this machine's overrides live — `$XDG_CONFIG_HOME`. Explicit for the same reason Home is:
	// the override it resolves decides where every write lands, so a suite must be able to point it at
	// a fixture instead of the developer's own.
	ConfigHome string
	Out, Err   io.Writer
}

// Run is the entry point the command uses: the process's own directory, argv[0] and HOME.
func Run(args []string, out, errOut io.Writer) int {
	return Invocation{Args: args, Out: out, Err: errOut}.Exec()
}

// Exec runs one invocation and returns the process exit code. 0 is a result, 1 is a gate's block
// (`gate` and `check-ignore` alone), and 2 is "this did not run" — never a result.
func (inv Invocation) Exec() (code int) {
	r := newRun(inv)
	// exit 2 = "this did not run", never a result. Every path that stops halfway leaves by here.
	// The shell version's `refuse` was an `exit`, reachable from any depth, and the header of that
	// file says three times over what happens when one is swallowed by a `$( )` subshell: the caller
	// runs on with an empty substitution and the error already printed. A panic carries the same
	// reach with none of that hazard — nothing in Go recovers it but this function.
	defer func() {
		switch signal := recover().(type) {
		case nil:
		case stop:
			code = signal.code
		default:
			panic(signal)
		}
	}()
	r.resolveRoot()
	r.dispatch()
	return 0
}

type stop struct{ code int }

type run struct {
	args                  []string
	dir, home, configHome string
	out, errOut           io.Writer
	skillDir              string
	template              string
	todoGate              string
	fingerprintBin        string

	root string
	// The scratch directory this invocation acts on, resolved once by resolveIdsdDir. In committed mode
	// it is inside the tree; in throwaway mode it never is. Read it rather than rebuilding `.idsd` from
	// the root — that is what made the location per-worktree.
	idsdDir    string
	reportsDir string
	// Set when a machine-local override moved the scratch root, and printed by every command it affects.
	overrideNote string

	// Set by setReportPaths once the intent is known. Empty until then, so a subcommand that reads a
	// report resolves it first, through requireReport or resolveReport.
	report          string
	stageReturnsDir string

	ambiguousNames string
	// Files in qualify-reports/ whose name is not a slug, counted by the last reportNames call so a
	// caller can say it listed fewer reports than the directory holds.
	unnameableReports int
	// One fingerprint per invocation. `list` scores every report against the same working tree, so
	// the walk currentTree does is the same walk each time.
	cachedTree string
	openTodos  string
}

const reportSuffix = "-qualify-report.md"

const noItemsMarker = "no-items"

func newRun(inv Invocation) *run {
	r := &run{
		args:       inv.Args,
		dir:        inv.Dir,
		home:       inv.Home,
		configHome: inv.ConfigHome,
		out:        inv.Out,
		errOut:     inv.Err,
	}
	if r.dir == "" {
		// Failing here would be a directory that no longer exists, which git is about to refuse for
		// its own reasons anyway; leaving it empty means "the process's own", as exec.Cmd reads it.
		r.dir, _ = os.Getwd()
	}
	if inv.Home == "" {
		r.home = os.Getenv("HOME")
	}
	if inv.ConfigHome == "" {
		r.configHome = os.Getenv("XDG_CONFIG_HOME")
	}
	self := inv.Self
	if self == "" {
		self = os.Args[0]
	}
	scripts := r.absPath(shell.DirName(self))
	r.skillDir = filepath.Clean(scripts + "/..")
	r.template = r.skillDir + "/templates/qualify-report-template.md"
	r.todoGate = scripts + "/todo-gate.sh"
	// The one script that fingerprints a tree. Never recompute the recipe here: get it half right,
	// with a throwaway index but no throwaway object store, and every untracked file's content lands
	// in the human's own .git/objects for good, referenced by no ref and so collected by nothing.
	r.fingerprintBin = r.home + "/.kk-flavor/scripts/tree-fingerprint.sh"
	return r
}

// The invocation's own directory is what a relative argv[0] resolves against, never the process's:
// a caller that set Dir chose which repo this run acts on, and picking up os.Getwd() here would
// resolve the skill dir against a different one.
func (r *run) absPath(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(r.dir + "/" + path)
}

func (r *run) resolveRoot() {
	root, status := r.capture(nil, "git", "rev-parse", "--show-toplevel")
	if status != 0 {
		r.refuse("error: not a git repo")
	}
	r.root = root
	r.resolveIdsdDir()
	r.reportsDir = r.idsdDir + "/qualify-reports"
}

func (r *run) dispatch() {
	// Before the subcommand, so the location is named above whatever it prints — including a refusal,
	// which is where knowing the directory matters most.
	r.noteOverride()
	switch r.arg(0) {
	case "root":
		r.line("%s", r.idsdDir)
	case "init":
		r.cmdInit(r.args[1:])
	case "repo-mode":
		r.line("%s", r.repoMode())
	case "stage-returned":
		r.cmdStageReturned()
	case "no-items":
		r.cmdNoItems()
	case "stamp":
		r.cmdStamp()
	case "gate":
		r.cmdGate()
	case "carry":
		r.cmdCarry()
	case "invalidate":
		r.cmdInvalidate()
	case "check-ignore":
		r.cmdCheckIgnore()
	case "promote":
		r.cmdPromote()
	case "discard":
		r.cmdDiscard()
	case "state":
		r.cmdState()
	case "list":
		r.cmdList()
	case "close":
		r.cmdClose(r.args[1:])
	default:
		r.refuse("usage: report.sh {init <intent>|root|repo-mode|invalidate|stage-returned <stage>|no-items <stage>|stamp \"<stages>\"|gate|carry|check-ignore|promote|discard|close|state|list} [<intent>]",
			"  every subcommand that reads a report takes the intent as its last argument; omit it when only one is open")
	}
}

// The nth argument or the empty string — `${n:-}`, which is how every optional intent name arrives.
func (r *run) arg(n int) string {
	if n < len(r.args) {
		return r.args[n]
	}
	return ""
}

func (r *run) refuse(lines ...string) {
	r.errLines(lines...)
	panic(stop{2})
}

// A code the caller is meant to read as a result: `gate`'s block and `check-ignore`'s warning.
func (r *run) exit(code int) {
	panic(stop{code})
}

func (r *run) line(format string, args ...any) {
	fmt.Fprintf(r.out, format+"\n", args...)
}

func (r *run) errLines(lines ...string) {
	errLinesTo(r.errOut, lines...)
}

// `printf '%s\n' "$@" >&2`: one line per argument, an empty one included — a refusal builds its last
// line conditionally and prints it either way.
func errLinesTo(errOut io.Writer, lines ...string) {
	for _, line := range lines {
		fmt.Fprintln(errOut, line)
	}
}

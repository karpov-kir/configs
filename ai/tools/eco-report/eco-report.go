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
// Two seams stay out of this package, and must: `ai/tools/tree-fingerprint/` owns the
// tree-fingerprint recipe, imported and run in process, and `todo-gate.sh` owns the open-item scan,
// which is spawned. Neither is reimplemented here — newRun says what recomputing the first one costs.
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
//	invalidate       clear reviewed-tree/reviewed-worktree/reviewed-stages and drop the stage markers at
//	                 pass start, so no stamp outlives its tree; stamp refuses until this pass has run it
//	stage-returned <stage>  mark a stage returned, recording the report as it then stood; stamp refuses until
//	                 the report has changed since, so a stage's items cannot be left unrecorded. One stage at
//	                 a time — refused while another stage's mark still has nothing recorded against it
//	no-items <stage> mark a stage already marked returned as having surfaced nothing, the one way to clear
//	                 its marker without editing the report
//	decisions-reviewed  record that this pass re-evaluated the decision log — bumping what it reached and
//	                 found still true, evicting what its subject has left. stamp refuses until it has run,
//	                 and invalidate clears it, so every pass accounts for the log afresh
//	stamp "<stages>" compute the tree fingerprint (throwaway index) and record reviewed-tree +
//	                 reviewed-worktree + reviewed-stages, one entry per pipeline stage. Any `(turnaround)`
//	                 marks the pass not-full. Refuses when this worktree's identity cannot be
//	                 established, since gate reads it to tell this tree's review from a sibling's. Run
//	                 `stamp` bare for the grammar; that usage text is the authority on it
//	gate             done-blocker: stale tree OR turnaround-trimmed stages OR a ship whose intent never
//	                 reached `status: approved` (all three human-overridable) OR any open `- [ ]` in the
//	                 report or in the ship's intent file (never overridable) → non-zero + reasons
//	intent-ready <NNN-slug>  build-blocker over the ICE itself: unfilled template placeholders, an
//	                 empty required section, an unsigned collaborative intent, or a depends-on edge
//	                 that has not shipped → non-zero + reasons. Judgement is the grill's, not this
//	carry            print prior open `- [ ]` (with their section) so re-qualify loses none
//	check-ignore     keep qualify-reports/ out of the fingerprint, by the mechanism that fits the repo mode
//	promote          throwaway → committed: ignore qualify-reports/ via .gitignore, MOVE the scratch
//	                 directory into the tree as .idsd/, stage it. Every refusal after the move puts the
//	                 directory back where it came from
//	discard          throwaway only: remove this ship's scratch (report, intent file, stage markers),
//	                 and the whole scratch directory when nothing else remains. Another intent, or an
//	                 authored charter/constraints/language/playbook, is "something" — those are the
//	                 human's, not this ship's scratch
//	state            print the `continue` routing token:
//	                 no-report|resume|re-qualify|decide|finalize|ready|done
//	list             one line per open ship, `<intent><TAB><state>`, for routing with several in flight
//	close [--force]  retire one landed ship's report and stage markers. Refuses while an open `- [ ]`
//	                 stands, since nothing else keeps a copy
//	record <append|bump|revise|evict|admit> <decisions|playbook|language|constraints> "<text>" ["<new text>"]
//	                 the one way to write the four shared records, which every worktree of the clone
//	                 shares. Serialised under flock, so two agents writing at once both land; a hand
//	                 edit is what silently drops one of them. records.go says why it locks as it does.
//	                 Each record is capped: an append into a full one refuses, and `admit` is the only
//	                 way in, dropping the entry the new one beat
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
	treefingerprint "kk-flavor/tools/tree-fingerprint"
)

// Invocation is one run of the tool. The three fields it would otherwise read from its own process —
// the working directory git is asked about, argv[0] the skill dir is derived from, and $HOME the
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
	// How the working tree is fingerprinted. Nil is the shipped recipe, called IN PROCESS rather than
	// spawned as `tree-fingerprint.sh`.
	//
	// A seam rather than a hardcoded call because two properties still need observing from outside: how
	// MANY times a run fingerprints (one, cached), and what a failed fingerprint does to the commands
	// that read one.
	Fingerprint func(root string) (string, error)
}

// Run is the entry point the command uses: the process's own directory, argv[0] and HOME.
func Run(args []string, out, errOut io.Writer) int {
	return Invocation{Args: args, Out: out, Err: errOut}.Exec()
}

// Exec runs one invocation and returns the process exit code. 0 is a result, 1 is a gate's block
// (`gate`, `intent-ready` and `check-ignore` alone), and 2 is "this did not run" — never a result.
func (inv Invocation) Exec() (code int) {
	r := newRun(inv)
	// exit 2 = "this did not run", never a result. Every path that stops halfway leaves by here. A panic
	// is what gives `refuse` reach from any depth, and nothing in Go recovers it but this function.
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
	// The repository answers already asked, per invocation. git.go → memoGit owns it.
	gitMemo map[string]gitAnswer

	// The fingerprint recipe, in process. Never nil once Exec has built the run.
	fingerprint func(root string) (string, error)

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
	// Appended to requireReport's no-report refusal. `gate` is the one caller that needs it: its reader
	// is standing at a merge, where "run init first" reads as the step that clears the gate.
	noReportNote []string
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
		args:        inv.Args,
		dir:         inv.Dir,
		home:        inv.Home,
		configHome:  inv.ConfigHome,
		out:         inv.Out,
		errOut:      inv.Err,
		fingerprint: inv.Fingerprint,
	}
	if r.fingerprint == nil {
		r.fingerprint = treefingerprint.Fingerprint
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
	// Idempotent: several subcommands resolve the root on the way to their own work, and the answer is
	// fixed the moment the invocation starts. Without the guard each of them spawns another
	// `rev-parse --show-toplevel`.
	if r.root != "" {
		return
	}
	root, ok := layoutRoot(r.dir)
	if !ok {
		// The layout could not answer — an environment override, or a shape it does not know. git can.
		var status int
		root, status = r.capture(nil, "git", "rev-parse", "--show-toplevel")
		if status != 0 {
			r.refuse("error: not a git repo")
		}
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
	case "decisions-reviewed":
		r.cmdDecisionsReviewed()
	case "stamp":
		r.cmdStamp()
	case "gate":
		r.cmdGate()
	case "intent-ready":
		r.cmdIntentReady()
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
	case "record":
		r.cmdRecord(r.args[1:])
	default:
		r.refuse("usage: report.sh {init <intent>|root|repo-mode|invalidate|stage-returned <stage>|no-items <stage>|decisions-reviewed|stamp \"<stages>\"|gate|intent-ready <NNN-slug>|carry|check-ignore|promote|discard|close|state|list|record <op> <record> \"<text>\"} [<intent>]",
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

// A code the caller is meant to read as a result: `gate`'s and `intent-ready`'s block, and
// `check-ignore`'s warning.
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

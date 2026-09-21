// A caller builds a Run and declares its mounts with AddConfig, named one by one when the guard
// reports, and AddBulk, a count for a homogeneous set like ai/'s skills. It adds an unmount scan for
// any directory it mounts a discovered set into, then calls Mount. Report is the end of every path
// through a run, the guard that stops before the first write included.
//
// Report prints the account and answers the exit code. A refusal that ended the run its own way
// would report through something the caller has no say over.

// The package reads no environment variable. $HOME arrives as the targets a caller builds, and the
// registry's directory as ConfigHome. A suite can then point a whole run at a throwaway home instead
// of faking a filesystem. WriteRoot holds it there, and every write resolves its parent physically
// and refuses to land outside that root.

// A harness bug once handed every case the same home, followed a live symlink into this checkout and
// overwrote real config files in the working tree. The suite's own report of it was read as a
// harness bug, and the question of what the run had already written went unasked.

// The run refuses to delete anything it did not write itself. It drops its own mounts once this
// checkout has no source for them. It stops before writing anything when this machine's config is
// already mounted from a different checkout.

// Package installer is the mounting machinery every installer in this repository runs on. It links a
// source in this checkout at a target in $HOME or in a project. Beside the mounts it writes the
// marked region an installer owns inside a file someone else authored, and the record of which
// projects this machine was installed into.
package installer

import (
	"fmt"
	"io"
	"strings"

	"configs/ai/tools/shell"
)

// RunOptions is everything a run needs that it must not go looking for itself.
type RunOptions struct {
	// The value is resolved physically on the way in, for the reason realDir exists. /var is a symlink
	// to /private/var on macOS. A guard comparing a resolved root against an unresolved Repo calls this
	// checkout a stranger to itself, and it refuses a machine that is mounted correctly.

	// Repo is the directory the calling installer lives in — env/ or ai/. Every source path is built
	// from it, and mountForeignRoot recognises a stranger by finding a file named ScriptName at the
	// same relative depth under a different root.
	Repo string
	// ScriptName is the calling installer's own filename, which the guard looks for under a candidate
	// root. The caller takes it from the running program, so a rename cannot leave the guard hunting
	// for a name that has gone.
	ScriptName string
	// Label is what the report line calls the run — "env bootstrap", "ai bootstrap". Two installers
	// print here, and a human reading a terminal has to know which answered.
	Label string
	// BulkLabel is what the report calls a homogeneous mount set — "skills". A run declaring bulk
	// mounts needs it. Any other run leaves it unprinted.
	BulkLabel string

	// Machine-wide, a repoint moves a human's whole configuration. For a project install it moves that
	// project's skills alone. A refusal saying "this machine's configuration" about one project is
	// false in a way that teaches people to ignore it.

	// MountScopeLabel is what the guard's refusal calls the thing at stake. Empty takes the
	// machine-wide wording.
	MountScopeLabel string
	// ConfigHome is where the install registry lives — the caller's already-resolved
	// ${XDG_CONFIG_HOME:-$HOME/.config}. The caller passes it as data, so no case can reach the
	// developer's own registry by forgetting to set something.
	ConfigHome string
	DryRun     bool
	Relocate   bool
	// Out is where the account goes, and nil discards it. No caller wants that, and every case that
	// only reads Refusals can live with it.
	Out io.Writer
	// WriteRoot bounds every write this run makes to one directory tree, resolved physically so a
	// symlink in the path cannot route one out of it. Empty is unbounded, which is what an installer
	// on a real machine runs as — the bound is for a suite driving the real linking logic against a
	// throwaway home.
	WriteRoot string
}

// Run is one installer's pass over this machine: the mounts it declares, the writes it makes, and the
// account it gives of both.
type Run struct {
	repo            string
	scriptName      string
	label           string
	bulkLabel       string
	mountScopeLabel string
	configHome      string
	dryRun          bool
	relocate        bool
	out             io.Writer
	tree            *tree

	refusals []string
	breaches []string

	configs []Mount
	bulk    []Mount
	scans   []unmountScan

	// Which foreign root each declared mount resolves to. Mount's survey fills these, and the refusal
	// that reports one root's worth of them reads them. Parallel to configs and bulk.
	configForeign []string
	bulkForeign   []string
	foreignRoots  []string
}

// Mount is one declared link: a source in this checkout, and where it is mounted.
type Mount struct {
	Source string
	Target string
}

const machineScope = "this machine's configuration"

// Half of what these messages quote is text the tree chose, a skill directory name off a branch or a
// symlink value read off the machine. A name long enough to fill a terminal buries the run's own
// account of what it removed, as surely as a control byte rewrites it.

// maxMessageBytes bounds one printed line.
const maxMessageBytes = 1024

func NewRun(options RunOptions) *Run {
	repo := options.Repo
	if resolved := realDir(repo); resolved != "" {
		repo = resolved
	}
	scope := options.MountScopeLabel
	if scope == "" {
		scope = machineScope
	}
	out := options.Out
	if out == nil {
		out = io.Discard
	}
	run := &Run{
		repo:            repo,
		scriptName:      options.ScriptName,
		label:           options.Label,
		bulkLabel:       options.BulkLabel,
		mountScopeLabel: scope,
		configHome:      options.ConfigHome,
		dryRun:          options.DryRun,
		relocate:        options.Relocate,
		out:             out,
	}
	run.tree = newTree(options.WriteRoot, run.noteBreach)
	return run
}

// A refusal says a human has something to fix. A breach says this run went for a file it had no
// business touching. The two must arrive as separate facts, so a breach goes on a list of its own
// that a suite reads.

// Records a write the bound turned away.
func (r *Run) noteBreach(message string) {
	r.breaches = append(r.breaches, message)
}

// Every message loses its control bytes on the way out. A control byte in a name the tree chose
// drives the terminal instead of printing. `ESC[2K` erases the line it lands in, and `ESC[1A` moves
// the cursor up one line. A name carrying either can wipe the run's own record of what it removed or
// refused.

// Say prints one line of the account.
func (r *Run) Say(message string) {
	fmt.Fprintln(r.out, clean(message))
}

// Refuse records what a human has to fix and prints it at the step it happened in. The reader then
// sees it beside that step as well as in the summary.
func (r *Run) Refuse(message string) {
	message = clean(message)
	r.refusals = append(r.refusals, message)
	fmt.Fprintf(r.out, "  REFUSED  %s\n", message)
}

func (r *Run) Refusals() []string {
	return r.refusals
}

// Breaches is every write this run tried to make outside WriteRoot. It is empty on a real machine,
// which sets no bound. A suite fails its whole run on a non-empty one, so the case that noticed does
// not read as merely red.
func (r *Run) Breaches() []string {
	return r.breaches
}

// Report is the end of every path through a run. It answers the exit code instead of taking one,
// because the collected list and that code are the whole contract with a caller.
func (r *Run) Report() int {
	fmt.Fprintln(r.out)
	if len(r.refusals) == 0 {
		r.Say(r.label + ": ok")
		return 0
	}
	fmt.Fprintf(r.out, "%s: %d thing(s) need you:\n", r.label, len(r.refusals))
	for _, item := range r.refusals {
		fmt.Fprintf(r.out, "  - %s\n", item)
	}
	return 1
}

// Every write in this package goes through apply, which is the single place the dry run branches. A
// second `if dryRun` beside a write is how a flag that must write no file starts writing on one path.

// change is one write and what the run says about it. That is the line a dry run prints, the line a
// real run prints, and the refusal when the write could not land.
type change struct {
	// would is the dry run's line, did the real run's, each already indented by apply.
	would string
	did   string

	// A sentence, because one write has more than one way to fail and each sends the reader somewhere
	// different. A failure to create the parent sends them somewhere other than a failure to link.

	// write does the work and answers the refusal's whole sentence when it could not, empty when it
	// did.
	write func() string
}

func (r *Run) apply(c change) bool {
	if r.dryRun {
		r.Say("  " + c.would)
		return true
	}
	if failure := c.write(); failure != "" {
		r.Refuse(failure)
		return false
	}
	r.Say("  " + c.did)
	return true
}

func clean(message string) string {
	return shell.CutBytesMarked(shell.Oneline(message), maxMessageBytes)
}

// ai/README.md's skills loop linked with `ln -sfn "$d"` for as long as the skills existed, and `$d`
// came from a `*/` glob. Every machine set up from it holds links that read back as `…/kk-tighten/`
// where this package computes `…/kk-tighten`.

// The two name the same directory with different strings. Compared raw, each one reports as stale, a
// correct link is rewritten on every run, and idempotence goes to a cosmetic difference.

// ai/README.md now documents `${d%/}`, so new machines will not have them. This stays for machines
// already set up, because the readback is a property of links made years ago.

// Returns where a symlink points, with a trailing slash stripped.
func linkValue(path string) string {
	value, err := readLink(path)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(value, "/")
}

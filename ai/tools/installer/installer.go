// Package installer is the mounting machinery every installer in this repository runs on: link a
// source in this checkout at a target in $HOME or in a project, refuse rather than delete anything it
// did not write itself, drop its own mounts once this checkout has no source for them, and stop
// before writing anything when this machine's config is already mounted from a different checkout.
// Beside the mounts it writes the marked region an installer owns inside a file someone else
// authored, and the record of which projects this machine was installed into.
//
// A caller builds a Run, declares its mounts with AddConfig — named one by one when the guard reports
// — and AddBulk — counted, for a homogeneous set like ai/'s skills — adds an unmount scan for any
// directory it mounts a discovered set into, then calls Mount. Report is the end of every path
// through a run, the guard that stops before the first write included: it prints the account and
// answers the exit code, because a refusal that ended the run its own way would report through
// something the caller has no say over.
//
// Nothing here reads the environment. $HOME arrives as the targets a caller builds and the registry's
// directory as ConfigHome, which is what lets a suite point a whole run at a throwaway home rather
// than fake a filesystem. WriteRoot then holds it there: every write resolves its parent physically
// and refuses to land outside that root. A harness bug once handed every case the same home, followed
// a live symlink into this checkout and overwrote real config files in the working tree — and the
// suite's own report of it was read as a harness bug without anyone asking what the run had already
// written.
package installer

import (
	"fmt"
	"io"
	"strings"

	"kk-flavor/tools/shell"
)

// RunOptions is everything a run needs that it must not go looking for itself.
type RunOptions struct {
	// Repo is the directory the calling installer lives in — env/ or ai/. Every source path is built
	// from it, and the second-checkout guard recognises a stranger by finding a file named ScriptName
	// at the same relative depth under a different root.
	//
	// Resolved physically on the way in, for the reason realDir exists: /var is a symlink to
	// /private/var on macOS, so a guard comparing a resolved root against an unresolved Repo calls this
	// checkout a stranger to itself and refuses a machine that is mounted correctly.
	Repo string
	// ScriptName is the calling installer's own filename, which the guard looks for under a candidate
	// root. Taken from the running program rather than written down, so a rename cannot leave the guard
	// hunting for a name nothing has.
	ScriptName string
	// Label is what the report line calls the run — "env bootstrap", "ai bootstrap". Two installers
	// print here, and a human reading a terminal has to know which answered.
	Label string
	// BulkLabel is what the report calls a homogeneous mount set — "skills". A run declaring bulk
	// mounts needs it; one that does not never prints it.
	BulkLabel string
	// MountScopeLabel is what the guard's refusal calls the thing at stake. Machine-wide, repointing
	// these moves a human's whole configuration; for a project install it moves that project's skills
	// and nothing else, and a refusal saying "this machine's configuration" about one project is false
	// in a way that teaches people to ignore it. Empty takes the machine-wide wording.
	MountScopeLabel string
	// ConfigHome is where the install registry lives — the caller's already-resolved
	// ${XDG_CONFIG_HOME:-$HOME/.config}. Data rather than an environment read, so no case can reach the
	// developer's own registry by forgetting to set something.
	ConfigHome string
	DryRun     bool
	Relocate   bool
	// Out is where the account goes; nil discards it, which no caller wants and every case that only
	// reads Refusals can live with.
	Out io.Writer
	// WriteRoot bounds every write this run makes to one directory tree, resolved physically so a
	// symlink in the path cannot route one out of it. Empty is unbounded, which is what an installer
	// on a real machine runs as — the bound is for a suite driving the real linking logic against a
	// throwaway home. See the package comment for what its absence cost once.
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

	// Refusals are collected rather than fatal: a machine missing one cask should still get every
	// link, and a human fixing three named problems in one pass beats discovering them one run at a
	// time.
	refusals []string
	breaches []string

	configs []Mount
	bulk    []Mount
	scans   []unmountScan

	// Which foreign root each declared mount resolves to, filled by Mount's survey and read by the
	// refusal that reports one root's worth of them. Parallel to configs and bulk.
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

// maxMessageBytes bounds one printed line. Half of what these messages quote is text the tree chose
// rather than text this package wrote — a skill directory name off a branch, a symlink value read off
// the machine — and a name long enough to fill a terminal buries the run's own account of what it
// removed as surely as a control byte rewrites it. Every path this repository writes is well under
// the bound, so reaching it says something about the name rather than about the paths growing.
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

// A write the bound turned away, recorded where a suite reads it rather than only refused. A refusal
// says a human has something to fix; a breach says this run went for a file it had no business
// touching, and the two must not arrive as the same fact.
func (r *Run) noteBreach(message string) {
	r.breaches = append(r.breaches, message)
}

// Say prints one line of the account. Every message loses its control bytes on the way out: a control
// byte in a name the tree chose drives the terminal instead of printing, and `ESC[2K` erases the line
// it lands in while `ESC[1A` moves to the line above — so a name carrying either can wipe the run's
// own record of what it removed or refused.
//
// shell.Oneline rather than a substitution of this package's own, and it reaches one byte the shell
// could not: a raw 0x9b is the CSI an 8-bit terminal acts on, and catching it needs the decoding
// `[[:cntrl:]]` cannot do.
func (r *Run) Say(message string) {
	fmt.Fprintln(r.out, clean(message))
}

// Refuse records what a human has to fix and prints it where it happened, so the reader sees it beside
// the step it belongs to as well as in the summary.
func (r *Run) Refuse(message string) {
	message = clean(message)
	r.refusals = append(r.refusals, message)
	fmt.Fprintf(r.out, "  REFUSED  %s\n", message)
}

func (r *Run) Refusals() []string {
	return r.refusals
}

// Breaches is every write this run tried to make outside WriteRoot. Empty on a real machine, where
// there is no bound; a suite fails its whole run on a non-empty one rather than reading the case that
// noticed as merely red. The distinction is the incident in the package comment: a case failed saying
// something had been written where nothing should be, and the report was read as a harness bug
// without asking what the broken run had already written to disk.
func (r *Run) Breaches() []string {
	return r.breaches
}

// Report is the end of every path through a run. It answers the exit code rather than taking it,
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

// change is one write and what the run says about it: the line a dry run prints, the line a real run
// prints, and the refusal when the write could not land. Every write in this package goes through
// apply, which is the single place the dry run branches — a second `if dryRun` beside a write is how a
// flag that must write nothing starts writing on one path.
type change struct {
	// would is the dry run's line, did the real run's, each already indented by apply.
	would string
	did   string
	// write does the work and answers the refusal's whole sentence when it could not, empty when it
	// did. A sentence rather than an error, because one write has more than one way to fail and each
	// sends the reader somewhere different — a parent that could not be created is not a link that
	// could not be made.
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

// Where a symlink points, with a trailing slash stripped.
//
// ai/README.md's skills loop linked with `ln -sfn "$d"` for as long as the skills existed, where `$d`
// came from a `*/` glob, so every machine set up from it holds links that read back as
// `…/kk-tighten/` while this package computes `…/kk-tighten`. Same directory, different string:
// compared raw, each one reports as stale and a correct link is rewritten on every run, which is
// idempotence lost to a cosmetic difference.
//
// ai/README.md now documents `${d%/}`, so new machines will not have them. This stays for the ones
// already set up: the readback is a property of links made years ago, not of the current README.
func linkValue(path string) string {
	value, err := readLink(path)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(value, "/")
}

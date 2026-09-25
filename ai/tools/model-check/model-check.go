// Package modelcheck resolves every model a policy holds for one client against the CLI that would
// run it, and says which model answered each. An account can serve another model than the row asks
// for, and only the call's own report says so. On 2026-09-25 an organisation's team account answered
// `sonnet` and `haiku` as claude-opus-5-5[1m], and every row naming either had run on Opus unnoticed.
// A substitution is reported and never fails the run. The rows keep the model they intend.
//
// A name in models.json is an assertion until something asks a provider about it. Nothing did: the
// policy validates the shape of a name and nothing else, so the first proof that an account can run it
// arrives when a judge roll is refused mid-gate — as exit 2, the same status an unknown kind, an
// unreadable path and a blown deadline all produce.
//
// # Whether a cheaper question exists
//
// Asked before this was built, because a probe that spends a model call per name per gate run is not
// one a gate can hold. Measured 2026-09-15: neither CLI offers a supported way to ask. `claude` has no
// catalogue subcommand and no dry run, and refuses `--max-budget-usd 0` outright. `codex` has no
// models subcommand. `~/.codex/models_cache.json` does hold the account's list, but it is another
// tool's private cache, outside the repository and particular to one machine's login — reading it
// would be a gate keyed on a file nobody here maintains.
//
// So the question costs a call, and the gate's own keying is the cache the alternative would have had
// to build by hand: one CLI invocation per distinct name, carrying a one-line prompt that asks for a
// single character back, and paid only when models.json or the packages the check is built from move.
// A name the provider refuses buys no completion: claude answers from its own catalogue without
// reaching the API, and codex reaches it and comes back 400. A name it accepts costs the smallest
// call that CLI can make, six of them for the whole file today.
//
// The probe goes through reader-judge's own caller, so a name that passes here is proven against the
// argv the judge will actually use, and the refusal this reports is the one the judge reports.
package modelcheck

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"time"

	modelpolicy "configs/ai/tools/model-policy"
	readerjudge "configs/ai/tools/reader-judge"
	"configs/ai/tools/shell"
)

// Long enough that a slow API does not read as a bad name, short enough that a handful cannot hold a
// gate for an hour. A judge roll gets 420s for reading a document; this sends one character.
const probeDeadline = 120 * time.Second

// The whole of what a probe asks: text the model can answer, since the point is to reach the API at
// all, and text nobody could mistake for a judgement — no view of any file reaches a provider here.
const (
	probePrompt = "Reply with exactly one character: ."
	probeView   = "."
)

// maxSelections bounds what one run may spend: the probe costs a CLI call per distinct selection, the
// file is whatever the gated branch put there, the gate puts no timeout on a unit, and `validate` caps
// neither the rows nor the tier order. 24 is far above any real file — this repo's holds six
// selections — and far below a bill worth noticing.
const maxSelections = 24

// maxReportedSelectionBytes bounds the name part of one reported line, the way every tool here bounds
// a name it echoes. 120, against the 80 in reader-judge's echoable. A line here names a selection's
// source, a client, a model and an effort. That one names a single argument.
const maxReportedSelectionBytes = 120

// Probe asks one provider whether it will run one selection — the whole selection rather than the
// three strings inside it, since client and model are both strings and a transposed pair would compile
// and quietly ask a different question. The seam lets the suite drive every verdict without a bill.
type Probe func(selection modelpolicy.Selection) (readerjudge.Served, error)

type Command struct {
	Args       []string
	Invocation string
	Stdout     io.Writer
	Stderr     io.Writer
	// Nil means the real CLIs.
	Probe Probe
	// Accounts names the login a call inheriting this process's variables runs on, and the login a roll
	// runs on. Nil means the real `claude auth status`.
	Accounts func() (inherited, own string)
}

func Run(command Command) int {
	flags := flag.NewFlagSet("model-check", flag.ContinueOnError)
	flags.SetOutput(command.Stderr)
	config := flags.String("config", "", "policy JSON file; defaults to models.json in the installed flavor's configs directory")
	agent := flags.String("agent", "", "the client whose selections to probe: claude or codex")
	flags.Usage = func() {
		fmt.Fprintln(command.Stderr, usage+"\nAsks one client for every model selection models.json holds for it, and says which model answered each. It judges no text.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(command.Args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		return refuse(command.Stderr, "unexpected positional arguments")
	}
	if *agent != "claude" && *agent != "codex" {
		return refuse(command.Stderr, "--agent=claude or --agent=codex names the client to probe\n"+usage)
	}
	configPath := *config
	if configPath == "" {
		installed, err := modelpolicy.InstalledPath(command.Invocation)
		if err != nil {
			return refuse(command.Stderr, err)
		}
		configPath = installed
	}
	policy, err := modelpolicy.Load(configPath)
	if err != nil {
		return refuse(command.Stderr, err)
	}
	probe := command.Probe
	if probe == nil {
		probe = liveProbe
	}
	var named []modelpolicy.Selection
	for _, selection := range policy.Selections() {
		if selection.Client == *agent {
			named = append(named, selection)
		}
	}
	if len(named) > maxSelections {
		return refuse(command.Stderr, fmt.Sprintf(
			"%s holds %d distinct model selections, past the %d this may probe — each one costs a call to a provider, so a file this size is a mistake to refuse rather than a bill to pay",
			configPath, len(named), maxSelections))
	}
	status := report(command.Stdout, command.Stderr, configPath, resolveAll(named, probe))
	if *agent == "claude" {
		accounts := command.Accounts
		if accounts == nil {
			accounts = readerjudge.ClaudeAccounts
		}
		reportAccounts(command.Stdout, accounts)
	}
	return status
}

const usage = "usage: model-check.sh --agent=claude|codex [--config <policy.json>]"

// reportAccounts names the login the calls ran on. Inside the desktop app a call inheriting its
// variables uses the app's sign-in, and a roll uses the CLI's own login. A person with several
// accounts can switch the app and leave the CLI on another. A usage limit then reads as the app's.
func reportAccounts(stdout io.Writer, accounts func() (string, string)) {
	inherited, own := accounts()
	switch {
	case own == "":
		fmt.Fprintln(stdout, "model-check: account — the CLI names no login; `claude auth login` signs it in")
	case inherited == own || inherited == "":
		fmt.Fprintf(stdout, "model-check: account — %s\n", own)
	default:
		fmt.Fprintf(stdout, "model-check: account — WARNING: the app runs on %s and the CLI on %s; "+
			"`claude auth login` switches the CLI\n", inherited, own)
	}
}

// Three and not two: a name nothing could ask about is neither good nor bad, and calling it either is
// the stale green this unit exists to stop. Named rather than carried as two possibly-empty strings,
// which also spell a fourth state meaning nothing.
type outcome int

const (
	willRun outcome = iota
	substituted
	refusedTheName
	nobodyCouldAsk
)

type verdict struct {
	named   modelpolicy.Selection
	outcome outcome
	// What the provider or this machine said. Empty for willRun, which has nothing to explain.
	reason string
	served readerjudge.Served
}

func resolveAll(selections []modelpolicy.Selection, probe Probe) []verdict {
	var verdicts []verdict
	for _, named := range selections {
		v := verdict{named: named}
		served, err := probe(named)
		v.served = served
		switch {
		case err == nil && served.Substituted():
			v.outcome = substituted
		case err == nil:
			v.outcome = willRun
		case isRefusal(err):
			v.outcome, v.reason = refusedTheName, err.Error()
		default:
			v.outcome, v.reason = nobodyCouldAsk, err.Error()
		}
		verdicts = append(verdicts, v)
	}
	return verdicts
}

func isRefusal(err error) bool {
	var refused *readerjudge.ModelRefused
	return errors.As(err, &refused)
}

// A refused name is exit 1, the repo's "ran, and has findings". Nothing resolved at all is exit 2,
// "did not run": on a machine carrying neither CLI this measured no name in the file.
//
// A file that is part resolved passes on what it reached and names what it did not, so the scope is
// narrow: this machine's PATH decides it, and the gate keys the unit on files, so installing a missing
// CLI does not re-run a verdict already cached. Read a pass as "no provider reachable from this
// machine refuses a name in this file", never as "every name runs".
func report(stdout, stderr io.Writer, config string, verdicts []verdict) int {
	refused, resolved, swapped := 0, 0, 0
	for _, v := range verdicts {
		where := reportedName(v.named)
		switch v.outcome {
		case substituted:
			resolved++
			swapped++
			fmt.Fprintf(stdout, "model-check: %s — SUBSTITUTED: %s answered; this account does not serve %s, "+
				"likely an organisation's model restriction\n", where,
				shell.CutBytesMarked(shell.Oneline(v.served.Answered), maxReportedSelectionBytes), v.named.Model)
		case refusedTheName:
			refused++
			fmt.Fprintf(stdout, "model-check: %s — REFUSED: %s\n", where, v.reason)
		case nobodyCouldAsk:
			fmt.Fprintf(stderr, "model-check: %s — not resolved: %s\n", where, v.reason)
		case willRun:
			resolved++
			if v.served.Answered == "" {
				fmt.Fprintf(stdout, "model-check: %s — the provider will run it; it reports no served model\n", where)
			} else {
				fmt.Fprintf(stdout, "model-check: %s — served as requested\n", where)
			}
		}
	}
	if swapped > 0 {
		fmt.Fprintf(stdout, "model-check: WARNING: optimal model use is not possible on this account: %d selection(s) "+
			"run on another model than the row asks for. The rows keep the model they intend, and take effect "+
			"where an account serves it.\n", swapped)
	}
	if refused > 0 {
		fmt.Fprintf(stderr, "model-check: %s names %d model(s) the provider will not run — fix the name in that file rather than reading the next judge failure as a broken judge\n", config, refused)
		return 1
	}
	// No guard for an empty run, because a parsed policy cannot be one: validate demands both maps be
	// non-empty and every row name a model for both clients, so there is always something to ask.
	if resolved == 0 {
		return refuse(stderr, fmt.Sprintf("no provider could be reached, so no name in %s was resolved — this is unchecked, not clean", config))
	}
	return 0
}

// Shaped the way every other policy-derived string these tools echo is shaped. Nothing in a parsed
// policy can carry a newline today — validName bars whitespace and control characters — so this forges
// no line now. It stops a message an orchestrator reads from resting on a validator two packages away
// that nothing here tests.
func reportedName(named modelpolicy.Selection) string {
	where := named.Origin + " " + named.Client + " " + named.Model
	if named.Effort != "" {
		where += " at " + named.Effort
	}
	return shell.CutBytesMarked(shell.Oneline(where), maxReportedSelectionBytes)
}

// The real question: run the CLI the judge would run, with the model and effort the selection holds and
// nothing to read. A CLI that is not installed comes back unresolved rather than refused — the name
// may be perfectly good and this machine simply cannot ask.
func liveProbe(selection modelpolicy.Selection) (readerjudge.Served, error) {
	var served readerjudge.Served
	if _, err := exec.LookPath(selection.Client); err != nil {
		return served, fmt.Errorf("%s is not on PATH here, so nothing on this machine can ask about its models", selection.Client)
	}
	settings := modelpolicy.Settings{Model: selection.Model, Effort: selection.Effort}
	call := readerjudge.ClaudeCallerObserved(probeDeadline, settings, func(s readerjudge.Served) { served = s })
	if selection.Client == "codex" {
		call = readerjudge.CodexCaller(probeDeadline, settings)
	}
	_, err := call(probePrompt, probeView)
	return served, err
}

// Every decline leaves the same two marks the other tools leave: the tool's name ahead of the reason
// on stderr, and status 2, which the shell stubs read as "did not run".
func refuse(stderr io.Writer, reason ...any) int {
	fmt.Fprintln(stderr, append([]any{"model-check:"}, reason...)...)
	return 2
}

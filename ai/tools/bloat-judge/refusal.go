package bloatjudge

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"configs/ai/tools/shell"
)

// ModelRefused is the provider declining the name it was handed, rather than failing to answer with
// it. Its own type because the three ways a roll ends without a verdict want three different repairs:
// a deadline is a slow machine or API, an unparsed reply is the model ignoring the format, and this
// one is a string in models.json this login cannot run — nothing a retry or another text changes.
type ModelRefused struct {
	Client string
	Model  string
}

func (r *ModelRefused) Error() string {
	return fmt.Sprintf("%s refused the model %s", r.Client, echoable(r.Model))
}

// What each CLI was measured saying when it will not run the model it was given. 2026-09-15, on this
// repo's machine, both against `definitely-not-a-model-xyz`:
//
//	claude exits 1 with "There's an issue with the selected model (…). It may not exist or you may
//	not have access to it." on stdout and "[claude-code:unrecognized_model] {…}" on stderr.
//
//	codex exits 1 with `ERROR: {"type":"error","status":400,"error":{"type":"invalid_request_error",
//	"message":"The '…' model is not supported when using Codex with a ChatGPT account."}}` on stderr.
//
// Only the unknown-name case was measured. A name the CLI knows but the account cannot reach is the
// other half of what this catches: claude's line is worded for both ("may not exist or you may not
// have access to it") and codex's names the account outright, but neither was driven.
//
// A marker that stops matching costs the message and nothing else: the roll still fails, at the
// wording this replaced. So the list is short and carries the stable half of each sentence.
var modelRefusalMarkers = map[string][]string{
	"claude": {"unrecognized_model", "issue with the selected model"},
	"codex":  {"model is not supported when using Codex"},
}

// Both streams, because the table above splits them and a version of either could move its line.
// `Output` returns stderr only on the ExitError, which is why it is pulled back out of the error.
//
// The judged text is subtracted first, which is why this takes the input at all: `codex exec` echoes
// the message it was handed to stderr, `Output` keeps a failed roll's stdout, and the markers above
// sit verbatim in this file and in provider_test.go. Judging those and hitting an unrelated failure
// would otherwise report a refused model and send someone to edit a correct config.
//
// Line by line rather than by substring, so an echoed line the CLI indented is still subtracted — both
// sides are trimmed — and a marker on the CLI's own line survives however the input was framed. It
// keys on the trimmed line entire, so a CLI that prefixes or reflows one defeats it. Keep it narrow
// anyway: subtracting too much only loses a refusal to the roll's own error, where subtracting too
// little is the false refusal above.
func refusedTheModel(client, echoed string, stdout []byte, err error) bool {
	markers, known := modelRefusalMarkers[client]
	if !known {
		return false
	}
	said := string(stdout)
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		said += "\n" + string(exit.Stderr)
	}
	fromTheInput := map[string]bool{}
	for _, line := range shell.SplitLines(echoed) {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			fromTheInput[trimmed] = true
		}
	}
	for _, line := range shell.SplitLines(said) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || fromTheInput[trimmed] {
			continue
		}
		for _, marker := range markers {
			if strings.Contains(trimmed, marker) {
				return true
			}
		}
	}
	return false
}

// RollTimedOut is the bound in deadline.go firing. Its own type so the wrapper that names files can
// reach it: the repair is a `roll-timeout` line, and an exit 2 that does not name that file sends the
// reader hunting a fault in the text they were judging. The wording is asserted in deadline_test.go,
// being how a caller tells a roll that was cut off from one whose model crashed.
type RollTimedOut struct {
	Deadline time.Duration
}

func (t *RollTimedOut) Error() string {
	return fmt.Sprintf("the model did not answer within %s", t.Deadline)
}

// ProviderExhausted is the account out of capacity for now: the name is good, the text is good, and
// the answer is a wait rather than an edit. Its own type beside ModelRefused because the repairs
// differ — that one sends someone to models.json, this one sends them to the clock.
type ProviderExhausted struct {
	Client string
}

func (e *ProviderExhausted) Error() string {
	return fmt.Sprintf("%s has no capacity left on this login, so nothing was judged", e.Client)
}

// What a CLI was measured saying when the login is out of capacity. Measured 2026-09-16 on this
// repo's machine: `claude -p` prints "You've hit your session limit · resets 3:50pm (Europe/Moscow)"
// on stdout and exits 0 — the apology takes the answer's place, at the status a good answer uses.
//
// The second claude marker and the codex ones are the same sentence in the wordings those CLIs use
// elsewhere for the same condition, and are NOT measured. A marker that never matches costs nothing
// here: the roll then ends the way it ended before this existed.
var exhaustionMarkers = map[string][]string{
	"claude": {"hit your session limit", "usage limit reached"},
	"codex":  {"usage limit reached", "rate limit", "quota"},
}

// exhausted reads a SUCCESSFUL call's output for one of those. The judged text is subtracted first,
// for the reason refusedTheModel subtracts it: a document discussing rate limits would otherwise
// report the account exhausted and send someone to wait out a limit they never hit.
func exhausted(client, echoed string, stdout []byte) bool {
	markers, known := exhaustionMarkers[client]
	if !known {
		return false
	}
	fromTheInput := map[string]bool{}
	for _, line := range shell.SplitLines(echoed) {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			fromTheInput[trimmed] = true
		}
	}
	for _, line := range shell.SplitLines(string(stdout)) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || fromTheInput[trimmed] {
			continue
		}
		for _, marker := range markers {
			if strings.Contains(strings.ToLower(trimmed), marker) {
				return true
			}
		}
	}
	return false
}

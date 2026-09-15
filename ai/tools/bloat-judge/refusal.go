package bloatjudge

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"kk-flavor/tools/shell"
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
// Line by line rather than by substring, so an echoed line the CLI indented or prefixed is still
// subtracted, and a marker on the CLI's own line survives however the input was framed. A CLI that
// reflows a line mid-sentence defeats it; what it removes is the echo both were measured producing.
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

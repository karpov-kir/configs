package writereval

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// The pipeline's writer runs the record check on each block, rewrites for every finding, and answers
// `none` after two rewrites. The eval's writer had no tools, so no check ever refused it. Run 10's
// check refused one block twice and sent its fact to the human. The harness now runs the same loop.

// writerCall is one call to the writer row.
type writerCall func(text string) (string, error)

// recordCheck runs the check over what the writer would pipe, and returns its finding lines.
type recordCheck func(input string, record bool) ([]string, error)

// checkRewrites is how many times the check sends a block back before the writer answers none.
const checkRewrites = 2

// checkInput is what the pipeline's writer pipes: the record, a `---` line, the block, then the
// declaration it sits on and the body under it.
func checkInput(r Return, code string) (string, bool) {
	lines := strings.Split(code, "\n")
	at := r.At
	if at < 1 || at > len(lines) {
		at = 1
	}
	var out []string
	record := r.Note == PartWritten
	if record {
		out = append(out, r.Record...)
		out = append(out, "---")
	}
	out = append(out, r.CommentLines()...)
	out = append(out, lines[at-1:]...)
	return strings.Join(out, "\n") + "\n", record
}

// rewriteAsk is the turn that hands a block back with the check's findings, as the pipeline's writer
// reads them off its own run.
func rewriteAsk(asked, answered string, findings []string, attempt int) string {
	last := ""
	if attempt == checkRewrites {
		last = " This is your second rewrite. A part still carrying a finding after it is none, with " +
			"both attempts shown, and its facts go back as `does not fit`."
	}
	return fmt.Sprintf("%s\n\n=== your answer ===\n%s\n\n=== the check ===\nYou ran the record check over "+
		"that block, and it printed:\n\n%s\n\nRewrite the block for every finding, as the brief's check "+
		"section says. This is attempt %d.%s Answer in the same shape as before.",
		asked, answered, strings.Join(findings, "\n"), attempt, last)
}

// writeChecked runs one roll the way the pipeline runs a site: the writer's answer, the check, and up
// to two rewrites. A block still refused after the second is the pipeline's `none` with the facts
// sent to the human, and the roll is scored as that.
func writeChecked(call writerCall, check recordCheck, asked, code string) (Return, string, error) {
	raw, err := call(asked)
	if err != nil {
		return Return{}, "", err
	}
	r := ParseReturn(raw)
	for attempt := 1; ; attempt++ {
		if ClassOf(r) != ExpectWritten {
			return r, raw, nil
		}
		input, record := checkInput(r, code)
		findings, err := check(input, record)
		if err != nil {
			return Return{}, raw, err
		}
		if len(findings) == 0 {
			return r, raw, nil
		}
		if attempt > checkRewrites {
			refused := Return{None: true, Summary: r.Summary, Note: PartNone, Attempts: checkRewrites,
				Rounds: checkRewrites, Routed: append(append([]string(nil), r.Routed...), "does not fit")}
			if r.Summary == PartWritten {
				refused.Summary = PartNone
			}
			return refused, raw + "\n[the check still refused it: " + strings.Join(findings, "; ") + "]", nil
		}
		raw, err = call(rewriteAsk(asked, raw, findings, attempt))
		if err != nil {
			return Return{}, raw, err
		}
		r = ParseReturn(raw)
		r.Rounds = attempt
	}
}

// voiceCheckScript is the edit lane's check, run from this checkout.
const voiceCheckScript = "../../kk-flavor/skills/kk-edit/scripts/voice-check.sh"

var findingLine = regexp.MustCompile(`^-:\d+: `)

// runRecordCheck runs the checkout's voice check over the piped block.
func runRecordCheck(input string, record bool) ([]string, error) {
	args := []string{"--profile=comment", "--source"}
	if record {
		args = append(args, "--record")
	}
	command := exec.Command(voiceCheckScript, append(args, "-")...)
	command.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
		return nil, fmt.Errorf("the record check did not run: %v: %s", err, stderr.String())
	}
	var findings []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		if findingLine.MatchString(line) {
			findings = append(findings, line)
		}
	}
	return findings, nil
}

// A block the check refuses goes back with the findings, and one it passes is scored as written.
func TestTheHarnessRewritesABlockTheCheckRefuses(t *testing.T) {
	answers := []string{
		"summary: none\nnote: written\nat: 1\nfact: a ledger drops a book\nbears_on: keepsBook\ndoes: returns false\n// A ledger drops a book.",
		"summary: none\nnote: written\nat: 1\nfact: a ledger drops a book\nbears_on: keepsBook\ndoes: returns false\n// A ledger drops a book, and `keepsBook` returns false for it.",
	}
	var asked []string
	call := func(text string) (string, error) {
		asked = append(asked, text)
		return answers[len(asked)-1], nil
	}
	check := func(input string, record bool) ([]string, error) {
		if !record || !strings.Contains(input, "bears_on: keepsBook\ndoes: returns false\n---\n// A ledger") {
			t.Fatalf("the check was piped no record above the block:\n%s", input)
		}
		if strings.Contains(input, "`keepsBook`") {
			return nil, nil
		}
		return []string{"-:4: block-omits-bears-on: the block never says keepsBook"}, nil
	}
	r, _, err := writeChecked(call, check, "the site", "keepsBook: book => false,")
	if err != nil {
		t.Fatal(err)
	}
	if ClassOf(r) != ExpectWritten || r.Rounds != 1 || len(asked) != 2 {
		t.Fatalf("class %s after %d round(s) and %d call(s), want written after 1 and 2", ClassOf(r), r.Rounds, len(asked))
	}
	if !strings.Contains(asked[1], "block-omits-bears-on") {
		t.Fatalf("the rewrite turn carried no finding:\n%s", asked[1])
	}
}

// Two rewrites the check still refuses end the site as the pipeline ends it: none, with the facts
// sent to the human.
func TestABlockRefusedAfterTwoRewritesIsNone(t *testing.T) {
	calls := 0
	call := func(string) (string, error) {
		calls++
		return "summary: none\nnote: written\nat: 1\nfact: f\nbears_on: g\ndoes: h\n// A fact.", nil
	}
	check := func(string, bool) ([]string, error) { return []string{"-:4: does-untied: h"}, nil }
	r, _, err := writeChecked(call, check, "the site", "function g() {}")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 || ClassOf(r) != ExpectNone || len(r.Routed) != 1 || r.Routed[0] != "does not fit" {
		t.Fatalf("%d call(s), class %s, routed %v; want 3, none, does not fit", calls, ClassOf(r), r.Routed)
	}
}

// A summary on its own has no record to pipe, and the check reads only its prose.
func TestASummaryAloneIsCheckedWithoutARecord(t *testing.T) {
	call := func(string) (string, error) {
		return "summary: needed\nnote: none\nat: 1\n// Lists the postings of a closed book.", nil
	}
	check := func(input string, record bool) ([]string, error) {
		if record || strings.Contains(input, "---") {
			t.Fatalf("a summary alone was piped as a record:\n%s", input)
		}
		return nil, nil
	}
	if _, _, err := writeChecked(call, check, "the site", "export function list() {}"); err != nil {
		t.Fatal(err)
	}
}

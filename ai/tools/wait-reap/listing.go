// The process listing the reaper reads, and what in it counts as a waiter. This file holds what a row
// says; waitreap.go holds what to do about it.

package waitreap

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// process is one row of the listing: everything a waiter is judged by.
type process struct {
	pid     int
	age     time.Duration
	command string
}

// parseListing turns `ps -Ao pid=,etime=,args=` output into its rows. A row it cannot read refuses
// the whole listing, because a truncated listing reports a smaller pile of waiters and reads exactly
// like a clean machine.
func parseListing(text string) ([]process, error) {
	var rows []process
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		row, err := parseRow(trimmed)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func parseRow(line string) (process, error) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return process{}, fmt.Errorf("cannot read this row of the process listing, so the listing is not trustworthy: %q", line)
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil || pid <= 0 {
		// Not only unreadable input: a negative number reaching `kill` signals a whole process group.
		return process{}, fmt.Errorf("the first column of %q is no pid", line)
	}
	age, err := parseElapsed(fields[1])
	if err != nil {
		return process{}, fmt.Errorf("%s in %q", err, line)
	}
	command := strings.TrimSpace(line)
	command = strings.TrimSpace(strings.TrimPrefix(command, fields[0]))
	command = strings.TrimSpace(strings.TrimPrefix(command, fields[1]))
	return process{pid: pid, age: age, command: command}, nil
}

// parseElapsed reads the three shapes `ps` prints elapsed time in: MM:SS, HH:MM:SS and D-HH:MM:SS.
func parseElapsed(elapsed string) (time.Duration, error) {
	days := 0
	rest := elapsed
	if dash := strings.Index(rest, "-"); dash >= 0 {
		parsed, err := strconv.Atoi(rest[:dash])
		if err != nil {
			return 0, fmt.Errorf("%q is no elapsed time", elapsed)
		}
		days = parsed
		rest = rest[dash+1:]
	}
	parts := strings.Split(rest, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, fmt.Errorf("%q is no elapsed time", elapsed)
	}
	total := time.Duration(days) * 24 * time.Hour
	units := []time.Duration{time.Hour, time.Minute, time.Second}
	units = units[len(units)-len(parts):]
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil {
			return 0, fmt.Errorf("%q is no elapsed time", elapsed)
		}
		total += time.Duration(value) * units[i]
	}
	return total, nil
}

// Where one statement ends and the next begins. `ps` writes an embedded newline as the four
// characters `\012`, so a command written across lines is reachable only through that spelling as
// well as through a real newline.
const statementBreakPattern = `;|\n|\\012`

var statementBreak = regexp.MustCompile(statementBreakPattern)

// A loop's body opens after a separator, so prose containing the word "do" never reads as one. This
// pattern reuses statementBreakPattern, so a spelling added there reaches both.
var loopOpener = regexp.MustCompile(`(?:` + statementBreakPattern + `)\s*do\b`)

// What a waiting body may hold: a sleep, a no-op (how a loop written to spin is spelled), and an
// arithmetic assignment, which is how a bounded retry counts its own attempts. Anything else runs a
// command, and a loop running a command is doing work.
var waitingStatement = regexp.MustCompile(`^(?:sleep\s+[0-9.]+|:|[A-Za-z_][A-Za-z0-9_]*=\$\(\([^()]*\)\))$`)

// `read` consumes its input, so `while read -r line; do :; done` is a pipeline stage shaped like a
// loop. It ends when its input does.
var consumesInput = regexp.MustCompile(`(^|[\s;&|(])read($|[\s;&|)])`)

// The scratch directory the harness gives a session, which is where a waiter's own command names the
// session that started it.
var sessionScratch = regexp.MustCompile(`/tmp/claude-\d+/(-[^/\s]+)/([0-9a-fA-F-]{36})(?:/|\b)`)

// Where the shell wrapper hands over to the command the harness was actually given.
const evalMarker = "eval '"

var loopKeyword = regexp.MustCompile(`\b(?:until|while)\s`)

// `n=0` before a loop sets a counter and runs no command, so it is not work a kill would lose.
var assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=[^;&|$` + "`" + `]*$`)

// Redirections only: a trailing `2>/dev/null` is not work, a trailing `; echo done` is.
var redirectionsOnly = regexp.MustCompile(`^(?:\s*\d?(?:>>|>|<)\s*[^\s;&|]+)*\s*;?\s*$`)

// Every command the harness runs sources a shell snapshot of its own, which is what separates a
// waiter it started from a loop the human is running in their own terminal.
const harnessSnapshot = "/.claude/shell-snapshots/"

// isHarnessOwned reports whether the harness started this command.
func isHarnessOwned(command string) bool {
	return strings.Contains(command, harnessSnapshot)
}

// isWaitLoop reports whether a command holds a loop that polls without making progress. The report is
// built from this and is wider than what may be ended, because a loop left out of the report is one a
// human never learns to collect.
func isWaitLoop(command string) bool {
	return slices.ContainsFunc(loopBodies(command), bodyWaits)
}

// onlyWaits reports whether the whole command is one loop that only waits. That is what may be ended,
// because the process then holds no work to lose before the loop, inside it, or after it. It reads
// the command the harness was given, inside the wrapper, so the shell's own prelude and its trailing
// `pwd` do not count as work.
func onlyWaits(command string) bool {
	payload := harnessPayload(command)
	if payload == "" {
		return false
	}
	bodies := loopBodies(payload)
	if len(bodies) != 1 || !bodyOnlyWaits(bodies[0]) {
		return false
	}
	head, tail := aroundTheLoop(payload)
	return isAssignmentsOnly(head) && redirectionsOnly.MatchString(tail)
}

// harnessPayload returns the command the harness was asked to run, out of the shell wrapper it runs
// everything through.
func harnessPayload(command string) string {
	opener := strings.Index(command, evalMarker)
	if opener < 0 {
		return ""
	}
	payload := command[opener+len(evalMarker):]
	closer := strings.LastIndex(payload, "'")
	if closer < 0 {
		return ""
	}
	return payload[:closer]
}

// aroundTheLoop returns what sits before the loop's keyword and after its `done`.
func aroundTheLoop(payload string) (head string, tail string) {
	start := loopKeyword.FindStringIndex(payload)
	end := strings.LastIndex(payload, "done")
	if start == nil || end < 0 {
		return payload, ""
	}
	return payload[:start[0]], payload[end+len("done"):]
}

func isAssignmentsOnly(head string) bool {
	for _, statement := range statements(head) {
		if !assignment.MatchString(statement) {
			return false
		}
	}
	return true
}

// loopBodies returns the text between each loop's `do` and its `done`.
func loopBodies(command string) []string {
	var bodies []string
	rest := command
	for {
		opener := loopOpener.FindStringIndex(rest)
		if opener == nil {
			return bodies
		}
		head, body := rest[:opener[0]], rest[opener[1]:]
		closer := strings.Index(body, "done")
		if closer < 0 {
			return bodies
		}
		if loopKeyword.MatchString(head) && !consumesInput.MatchString(head) {
			bodies = append(bodies, body[:closer])
		}
		rest = body[closer+len("done"):]
	}
}

// isWork reports whether a statement runs a command instead of waiting. A loop running a command is
// making progress, and ending it would lose that progress.
func isWork(statement string) bool {
	return !waitingStatement.MatchString(statement)
}

// bodyWaits reports whether a loop body waits at all — sleeps, spins, or counts its own attempts. A
// body that does any of those is polling something, whatever else it does between passes.
func bodyWaits(body string) bool {
	return slices.ContainsFunc(statements(body), waitingStatement.MatchString)
}

// bodyOnlyWaits reports whether a loop body only waits, and does that at least once.
func bodyOnlyWaits(body string) bool {
	found := statements(body)
	return len(found) > 0 && !slices.ContainsFunc(found, isWork)
}

func statements(body string) []string {
	var found []string
	for _, statement := range statementBreak.Split(body, -1) {
		if trimmed := strings.TrimSpace(statement); trimmed != "" {
			found = append(found, trimmed)
		}
	}
	return found
}

// waitedOn returns the condition the first loop polls, which is the only part of a long command line
// a reader needs to recognise what the process is stuck on. It finds that loop the same way the kill
// path does, so the report names the loop the ruling was made about.
func waitedOn(command string) string {
	keyword := loopKeyword.FindStringIndex(command)
	if keyword == nil {
		return ""
	}
	condition := command[keyword[1]:]
	if opener := loopOpener.FindStringIndex(condition); opener != nil {
		condition = condition[:opener[0]]
	}
	return strings.TrimSpace(condition)
}

// sessionOf returns the project directory and session the command's own paths name, which is how a
// waiter is attributed to the session that started it.
func sessionOf(command string) (project string, session string, found bool) {
	matches := sessionScratch.FindAllStringSubmatch(command, -1)
	if len(matches) == 0 {
		return "", "", false
	}
	// A command naming two sessions names one that is not its own, and the text does not say which. The
	// wrong session's transcript would end the wrong waiter, so an ambiguous command is attributed to
	// no session and falls to the age rule.
	for _, other := range matches[1:] {
		if other[2] != matches[0][2] {
			return "", "", false
		}
	}
	return matches[0][1], matches[0][2], true
}

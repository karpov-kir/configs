package commentrun

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"configs/ai/tools/shell"
)

// taint reads each writer's transcript for a history read that showed it the blocks it was sent to
// replace. A hit reads a file the ledger lists, before the last write to it, and shows a comment line.
// The ledger names the writes, where runs 12 and 13 read them from shell text and overcounted. A chained
// command's output mixes its parts, so it goes to a hand read and never counts.
func taint(r *runner, opts options, _ []string) int {
	ledger := ""
	home, _ := os.LookupEnv("HOME")
	counted, byHand, scripted, read, rules := 0, 0, 0, 0, 0
	for _, arg := range opts.order {
		if value, found := strings.CutPrefix(arg, "--ledger="); found {
			ledger = value
			continue
		}
		if strings.HasPrefix(arg, "--") {
			return r.refuse("taint takes no %s", shell.Echoable(arg))
		}
		if ledger == "" {
			return r.refuse("%s names no --ledger before it", shell.Echoable(arg))
		}
		files, err := ledgerFiles(ledger)
		if err != nil {
			return r.refuse("cannot read %s", shell.Echoable(ledger))
		}
		calls, err := transcriptCalls(arg)
		if err != nil {
			return r.refuse("cannot read %s", shell.Echoable(arg))
		}
		read++
		// A run prompted with the rules by name holds their sum, and its writers are held to the reads.
		if runDir := filepath.Dir(ledger); rulesHeld(runDir) != "" {
			for _, finding := range ruleReads(calls, files, home, promptedAt(ledger)) {
				rules++
				fmt.Fprintf(r.stdout, "%s: rules: %s\n", arg, finding)
			}
		}
		for _, hit := range hits(calls, files, ledger) {
			if hit.isScriptWrite {
				scripted++
				fmt.Fprintf(r.stdout, "%s: script write: call %d wrote %s by shell, where writes go through the Edit tool: %s\n",
					arg, hit.at, hit.file, shell.CutBytesMarked(hit.command, 200))
				continue
			}
			if hit.isChained {
				byHand++
				fmt.Fprintf(r.stdout, "%s: read by hand: call %d, %s\n", arg, hit.at, shell.CutBytesMarked(hit.command, 200))
				continue
			}
			counted++
			fmt.Fprintf(r.stdout, "%s: tainted: call %d read %s before the last write to it: %s\n", arg, hit.at,
				hit.file, shell.CutBytesMarked(hit.command, 200))
		}
	}
	if read == 0 {
		return r.refuse("%s", "taint takes --ledger=<file> and the transcripts it covers")
	}
	fmt.Fprintf(r.stderr, "%s: %d transcript(s), %d tainted read(s), %d to read by hand, %d script write(s), %d rule-read finding(s)\n",
		r.self, read, counted, byHand, scripted, rules)
	if counted > 0 || byHand > 0 || scripted > 0 || rules > 0 {
		return exitFindings
	}
	return exitClean
}

// promptedAt is when the prompt of the writer keeping ledger was written, or zero where none stands
// beside it. The ledger is `comment-writer-<name>-queue.md`, and the prompt `spawn-writer-<name>.md`.
func promptedAt(ledger string) time.Time {
	name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(ledger), "comment-writer-"), "-queue.md")
	info, err := os.Stat(spawnFile(filepath.Dir(ledger), name))
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// call is one tool call of a transcript, in order, with what it returned, the model message that made
// it and when.
type call struct {
	at      int
	tool    string
	input   map[string]any
	result  string
	message string
	time    time.Time
}

func (c call) text(key string) string {
	v, _ := c.input[key].(string)
	return v
}

// transcriptCalls reads a JSONL transcript's tool calls and their results.
func transcriptCalls(path string) ([]call, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var calls []call
	byID := map[string]int{}
	scan := bufio.NewScanner(file)
	scan.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for scan.Scan() {
		var record struct {
			Timestamp time.Time `json:"timestamp"`
			Message   struct {
				ID      string          `json:"id"`
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(scan.Bytes(), &record) != nil {
			continue
		}
		var parts []struct {
			Type      string          `json:"type"`
			ID        string          `json:"id"`
			Name      string          `json:"name"`
			Input     map[string]any  `json:"input"`
			ToolUseID string          `json:"tool_use_id"`
			Content   json.RawMessage `json:"content"`
		}
		if json.Unmarshal(record.Message.Content, &parts) != nil {
			continue
		}
		for _, part := range parts {
			switch part.Type {
			case "tool_use":
				byID[part.ID] = len(calls)
				calls = append(calls, call{at: len(calls) + 1, tool: part.Name, input: part.Input,
					message: record.Message.ID, time: record.Timestamp})
			case "tool_result":
				if at, found := byID[part.ToolUseID]; found {
					calls[at].result = resultText(part.Content)
				}
			}
		}
	}
	return calls, scan.Err()
}

// resultText is a tool result's text, which arrives as a string or as a list of text parts.
func resultText(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var out []string
	for _, part := range parts {
		out = append(out, part.Text)
	}
	return strings.Join(out, "\n")
}

var reLedgerEntry = regexp.MustCompile(`(?m)^\s*(?:[-*]\s*)?(?:Block|File|Comment) \d+/\d+ ([^\s:|]+)`)

// ledgerFiles is every file the writer's ledger holds an entry for.
func ledgerFiles(path string) ([]string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var files []string
	for _, m := range reLedgerEntry.FindAllStringSubmatch(string(body), -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			files = append(files, m[1])
		}
	}
	return files, nil
}

var (
	reHistoryRead = regexp.MustCompile(`\bgit\s+(diff|show|log)\b`)
	reChained     = regexp.MustCompile(`;|&&|\|\||\n`)
	// A diff prints a removed line under `-`: run 18's writers ran `git diff -- <file>` with no range, and
	// it printed HEAD's blocks against the stripped tree.
	reCommentOut = regexp.MustCompile(`(?m)^[+-]?\s*(\d+[:\t]\s*)?(//|/\*|\*\s|\*/|#\s)`)
	// reScriptEdit is a shell command that edits a file it names in place: sed or perl with -i, tee, or
	// a script that writes. Writes go through the Edit tool, and run 18b's writer wrote by script once
	// and one insert slipped.
	reScriptEdit = regexp.MustCompile(`\bsed\s+(-\w+\s+)*-i|\bperl\s+-\w*i|\btee\b|\b(python3?|node|ruby)\b[^|]*\b(write|writeFile|open\([^)]*['"]w)`)
	// A log's graph opens lines on `* `, so a log read counts only a line opening a block. Run 13's one
	// log read flagged every file its writer held.
	reBlockOpener = regexp.MustCompile(`(?m)^[+-]?\s*(\d+[:\t]\s*)?(//|/\*)`)
	reLogRead     = regexp.MustCompile(`\bgit\s+log\b`)
	// reRedirectTarget is a redirect with its target in the first group. A redirect counts only when its
	// target is the file, since `2>&1` and a pipe into a check name the file and leave it as it was.
	reRedirectTarget = regexp.MustCompile(`>>?\s*['"]?(\S+)`)
	// reFlagValue is a file named as a flag's value, which a check reads. Run 25's writer wrote two
	// scratch files by python and named the source file to a check by `--file=`. The stage read a write.
	reFlagValue = regexp.MustCompile(`(?:^|\s)--[\w-]+=[^\s'"]+`)
)

// hit is one history read that showed the writer a comment line of a file it still wrote to after, or
// a write to such a file by shell script.
type hit struct {
	at            int
	file          string
	command       string
	isChained     bool
	isScriptWrite bool
}

// hits reads the calls against the ledger's files. The last write to a file is the ledger's entry for
// it: the call writing the ledger with the file's name in it. Where the ledger was never written that
// way, it is the last edit of the file itself.
func hits(calls []call, files []string, ledger string) []hit {
	byLedger, byEdit := map[string]int{}, map[string]int{}
	for _, c := range calls {
		target := c.text("file_path") + c.text("path") + c.text("notebook_path")
		command := c.text("command")
		writesLedger := strings.Contains(target, ledger) || strings.Contains(command, ledger)
		for _, file := range files {
			switch {
			case writesLedger && strings.Contains(fmt.Sprint(c.input), file):
				byLedger[file] = c.at
			case (c.tool == "Edit" || c.tool == "Write" || c.tool == "MultiEdit") && strings.HasSuffix(target, file):
				byEdit[file] = c.at
			case c.tool == "Bash" && !writesLedger && scriptWrites(command, file):
				byEdit[file] = c.at
			}
		}
	}
	last := func(file string) int {
		if at, found := byLedger[file]; found {
			return at
		}
		return byEdit[file]
	}
	var out []hit
	for _, c := range calls {
		command := c.text("command")
		if c.tool == "Bash" && !strings.Contains(command, ledger) {
			for _, file := range named(command, files) {
				if scriptWrites(command, file) {
					out = append(out, hit{at: c.at, file: file, command: shell.Oneline(command), isScriptWrite: true})
				}
			}
		}
		if c.tool != "Bash" || !reHistoryRead.MatchString(command) || !reCommentOut.MatchString(c.result) {
			continue
		}
		if reLogRead.MatchString(command) && !reHistoryRead.MatchString(reLogRead.ReplaceAllString(command, "")) &&
			!reBlockOpener.MatchString(c.result) {
			continue
		}
		targets := named(command, files)
		if len(targets) == 0 {
			targets = files
		}
		for _, file := range targets {
			if c.at < last(file) {
				out = append(out, hit{at: c.at, file: file, command: shell.Oneline(command),
					isChained: reChained.MatchString(strings.TrimSpace(command))})
				break
			}
		}
	}
	return out
}

// named is the ledger's files a command names.
func named(command string, files []string) []string {
	var out []string
	for _, file := range files {
		if strings.Contains(command, file) {
			out = append(out, file)
		}
	}
	return out
}

// scriptWrites says the shell command writes the file: an in-place edit naming it other than as a flag's
// value, or a redirect into it.
func scriptWrites(command, file string) bool {
	if !strings.Contains(command, file) {
		return false
	}
	if reScriptEdit.MatchString(command) && strings.Contains(reFlagValue.ReplaceAllString(command, ""), file) {
		return true
	}
	for _, m := range reRedirectTarget.FindAllStringSubmatch(command, -1) {
		if strings.Contains(m[1], file) {
			return true
		}
	}
	return false
}

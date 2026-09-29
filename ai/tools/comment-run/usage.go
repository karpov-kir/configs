package commentrun

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"configs/ai/tools/shell"
)

// usageStage reads each writer's transcript and prints the report's figures for it. They are its
// context at the first tool call and at its last turn, its output, its tool calls and its wall time.
// Runs 16 and 18 estimated a writer's start-up from totals, and the report had no line to read it from.
func usageStage(r *runner, _ options, transcripts []string) int {
	if len(transcripts) == 0 {
		return r.refuse("%s", "usage takes the writers' transcripts")
	}
	fmt.Fprintln(r.stdout, "writer | context at first call | context at end | output | tool calls | wall time")
	for _, path := range transcripts {
		u, err := readUsage(path)
		if err != nil {
			return r.refuse("cannot read %s", shell.Echoable(path))
		}
		fmt.Fprintf(r.stdout, "%s | %d | %d | %d | %d | %d s\n", filepath.Base(path), u.first, u.last, u.output, u.calls,
			int(u.end.Sub(u.start).Seconds()))
	}
	return exitClean
}

// transcriptUsage is one transcript's figures. A context is a turn's input, cache read and cache written
// together, which is what the model held at that turn.
type transcriptUsage struct {
	first, last, output, calls int
	start, end                 time.Time
}

func readUsage(path string) (transcriptUsage, error) {
	var u transcriptUsage
	file, err := os.Open(path)
	if err != nil {
		return u, err
	}
	defer file.Close()
	scan := bufio.NewScanner(file)
	scan.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for scan.Scan() {
		var record struct {
			Timestamp time.Time `json:"timestamp"`
			Message   struct {
				Content json.RawMessage `json:"content"`
				Usage   *struct {
					Input       int `json:"input_tokens"`
					CacheRead   int `json:"cache_read_input_tokens"`
					CacheCreate int `json:"cache_creation_input_tokens"`
					Output      int `json:"output_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal(scan.Bytes(), &record) != nil {
			continue
		}
		if !record.Timestamp.IsZero() {
			if u.start.IsZero() {
				u.start = record.Timestamp
			}
			u.end = record.Timestamp
		}
		if record.Message.Usage == nil {
			continue
		}
		context := record.Message.Usage.Input + record.Message.Usage.CacheRead + record.Message.Usage.CacheCreate
		u.last = context
		u.output += record.Message.Usage.Output
		var parts []struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(record.Message.Content, &parts) != nil {
			continue
		}
		for _, part := range parts {
			if part.Type == "tool_use" {
				if u.calls == 0 {
					u.first = context
				}
				u.calls++
			}
		}
	}
	return u, scan.Err()
}

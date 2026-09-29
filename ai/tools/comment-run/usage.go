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

// usageStage reads each writer's transcript and prints the run's cost lines, per writer and in total.
// Each turn reads the whole context again from the cache, so a writer's cost is its context times its
// turns. Run 18's one writer took 133 turns and read 34.9M tokens from the cache, and its end figure of
// 431k hid that.
func usageStage(r *runner, _ options, transcripts []string) int {
	if len(transcripts) == 0 {
		return r.refuse("%s", "usage takes the writers' transcripts")
	}
	fmt.Fprintln(r.stdout, "writer | turns | context at start | context at first tool call | context at end | cache read | cache written | output | tool calls | wall time")
	var total transcriptUsage
	for _, path := range transcripts {
		u, err := readUsage(path)
		if err != nil {
			return r.refuse("cannot read %s", shell.Echoable(path))
		}
		fmt.Fprintf(r.stdout, "%s | %s\n", filepath.Base(path), u.line())
		total.turns += u.turns
		total.cacheRead += u.cacheRead
		total.cacheWritten += u.cacheWritten
		total.output += u.output
		total.calls += u.calls
		total.wall += u.wall
	}
	fmt.Fprintf(r.stdout, "total | %d | | | | %d | %d | %d | %d | %d s\n", total.turns, total.cacheRead,
		total.cacheWritten, total.output, total.calls, int(total.wall.Seconds()))
	return exitClean
}

// transcriptUsage is one transcript's figures. A turn is one model message, counted once by its id,
// though the transcript writes a line for each of its content blocks. A context is a turn's input,
// cache read and cache written together, which is what the model held at that turn.
type transcriptUsage struct {
	turns, start, first, last              int
	cacheRead, cacheWritten, output, calls int
	wall                                   time.Duration
}

func (u transcriptUsage) line() string {
	return fmt.Sprintf("%d | %d | %d | %d | %d | %d | %d | %d | %d s", u.turns, u.start, u.first, u.last, u.cacheRead,
		u.cacheWritten, u.output, u.calls, int(u.wall.Seconds()))
}

func readUsage(path string) (transcriptUsage, error) {
	var u transcriptUsage
	file, err := os.Open(path)
	if err != nil {
		return u, err
	}
	defer file.Close()
	seen := map[string]bool{}
	var start, end time.Time
	scan := bufio.NewScanner(file)
	scan.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for scan.Scan() {
		var record struct {
			Timestamp time.Time `json:"timestamp"`
			Message   struct {
				ID      string          `json:"id"`
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
			if start.IsZero() {
				start = record.Timestamp
			}
			end = record.Timestamp
		}
		var parts []struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(record.Message.Content, &parts)
		usage := record.Message.Usage
		if usage == nil {
			continue
		}
		context := usage.Input + usage.CacheRead + usage.CacheCreate
		if !seen[record.Message.ID] || record.Message.ID == "" {
			seen[record.Message.ID] = true
			u.turns++
			if u.turns == 1 {
				u.start = context
			}
			u.cacheRead += usage.CacheRead
			u.cacheWritten += usage.CacheCreate
			u.output += usage.Output
		}
		u.last = context
		for _, part := range parts {
			if part.Type == "tool_use" {
				if u.calls == 0 {
					u.first = context
				}
				u.calls++
			}
		}
	}
	u.wall = end.Sub(start)
	return u, scan.Err()
}

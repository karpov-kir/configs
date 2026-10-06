package commentpass

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Caller sends one file's call: the page as the system prompt and the file's prompt as the message.
// It returns the reply's text. The CLI caller runs today. An API caller at temperature 0 takes the
// same shape, once an API key is added.
type Caller func(system, user string) (string, error)

// callTimeout bounds one file's call. A file and the page fit in one turn with no tools.
const callTimeout = 10 * time.Minute

// CLICaller runs `claude -p` with the page and the prompt as its only input. The call gets no tools,
// settings or MCP servers, and the page stands in for the CLI's own system prompt.
func CLICaller(model string) Caller {
	return func(system, user string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
		defer cancel()
		args := []string{"-p", "--output-format", "json", "--tools", "", "--setting-sources", "",
			"--strict-mcp-config", "--system-prompt", system}
		if model != "" {
			args = append(args, "--model", model)
		}
		cmd := exec.CommandContext(ctx, "claude", args...)
		cmd.Stdin = strings.NewReader(user)
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		runErr := cmd.Run()
		var reply struct {
			Result  string `json:"result"`
			IsError bool   `json:"is_error"`
		}
		if json.Unmarshal(out.Bytes(), &reply) != nil {
			return "", fmt.Errorf("the CLI answered no JSON (%v): %s", runErr, firstLine(out.String()+errOut.String()))
		}
		if reply.IsError || runErr != nil {
			return "", fmt.Errorf("the CLI's call failed: %s", firstLine(reply.Result+errOut.String()))
		}
		return reply.Result, nil
	}
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	if len(line) > 200 {
		line = line[:200] + "…"
	}
	return line
}

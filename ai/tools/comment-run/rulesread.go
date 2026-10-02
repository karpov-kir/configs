package commentrun

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	commentstrip "configs/ai/tools/comment-strip"
)

// writerRuleFiles are the two files a writer reads in its first turn, below the flavor root. The prompt
// names them and quotes neither, so a rule has one copy. Run 16's twenty writers each read both, which
// is why the prompt quoted them, and a thin writer of at most three pays one request for the reads.
var writerRuleFiles = []string{"workers/comment-writer.md", "standards/comments.md"}

// rulesFile is where prompts writes the sum of the rules it named. A writer reads the rules from the
// mount when it starts, and a merge between the prompt and the read would hand it rules the archive
// never records.
const rulesFile = "rules.sum"

// recordRules writes the sum of the rules standing now into runDir. A sum the run wrote earlier that
// differs refuses it, since the run's writers would hold two sets of rules.
func recordRules(runDir string) error {
	now := commentstrip.RulesSum()
	if now == "" {
		return fmt.Errorf("cannot read the rules under ~/.kk-flavor")
	}
	if held, err := os.ReadFile(filepath.Join(runDir, rulesFile)); err == nil && strings.TrimSpace(string(held)) != now {
		return fmt.Errorf("the rules changed since this run's first prompt: %s then, %s now; start a new run",
			strings.TrimSpace(string(held)), now)
	}
	return os.WriteFile(filepath.Join(runDir, rulesFile), []byte(now+"\n"), 0o644)
}

// rulesHeld is the sum prompts recorded in runDir, or "" where it recorded none.
func rulesHeld(runDir string) string {
	held, _ := os.ReadFile(filepath.Join(runDir, rulesFile))
	return strings.TrimSpace(string(held))
}

// reRuleTree is a path into the two rule directories, by the mount or by the checkout it points at.
var reRuleTree = regexp.MustCompile(`kk-flavor/(standards|workers)\b[^\s'"|;&)]*`)

// ruleReads checks a writer's calls against its prompt. The first turn holds two calls, each reading a
// rule file whole after the prompt was written. Every later call keeps out of the rule directories,
// except one that runs a script there, such as the voice check.
func ruleReads(calls []call, home string, prompted time.Time) []string {
	if len(calls) == 0 {
		return []string{"the transcript holds no call, so the writer read no rule"}
	}
	resolve := func(path string) string {
		if rest, found := strings.CutPrefix(path, "~/"); found {
			path = filepath.Join(home, rest)
		}
		if real, err := filepath.EvalSymlinks(path); err == nil {
			return real
		}
		return path
	}
	want := map[string]bool{}
	for _, file := range writerRuleFiles {
		want[resolve(filepath.Join(home, ".kk-flavor", file))] = true
	}
	var out []string
	first := calls[0].message
	read := map[string]bool{}
	for _, c := range calls {
		if c.message != first || first == "" {
			break
		}
		path := resolve(c.text("file_path"))
		switch {
		case c.tool != "Read" || !want[path]:
			out = append(out, fmt.Sprintf("call %d: the first turn makes a call other than the two rule reads", c.at))
		case c.input["offset"] != nil || c.input["limit"] != nil:
			out = append(out, fmt.Sprintf("call %d: reads %s in part", c.at, filepath.Base(path)))
		case prompted.IsZero() || c.time.IsZero():
			out = append(out, fmt.Sprintf("call %d: no prompt time or call time to show the read came after the prompt", c.at))
		case c.time.Before(prompted):
			out = append(out, fmt.Sprintf("call %d: reads %s before its prompt was written", c.at, filepath.Base(path)))
		default:
			read[path] = true
		}
	}
	if len(read) != len(want) {
		out = append(out, fmt.Sprintf("the first turn reads %d of the %d rule files whole", len(read), len(want)))
	}
	for _, c := range calls {
		if c.message == first && first != "" {
			continue
		}
		var named []string
		for _, path := range reRuleTree.FindAllString(c.text("command")+" "+c.text("file_path")+" "+c.text("path")+" "+c.text("pattern"), -1) {
			if c.tool != "Bash" || !strings.HasSuffix(path, ".sh") {
				named = append(named, path)
			}
		}
		if len(named) > 0 {
			out = append(out, fmt.Sprintf("call %d: reads %s, outside the two rule files", c.at, named[0]))
		}
	}
	return out
}

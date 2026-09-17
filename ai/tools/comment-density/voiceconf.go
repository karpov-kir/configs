// The voice check's configuration: the words this repository coined, and the findings it has decided
// to keep. Both live in one file so a repository states its own vocabulary once.
//
// The file is `comment-voice.conf`, looked for in this order: COMMENT_VOICE_CONF, then the
// repository's own `.kk-flavor/comment-voice.conf`, then
// `${XDG_CONFIG_HOME:-~/.config}/kk-flavor/comment-voice.conf`. The repository's own copy comes before
// the machine's because a coined word is a property of the codebase, not of who is typing.
//
// A missing file is not an error: the scan runs with no coined words and no allowlist, which is the
// setting every repository starts at.
package density

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"kk-flavor/tools/shell"
)

const voiceConfName = "comment-voice.conf"

// An allowlist entry is one finding a human decided to keep, by the exact text that matched.
//
// The reason is required and never parsed. It is there so the next reader can see why the entry
// exists, and so an entry cannot be added by a run that was only trying to reach zero: writing a
// sentence about why is the cost that makes an allowlist an argument rather than a suppression.
type allowEntry struct {
	check  string
	text   string
	reason string
}

type allowlist []allowEntry

func (a allowlist) filter(found []Finding) []Finding {
	if len(a) == 0 {
		return found
	}
	var kept []Finding
	for _, f := range found {
		if !a.covers(f) {
			kept = append(kept, f)
		}
	}
	return kept
}

func (a allowlist) covers(f Finding) bool {
	for _, entry := range a {
		if entry.check == f.Check && entry.text == f.Text {
			return true
		}
	}
	return false
}

// voiceConfig reads the conf, returning the coined words and the allowlist. A conf that does not parse
// refuses the run: a scan that silently ignored half its own allowlist would report findings a human
// already answered, and the writer would learn to ignore the report.
func voiceConfig(cwd string) ([]string, allowlist, error) {
	path, ok := voiceConfPath(cwd)
	if !ok {
		return nil, nil, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read %s — exit 2, the scan did NOT run",
			shell.CutBytesMarked(shell.Oneline(path), maxPathBytes))
	}
	coined, allowed, err := parseVoiceConf(string(body))
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w — exit 2, the scan did NOT run",
			shell.CutBytesMarked(shell.Oneline(path), maxPathBytes), err)
	}
	return coined, allowed, nil
}

func voiceConfPath(cwd string) (string, bool) {
	if named, set := os.LookupEnv("COMMENT_VOICE_CONF"); set && named != "" {
		return named, true
	}
	if repo := shell.Join(shell.Join(cwd, ".kk-flavor"), voiceConfName); shell.IsRegularFile(repo) {
		return repo, true
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		base = shell.Join(home, ".config")
	}
	machine := shell.Join(shell.Join(base, "kk-flavor"), voiceConfName)
	return machine, shell.IsRegularFile(machine)
}

// parseVoiceConf reads two line shapes:
//
//	coined <word>                       a word this repository coined
//	allow <check> <matched text> # <reason>
//
// The reason is separated by ` # ` because a matched text can hold anything a comment can, spaces
// included, and a positional separator would cut the text at its first space.
func parseVoiceConf(body string) ([]string, allowlist, error) {
	var coined []string
	var allowed allowlist
	for number, raw := range shell.SplitLines(body) {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		keyword, rest, _ := strings.Cut(line, " ")
		rest = strings.TrimSpace(rest)
		switch keyword {
		case "coined":
			if rest == "" {
				return nil, nil, fmt.Errorf("line %d names no word to treat as coined", number+1)
			}
			coined = append(coined, rest)
		case "allow":
			entry, err := parseAllowLine(rest, number+1)
			if err != nil {
				return nil, nil, err
			}
			allowed = append(allowed, entry)
		default:
			return nil, nil, fmt.Errorf("line %d starts with %q, which is neither `coined` nor `allow`",
				number+1, shell.CutBytesMarked(shell.Oneline(keyword), 40))
		}
	}
	return coined, allowed, nil
}

func parseAllowLine(rest string, number int) (allowEntry, error) {
	check, remainder, found := strings.Cut(rest, " ")
	if !found {
		return allowEntry{}, fmt.Errorf("line %d allows nothing: an entry is `allow <check> <matched text> # <reason>`", number)
	}
	if !slices.Contains(AllChecks, check) {
		return allowEntry{}, fmt.Errorf("line %d allows check %q, which is not one this scan runs. Checks: %s",
			number, shell.CutBytesMarked(shell.Oneline(check), 40), strings.Join(AllChecks, " "))
	}
	text, reason, hasReason := strings.Cut(remainder, " # ")
	text = strings.TrimSpace(text)
	reason = strings.TrimSpace(reason)
	if text == "" {
		return allowEntry{}, fmt.Errorf("line %d allows check %q with no matched text to match against", number, check)
	}
	if !hasReason || reason == "" {
		return allowEntry{}, fmt.Errorf("line %d allows %q with no reason after ` # `; an entry with no reason is refused", number, check)
	}
	return allowEntry{check: check, text: text, reason: reason}, nil
}

// The voice check's configuration: the words a codebase coined, and the findings it has decided to
// keep. Both live in one file so the vocabulary is stated once.
//
// The file is `comment-voice.conf`. When COMMENT_VOICE_CONF names a file, that is the only file read.
// Without it, the flavor's copy in its configs directory and this machine's
// `${XDG_CONFIG_HOME:-~/.config}/kk-flavor/comment-voice.conf` are read, and their entries add up.
// An override would drop one codebase's words wherever the other file answered first.
//
// The repository being checked keeps no copy. A tree under review that could list findings to keep
// could silence the check on itself.
//
// Where no conf exists, the scan runs with an empty vocabulary and allowlist, the setting every
// codebase starts at. A file COMMENT_VOICE_CONF names must exist, because the caller asked for it.
package voicecheck

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"unicode/utf8"

	"configs/ai/tools/flavorconfig"
	"configs/ai/tools/shell"
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

// The conf is a settings file, not a corpus. A file over this is not one somebody typed.
const maxVoiceConfBytes = 64 * 1024

// Where a conf came from, which the run prints. The flavor's conf and the operator's own carry
// different weight, and the path does not show it.
const (
	confNamed   = "named by COMMENT_VOICE_CONF"
	confShipped = "shipped with the flavor"
	confMachine = "this machine's"
)

type voiceConfSource struct {
	path   string
	origin string
}

// voiceConfig reads every conf that applies and adds up their entries. It returns the coined words,
// the domain words, the allowlist, and a phrase naming the files it read. A conf that does not parse
// refuses the run. A scan that skipped half its allowlist would report findings a human answered.
//
// A conf that is present and unusable also refuses: a dangling symlink, a directory, an unreadable
// file. Skipped, it would leave the scan clean without that file's entries, and that looks the same as
// the config working (ecosystem.md → Conventions a new file joins).
func voiceConfig() ([]string, []string, allowlist, string, error) {
	var coined, domain []string
	var allowed allowlist
	var answered []string
	for _, source := range voiceConfSources() {
		c, d, a, err := readVoiceConf(source)
		if err != nil {
			return nil, nil, nil, "", err
		}
		coined, domain, allowed = append(coined, c...), append(domain, d...), append(allowed, a...)
		answered = append(answered, source.origin+" "+shell.CutBytesMarked(shell.Oneline(source.path), maxPathBytes))
	}
	return coined, domain, allowed, strings.Join(answered, " and "), nil
}

func readVoiceConf(source voiceConfSource) ([]string, []string, allowlist, error) {
	named := shell.CutBytesMarked(shell.Oneline(source.path), maxPathBytes)
	refuse := func(why string) ([]string, []string, allowlist, error) {
		return nil, nil, nil, fmt.Errorf("%s (%s) %s — exit 2, the scan did NOT run", named, source.origin, why)
	}
	info, err := os.Lstat(source.path)
	if err != nil {
		return refuse("is not there")
	}
	if !info.Mode().IsRegular() {
		return refuse("is not a regular file this scan will read")
	}
	body, err := readCapped(source.path, maxVoiceConfBytes)
	if err != nil {
		return refuse(err.Error())
	}
	coined, domain, allowed, err := parseVoiceConf(body)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%s (%s): %w — exit 2, the scan did NOT run", named, source.origin, err)
	}
	return coined, domain, allowed, nil
}

// readCapped reads a file and refuses one that is larger than the cap. The cap is enforced on the READ
// rather than on the size Lstat reported: a file can grow between the two, and on some systems a
// special file reports a size it does not have.
func readCapped(path string, cap int64) (string, error) {
	handle, err := os.Open(path)
	if err != nil {
		return "", errors.New("cannot be read")
	}
	defer handle.Close()
	body, err := io.ReadAll(io.LimitReader(handle, cap+1))
	if err != nil {
		return "", errors.New("cannot be read")
	}
	if int64(len(body)) > cap {
		return "", errors.New("is larger than a settings file")
	}
	return string(body), nil
}

// voiceConfSources says which confs apply. A path the environment names always applies, because the
// caller asked for it. Each of the other two applies when something sits at its path. Lstat does the
// probing, so a dangling symlink counts as present and the read refuses it.
func voiceConfSources() []voiceConfSource {
	if named, set := os.LookupEnv("COMMENT_VOICE_CONF"); set && named != "" {
		return []voiceConfSource{{named, confNamed}}
	}
	home, _ := os.UserHomeDir()
	var sources []voiceConfSource
	if shipped := flavorconfig.Path(home, voiceConfName); shipped != "" && exists(shipped) {
		sources = append(sources, voiceConfSource{shipped, confShipped})
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" && home != "" {
		base = shell.Join(home, ".config")
	}
	if base != "" {
		if machine := shell.Join(shell.Join(base, "kk-flavor"), voiceConfName); exists(machine) {
			sources = append(sources, voiceConfSource{machine, confMachine})
		}
	}
	return sources
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// parseVoiceConf reads two line shapes:
//
//	coined <word>                       a word this repository coined
//	allow <check> <matched text> # <reason>
//
// The reason is separated by ` # ` because a matched text can hold anything a comment can, spaces
// included, and a positional separator would cut the text at its first space.
func parseVoiceConf(body string) ([]string, []string, allowlist, error) {
	var coined []string
	var domain []string
	var allowed allowlist
	for number, raw := range shell.SplitLines(body) {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Refused before anything is built from it. A coined word is interpolated into a regular
		// expression, and Go's regexp rejects invalid UTF-8 — reached through MustCompile that is a
		// panic printing the conf's own bytes and a stack trace of absolute host paths, which undoes
		// the whole point of refusing without echoing the file.
		if !utf8.ValidString(line) {
			return nil, nil, nil, fmt.Errorf("line %d is not valid UTF-8", number+1)
		}
		keyword, rest, _ := strings.Cut(line, " ")
		rest = strings.TrimSpace(rest)
		switch keyword {
		case "coined":
			if rest == "" {
				return nil, nil, nil, fmt.Errorf("line %d names no word to treat as coined", number+1)
			}
			coined = append(coined, rest)
		// A `domain` word is a compound this codebase's own identifiers spell and its readers know.
		// The coined-identifier check fires on every other compound the code spells, so a repository
		// seeds the ones its readers already carry and a new invention stands out against them.
		case "domain":
			if rest == "" {
				return nil, nil, nil, fmt.Errorf("line %d names no word to treat as the domain's", number+1)
			}
			domain = append(domain, rest)
		case "allow":
			entry, err := parseAllowLine(rest, number+1)
			if err != nil {
				return nil, nil, nil, err
			}
			allowed = append(allowed, entry)
		default:
			return nil, nil, nil, fmt.Errorf("line %d starts with a word that is none of `coined`, `domain` and `allow`", number+1)
		}
	}
	return coined, domain, allowed, nil
}

func parseAllowLine(rest string, number int) (allowEntry, error) {
	check, remainder, found := strings.Cut(rest, " ")
	if !found {
		return allowEntry{}, fmt.Errorf("line %d allows nothing: an entry is `allow <check> <matched text> # <reason>`", number)
	}
	if !slices.Contains(AllChecks, check) {
		return allowEntry{}, fmt.Errorf("line %d allows a check this scan does not run. Checks: %s",
			number, strings.Join(AllChecks, " "))
	}
	text, reason, hasReason := strings.Cut(remainder, " # ")
	text = strings.TrimSpace(text)
	reason = strings.TrimSpace(reason)
	if text == "" {
		return allowEntry{}, fmt.Errorf("line %d allows a check with no matched text to match against", number)
	}
	if !hasReason || reason == "" {
		return allowEntry{}, fmt.Errorf("line %d allows a check with no reason after ` # `; an entry with no reason is refused", number)
	}
	return allowEntry{check: check, text: text, reason: reason}, nil
}

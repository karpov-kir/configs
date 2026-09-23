// The voice check's configuration: the words this repository coined, and the findings it has decided
// to keep. Both live in one file so a repository states its own vocabulary once.
//
// The file is `comment-voice.conf`, looked for in this order: COMMENT_VOICE_CONF, then the
// repository's own `.kk-flavor/comment-voice.conf`, then
// `${XDG_CONFIG_HOME:-~/.config}/kk-flavor/comment-voice.conf`. The repository's own copy comes before
// the machine's because a coined word is a property of the codebase, not of who is typing.
//
// No conf on the search path is not an error: the scan runs with no coined words and no allowlist,
// which is the setting every repository starts at. A conf NAMED by COMMENT_VOICE_CONF and then absent
// is an error, because the caller asked for a file and did not get it.
package voicecheck

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"unicode/utf8"

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

// Where a conf came from, so a run can say which one answered. A conf shipped by the tree under review
// and a conf the operator set on their own machine carry different weight, and a reader of the report
// cannot tell them apart from a path alone.
const (
	confNamed      = "named by COMMENT_VOICE_CONF"
	confRepository = "shipped by this working tree"
	confMachine    = "this machine's"
)

// voiceConfig reads the conf, returning the coined words, the allowlist, a phrase naming what answered,
// and nothing at all where no conf exists. A conf that does not parse refuses the run: a scan that
// silently ignored half its own allowlist would report findings a human already answered.
//
// Present-but-unusable refuses rather than falling back. A dangling symlink, a directory or an
// unreadable file at either path would otherwise leave the scan running with no coined words and no
// allowlist, reporting clean — and a default quietly restored is indistinguishable from the override
// working (ecosystem.md → Conventions a new file joins).
//
// One Lstat decides both selection and validity. Split across two calls, the tree under review could
// ship a symlink that passes selection and fails validation, which refuses the run and takes the
// machine's own conf out of reach — a branch disabling the check for anyone who reads it.
func voiceConfig(cwd string) ([]string, []string, allowlist, string, error) {
	path, origin, found := voiceConfPath(cwd)
	if !found {
		return nil, nil, nil, "", nil
	}
	named := shell.CutBytesMarked(shell.Oneline(path), maxPathBytes)
	refuse := func(why string) ([]string, []string, allowlist, string, error) {
		return nil, nil, nil, "", fmt.Errorf("%s (%s) %s — exit 2, the scan did NOT run", named, origin, why)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return refuse("is not there")
	}
	if !info.Mode().IsRegular() {
		return refuse("is not a regular file this scan will read")
	}
	body, err := readCapped(path, maxVoiceConfBytes)
	if err != nil {
		return refuse(err.Error())
	}
	coined, domain, allowed, err := parseVoiceConf(body)
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("%s (%s): %w — exit 2, the scan did NOT run", named, origin, err)
	}
	return coined, domain, allowed, origin + " " + named, nil
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

// voiceConfPath says which conf answers and where it came from. A path named by the environment is
// always "found": the caller asked for that file, so its absence is a refusal rather than a fallback.
// The two searched paths are probed with Lstat, so a dangling symlink counts as present and is refused
// by the caller rather than skipped over in silence.
func voiceConfPath(cwd string) (path, origin string, found bool) {
	if named, set := os.LookupEnv("COMMENT_VOICE_CONF"); set && named != "" {
		return named, confNamed, true
	}
	if repo := shell.Join(shell.Join(cwd, ".kk-flavor"), voiceConfName); exists(repo) {
		return repo, confRepository, true
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", false
		}
		base = shell.Join(home, ".config")
	}
	machine := shell.Join(shell.Join(base, "kk-flavor"), voiceConfName)
	return machine, confMachine, exists(machine)
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// parseVoiceConf reads four line shapes:
//
//	coined <word>                       a word this repository coined
//	domain <word>                       a compound this codebase's readers know
//	allow <check> <matched text> # <reason>
//	band <kind> <words> <median> # <reason>
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
		// A `band` is a body's width for a kind, and `confBands` reads it. It is checked here so a
		// malformed one refuses every run, the way a malformed allow entry does.
		case "band":
			if _, _, err := parseBandLine(rest, number+1); err != nil {
				return nil, nil, nil, err
			}
		default:
			return nil, nil, nil, fmt.Errorf("line %d starts with a word that is none of `coined`, `domain`, `allow` and `band`", number+1)
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

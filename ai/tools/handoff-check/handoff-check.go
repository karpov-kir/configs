// Package handoffcheck is the gate under kk-handoff: it reads a drafted handoff prompt and refuses
// the ones a fresh session cannot act on. A handoff prompt is the only thing the receiving session
// gets — it opens its own tree and holds none of the conversation that produced the work — so every
// scan here asks one question: can somebody act on this without asking the author anything.
//
// It is a library with a thin command beside it, for the reason eco-check states: the suite drives it
// once per case, and a process spawn per case is what makes a mutation run take hours. Nothing here
// writes to os.Stdout or calls os.Exit. Run reports through the writers it is handed and returns the
// code the command exits on, and every counter lives on the scan Run builds, so two runs in one
// process cannot see each other's.
//
// The two things that reach outside the draft are git calls against the repository the caller names:
// whether the base commit resolves, and whether the tree is dirty. Both are read-only, and the SHA
// handed to git is re-checked for its hex-only shape at the call, because that shape is the whole
// reason a token lifted out of a draft is safe to pass.
//
// `handoff-check.sh` in kk-handoff's scripts/ is the stub that reaches this binary.
package handoffcheck

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"kk-flavor/tools/shell"
)

// The template's seven headings, in the order the report walks them, and the three that may not be
// emptied with `None`. The gate's copy of the template lives here and nowhere else; the suite runs
// the gate over the shipped `handoff-prompt.md` so the two cannot drift apart in silence.
var (
	sections   = []string{"The task", "Measured facts", "The lead", "Out of scope", "Traps", "Where it starts", "Licence"}
	refuseNone = map[string]bool{"The task": true, "Where it starts": true, "Licence": true}
)

// A slot holding a word or two is filled only in the sense that it is not blank. Five sits under the
// shortest genuinely useful slot the template shows an example of, so a real answer never trips it
// while "Careful." and "Some." do. Licence is exempt, and has to be: it carries a human sentence
// quoted verbatim, its length is not the drafting agent's to choose, and the only way to clear a
// floor there would be to pad the quote.
const minWords = 5

// Each phrase reaches back into the conversation the receiver never saw. The finding prints the text
// that matched, never the pattern, which would send the author hunting for a literal that is not in
// the draft. The noun list after "the" is closed rather than open: a handoff about filesystem work
// may legitimately say "the directory above", and refusing that costs more than the miss.
var dangling = []string{
	"as discussed",
	"as mentioned",
	"as we discussed",
	"as noted above",
	"see above",
	"the above",
	"the (file|diff|code|snippet|output|command|error|log|script|list) above",
	"as before",
	"as i said",
	"we decided",
	"like last time",
	"in this conversation",
	"earlier in this session",
	"as you saw",
}

// Bounded on both sides, or a phrase matches inside a word: "was discussed" contains "as discussed",
// "was before" contains "as before", and the finding then quotes the author a phrase they cannot find
// written that way anywhere in the draft. `[^0-9A-Za-z]` is `[[:alnum:]]` under LC_ALL=C, which is
// the locale the shell stub's predecessor pinned and the only one these patterns were written for.
var dangleMatchers = compileDangling()

func compileDangling() []*regexp.Regexp {
	matchers := make([]*regexp.Regexp, 0, len(dangling))
	for _, phrase := range dangling {
		matchers = append(matchers, regexp.MustCompile(`(^|[^0-9A-Za-z])(`+phrase+`)([^0-9A-Za-z]|$)`))
	}
	return matchers
}

// Every hex run of seven or more that stands as its own token. Which of them the repository knows is
// git's answer, not this scan's: a "Where it starts" naming a branch and a path holds hex-shaped
// tokens that are not commits, and refusing on the first one would refuse the sensible draft.
//
// Bounded, because a temporary directory is named `handoff-check1691946027` and a draft naming that
// path would hand git the digits out of the middle of a word. The draft then never gets "no base
// commit named" — it gets "base commit does not resolve", quoting a token nobody wrote as a commit.
var hexRun = regexp.MustCompile(`(^|[^0-9A-Za-z])([0-9a-f]{7,})([^0-9A-Za-z]|$)`)

var edgeTrim = regexp.MustCompile(`^[^0-9A-Za-z]+|[^0-9A-Za-z]+$`)

// git runs the read-only git commands this gate needs. Held as a field so the suite can drive the
// scan without a repository where the scan is what is under test.
type runner func(dir string, args ...string) (string, error)

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	return string(out), err
}

// One draft's accumulated state. Nothing here is package-level, so two runs in one process cannot see
// each other's counts.
type scan struct {
	repoPath string // the resolved absolute path the draft has to name

	findings []string
	declared []string // `declared None:` lines, which pass and are printed so the human sees them
	shas     []string // base-commit candidates, in the order they were written

	seen   map[string]bool
	filled map[string]bool
	first  map[string]string
	words  map[string]int

	titles int
	quoted bool // Licence carries a `>` blockquote
	named  bool // "Where it starts" holds the repository's absolute path
}

func newScan(repoPath string) *scan {
	return &scan{
		repoPath: repoPath,
		seen:     map[string]bool{},
		filled:   map[string]bool{},
		first:    map[string]string{},
		words:    map[string]int{},
	}
}

func (s *scan) flag(text string) { s.findings = append(s.findings, text) }

// Run reads the draft, reports through out and errOut, and returns the exit code. prog names this
// program in a refusal, so a stub invoked under its own name says that name back. Findings print one
// per line on out. Two other kinds print alongside them and neither fails the draft: a `declared
// None:` line per slot the draft empties on purpose, and a `note:` when the repository is dirty.
// Returns 1 with findings, 0 when clean, and 2 when it could not run and said why on errOut. A 2
// prints no findings, so a caller must never read it as a clean draft.
func Run(prog, draft, repo string, out, errOut io.Writer) int {
	return run(prog, draft, repo, out, errOut, runGit)
}

func run(prog, draft, repo string, out, errOut io.Writer, git runner) int {
	die := func(format string, args ...any) int {
		fmt.Fprintf(errOut, "%s: %s\n", prog, fmt.Sprintf(format, args...))
		return 2
	}

	if draft == "" {
		return die("usage: %s <draft.md> [<repo>]", prog)
	}
	if repo == "" {
		repo = "."
	}
	body, code := readDraft(draft, die)
	if code != 0 {
		return code
	}
	if !shell.IsDir(repo) {
		return die("not a directory: %s", repo)
	}
	if _, err := git(repo, "rev-parse", "--git-dir"); err != nil {
		return die("not a git work tree: %s — pass the repository as the second argument, or the base commit goes unverified", repo)
	}
	// The chip carries the working directory; a pasted prompt carries nothing, so the draft has to
	// name the repository itself. Resolved rather than trusted as typed, because "." and a relative
	// path both have to match what a draft would sensibly write down.
	repoPath := shell.CanonicalDir(repo)
	if repoPath == "" {
		return die("could not resolve: %s", repo)
	}

	s := newScan(repoPath)
	s.read(shell.SplitLines(body))
	s.report()
	s.resolveBase(repo, git)

	for _, line := range s.declared {
		fmt.Fprintln(out, line)
	}
	if note := dirtyNote(repo, git); note != "" {
		fmt.Fprintln(out, note)
	}
	if len(s.findings) == 0 {
		return 0
	}
	for _, finding := range s.findings {
		fmt.Fprintln(out, finding)
	}
	return 1
}

// readDraft is the draft's own three refusals, in the order the shell gate put them: a path that is
// not a readable regular file cannot be told apart from an unreadable one by a stat alone, so the
// read itself answers both.
func readDraft(draft string, die func(string, ...any) int) (string, int) {
	if !shell.IsRegularFile(draft) {
		return "", die("no such file: %s", draft)
	}
	body, err := readFile(draft)
	if err != nil {
		return "", die("cannot read: %s", draft)
	}
	if len(body) == 0 {
		return "", die("empty draft: %s — nothing was checked", draft)
	}
	return body, 0
}

// read walks the draft once. Fence and comment state come first, so every rule below sees a line in
// the context it really sits in.
func (s *scan) read(lines []string) {
	section := ""
	fence := false
	comment := false

	for i, raw := range lines {
		lineNo := i + 1

		if isFence(raw) {
			fence = !fence
			continue
		}
		if fence {
			// Fenced content counts as content: command output is exactly what Measured facts asks
			// for, and a slot holding only a fence used to be reported as empty, which named the wrong
			// problem. It goes through the same accounting as an unfenced line and stays out of only
			// one thing, the reachback scan, where a pasted transcript would trip phrases nobody wrote.
			s.absorb(section, raw)
			continue
		}

		if comment {
			if strings.Contains(raw, "-->") {
				comment = false
			}
			continue
		}
		if strings.Contains(raw, "<!--") {
			s.flag(fmt.Sprintf("template comment left at line %d — that slot is unfilled", lineNo))
			if !strings.Contains(raw, "-->") {
				comment = true
			}
			continue
		}

		if strings.HasPrefix(raw, "# ") {
			s.readTitle(raw)
			continue
		}
		if strings.HasPrefix(raw, "## ") {
			section = s.readHeading(raw)
			continue
		}

		text := s.absorb(section, raw)
		if section != "" && text != "" {
			if _, held := s.first[section]; !held {
				s.first[section] = text
			}
			if section == "Licence" && isBlockquote(text) {
				s.quoted = true
			}
		}
		// A blockquote in Licence carries the words a human wrote, so a dangling phrase inside one is
		// theirs. Everywhere else a `>` is the drafting agent quoting something it found, and exempting
		// those would let any line dodge this scan by growing a leading marker.
		if section == "Licence" && strings.HasPrefix(text, ">") {
			continue
		}
		s.readReachback(text, lineNo)
	}
}

func (s *scan) readTitle(raw string) {
	title := strings.TrimLeft(raw[2:], shell.SpaceBytes)
	s.titles++
	// Anchored, because a real title may hold an angle bracket. "Cut the run to <10 minutes" is a fine
	// title, and refusing it sends the agent off to reword a sound line.
	if title == "" || (strings.HasPrefix(title, "<") && strings.HasSuffix(title, ">")) {
		s.flag("the title line is still the template placeholder")
	}
}

func (s *scan) readHeading(raw string) string {
	name := strings.TrimRight(raw[3:], shell.SpaceBytes)
	switch {
	case !isRequired(name):
		s.flag("unknown section: " + name + " — the seven headings are fixed")
	case s.seen[name]:
		s.flag("duplicate section: " + name)
	}
	s.seen[name] = true
	return name
}

func (s *scan) readReachback(text string, lineNo int) {
	low := asciiLower(text)
	for _, matcher := range dangleMatchers {
		if hit := matcher.FindString(low); hit != "" {
			// The finding names the way out, because a caller is told to fix what a finding says and
			// never to argue with one. A phrase quoted verbatim cannot be reworded, and without the
			// second half that draft has no repair its instructions allow.
			s.flag(fmt.Sprintf("depends on this conversation: %q at line %d — reword it for someone who was not here, or fence it if it is quoted verbatim",
				edgeTrim.ReplaceAllString(hit, ""), lineNo))
		}
	}
}

// absorb is everything a line contributes to the slot it sits in, so a fenced line and a plain one are
// counted by the same code. Splitting them is what let a fenced "Where it starts" fill its slot and
// still be refused for naming neither the repository nor a commit, both of which were written in it.
// Returns the trimmed line, or "" for one that belongs to no slot.
func (s *scan) absorb(section, raw string) string {
	text := strings.Trim(raw, shell.SpaceBytes)
	if section == "" || text == "" {
		return text
	}
	s.filled[section] = true
	s.words[section] += len(shell.SplitFields(text))
	if section == "Where it starts" {
		// The absolute path, nothing shorter. A basename is a substring test the receiver cannot rely
		// on: a repository called "ai" would be satisfied by the word "available" in the slot, and the
		// check that exists to stop a receiver guessing which checkout to open could no longer fail.
		if strings.Contains(text, s.repoPath) {
			s.named = true
		}
		for _, match := range hexRun.FindAllStringSubmatch(text, -1) {
			s.shas = append(s.shas, match[2])
		}
	}
	return text
}

// report is every finding the whole draft answers for, walked in the template's own heading order so
// the output reads down the page.
func (s *scan) report() {
	if s.titles == 0 {
		s.flag("no title line — the draft needs one `# ` line for the chip title")
	}
	if s.titles > 1 {
		s.flag("more than one title line")
	}
	for _, name := range sections {
		switch {
		case !s.seen[name]:
			s.flag("missing section: " + name)
		case !s.filled[name]:
			s.flag("empty section: " + name)
		case isNone(s.first[name]):
			s.reportNone(name)
		default:
			s.reportSubstance(name)
		}
	}
}

func (s *scan) reportNone(name string) {
	first := s.first[name]
	switch {
	case refuseNone[name]:
		s.flag("None refused in: " + name)
	case first == "None":
		s.flag("None with no reason: " + name)
	default:
		s.declared = append(s.declared, "declared None: "+name+" — "+first)
	}
}

func (s *scan) reportSubstance(name string) {
	if name != "Licence" && s.words[name] < minWords {
		s.flag(fmt.Sprintf("barely filled: %s — %d word(s), which a fresh session cannot act on", name, s.words[name]))
	}
	if name == "Licence" && !s.quoted {
		s.flag("the licence is not quoted: Licence carries no `>` blockquote, so it was paraphrased")
	}
	if name == "Where it starts" && !s.named {
		s.flag("no repository named in: Where it starts — a pasted prompt carries no working directory, so write " + s.repoPath)
	}
}

// resolveBase asks the repository about every hex-shaped token the slot held. One that resolves is
// enough; reporting the first candidate rather than "no commit" tells the human which token was tried.
func (s *scan) resolveBase(repo string, git runner) {
	if len(s.shas) == 0 {
		s.flag("no base commit named in: Where it starts")
		return
	}
	for _, sha := range s.shas {
		// Re-checked here, not trusted from the scan: this string becomes a git argument, and the
		// hex-only shape is the whole reason it is safe to pass one built out of the draft.
		if !isHex(sha) {
			continue
		}
		if _, err := git(repo, "cat-file", "-e", sha+"^{commit}"); err == nil {
			return
		}
	}
	s.flag(fmt.Sprintf("base commit does not resolve in %s: %s", repo, s.shas[0]))
}

// dirtyNote is advisory, never a finding: some handoffs deliberately start from a committed base and
// leave the caller's tree alone. The receiver cannot tell the two apart, and the human can. `-uall`,
// because the default collapses an untracked directory into one status line and this is printed as a
// file count.
func dirtyNote(repo string, git runner) string {
	out, err := git(repo, "status", "--porcelain", "-uall")
	if err != nil {
		return ""
	}
	dirty := len(shell.SplitLines(out))
	if dirty == 0 {
		return ""
	}
	return fmt.Sprintf("note: %s has %d uncommitted file(s) — work not in the base commit does not travel", repo, dirty)
}

func isRequired(name string) bool {
	for _, section := range sections {
		if section == name {
			return true
		}
	}
	return false
}

// isNone is the slot that says there is nothing, in the two shapes the template sanctions: the bare
// word, and the word followed by its reason. "Nonetheless" is not one of them.
func isNone(first string) bool {
	return first == "None" || (strings.HasPrefix(first, "None") && len(first) > 4 && shell.IsSpaceByte(first[4]))
}

func isFence(line string) bool {
	trimmed := strings.TrimLeft(line, shell.SpaceBytes)
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

// isBlockquote is a `>` with something after it, which is what makes Licence a quote rather than a
// stray marker on an empty line.
func isBlockquote(text string) bool {
	rest := strings.TrimLeft(strings.TrimPrefix(text, ">"), shell.SpaceBytes)
	return strings.HasPrefix(text, ">") && rest != ""
}

func isHex(text string) bool {
	if text == "" {
		return false
	}
	for i := 0; i < len(text); i++ {
		b := text[i]
		if (b < '0' || b > '9') && (b < 'a' || b > 'f') {
			return false
		}
	}
	return true
}

// asciiLower is tolower under LC_ALL=C, which touches ASCII and nothing else. strings.ToLower would
// fold non-ASCII too, so a draft's own bytes would decide whether a phrase matched.
func asciiLower(text string) string {
	out := []byte(text)
	for i, b := range out {
		if b >= 'A' && b <= 'Z' {
			out[i] = b + 'a' - 'A'
		}
	}
	return string(out)
}

// readFile is the whole read: a draft is a prompt somebody wrote, so nothing here streams or bounds
// it, and a directory or an unreadable path comes back as the error the caller turns into a refusal.
func readFile(path string) (string, error) {
	body, err := os.ReadFile(path)
	return string(body), err
}

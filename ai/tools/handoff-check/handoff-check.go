// A handoff prompt is all the receiving session gets. It opens its own tree and holds none of the
// conversation that produced the work. Every scan here asks one question: can somebody act on this
// without asking the author anything.

// It is a library with a thin command beside it, for the reason eco-check states. The suite drives
// it once per case, and a process spawn per case is what puts a suite over the time budget
// testing.md sets.

// No code here writes to os.Stdout or calls os.Exit. Run reports through the writers it is handed
// and returns the code the command exits on. Every counter lives on the scan Run builds, so two runs
// in one process cannot see each other's.

// Four things reach outside the draft, all read-only and all against the repository the caller
// names. They are whether the directory is a work tree, whether the base commit resolves, whether
// the tree is dirty, and how repokey abbreviates that clone. All four go through the single
// `repo.Git` Run is handed, so the four answers are about one repository.

// The SHA handed to the port is re-checked for its hex-only shape at the call. That shape is the
// whole reason a token lifted out of a draft is safe to pass.

// The abbreviation goes through repokey, so the prefix the title is held against is the same string
// `repo-key.sh --abbrev` prints.

// `handoff-check.sh` in kk-handoff's scripts/ is the stub that reaches this binary.

// Package handoffcheck is the gate under kk-handoff. It reads a drafted handoff prompt and refuses
// the ones a fresh session cannot act on.
package handoffcheck

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"configs/ai/tools/repo"
	repokey "configs/ai/tools/repo-key"
	"configs/ai/tools/shell"
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
// written that way anywhere in the draft. `[^0-9A-Za-z]` is `[[:alnum:]]` under LC_ALL=C, the only
// locale these patterns were written for.
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

// One draft's accumulated state. Nothing here is package-level, so two runs in one process cannot see
// each other's counts.
type scan struct {
	repoPath   string // the resolved absolute path the draft has to name
	repoAbbrev string // how `repo-key` abbreviates that clone, or "" where it could not say
	prefix     string // the title's opening bracketed word, "" where it holds none worth weighing

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

func newScan(repoPath, repoAbbrev string) *scan {
	return &scan{
		repoPath:   repoPath,
		repoAbbrev: repoAbbrev,
		seen:       map[string]bool{},
		filled:     map[string]bool{},
		first:      map[string]string{},
		words:      map[string]int{},
	}
}

func (s *scan) flag(text string) { s.findings = append(s.findings, text) }

// The bounds on what a message quotes and on the line it becomes. A name sits at the head of its
// message and a path in its middle, so each is cut where it stands: uncut, the words after it — which
// carry the repair — are what the line bound would drop instead. The line bound is eco-check's, at the
// width `report.go` holds, and it backstops the sites no per-field cut reaches.
const (
	findingNameCap = 80
	pathCap        = 120
	lineWidthCap   = 500
)

// printLine is the one place anything leaves this gate, and so the one place the escaping can be held.
// Every line is composed out of the draft's own bytes or out of a path the caller named, and a raw
// `ESC [ 2 K` with a carriage return erases the line it is printed on while `ESC [ n A` first moves up
// over the lines above it. `base commit does not resolve` is the LAST line printed, so an escape
// carried in it reaches every real finding already on screen — and kk-handoff tells the drafting agent
// to fix what a finding names and never to argue with one, which makes a forged finding an
// instruction rather than a smudge.
//
// Held here and nowhere else. A site added later cannot opt out of a printer, and a second guard at
// the message would sit unobservable behind this one. No case could be written that fails when that
// second guard is missing.
func printLine(w io.Writer, text string) {
	fmt.Fprintln(w, shell.CutBytesMarked(shell.Oneline(text), lineWidthCap))
}

// Run reads the draft, reports through out and errOut, and returns the exit code. prog names this
// program in a refusal, so a stub invoked under its own name says that name back. Findings print one
// per line on out. Two other kinds print alongside them and neither fails the draft: a `declared
// None:` line per slot the draft empties on purpose, and a `note:` when the repository is dirty.
// Returns 1 with findings, 0 when clean, and 2 when it could not run and said why on errOut. A 2
// prints no findings, so a caller must never read it as a clean draft.
//
// git answers about repoDir alone, so the command has to hand it an adapter no
// environment can redirect. git reads GIT_DIR and GIT_COMMON_DIR before the directory it was handed.
func Run(prog, draft, repoDir string, git repo.Git, out, errOut io.Writer) int {
	die := func(format string, args ...any) int {
		printLine(errOut, prog+": "+fmt.Sprintf(format, args...))
		return 2
	}

	if draft == "" {
		return die("usage: %s <draft.md> [<repo>]", prog)
	}
	if repoDir == "" {
		repoDir = "."
	}
	body, code := readDraft(draft, die)
	if code != 0 {
		return code
	}
	if !shell.IsDir(repoDir) {
		return die("not a directory: %s", repoDir)
	}
	if _, err := git.GitDir(repoDir); err != nil {
		return die("not a git work tree: %s — pass the repository as the second argument, or the base commit goes unverified", repoDir)
	}
	// The chip carries the working directory; a pasted prompt carries nothing, so the draft has to
	// name the repository itself. Resolved rather than trusted as typed, because "." and a relative
	// path both have to match what a draft would sensibly write down.
	repoPath := shell.CanonicalDir(repoDir)
	if repoPath == "" {
		return die("could not resolve: %s", repoDir)
	}

	// `repoDir`, because the drafting session is often standing in another checkout. An error yields
	// no abbreviation. reportTitlePrefix says why that produces no finding either.

	// The gate's own port, and never one repokey builds for itself. Two adapters can answer about two
	// repositories, and the title would then be weighed against one clone while the base commit and
	// the dirty count came from another.

	// ResolveAbbrev wants an environment that cannot relocate git, and Run's doc puts that on whoever
	// wires the port instead of on this call.
	repoAbbrev, _ := repokey.ResolveAbbrev(git, repoDir)

	s := newScan(repoPath, repoAbbrev)
	s.read(shell.SplitLines(body))
	s.report()
	s.resolveBase(git, repoDir)

	for _, line := range s.declared {
		printLine(out, line)
	}
	if note := dirtyNote(git, repoDir); note != "" {
		printLine(out, note)
	}
	if len(s.findings) == 0 {
		return 0
	}
	for _, finding := range s.findings {
		printLine(out, finding)
	}
	return 1
}

// The draft's own three refusals. A path that is not a readable regular file cannot be told apart from
// an unreadable one by a stat alone, so the read itself answers both.
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

// readTitle reads the `# ` line as the two slots it is — `[<repo abbrev>] <one imperative line>` — and
// each of them on its own. Read as a single string, a filled half in front of an unfilled one made the
// line look done: a real repository prefix hid a work half nobody had written.
func (s *scan) readTitle(raw string) {
	// Trimmed at BOTH ends, unlike every other line reader here until it was the only one that was not:
	// `\r` is in SpaceBytes, so a CRLF draft left it on the end of the work half and no half ever
	// matched its closing `>`. The whole title check passed a draft in which nothing was filled in.
	title := strings.Trim(raw[2:], shell.SpaceBytes)
	s.titles++
	prefix, work, prefixed := titleHalves(title)
	// With no prefix the line IS the work half, and naming a slot the author never wrote would send
	// them looking for one.
	switch {
	case isPlaceholder(work) && prefixed:
		s.flag("the title's work half is still the template placeholder")
	case isPlaceholder(work):
		s.flag("the title line is still the template placeholder")
	}
	// A title carrying no prefix passes. The slot's correctness is held here, its presence is not, and
	// that asymmetry is the ask rather than a gap: the words this was built to are "a consistent prefix
	// where we have it". Refusing a title for not having one goes past them.
	if prefixed {
		s.readTitlePrefix(prefix)
	}
}

// titleHalves splits the title into the repository abbreviation inside its leading brackets and the
// work half after them: `[IT] Cut the run` is "IT" and "Cut the run". The bracket has to open the
// line, and a line without one is all work half — a title reading "Cut the [flaky] resolver test out"
// would otherwise hand its own first words over to be refused as a repository abbreviation.
//
// Cut on `]` and not on `] `, so the space after it is the work half's to lose. `# [IT]` alone was
// read as one unsplit line, which is neither half filled in and was the shape this whole check
// exists to refuse; `[IT]Cut the run` skipped the prefix check for want of that space.
func titleHalves(title string) (prefix, work string, prefixed bool) {
	if !strings.HasPrefix(title, "[") {
		return "", title, false
	}
	prefix, work, prefixed = strings.Cut(title, "]")
	if !prefixed {
		return "", title, false
	}
	return strings.TrimPrefix(prefix, "["), strings.TrimLeft(work, shell.SpaceBytes), true
}

// readTitlePrefix reads the bracketed word the title opens with. Unfilled is answered here, because
// one look at the line settles it; which name stands there takes a second slot to judge and is
// reportTitlePrefix's, once the whole draft has been walked.
func (s *scan) readTitlePrefix(prefix string) {
	if isPlaceholder(prefix) {
		s.flag("the title's repository prefix is still the template placeholder — fill it from the tool the template names")
		return
	}
	s.prefix = prefix
}

// reportTitlePrefix holds the opening bracketed word against the repository the draft points at. That
// position is the repository slot, so whatever stands there has to be what `repo-key.sh --abbrev`
// prints — nothing can tell `[flaky]` from `[INV]`, and an abbreviation each session invents for
// itself puts two of them on one repository's sessions.
//
// The finding says what was seen and never what was meant. An author who wrote `[flaky]` as prose was
// not filling a slot, and telling them their repository prefix is wrong names a mistake they did not
// make; the line is still refused, and the reason given is the one that is observably true.
func (s *scan) reportTitlePrefix() {
	// No word in the slot, or nothing to weigh it against: a repository this machine cannot name yields
	// no abbreviation to compare, and a sound draft must not be refused for the gate's own blind spot.
	if s.prefix == "" || s.repoAbbrev == "" {
		return
	}
	// The abbreviation in hand belongs to whatever repository this process was pointed at, which is only
	// the draft's repository once the draft has named it. Run from `alpha` with no second argument over a
	// correct draft about `beta`, the comparison would tell a correct author to write `[alpha]` — the
	// mistake-nobody-made this function's own words are against. `no repository named` refuses that
	// draft anyway, so nothing passes quietly.
	if !s.named {
		return
	}
	if s.prefix != s.repoAbbrev {
		s.flag(fmt.Sprintf("the title opens with [%s], which is the repository slot, and %s abbreviates to %s — use [%s], or move a bracketed word that is not the repository off the start of the line",
			shell.CutBytesMarked(s.prefix, findingNameCap), shell.CutBytesMarked(s.repoPath, pathCap), s.repoAbbrev, s.repoAbbrev))
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
	low := shell.AsciiLower(text)
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
	s.reportTitlePrefix()
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
		s.flag("no repository named in: Where it starts — a pasted prompt carries no working directory, so write " + shell.CutBytesMarked(s.repoPath, pathCap))
	}
}

// resolveBase asks the repository about every hex-shaped token the slot held. One that resolves is
// enough; reporting the first candidate rather than "no commit" tells the human which token was tried.
func (s *scan) resolveBase(git repo.Git, repoDir string) {
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
		// `^{commit}`, so a hex run naming a blob or a tree is left out as a base. Both halves of the
		// condition matter. The port answers an unresolvable revision with the empty string and a nil
		// error, and `err == nil` alone would accept every hex token the draft held.
		if id, err := git.Resolve(repoDir, sha+"^{commit}"); err == nil && id != "" {
			return
		}
	}
	s.flag(fmt.Sprintf("base commit does not resolve in %s: %s", repoDir, s.shas[0]))
}

// dirtyNote is advisory, never a finding: some handoffs deliberately start from a committed base and
// leave the caller's tree alone. The receiver cannot tell the two apart, and the human can. What is
// printed is a file count, which is why it comes off `Status`. That listing is `-uall`, and git's
// default collapses a whole untracked directory into one line.
func dirtyNote(git repo.Git, repoDir string) string {
	entries, err := git.Status(repoDir)
	if err != nil || len(entries) == 0 {
		return ""
	}
	return fmt.Sprintf("note: %s has %d uncommitted file(s) — work not in the base commit does not travel", repoDir, len(entries))
}

// isPlaceholder is a title half still holding its template `<…>`, anchored at both ends. A real half
// may carry an angle bracket — "Cut the run to <10 minutes" is a sound work half and "[<10min]" a
// plausible prefix — so an unanchored test sends the agent off to reword lines that were never wrong.
// An empty half is unwritten in the same way a placeholder is.
func isPlaceholder(half string) bool {
	return half == "" || (strings.HasPrefix(half, "<") && strings.HasSuffix(half, ">"))
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

// readFile is the whole read: a draft is a prompt somebody wrote, so nothing here streams or bounds
// it, and a directory or an unreadable path comes back as the error the caller turns into a refusal.
func readFile(path string) (string, error) {
	body, err := os.ReadFile(path)
	return string(body), err
}

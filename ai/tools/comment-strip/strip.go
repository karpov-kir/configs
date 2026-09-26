// Removes a file's comment blocks before a writer reads the code, so the block it then writes is the
// code's alone. It runs without a model: the judge's Apply with every offered unit gone, plus a
// record of what went.
//
//	usage: comment-strip.sh --facts=<dir> [--archive=<dir>] <path>
//
// The file is rewritten in place. Each removed block is written to `<dir>/<n>.facts` under the site
// it sat on, which the writer opens when it asks whether a note is owed. That site is the first code
// line after the block AS THE STRIPPED FILE NUMBERS IT, the file the writer reads. Stdout lists the
// same sites. Exit 1 removed something, 0 removed none, 2 did not run.
package commentstrip

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	readerjudge "configs/ai/tools/reader-judge"
	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
	voicecheck "configs/ai/tools/voice-check"
)

const factsOption = "--facts="
const archiveOption = "--archive="

// The grammar. It carries the stub's name where argv[0] would carry the binary's. A caller then
// reads a usage line they can retype. A refusal states it: an argument this tool refuses comes from
// a caller who needs the form, and the refusal alone gives them half of it.
const usage = "usage: comment-strip.sh --facts=<dir> [--archive=<dir>] [--lines=<n,...>] <path>\n" +
	"       comment-strip.sh --archive=<dir> --contradict=<run> <path> <claim> <review sentence>"

const (
	exitClean     = 0
	exitCut       = 1
	exitDidNotRun = 2
)

// FactsRequested says whether the arguments open with the facts directory this tool requires.
func FactsRequested(args []string) bool {
	return len(args) > 0 && strings.HasPrefix(args[0], factsOption)
}

// directive is a comment the toolchain reads: a lint suppression, a compiler pragma, a build tag, a
// coverage marker or an interpreter line. Its text instructs a program, so a block holding one stays
// whole and stderr names it. Its removal changes what the code does.
var directive = regexp.MustCompile(`^(eslint-|@ts-|prettier-|istanbul |biome-|tslint:|noqa|pylint:|type: |nolint|go:|\+build|#!|/// <reference|@jsx|c8 |v8 |webpack|@vitest-|@jest-|jscpd:)`)

// These are the lines the checks in ai/tools/ read out of a comment. The list of directives holds
// what a language's toolchain reads, and these are the same thing one layer in. The header spellings
// cover the block opening the file, since eco-check's header scans read no further. A region marker
// has a reader wherever it sits, so the strip keeps it there.
var (
	headerInput = regexp.MustCompile(`^(usage:|untested:)`)
	suiteInput  = regexp.MustCompile(`[A-Za-z0-9_.-]+-test\.sh`)
	regionInput = regexp.MustCompile(`^--- (end )?shared:[A-Za-z0-9_-]+ ---$`)
)

func isDirective(raw string) bool {
	return directive.MatchString(commentText(raw))
}

// commentText is a comment line with its marker and the space around it taken off, which is the text
// a reader of that line matches against.
func commentText(raw string) string {
	line := strings.TrimLeft(raw, shell.SpaceBytes)
	for _, marker := range []string{"//", "/*", "*/", "*", "#"} {
		if strings.HasPrefix(line, marker) {
			line = line[len(marker):]
			break
		}
	}
	return strings.TrimRight(strings.TrimLeft(line, shell.SpaceBytes), shell.SpaceBytes)
}

// isToolInput says a check reads this line. `leading` says the line stands in the block opening the
// file, and a header scan reads no further.
func isToolInput(raw string, leading bool) bool {
	text := commentText(raw)
	if regionInput.MatchString(text) {
		return true
	}
	return leading && (headerInput.MatchString(text) || suiteInput.MatchString(text))
}

func holdsDirective(lines []string, u readerjudge.Unit) bool {
	leading := opensTheFile(lines, u)
	for at := u.Line; at < u.Line+u.Span && at <= len(lines); at++ {
		if isDirective(lines[at-1]) || isToolInput(lines[at-1], leading) {
			return true
		}
	}
	return false
}

// opensTheFile says a header scan reads this block. Blank lines and an interpreter line may precede
// it, and code may not. A block further into the file carrying the same words is prose.
func opensTheFile(lines []string, u readerjudge.Unit) bool {
	for at := 1; at < u.Line && at <= len(lines); at++ {
		line := strings.TrimSpace(lines[at-1])
		if line == "" || strings.HasPrefix(line, "#!") {
			continue
		}
		return false
	}
	return true
}

// Strip runs the grammar this file's header states. The facts directory must be empty or absent: a
// file already there reads exactly like one this run wrote, and the writer would take another block's
// facts as this one's.
//
// The repository arrives as a parameter for the tree's hyphenated names, and a nil one reads none.
func Strip(self string, args []string, cwd string, git repo.Git, stdout, stderr io.Writer) int {
	refuse := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "%s: %s — the strip did NOT run\n", self, fmt.Sprintf(format, a...))
		fmt.Fprintf(stderr, "%s\n", usage)
		return exitDidNotRun
	}
	if len(args) > 1 && strings.HasPrefix(args[0], archiveOption) && strings.HasPrefix(args[1], contradictOption) {
		return contradict(strings.TrimPrefix(args[0], archiveOption), strings.TrimPrefix(args[1], contradictOption),
			args[2:], cwd, refuse)
	}
	if !FactsRequested(args) {
		return refuse("%s", "--facts=<dir> must come first")
	}
	dir := strings.TrimPrefix(args[0], factsOption)
	if dir == "" {
		return refuse("%s", "--facts needs a directory")
	}
	args = args[1:]
	archive := ""
	if len(args) > 0 && strings.HasPrefix(args[0], archiveOption) {
		archive = strings.TrimPrefix(args[0], archiveOption)
		if archive == "" {
			return refuse("%s", "--archive needs a directory")
		}
		args = args[1:]
	}
	// A code-review finding goes back to the writer at its own site, and the file's other blocks stay.
	// Run 11 re-stripped 36 sites for six findings, since the strip took every block of a file.
	var only map[int]bool
	if len(args) > 0 && strings.HasPrefix(args[0], linesOption) {
		only = map[int]bool{}
		for _, field := range strings.Split(strings.TrimPrefix(args[0], linesOption), ",") {
			at, err := strconv.Atoi(strings.TrimSpace(field))
			if err != nil || at < 1 {
				return refuse("--lines holds %q, which is not a line", shell.Echoable(field))
			}
			only[at] = true
		}
		args = args[1:]
	}
	// Every block in a file the change touches is a site. `--changed` offered only the blocks the diff
	// touched. Run 10 left an older block standing in a file whose other blocks went. Code review then
	// found it wrong, and no lane rewrote it.
	if len(args) > 0 && (args[0] == "--changed" || strings.HasPrefix(args[0], "--changed=")) {
		return refuse("%s", "--changed is gone: every block in a file the change touches is a site")
	}
	if len(args) != 1 {
		return refuse("%s", "the strip takes one path, and only a source file has comment blocks")
	}
	path := args[0]
	readPath := path
	if !filepath.IsAbs(readPath) {
		readPath = filepath.Join(cwd, readPath)
	}
	info, err := os.Stat(readPath)
	if err != nil {
		return refuse("cannot read %s", shell.Echoable(path))
	}
	raw, err := os.ReadFile(readPath)
	if err != nil {
		return refuse("cannot read %s", shell.Echoable(path))
	}
	if archive != "" && !filepath.IsAbs(archive) {
		archive = filepath.Join(cwd, archive)
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(cwd, dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return refuse("cannot create %s", shell.Echoable(dir))
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) > 0 {
		return refuse("%s is not empty, and a facts file already there reads like one this run wrote", shell.Echoable(dir))
	}

	content := string(raw)
	lines := shell.SplitLines(content)
	var units []readerjudge.Unit
	for _, u := range readerjudge.CommentBlocks(lines) {
		if only != nil && !blockAt(lines, u, only) {
			continue
		}
		if holdsDirective(lines, u) {
			fmt.Fprintf(stderr, "%s:%d: a comment the toolchain reads, kept\n", path, u.Line)
			continue
		}
		units = append(units, u)
	}
	var records []archived
	if archive != "" {
		var err error
		if records, err = readArchive(archive, path); err != nil {
			return refuse("%s", err.Error())
		}
	}
	// A file with no block left standing still has sites, where an earlier run removed one and the
	// archive kept its claims. A clean return here is what stranded them.
	if len(units) == 0 && len(records) == 0 {
		return exitClean
	}

	// Sites are numbered in the stripped file. The writer reads that file, and a site naming a line of
	// the old one would point it at the wrong declaration by the height of every block above it.
	removedBefore := 0
	sites := make([]site, 0, len(units))
	for n, u := range units {
		next := u.Line + u.Span
		for next <= len(lines) && strings.TrimSpace(lines[next-1]) == "" {
			next++
		}
		removedBefore += u.Span
		at := next - removedBefore
		if next > len(lines) {
			at = len(lines) - removedBefore
		}
		var record strings.Builder
		for offset := 0; offset < u.Span; offset++ {
			record.WriteString(lines[u.Line-1+offset])
			record.WriteByte('\n')
		}
		decl := ""
		if next <= len(lines) {
			decl = strings.TrimSpace(lines[next-1])
		}
		sites = append(sites, site{line: at, facts: fmt.Sprintf("%d.facts", n+1), record: record.String(), decl: decl})
	}
	gone := make([]int, len(units))
	for i := range units {
		gone[i] = i + 1
	}
	stripped := readerjudge.Apply(lines, units, gone)
	// A file header sits above a blank line, and removing the header leaves that blank as line 1. The
	// writer then opens a file whose first line is empty. The formatter drops it at the gate, which
	// puts a line the change never wrote into the change set. Only blankness this run created goes,
	// and a file that already opened on a blank line keeps it.
	if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
		short := strings.TrimLeft(stripped, "\n")
		// What goes here sits over every site, so each site moves up by as many lines. The loop that
		// numbered them counted the blocks alone, so a site one line high names the declaration before
		// its own.
		for i := range sites {
			sites[i].line = max(sites[i].line-(len(stripped)-len(short)), 1)
		}
		stripped = short
	}
	if !strings.HasSuffix(content, "\n") {
		stripped = strings.TrimSuffix(stripped, "\n")
	}
	// A declaration two sites in one file share names neither of them, so both fall back to their line.
	shared := map[string]bool{}
	{
		count := map[string]int{}
		for _, s := range sites {
			count[s.decl]++
		}
		for decl, n := range count {
			shared[decl] = n > 1
		}
	}

	// A site whose block an earlier run removed stands in the archive alone. The unit loop above walks
	// comment blocks, so an empty site is invisible to it and its claims sit unread. The run that
	// deleted the block decided under the rules of its day. This offers the site again, with the
	// claims and an empty block, so the writer decides it under the rules standing now.
	held := recordSites(records, sites, shared)
	if archive != "" && only == nil {
		lines := shell.SplitLines(stripped)
		height := len(lines)
		offered := map[int]bool{}
		for _, s := range sites {
			offered[s.line] = true
		}
		for _, record := range records {
			if _, found := held[record.name]; found {
				continue
			}
			at := declarationLine(lines, record.decl, record.line, min(max(record.line, 1), max(height, 1)))
			held[record.name] = at
			// Two records reading to one line are one site. earlierFacts, the writer of a site's earlier
			// claims, gathers every record held at a line. A second site there doubled them. Run 8 offered 11
			// lines twice, and the writers answered each duplicate `none`.
			if offered[at] {
				continue
			}
			offered[at] = true
			sites = append(sites, site{line: at, facts: fmt.Sprintf("%d.facts", len(sites)+1), decl: record.decl})
		}
	}

	// A facts file carries its site, so it is written once the site is final. A refusal here leaves the
	// source file as the run read it.
	for _, s := range sites {
		record := fmt.Sprintf("%s:%d\n%s", path, s.line, s.record)
		// Every claim ever made at this site, beside the block standing now. A writer drops a fact by
		// its own rules on one run. The strip reads the block as it stands, so the dropped text lives
		// in that run's facts directory alone and a later run never weighs it.
		if archive != "" {
			record += earlierFacts(records, s.record, s.line, held)
		}
		// A claim code review contradicted comes back with that finding under it. The archive record
		// never holds the finding, and the claim carries it every time it is offered.
		offer := record
		if archive != "" {
			offer += contradictedIn(archive, path, record)
		}
		if err := os.WriteFile(filepath.Join(dir, s.facts), []byte(offer), 0o644); err != nil {
			return refuse("cannot write %s", shell.Echoable(filepath.Join(dir, s.facts)))
		}
		if archive != "" {
			// What is kept is the claims alone, with the site line left off: a later run writes its
			// own site, and the line a block sits on moves between runs.
			_, claims, _ := strings.Cut(record, "\n")
			if err := keepForLater(archive, path, s.line, s.decl, claims); err != nil {
				return refuse("%s", err.Error())
			}
		}
	}
	if err := os.WriteFile(readPath, []byte(stripped), info.Mode().Perm()); err != nil {
		return refuse("cannot write %s", shell.Echoable(path))
	}
	// The writer's audit classifies a noun phrase as the code's word by looking it up here, so the
	// list sits beside the facts. Absent, every noun audits as none of the three and the writer
	// rewrites until it declines the site.
	if err := os.WriteFile(filepath.Join(dir, identifiersFile),
		[]byte(strings.Join(append(identifierWords(lines), treeNames(cwd, git)...), "\n")+"\n"), 0o644); err != nil {
		return refuse("cannot write %s", shell.Echoable(filepath.Join(dir, identifiersFile)))
	}
	for _, s := range sites {
		fmt.Fprintf(stdout, "%s:%d %s\n", path, s.line, s.facts)
	}
	return exitCut
}

// identifiersFile is what the writer's audit reads to tell the code's own words from English.
const identifiersFile = "identifiers.txt"

var identifierToken = regexp.MustCompile(`[A-Za-z_$][\w$]*`)
var camelHump = regexp.MustCompile(`([a-z0-9])([A-Z])`)

// identifierWords is every word a file's identifiers spell, split at the camel humps, lowercased and
// deduplicated. A reader meets `entryType` in prose as "entry type", so the audit needs the humps
// beside the whole. Comment lines stay out, since a word taken from them would audit as the code's
// own and the comments are what the writer replaces.
func identifierWords(lines []string) []string {
	inComment := map[int]bool{}
	for _, u := range readerjudge.CommentBlocks(lines) {
		for offset := 0; offset < u.Span; offset++ {
			inComment[u.Line+offset] = true
		}
	}
	seen := map[string]bool{}
	for at, line := range lines {
		if inComment[at+1] {
			continue
		}
		for _, token := range identifierToken.FindAllString(line, -1) {
			// Both spellings. The list held the lowercased form alone, so a writer looking a name up as
			// prose spells it missed one the file imports.
			seen[token] = true
			seen[strings.ToLower(token)] = true
			for _, hump := range strings.Fields(camelHump.ReplaceAllString(token, "$1 $2")) {
				seen[hump] = true
				seen[strings.ToLower(hump)] = true
			}
		}
	}
	words := make([]string, 0, len(seen))
	for word := range seen {
		words = append(words, word)
	}
	sort.Strings(words)
	return words
}

// archiveName is where a site's history lives: one file per file and line, under the archive a caller
// keys by the change set's base revision.
func archiveName(path string, line int) string {
	safe := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' {
			return '_'
		}
		return r
	}, path)
	return fmt.Sprintf("%s@%d.facts", safe, line)
}

// archived is one record an earlier run left: the site it was taken from, and the claims made there.
// site is one place a block stood. Its line is a line of the stripped file, which is the file the
// writer reads. A line of the file as it arrived would name a different declaration, off by the
// height of every block the strip removed before it.
type site struct {
	line   int
	facts  string
	record string
	// The declaration this site sits on, which is how a later run finds the site again once an edit
	// over it has moved its line.
	decl string
}

type archived struct {
	name   string
	line   int
	decl   string
	claims string
}

// declMarker names the declaration a record's site sat on. A block that has since moved is read as
// the same site by it. A record written before this line existed carries no declaration, and matches
// on its line alone.
const declMarker = "# the site's declaration:"

// readArchive is every record an earlier run left for this file, newest line last.
func readArchive(archive, path string) ([]archived, error) {
	entries, err := os.ReadDir(archive)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read the archive at %s", shell.Echoable(archive))
	}
	head := strings.TrimSuffix(archiveName(path, 0), "@0.facts") + "@"
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), head) {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	var out []archived
	for _, name := range names {
		body, err := os.ReadFile(filepath.Join(archive, name))
		if err != nil {
			return nil, fmt.Errorf("cannot read %s", shell.Echoable(filepath.Join(archive, name)))
		}
		line, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, head), ".facts"))
		if err != nil {
			continue
		}
		record := archived{name: name, line: line, claims: string(body)}
		if rest, found := strings.CutPrefix(record.claims, declMarker); found {
			head, claims, _ := strings.Cut(rest, "\n")
			record.decl, record.claims = strings.TrimSpace(head), claims
		}
		out = append(out, record)
	}
	return out, nil
}

// recordSites reads each archived record to the line of the site it came from. The declaration
// decides first, and an edit over a site moves the site and leaves the declaration alone. Where no
// declaration matches, the record is read by its line, and the site there is the same one renamed.
// A read by the line alone handed every site in a file the whole file's history.
func recordSites(records []archived, sites []site, shared map[string]bool) map[string]int {
	at := map[string]int{}
	for _, record := range records {
		if record.decl == "" {
			continue
		}
		for _, s := range sites {
			if s.decl != "" && !shared[s.decl] && record.decl == s.decl {
				at[record.name] = s.line
			}
		}
	}
	for _, record := range records {
		if _, found := at[record.name]; found {
			continue
		}
		for _, s := range sites {
			if record.line == s.line {
				at[record.name] = s.line
			}
		}
	}
	return at
}

// declarationLine is where a record's declaration stands in the file now. A record keeping no
// declaration gives up `fallback`, its recorded line. Where the file holds the declaration nowhere, it
// gives up fileLevel, the site line meaning none. The recorded line of a renamed declaration is a
// brace, a blank or another declaration by then, and run 8 put 30 records there.
func declarationLine(lines []string, decl string, recorded, fallback int) int {
	if decl == "" {
		return fallback
	}
	at := 0
	for n, line := range lines {
		if strings.TrimSpace(line) != decl {
			continue
		}
		if at == 0 || abs(n+1-recorded) < abs(at-recorded) {
			at = n + 1
		}
	}
	if at == 0 {
		return fileLevel
	}
	return at
}

// fileLevel is the site line of a claim whose declaration left the file. The writer places such a
// claim anywhere in the file or declines it.
const fileLevel = 0

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// earlierFacts is every claim an earlier run recorded at this site, with the block standing now left
// out. A block byte-identical to one already held is dropped. A site stripped twice with one block
// between hands the writer that block once.
func earlierFacts(records []archived, standing string, line int, at map[string]int) string {
	seen := map[string]bool{strings.TrimSpace(standing): true}
	var out strings.Builder
	for _, record := range records {
		if at[record.name] != line {
			continue
		}
		// An archived record holds one claim block per section. A site stripped three times hands over
		// three claims, each on its own.
		for _, block := range strings.Split(record.claims, earlierMarker) {
			block = strings.TrimSpace(block)
			if block == "" || seen[block] {
				continue
			}
			seen[block] = true
			fmt.Fprintf(&out, "\n%s\n%s\n", earlierMarker, block)
		}
	}
	return out.String()
}

// earlierMarker tells the writer which claims came from a run before this one. Question 3 weighs
// them the way it weighs a standing claim, and a reader of the facts file can see which of them the
// block standing now leaves out.
const earlierMarker = "# claimed at this site by an earlier run:"

// keepForLater records this run's facts for the runs after it, under the declaration its site sits
// on. A later run finds the site by that declaration once an edit has moved its line.
func keepForLater(archive, path string, line int, decl, record string) error {
	if err := os.MkdirAll(archive, 0o755); err != nil {
		return fmt.Errorf("cannot create the archive at %s", shell.Echoable(archive))
	}
	if decl != "" {
		record = declMarker + " " + decl + "\n" + record
	}
	name := filepath.Join(archive, archiveName(path, line))
	if err := os.WriteFile(name, []byte(record), 0o644); err != nil {
		return fmt.Errorf("cannot write %s", shell.Echoable(name))
	}
	return nil
}

// treeNames is the hyphenated names the repository spells in its paths and its code, which the
// writer's audit reads as the code's own words beside the file's identifiers. A repository kept a list
// of them by hand once, and on 2026-09-23 the list went. The tree is their source now.
func treeNames(cwd string, git repo.Git) []string {
	if git == nil {
		return nil
	}
	root, err := git.TopLevel(cwd)
	if err != nil || root == "" {
		return nil
	}
	home, _ := os.LookupEnv("XDG_CACHE_HOME")
	if home == "" {
		if user, ok := os.LookupEnv("HOME"); ok && user != "" {
			home = filepath.Join(user, ".cache")
		}
	}
	names := voicecheck.DerivedNames(root, git, home)
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

const linesOption = "--lines="
const contradictOption = "--contradict="

// blockAt says the block covers one of the lines, or sits on one of them as its declaration. A review
// names either the comment's own lines or the declaration under it.
func blockAt(lines []string, u readerjudge.Unit, only map[int]bool) bool {
	for at := u.Line; at < u.Line+u.Span; at++ {
		if only[at] {
			return true
		}
	}
	next := u.Line + u.Span
	for next <= len(lines) && strings.TrimSpace(lines[next-1]) == "" {
		next++
	}
	return only[next]
}

// contradictedName is the file under the archive holding what code review contradicted in one source
// file, one claim a line.
func contradictedName(archive, path string) string {
	return filepath.Join(archive, strings.TrimSuffix(archiveName(path, 0), "@0.facts")+".contradicted")
}

// contradict records a claim code review found false, with the run and the review's sentence. Run 11
// wrote again a claim runs 9 and 10 had found false, because the strip offered it from the archive with
// no line saying a review had read it.
func contradict(archive, run string, args []string, cwd string, refuse func(string, ...any) int) int {
	if archive == "" || run == "" || len(args) != 3 {
		return refuse("%s", "--contradict=<run> takes the path, the claim and the review's sentence")
	}
	if !filepath.IsAbs(archive) {
		archive = filepath.Join(cwd, archive)
	}
	if err := os.MkdirAll(archive, 0o755); err != nil {
		return refuse("cannot create the archive at %s", shell.Echoable(archive))
	}
	clean := func(text string) string { return strings.Join(strings.Fields(text), " ") }
	name := contradictedName(archive, args[0])
	file, err := os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return refuse("cannot write %s", shell.Echoable(name))
	}
	defer file.Close()
	if _, err := fmt.Fprintf(file, "%s\t%s\t%s\n", clean(args[1]), clean(run), clean(args[2])); err != nil {
		return refuse("cannot write %s", shell.Echoable(name))
	}
	return exitClean
}

// normalClaim is a claim as the match reads it: lower-case words, with markers and backticks gone.
func normalClaim(text string) string {
	var words []string
	for _, line := range strings.Split(text, "\n") {
		words = append(words, strings.Fields(strings.ToLower(commentText(line)))...)
	}
	return strings.ReplaceAll(strings.Join(words, " "), "`", "")
}

// contradictedIn is a `contradicted:` line for every recorded claim the offered facts hold.
func contradictedIn(archive, path, offered string) string {
	body, err := os.ReadFile(contradictedName(archive, path))
	if err != nil {
		return ""
	}
	held := normalClaim(offered)
	var out strings.Builder
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 || fields[0] == "" {
			continue
		}
		if strings.Contains(held, normalClaim(fields[0])) {
			fmt.Fprintf(&out, "\ncontradicted: %s %s (the claim: %s)\n", fields[1], fields[2], fields[0])
		}
	}
	return out.String()
}

package voicecheck

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"configs/ai/tools/diffscan"
	"configs/ai/tools/repo"
)

// A carrier is code that reads or enforces a fact. Run 10 landed six blocks as string constants on a
// field no code read, and the facts lost their reader. `--carriers=<facts dir>` reads the code a
// change set adds against the blocks the strip archived, and reports four landings that are no carrier.
const (
	checkCarrierQuotes   = "carrier-quotes-the-block"
	checkCarrierSentence = "carrier-sentence-name"
	checkCarrierUnread   = "carrier-unread"
	checkCarrierTest     = "carrier-is-a-test"
)

// reTestTitle is a test's title call, with the title in its second group. Run 16 routed some 30 claims
// to a test's name, and eight were a why or an ordering the reader of the site then lacked.
var reTestTitle = regexp.MustCompile(`\b(?:it|test|describe)(?:\.\w+)?\s*\(\s*(['"\x60])(.*?)['"\x60]`)

// minQuotedRunWords is how many words in a row a string shares with a block before it is the block's
// prose moved into a value.
const minQuotedRunWords = 6

// A declared name longer than this is a sentence.
const maxCarrierNameWords = 5

var reCarrierWord = regexp.MustCompile(`[a-z0-9]+`)

// wordRuns lists the text's word runs, lower-cased and in order, each as long as minQuotedRunWords, the
// run length.
func wordRuns(text string) []string {
	words := reCarrierWord.FindAllString(strings.ToLower(text), -1)
	var out []string
	for i := 0; i+minQuotedRunWords <= len(words); i++ {
		out = append(out, strings.Join(words[i:i+minQuotedRunWords], " "))
	}
	return out
}

// reMessageSite is a string a human reads when a rule fires: a lint rule's message or an error's.
var reMessageSite = regexp.MustCompile(`\bmessage\s*:|\bnew\s+\w*(Error|Exception)\s*\(`)

// reMessageOpens is a message site whose text starts on the next line: a key or a call left open. Run
// 11 flagged a lint rule's message and a skip message written that way.
var reMessageOpens = regexp.MustCompile(`(\bmessage\s*:|\bnew\s+\w*(Error|Exception)\s*\()\s*$`)

// reProse is a string a person reads: it holds a space. A token value, such as an enum's published
// name, holds none, and an archived block paraphrasing that token is no carrier moved into it.
var reProse = regexp.MustCompile(`\s`)

// reValueName is a constant or a variable declared on the line.
var reValueName = regexp.MustCompile(`\b(?:const|let|var)\s+([A-Za-z_$][\w$]*)`)

// reTypeMember is a member of an interface or a type: a name, a type, and a semicolon to end it. An
// object literal's entry ends on a comma, and it passes a value to whatever reads it.
var reTypeMember = regexp.MustCompile(`^\s*(?:readonly\s+)?([A-Za-z_$][\w$]*)\??\s*:[^,]*;\s*$`)

// nameWords counts the words a name spells, split at humps and underscores. A digit is no word.
func nameWords(name string) int {
	count := 0
	for _, part := range strings.FieldsFunc(camelHump.ReplaceAllString(name, "$1 $2"), func(r rune) bool {
		return r == ' ' || r == '_' || r == '$'
	}) {
		if strings.ContainsAny(strings.ToLower(part), "abcdefghijklmnopqrstuvwxyz") {
			count++
		}
	}
	return count
}

// treeReader finds the lines of the tree that spell each name as a whole word.
type treeReader func(names []string) (map[string][]string, error)

// CarrierFindings reads the added lines of one change set against the archived blocks. A unit test's
// strings are fixtures, and only its titles are read: a test named for an archived fact took the
// fact from the site.
func CarrierFindings(blocks []string, added *addedLines, tree treeReader, standing map[string]bool) ([]Finding, error) {
	archived := map[string]bool{}
	for _, text := range blocks {
		for _, run := range wordRuns(text) {
			archived[run] = true
		}
	}
	var found []Finding
	declaredAt := map[string]Finding{}
	var unreadCandidates []string
	for _, file := range added.order {
		if IsTestFile(file) {
			found = append(found, testTitleFindings(file, added.byFile[file], archived, standing)...)
			continue
		}
		inMessage := false
		for _, line := range added.byFile[file] {
			code := strings.TrimLeft(line.text, " \t")
			if isComment(code) {
				continue
			}
			message := inMessage || reMessageSite.MatchString(line.text)
			inMessage = reMessageOpens.MatchString(line.text) || (inMessage && strings.HasSuffix(strings.TrimSpace(line.text), "+"))
			if !message {
				found = append(found, quotedBlockFindings(file, line, archived)...)
			}
			found = append(found, sentenceNameFindings(file, line)...)
			for _, name := range lineDeclarations(line.text) {
				if _, seen := declaredAt[name]; !seen {
					declaredAt[name] = Finding{File: file, Line: line.at, Check: checkCarrierUnread, Text: name}
					unreadCandidates = append(unreadCandidates, name)
				}
			}
		}
	}
	if len(unreadCandidates) > 0 {
		spelled, err := tree(unreadCandidates)
		if err != nil {
			return nil, err
		}
		for _, name := range unreadCandidates {
			if !readsAnywhere(name, spelled[name]) {
				found = append(found, declaredAt[name])
			}
		}
	}
	return found, nil
}

// archivedRun is the first word run the text shares with an archived block, or "" where it shares none.
func archivedRun(text string, archived map[string]bool) string {
	for _, run := range wordRuns(text) {
		if archived[run] {
			return run
		}
	}
	return ""
}

// quotedBlockFindings reports each prose string literal on the line that shares a word run with an
// archived block.
func quotedBlockFindings(file string, line addedLine, archived map[string]bool) []Finding {
	var found []Finding
	for _, literal := range reStringLiteral.FindAllString(line.text, -1) {
		if run := archivedRun(literal, archived); run != "" && reProse.MatchString(literal) {
			found = append(found, Finding{File: file, Line: line.at, Check: checkCarrierQuotes, Text: run})
		}
	}
	return found
}

// testTitleFindings reports a test title that quotes an archived block no comment in the tree holds any
// more. The fault is the block dropped in favour of the test. A title that repeats a block still
// standing is no fault: run 18 reported 14 titles whose blocks the change still carried.
func testTitleFindings(file string, lines []addedLine, archived, standing map[string]bool) []Finding {
	var found []Finding
	for _, line := range lines {
		for _, m := range reTestTitle.FindAllStringSubmatch(line.text, -1) {
			if run := archivedRun(m[2], archived); run != "" && !standing[run] {
				found = append(found, Finding{File: file, Line: line.at, Check: checkCarrierTest, Text: run})
			}
		}
	}
	return found
}

// sentenceNameFindings reports each value the line declares under a name of more words than a carrier
// name holds. A function or a type named at length carries no fact moved out of a block, and run 11
// flagged a six-word function name.
func sentenceNameFindings(file string, line addedLine) []Finding {
	var out []Finding
	for _, m := range reValueName.FindAllStringSubmatch(line.text, -1) {
		if nameWords(m[1]) > maxCarrierNameWords {
			out = append(out, Finding{File: file, Line: line.at, Check: checkCarrierSentence, Text: m[1]})
		}
	}
	return out
}

// lineDeclarations is each constant, variable or type member the line declares.
func lineDeclarations(text string) []string {
	var out []string
	for _, re := range []*regexp.Regexp{reValueName, reTypeMember} {
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			out = append(out, m[1])
		}
	}
	return out
}

// readsAnywhere says one of the lines reads the name, and does more than declare or assign it.
func readsAnywhere(name string, lines []string) bool {
	quoted := regexp.QuoteMeta(name)
	writes := regexp.MustCompile(`\b(?:const|let|var)\s+` + quoted + `\b|(?:^|[^.\w$])` + quoted +
		`\??\s*:[^:]|(?:^|[^.\w$])` + quoted + `\s*=[^=>]`)
	spells := regexp.MustCompile(`(?:^|[^\w$])` + quoted + `(?:[^\w$]|$)`)
	for _, line := range lines {
		code := strings.TrimLeft(line, " \t")
		if isComment(code) || !spells.MatchString(line) {
			continue
		}
		if !writes.MatchString(line) {
			return true
		}
		// A line can declare one use and read another: `total: total + posting.amount`.
		if len(spells.FindAllStringIndex(line, -1)) > len(writes.FindAllStringIndex(line, -1)) {
			return true
		}
	}
	return false
}

// gitTree reads the tracked files at root for each name, as whole words. A read inside a unit test is
// left out, because the reader of the site sees only the site.
func gitTree(root string) treeReader {
	return func(names []string) (map[string][]string, error) {
		args := []string{"-C", root, "grep", "--full-name", "-w", "-F", "-I"}
		for _, name := range names {
			args = append(args, "-e", name)
		}
		out, err := exec.Command("git", args...).Output()
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
				return map[string][]string{}, nil
			}
			return nil, fmt.Errorf("git grep over the tree failed (%v) — the check did NOT run", err)
		}
		held := map[string][]string{}
		for _, grepped := range strings.Split(string(out), "\n") {
			file, line, _ := strings.Cut(grepped, ":")
			if IsTestFile(file) {
				continue
			}
			for _, name := range names {
				if strings.Contains(line, name) {
					held[name] = append(held[name], line)
				}
			}
		}
		return held, nil
	}
}

// readArchivedBlocks reads every facts file under dir. The strip writes one per site, and each holds
// the blocks that stood there.
func readArchivedBlocks(dir string) ([]string, error) {
	var blocks []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".facts") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		blocks = append(blocks, string(body))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("the facts directory %s could not be read (%v) — the check did NOT run", dir, err)
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("%s holds no .facts file, so no block can be matched — the check did NOT run", dir)
	}
	return blocks, nil
}

// carriers is the `--carriers=<dir>` mode. It reads the change set the revisions name against the
// blocks the strip archived.
func carriers(out console, dir string, args []string, cwd string, git repo.Git, cfg Config) int {
	blocks, err := readArchivedBlocks(dir)
	if err != nil {
		return out.refuse(err)
	}
	if err := diffscan.RefuseNonRevisions(git, args, cwd); err != nil {
		return out.refuseArguments(err)
	}
	diff, err := diffscan.Diff(git, cwd, args)
	if err != nil {
		return out.refuse(err)
	}
	s := scanner{profile: ProfileComment, notice: func(line string) { out.note("%s", line) }}
	added := newAddedLines()
	if err := s.readDiff(added, diff); err != nil {
		return out.refuse(err)
	}
	root := cwd
	if top, err := git.TopLevel(cwd); err == nil && top != "" {
		root = top
	}
	standing, err := standingRuns(root)
	if err != nil {
		return out.refuse(err)
	}
	found, err := CarrierFindings(blocks, added, gitTree(root), standing)
	if err != nil {
		return out.refuse(err)
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].File != found[j].File {
			return found[i].File < found[j].File
		}
		return found[i].Line < found[j].Line
	})
	for _, f := range found {
		fmt.Fprintln(out.stdout, f.String())
	}
	out.note("carriers: %d finding(s) over %d file(s) against %d archived block(s).",
		len(found), len(added.order), len(blocks))
	if len(found) > 0 {
		out.note("each finding is a landing that carries nothing: refuse the `carried by` lines of its file, so the refactor lane lands a real carrier or returns `stays:`. A carrier-is-a-test finding sits in the test: refuse the `carried by` line of the block whose words its title quotes, and the claim stays at its site.")
		return exitFound
	}
	return exitClean
}

// reTreeComment is a tracked line that opens on a comment marker, as git grep prints it with its path.
var reTreeComment = regexp.MustCompile(`^([^:]+):(\d+):\s*(//+|/\*+|\*+/?|#)\s?(.*)$`)

// standingRuns is every word run of the comment blocks the tree at root holds now. A block is the
// consecutive comment lines of one file.
func standingRuns(root string) (map[string]bool, error) {
	out, err := exec.Command("git", "-C", root, "grep", "-n", "-I", "--full-name", "-E", `^\s*(//|/\*|\*|#)`).Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("git grep over the tree's comments failed (%v) — the check did NOT run", err)
	}
	runs := map[string]bool{}
	var block []string
	lastFile, lastLine := "", 0
	flush := func() {
		for _, run := range wordRuns(strings.Join(block, " ")) {
			runs[run] = true
		}
		block = nil
	}
	for _, grepped := range strings.Split(string(out), "\n") {
		m := reTreeComment.FindStringSubmatch(grepped)
		if m == nil {
			continue
		}
		at, _ := strconv.Atoi(m[2])
		if m[1] != lastFile || at != lastLine+1 {
			flush()
		}
		block = append(block, m[4])
		lastFile, lastLine = m[1], at
	}
	flush()
	return runs, nil
}

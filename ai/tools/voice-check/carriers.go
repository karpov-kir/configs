package voicecheck

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"configs/ai/tools/diffscan"
	"configs/ai/tools/repo"
)

// A carrier is code that reads or enforces a fact: a type that fails the build, a test, a check, the
// declaration's own name, or a message a human sees when the rule fires. Run 10 landed six blocks as
// string constants on a catalogue field no code reads, each named as a sentence. The refactor lane
// called each one `carried by`, and the fact then had no comment and no reader.
//
// `--carriers=<facts dir>` reads a change set's added code against the blocks the strip archived
// there, and reports the three shapes that carry nothing.
const (
	checkCarrierQuotes   = "carrier-quotes-the-block"
	checkCarrierSentence = "carrier-sentence-name"
	checkCarrierUnread   = "carrier-unread"
)

// carrierRunWords is how many words in a row a string shares with a block before it is the block's
// prose moved into a value.
const carrierRunWords = 6

// carrierNameWords is the most words a declared name may hold. Past it the name is a sentence.
const carrierNameWords = 5

var reCarrierWord = regexp.MustCompile(`[a-z0-9]+`)

// wordRuns is every run of carrierRunWords words the text holds, lower-cased, in the text's order.
func wordRuns(text string) []string {
	words := reCarrierWord.FindAllString(strings.ToLower(text), -1)
	var out []string
	for i := 0; i+carrierRunWords <= len(words); i++ {
		out = append(out, strings.Join(words[i:i+carrierRunWords], " "))
	}
	return out
}

// reMessageSite is a string a human reads when a rule fires: a lint rule's message or an error's.
var reMessageSite = regexp.MustCompile(`\bmessage\s*:|\bnew\s+\w*Error\s*\(`)

// reValueName is a constant or a variable declared on the line.
var reValueName = regexp.MustCompile(`\b(?:const|let|var)\s+([A-Za-z_$][\w$]*)`)

// reTypeMember is a member of an interface or a type: a name, a type, and a semicolon to end it. An
// object literal's entry ends on a comma, and it passes a value to whatever reads it.
var reTypeMember = regexp.MustCompile(`^\s*(?:readonly\s+)?([A-Za-z_$][\w$]*)\??\s*:[^,]*;\s*$`)

// reNamedDeclaration is any other name the line declares.
var reNamedDeclaration = regexp.MustCompile(`\b(?:function|class|enum|interface|type)\s+([A-Za-z_$][\w$]*)`)

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
// file is left out: its strings are fixtures, and a test named for the fact is a carrier.
func CarrierFindings(blocks []string, added *addedLines, tree treeReader) ([]Finding, error) {
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
		if isTestFile(file) {
			continue
		}
		for _, line := range added.byFile[file] {
			code := strings.TrimLeft(line.text, " \t")
			if isComment(code) {
				continue
			}
			if !reMessageSite.MatchString(line.text) {
				for _, literal := range reStringLiteral.FindAllString(line.text, -1) {
					for _, run := range wordRuns(literal) {
						if archived[run] {
							found = append(found, Finding{File: file, Line: line.at, Check: checkCarrierQuotes, Text: run})
							break
						}
					}
				}
			}
			var names []string
			for _, re := range []*regexp.Regexp{reValueName, reTypeMember, reNamedDeclaration} {
				for _, m := range re.FindAllStringSubmatch(line.text, -1) {
					names = append(names, m[1])
				}
			}
			for _, name := range names {
				if nameWords(name) > carrierNameWords {
					found = append(found, Finding{File: file, Line: line.at, Check: checkCarrierSentence, Text: name})
				}
			}
			for _, re := range []*regexp.Regexp{reValueName, reTypeMember} {
				for _, m := range re.FindAllStringSubmatch(line.text, -1) {
					if _, seen := declaredAt[m[1]]; !seen {
						declaredAt[m[1]] = Finding{File: file, Line: line.at, Check: checkCarrierUnread, Text: m[1]}
						unreadCandidates = append(unreadCandidates, m[1])
					}
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

// gitTree reads the tracked files at root for each name, as whole words.
func gitTree(root string) treeReader {
	return func(names []string) (map[string][]string, error) {
		args := []string{"-C", root, "grep", "-h", "-w", "-F", "-I"}
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
		for _, line := range strings.Split(string(out), "\n") {
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

// carriers is the `--carriers=<dir>` mode: the change set named by the revisions, read against the
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
	found, err := CarrierFindings(blocks, added, gitTree(root))
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
		out.note("each finding is a `carried by` that carries nothing: the block goes back to the writer.")
		return exitFound
	}
	return exitClean
}

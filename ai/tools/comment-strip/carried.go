package commentstrip

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	readerjudge "configs/ai/tools/reader-judge"
	"configs/ai/tools/shell"
	voicecheck "configs/ai/tools/voice-check"
)

// The archive records a block the refactor lane carried into code, and stops offering its site while
// the carrier stands. Runs 14 and 16 kept no record of a carried verdict. The next strip offered the
// carried claim again, and the writers wrote it back as a fact about the world.

// carried is one block the refactor lane carried, as the archive keeps it.
type carried struct {
	Run     string `json:"run"`
	Rules   string `json:"rules"`
	Decl    string `json:"decl"`
	ID      string `json:"id"`
	Carrier string `json:"carrier"`
}

// carriedName is the file under the archive holding the blocks carried out of one source file.
func carriedName(archive, path string) string {
	return filepath.Join(archive, strings.TrimSuffix(archiveName(path, 0), "@0.facts")+".carried")
}

func readCarried(archive, path string) []carried {
	body, err := os.ReadFile(carriedName(archive, path))
	if err != nil {
		return nil
	}
	var held []carried
	if json.Unmarshal(body, &held) != nil {
		return nil
	}
	return held
}

// Carry records that the refactor lane carried the block covering or sitting on line `at` of `lines`,
// the file as the writers left it, into `carrier`.
func Carry(archive, run, path string, lines []string, at int, carrier string) error {
	rules := rulesSum()
	if rules == "" {
		return fmt.Errorf("%s", "cannot read the rules under ~/.kk-flavor, and a carried block keeps the rules it was carried under")
	}
	if namesATest(carrier) {
		return fmt.Errorf("%s names a test, and a test is never a carrier: the claim is shown by the body or it stays at the site",
			shell.Echoable(carrier))
	}
	for _, u := range readerjudge.CommentBlocks(lines) {
		if !blockAt(lines, u, map[int]bool{at: true}) {
			continue
		}
		decl := declarationUnder(lines, u)
		if decl > len(lines) {
			return fmt.Errorf("the block on line %d of %s sits on no code", u.Line, shell.Echoable(path))
		}
		entry := carried{Run: run, Rules: rules, Decl: strings.TrimSpace(lines[decl-1]), ID: recordID(blockText(lines, u)),
			Carrier: strings.TrimSpace(carrier)}
		var kept []carried
		for _, c := range readCarried(archive, path) {
			if c.Decl != entry.Decl {
				kept = append(kept, c)
			}
		}
		body, err := json.MarshalIndent(append(kept, entry), "", " ")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(archive, 0o755); err != nil {
			return fmt.Errorf("cannot create the archive at %s", shell.Echoable(archive))
		}
		return os.WriteFile(carriedName(archive, path), append(body, '\n'), 0o644)
	}
	return fmt.Errorf("no comment block covers or sits on line %d of %s", at, shell.Echoable(path))
}

var (
	reCarrierSpan  = regexp.MustCompile("`([^`]+)`")
	reCarrierQuote = regexp.MustCompile(`'([^']{3,})'|"([^"]{3,})"`)
	reCarrierName  = regexp.MustCompile(`\b[A-Za-z_$][\w$]*(?:[A-Z_][\w$]*|\.[A-Za-z_$][\w$]*)\b`)
)

// carrierNames is what a carrier verdict names: its code spans, its quoted test names, and the
// identifiers it spells. The tree is searched for each of them.
func carrierNames(carrier string) []string {
	var names []string
	for _, m := range reCarrierSpan.FindAllStringSubmatch(carrier, -1) {
		names = append(names, m[1])
	}
	rest := reCarrierSpan.ReplaceAllString(carrier, " ")
	for _, m := range reCarrierQuote.FindAllStringSubmatch(rest, -1) {
		names = append(names, m[1]+m[2])
	}
	rest = reCarrierQuote.ReplaceAllString(rest, " ")
	names = append(names, reCarrierName.FindAllString(rest, -1)...)
	return names
}

// reTestWord is a word naming a test or its evidence, such as a spec, a suite, a fixture or a snapshot.
// "Pinned by" and "covered by" count too. reTitleCall is a test's title call.
var (
	reTestWord  = regexp.MustCompile(`(?i)\b(test\w*|specs?|suites?|fixtures?|e2e|golden|snapshots?|assert\w*)\b|\b(pinned|covered)\s+by\b`)
	reTitleCall = regexp.MustCompile(`(?i)\b(it|describe|test)\s*\(`)
)

// reTestVerb is "tests" closing a relative clause, as in "the platform that the branch tests;", with
// the word before it.
var reTestVerb = regexp.MustCompile(`(?i)(\b(?:that|which)\s+(?:(?:the|this|its)\s+)?)(\S+)\s+tests(\s*(?:[;.,)]|$))`)

// verbSubjects are the code that tests something in a message carrier's wording. After any other word
// "tests" is read as a noun, and a carrier naming one is refused, which only reopens its block.
var verbSubjects = map[string]bool{"branch": true, "check": true, "guard": true, "condition": true,
	"code": true, "call": true, "function": true, "lookup": true}

// reBackticked is a name in backticks, read only as a file name, so a `retry-spec` key is no test.
var reBackticked = regexp.MustCompile("`[^`]*`")

// namesATest says the carrier verdict names a test: a title call, a test word outside a backticked
// name, or a unit test's file. It refuses when unsure, and a refusal only reopens its block. A word
// list stays open to the next synonym, so the refactor lane's own rule and the standing check back it.
func namesATest(carrier string) bool {
	if reTitleCall.MatchString(carrier) {
		return true
	}
	prose := reTestVerb.ReplaceAllStringFunc(carrier, func(m string) string {
		parts := reTestVerb.FindStringSubmatch(m)
		if !verbSubjects[strings.ToLower(parts[2])] {
			return m
		}
		return parts[1] + parts[2] + " checks" + parts[3]
	})
	if reTestWord.MatchString(reBackticked.ReplaceAllString(prose, " ")) {
		return true
	}
	for _, token := range strings.FieldsFunc(carrier, func(r rune) bool { return strings.ContainsRune(" `'\"", r) }) {
		if voicecheck.IsTestFile(strings.Trim(token, ".,;:()[]")) {
			return true
		}
	}
	return false
}

// carrierStands says the carrier is code the site shows, and every name it cites still appears in the
// tree at root, outside a unit test's file. Where root is no git work tree, it reads only the file.
func carrierStands(root, file, carrier string) bool {
	names := carrierNames(carrier)
	if len(names) == 0 || namesATest(carrier) {
		return false
	}
	for _, name := range names {
		if spelledOutsideTests(root, name) {
			continue
		}
		body, err := os.ReadFile(file)
		if err != nil || voicecheck.IsTestFile(file) || !strings.Contains(string(body), name) {
			return false
		}
	}
	return true
}

// spelledOutsideTests says a tracked file at root other than a unit test spells the name.
func spelledOutsideTests(root, name string) bool {
	out, err := exec.Command("git", "-C", root, "grep", "-l", "-F", "--", name).Output()
	if err != nil {
		return false
	}
	for _, path := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if path != "" && !voicecheck.IsTestFile(path) {
			return true
		}
	}
	return false
}

// carriedDecls is every declaration whose block was carried into code that still stands, under the
// rules standing now, with no contradiction against the carried block.
func carriedDecls(archive, path, root, file string, contradiction map[string]bool) map[string]carried {
	out := map[string]carried{}
	rules := rulesSum()
	for _, c := range readCarried(archive, path) {
		if c.Rules == rules && rules != "" && !contradiction[c.ID] && carrierStands(root, file, c.Carrier) {
			out[c.Decl] = c
		}
	}
	return out
}

// CarriedVerdict is a block the refactor lane carried into code that still stands, and the next full
// strip offers no site for.
type CarriedVerdict struct {
	Decl, Run, Carrier string
}

// CarriedVerdicts reads the carried blocks of one file against the tree at root, and only reads. A
// carried block counts as kept where its declaration stands in the file and its carrier stands too.
func CarriedVerdicts(archive, path, root, file string, lines []string) []CarriedVerdict {
	present := map[string]bool{}
	for _, line := range lines {
		present[strings.TrimSpace(line)] = true
	}
	var out []CarriedVerdict
	for decl, c := range carriedDecls(archive, path, root, file, contradictedIDs(archive, path)) {
		if present[decl] {
			out = append(out, CarriedVerdict{Decl: decl, Run: c.Run, Carrier: c.Carrier})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Decl < out[j].Decl })
	return out
}

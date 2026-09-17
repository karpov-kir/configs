package ecoreport_test

// How a fixture is built and read: the file and git primitives the builders in harness_test.go stand
// on, and the text helpers the cases compare output with. Split from the harness for size alone —
// what a case *says* is there; what puts a tree on disk is here.

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"kk-flavor/tools/repo/repotest"
)

// Fixture I/O. The builders fail the case rather than returning an error: a fixture that did not get
// built leaves its assertions passing against a tree they were never given. The queries answer
// instead. A case asks `exists`, `isFile` or `read` a question, and a false or empty answer is its
// result, not a broken fixture.
func (f *fixture) mkdirAll(dir string) {
	f.t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatalf("mkdir %s: %v", dir, err)
	}
}

// Builds the parents too. A ship is a directory now, so most fixture writes land one level inside one
// that does not exist yet, and a case that had to mkdir before every write would say more about the
// layout than about what it is testing.
func (f *fixture) write(path, content string) {
	f.t.Helper()
	f.mkdirAll(filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.t.Fatalf("write %s: %v", path, err)
	}
}

func (f *fixture) appendTo(path, content string) {
	f.t.Helper()
	handle, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		f.t.Fatalf("append %s: %v", path, err)
	}
	defer handle.Close()
	if _, err := handle.WriteString(content); err != nil {
		f.t.Fatalf("append %s: %v", path, err)
	}
}

func (f *fixture) read(path string) string {
	f.t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(content)
}

func (f *fixture) symlink(target, link string) {
	f.t.Helper()
	f.mkdirAll(filepath.Dir(link))
	if err := os.Symlink(target, link); err != nil {
		f.t.Fatalf("symlink %s -> %s: %v", link, target, err)
	}
}

func (f *fixture) chmod(path string, mode os.FileMode) {
	f.t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		f.t.Fatalf("chmod %s: %v", path, err)
	}
}

func (f *fixture) remove(path string) {
	f.t.Helper()
	if err := os.RemoveAll(path); err != nil {
		f.t.Fatalf("remove %s: %v", path, err)
	}
}

func (f *fixture) exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func (f *fixture) isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func (f *fixture) find(root string) []string {
	var found []string
	_ = filepath.Walk(root, func(path string, _ os.FileInfo, err error) error {
		if err == nil {
			found = append(found, path)
		}
		return nil
	})
	return found
}

func (f *fixture) entries(dir string) []string {
	listing, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range listing {
		names = append(names, entry.Name())
	}
	return names
}

// What the repository holds, as a case states it. The fake answers the tool from this table, so a case
// says "the index holds this" rather than running a command to make it so — and nothing forks.

// Put paths in the index and at HEAD, with whatever is on disk at each. What `git add` followed by
// `git commit` gave the old fixtures, which every case using it wanted only as a starting state.
func (f *fixture) track(paths ...string) {
	f.t.Helper()
	committed := map[string]string{}
	for name, body := range f.fake.Revs[repotest.WorkTree] {
		committed[name] = body
	}
	for _, name := range paths {
		body := f.read(f.repo + "/" + name)
		f.fake.Write(name, body)
		committed[name] = body
	}
	f.commitNamed("HEAD", committed)
}

// A commit under a name a case can use, holding the given content, and the object id it resolves to.
//
// Both spellings, because the tool resolves a base-ref to an id and then diffs against THAT: a
// revision the table knows only by name is one it cannot diff, and git answers about either spelling.
func (f *fixture) commitNamed(rev string, files map[string]string) string {
	f.t.Helper()
	f.fake.Commit(rev, files)
	id := f.fake.Refs[rev]
	f.fake.Revs[id] = f.fake.Revs[rev]
	f.fake.Refs[id] = id
	return id
}

// Whether the index holds anything under a path — the question `scratch` and the committed-mode
// assertions ask.
func (f *fixture) isTracked(prefix string) bool {
	for name := range f.fake.Revs[repotest.WorkTree] {
		if name == prefix || strings.HasPrefix(name, prefix+"/") {
			return true
		}
	}
	return false
}

// What the tool staged, in the form `git diff --cached --name-only` used to answer: one path per line,
// root-relative, sorted. Both the pathspecs it named and the files under them, because git expands a
// directory pathspec and every case here reads this as that expansion.
func (f *fixture) staged() string {
	seen := map[string]bool{}
	var names []string
	for _, staged := range append(append([]string{}, f.fake.Added...), f.stagedFiles...) {
		name := strings.TrimPrefix(staged, f.canonicalRepo()+"/")
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return strings.Join(names, "\n")
}

// Say that git ignores these paths, and which file said so. `source` is what `check-ignore -v` names —
// `.gitignore` is the only answer that travels with the repository, and the tool turns on that
// difference.
func (f *fixture) ignore(source string, paths ...string) {
	f.fake.Ignore(source, paths...)
}

// The commit HEAD names. A case that needs the tree to have moved on sets a new one.
func (f *fixture) head() string { return f.fake.Refs["HEAD"] }

// Move HEAD to another commit, holding the content it holds now. An object id resolves to itself,
// which is git's own answer and what a case naming a revision by its id depends on.
func (f *fixture) setHead(commit string) {
	f.t.Helper()
	f.fake.Refs["HEAD"] = commit
	f.fake.Refs[commit] = commit
	f.fake.Revs[commit] = f.fake.Revs["HEAD"]
}

// `grep -qx` — the whole line, never a substring, which is what several cases mean by "prints this".
func containsLine(text, line string) bool {
	for _, candidate := range strings.Split(text, "\n") {
		if candidate == line {
			return true
		}
	}
	return false
}

func countLines(text string, counts func(string) bool) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if counts(line) {
			count++
		}
	}
	return count
}

func countLinesEqual(text, line string) int {
	return countLines(text, func(candidate string) bool { return candidate == line })
}

func countLinesWithPrefix(text, prefix string) int {
	return countLines(text, func(line string) bool { return strings.HasPrefix(line, prefix) })
}

func countNonEmptyLines(text string) int {
	return countLines(text, func(line string) bool { return line != "" })
}

func countLinesEndingWith(text, suffix string) int {
	return countLines(text, func(line string) bool { return strings.HasSuffix(line, suffix) })
}

// The state column of a `list` line — `grep '^<name>[[:space:]]' | cut -f2`.
func stateOf(listing, name string) string {
	for _, line := range strings.Split(listing, "\n") {
		if rest, ok := strings.CutPrefix(line, name); ok && strings.HasPrefix(rest, "\t") {
			return rest[1:]
		}
	}
	return ""
}

// The report split at its frontmatter delimiters, so a case can say which half a line is in. Line 1
// opens the frontmatter and the next `---` closes it, which is the rule the rewrites apply.
func frontmatterAndBody(report string) (frontmatter, body string) {
	lines := strings.Split(report, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", report
	}
	for i, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			return strings.Join(lines[1:i+1], "\n"), strings.Join(lines[i+2:], "\n")
		}
	}
	return strings.Join(lines[1:], "\n"), ""
}

func sortedWords(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	sort.Strings(words)
	return strings.Join(words, " ") + " "
}

func (f *fixture) isSymlink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

// `grep -rlF <needle>` — the files under a directory holding the text, which is how a case asks
// whether content from outside the repo reached .idsd/.
func (f *fixture) filesContaining(dir, needle string) []string {
	var found []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if content, err := os.ReadFile(path); err == nil && strings.Contains(string(content), needle) {
			found = append(found, path)
		}
		return nil
	})
	return found
}

func joinLines(values []string) string { return strings.Join(values, "\n") }

// `sed -i 's/^<prefix>.*/<line>/'` and `grep -v '^<prefix>'` over a fixture file: the two edits the
// template cases break their own copy with.
func (f *fixture) replaceLine(path, prefix, line string) {
	f.t.Helper()
	var kept []string
	for _, existing := range strings.Split(f.read(path), "\n") {
		if strings.HasPrefix(existing, prefix) {
			existing = line
		}
		kept = append(kept, existing)
	}
	f.write(path, strings.Join(kept, "\n"))
}

func (f *fixture) dropLines(path, prefix string) {
	f.t.Helper()
	var kept []string
	for _, existing := range strings.Split(f.read(path), "\n") {
		if !strings.HasPrefix(existing, prefix) {
			kept = append(kept, existing)
		}
	}
	f.write(path, strings.Join(kept, "\n"))
}

func indent(text string) string {
	var out strings.Builder
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		fmt.Fprintf(&out, "          %s\n", line)
	}
	return out.String()
}

// One frontmatter field's value out of a report's text — the assertion counterpart to the tool's own
// fieldValue, which reads a file rather than a string a case already holds.
func fieldFrom(text, field string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, field+":") {
			return strings.TrimSpace(strings.TrimPrefix(line, field+":"))
		}
	}
	return ""
}

type fixtureStageContext struct {
	Version  int    `json:"version"`
	Attempt  string `json:"attempt"`
	Head     string `json:"head"`
	Tree     string `json:"tree"`
	Worktree string `json:"worktree"`
}

type fixtureStageResult struct {
	fixtureStageContext
	Id      string            `json:"id"`
	Stage   string            `json:"stage"`
	Status  string            `json:"status"`
	Outcome string            `json:"outcome"`
	Items   []json.RawMessage `json:"items"`
}

type cleanStageOptions struct {
	dir     string
	stage   string
	outcome string
	intent  string
}

func (f *fixture) cleanStageResultIn(options cleanStageOptions) string {
	dir, stage, outcome, intent := options.dir, options.stage, options.outcome, options.intent
	f.t.Helper()
	f.runReportIn(dir, "result-context", intent)
	if f.status != 0 {
		f.t.Fatalf("read stage context: %s", f.evidence())
	}
	var context fixtureStageContext
	if err := json.Unmarshal([]byte(f.out), &context); err != nil {
		f.t.Fatalf("decode stage context: %v: %s", err, f.out)
	}
	file, err := os.CreateTemp(f.base, "stage-result-*.json")
	if err != nil {
		f.t.Fatal(err)
	}
	result := fixtureStageResult{fixtureStageContext: context, Id: filepath.Base(file.Name()), Stage: stage, Status: "complete", Outcome: outcome, Items: []json.RawMessage{}}
	err = json.NewEncoder(file).Encode(result)
	closeErr := file.Close()
	if err != nil {
		f.t.Fatal(err)
	}
	if closeErr != nil {
		f.t.Fatal(closeErr)
	}
	return file.Name()
}

func (f *fixture) recordCleanStage(stage string, intent ...string) {
	f.t.Helper()
	ship := ""
	if len(intent) != 0 {
		ship = intent[0]
	}
	f.recordCleanStageIn(cleanStageOptions{dir: f.repo, stage: stage, intent: ship})
}

func (f *fixture) recordCleanStageIn(options cleanStageOptions) {
	dir, intent := options.dir, options.intent
	options.outcome = "complete"
	f.t.Helper()
	path := f.cleanStageResultIn(options)
	f.runReportIn(dir, "stage-result", path, intent)
}

func (f *fixture) stageResultsPath(intent string) string {
	f.t.Helper()
	report, err := filepath.EvalSymlinks(f.reportPath(intent))
	if err != nil {
		f.t.Fatal(err)
	}
	return fmt.Sprintf("%s/.git/idsd-stage-results/%x.json", f.repo, sha256.Sum256([]byte(report)))
}

// The tree fingerprint every fixture runs on. The RECIPE belongs to `ai/tools/tree-fingerprint/`,
// which owns its own suite; what the cases here need of it is the one property they all turn on — a
// value that moves when the tree's content moves and stands still otherwise — and the shipped recipe
// pays five git spawns per reading to deliver it.
//
// Ignorable report files are skipped for the same reason git's own ignore rules skip them: a report
// written inside the tree it fingerprints makes every stamp stale on arrival, and assertReportIsIgnored
// is the guard that exists because of it. `.git` is skipped because git's own recipe never reads it.
//
// Forty hex characters, because `gate` prints the value and index_test.go matches it as one.
func (f *fixture) newTreeFingerprint() func(string) (string, error) {
	return func(root string) (string, error) {
		sum := sha1.New()
		err := filepath.Walk(root, func(full string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			name, relErr := filepath.Rel(root, full)
			if relErr != nil {
				return relErr
			}
			// A linked worktree's `.git` is a FILE, and SkipDir on one skips the REST of the directory
			// holding it — which here is the whole worktree, since `.git` sorts first. The tree then
			// fingerprints as empty and every freshness case passes on a reading of nothing.
			if name == ".git" {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if info.IsDir() || fingerprintSkips(name) {
				return nil
			}
			if info.Mode()&os.ModeSymlink != 0 {
				target, linkErr := os.Readlink(full)
				fmt.Fprintf(sum, "%s\x00link\x00%s\x00", name, target)
				return linkErr
			}
			body, readErr := os.ReadFile(full)
			fmt.Fprintf(sum, "%s\x00%04o\x00%s\x00", name, info.Mode().Perm(), body)
			return readErr
		})
		if err != nil {
			return "", err
		}
		return hex.EncodeToString(sum.Sum(nil)), nil
	}
}

// The ship working files `.gitignore` covers, by the same patterns the tool writes — ignoreEntries()
// mirrors ignoreSurface(), so the fixture and the tool cannot disagree about which files a fingerprint
// must not see.
func fingerprintSkips(name string) bool {
	for _, entry := range ignoreEntries() {
		if matched, _ := path.Match(entry, filepath.ToSlash(name)); matched {
			return true
		}
	}
	return false
}

// A linked worktree, built the way git builds one: a directory whose `.git` is a FILE naming a git dir
// under the main repository's `worktrees/`, and a `commondir` in that git dir pointing back at the
// shared store. layout.go reads exactly this shape, so nothing has to run `git worktree add`.
//
// The two properties every case using this turns on come straight from the layout: the worktree's own
// git dir is its own, which is where its identity token is minted, and its common dir is the clone's,
// which is where the one scratch directory lives.
func (f *fixture) newLinkedWorktree(name string) string {
	f.t.Helper()
	linked := f.base + "/" + name
	// Canonical, because the pointer is what layoutGitDir reads and what layoutCommonDir then resolves
	// against: a `/var` spelling here resolves one location and the main tree's `/private/var` another,
	// and two worktrees of one clone would answer two scratch directories while both looked right.
	gitDir := f.canonicalRepo() + "/.git/worktrees/" + name
	f.mkdirAll(gitDir)
	f.write(gitDir+"/commondir", "../..\n")
	f.write(gitDir+"/HEAD", "ref: refs/heads/"+name+"\n")
	f.mkdirAll(linked)
	f.write(linked+"/.git", "gitdir: "+gitDir+"\n")
	// Its own git dir, the clone's common dir, and the same history and ignore answers as the tree it
	// was added from — which is what a real linked worktree answers, and what `repo/exec_test.go` holds
	// git to. Shared maps rather than copies, so a case that tracks or ignores something afterwards is
	// answered the same from both.
	sibling := repotest.New(canonical(linked))
	sibling.Git, sibling.Common = gitDir, f.canonicalRepo()+"/.git"
	sibling.Revs, sibling.Refs, sibling.Sources, sibling.Fail = f.fake.Revs, f.fake.Refs, f.fake.Sources, f.fake.Fail
	f.worktrees[canonical(linked)] = sibling
	// The tracked file the main tree has, so the two trees fingerprint alike — which is the state a
	// freshly added worktree is in and the precondition several cases state for themselves.
	f.write(linked+"/tracked.txt", f.read(f.repo+"/tracked.txt"))
	return linked
}

// `git worktree remove`: the checkout and the private git dir git deletes with it. The token minted in
// that git dir goes too, which is the whole mechanism a recreated worktree reads as a different one.
func (f *fixture) removeLinkedWorktree(name string) {
	f.t.Helper()
	delete(f.worktrees, canonical(f.base+"/"+name))
	f.remove(f.base + "/" + name)
	f.remove(f.repo + "/.git/worktrees/" + name)
}

// `git worktree move`: the checkout changes place and keeps its git dir, so it keeps its identity.
func (f *fixture) moveLinkedWorktree(from, to string) {
	f.t.Helper()
	// Keyed before the rename: canonical() resolves a path that exists, and after the move the old one
	// does not, so the entry would be looked up under a spelling nothing holds.
	was := canonical(from)
	if err := os.Rename(from, to); err != nil {
		f.t.Fatalf("move %s to %s: %v", from, to, err)
	}
	moved := f.worktrees[was]
	delete(f.worktrees, was)
	moved.Root = canonical(to)
	f.worktrees[canonical(to)] = moved
}

// A path as the tool will resolve it: physically, the way layoutRoot answers and the way git's own
// `--show-toplevel` does. A path that does not exist yet keeps whatever it was given.
func canonical(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}

// The report template, written by the fixture rather than read from the installed skill: reading it
// from outside this module is what made a plain `go test` answer `(cached)` over a changed one. Four
// placeholder frontmatter lines and a body: `init` refuses a template missing any of them, and every
// case that drifts one edits this copy. The shipped one is
// `ai/kk-flavor/skills/idsd-qualify/templates/qualify-report-template.md`; nothing here reads it.
const reportTemplate = `---
intent: <NNN-slug or "review: <description>">
reviewed-tree: <hash>
reviewed-worktree: <worktree>
reviewed-stages: <stages>
---

# Decide
`

// A second clone of this repository: another checkout with a git dir of its own. What makes it a
// second CLONE rather than a second worktree is that its git dir is not the first's — which is the
// whole of what a repo key is taken from, and the reason two clones must never share a scratch
// directory.
func (f *fixture) newSecondClone(name string) string {
	f.t.Helper()
	clone := f.base + "/" + name
	f.mkdirAll(clone + "/.git")
	f.write(clone+"/.git/HEAD", "ref: refs/heads/main\n")
	f.write(clone+"/tracked.txt", f.read(f.repo+"/tracked.txt"))
	return clone
}

// What the tree's own .gitignore says about a path, read at the moment of the question — `promote`
// writes that file mid-run and then asks whether the write took effect, so an answer arranged before
// the run could only ever say yes. The bool is whether any rule matched at all.
//
// Only literal glob matching with git's last-match-wins negation is modelled. An implementation of
// gitignore here would be the fake agreeing with itself about a question `repo/exec_test.go` holds
// real git to; a case wanting any other rule — `.git/info/exclude`, a global excludesFile — states the
// answer with f.ignore, and .gitignore is consulted first because that is git's own precedence.
func (f *fixture) gitignoreSourceFor(full string) (source string, matched bool) {
	// Relative to whichever working tree holds it: a committed `.idsd/` is checked out into every linked
	// worktree, and the rules that cover it are the same tracked .gitignore.
	name := strings.TrimPrefix(full, f.canonicalRepo()+"/")
	for root := range f.worktrees {
		if rest, inside := strings.CutPrefix(full, root+"/"); inside {
			name = rest
		}
	}
	negated := false
	for _, line := range strings.Split(f.read(f.repo+"/.gitignore"), "\n") {
		rule := strings.TrimSpace(line)
		if rule == "" || strings.HasPrefix(rule, "#") {
			continue
		}
		if hit, _ := path.Match(strings.TrimPrefix(rule, "!"), name); hit {
			matched, negated = true, strings.HasPrefix(rule, "!")
		}
	}
	switch {
	case negated:
		return "", true
	case matched:
		return ".gitignore", true
	}
	return "", false
}

package commentrun

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	commentstrip "configs/ai/tools/comment-strip"
	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
)

// seed prepares a run. It seeds the archive from each earlier head of the change, and maps the claims
// code review found false to the record ids a trial strip names. Then it strips the change set in the
// tree the command stands in. The tree must sit at the range's head, clean. Each earlier head is stripped in a
// disposable worktree, so every archived record carries its site's declaration.
func seed(r *runner, opts options, _ []string) int {
	runDir, archive, span := opts.one("run-dir"), opts.one("archive"), opts.one("range")
	base, head, found := strings.Cut(span, "..")
	if runDir == "" || archive == "" || !found || base == "" || head == "" || strings.HasPrefix(head, ".") {
		return r.refuse("%s", "seed takes --run-dir=<dir>, --archive=<dir> and --range=<base>..<head>")
	}
	r.absolute(&runDir, &archive)
	top, err := git(r.cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return r.refuse("%s is no git work tree", shell.Echoable(r.cwd))
	}
	headSha, err := git(top, "rev-parse", "--verify", head+"^{commit}")
	if err != nil {
		return r.refuse("%s names no commit", shell.Echoable(head))
	}
	if at, _ := git(top, "rev-parse", "HEAD"); at != headSha {
		return r.refuse("the tree stands at %s and the range's head is %s", at, headSha)
	}
	if dirty, _ := git(top, "status", "--porcelain"); dirty != "" {
		return r.refuse("%s", "the tree has changes of its own, and the strip would read them as the change set's")
	}
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return r.refuse("cannot create %s", shell.Echoable(runDir))
	}

	var heads []string
	if listed := opts.one("heads"); listed != "" {
		heads = strings.Split(listed, ",")
	}
	for _, rev := range heads {
		if code := r.inWorktree(top, runDir, rev, func(tree string) int {
			_, code := r.stripAll(tree, base, rev, filepath.Join(runDir, "seed-facts", short(rev)), archive, nil)
			return code
		}); code != exitClean {
			return code
		}
	}
	if tsv := opts.one("contradictions"); tsv != "" {
		if code := r.contradict(top, runDir, archive, base, headSha, tsv); code != exitClean {
			return code
		}
	}

	files, err := changed(top, base, headSha)
	if err != nil {
		return r.refuse("cannot list the change set: %v", err)
	}
	if err := copyFiles(top, files, filepath.Join(runDir, "pre-strip")); err != nil {
		return r.refuse("%v", err)
	}
	var errs strings.Builder
	sites, code := r.stripAll(top, base, headSha, filepath.Join(runDir, "facts"), archive, &errs)
	if code != exitClean {
		return code
	}
	if err := copyFiles(top, files, filepath.Join(runDir, "post-strip")); err != nil {
		return r.refuse("%v", err)
	}
	kept := strings.Count(errs.String(), commentstrip.KeptLine)
	run := fmt.Sprintf("top=%s\nbase=%s\nhead=%s\narchive=%s\n", top, base, headSha, archive)
	for name, body := range map[string]string{"run.txt": run, "sites.txt": strings.Join(sites, ""), "strip.err": errs.String()} {
		if err := os.WriteFile(filepath.Join(runDir, name), []byte(body), 0o644); err != nil {
			return r.refuse("cannot write %s", shell.Echoable(filepath.Join(runDir, name)))
		}
	}
	for _, s := range sites {
		fmt.Fprint(r.stdout, s)
	}
	fmt.Fprintf(r.stderr, "%s: %d site(s) in %d file(s), %d block(s) kept as an earlier run wrote them\n",
		r.self, len(sites), len(files), kept)
	return exitClean
}

// stripAll strips every file the change set holds at rev, in the tree at dir, one facts directory per
// file. It returns each site the strip printed, as `<site> <facts file>`.
func (r *runner) stripAll(dir, base, rev, factsRoot, archive string, errs *strings.Builder) ([]string, int) {
	files, err := changed(dir, base, rev)
	if err != nil {
		return nil, r.refuse("cannot list the change set at %s: %v", short(rev), err)
	}
	var sites []string
	for _, file := range files {
		facts := filepath.Join(factsRoot, strings.ReplaceAll(file, "/", "_"))
		var out, errOut strings.Builder
		code := commentstrip.Strip("comment-strip.sh", []string{"--facts=" + facts, "--archive=" + archive, file},
			dir, repo.Exec{}, &out, &errOut)
		if errs != nil {
			errs.WriteString(errOut.String())
		}
		if code == exitDidNotRun {
			return nil, r.refuse("the strip refused %s: %s", shell.Echoable(file), shell.Oneline(errOut.String()))
		}
		for _, line := range shell.SplitLines(out.String()) {
			if site, name, found := strings.Cut(line, " "); found {
				sites = append(sites, fmt.Sprintf("%s %s\n", site, filepath.Join(facts, name)))
			}
		}
	}
	return sites, exitClean
}

// inWorktree runs work in a disposable worktree at rev, and removes the worktree whatever work did.
func (r *runner) inWorktree(top, runDir, rev string, work func(tree string) int) int {
	tree := filepath.Join(runDir, "worktrees", short(rev))
	if _, err := git(top, "worktree", "add", "--detach", tree, rev); err != nil {
		return r.refuse("cannot add a worktree at %s: %v", shell.Echoable(rev), err)
	}
	defer func() {
		if _, err := git(top, "worktree", "remove", "--force", tree); err != nil {
			fmt.Fprintf(r.stderr, "%s: the worktree at %s stays: %v\n", r.self, tree, err)
		}
	}()
	return work(tree)
}

// contradict maps each claim code review found false to the record ids a trial strip names, and
// records the contradiction by id. A trial strip at the head runs against a copy of the archive, so the
// ids are read before the live strip. A claim no block holds is recorded by its text. The tsv holds a
// path, the claim, the run that found it and the review's sentence, one claim to a line.
func (r *runner) contradict(top, runDir, archive, base, head, tsv string) int {
	body, err := os.ReadFile(tsv)
	if err != nil {
		return r.refuse("cannot read %s", shell.Echoable(tsv))
	}
	trial := filepath.Join(runDir, "archive-trial")
	if err := os.RemoveAll(trial); err != nil {
		return r.refuse("cannot clear %s", shell.Echoable(trial))
	}
	if err := copyTree(archive, trial); err != nil {
		return r.refuse("%v", err)
	}
	facts := filepath.Join(runDir, "trial-facts")
	if code := r.inWorktree(top, runDir, head, func(tree string) int {
		_, code := r.stripAll(tree, base, head, facts, trial, nil)
		return code
	}); code != exitClean {
		return code
	}
	byID, byText := 0, 0
	recorded := map[string]bool{}
	for n, line := range shell.SplitLines(string(body)) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			return r.refuse("%s line %d holds %d field(s), and a claim takes path, claim, run and sentence",
				shell.Echoable(tsv), n+1, len(fields))
		}
		path, claim, run, sentence := fields[0], fields[1], fields[2], fields[3]
		ids := blocksHolding(filepath.Join(facts, strings.ReplaceAll(path, "/", "_")), claim)
		if len(ids) == 0 {
			ids = []string{claim}
			byText++
		}
		for _, id := range ids {
			if recorded[path+"\x00"+id] {
				continue
			}
			recorded[path+"\x00"+id] = true
			var out, errOut strings.Builder
			if code := commentstrip.Strip("comment-strip.sh", []string{"--archive=" + archive, "--contradict=" + run,
				path, id, sentence}, top, repo.Exec{}, &out, &errOut); code != exitClean {
				return r.refuse("the strip refused a contradiction for %s: %s", shell.Echoable(path), shell.Oneline(errOut.String()))
			}
			if id != claim {
				byID++
			}
		}
	}
	fmt.Fprintf(r.stderr, "%s: %d contradiction(s) recorded by record id, %d by text where no block holds the claim\n",
		r.self, byID, byText)
	return exitClean
}

var reRecordID = regexp.MustCompile(`^# record (r[0-9a-f]{10})$`)

// blocksHolding is the record id of every claim block a facts directory offers whose words hold the
// claim's.
func blocksHolding(dir, claim string) []string {
	want := normal(claim)
	names, _ := filepath.Glob(filepath.Join(dir, "*.facts"))
	sort.Strings(names)
	var ids []string
	for _, name := range names {
		file, err := os.Open(name)
		if err != nil {
			continue
		}
		id, block := "", []string{}
		flush := func() {
			if id != "" && strings.Contains(normal(strings.Join(block, "\n")), want) {
				ids = append(ids, id)
			}
		}
		scan := bufio.NewScanner(file)
		for scan.Scan() {
			line := scan.Text()
			switch m := reRecordID.FindStringSubmatch(line); {
			case m != nil:
				flush()
				id, block = m[1], nil
			case strings.HasPrefix(line, "# claimed at this site by an earlier run:") || strings.HasPrefix(line, "contradicted:"):
				flush()
				id, block = "", nil
			case id != "":
				block = append(block, line)
			}
		}
		flush()
		file.Close()
	}
	return ids
}

var reMarker = regexp.MustCompile(`^\s*(/\*\*?|\*/|\*|//|#)\s?`)

// normal is text as the claim match reads it: lower-case words with comment markers and backticks gone.
func normal(text string) string {
	var words []string
	for _, line := range strings.Split(text, "\n") {
		words = append(words, strings.Fields(strings.ToLower(strings.ReplaceAll(reMarker.ReplaceAllString(line, ""), "*/", "")))...)
	}
	return strings.ReplaceAll(strings.Join(words, " "), "`", "")
}

// changed is every file the change set adds or modifies at rev.
func changed(dir, base, rev string) ([]string, error) {
	out, err := git(dir, "diff", "--name-only", "--diff-filter=AMR", base+"..."+rev)
	if err != nil {
		return nil, err
	}
	return shell.SplitLines(out), nil
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", args[0], shell.Oneline(string(exit.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func short(rev string) string {
	if len(rev) > 12 {
		return rev[:12]
	}
	return rev
}

// copyFiles copies the change set's files aside, so a run can read the tree as the strip found it.
func copyFiles(top string, files []string, to string) error {
	for _, file := range files {
		body, err := os.ReadFile(filepath.Join(top, file))
		if err != nil {
			continue
		}
		dst := filepath.Join(to, file)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("cannot create %s", shell.Echoable(filepath.Dir(dst)))
		}
		if err := os.WriteFile(dst, body, 0o644); err != nil {
			return fmt.Errorf("cannot write %s", shell.Echoable(dst))
		}
	}
	return nil
}

// copyTree copies a directory of plain files, the archive's shape.
func copyTree(from, to string) error {
	if err := os.MkdirAll(to, 0o755); err != nil {
		return fmt.Errorf("cannot create %s", shell.Echoable(to))
	}
	entries, err := os.ReadDir(from)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot read %s", shell.Echoable(from))
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		body, err := os.ReadFile(filepath.Join(from, entry.Name()))
		if err != nil {
			return fmt.Errorf("cannot read %s", shell.Echoable(entry.Name()))
		}
		if err := os.WriteFile(filepath.Join(to, entry.Name()), body, 0o644); err != nil {
			return fmt.Errorf("cannot write %s", shell.Echoable(entry.Name()))
		}
	}
	return nil
}

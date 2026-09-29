package commentrun

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	commentstrip "configs/ai/tools/comment-strip"
	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
	treefingerprint "configs/ai/tools/tree-fingerprint"
)

// loop sends code-review findings back to a writer at their sites in one file. It records each
// contradiction first, strips only those sites, adds each review sentence to its facts file and fills
// one prompt. The fingerprint is the run's own tree: run 14's runner computed it in the wrong one.

// Sites are stripped from the last line up, and each strip moves the sites stripped before it up by the
// lines it removed. Run 16 looped two sites of one file in two calls, and the second prompt named a
// line the first strip had moved.
func loop(r *runner, opts options, _ []string) int {
	runDir, archive, run := opts.one("run-dir"), opts.one("archive"), opts.one("run")
	type finding struct {
		line                  int
		sentence, contradicts string
		printed               []string
	}
	var findings []*finding
	path, contradicts, site := "", "", ""
	for _, arg := range opts.order {
		switch {
		case strings.HasPrefix(arg, "--contradict="):
			contradicts = strings.TrimPrefix(arg, "--contradict=")
		case strings.HasPrefix(arg, "--"):
		case site == "":
			site = arg
		default:
			colon := strings.LastIndex(site, ":")
			line, err := strconv.Atoi(site[colon+1:])
			if colon <= 0 || err != nil || line < 1 {
				return r.refuse("%s names no <path>:<line>", shell.Echoable(site))
			}
			if path != "" && site[:colon] != path {
				return r.refuse("%s is another file than %s: loop takes the sites of one file", shell.Echoable(site), shell.Echoable(path))
			}
			path = site[:colon]
			findings = append(findings, &finding{line: line, sentence: arg, contradicts: contradicts})
			contradicts, site = "", ""
		}
	}
	if runDir == "" || archive == "" || run == "" || len(findings) == 0 || site != "" {
		return r.refuse("%s", "loop takes --run-dir=<dir>, --archive=<dir>, --run=<run>, and each site as <path>:<line> with the review's sentence")
	}
	for _, p := range []*string{&runDir, &archive} {
		if !filepath.IsAbs(*p) {
			*p = filepath.Join(r.cwd, *p)
		}
	}
	held, err := readRun(filepath.Join(runDir, "run.txt"))
	if err != nil {
		return r.refuse("%s holds no run.txt: run seed first", shell.Echoable(runDir))
	}
	tree := held["top"]
	var out, errOut strings.Builder
	for _, f := range findings {
		if f.contradicts == "" {
			continue
		}
		if code := commentstrip.Strip("comment-strip.sh", []string{"--archive=" + archive, "--contradict=" + run, path,
			"line:" + strconv.Itoa(f.line), f.contradicts}, tree, repo.Exec{}, &out, &errOut); code != exitClean {
			return r.refuse("the strip refused the contradiction at line %d: %s", f.line, shell.Oneline(errOut.String()))
		}
	}
	var lines []string
	for _, f := range findings {
		lines = append(lines, strconv.Itoa(f.line))
	}
	name := "loop-" + strings.ReplaceAll(path, "/", "_") + "-" + strings.Join(lines, "-")
	facts := filepath.Join(runDir, "review-loop", name)
	sort.SliceStable(findings, func(i, j int) bool { return findings[i].line > findings[j].line })
	height := func() int {
		body, _ := os.ReadFile(filepath.Join(tree, path))
		return len(shell.SplitLines(string(body)))
	}
	var stripped []*finding
	for _, f := range findings {
		before := height()
		out.Reset()
		errOut.Reset()
		dir := filepath.Join(facts, strconv.Itoa(f.line))
		if code := commentstrip.Strip("comment-strip.sh", []string{"--facts=" + dir, "--archive=" + archive,
			"--lines=" + strconv.Itoa(f.line), path}, tree, repo.Exec{}, &out, &errOut); code != 1 {
			return r.refuse("the strip took no block at %s:%d: %s", shell.Echoable(path), f.line, shell.Oneline(errOut.String()))
		}
		removed := before - height()
		for _, earlier := range stripped {
			for i, printed := range earlier.printed {
				at, file, _ := strings.Cut(printed, " ")
				n, _ := strconv.Atoi(at[strings.LastIndex(at, ":")+1:])
				earlier.printed[i] = fmt.Sprintf("%s:%d %s", path, n-removed, file)
			}
		}
		for _, printed := range shell.SplitLines(out.String()) {
			at, file, _ := strings.Cut(printed, " ")
			name := filepath.Join(dir, file)
			body, err := os.ReadFile(name)
			if err != nil {
				return r.refuse("cannot read %s", shell.Echoable(name))
			}
			if err := os.WriteFile(name, append(body, []byte("\n# code review:\n"+f.sentence+"\n")...), 0o644); err != nil {
				return r.refuse("cannot write %s", shell.Echoable(name))
			}
			f.printed = append(f.printed, at+" "+name)
		}
		stripped = append(stripped, f)
	}
	fingerprint, err := treefingerprint.Fingerprint(tree)
	if err != nil {
		return r.refuse("cannot fingerprint %s: %v", shell.Echoable(tree), err)
	}
	home, _ := os.LookupEnv("HOME")
	template, err := os.ReadFile(filepath.Join(home, spawnTemplate))
	if err != nil {
		return r.refuse("cannot read the spawn template at ~/%s", spawnTemplate)
	}
	licence, _ := os.ReadFile(filepath.Join(runDir, "licence.txt"))
	var sites, printed strings.Builder
	n := 0
	for i := len(stripped) - 1; i >= 0; i-- {
		for _, line := range stripped[i].printed {
			n++
			at, file, _ := strings.Cut(line, " ")
			fmt.Fprintf(&sites, "%d. `%s` — facts `%s`\n", n, at, file)
			fmt.Fprintf(&printed, "%s %s\n", at, filepath.Base(file))
		}
	}
	emphasis := strings.TrimSpace(string(licence))
	if emphasis == "" {
		emphasis = "none"
	}
	prompt, err := fill(string(template), map[string]string{
		"Model": "`comment-writer`, the `workers` row in `~/.kk-flavor/configs/models.json`, as the runner resolved it for this dispatch.",
		"Candidate and evidence": fmt.Sprintf("the tree at `%s`, at HEAD `%s`, base `%s`, tree fingerprint `%s`. Code review "+
			"sent %d block(s) back, and the strip removed them; each facts file carries the review's sentence under "+
			"`# code review:`. No reusable verdicts.", tree, held["head"], held["base"], fingerprint, len(stripped)),
		"Change scope": fmt.Sprintf("the sites code review sent back, in `%s`:\n%s\nYou write into that file only. "+verdictSentence,
			path, strings.TrimRight(sites.String(), "\n")),
		"Held by a concurrent lane": "",
		"Ledger":                    "`" + filepath.Join(runDir, "comment-writer-"+name+"-queue.md") + "`",
		"Patch queue":               "",
		"User-stated emphasis":      emphasis,
		"Deterministic tool output": "`comment-strip.sh --facts=<dir> --archive=<archive> --lines=<line> " + path + "` at each site, the last line first, with its sites as the final tree numbers them:\n```\n" + strings.TrimRight(printed.String(), "\n") + "\n```",
	}, "~/.kk-flavor/workers/comment-writer.md")
	if err != nil {
		return r.refuse("%v", err)
	}
	prompted := filepath.Join(runDir, "spawn-writer-"+name+".md")
	if err := os.WriteFile(prompted, []byte(prompt), 0o644); err != nil {
		return r.refuse("cannot write %s", shell.Echoable(prompted))
	}
	fmt.Fprintln(r.stdout, prompted)
	return exitClean
}

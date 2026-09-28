package commentrun

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	commentstrip "configs/ai/tools/comment-strip"
	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
	treefingerprint "configs/ai/tools/tree-fingerprint"
)

// loop sends one code-review finding back to a writer at its site. Where review found the claim false,
// it records the contradiction first, and the strip then offers it. It strips only that site, adds the
// review's sentence to its facts file and fills the loop writer's prompt. The fingerprint is the run's
// own tree: run 14's runner computed it in its session's worktree.
func loop(r *runner, opts options, operands []string) int {
	runDir, archive, run := opts.one("run-dir"), opts.one("archive"), opts.one("run")
	if runDir == "" || archive == "" || run == "" || len(operands) != 2 {
		return r.refuse("%s", "loop takes --run-dir=<dir>, --archive=<dir>, --run=<run>, the site as <path>:<line> and the review's sentence")
	}
	site, sentence := operands[0], operands[1]
	colon := strings.LastIndex(site, ":")
	line, err := strconv.Atoi(site[colon+1:])
	if colon <= 0 || err != nil || line < 1 {
		return r.refuse("%s names no <path>:<line>", shell.Echoable(site))
	}
	path := site[:colon]
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
	if contradicted := opts.one("contradict"); contradicted != "" {
		if code := commentstrip.Strip("comment-strip.sh", []string{"--archive=" + archive, "--contradict=" + run, path,
			"line:" + strconv.Itoa(line), contradicted}, tree, repo.Exec{}, &out, &errOut); code != exitClean {
			return r.refuse("the strip refused the contradiction: %s", shell.Oneline(errOut.String()))
		}
	}
	name := "loop-" + strings.ReplaceAll(path, "/", "_") + "-" + strconv.Itoa(line)
	facts := filepath.Join(runDir, "review-loop", name)
	out.Reset()
	errOut.Reset()
	if code := commentstrip.Strip("comment-strip.sh", []string{"--facts=" + facts, "--archive=" + archive,
		"--lines=" + strconv.Itoa(line), path}, tree, repo.Exec{}, &out, &errOut); code != 1 {
		return r.refuse("the strip took no block at %s: %s", shell.Echoable(site), shell.Oneline(errOut.String()))
	}
	stripped := shell.SplitLines(out.String())
	for _, printed := range stripped {
		_, file, _ := strings.Cut(printed, " ")
		name := filepath.Join(facts, file)
		body, err := os.ReadFile(name)
		if err != nil {
			return r.refuse("cannot read %s", shell.Echoable(name))
		}
		if err := os.WriteFile(name, append(body, []byte("\n# code review:\n"+sentence+"\n")...), 0o644); err != nil {
			return r.refuse("cannot write %s", shell.Echoable(name))
		}
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
	var sites strings.Builder
	for n, printed := range stripped {
		at, file, _ := strings.Cut(printed, " ")
		fmt.Fprintf(&sites, "%d. `%s` — facts `%s`\n", n+1, at, filepath.Join(facts, file))
	}
	emphasis := strings.TrimSpace(string(licence))
	if emphasis == "" {
		emphasis = "none"
	}
	prompt, err := fill(string(template), map[string]string{
		"Model": "`comment-writer`, the `workers` row in `~/.kk-flavor/configs/models.json`, as the runner resolved it for this dispatch.",
		"Candidate and evidence": fmt.Sprintf("the tree at `%s`, at HEAD `%s`, base `%s`, tree fingerprint `%s`. Code review "+
			"sent one block back, and the strip removed it; its facts file carries the review's sentence under "+
			"`# code review:`. No reusable verdicts.", tree, held["head"], held["base"], fingerprint),
		"Change scope": fmt.Sprintf("the one site code review sent back, in `%s`:\n%s\nYou write into that file only. "+verdictSentence,
			path, strings.TrimRight(sites.String(), "\n")),
		"Held by a concurrent lane": "",
		"Ledger":                    "`" + filepath.Join(runDir, "comment-writer-"+name+"-queue.md") + "`",
		"Patch queue":               "",
		"User-stated emphasis":      emphasis,
		"Deterministic tool output": "`comment-strip.sh --facts=<dir> --archive=<archive> --lines=" + strconv.Itoa(line) + " " + path + "`, stdout:\n```\n" + strings.TrimRight(out.String(), "\n") + "\n```",
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

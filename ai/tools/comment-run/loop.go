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

// loop sends a round's review findings back to one writer, at their sites in every file of the round.
// It records each contradiction first, strips only those sites, adds each review sentence to its facts
// file and fills one prompt. Run 16 dispatched one loop writer per site, and each paid a writer's
// start-up. The fingerprint is the run's own tree: run 14's runner computed it in the wrong one.
func loop(r *runner, opts options, _ []string) int {
	runDir, archive, run := opts.one("run-dir"), opts.one("archive"), opts.one("run")
	var findings []*loopFinding
	contradicts, site := "", ""
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
			findings = append(findings, &loopFinding{path: site[:colon], line: line, sentence: arg, contradicts: contradicts})
			contradicts, site = "", ""
		}
	}
	if runDir == "" || archive == "" || run == "" || len(findings) == 0 || site != "" {
		return r.refuse("%s", "loop takes --run-dir=<dir>, --archive=<dir>, --run=<run>, and each site as <path>:<line> with the review's sentence")
	}
	r.absolute(&runDir, &archive)
	held, err := readRun(runDir)
	if err != nil {
		return r.refuse("%v", err)
	}
	// The dispatch reads the template and the rules before any file is stripped. A missing one refuses the
	// round while every file and the contradictions stand as they were.
	dispatch, err := newWriterDispatch(runDir)
	if err != nil {
		return r.refuse("%v", err)
	}
	tree := held["top"]
	var out, errOut strings.Builder
	for _, f := range findings {
		if f.contradicts == "" {
			continue
		}
		if code := commentstrip.Strip("comment-strip.sh", []string{"--archive=" + archive, "--contradict=" + run, f.path,
			"line:" + strconv.Itoa(f.line), f.contradicts}, tree, repo.Exec{}, &out, &errOut); code != exitClean {
			return r.refuse("the strip refused the contradiction at %s:%d: %s", shell.Echoable(f.path), f.line, shell.Oneline(errOut.String()))
		}
	}
	// One round is one writer, and its name counts the rounds already prompted.
	previous, _ := filepath.Glob(spawnFile(runDir, "loop-round-*"))
	name := fmt.Sprintf("loop-round-%d", len(previous)+1)
	facts := filepath.Join(runDir, "review-loop", name)
	var paths []string
	byPath := map[string][]*loopFinding{}
	for _, f := range findings {
		if _, seen := byPath[f.path]; !seen {
			paths = append(paths, f.path)
		}
		byPath[f.path] = append(byPath[f.path], f)
	}
	// A round strips its files together or none of them. A refused site restores every file of the round
	// and removes its facts, and the round can run again as it was given.
	before := map[string][]byte{}
	for _, path := range paths {
		body, err := os.ReadFile(filepath.Join(tree, path))
		if err != nil {
			return r.refuse("cannot read %s", shell.Echoable(path))
		}
		before[path] = body
	}
	undo := func() {
		for path, body := range before {
			_ = os.WriteFile(filepath.Join(tree, path), body, 0o644)
		}
		_ = os.RemoveAll(facts)
	}
	var sites, printed strings.Builder
	n := 0
	for _, path := range paths {
		stripped, err := loopStrip(tree, archive, filepath.Join(facts, strings.ReplaceAll(path, "/", "_")), path, byPath[path])
		if err != nil {
			undo()
			return r.refuse("%v; no file of the round was stripped", err)
		}
		for i := len(stripped) - 1; i >= 0; i-- {
			for _, line := range stripped[i].printed {
				n++
				at, file, _ := strings.Cut(line, " ")
				fmt.Fprintf(&sites, "%d. `%s` — facts `%s`\n", n, at, file)
				fmt.Fprintf(&printed, "%s %s\n", at, filepath.Base(file))
			}
		}
	}
	fingerprint, err := treefingerprint.Fingerprint(tree)
	if err != nil {
		undo()
		return r.refuse("cannot fingerprint %s: %v; no file of the round was stripped", shell.Echoable(tree), err)
	}
	prompted, err := dispatch.write(name, map[string]string{
		"Candidate and evidence": fmt.Sprintf("the tree at `%s`, at HEAD `%s`, base `%s`, tree fingerprint `%s`. Review "+
			"sent %d block(s) back, and the strip removed them; each facts file carries the review's sentence under "+
			"`# code review:`. No reusable verdicts.", tree, held["head"], held["base"], fingerprint, n),
		"Change scope": fmt.Sprintf("the sites review sent back, in `%s`:\n%s\nYou write into those file(s) only.",
			strings.Join(paths, "`, `"), strings.TrimRight(sites.String(), "\n")),
		"Held by a concurrent lane": "",
		"Deterministic tool output": "`comment-strip.sh --facts=<dir> --archive=<archive> --lines=<line> <file>` at each site, each file's last line first, with its sites as the final tree numbers them:\n```\n" + strings.TrimRight(printed.String(), "\n") + "\n```",
	})
	if err != nil {
		undo()
		return r.refuse("%v; no file of the round was stripped", err)
	}
	fmt.Fprintln(r.stdout, prompted)
	return exitClean
}

// loopFinding is one review sentence at one site, and the sites its strip printed.
type loopFinding struct {
	path                  string
	line                  int
	sentence, contradicts string
	printed               []string
}

// loopStrip strips one file's sites from the last line up, and adds each review sentence to its facts
// file. A later strip moves each earlier site up by the lines it removed. Run 16 looped two sites of
// one file in two calls, and the second prompt named a line the first strip had moved.
func loopStrip(tree, archive, facts, path string, findings []*loopFinding) ([]*loopFinding, error) {
	sort.SliceStable(findings, func(i, j int) bool { return findings[i].line > findings[j].line })
	height := func() int {
		body, _ := os.ReadFile(filepath.Join(tree, path))
		return len(shell.SplitLines(string(body)))
	}
	var out, errOut strings.Builder
	var stripped []*loopFinding
	for _, f := range findings {
		before := height()
		out.Reset()
		errOut.Reset()
		dir := filepath.Join(facts, strconv.Itoa(f.line))
		if code := commentstrip.Strip("comment-strip.sh", []string{"--facts=" + dir, "--archive=" + archive,
			"--lines=" + strconv.Itoa(f.line), path}, tree, repo.Exec{}, &out, &errOut); code != 1 {
			return nil, fmt.Errorf("the strip took no block at %s:%d: %s", shell.Echoable(path), f.line, shell.Oneline(errOut.String()))
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
				return nil, fmt.Errorf("cannot read %s", shell.Echoable(name))
			}
			if err := os.WriteFile(name, append(body, []byte("\n# code review:\n"+f.sentence+"\n")...), 0o644); err != nil {
				return nil, fmt.Errorf("cannot write %s", shell.Echoable(name))
			}
			f.printed = append(f.printed, at+" "+name)
		}
		stripped = append(stripped, f)
	}
	return stripped, nil
}

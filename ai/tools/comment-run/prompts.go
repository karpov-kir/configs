package commentrun

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"configs/ai/tools/shell"
)

// prompts partitions the sites seed printed over the writers and writes each writer's spawn prompt.
// A file's sites go to one writer, and the files go to the writer holding the fewest sites so far. The
// emphasis slot quotes only the human's words, from `licence.txt` in the run directory. Runs 11 and
// 12 carried an approval sentence the human never wrote.
func prompts(r *runner, opts options, _ []string) int {
	runDir := opts.one("run-dir")
	workers, err := strconv.Atoi(opts.one("workers"))
	if runDir == "" || err != nil || workers < 1 || workers > 26 {
		return r.refuse("%s", "prompts takes --run-dir=<dir> and --workers=<n>, from 1 to 26")
	}
	if !filepath.IsAbs(runDir) {
		runDir = filepath.Join(r.cwd, runDir)
	}
	run, err := readRun(filepath.Join(runDir, "run.txt"))
	if err != nil {
		return r.refuse("%s holds no run.txt: run seed first", shell.Echoable(runDir))
	}
	body, err := os.ReadFile(filepath.Join(runDir, "sites.txt"))
	if err != nil {
		return r.refuse("%s holds no sites.txt: run seed first", shell.Echoable(runDir))
	}
	byFile := map[string][]string{}
	var order []string
	for _, line := range shell.SplitLines(string(body)) {
		site, _, _ := strings.Cut(line, " ")
		file := site[:strings.LastIndex(site, ":")]
		if _, seen := byFile[file]; !seen {
			order = append(order, file)
		}
		byFile[file] = append(byFile[file], line)
	}
	if len(order) == 0 {
		fmt.Fprintf(r.stderr, "%s: no site to write, so no writer is prompted\n", r.self)
		return exitClean
	}
	groups := partition(order, byFile, min(workers, len(order)))
	licence, _ := os.ReadFile(filepath.Join(runDir, "licence.txt"))
	for n, files := range groups {
		name := string(rune('A' + n))
		prompt := writerPrompt(run, runDir, name, files, groups, byFile, strings.TrimSpace(string(licence)))
		out := filepath.Join(runDir, "spawn-writer-"+name+".md")
		if err := os.WriteFile(out, []byte(prompt), 0o644); err != nil {
			return r.refuse("cannot write %s", shell.Echoable(out))
		}
		count := 0
		for _, file := range files {
			count += len(byFile[file])
		}
		fmt.Fprintf(r.stdout, "%s %d site(s) in %d file(s) %s\n", name, count, len(files), out)
	}
	return exitClean
}

// partition gives each file to the group holding the fewest sites, the largest file first.
func partition(files []string, byFile map[string][]string, n int) [][]string {
	sorted := append([]string(nil), files...)
	sort.SliceStable(sorted, func(i, j int) bool { return len(byFile[sorted[i]]) > len(byFile[sorted[j]]) })
	groups := make([][]string, n)
	load := make([]int, n)
	for _, file := range sorted {
		least := 0
		for g := range groups {
			if load[g] < load[least] {
				least = g
			}
		}
		groups[least] = append(groups[least], file)
		load[least] += len(byFile[file])
	}
	return groups
}

func readRun(path string) (map[string]string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	run := map[string]string{}
	for _, line := range shell.SplitLines(string(body)) {
		if key, value, found := strings.Cut(line, "="); found {
			run[key] = value
		}
	}
	return run, nil
}

func writerPrompt(run map[string]string, runDir, name string, files []string, groups [][]string,
	byFile map[string][]string, licence string) string {
	var sites, held strings.Builder
	n := 0
	for _, file := range files {
		for _, line := range byFile[file] {
			n++
			site, facts, _ := strings.Cut(line, " ")
			fmt.Fprintf(&sites, "%d. `%s` — facts `%s`\n", n, site, facts)
		}
	}
	for g, other := range groups {
		if string(rune('A'+g)) == name {
			continue
		}
		fmt.Fprintf(&held, "`%s` (comment-writer %c); ", strings.Join(other, "`, `"), 'A'+g)
	}
	others := strings.TrimSuffix(held.String(), "; ")
	if others == "" {
		others = "none"
	}
	emphasis := "none"
	if licence != "" {
		emphasis = licence
	}
	return fmt.Sprintf(`Apply the `+"`~/.kk-flavor/workers/comment-writer.md`"+` contract for the requested scope. Read its common procedure and only the branch references this task needs. Work as a leaf; return further-work requests to the caller.

Model: `+"`comment-writer`"+`, the `+"`workers`"+` row in `+"`~/.kk-flavor/configs/models.json`"+`.

Candidate and evidence: the tree at `+"`%s`"+`, at HEAD `+"`%s`"+`, base `+"`%s`"+`. The tree is HEAD with the strip's removals applied; no block stands at any site you are given. A block the strip kept stands at a site you are not given, and you leave it as it stands.

Change scope: the change set `+"`%s...%s`"+`. Your sites, %d, in `+"`%s`"+`, one facts directory per file, `+"`identifiers.txt`"+` beside each facts file:
%s
You write into those file(s) only. Write each block into its file as soon as it passes its gate.

Held by a concurrent lane: read freely, write none, and return a fix that lands there as a proposal: %s. Every other file: read freely, write none.

Ledger: `+"`%s`"+`

User-stated emphasis, the human's own words: %s

You are spawned, with no interactive user: return your verdicts and findings as data, or `+"`blocked: <what you need>`"+`, per your contract and `+"`~/.kk-flavor/standards/skill-protocol.md`"+`. An act your own contract leaves to its caller or the human is one you return as a proposal.
`, run["top"], run["head"], run["base"], run["base"], run["head"], n, strings.Join(files, "`, `"),
		strings.TrimRight(sites.String(), "\n"), others,
		filepath.Join(runDir, "comment-writer-"+name+"-queue.md"), emphasis)
}

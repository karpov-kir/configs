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
	home, _ := os.LookupEnv("HOME")
	template, err := os.ReadFile(filepath.Join(home, spawnTemplate))
	if err != nil {
		return r.refuse("cannot read the spawn template at ~/%s", spawnTemplate)
	}
	licence, _ := os.ReadFile(filepath.Join(runDir, "licence.txt"))
	for n, files := range groups {
		name := string(rune('A' + n))
		prompt, err := writerPrompt(string(template), run, runDir, name, files, groups, byFile, strings.TrimSpace(string(licence)))
		if err != nil {
			return r.refuse("%v", err)
		}
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

// spawnTemplate is the template every dispatch in the ecosystem fills, under the flavor root.
const spawnTemplate = ".kk-flavor/templates/spawn-prompt.md"

// fill fills the spawn template's slots by their labels, and omits a slot left empty, as the template
// says. A slot the tool does not know refuses the prompt: a copy of the template drifted from the
// template the rest of the ecosystem dispatches with.
func fill(template string, slots map[string]string, contract string) (string, error) {
	if _, rest, found := strings.Cut(template, "-->"); found && strings.HasPrefix(strings.TrimSpace(template), "<!--") {
		template = rest
	}
	var out []string
	for _, paragraph := range strings.Split(strings.TrimSpace(template), "\n\n") {
		paragraph = strings.TrimSpace(paragraph)
		switch {
		case paragraph == "":
		case strings.HasPrefix(paragraph, "Apply the `<"):
			_, rest, _ := strings.Cut(paragraph, ">`")
			out = append(out, "Apply the `"+contract+"`"+rest)
		case strings.HasPrefix(paragraph, "You are spawned"):
			out = append(out, paragraph)
		case strings.HasPrefix(paragraph, "Reached by a handoff"):
			// A writer is dispatched by the lane itself, and no handoff opened it.
		default:
			at := strings.Index(paragraph, ": <")
			if at < 0 {
				return "", fmt.Errorf("the template holds a paragraph this tool cannot fill: %s", shell.CutBytesMarked(paragraph, 60))
			}
			label := paragraph[:at]
			value, known := "", false
			for name, v := range slots {
				if strings.HasPrefix(label, name) {
					value, known = v, true
				}
			}
			if !known {
				return "", fmt.Errorf("the template names a slot this tool does not fill: %s", shell.CutBytesMarked(label, 60))
			}
			if value != "" {
				out = append(out, paragraph[:at+2]+value)
			}
		}
	}
	return strings.Join(out, "\n\n") + "\n", nil
}

func writerPrompt(template string, run map[string]string, runDir, name string, files []string, groups [][]string,
	byFile map[string][]string, licence string) (string, error) {
	var sites, held, stdout strings.Builder
	n := 0
	for _, file := range files {
		for _, line := range byFile[file] {
			n++
			site, facts, _ := strings.Cut(line, " ")
			fmt.Fprintf(&sites, "%d. `%s` — facts `%s`\n", n, site, facts)
			fmt.Fprintf(&stdout, "%s %s\n", site, filepath.Base(facts))
		}
	}
	for g, other := range groups {
		if string(rune('A'+g)) == name {
			continue
		}
		fmt.Fprintf(&held, "`%s` (comment-writer %c); ", strings.Join(other, "`, `"), 'A'+g)
	}
	others := strings.TrimSuffix(held.String(), "; ")
	emphasis := "none"
	if licence != "" {
		emphasis = licence
	}
	return fill(template, map[string]string{
		"Model": "`comment-writer`, the `workers` row in `~/.kk-flavor/configs/models.json`, as the runner resolved it for this dispatch.",
		"Candidate and evidence": fmt.Sprintf("the tree at `%s`, at HEAD `%s`, base `%s`. The tree is HEAD with the strip's "+
			"removals applied; no block stands at any site you are given. A block the strip kept stands at a site you are "+
			"not given, and you leave it as it stands. No reusable verdicts.", run["top"], run["head"], run["base"]),
		"Change scope": fmt.Sprintf("the change set `%s...%s`. Your sites, %d, in `%s`, one facts directory per file, "+
			"`identifiers.txt` beside each facts file:\n%s\nYou write into those file(s) only. Write each block into its file "+
			"as soon as it passes its gate.", run["base"], run["head"], n, strings.Join(files, "`, `"),
			strings.TrimRight(sites.String(), "\n")),
		"Held by a concurrent lane": others,
		"Ledger":                    "`" + filepath.Join(runDir, "comment-writer-"+name+"-queue.md") + "`",
		"Patch queue":               "",
		"User-stated emphasis":      emphasis,
		"Deterministic tool output": "`comment-strip.sh --facts=<dir> --archive=<archive> <file>` on your file(s), stdout:\n```\n" + strings.TrimRight(stdout.String(), "\n") + "\n```",
	}, "~/.kk-flavor/workers/comment-writer.md")
}

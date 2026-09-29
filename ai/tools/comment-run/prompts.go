package commentrun

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"configs/ai/tools/shell"
)

// prompts partitions the sites seed printed over the writers and writes each writer's spawn prompt.
// It decides the number of writers from the sites. Run 18's ten writers read 48.9M tokens from the cache
// over 412 turns, and run 18b's one writer read 34.9M over 133 turns for the same 162 sites.

// The emphasis slot quotes only the human's words, from `licence.txt` in the run directory. Runs 11
// and 12 carried an approval sentence the human never wrote.
func prompts(r *runner, opts options, _ []string) int {
	runDir := opts.one("run-dir")
	if opts.one("workers") != "" {
		return r.refuse("%s", "prompts decides the number of writers from the sites and takes no --workers")
	}
	if runDir == "" {
		return r.refuse("%s", "prompts takes --run-dir=<dir>")
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
	home, _ := os.LookupEnv("HOME")
	template, err := os.ReadFile(filepath.Join(home, spawnTemplate))
	if err != nil {
		return r.refuse("cannot read the spawn template at ~/%s", spawnTemplate)
	}
	rules, err := writerRules(home)
	if err != nil {
		return r.refuse("%v", err)
	}
	licence, _ := os.ReadFile(filepath.Join(runDir, "licence.txt"))
	batches := plan(order, byFile)
	n := 0
	for b, groups := range batches {
		first := n
		for _, files := range groups {
			name := string(rune('A' + n))
			n++
			others := map[string][]string{}
			for g, other := range groups {
				if first+g != n-1 {
					others[string(rune('A'+first+g))] = other
				}
			}
			prompt, err := writerPrompt(string(template), rules, run, runDir, name, files, others, byFile, strings.TrimSpace(string(licence)))
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
			fmt.Fprintf(r.stdout, "%s batch %d, %d site(s) in %d file(s) %s\n", name, b+1, count, len(files), out)
		}
	}
	if len(batches) > 1 {
		fmt.Fprintf(r.stderr, "%s: %d batches: dispatch each batch's writers together, and the next batch after it returns\n",
			r.self, len(batches))
	}
	return exitClean
}

// The writer count the reviewer set on 2026-09-29: one writer up to 150 sites, two up to 300, three
// above, and at most three at a time. Run 18b's one writer grew about 1.7k tokens a site from 147k. A
// writer's context then holds some 400 sites, and past three writers' reach the batches follow.
const (
	oneWriterSites   = 150
	twoWriterSites   = 300
	maxWriters       = 3
	writerReachSites = 400
)

// writersFor is how many writers a number of sites takes, up to the three one batch holds.
func writersFor(sites int) int {
	switch {
	case sites <= oneWriterSites:
		return 1
	case sites <= twoWriterSites:
		return 2
	}
	return maxWriters
}

// plan cuts the files into batches of at most three writers' reach, then gives each batch's files to
// its writers. A file is never split.
func plan(order []string, byFile map[string][]string) [][][]string {
	var batches [][][]string
	var batch []string
	load := 0
	for _, file := range order {
		if load > 0 && load+len(byFile[file]) > maxWriters*writerReachSites {
			batches = append(batches, partition(batch, byFile, writersFor(load)))
			batch, load = nil, 0
		}
		batch = append(batch, file)
		load += len(byFile[file])
	}
	return append(batches, partition(batch, byFile, writersFor(load)))
}

// partition gives the files to n writers. A directory stays together up to one writer's room, the
// larger of its share and 150 sites. A larger directory is dealt out file by file. Each unit goes to
// the writer holding the fewest sites, the largest unit first.
func partition(files []string, byFile map[string][]string, n int) [][]string {
	n = min(n, len(files))
	total := 0
	for _, file := range files {
		total += len(byFile[file])
	}
	share := max((total+n-1)/n, oneWriterSites)
	byDir := map[string][]string{}
	var dirs []string
	for _, file := range files {
		dir := filepath.Dir(file)
		if _, seen := byDir[dir]; !seen {
			dirs = append(dirs, dir)
		}
		byDir[dir] = append(byDir[dir], file)
	}
	var units [][]string
	sizeOf := func(unit []string) int {
		size := 0
		for _, file := range unit {
			size += len(byFile[file])
		}
		return size
	}
	for _, dir := range dirs {
		if sizeOf(byDir[dir]) <= share {
			units = append(units, byDir[dir])
			continue
		}
		for _, file := range byDir[dir] {
			units = append(units, []string{file})
		}
	}
	sort.SliceStable(units, func(i, j int) bool { return sizeOf(units[i]) > sizeOf(units[j]) })
	// A writer holds at least one unit, so a directory that fits one writer never leaves another empty.
	n = min(n, len(units))
	groups := make([][]string, n)
	load := make([]int, n)
	for _, unit := range units {
		least := 0
		for g := range groups {
			if load[g] < load[least] {
				least = g
			}
		}
		groups[least] = append(groups[least], unit...)
		load[least] += sizeOf(unit)
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

// writerContract names the contract a writer applies, which reaches it in the prompt.
const writerContract = "comment-writer brief given in full at the end of this prompt"

// writerRules is the text every writer opens on: the brief, and the standard's Comments section it
// writes to. Run 16's twenty writers each read both from disk before their first site, and every read
// cost calls. The files stay the rules, and the prompt carries them word for word.
func writerRules(home string) (string, error) {
	brief, err := os.ReadFile(filepath.Join(home, ".kk-flavor", "workers", "comment-writer.md"))
	if err != nil {
		return "", fmt.Errorf("cannot read the brief at ~/.kk-flavor/workers/comment-writer.md")
	}
	style, err := os.ReadFile(filepath.Join(home, ".kk-flavor", "standards", "code-style.md"))
	if err != nil {
		return "", fmt.Errorf("cannot read the standard at ~/.kk-flavor/standards/code-style.md")
	}
	text := string(style)
	start := strings.Index(text, "## Comments")
	if start < 0 {
		return "", fmt.Errorf("the standard holds no Comments section")
	}
	end := strings.Index(text[start+1:], "\n## ")
	section := text[start:]
	if end >= 0 {
		section = text[start : start+1+end]
	}
	return "The rules, word for word. Read no rule file: the text below is what the files hold, and your task " +
		"follows it.\n\n" +
		"=== ~/.kk-flavor/workers/comment-writer.md ===\n" + strings.TrimSpace(string(brief)) + "\n\n" +
		"=== ~/.kk-flavor/standards/code-style.md → Comments ===\n" + strings.TrimSpace(section) + "\n", nil
}

// returnFile is where a writer writes its return. Run 18b's one writer held 162 sites, and its return
// outgrew a message. archive-written could read only the summary it sent instead.
func returnFile(runDir, name string) string {
	return filepath.Join(runDir, "return-writer-"+name+".md")
}

// returnSentence tells a writer how to work: a file at a time, each block through the Edit tool, and the
// return to a file. Every turn reads the whole context again, and run 18's writers took 2.6 turns a site
// where run 18b's took 0.8. Run 18b's writer wrote by shell script, and one insert slipped.
func returnSentence(path string) string {
	return "Work a file at a time. Read all of a file's facts files in one call. Check all of its blocks in one " +
		"voice-check call, where the brief checks one block: each part is a record, a line reading `---`, and the " +
		"block with its declaration and body, the parts apart on a line reading `===`, and each finding names its " +
		"part as `-#<n>` on stdin. Then write the file's blocks with Edit calls issued together in one turn; no shell " +
		"command writes a source file. Write your whole return, in the brief's Verdict shape, to `" + path +
		"` with the Write tool, and end your message with that path."
}

func writerPrompt(template, rules string, run map[string]string, runDir, name string, files []string,
	others map[string][]string, byFile map[string][]string, licence string) (string, error) {
	var sites, stdout strings.Builder
	n := 0
	for _, file := range files {
		for _, line := range byFile[file] {
			n++
			site, facts, _ := strings.Cut(line, " ")
			fmt.Fprintf(&sites, "%d. `%s` — facts `%s`\n", n, site, facts)
			fmt.Fprintf(&stdout, "%s %s\n", site, filepath.Base(facts))
		}
	}
	var names []string
	for other := range others {
		names = append(names, other)
	}
	sort.Strings(names)
	var held []string
	for _, other := range names {
		held = append(held, fmt.Sprintf("`%s` (comment-writer %s)", strings.Join(others[other], "`, `"), other))
	}
	emphasis := "none"
	if licence != "" {
		emphasis = licence
	}
	prompt, err := fill(template, map[string]string{
		"Model": "`comment-writer`, the `workers` row in `~/.kk-flavor/configs/models.json`, as the runner resolved it for this dispatch.",
		"Candidate and evidence": fmt.Sprintf("the tree at `%s`, at HEAD `%s`, base `%s`. The tree is HEAD with the strip's "+
			"removals applied; no block stands at any site you are given. A block the strip kept stands at a site you are "+
			"not given, and you leave it as it stands. No reusable verdicts.", run["top"], run["head"], run["base"]),
		"Change scope": fmt.Sprintf("the change set `%s...%s`. Your sites, %d, in `%s`, one facts directory per file, "+
			"`identifiers.txt` beside each facts file:\n%s\nYou write into those file(s) only. "+verdictSentence+" "+returnSentence(returnFile(runDir, name)), run["base"], run["head"], n,
			strings.Join(files, "`, `"), strings.TrimRight(sites.String(), "\n")),
		"Held by a concurrent lane": strings.Join(held, "; "),
		"Ledger":                    "`" + filepath.Join(runDir, "comment-writer-"+name+"-queue.md") + "`",
		"Patch queue":               "",
		"User-stated emphasis":      emphasis,
		"Deterministic tool output": "`comment-strip.sh --facts=<dir> --archive=<archive> <file>` on your file(s), stdout:\n```\n" + strings.TrimRight(stdout.String(), "\n") + "\n```",
	}, writerContract)
	if err != nil {
		return "", err
	}
	// The rules come first, so every writer opens on the same text and only the sites differ.
	return rules + "\n" + prompt, nil
}

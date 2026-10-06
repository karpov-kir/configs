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
func prompts(r *runner, opts options, _ []string) int {
	runDir := opts.one("run-dir")
	if opts.one("workers") != "" {
		return r.refuse("%s", "prompts decides the number of writers from the sites and takes no --workers")
	}
	if runDir == "" {
		return r.refuse("%s", "prompts takes --run-dir=<dir>")
	}
	r.absolute(&runDir)
	run, err := readRun(runDir)
	if err != nil {
		return r.refuse("%v", err)
	}
	body, err := os.ReadFile(filepath.Join(runDir, "sites.txt"))
	if err != nil {
		return r.refuse("%s holds no sites.txt: run seed first", shell.Echoable(runDir))
	}
	lines := shell.SplitLines(string(body))
	facts, err := factsRoot(lines)
	if err != nil {
		return r.refuse("%v", err)
	}
	byFile := map[string][]string{}
	var order []string
	for _, line := range lines {
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
	dispatch, err := newWriterDispatch(runDir)
	if err != nil {
		return r.refuse("%v", err)
	}
	warnWithoutWriterAgent(r)
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
			out, err := dispatch.write(name, writerSlots(run, writerShare{files: files, byFile: byFile, others: others}, facts))
			if err != nil {
				return r.refuse("%v", err)
			}
			count := 0
			for _, file := range files {
				count += len(byFile[file])
			}
			fmt.Fprintf(r.stdout, "%s batch %d, %d site(s) in %d file(s) %s\n", name, b+1, count, len(files), out)
		}
	}
	fmt.Fprintln(r.stdout, dispatchLine)
	if len(batches) > 1 {
		fmt.Fprintf(r.stderr, "%s: %d batches: dispatch each batch's writers together, and the next batch after it returns\n",
			r.self, len(batches))
	}
	return exitClean
}

// dispatchLine tells the dispatcher how a prompt reaches its writer. A writer handed a prompt's path
// read it in four Read calls, and every call reads the whole context again. A session keeps the agent
// definition it loaded when it started: on 2026-10-02 one started before a pull dispatched the writer
// with its old body, and the writer returned blocked.
const dispatchLine = "Dispatch each writer with its prompt file's text as the task message, never its path for the writer to Read. " +
	"A session started before the flavor checkout was last pulled dispatches the writer's old definition: restart it first."

// The reviewer set these counts on 2026-10-02. A thin writer opens near 5k tokens, and every request
// reads its whole context again, so a writer costs the square of its sites. One thin writer over 135
// sites grew to 254k and read 13.7M tokens from the cache. Three writers of 45 should re-read a third of
// that. Writes and output stay, so the bill should fall 30 to 40%. Both figures are estimates from that
// one run until the next full pass of like size reports its usage. The reach is from run 18b: its writer grew
// about 1.7k tokens a site from 147k, so a writer's context holds some 400 sites.
const (
	oneWriterSites   = 50
	twoWriterSites   = 100
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
// larger of its share and 50 sites. A larger directory is dealt out file by file. Each unit goes to
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

// readRun reads the run.txt seed wrote into runDir, or says to run seed first where it stands no run.txt.
func readRun(runDir string) (map[string]string, error) {
	body, err := os.ReadFile(filepath.Join(runDir, "run.txt"))
	if err != nil {
		return nil, fmt.Errorf("%s holds no run.txt: run seed first", shell.Echoable(runDir))
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
func fill(template string, slots map[string]string) (string, error) {
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
			out = append(out, "Apply the `"+writerContract+"`"+rest)
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

// writerContract names the contract a writer applies, by the path it reads it at.
const writerContract = "~/.kk-flavor/workers/comment-writer.md"

// readSentence opens every writer's prompt. The prompt names the rules and quotes neither, so a rule has
// one copy, and taint holds the writer to the reads. The brief says the writer runs under the skill
// protocol, and a writer that read it would carry 19k bytes it was dispatched without.
const readSentence = "Your first turn reads `~/.kk-flavor/workers/comment-writer.md` and `~/.kk-flavor/standards/comments.md` " +
	"whole, in one message, and makes no other call. Read no other file under `~/.kk-flavor/standards` or " +
	"`~/.kk-flavor/workers`; running a script there is not a read.\n"

// returnFile is where a writer writes its return. Run 18b's one writer held 162 sites, and its return
// outgrew a message. archive-written could read only the summary it sent instead.
func returnFile(runDir, name string) string {
	return filepath.Join(runDir, "return-writer-"+name+".md")
}

// returnSentence tells a writer how to work: a file at a time, each block through the Edit tool, and the
// return to a file. Every turn reads the whole context again, and run 18's writers took 2.6 turns a site
// where run 18b's took 0.8. Run 18b's writer wrote by shell script, and one insert slipped. Run 25's writer
// left its check inputs in the scratchpad root, so the sentence names the run directory for them. Run
// 26's writers changed the spacing of code lines under their blocks.
func returnSentence(path string) string {
	return "Append each site's verdict line to your ledger as you finish the site, in the shape below. Work a file at a time. Read all of a file's facts files in one call. Check all of its blocks in one " +
		"voice-check call, where the brief checks one block: each part is a record, a line reading `---`, and the " +
		"block with its declaration and body, the parts apart on a line reading `===`, and each finding names its " +
		"part as `-#<n>` on stdin. A file you write for a check, the voice-check input among them, goes in `" +
		filepath.Dir(path) + "`. Then write the file's blocks with Edit calls issued together in one turn; no shell " +
		"command writes a source file. An Edit changes comment lines only, and the code line under a block keeps its " +
		"spacing, since archive-written refuses a changed code line. Write your whole return, in the brief's Verdict shape, to `" + path +
		"` with the Write tool, and end your message with that path."
}

// writerDispatch is what every writer's prompt shares: the spawn template and the emphasis slot's words.
type writerDispatch struct {
	runDir, template, emphasis string
}

// newWriterDispatch reads the spawn template under HOME and the licence in the run directory, and
// records the rules' sum there. The emphasis slot quotes only the human's words, from `licence.txt`. Runs 11 and 12
// carried an approval sentence the human never wrote.
func newWriterDispatch(runDir string) (writerDispatch, error) {
	home, _ := os.LookupEnv("HOME")
	template, err := os.ReadFile(filepath.Join(home, spawnTemplate))
	if err != nil {
		return writerDispatch{}, fmt.Errorf("cannot read the spawn template at ~/%s", spawnTemplate)
	}
	if err := recordRules(runDir); err != nil {
		return writerDispatch{}, err
	}
	licence, _ := os.ReadFile(filepath.Join(runDir, "licence.txt"))
	emphasis := strings.TrimSpace(string(licence))
	if emphasis == "" {
		emphasis = "none"
	}
	return writerDispatch{runDir: runDir, template: string(template), emphasis: emphasis}, nil
}

// spawnFile is where the prompt of the writer called name is written.
func spawnFile(runDir, name string) string {
	return filepath.Join(runDir, "spawn-writer-"+name+".md")
}

// write writes the prompt of the writer called name, and returns its path. The prompt is the read
// sentence, then the template filled with the stage's slots and the slots every writer shares. The change scope ends on
// the verdict shape and on where the return goes.
func (d writerDispatch) write(name string, slots map[string]string) (string, error) {
	slots["Model"] = "`comment-writer`, the `workers` row in `~/.kk-flavor/configs/models.json`, as the runner resolved it for this dispatch."
	slots["Change scope"] += " " + verdictSentence + " " + returnSentence(returnFile(d.runDir, name))
	slots["Ledger"] = "`" + filepath.Join(d.runDir, "comment-writer-"+name+"-queue.md") + "`"
	slots["Patch queue"] = ""
	// A writer goes to its own agent type, so no tool sends it to the general agent.
	slots["Needs"] = ""
	slots["User-stated emphasis"] = d.emphasis
	prompt, err := fill(d.template, slots)
	if err != nil {
		return "", err
	}
	path := spawnFile(d.runDir, name)
	if err := os.WriteFile(path, []byte(readSentence+"\n"+prompt), 0o644); err != nil {
		return "", fmt.Errorf("cannot write %s", shell.Echoable(path))
	}
	return path, nil
}

// writerShare is one writer's part of a batch: its files, each file's site lines, and the files each
// other writer of the batch holds, by that writer's name.
type writerShare struct {
	files          []string
	byFile, others map[string][]string
}

// writerSlots fills the slots a writer of the prompts stage takes from its share of the batch. The strip's
// stdout names each site once. A 135-site prompt said every site three times, and its sites outgrew its rules.
func writerSlots(run map[string]string, share writerShare, facts string) map[string]string {
	files, others := share.files, share.others
	var stdout strings.Builder
	n := 0
	for _, file := range files {
		for _, line := range share.byFile[file] {
			n++
			site, path, _ := strings.Cut(line, " ")
			fmt.Fprintf(&stdout, "%s %s\n", site, filepath.Base(path))
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
	return map[string]string{
		"Candidate and evidence": fmt.Sprintf("the tree at `%s`, at HEAD `%s`, base `%s`. The tree is HEAD with the strip's "+
			"removals applied; no block stands at any site you are given. A block the strip kept stands at a site you are "+
			"not given, and you leave it as it stands. No reusable verdicts.", run["top"], run["head"], run["base"]),
		"Change scope": fmt.Sprintf("the change set `%s...%s`. Your sites, %d, are the strip's stdout below, one "+
			"`<file>:<line> <facts file>` line each. %s You write into the files those lines name only.",
			run["base"], run["head"], n, factsSentence(facts, "`<root>/<file with / as _>/<facts file>`")),
		"Held by a concurrent lane": strings.Join(held, "; "),
		"Deterministic tool output": "`comment-strip.sh --facts=<dir> --archive=<archive> <file>` on your file(s), stdout:\n```\n" + strings.TrimRight(stdout.String(), "\n") + "\n```",
	}
}

// factsSentence says where a site's facts file stands, from the root and how a path is built under it.
func factsSentence(root, built string) string {
	return "A site's facts file is " + built + ", the root being `" + root + "`, with `identifiers.txt` beside it."
}

// factsRoot is the directory every site line's facts file stands under, by way of its file's facts
// directory. A site line built any other way refuses the prompt, which could not name its facts file.
func factsRoot(lines []string) (string, error) {
	root := ""
	for _, line := range lines {
		site, path, _ := strings.Cut(line, " ")
		colon := strings.LastIndex(site, ":")
		if colon <= 0 {
			return "", fmt.Errorf("the site line %s names no <file>:<line>", shell.Echoable(line))
		}
		file := site[:colon]
		dir := filepath.Dir(path)
		at := filepath.Dir(dir)
		if filepath.Join(factsDir(at, file), filepath.Base(path)) != path || root != "" && at != root {
			return "", fmt.Errorf("the site line %s names no facts file under the run's facts root", shell.Echoable(site))
		}
		root = at
	}
	return root, nil
}

// writerAgent is where the installer mounts the thin writer for Claude. Codex has no directory for a
// defined agent, and the installer mounts none there.
const writerAgent = ".claude/agents/comment-writer.md"

// warnWithoutWriterAgent names a home missing the thin writer. A writer dispatched there starts
// as a general agent, some 32k tokens heavier at its first turn, and the run's report names that.
func warnWithoutWriterAgent(r *runner) {
	home, _ := os.LookupEnv("HOME")
	if _, err := os.Stat(filepath.Join(home, writerAgent)); err == nil {
		return
	}
	fmt.Fprintf(r.stderr, "%s: no comment-writer agent at ~/%s: a writer dispatched here starts as a general agent; "+
		"run bootstrap for Claude, or name the general writers in the run's report\n", r.self, writerAgent)
}

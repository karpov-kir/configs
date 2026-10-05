package commentrun

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	commentstrip "configs/ai/tools/comment-strip"
	"configs/ai/tools/shell"
)

var (
	// reVerdictLine is a verdict as a writer returns it, `none` included: archive-written's own pattern
	// reads only the blocks written.
	reVerdictLine = regexp.MustCompile(`(?m)^\**Block \d+/\d+ (\S+?):(\d+)[^|\n]*\|\s*(OK|none)\b`)
	// reLoopRound names a loop round's return, by the round's number.
	reLoopRound = regexp.MustCompile(`^return-writer-loop-round-(\d+)\.md$`)
	// reListedFacts is a site with its facts path in a prompt: the numbered list prompts carried before
	// 2026-10-02, and the strip's stdout block after it.
	reListedFacts = regexp.MustCompile("(?m)^\\d+\\. `([^`]+)` — facts `([^`]+)`$")
	reStdoutFacts = regexp.MustCompile(`(?m)^(\S+:\d+) (\S+\.facts)$`)
)

// declinedStage records, from a finished run's returns, the sites its writers declined, for an archive
// that predates `.declined`. Each round's verdict stands over the rounds before it.
func declinedStage(r *runner, opts options, _ []string) int {
	run, runDir, archive, tree := opts.one("run"), opts.one("run-dir"), opts.one("archive"), opts.one("tree")
	if run == "" || runDir == "" || archive == "" {
		return r.refuse("%s", "declined takes --run=<run>, --run-dir=<dir> and --archive=<dir>, and --tree=<dir> where the run's tree moved")
	}
	r.absolute(&runDir, &archive)
	if tree == "" {
		held, err := readRun(runDir)
		if err != nil {
			return r.refuse("%v", err)
		}
		tree = held["top"]
	}
	r.absolute(&tree)
	declined, cleared, err := decideFromReturns(runDir, archive, run, tree, r.stderr)
	if err != nil {
		return r.refuse("%v", err)
	}
	fmt.Fprintf(r.stdout, "%d declined site(s) recorded, %d decline(s) cleared by a block written later\n", declined, cleared)
	return exitClean
}

// decideFromReturns reads every writer return in runDir in round order, maps each verdict through its
// round's prompt to the archived claims it weighed, and records the last verdict on each: `none`
// declines the site, and a block written takes the decline away.
func decideFromReturns(runDir, archive, run, tree string, warn io.Writer) (int, int, error) {
	returns, err := filepath.Glob(returnFile(runDir, "*"))
	if err != nil {
		return 0, 0, err
	}
	sort.SliceStable(returns, func(i, j int) bool { return roundOf(returns[i]) < roundOf(returns[j]) })
	final := map[string]map[string]bool{}
	for _, ret := range returns {
		facts, err := factsBySite(runDir, ret)
		if err != nil {
			return 0, 0, err
		}
		body, err := os.ReadFile(ret)
		if err != nil {
			return 0, 0, fmt.Errorf("cannot read %s", shell.Echoable(ret))
		}
		for _, m := range reVerdictLine.FindAllStringSubmatch(string(body), -1) {
			path, site, verdict := m[1], m[1]+":"+m[2], m[3]
			factsPath, found := facts[site]
			if !found {
				fmt.Fprintf(warn, "%s: %s is no site its round offered, so its verdict is left unrecorded\n", filepath.Base(ret), site)
				continue
			}
			text, err := os.ReadFile(factsPath)
			if err != nil {
				return 0, 0, fmt.Errorf("cannot read %s", shell.Echoable(factsPath))
			}
			all, own, err := commentstrip.OfferedRecords(archive, path, string(text))
			if err != nil {
				return 0, 0, err
			}
			if final[path] == nil {
				final[path] = map[string]bool{}
			}
			names := own
			if verdict == "none" {
				names = all
			}
			for _, name := range names {
				final[path][name] = verdict == "none"
			}
		}
	}
	declined, cleared := 0, 0
	var paths []string
	for path := range final {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		var none, written []string
		for name, isNone := range final[path] {
			if isNone {
				none = append(none, name)
			} else {
				written = append(written, name)
			}
		}
		n, err := commentstrip.Decide(archive, run, tree, path, none, true)
		if err != nil {
			return 0, 0, err
		}
		declined += n
		n, err = commentstrip.Decide(archive, run, tree, path, written, false)
		if err != nil {
			return 0, 0, err
		}
		cleared += n
	}
	return declined, cleared, nil
}

// roundOf orders a return: the writers' first, then each loop round by its number.
func roundOf(ret string) int {
	if m := reLoopRound.FindStringSubmatch(filepath.Base(ret)); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

// factsBySite maps each site a return's round offered to its facts file under runDir. The writers' round
// reads sites.txt, and a loop round its prompt. A path recorded under another directory, as a copied
// run's is, is read under runDir by what follows `facts/` or `review-loop/`.
func factsBySite(runDir, ret string) (map[string]string, error) {
	out := map[string]string{}
	under := func(recorded, marker string) string {
		if _, rest, found := strings.Cut(recorded, "/"+marker+"/"); found {
			return filepath.Join(runDir, marker, rest)
		}
		return recorded
	}
	if m := reLoopRound.FindStringSubmatch(filepath.Base(ret)); m != nil {
		round := "loop-round-" + m[1]
		prompt, err := os.ReadFile(spawnFile(runDir, round))
		if err != nil {
			return nil, fmt.Errorf("cannot read the prompt of %s", round)
		}
		for _, l := range reListedFacts.FindAllStringSubmatch(string(prompt), -1) {
			out[l[1]] = under(l[2], "review-loop")
		}
		for _, l := range reStdoutFacts.FindAllStringSubmatch(string(prompt), -1) {
			if _, listed := out[l[1]]; !listed {
				out[l[1]] = filepath.Join(runDir, "review-loop", round, l[2])
			}
		}
		return out, nil
	}
	body, err := os.ReadFile(filepath.Join(runDir, "sites.txt"))
	if err != nil {
		return nil, fmt.Errorf("%s holds no sites.txt", shell.Echoable(runDir))
	}
	for _, line := range shell.SplitLines(string(body)) {
		if site, facts, found := strings.Cut(line, " "); found {
			out[site] = under(facts, "facts")
		}
	}
	return out, nil
}

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
	// A backfill records the rules and code standing now as what the run decided on. A run decided
	// under other rules is refused.
	if held := rulesHeld(runDir); held != "" && !commentstrip.SameRules(held, commentstrip.RulesSum()) {
		return r.refuse("the rules changed since %s's prompts: %s then, %s now; its declines would hold for rules no writer weighed",
			run, held, commentstrip.RulesSum())
	}
	fmt.Fprintf(r.stderr, "%s: a run without a record of what each site carried is read by its claims' words, against the code in %s as it stands now\n",
		r.self, shell.Echoable(tree))
	declined, cleared, err := decideFromReturns(runDir, archive, run, tree, r.stderr)
	if err != nil {
		return r.refuse("%v", err)
	}
	fmt.Fprintf(r.stdout, "%d declined site(s) recorded, %d decline(s) cleared by a block written later\n", declined, cleared)
	return exitClean
}

// decideFromReturns reads every writer return in runDir in round order, each verdict mapped to the facts
// its site carried. Where the strip recorded the site's records, the last verdict on each stands. An
// older run maps by the claims' words, and a record any block was written for stays undeclined there:
// a claim repeated at another declaration reads the same.
func decideFromReturns(runDir, archive, run, tree string, warn io.Writer) (int, int, error) {
	returns, err := filepath.Glob(returnFile(runDir, "*"))
	if err != nil {
		return 0, 0, err
	}
	sort.SliceStable(returns, func(i, j int) bool { return roundOf(returns[i]) < roundOf(returns[j]) })
	type verdict struct {
		offered commentstrip.Offered
		isNone  bool
	}
	exact := map[string]map[string]verdict{}
	none, written := map[string]map[string]bool{}, map[string]map[string]bool{}
	mark := func(m map[string]map[string]bool, path string, names []string) {
		if m[path] == nil {
			m[path] = map[string]bool{}
		}
		for _, name := range names {
			m[path][name] = true
		}
	}
	for _, ret := range returns {
		facts, err := factsBySite(runDir, ret)
		if err != nil {
			fmt.Fprintf(warn, "%s: %v, so its verdicts are left unrecorded\n", filepath.Base(ret), err)
			continue
		}
		body, err := os.ReadFile(ret)
		if err != nil {
			return 0, 0, fmt.Errorf("cannot read %s", shell.Echoable(ret))
		}
		for _, m := range reVerdictLine.FindAllStringSubmatch(string(body), -1) {
			path, site, isNone := m[1], m[1]+":"+m[2], m[3] == "none"
			factsPath, found := facts[site]
			if !found {
				fmt.Fprintf(warn, "%s: %s is no site its round offered, so its verdict is left unrecorded\n", filepath.Base(ret), site)
				continue
			}
			if offered := commentstrip.ReadOffered(factsPath); offered != nil {
				if exact[path] == nil {
					exact[path] = map[string]verdict{}
				}
				for _, o := range offered {
					exact[path][o.Decl+"\x00"+o.Claims] = verdict{offered: o, isNone: isNone}
				}
				continue
			}
			text, err := os.ReadFile(factsPath)
			if err != nil {
				fmt.Fprintf(warn, "%s: cannot read the facts of %s, so its verdict is left unrecorded\n", filepath.Base(ret), site)
				continue
			}
			all, _, err := commentstrip.OfferedRecords(archive, path, string(text))
			if err != nil {
				return 0, 0, err
			}
			if isNone {
				mark(none, path, all)
			} else {
				mark(written, path, all)
			}
		}
	}
	declined, cleared := 0, 0
	for path, byRecord := range exact {
		var declines, clears []commentstrip.Offered
		for _, v := range byRecord {
			if v.isNone {
				declines = append(declines, v.offered)
			} else {
				clears = append(clears, v.offered)
			}
		}
		n, err := commentstrip.DecideOffered(archive, run, path, declines, true)
		if err != nil {
			return 0, 0, err
		}
		declined += n
		if n, err = commentstrip.DecideOffered(archive, run, path, clears, false); err != nil {
			return 0, 0, err
		}
		cleared += n
	}
	var paths []string
	for path := range none {
		paths = append(paths, path)
	}
	for path := range written {
		if none[path] == nil {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		var declines, clears []string
		for name := range none[path] {
			if !written[path][name] {
				declines = append(declines, name)
			}
		}
		for name := range written[path] {
			clears = append(clears, name)
		}
		n, err := commentstrip.Decide(archive, run, tree, path, declines, true)
		if err != nil {
			fmt.Fprintf(warn, "%s: %v, so its declines are left unrecorded\n", path, err)
			continue
		}
		declined += n
		if n, err = commentstrip.Decide(archive, run, tree, path, clears, false); err != nil {
			fmt.Fprintf(warn, "%s: %v\n", path, err)
			continue
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

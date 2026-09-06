package main

// The mutants themselves: what each one breaks, where, and which case has to notice. Data only — the
// harness that runs them is main.go beside this file, which stays readable as one program because the
// list it walks does not grow through it.

type mutant struct {
	label string
	file  string
	// The suite that has to notice. It is not always the package the edit lands in: shell and ecoroot
	// are read by both ports, and a figure ecocheck alone prints is asserted by the agreement cases,
	// which live in ecostats' suite.
	suite string
	// The test within that suite that has to notice, as `go test -run` reads it. Empty runs the whole
	// suite: a mutant costs the suite it names, and ecocheck's forks two `bash -n` per script, so
	// naming the test is what keeps a list this long inside the gate's budget.
	//
	// Named at test-function granularity and never at the subtest's: a t.Run name is prose and gets
	// reworded, and this field would then be the stale anchor the `from` string exists to not be. The
	// *case* is named in the comment above any mutant where which one it is not obvious.
	by   string
	from string
	to   string
}

// Every one of these aims at a guard with a case behind it. A mutation that kills nothing means the
// guard is unobserved.
//
// A guard is disabled by appending ` && false`, never by replacing its condition with `false`. Go
// refuses to compile a function whose local is no longer read, so replacing `if tracked != "" {` with
// `if false {` costs `tracked` its last use and the mutant does not build. `broken` is the verdict for
// that, and it proves nothing about the guard — but it prints beside `killed` in the same column, and
// six mutants sat there reading as a finished list. Appending keeps every name in the condition read.
var mutants = []mutant{
	{"direction: the shared finding bound removed", "direction.go", "./eco-check/", "TestDirectionScan", "*count <= findingCap", "*count <= 100000"},
	{"report: the per-rank cap removed", "report.go", "./eco-check/", "TestTheGravestFindingSurvivesAFlood", "return shownInRank[rank] >= findingCap", "return shownInRank[rank] >= 100000"},
	// `|| true`, not the `&& false` the header above describes: the anchored condition is the one
	// that *skips* a finding, so appending to it is what leaves the floor keeping nothing.
	{"report: the per-class floor removed", "report.go", "./eco-check/", "TestTheGravestFindingSurvivesAFlood", "if findings[i].class.shown > 0 || isRankFull(findings[i].class.rank) {", "if findings[i].class.shown > 0 || isRankFull(findings[i].class.rank) || true {"},
	{"report: the catch-all class given a note of its own", "report.go", "./eco-check/", "TestASuppressionNoteCountsOnlyItsOwnClass", "case class.prefix != \"\" && !isNoted[class]:", "case !isNoted[class]:"},
	// Both rows go, or the mutation shows nothing: with one left, the catch-all still holds a single
	// kind and the report reads the same. The defect needs two kinds in one class. The floor then
	// keeps one line between them, and whichever sorts second prints nothing at all.
	{"report: two rank-5 kinds put back in one class", "report.go", "./eco-check/", "TestTheGravestFindingSurvivesAFlood", "\t{skillDirWithoutSkillFile, 5},\n\t{skillWithoutDescription, 5},\n", ""},
	// The half that removing one row does show. A kind with no row keeps its floor line but loses its
	// own count. The rest of its flood then leaves through the trailing "further finding(s) not
	// shown", attributed to nothing.
	{"report: a flooded rank-5 kind left noteless", "report.go", "./eco-check/", "TestTheGravestFindingSurvivesAFlood", "\t{danglingHomeRef, 5},\n", ""},
	// The shared-region rows, on the same reasoning as the two rank-5 mutants above. `not checked for
	// drift` is the one that then prints nothing at all, and it is the graver of the two: the drift
	// check did not run on that region.
	{"report: two shared-region kinds put back in one class", "report.go", "./eco-check/", "TestTheGravestFindingSurvivesAFlood", "\t{sharedRegionHasDrifted, 1},\n\t{sharedRegionNotChecked, 1},\n", ""},
	// The third row on its own, which the pair above leaves standing. This kind is what a deleted copy
	// of a shared region produces. Removing its row leaves it alone in the catch-all, where the floor
	// still prints it. So what moves is where the line lands, and the case that notices reads its rank.
	{"report: a shared region with no counterpart left at the default rank", "report.go", "./eco-check/", "TestTheGravestFindingSurvivesAFlood", "\t{sharedRegionWithoutCounterpart, 1},\n", ""},
	// findingCap is the right number only for the class that filled the rank. Every class the floor
	// left at one line then gets a negative count, so a case that only asserts a note is there stays
	// green.
	{"report: a note counting its whole rank's withheld findings", "report.go", "./eco-check/", "TestTheGravestFindingSurvivesAFlood", "class.total-class.shown, suppressedMarker", "class.total-findingCap, suppressedMarker"},
	// The flag is set here and read at the tally. Unset, every note counts as a finding and the
	// trailing "further finding(s) not shown" is short by one per note.
	{"report: a note counted as one of the findings", "report.go", "./eco-check/", "TestTheGravestFindingSurvivesAFlood", "rankedLine{rank: class.rank, isNote: true,", "rankedLine{rank: class.rank, isNote: false,"},
	// Only observable on a tree whose own committed path holds the marker wording, which is why the
	// case builds that filename out of the constant.
	{"report: notes told apart by their text", "report.go", "./eco-check/", "TestTheGravestFindingSurvivesAFlood", "if !line.isNote {", "if !strings.Contains(bounded, suppressedMarker) {"},
	// The head that keeps a finding's first bytes the checker's own. Removed, the finding leads with a
	// basename two committed scripts share, joins whatever class that text matches, and takes the one
	// line the floor reserves for it.
	{"subcommands: a finding led with a basename the tree chose", "subcommands.go", "./eco-check/", "TestTheGravestFindingSurvivesAFlood", "subcommandUsageDoesNotName + shell.CutBytesMarked(shell.Oneline(name), findingNameCap) +\n\t\t\t\" — \" + c.scriptNamed(base) + \" accepts it\")", "c.scriptNamed(base) + \" accepts \" + shell.CutBytesMarked(shell.Oneline(name), findingNameCap))"},
	{"report: an unread dispatch left at the default rank", "report.go", "./eco-check/", "TestAnUnreadDispatchSurvivesAFlood", "\t{unreadDispatch, 2},\n", ""},
	// The rank that keeps this scan's "I checked nothing about that file" lines out of rank 5, where
	// they share one budget with `dangling link:` and sort below every one of them. Dropped, a flood of
	// crafted links hides them and the report reads clean of the very thing they exist to say.
	// Ranking on the whole line rather than its head: a crafted link target then carries a ranked
	// phrase into a `dangling link:` finding and promotes the flood above what it is burying. The case
	// that observes this passed over the mutation until its forged phrase was one the rank table still
	// holds.
	{"report: findings ranked on the whole line, not its head", "report.go", "./eco-check/", "TestTheGravestFindingSurvivesAFlood", "if strings.HasPrefix(line, candidate.prefix) {", "if strings.Contains(line, candidate.prefix) {"},
	// Two reads, two bounds, two mutants: the line-oriented read and the read the budget counts its
	// files through. `info.Size() > maxFileBytes` matches twice, so each anchor carries the return above
	// it to say which read it means.
	//
	// The comparison is what each mutation disables, never the refusal under it: leave the c.add
	// standing and the finding still fires, so a case asserting the message passes over a file that was
	// read anyway. Written that way, both of these killed nothing.
	{"shell: per-file byte bound removed from the line read", "shell.go", "./eco-check/", "TestOversizeFileIsReportedNotRead", `		return nil, err
	}
	if info.Size() > maxFileBytes {`, `		return nil, err
	}
	if info.Size() > (1 << 62) {`},
	{"shell: per-file byte bound removed from the budget read", "shell.go", "./eco-check/", "TestOversizeBudgetFileIsReportedNotCounted", `		return 0, 0
	}
	if info.Size() > maxFileBytes {`, `		return 0, 0
	}
	if info.Size() > (1 << 62) {`},
	{"refs: citation target read with no regular-file test", "citations.go", "./eco-check/", "TestCitationTargetMustBeARegularFile", "if !shell.IsRegularFile(target) {", "if false {"},
	// Both directions off the bare-rule-ID scan's single pattern, which is why they share an anchor.
	// The second is aimed at the quiet case alone: it widens the separator and the phrase just far
	// enough to swallow the delimited citation that finding recommends writing, and leaves the three
	// cases that assert a finding green. A mutant that also broke those would let their failure stand
	// in for the quiet one's, which proves nothing about it.
	{"refs: bare rule-ID scan never fires", "rule-ids.go", "./eco-check/", "TestBareRuleIDCitations", `[Cc]ore [Pp]rinciples? +#?[0-9]+`, `[Zz]ore [Pp]rinciples? +#?[0-9]+`},
	{"refs: bare rule-ID scan reports the form it recommends", "rule-ids.go", "./eco-check/", "TestBareRuleIDCitations", `[Cc]ore [Pp]rinciples? +#?[0-9]+`, `[Cc]ore[ -][Pp]rinciples?[^0-9]*[0-9]+`},
	{"scripts: parse-error text left unsanitised", "scripts.go", "./eco-check/", "TestParseErrorsCarryNoControlByte", `syntaxError+shell.Oneline(line)`, `syntaxError+line`},
	{"mounts: resolved mount path left unsanitised", "mounts.go", "./eco-check/", "TestMountFindingCarriesNoControlByte", "shell.Oneline(mountHave)", "mountHave"},
	// The name each of the three mount messages leads with, one mutant per message. A skill directory
	// name is the branch's own text, and the sanitiser on it is a separate call from the one on the
	// path beside it — so a case exercising the path arm leaves the name arm free to be deleted.
	{"mounts: an unmounted skill's own name left unsanitised", "mounts.go", "./eco-check/", "TestMountFindingCarriesNoControlByte", `shell.Join(skillsMount, shell.Oneline(name)) + " is missing`, `shell.Join(skillsMount, name) + " is missing`},
	{"mounts: that name left unsanitised on the elsewhere arm", "mounts.go", "./eco-check/", "TestMountFindingCarriesNoControlByte", `shell.Join(skillsMount, shell.Oneline(name)) + " -> " + shell.Oneline(mountHave)`, `shell.Join(skillsMount, name) + " -> " + shell.Oneline(mountHave)`},
	// The reverse half, which the loop over this tree's own skill directories cannot reach at all.
	{"mounts: a mount that outlived its skill goes unreported", "mounts.go", "./eco-check/", "TestAMountThatOutlivedItsSkillIsReported", "if shell.IsDir(mountPath) {", "if shell.IsDir(mountPath) || true {"},
	// The other direction of that same guard: dropped, a skill renamed with its old mount still
	// resolving to the new directory reads as a deletion.
	{"mounts: a mount that still resolves reported as gone", "mounts.go", "./eco-check/", "TestAMountThatOutlivedItsSkillIsReported", "if shell.IsDir(mountPath) {", "if shell.IsDir(mountPath) && false {"},
	{"mounts: another checkout's mount reported as this tree's", "mounts.go", "./eco-check/", "TestAMountThatOutlivedItsSkillIsReported", `if mountedInto == "" || mountedInto != skillsHere {`, `if (mountedInto == "" || mountedInto != skillsHere) && false {`},
	{"mounts: a skill this tree still has reported by both halves", "mounts.go", "./eco-check/", "TestAMountThatOutlivedItsSkillIsReported", "if skillDirs[name] {", "if skillDirs[name] && false {"},
	{"mounts: a mount without a skill left unsanitised", "mounts.go", "./eco-check/", "TestAMountWithoutASkillCarriesNoControlByte", "shell.Oneline(target)", "target"},
	{"mounts: a mount's own name left unsanitised", "mounts.go", "./eco-check/", "TestAMountWithoutASkillCarriesNoControlByte", `shell.Join(skillsMount, shell.Oneline(name)) + " -> " + shell.Oneline(target)`, `shell.Join(skillsMount, name) + " -> " + shell.Oneline(target)`},
	// A relative target, resolved the way mountTarget says it has to be. Both directions, because an
	// absolute target must not move either.
	// Appended to rather than replaced, per the note at the head of this list: `strings.HasPrefix` here
	// is the package's last use of `strings`, so a bare `false` or `true` orphans the import and the
	// mutant comes back broken instead of killed.
	{"mounts: a relative target read against the working directory", "mounts.go", "./eco-check/", "TestAMountThatOutlivedItsSkillIsReported", `if !strings.HasPrefix(target, "/") {`, `if !strings.HasPrefix(target, "/") && false {`},
	{"mounts: an absolute target rewritten as a relative one", "mounts.go", "./eco-check/", "TestAMountThatOutlivedItsSkillIsReported", `if !strings.HasPrefix(target, "/") {`, `if !strings.HasPrefix(target, "/") || true {`},
	// The skip note, both directions: only the pair makes absence of that line readable as "the scan
	// ran".
	{"mounts: a skipped scan never says it was skipped", "mounts.go", "./eco-check/", "TestTheMountScanAsksOnlyAboutTheInstalledCheckout", "if c.root.IsInstalled() {", "if c.root.IsInstalled() || true {"},
	{"mounts: a scan that ran reported as skipped", "mounts.go", "./eco-check/", "TestTheMountScanAsksOnlyAboutTheInstalledCheckout", "if c.root.IsInstalled() {", "if c.root.IsInstalled() && false {"},
	// Its rank, which decides whether the finding is read at all under a flood.
	{"report: a mount without a skill left at the default rank", "report.go", "./eco-check/", "TestTheGravestFindingSurvivesAFlood", "\t{mountWithoutASkill, 1},\n", ""},

	// The delimited-citation half. Undelimited is how a section citation stops resolving in silence,
	// so both directions are needed: the finding never firing, and it firing on the two forms the
	// finding itself recommends.
	{"citations: the undelimited form unreported", "citations.go", "./eco-check/", "TestDelimitedSectionCitations", "if !cited.isDelimited {", "if false {"},
	// The other empty-resolution branch. Left standing, the citation falls through with an empty target
	// and is reported as "not a regular file" instead — a finding, so a case asserting merely that
	// something was reported would pass over it.
	{"citations: an unresolvable cited path unreported", "citations.go", "./eco-check/", "TestUnresolvableCitationPaths", `if target == "" {`, "if false {"},
	{"citations: the delimited forms reported as undelimited", "citation-syntax.go", "./eco-check/", "TestDelimitedSectionCitations", "isDelimited = section != \"\"", "isDelimited = false"},

	// The skill-directory scan. Each of the three defects makes a skill unreachable rather than
	// merely mis-linked, and each has exactly one case behind it.
	{"skills: a directory carrying no SKILL.md unreported", "skills.go", "./eco-check/", "TestSkillDirectory", `if !c.holdsRegularFile(shell.Join(entry.path, "SKILL.md")) {`, "if false {"},
	{"skills: a name/dir mismatch unreported", "skills.go", "./eco-check/", "TestSkillDirectory", "if declared != shell.BaseName(shell.DirName(file)) {", "if false {"},
	{"skills: a SKILL.md with no description unreported", "skills.go", "./eco-check/", "TestSkillDirectory", `if shell.FrontmatterDescription(lines) == "" {`, "if false {"},

	// The test-position scan. Each aims at one of the ways a script can hide from kk-reduce's Phase 6:
	// a header that declares nothing, one naming a suite that is not there, and the four bounds that
	// keep a crafted header from being read as a declaration.
	//
	// One guard here has no mutant, and it cannot be given a case at all: the `isCleanBasename` filter
	// on the suite map excludes exactly the names `namedTestSuite` cannot extract from a header, so no
	// excluded key could ever have matched one. It guards against a `find` listing split on newlines,
	// which os.ReadDir does not produce.
	{"test position: over-long header not reported", "scripts.go", "./eco-check/", "TestScriptTestPosition", "if len(named) > namedSuiteCap {", "if false {"},
	{"test position: header bound removed", "scripts.go", "./eco-check/", "TestScriptTestPosition", "if i >= headerLineCap {", "if i >= 100000 {"},
	{"test position: header read past the comment block", "scripts.go", "./eco-check/", "TestScriptTestPosition", `if !strings.HasPrefix(line, "#") {`, "if false {"},
	{"test position: the -mutate.sh exemption removed", "scripts.go", "./eco-check/", "TestScriptTestPosition", `|| strings.HasSuffix(base, "-mutate.sh")`, "|| false"},
	{"test position: a named suite that is absent unreported", "scripts.go", "./eco-check/", "TestScriptTestPosition", "if len(carriers[suite]) == 0 {", "if false {"},
	{"test position: a script declaring nothing unreported", "scripts.go", "./eco-check/", "TestScriptTestPosition", "if !anyMatch(header, untestedDeclared) {", "if false {"},
	{"test position: a bare untested: clears the check", "scripts.go", "./eco-check/", "TestScriptTestPosition", `untested:[[:space:]]*[^[:space:]]`, `untested:[[:space:]]*`},
	{"test position: a dash-led suite name goes unread", "scripts.go", "./eco-check/", "TestScriptTestPosition", `[A-Za-z0-9_.-]+-test\.sh`, `[A-Za-z0-9_.]+-test\.sh`},

	// The subcommand call-site scan's Go half. The first mutant below removes the Go dispatch
	// entirely, which is this scan going quiet rather than failing.
	{"subcommands: the Go dispatch never consulted", "subcommands.go", "./eco-check/", "TestGoDispatchSubcommandCallSites", "want(base, c.toolSubcommands(base, lines, opened))", "want(base, nil)"},
	{"subcommands: a missing tool source goes quiet", "subcommands.go", "./eco-check/", "TestADispatchThatCannotBeReadIsReported", `return nil, "no source directory at " + named`, `return nil, ""`},
	{"subcommands: a source with no dispatch goes quiet", "subcommands.go", "./eco-check/", "TestADispatchThatCannotBeReadIsReported", `return nil, "no switch under " + named + " refuses with a '" + shell.Oneline(marker) + "' line"`, `return nil, ""`},
	{"subcommands: any switch read as the dispatch", "subcommands.go", "./eco-check/", "TestGoDispatchSubcommandCallSites", "carries = carries || strings.Contains(line, marker)", "carries = true"},
	{"subcommands: the usage grammar read one line only", "subcommands.go", "./eco-check/", "TestGoDispatchSubcommandCallSites", "if closed || len(grammar) > usageGrammarCap {", "if closed || len(grammar) > 0 {"},
	{"subcommands: count bound removed", "subcommands.go", "./eco-check/", "TestSubcommandCountIsBounded", "if len(queries) >= subcommandCap {", "if len(queries) >= 100000 {"},
	{"subcommands: a name only the dispatch has goes unreported", "subcommands.go", "./eco-check/", "TestUsageAndDispatchAreHeldAgainstEachOther", "onlyIn(dispatched, documented)", "onlyIn(dispatched, dispatched)"},
	{"subcommands: a name only the usage has goes unreported", "subcommands.go", "./eco-check/", "TestUsageAndDispatchAreHeldAgainstEachOther", "onlyIn(documented, dispatched)", "onlyIn(documented, documented)"},

	// The subcommand scan's shell half, and the reach that made it go quiet rather than fail. Three
	// mutants share the opening pattern's anchor, because they are three different questions about
	// one line: whether it still reaches every spelling, and the two ways a looser one over-reaches.
	// The first is the defect as it stood — matched as a single literal, the scan checked neither of
	// that stub's subcommands and reported nothing about the file.
	{"subcommands: the dispatch opening matched as one literal again", "subcommands.go", "./eco-check/", "TestAShellDispatchIsReadInEverySpellingOfItsOpening", `^case [^#]*\$\{?1[^0-9]`, `^case "\$\{1:-\}" in`},
	{"subcommands: any top-level case read as a dispatch", "subcommands.go", "./eco-check/", "TestATopLevelCaseIsNotAlwaysADispatch", `^case [^#]*\$\{?1[^0-9]`, `^case `},
	{"subcommands: an in-function lookup table read as a dispatch", "subcommands.go", "./eco-check/", "TestATopLevelCaseIsNotAlwaysADispatch", `^case [^#]*\$\{?1[^0-9]`, `case [^#]*\$\{?1[^0-9]`},
	// The gap between the loose opening and the strict arm pattern, which is the whole reason the
	// first mutant above is a *finding* now rather than a silence: a dispatch this scan holds and can
	// name no subcommand of has to say so.
	{"subcommands: a dispatch with no readable arm goes quiet", "subcommands.go", "./eco-check/", "TestADispatchWhoseArmsCannotBeReadIsReported", "if opened && len(labels) == 0 {", "if opened && len(labels) == 0 && false {"},
	// The same gap through the stub half. Two mutants, one anchor: the arm keeps two determinations
	// quiet, and each of them is a separate claim about a script that is not broken.
	{"subcommands: a script whose own dispatch was read reported unreachable", "subcommands.go", "./eco-check/", "TestAUsageGrammarWithNoDispatchBehindItIsReported", "if opened || len(documented) == 0 {", "if len(documented) == 0 {"},
	{"subcommands: a script naming no subcommand reported unreachable", "subcommands.go", "./eco-check/", "TestAUsageGrammarWithNoDispatchBehindItIsReported", "if opened || len(documented) == 0 {", "if opened {"},
	// The firing side of that arm, which the two above leave standing — they redden the determinations
	// beside it, and a mutant that only reddens the quiet cases proves nothing about the finding. Both
	// carry the return under the call, or the anchor also matches the arm below. The second is the
	// promise the message makes: it says the usage's own list was checked in the dispatch's place, so
	// dropping that list makes the line a claim about work nothing did.
	{"subcommands: a script with no way to a dispatch reported as nothing at all", "subcommands.go", "./eco-check/", "TestAUsageGrammarWithNoDispatchBehindItIsReported", "c.reportUnreadDispatch(base, `it names no tool=\"<name>\" to reach one through, and opens no case dispatch on $1`, documented)", "_ = base"},
	{"subcommands: the usage list announced as checked and then dropped", "subcommands.go", "./eco-check/", "TestAUsageGrammarWithNoDispatchBehindItIsReported", "case dispatch on $1`, documented)\n\t\treturn documented", "case dispatch on $1`, documented)\n\t\treturn nil"},

	// The rest live outside ecocheck/. A mutant names its file relative to that directory, and the
	// overlay reaches a dependency, or a sibling package, as readily as the package under test.
	// ecoroot holds the `@import` scan and the mount: they are facts about one checkout, not shell
	// primitives, so they sit with the root every path is built from.
	{"imports: name cut a fixed two bytes past the boundary", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", "token[at[0]+boundary+1:at[1]]", "token[at[0]+boundary*0+2:at[1]]"},
	{"imports: uncounted name left unsanitised", "../eco-root/imports.go", "./eco-check/", "TestUncountedNoteCarriesNoControlByte", "shell.CutBytesMarked(shell.Oneline(name), 60)", "shell.CutBytesMarked(name, 60)"},
	// The three caps on the naming half of that note, which shares its anchor with the mutant above:
	// a mutant is refused when *its own* anchor matches twice, and two mutants aiming at one line are
	// two separate questions about it.
	{"imports: uncounted names not capped in entries", "../eco-root/imports.go", "./eco-check/", "TestUncountedNamesAreCapped", "if len(shown) > uncountedNamedCap {", "if len(shown) > 100000 {"},
	// The other half of that cap: the entries kept had a mutant, the arithmetic saying how many were
	// withheld had none. Unfired, the note names ten and says nothing about the rest — a list reading as
	// whole, which is what every marking cut above exists to stop, one level up.
	{"imports: the withheld count never says how many", "../eco-root/imports.go", "./eco-check/", "TestUncountedNamesAreCapped", "if len(uncounted) > uncountedNamedCap {", "if len(uncounted) > 100000 {"},
	{"imports: the uncounted list not capped in bytes", "../eco-root/imports.go", "./eco-check/", "TestUncountedNamesAreCapped", "shell.CutBytesMarked(joined.String(), 200)", "joined.String()"},
	{"imports: one uncounted name not capped in bytes", "../eco-root/imports.go", "./eco-check/", "TestUncountedNamesAreCapped", "shell.CutBytesMarked(shell.Oneline(name), 60)", "shell.Oneline(name)"},
	// The two trims that make DirName dirname(1) rather than a split on the last slash. Separate
	// mutants, because they answer different inputs. The first puts back the divergence where DirName
	// split a trailing slash and BaseName trimmed it. The second gives `a//b` the parent `a/` instead
	// of `a`. The first anchor carries the signature above it, or it also matches BaseName's own trim.
	{"path: DirName splits a trailing slash instead of trimming it", "../shell/path.go", "./shell/", "TestDirNameAndBaseNameAreDirnameAndBasename", `func DirName(path string) string {
	trimmed := strings.TrimRight(path, "/")`, `func DirName(path string) string {
	trimmed := path`},
	{"path: DirName leaves a repeated slash on the parent", "../shell/path.go", "./shell/", "TestDirNameAndBaseNameAreDirnameAndBasename", `if parent := strings.TrimRight(trimmed[:i], "/"); parent != "" {`, `if parent := trimmed[:i]; parent != "" {`},
	// The marker itself, now that its contract has a case of its own. A constant, so the mutation is its
	// value: an ellipsis character carries the bytes Oneline strips, which is the announcement putting
	// back what the sanitiser removed.
	{"cut: the marker carries a byte Oneline strips", "../shell/text.go", "./shell/", "TestCutMarkerCarriesNoByteOnelineStrips", `const CutMarker = "..."`, "const CutMarker = \"\u2026\""},
	// The marking cut, which every message quoting a name the tree chose now goes through. Both
	// directions, because both are a lie about the same thing: a cut that says nothing is read as the
	// whole text, and a mark on text that was never cut says the opposite. The second anchor carries
	// the two lines under it, or it also matches CutBytes' own early return one function up.
	{"cut: a message cut with nothing marking it", "../shell/text.go", "./shell/", "TestCutBytesMarkedSaysWhenItCut", "return CutBytes(text, n-len(CutMarker)) + CutMarker", "return CutBytes(text, n)"},
	{"cut: a message that was never cut marked anyway", "../shell/text.go", "./shell/", "TestCutBytesMarkedSaysWhenItCut", `	if len(text) <= n {
		return text
	}
	return CutBytes(text, n-len(CutMarker)) + CutMarker`, `	if len(text) <= n && false {
		return text
	}
	return CutBytes(text, n-len(CutMarker)) + CutMarker`},
	// And every call site whose own case observes the mark, one mutant each. Reverting one to plain
	// CutBytes is the whole mutation: the message still fires and still carries a name, so a case
	// asserting merely that something was reported passes over it — which is why each of these names a
	// case that reads the marker itself. Both anchors in a budget.go carry the text before the call,
	// or each also matches the import refusal in the same file.
	{"stats: refused budget name cut without a mark", "../eco-stats/budget.go", "./eco-stats/", "TestACutMessageSaysThatItWasCut", "s.root.Named(), shell.CutBytesMarked(shell.Oneline(name), 80))", "s.root.Named(), shell.CutBytes(shell.Oneline(name), 80))"},
	{"stats: unreadable reason cut without a mark", "../eco-stats/measure.go", "./eco-stats/", "TestACutMessageSaysThatItWasCut", "shell.CutBytesMarked(shell.Oneline(err.Error()), 160)", "shell.CutBytes(shell.Oneline(err.Error()), 160)"},
	{"check: refused budget name cut without a mark", "budget.go", "./eco-check/", "TestACutRefusalSaysThatItWasCut", `") — not read, not counted: " + shell.CutBytesMarked(shell.Oneline(name), findingNameCap))`, `") — not read, not counted: " + shell.CutBytes(shell.Oneline(name), findingNameCap))`},
	{"check: refused import name cut without a mark", "budget.go", "./eco-check/", "TestACutRefusalSaysThatItWasCut", `"), named but not counted: " + shell.CutBytesMarked(shell.Oneline(name), findingNameCap))`, `"), named but not counted: " + shell.CutBytes(shell.Oneline(name), findingNameCap))`},
	{"citations: uncheckable head cut without a mark", "citations.go", "./eco-check/", "TestAnUncheckableCitationSaysWhenItsHeadWasCut", "shell.CutBytesMarked(shell.Oneline(cited.head), 60)", "shell.CutBytes(shell.Oneline(cited.head), 60)"},
	{"subcommands: unread dispatch path cut without a mark", "subcommands.go", "./eco-check/", "TestAnUnreadableDispatchPathSaysItWasCut", "shell.CutBytesMarked(shell.Oneline(dir), 120)", "shell.CutBytes(shell.Oneline(dir), 120)"},
	// The subcommand name the same file's three kind-first findings lead with, one mutant per site. The
	// latter two carry the kind string, or each anchor also matches the other and preflight refuses the
	// run. Reverting one leaves the finding firing with a name that reads whole. The path it cut off the
	// end is the half the reader needs.
	{"subcommands: a cut call-site name left unmarked", "subcommands.go", "./eco-check/", "TestALongSubcommandNameIsCutBeforeItsAttribution", "shell.CutBytesMarked(shell.Oneline(site.subcommand), findingNameCap)", "shell.CutBytes(shell.Oneline(site.subcommand), findingNameCap)"},
	{"subcommands: a cut usage-gap name left unmarked", "subcommands.go", "./eco-check/", "TestALongSubcommandNameIsCutBeforeItsAttribution", "subcommandUsageDoesNotName + shell.CutBytesMarked(shell.Oneline(name), findingNameCap)", "subcommandUsageDoesNotName + shell.CutBytes(shell.Oneline(name), findingNameCap)"},
	{"subcommands: a cut dispatch-gap name left unmarked", "subcommands.go", "./eco-check/", "TestALongSubcommandNameIsCutBeforeItsAttribution", "subcommandDispatchDoesNotAccept + shell.CutBytesMarked(shell.Oneline(name), findingNameCap)", "subcommandDispatchDoesNotAccept + shell.CutBytes(shell.Oneline(name), findingNameCap)"},
	// The region name the three shared-region findings lead with. One mutant, not three: the cut is one
	// call above the switch. Every arm reads the same `named`, so one revert reaches all of them.
	{"scripts: a cut region name left unmarked", "scripts.go", "./eco-check/", "TestALongSharedRegionNameIsCutBeforeItsDetail", "named := shell.CutBytesMarked(name, findingNameCap)", "named := shell.CutBytes(name, findingNameCap)"},
	// The bound itself, which the mark above says nothing about: removed, the name runs to the printer's
	// own 500-byte cut and takes the detail after it off the line.
	{"scripts: the region-name bound removed", "scripts.go", "./eco-check/", "TestALongSharedRegionNameIsCutBeforeItsDetail", "named := shell.CutBytesMarked(name, findingNameCap)", "named := shell.CutBytesMarked(name, 100000)"},
	// The printer's own width bound, which runs after every one of the marks above and would otherwise
	// take one off with the tail it removes.
	{"report: a finding line cut without a mark", "report.go", "./eco-check/", "TestACutFindingLineSaysThatItWasCut", "shell.CutBytesMarked(line.text, lineWidthCap)", "shell.CutBytes(line.text, lineWidthCap)"},
	// The third mutant on this one line, and the anchor carries the WriteString around it: the bare
	// call is the anchor of the two above, and an anchor matching twice is refused.
	{"imports: uncounted name cut without a mark", "../eco-root/imports.go", "./eco-check/", "TestACutUncountedNameSaysThatItWasCut", "joined.WriteString(shell.CutBytesMarked(shell.Oneline(name), 60))", "joined.WriteString(shell.CutBytes(shell.Oneline(name), 60))"},
	// The list the names are joined into, which is cut at its own bound and reads as whole just as
	// readily. It shares its anchor with `imports: the uncounted list not capped in bytes`, which asks
	// a different question about that call: that one removes the bound, this one removes only the word
	// about it.
	{"imports: uncounted list cut without a mark", "../eco-root/imports.go", "./eco-check/", "TestACutUncountedListSaysThatItWasCut", "shell.CutBytesMarked(joined.String(), 200)", "shell.CutBytes(joined.String(), 200)"},

	// ecostats' own skip note, both directions: only the pair makes absence of that line readable as
	// "the scan ran and counted none". Its gate's mutant is `stats: mounted-outside gate removed`, below.
	{"stats: a skipped mount scan reported as a measured zero", "../eco-stats/report.go", "./eco-stats/", "TestASkillMountedFromOutsideTheTreeIsReportedApart", "if !s.outsideMeasured {", "if !s.outsideMeasured && false {"},
	{"stats: a measured figure reported as not measured", "../eco-stats/report.go", "./eco-stats/", "TestASkillMountedFromOutsideTheTreeIsReportedApart", "if !s.outsideMeasured {", "if !s.outsideMeasured || true {"},
	// The two states the flag cannot tell apart on its own. A mount that will not list must not reach
	// the silence that now means "ran and counted none", and a measured zero must not grow a row.
	{"stats: an unlistable skills mount published as a measured zero", "../eco-stats/budget.go", "./eco-stats/", "TestASkillMountedFromOutsideTheTreeIsReportedApart", "if _, err := os.ReadDir(mount); err != nil && !errors.Is(err, fs.ErrNotExist) {", "if _, err := os.ReadDir(mount); err != nil && !errors.Is(err, fs.ErrNotExist) && false {"},
	{"stats: a measured zero printed as a row", "../eco-stats/report.go", "./eco-stats/", "TestASkillMountedFromOutsideTheTreeIsReportedApart", "if s.outsideSkills == 0 {", "if s.outsideSkills == 0 && false {"},
	// Two mutants for the location itself, because it is carried by two things: the read the scan uses,
	// and the count that read feeds.
	{"stats: an outside path reported as one under the root", "../eco-stats/budget.go", "./eco-stats/", "TestASkillMountedFromOutsideTheTreeIsReportedApart", "lines := s.readOutsideLines(file, errOut)", "lines := s.readTreeLines(file, errOut)"},
	{"stats: the outside-path count never rises", "../eco-stats/measure.go", "./eco-stats/", "TestASkillMountedFromOutsideTheTreeIsReportedApart", "s.unreadableOutside += s.unreadable - before", "s.unreadableOutside += (s.unreadable - before) * 0"},

	// The `@import` guard family. What the scan reads an import out of, and what the mount will
	// resolve one to, are the two halves: a budget blind to a resolved import under-reports the tier
	// it exists to measure, and a mount that resolves a shape nothing legitimate produces reads a
	// file the tree pointed it at.
	{"imports: a fenced mention counted as an import", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", "if shell.IsFenceDelimiter(line) {", "if false {"},
	{"imports: a backticked mention counted as an import", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", `backtickSpan.ReplaceAllString(line, " ")`, "line"},
	{"imports: the non-word boundary before the @ dropped", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", "`[^A-Za-z0-9_]@[~A-Za-z0-9._/-]+\\.[A-Za-z0-9]+`", "`@[~A-Za-z0-9._/-]+\\.[A-Za-z0-9]+`"},
	{"imports: the field's leading-space prefix dropped", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", `token := " " + field`, "token := field"},
	{"imports: resolution not gated on the installed checkout", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", `if !m.isInstalled || name == "" {`, "if false {"},
	{"imports: a traversal refused without a word", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", `return "", "a traversal, not a bare filename"`, `return "", ""`},
	{"imports: a subdirectory import resolved anyway", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", `case strings.Contains(name, "/"):`, "case false:"},
	{"imports: a name this carrier never declared resolved", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", "if !m.declared[name] {", "if false {"},
	{"imports: a symlink at the mount followed", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", "if shell.IsSymlink(mounted) {", "if false {"},
	{"imports: an unreadable file at the mount counted", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", "if !isReadable(mounted) {", "if false {"},
	{"imports: resolution attempt cap removed", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", "if attempts < 64 {", "if attempts < 100000 {"},

	// The stats side. ecostats' suite holds both the case for the refusal and the case that the other
	// tool refuses it too.
	{"contained-in-root: readability test removed", "../eco-root/contained.go", "./eco-stats/", "", " || !isReadable(path) {", " {"},
	// `* 0` rather than a bare 0, or `words` goes unused and the mutant does not compile: a `broken`
	// verdict says nothing about the guard.
	{"ecocheck: budget words not counted", "budget.go", "./eco-stats/", "", "budgetWords += words", "budgetWords += words * 0"},
	// To the end of the line, or the anchor also matches the three `+= wordsInFile(…)` above it.
	{"stats: resolved import contributes nothing", "../eco-stats/budget.go", "./eco-stats/", "", "s.alwaysLoadedWords += words\n", "s.alwaysLoadedWords += 0\n"},
	// The collapse follows the guard wherever it moves, and this anchor has gone stale twice already —
	// preflight refuses the whole run over one such entry. The guard now sits in isControlChar: the C0
	// term is what the mutant drops, and the C1 term below it keeps `char` read so the mutant builds.
	{"stats: no newline collapse in the note", "../shell/text.go", "./eco-stats/", "TestTheNoteCannotForgeALedgerRow", "char < 0x20 || char == 0x7f", "char == 0x7f"},
	{"stats: no pipe escaping in the note", "../eco-stats/eco-stats.go", "./eco-stats/", "", "strings.ReplaceAll(note, \"|\", `\\|`)", "strings.ReplaceAll(note, \"|\", \"|\")"},
	{"stats: no note-length bar", "../eco-stats/eco-stats.go", "./eco-stats/", "", "words > noteWordCap", "words > 100000"},
	{"stats: import refusals unreported", "../eco-stats/budget.go", "./eco-stats/", "", `fmt.Fprintf(errOut, "stats.sh: import refused`, `fmt.Fprintf(io.Discard, "stats.sh: import refused`},
	// The name and the path both, because the path is built from the name: sanitising one and printing
	// the other through would leave the ESC byte on the line anyway.
	{"stats: Read-always target left unsanitised", "../eco-stats/budget.go", "./eco-stats/", "TestAMissingReadAlwaysTargetCannotReachTheTerminalRaw", "shell.Oneline(target), shell.Oneline(file))", "target, file)"},
	{"stats: ledger not taken out of prose", "../eco-stats/measure.go", "./eco-stats/", "", "s.prose -= s.ledgerWords", "s.prose -= 0"},
	{"stats: ledger figure unreported", "../eco-stats/report.go", "./eco-stats/", "", `fmt.Fprintf(out, "ledger:`, `fmt.Fprintf(io.Discard, "ledger:`},
	// Anchored through the format verb, because the row has a sibling: the line saying the figure was
	// not measured at all opens with the same label, and an anchor matching both refuses the run.
	{"stats: mounted-outside unreported", "../eco-stats/report.go", "./eco-stats/", "", `fmt.Fprintf(out, "mounted outside:%4d words`, `fmt.Fprintf(io.Discard, "mounted outside:%4d words`},
	{"stats: mounted-outside gate removed", "../eco-stats/budget.go", "./eco-stats/", "", "if !s.root.IsInstalled() {", "if false {"},
	// The same gate, in the other tool. A clone, a worktree or a CI runner mounts none of this tree, so
	// every mount finding there restates that rather than saying anything about the tree under review.
	{"mounts: installed gate removed", "mounts.go", "./eco-check/", "TestTheMountScanAsksOnlyAboutTheInstalledCheckout", "if !c.root.IsInstalled() {", "if false {"},
	{"stats: in-tree mounts not excluded", "../eco-stats/budget.go", "./eco-stats/", "", "if s.root.HoldsSkillFile(file) {", "if false {"},
	{"stats: ledger symlink followed on write", "../eco-stats/ledger.go", "./eco-stats/", "", "if shell.IsSymlink(history) {", "if false {"},
	{"stats: fresh ledger loses the + legend", "../eco-stats/ledger.go", "./eco-stats/", "", "makes it a lower bound", "makes it a lower limit"},
	// Nothing but the seed-versus-live case notices: this text is written only where there is no
	// ledger yet.
	{"stats: fresh ledger loses the measurement absolute", "../eco-stats/ledger.go", "./eco-stats/", "", "never edited — however that edit is authorised", "never edited"},
	{"stats: fresh ledger loses its columns", "../eco-stats/ledger.go", "./eco-stats/", "", "| date | prose | scripts | always-loaded | skills | what ran |", "| date | prose | scripts | always-loaded | skills |"},

	// The root both tools resolve through. Neither tool's suite reaches it: every fixture there names
	// its root outright, so a candidate dropped from the list goes unnoticed in both.
	{"ecoroot: the ./ai candidate dropped", "../eco-root/eco-root.go", "./eco-root/", "", `var candidates = []string{".", "./ai"}`, `var candidates = []string{"."}`},
	{"ecoroot: a root needs only one of the two directories", "../eco-root/eco-root.go", "./eco-root/", "", "&& shell.IsDir(shell.Join(dir, skillsDir))", ""},

	// ecoreport. This tool deletes files (`discard`) and writes the human's index (`promote`), and the
	// stamp it writes is what the merge gate trusts — so the weight below is on the three questions
	// where being wrong is unrecoverable: which report an invocation resolves to, what `discard`
	// decides is this ship's scratch and not the human's, and what the stamp will accept as a review
	// that happened. Naming the test here buys attribution rather than budget: "some case went red" would
	// not say which guard is observed.

	// paths.go — which report an invocation acts on, and what else on disk belongs to the ship it
	// names.
	//
	// Observable from resolveReport alone: `init` trims the value before it calls this, so the case is
	// the one naming the ship the way `init` was given it, leading whitespace and all.
	{"report name: the leading-space trim dropped", "../eco-report/paths.go", "./eco-report/", "TestTheFilenameAndTheFrontmatterNameTheSameShip", "firstField(trimLeadingSpace(value))", "firstField(value)"},
	{"report name: a standalone review has no stem of its own", "../eco-report/paths.go", "./eco-report/", "TestAnExistingReportIsNotSilentlyReplaced", `case slug == "" || strings.HasPrefix(slug, "review:"):`, `case slug == "":`},
	// Both halves of the one arm that keeps a value out of a path: the dot that no listing can see,
	// and the charset that stops a `/` reaching one.
	{"report name: a leading dot no longer refused", "../eco-report/paths.go", "./eco-report/", "TestAnIntentValueCannotNameAFileOutsideQualifyReports", `case strings.HasPrefix(slug, "."), !isSlugCharset(slug):`, `case !isSlugCharset(slug):`},
	{"report name: the slug charset no longer refused", "../eco-report/paths.go", "./eco-report/", "TestAnIntentValueCannotNameAFileOutsideQualifyReports", `case strings.HasPrefix(slug, "."), !isSlugCharset(slug):`, `case strings.HasPrefix(slug, "."):`},
	{"report listing: a dot-named report joins the listing", "../eco-report/paths.go", "./eco-report/", "TestADotNamedReportIsInvisibleToEveryDiscoveryPath", `if strings.HasPrefix(name, ".") || !entry.IsDir() {`, "if !entry.IsDir() {"},
	{"report listing: a directory counted as a report", "../eco-report/paths.go", "./eco-report/", "TestADotNamedReportIsInvisibleToEveryDiscoveryPath", `if !shell.IsRegularFile(r.shipDir(name) + "/" + reportName) {`, "if false {"},
	{"resolve: several open reports resolved to one", "../eco-report/paths.go", "./eco-report/", "TestStateNeverAnswersATokenItCannotStandBehind", "if len(names) != 1 {", "if false {"},
	{"resolve: a name that could name no report resolved anyway", "../eco-report/paths.go", "./eco-report/", "TestASubcommandRefusesAnIntentNameThatCouldNameNoReport", `if stem == "" {`, "if false {"},
	{"require report: a named report that is not there read anyway", "../eco-report/paths.go", "./eco-report/", "TestANamedReportThatIsNotThereIsRefusedRatherThanRead", "if !shell.IsRegularFile(r.report) {", "if false {"},
	{"readable: an unreadable report read as one that is there", "../eco-report/paths.go", "./eco-report/", "TestAnUnreadableReportIsNotAState", "if !isReadable(r.report) {", "if false {"},
	// The ship-exists guard, then each of the three things that satisfy it. Removed whole, `discard
	// <any-legal-slug>` deletes at exit 0 in a repo that never used idsd.
	{"discard: the ship-exists guard never refuses", "../eco-report/paths.go", "./eco-report/", "TestDiscardDeletesNothingForAShipThatIsNotHere", "func (r *run) assertShipExists(slug string) {\n\tif shell.IsRegularFile(r.report) {", "func (r *run) assertShipExists(slug string) {\n\tif true {"},
	{"discard: an intent file no longer identifies a closed ship", "../eco-report/paths.go", "./eco-report/", "TestDiscardDeletesNothingForAShipThatIsNotHere", `if shell.IsRegularFile(r.shipDir(slug)+"/"+intentName) || shell.IsRegularFile(r.archiveDir(slug)+"/"+intentName) {`, "if false {"},
	{"discard: the review exception removed", "../eco-report/paths.go", "./eco-report/", "TestAStandaloneReviewCanStillBeTornDownAfterItIsClosed", `if slug == "review" {`, "if false {"},
	// The durable files are a table, and a table gets a row each: each one kept .idsd/ standing
	// with nothing observing that it did.
	{"surviving: charter.md no longer keeps .idsd/", "../eco-report/paths.go", "./eco-report/", "TestDiscardDestructivePath", `[]string{"charter.md", "constraints.md", "language.md", "playbook.md"}`, `[]string{"constraints.md", "language.md", "playbook.md"}`},
	{"surviving: constraints.md no longer keeps .idsd/", "../eco-report/paths.go", "./eco-report/", "TestEveryDurableFileKeepsIdsdStanding", `[]string{"charter.md", "constraints.md", "language.md", "playbook.md"}`, `[]string{"charter.md", "language.md", "playbook.md"}`},
	{"surviving: language.md no longer keeps .idsd/", "../eco-report/paths.go", "./eco-report/", "TestEveryDurableFileKeepsIdsdStanding", `[]string{"charter.md", "constraints.md", "language.md", "playbook.md"}`, `[]string{"charter.md", "constraints.md", "playbook.md"}`},
	{"surviving: playbook.md no longer keeps .idsd/", "../eco-report/paths.go", "./eco-report/", "TestEveryDurableFileKeepsIdsdStanding", `[]string{"charter.md", "constraints.md", "language.md", "playbook.md"}`, `[]string{"charter.md", "constraints.md", "language.md"}`},
	{"surviving: a parallel ship's report no longer counted", "../eco-report/paths.go", "./eco-report/", "TestDiscardDestructivePath", "if left := len(r.reportNames()); left != 0 {", "if left := len(r.reportNames()); left < 0 {"},
	{"surviving: another ship's intent file no longer counted", "../eco-report/paths.go", "./eco-report/", "TestEveryDurableFileKeepsIdsdStanding", "if shell.PathExists(intents) || shell.PathExists(archive) {", "if false {"},
	{"surviving: stray content counted as intents", "../eco-report/paths.go", "./eco-report/", "TestEveryDurableFileKeepsIdsdStanding", "if left := countShipFolders(intents, archive); left > 0 {", "if left := countShipFolders(intents, archive); left >= 0 {"},
	{"ship count: a plain file counted as a ship folder", "../eco-report/paths.go", "./eco-report/", "TestEveryDurableFileKeepsIdsdStanding", `if !entry.IsDir() || !shell.IsRegularFile(dir+"/"+entry.Name()+"/"+intentName) {`, `if !shell.IsRegularFile(dir+"/"+entry.Name()+"/"+intentName) {`},
	{"ship count: a folder holding no intent counted as one", "../eco-report/paths.go", "./eco-report/", "TestEveryDurableFileKeepsIdsdStanding", `if !entry.IsDir() || !shell.IsRegularFile(dir+"/"+entry.Name()+"/"+intentName) {`, "if !entry.IsDir() {"},
	// A symlink test reads the final component only, so all three of the write's path components need
	// one, and each has its own case.
	{"write paths: .idsd no longer tested for a link", "../eco-report/paths.go", "./eco-report/", "TestDiscardRefusesASymlinkedIdsdRatherThanDeletingThroughIt", `[]string{r.idsdDir, r.intentsDir}`, `[]string{r.intentsDir}`},
	// git refuses any pathspec beyond a symbolic link, so with this path untested the ignore check
	// refuses instead and the exit alone cannot tell the two apart. The case is the one asserting
	// which refusal it is — the other names a remedy that reports ok and leaves the link standing.
	{"write paths: intents/ no longer tested for a link", "../eco-report/paths.go", "./eco-report/", "TestInitRefusesRatherThanWritingThroughALink", `[]string{r.idsdDir, r.intentsDir}`, `[]string{r.idsdDir}`},
	{"write paths: the report itself no longer tested for a link", "../eco-report/paths.go", "./eco-report/", "TestInitRefusesRatherThanWritingThroughALink", "if shell.IsSymlink(r.report) {", "if false {"},
	{"stage markers: not keyed by the report stem", "../eco-report/paths.go", "./eco-report/", "TestTwoIntentsShipSideBySide", `r.gitPath("idsd-stage-returns/" + name)`, `r.gitPath("idsd-stage-returns")`},

	// frontmatter.go — the three lines every later reader greps, and the rewrites that write them.
	{"frontmatter: a body line read as a field", "../eco-report/frontmatter.go", "./eco-report/", "TestAStampRewritesTheFrontmatterAndNothingElse", "if strings.HasPrefix(line, prefix) {", "if strings.Contains(line, prefix) {"},
	// One set, read whole by every reader. A reader knowing only some of it accepts what the others
	// reject, which is a report reading as reviewed to `gate` and unreviewed to `state`.
	{"unstamped: 'pending' reads as a completed review", "../eco-report/frontmatter.go", "./eco-report/", "TestTwoIntentsShipSideBySide", `case "", "pending", "<hash>", "<stages>", "<worktree>":`, `case "", "<hash>", "<stages>", "<worktree>":`},
	{"unstamped: the template's <hash> reads as a completed review", "../eco-report/frontmatter.go", "./eco-report/", "TestTheFilenameAndTheFrontmatterNameTheSameShip", `case "", "pending", "<hash>", "<stages>", "<worktree>":`, `case "", "pending", "<stages>", "<worktree>":`},
	{"unstamped: the template's <stages> reads as a stage record", "../eco-report/frontmatter.go", "./eco-report/", "TestGateBlocksOnEachOfItsReasonsAndClearsOnNone", `case "", "pending", "<hash>", "<stages>", "<worktree>":`, `case "", "pending", "<hash>", "<worktree>":`},
	{"unstamped: the template's <worktree> reads as a completed review", "../eco-report/frontmatter.go", "./eco-report/", "TestADriftedTemplateIsRefusedBeforeAnyReportIsScaffolded", `case "", "pending", "<hash>", "<stages>", "<worktree>":`, `case "", "pending", "<hash>", "<stages>":`},
	{"unstamped: an absent field reads as a completed review", "../eco-report/frontmatter.go", "./eco-report/", "TestGateBlocksOnEachOfItsReasonsAndClearsOnNone", `case "", "pending", "<hash>", "<stages>", "<worktree>":`, `case "pending", "<hash>", "<stages>", "<worktree>":`},
	{"turnaround trims: a turnaround trim no longer trims", "../eco-report/frontmatter.go", "./eco-report/", "TestATrimmedPassIsNotAFullOne", `strings.Contains(entry, "(turnaround)")`, `strings.Contains(entry, "(TURNAROUND)")`},
	{"intent slug: the charset no longer bounds the slug", "../eco-report/frontmatter.go", "./eco-report/", "TestAHandEditedIntentCannotSteerAPathOutOfIdsd", `if slug == "" || strings.HasPrefix(slug, "review:") || !isSlugCharset(slug) {`, `if slug == "" || strings.HasPrefix(slug, "review:") {`},
	{"template: a symlinked template read", "../eco-report/frontmatter.go", "./eco-report/", "TestADriftedTemplateIsRefusedBeforeAnyReportIsScaffolded", "if shell.IsSymlink(r.template) {", "if false {"},
	{"template: a missing template not named as the cause", "../eco-report/frontmatter.go", "./eco-report/", "TestADriftedTemplateIsRefusedBeforeAnyReportIsScaffolded", "if !shell.IsRegularFile(r.template) {", "if false {"},
	{"template: no intent: line to stamp", "../eco-report/frontmatter.go", "./eco-report/", "TestADriftedTemplateIsRefusedBeforeAnyReportIsScaffolded", `if !hasField(r.template, "intent") {`, "if false {"},
	{"template: reviewed-tree no longer required", "../eco-report/frontmatter.go", "./eco-report/", "TestADriftedTemplateIsRefusedBeforeAnyReportIsScaffolded", `[]string{"reviewed-tree", "reviewed-worktree", "reviewed-stages"}`, `[]string{"reviewed-worktree", "reviewed-stages"}`},
	{"template: reviewed-worktree no longer required", "../eco-report/frontmatter.go", "./eco-report/", "TestADriftedTemplateIsRefusedBeforeAnyReportIsScaffolded", `[]string{"reviewed-tree", "reviewed-worktree", "reviewed-stages"}`, `[]string{"reviewed-tree", "reviewed-stages"}`},
	{"template: reviewed-stages no longer required", "../eco-report/frontmatter.go", "./eco-report/", "TestADriftedTemplateIsRefusedBeforeAnyReportIsScaffolded", `[]string{"reviewed-tree", "reviewed-worktree", "reviewed-stages"}`, `[]string{"reviewed-tree", "reviewed-worktree"}`},
	{"template: a drifted placeholder accepted", "../eco-report/frontmatter.go", "./eco-report/", "TestADriftedTemplateIsRefusedBeforeAnyReportIsScaffolded", "if !isUnstamped(placeholder) {", "if false {"},
	{"frontmatter map: the closing delimiter never read", "../eco-report/frontmatter.go", "./eco-report/", "TestAStampRewritesTheFrontmatterAndNothingElse", "if i > 0 && shell.IsFrontmatterDelimiter(line) {", "if i > 0 && false {"},
	{"frontmatter map: the frontmatter never opens", "../eco-report/frontmatter.go", "./eco-report/", "TestAStampCannotOutliveThePassThatEarnedIt", "if i == 0 {", "if false {"},
	{"intent rewrite: every intent: line replaced, not the first", "../eco-report/frontmatter.go", "./eco-report/", "TestAStampRewritesTheFrontmatterAndNothingElse", "replaced = true", "replaced = false"},
	{"stamp rewrite: the old reviewed-stages line left standing", "../eco-report/frontmatter.go", "./eco-report/", "TestAStampRewritesTheFrontmatterAndNothingElse", "case strings.HasPrefix(line, \"reviewed-mode:\"), strings.HasPrefix(line, \"reviewed-stages:\"),\n\t\t\t\tstrings.HasPrefix(line, \"reviewed-worktree:\"):", "case strings.HasPrefix(line, \"reviewed-mode:\"),\n\t\t\t\tstrings.HasPrefix(line, \"reviewed-worktree:\"):"},
	{"stamp rewrite: an old layout's reviewed-mode left to be read", "../eco-report/frontmatter.go", "./eco-report/", "TestAStampRewritesTheFrontmatterAndNothingElse", "case strings.HasPrefix(line, \"reviewed-mode:\"), strings.HasPrefix(line, \"reviewed-stages:\"),\n\t\t\t\tstrings.HasPrefix(line, \"reviewed-worktree:\"):", "case strings.HasPrefix(line, \"reviewed-stages:\"),\n\t\t\t\tstrings.HasPrefix(line, \"reviewed-worktree:\"):"},
	{"invalidate: the stage record left stamped", "../eco-report/frontmatter.go", "./eco-report/", "TestInvalidateClearsThePassItStarts", "case strings.HasPrefix(line, \"reviewed-stages:\"):\n\t\t\treturn []string{\"reviewed-stages: pending\"}", "case strings.HasPrefix(line, \"reviewed-stages:\"):\n\t\t\treturn []string{line}"},

	{"ignore source: a machine-local info/exclude counted as ignoring", "../eco-report/git.go", "./eco-report/", "TestAMachineLocalExcludeDoesNotCountAsIgnoringTheReport", "case source == \".git/info/exclude\" || strings.HasSuffix(source, \"/.git/info/exclude\"):\n\t\treturn source, false", "case source == \".git/info/exclude\" || strings.HasSuffix(source, \"/.git/info/exclude\"):\n\t\treturn source, true"},

	{"gate: a sibling worktree's stamp gates clean", "../eco-report/worktree.go", "./eco-report/", "TestASiblingWorktreeCannotReadAStampItNeverEarned", "case recorded != mine:", "case recorded != mine \u0026\u0026 false:"},
	{"gate: an unstamped block never names an unestablishable identity", "../eco-report/gate.go", "./eco-report/", "TestAnIdentityThatCannotBeEstablishedIsNotAnIdentity", "if _, established := r.worktreeToken(); !established {", "if _, established := r.worktreeToken(); !established \u0026\u0026 false {"},
	{"gate: an unestablished identity reads as a match", "../eco-report/worktree.go", "./eco-report/", "TestAnIdentityThatCannotBeEstablishedIsNotAnIdentity", "case !established:", "case !established \u0026\u0026 false:"},
	// One mutant over the whole guard, where three finer ones stood. `state`'s three limbs cover each
	// other: an unestablished identity answers `mine` empty and a recorded token is never empty, and a
	// recorded value that is not a token is never equal to a token either — so removing any one limb
	// changes no answer, and the two that dropped a limb did not even compile, leaving `mine` or
	// `established` unused. `broken` and `KILLED NOTHING` are both findings here, and neither said
	// anything about the guard. This one removes all of it, which is the smallest edit the suite can see.
	{"state: the worktree guard gone, so a sibling routes to ready", "../eco-report/gate.go", "./eco-report/", "TestASiblingWorktreeCannotReadAStampItNeverEarned", "\tif vouch, _ := r.worktreeVouch(); vouch != vouchesForThisWorktree {\n\t\treturn \"re-qualify\"\n\t}\n", ""},
	{"stamp: an identity it could not establish is recorded anyway", "../eco-report/stamp.go", "./eco-report/", "TestAnIdentityThatCannotBeEstablishedIsNotAnIdentity", "if !established {", "if !established \u0026\u0026 false {"},
	{"worktree identity: the token shape is trusted rather than checked", "../eco-report/worktree.go", "./eco-report/", "TestAnIdentityThatCannotBeEstablishedIsNotAnIdentity", "if token := firstField(string(content)); isWorktreeToken(token) {", "if token := firstField(string(content)); token != \"\" {"},
	{"stamp: the reviewing worktree is not recorded", "../eco-report/frontmatter.go", "./eco-report/", "TestTheStampRecordsWhichWorktreeReviewedTheTree", `return []string{"reviewed-tree: " + tree, "reviewed-worktree: " + worktree, "reviewed-stages: " + entries}`, `return []string{"reviewed-tree: " + tree, "reviewed-stages: " + entries}`},
	{"invalidate: the reviewing worktree survives an invalidate", "../eco-report/frontmatter.go", "./eco-report/", "TestTheStampRecordsWhichWorktreeReviewedTheTree", "case strings.HasPrefix(line, \"reviewed-worktree:\"):\n\t\t\treturn []string{\"reviewed-worktree: pending\"}", "case strings.HasPrefix(line, \"reviewed-worktree:\") \u0026\u0026 false:\n\t\t\treturn []string{\"reviewed-worktree: pending\"}"},
	{"worktree identity: the path is compared instead of the token", "../eco-report/worktree.go", "./eco-report/", "TestAWorktreeIdentityIsNotItsPath", "func (r *run) reviewedWorktreeToken() string {\n\treturn firstField(fieldValue(r.report, \"reviewed-worktree\"))", "func (r *run) reviewedWorktreeToken() string {\n\tfields := shell.SplitFields(fieldValue(r.report, \"reviewed-worktree\"))\n\tif len(fields) == 0 {\n\t\treturn \"\"\n\t}\n\treturn fields[len(fields)-1]"},
	{"worktree identity: the token is minted fresh on every read", "../eco-report/worktree.go", "./eco-report/", "TestAWorktreeIdentityIsNotItsPath", "if content, err := os.ReadFile(path); err == nil {", "if content, err := os.ReadFile(path); err != nil {"},
	{"worktree identity: the token is not persisted, so a move loses it", "../eco-report/worktree.go", "./eco-report/", "TestAWorktreeIdentityIsNotItsPath", `if err := os.WriteFile(path, []byte(token+"\n"), 0o666); err != nil {`, "if err := error(nil); err != nil {"},

	{"gate: a stamped tree with no reviewing worktree gates clean", "../eco-report/worktree.go", "./eco-report/", "TestAStampedTreeWithNoReviewingWorktreeIsNotAReview", "case !isWorktreeToken(recorded) \u0026\u0026 !isUnstamped(r.reviewedTree()):", "case false:"},
	{"promote: the target check counts entries, not files", "../eco-report/scratch.go", "./eco-report/", "TestPromoteCountsFilesNotDirectoryEntries", "count, sample, err := filesUnder(target)", "entries, _ := os.ReadDir(target)\n\tcount, sample, err := len(entries), []string(nil), error(nil)"},
	{"reconcile: a nested symlink counts as nothing and is deleted", "../eco-report/shell.go", "./eco-report/", "TestANestedSymlinkIsNotSilentlyDeleted", "if entry.IsDir() {\n\t\t\treturn nil\n\t\t}\n\t\tcount++", "if entry.IsDir() || entry.Type() == fs.ModeSymlink {\n\t\t\treturn nil\n\t\t}\n\t\tcount++"},

	{"gate: the identical-trees claim is made unconditionally", "../eco-report/gate.go", "./eco-report/", "TestTheGateClaimsIdenticalTreesOnlyWhenTheyAre", "sameTree := \"\"\n\tif current == reviewed {", "sameTree := \"\"\n\tif true {"},

	{"override root: a symlinked root accepted", "../eco-report/root.go", "./eco-report/", "TestAnUntrustworthyOverrideRootIsRefused", "if shell.IsSymlink(root) {", "if shell.IsSymlink(root) \u0026\u0026 false {"},
	{"override root: a group-writable root accepted", "../eco-report/root.go", "./eco-report/", "TestAnUntrustworthyOverrideRootIsRefused", `if mode := info.Mode().Perm(); mode&0o022 != 0 {`, `if mode := info.Mode().Perm(); false {`},
	{"override root: an unreadable root passes as checked", "../eco-report/root.go", "./eco-report/", "TestAnUntrustworthyOverrideRootIsRefused", "if errors.Is(err, fs.ErrNotExist) {", "if errors.Is(err, fs.ErrNotExist) || true {"},

	{"override root: a writable ancestor accepted", "../eco-report/root.go", "./eco-report/", "TestAnUntrustworthyOverrideRootIsRefused", `for _, above := range directoriesAbove(root) {`, `for _, above := range []string(nil) {`},
	{"override root: the sticky exemption inverted, so a plain writable ancestor passes", "../eco-report/root.go", "./eco-report/", "TestAnUntrustworthyOverrideRootIsRefused", `return mode.Perm()&0o022 != 0 && mode&os.ModeSticky == 0`, `return mode.Perm()&0o022 != 0 && mode&os.ModeSticky != 0`},
	{"override config: a world-writable config accepted", "../eco-report/root.go", "./eco-report/", "TestTheOverrideConfigIsJudgedLikeTheRootItNames", `if info.Mode().Perm()&0o022 != 0 {`, `if false {`},
	{"override config: an unreadable config passes as a checked one", "../eco-report/root.go", "./eco-report/", "TestTheOverrideConfigIsJudgedLikeTheRootItNames", "\tinfo, err := os.Lstat(path)\n\tif err != nil {", "\tinfo, err := os.Lstat(path)\n\tif err != nil \u0026\u0026 false {"},
	{"override config: a symlinked config judged by its target", "../eco-report/root.go", "./eco-report/", "TestTheOverrideConfigIsJudgedLikeTheRootItNames", "	if shell.IsSymlink(path) {\n\t\tr.refuse(\"error: \"+shell.Oneline(path)+\" is a symlink", "	if false {\n\t\tr.refuse(\"error: \"+shell.Oneline(path)+\" is a symlink"},
	{"override config: a writable directory holding it accepted", "../eco-report/root.go", "./eco-report/", "TestTheOverrideConfigIsJudgedLikeTheRootItNames", `for _, above := range directoriesAbove(path) {`, `for _, above := range []string(nil) {`},

	// families.go — direction inside the skill layer. Each of these turns the scan into one that looks
	// like it works: it still runs, still reports nothing on a clean tree, and has stopped checking.
	{"families: the router exception swallows every any-repo skill", "families.go", "./eco-check/", "TestFamilyDirectionScan", "case name == familyRouter:", "case name == familyRouter || true:"},
	{"families: the scan runs in the permitted direction too", "families.go", "./eco-check/", "TestFamilyDirectionScan", "case strings.HasPrefix(name, workflowFamily):\n\t\t\tcontinue", "case strings.HasPrefix(name, workflowFamily) && false:\n\t\t\tcontinue"},
	{"families: the router keeps a blanket pass, claim or no claim", "families.go", "./eco-check/", "TestFamilyDirectionScan", "c.assertRouterClaimsItsException(name)", "_ = name"},
	{"families: a skill in neither family goes unreported", "families.go", "./eco-check/", "TestFamilyDirectionScan", "case !strings.HasPrefix(name, anyRepoFamily):", "case !strings.HasPrefix(name, anyRepoFamily) && false:"},
	{"families: only SKILL.md is read, so scripts go unchecked", "families.go", "./eco-check/", "TestFamilyDirectionScan", `c.filesNamed(dir, "*.md", "*.sh")`, `c.filesNamed(dir, "*.md")`},
	{"families: the state directory is no longer looked for", "families.go", "./eco-check/", "TestFamilyDirectionScan", "[]*regexp.Regexp{workflowName, stateDir}", "[]*regexp.Regexp{workflowName}"},

	// root.go — where the throwaway scratch lives. Every mutant here turns the fix back into the defect
	// it replaced, and each one still reports success on every command, which is why they are worth
	// pinning: the failure mode being guarded is silence, not an error.
	// These three sit on the `rev-parse` fallback under layout.go, which every other fixture in the
	// package skips by reading the filesystem instead. They name the case that switches the reader
	// off rather than the case that describes the property, because only that one reaches them.
	{"root: the scratch follows the worktree, not the clone", "../eco-report/root.go", "./eco-report/", "TestTheGitFallbackResolvesWhatTheLayoutReaderWould", `"rev-parse", "--git-common-dir"`, `"rev-parse", "--git-path", "."`},
	{"root: the shared git dir is left relative to the caller", "../eco-report/root.go", "./eco-report/", "TestTheGitFallbackResolvesWhatTheLayoutReaderWould", "if !filepath.IsAbs(path) {\n\t\tpath = r.root + \"/\" + path\n\t}", "if false {\n\t\tpath = r.root + \"/\" + path\n\t}"},
	{"root: the override key is built from the worktree's own name", "../eco-report/root.go", "./eco-report/", "TestAnOverrideKeyIsTheCloneNotTheWorktree", "name := shell.BaseName(shell.DirName(canonical))", "name := shell.BaseName(r.root)"},
	{"root: a broken override falls back to the default in silence", "../eco-report/root.go", "./eco-report/", "TestABrokenOverrideRefusesRatherThanFallingBack", "if root == \"\" {\n\t\tr.refuse(\"error: \"+path+\" sets no", "if false {\n\t\tr.refuse(\"error: \"+path+\" sets no"},
	{"root: an override inside the working tree is accepted", "../eco-report/root.go", "./eco-report/", "TestAnOverrideInsideTheWorkingTreeIsRefused", `if scratch != root && !strings.HasPrefix(scratch, root+"/") {`, "if true {"},
	{"root: an empty directory skeleton read as content", "../eco-report/shell.go", "./eco-report/", "TestAnInTreeScratchDirectoryIsNeverMigratedSilently", "if entry.IsDir() {\n\t\t\treturn nil\n\t\t}", "if entry.IsDir() {\n\t\t\tcount++\n\t\t\treturn nil\n\t\t}"},
	{"root: an in-tree scratch dir is migrated without asking", "../eco-report/migrate.go", "./eco-report/", "TestAnInTreeScratchDirectoryIsNeverMigratedSilently", "if count == 0 {", "if count == 0 || true {"},
	{"promote: a refusal strands the scratch in the working tree", "../eco-report/scratch.go", "./eco-report/", "TestNoRefusalLeavesTheScratchStrandedInTheTree", "if err := os.Rename(to, from); err != nil {", "if err := error(nil); err != nil {"},

	// git.go — every git call the tool makes, and the two exclusion mechanisms it writes through.
	// Only observable where a write goes through the answer, so the case is `check-ignore` run *in* a
	// linked worktree: prefixed with the root, the absolute git dir names a tree inside the worktree,
	// and the exclusion lands there while git goes on ignoring nothing.
	{"git dir: an absolute git path prefixed with the root", "../eco-report/git.go", "./eco-report/", "TestTheGitFallbackResolvesWhatTheLayoutReaderWould", `if strings.HasPrefix(path, "/") {`, "if false {"},
	// Only observable where a write goes through the answer AND the layout resolver has declined, since
	// gitPath answers from the layout first and never reaches this arm for a linked worktree. So the
	// case sets an environment override and writes from a linked worktree: prefixed with the root, the
	// absolute git dir names a tree inside the worktree, and the marker lands there while the next
	// command looks where it should have been.
	{"repo mode: a tracked .idsd read as throwaway", "../eco-report/git.go", "./eco-report/", "TestDiscardDestructivePath", `if tracked != "" {`, `if tracked != "" && false {`},
	{"repo mode: an unreadable index read as a mode", "../eco-report/git.go", "./eco-report/", "TestPromoteAndCheckIgnoreAlsoRefuseAnUnreadableIndex", `if _, status := r.memoGit(nil, "ls-files", ".idsd"); status != 0 {`, "if false {"},
	// The arm order is load-bearing, so the two forms of info/exclude are asked separately: the
	// relative one an ordinary repo has, and the absolute one a linked worktree has.
	{"ignore source: a machine-local exclude counted as ignoring", "../eco-report/git.go", "./eco-report/", "TestAGlobalExcludeDoesNotCountAsIgnoringTheReport", "case strings.HasPrefix(source, \"/\"):\n\t\treturn source, false", "case strings.HasPrefix(source, \"/\"):\n\t\treturn source, true"},
	{"ignore source: .gitignore no longer travels", "../eco-report/git.go", "./eco-report/", "TestInitWillNotWriteAReportIntoItsOwnFingerprint", `case source == ".gitignore" || strings.HasSuffix(source, "/.gitignore"):`, "case false:"},
	{"ignore surface: the report's own entry dropped", "../eco-report/git.go", "./eco-report/", "TestAGitignoreEntryIsWrittenOnceAndNeverFusedOntoTheLastLine", `		".idsd/intents/*/" + reportName,
`, ""},
	// The guard the glob entries made necessary: `-v` reports a negation as a match, so without this a
	// `!` rule after the entry reads as "ignored, by a file that travels" and promote stages the report.
	{"ignore source: a negating pattern read as ignoring", "../eco-report/git.go", "./eco-report/", "TestPromoteWritesNoGitignoreThroughALink", "if matched, _, _ := strings.Cut(pattern, \"\\t\"); strings.HasPrefix(matched, \"!\") {\n\t\treturn \"\"\n\t}", "_ = pattern"},
	{"append: the same entry added twice", "../eco-report/git.go", "./eco-report/", "TestAGitignoreEntryIsWrittenOnceAndNeverFusedOntoTheLastLine", "if line == entry {\n\t\t\t\treturn nil", "if line == entry {\n\t\t\t\tbreak"},
	{"append: the entry fused onto an unterminated last line", "../eco-report/git.go", "./eco-report/", "TestAGitignoreEntryIsWrittenOnceAndNeverFusedOntoTheLastLine", "if isNonEmptyFile(file) && !endsWithNewline(file) {", "if false {"},

	// seams.go — the two scripts this tool calls rather than reimplements.
	{"todo scan: a scan that did not run read as nothing open", "../eco-report/seams.go", "./eco-report/", "TestAScanThatDidNotRunIsNeverReadAsNothingOpen", "func (r *run) readOpenTodos(consequence string) {\n\titems, status := r.runTodoGate()\n\tif status > 1 {", "func (r *run) readOpenTodos(consequence string) {\n\titems, status := r.runTodoGate()\n\tif false {"},
	{"fingerprint: a missing script recomputed locally", "../eco-report/seams.go", "./eco-report/", "TestAMissingFingerprintScriptRefusesInsteadOfRecomputing", "if !isExecutable(r.fingerprintBin) {", "if false {"},
	{"fingerprint: an empty tree read as a fingerprint", "../eco-report/seams.go", "./eco-report/", "TestListWalksTheTreeOnceAndNeverStreamsAPartialAnswer", `if err != nil || tree == "" {`, "if false {"},
	{"fingerprint: the walk repeated once per ship", "../eco-report/seams.go", "./eco-report/", "TestListWalksTheTreeOnceAndNeverStreamsAPartialAnswer", `if r.cachedTree != "" {`, "if false {"},

	// intent_ready.go — the build-blocker over the ICE, plus the yamlValue read the merge gate shares
	// with it. Lose a guard here and the tool reports something it did not check: an "intent ready" over
	// a placeholder nobody filled, or — for the two yamlValue mutants, which the merge gate's own case
	// notices — a block naming the wrong status, or a block over an intent its author approved.
	{"intent-ready: the placeholder scan disabled", "../eco-report/intent_ready.go", "./eco-report/", "TestIntentReadyClearsAFilledIceAndBlocksOnEachDefect", `if placeholder := firstPlaceholder(stripCodeSpans(line)); placeholder != "" {`, `if placeholder := firstPlaceholder(stripCodeSpans(line)); placeholder != "" && false {`},
	{"intent-ready: a comparison read as a placeholder", "../eco-report/intent_ready.go", "./eco-report/", "TestIntentReadyClearsAFilledIceAndBlocksOnEachDefect", `if body != "" && body[0] != ' ' && !strings.HasPrefix(body, "!--") {`, `if body != "" && !strings.HasPrefix(body, "!--") {`},
	{"intent-ready: a placeholder standing after a comparison is skipped", "../eco-report/intent_ready.go", "./eco-report/", "TestIntentReadyClearsAFilledIceAndBlocksOnEachDefect", "\t\tif strings.HasPrefix(body, \"!--\") {\n\t\t\tstart += end\n\t\t}\n", "\t\tstart += end\n"},
	{"intent-ready: an empty required section read as filled", "../eco-report/intent_ready.go", "./eco-report/", "TestIntentReadyClearsAFilledIceAndBlocksOnEachDefect", `if inSection && strings.TrimSpace(line) != "" {`, "if inSection {"},
	{"intent-ready: a blanked field's own comment read as its value", "../eco-report/intent_ready.go", "./eco-report/", "TestGateBlocksAnIntentTheGapRoundsNeverApproved", `if strings.HasPrefix(value, "#") {`, "if false {"},
	{"intent-ready: a filled field's trailing comment read as part of its value", "../eco-report/intent_ready.go", "./eco-report/", "TestGateBlocksAnIntentTheGapRoundsNeverApproved", `if comment := strings.Index(value, " #"); comment >= 0 {`, `if comment := strings.Index(value, " #"); false && comment >= 0 {`},
	{"intent-ready: an archived dependency read as unbuilt", "../eco-report/intent_ready.go", "./eco-report/", "TestIntentReadyBlocksOnADependencyThatHasNotShipped", `case where == "archive":`, `case where == "archive" && false:`},
	{"gate: the intent's own follow-ups never scanned", "../eco-report/gate.go", "./eco-report/", "TestGateScansTheShipsIntentFileAsWellAsItsReport", `if intent := r.intentFilePath(); intent != "" {`, `if intent := r.intentFilePath(); intent != "" && false {`},

	// records.go — the four records every worktree of the clone shares. Every guard here stands between
	// an agent's write and another agent's entries, and losing one leaves a well-formed file: no crash
	// and no diff, so a mutant that survives here is a defect nothing reports.
	{"records: a shared lock where the write needs an exclusive one", "../eco-report/records.go", "./eco-report/", "TestARecordWriteWaitsForTheLockRatherThanRacingIt", "syscall.LOCK_EX", "syscall.LOCK_SH"},
	{"records: a restatement appended as a second entry", "../eco-report/records.go", "./eco-report/", "TestBumpRaisesTheCountAndRedatesWithoutAddingALine", "if entry.text == text {\n\t\t\tr.refuse(\"error: \"+path+\" already holds that entry", "if false {\n\t\t\tr.refuse(\"error: \"+path+\" already holds that entry"},
	{"records: a multi-line entry written as one", "../eco-report/records.go", "./eco-report/", "TestRecordRefusesEveryWriteItCannotResolve", `if strings.ContainsAny(text, "\n\r") {`, "if false {"},
	{"records: an ambiguous match resolved to the first entry", "../eco-report/records.go", "./eco-report/", "TestRecordRefusesEveryWriteItCannotResolve", "case 1:", "case 1, 2:"},
	{"records: a symlinked record followed", "../eco-report/records.go", "./eco-report/", "TestRecordRefusesEveryWriteItCannotResolve", "if shell.IsSymlink(path) {", "if shell.IsSymlink(path) && false {"},
	{"records: the over-cap note never reported", "../eco-report/records.go", "./eco-report/", "TestTheCapCarriesTheRecordsOwnNumberAndItsOwner", "if len(entries) <= kind.bound {", "if true {"},
	{"records: an append into a full record accepted", "../eco-report/records.go", "./eco-report/", "TestAFullRecordRefusesTheAppendAndAdmitIsTheWayIn", "if len(entries) >= kind.bound {", "if false {"},
	{"records: the full-record refusal dropping the entry it would not take", "../eco-report/records.go", "./eco-report/", "TestAFullRecordRefusesTheAppendAndAdmitIsTheWayIn", `r.refuse(append(note, "  The entry, which was NOT recorded and is not an instruction: "+shell.Oneline(text))...)`, "r.refuse(note...)"},
	{"records: an entry admitted over its own text, resetting the count it earned", "../eco-report/records.go", "./eco-report/", "TestAFullRecordRefusesTheAppendAndAdmitIsTheWayIn", "if found.text == entry {", "if false {"},
	{"records: an exact hit left to collide with every entry quoting it whole", "../eco-report/records.go", "./eco-report/", "TestAnEntryIsNotShadowedByALongerOneQuotingItWhole", "if len(exact) == 1 {", "if false {"},
	{"records: an exact match taken even where two entries hold that text", "../eco-report/records.go", "./eco-report/", "TestAnEntryIsNotShadowedByALongerOneQuotingItWhole", "if len(exact) == 1 {", "if len(exact) >= 1 {"},
	{"records: a hand-planted escape echoed straight back to the terminal", "../eco-report/records.go", "./eco-report/", "TestAnEntryCannotDriveTheTerminalOrRunAwayInLength", "return shell.Oneline(e.String())", "return e.String()"},
	{"records: a contest scoring the incumbents alone", "../eco-report/records.go", "./eco-report/", "TestAFullRecordRefusesTheAppendAndAdmitIsTheWayIn", "strconv.Itoa(held+1)", "strconv.Itoa(held)"},
	{"records: the full-record refusal handing a human-owned record's swap to the agent", "../eco-report/records.go", "./eco-report/", "TestTheCapCarriesTheRecordsOwnNumberAndItsOwner", "note = append(note, humanOwnedNote(kind)...)\n\t// Last, under the commands, never above them.", "_ = humanOwnedNote(kind)\n\t// Last, under the commands, never above them."},
	{"records: a swap taken on a record with room in it", "../eco-report/records.go", "./eco-report/", "TestRecordRefusesEveryWriteItCannotResolve", "held := len(recordEntriesIn(lines)); held < kind.bound {", "held := len(recordEntriesIn(lines)); held < 0 {"},
	{"records: an admitted entry inheriting the reach of the one it displaced", "../eco-report/records.go", "./eco-report/", "TestAFullRecordRefusesTheAppendAndAdmitIsTheWayIn", "admitted := recordEntry{count: 1, date: today(), text: entry}", "admitted := recordEntry{count: found.count, date: today(), text: entry}"},
	{"records: a swap reported by its winner alone", "../eco-report/records.go", "./eco-report/", "TestAFullRecordRefusesTheAppendAndAdmitIsTheWayIn", `r.line("in place of: %s", found.quoted())`, "_ = found"},
	{"records: the over-cap note that names no way out of it", "../eco-report/records.go", "./eco-report/", "TestTheCapCarriesTheRecordsOwnNumberAndItsOwner", `"  " + capLadderRungs + ", and only then evict what the judge names.",`, `"",`},
	{"records: a record that never says who it is written for", "../eco-report/records.go", "./eco-report/", "TestFirstWriteCreatesTheRecordWithItsHeader", `"Written for the next agent — never presented to a human, and no human maintains it.\n" +`, `"" +`},
	{"records: a revision that resets the entry's reach", "../eco-report/records.go", "./eco-report/", "TestReviseReplacesTheTextAndKeepsTheCount", "revised := recordEntry{count: found.count, date: today(), text: replacement}", "revised := recordEntry{count: 1, date: today(), text: replacement}"},
	{"records: a revision onto a duplicate of another entry", "../eco-report/records.go", "./eco-report/", "TestReviseReplacesTheTextAndKeepsTheCount", "if entry.text == replacement {", "if false {"},
	{"records: the over-cap note handing a human-owned record's moves to the agent", "../eco-report/records.go", "./eco-report/", "TestTheCapCarriesTheRecordsOwnNumberAndItsOwner", "note = append(note, humanOwnedNote(kind)...)\n\tr.errLines(note...)", "_ = humanOwnedNote(kind)\n\tr.errLines(note...)"},
	{"records: every record's over-cap note quoting the decision log's", "../eco-report/records.go", "./eco-report/", "TestTheCapCarriesTheRecordsOwnNumberAndItsOwner", `strconv.Itoa(kind.bound) + " — every append refuses until it is back down.",`, `strconv.Itoa(decisionsBound) + " — every append refuses until it is back down.",`},
	{"records: every full-record refusal quoting the decision log's cap", "../eco-report/records.go", "./eco-report/", "TestTheCapCarriesTheRecordsOwnNumberAndItsOwner", `strconv.Itoa(held) + " entries, its cap.`, `strconv.Itoa(decisionsBound) + " entries, its cap.`},
	{"records: an entry appended onto an unterminated last line", "../eco-report/records.go", "./eco-report/", "TestAMutationNeverLosesALineItDidNotTarget", "} else if !ended {", "} else if !ended && false {"},
	{"records: a no-match refusal that never says the text is in the file", "../eco-report/records.go", "./eco-report/", "TestRecordRefusesEveryWriteItCannotResolve", "if strings.Contains(line, text) {", "if strings.Contains(line, text) && false {"},
	{"records: the not-an-entry hint fired for a line that is one", "../eco-report/records.go", "./eco-report/", "TestRecordRefusesEveryWriteItCannotResolve", "if _, isEntry := parseRecordEntry(i, line); isEntry {", "if _, isEntry := parseRecordEntry(i, line); isEntry && false {"},
	{"records: an escape sequence stored verbatim and echoed back", "../eco-report/records.go", "./eco-report/", "TestAnEntryCannotDriveTheTerminalOrRunAwayInLength", "text = shell.Oneline(text)", "_ = shell.Oneline(text)"},
	{"records: an entry of any length at all accepted", "../eco-report/records.go", "./eco-report/", "TestAnEntryCannotDriveTheTerminalOrRunAwayInLength", "if len(text) > entryBound {", "if false {"},
	{"records: a bump creating the record it could not find", "../eco-report/records.go", "./eco-report/", "TestOnlyAnAppendCreatesARecord", "flags := os.O_RDWR | syscall.O_NOFOLLOW", "flags := os.O_RDWR | syscall.O_NOFOLLOW | os.O_CREATE"},
	{"records: a write into a scratch root git can reach", "../eco-report/records.go", "./eco-report/", "TestARecordIsNeverWrittenWhereGitCanReachIt", "r.assertScratchIsUnreachableByGit()", "_ = r.root"},
	{"records: a rewrite that never trims what it shrank", "../eco-report/records.go", "./eco-report/", "TestAnEvictLeavesNoTailOfWhatItRemoved", "if err := handle.Truncate(int64(len(content))); err != nil {", "if err := error(nil); err != nil {"},
	{"records: an unknown operation reaching the scratch directory", "../eco-report/records.go", "./eco-report/", "TestOnlyAnAppendCreatesARecord", `if op != "append" && op != "bump" && op != "revise" && op != "evict" && op != "admit" {`, "if false {"},
	{"records: an empty entry recorded as one", "../eco-report/records.go", "./eco-report/", "TestRecordRefusesEveryWriteItCannotResolve", `if strings.TrimSpace(text) == "" {`, "if false {"},
	{"records: a record name outside the ones this tool owns", "../eco-report/records.go", "./eco-report/", "TestRecordRefusesEveryWriteItCannotResolve", "if kind == nil {", "if false {"},
	{"records: a call with the wrong argument count reaching the switch", "../eco-report/records.go", "./eco-report/", "TestRecordRefusesEveryWriteItCannotResolve", "if len(args) < 3 || len(args) > 4 {", "if false {"},
	{"records: a fourth argument accepted by an op that takes three", "../eco-report/records.go", "./eco-report/", "TestRecordRefusesEveryWriteItCannotResolve", `if (op == "revise" || op == "admit") != (len(args) == 4) {`, "if false {"},
	{"records: a scratch directory anyone on the machine can read", "../eco-report/records.go", "./eco-report/", "TestOnlyAnAppendCreatesARecord", "os.MkdirAll(r.idsdDir, 0o700)", "os.MkdirAll(r.idsdDir, 0o777)"},

	// gate.go — the merge gate, the items a re-qualify must carry, and the routing token.
	{"gate: a stale tree no longer blocks", "../eco-report/gate.go", "./eco-report/", "TestTheHumanIndexIsNeverTouched", "if current != reviewed {\n\t\tif reviewed == \"\" {", "if false {\n\t\tif reviewed == \"\" {"},
	{"gate: an absent stage record no longer blocks", "../eco-report/gate.go", "./eco-report/", "TestGateBlocksOnEachOfItsReasonsAndClearsOnNone", "case isUnstamped(stages):", "case isUnstamped(stages) && false:"},
	{"gate: a turnaround trim no longer blocks", "../eco-report/gate.go", "./eco-report/", "TestATrimmedPassIsNotAFullOne", `case trims != "":`, "case false:"},
	{"gate: a scan that did not run no longer blocks", "../eco-report/gate.go", "./eco-report/", "TestGateBlocksOnEachOfItsReasonsAndClearsOnNone", "case status > 1:", "case false:"},
	{"gate: an open item no longer blocks the merge", "../eco-report/gate.go", "./eco-report/", "TestGateBlocksOnEachOfItsReasonsAndClearsOnNone", `case todos != "":`, "case false:"},
	{"gate: a clean gate reports nothing at all", "../eco-report/gate.go", "./eco-report/", "TestGateBlocksOnEachOfItsReasonsAndClearsOnNone", "\tr.line(\"gate clean: tree fresh, untrimmed qualify, %s, no open TODOs\", r.intentClaim())\n", ""},
	{"gate: an intent that never reached approved no longer blocks", "../eco-report/gate.go", "./eco-report/", "TestGateBlocksAnIntentTheGapRoundsNeverApproved", "\tdefault:\n\t\treturn path, status\n", "\tdefault:\n\t\treturn path, \"\"\n"},
	{"gate: a missing status reads as an approval", "../eco-report/gate.go", "./eco-report/", "TestGateBlocksAnIntentTheGapRoundsNeverApproved", "\tcase \"\":\n\t\treturn path, \"<none>\"\n", "\tcase \"\":\n\t\treturn path, \"\"\n"},
	{"gate: a ship with no intent file reads as an unapproved one", "../eco-report/gate.go", "./eco-report/", "TestGateBlocksOnEachOfItsReasonsAndClearsOnNone", "\tif path == \"\" {\n\t\treturn \"\", \"\"\n\t}\n", ""},
	{"gate: the clean line claims an approval it never read", "../eco-report/gate.go", "./eco-report/", "TestGateScansTheShipsIntentFileAsWellAsItsReport", "\tif r.intentFilePath() == \"\" {\n\t\treturn \"no intent to check\"\n\t}\n", ""},
	{"state token: the ICE's own follow-ups never reach the token", "../eco-report/seams.go", "./eco-report/", "TestGateScansTheShipsIntentFileAsWellAsItsReport", "\tif intent == \"\" {\n\t\treturn false\n\t}\n", "\tif intent == \"\" || true {\n\t\treturn false\n\t}\n"},
	{"state token: an unapproved intent answers ready anyway", "../eco-report/gate.go", "./eco-report/", "TestGateBlocksAnIntentTheGapRoundsNeverApproved", "if r.intentIsUnapproved() {", "if false {"},
	{"carry: the open items go unprinted", "../eco-report/gate.go", "./eco-report/", "TestCarryPrintsTheItemsARequalifyMustNotLose", "if r.openTodos != \"\" {\n\t\tr.line(\"%s\", r.openTodos)", "if false {\n\t\tr.line(\"%s\", r.openTodos)"},
	{"state: a closed ship's archived intent no longer answers done", "../eco-report/gate.go", "./eco-report/", "TestCloseOnACleanReportThePathDoneRuns", `if resolved == reportResolved && shell.IsRegularFile(r.archiveDir(stemOfReportPath(r.report))+"/"+intentName) {`, "if false {"},
	{"state: a token answered for a report that is not there", "../eco-report/gate.go", "./eco-report/", "TestStateAnswersEveryTokenItRoutesOn", "if resolved != reportResolved || !shell.IsRegularFile(r.report) {", "if false {"},
	{"state: the readability guard at its own call site removed", "../eco-report/gate.go", "./eco-report/", "TestAnUnreadableReportIsNotAState", `r.assertReportIsReadable("its state is unknown (permissions?), and 'resume' is what an unread report looks like")`, "_ = r.report"},
	{"state token: an archived intent no longer answers done", "../eco-report/gate.go", "./eco-report/", "TestStateAnswersEveryTokenItRoutesOn", `if slug := r.intentSlug(); slug != "" && shell.IsRegularFile(r.archiveDir(slug)+"/"+intentName) {`, "if false {"},
	{"state token: an unstamped report no longer answers resume", "../eco-report/gate.go", "./eco-report/", "TestTwoIntentsShipSideBySide", "if isUnstamped(reviewed) {", "if false {"},
	{"state token: a moved tree answers ready", "../eco-report/gate.go", "./eco-report/", "TestStateAnswersEveryTokenItRoutesOn", `return "re-qualify" // reviewed once, tree moved since`, `return "ready" // reviewed once, tree moved since`},
	{"state token: open items no longer answer decide", "../eco-report/gate.go", "./eco-report/", "TestStateAnswersEveryTokenItRoutesOn", "if r.anyOpenItemsBeforeMerge(\"the state is unknown.\") {", "if false {"},
	{"state token: a trimmed pass answers ready", "../eco-report/gate.go", "./eco-report/", "TestATrimmedPassIsNotAFullOne", `if isUnstamped(r.reviewedStages()) || r.turnaroundTrims() != "" {`, "if false {"},
	{"list: a partial listing streamed as it goes", "../eco-report/gate.go", "./eco-report/", "TestAnUnreadableReportIsNotAState", `listing += name + "\t" + r.stateToken() + "\n"`, `r.line("%s\t%s", name, r.stateToken())`},
	{"list: the readability guard removed", "../eco-report/gate.go", "./eco-report/", "TestAnUnreadableReportIsNotAState", `r.assertReportIsReadable("nothing was printed, this listing included")`, "_ = r.report"},
	{"list: no reports answered with an empty line", "../eco-report/gate.go", "./eco-report/", "TestAnIntentValueCannotNameAFileOutsideQualifyReports", `r.line("no reports")`, `r.line("")`},

	// scratch.go — excluded, promoted to durable, or torn down. Two of the three are destructive.
	{"check-ignore: the mode read without asserting it could be", "../eco-report/scratch.go", "./eco-report/", "TestPromoteAndCheckIgnoreAlsoRefuseAnUnreadableIndex", "so nothing scratch is ever staged.\n\tr.assertRepoModeReadable()", "so nothing scratch is ever staged.\n\t_ = r.root"},
	{"check-ignore: the committed branch never taken", "../eco-report/scratch.go", "./eco-report/", "TestPromoteReportsTheModeNotTheAdd", "if r.repoMode() == \"committed\" {\n\t\t// A repo promoted out of throwaway", "if false {\n\t\t// A repo promoted out of throwaway"},
	{"check-ignore: an unignored surface reported ok", "../eco-report/scratch.go", "./eco-report/", "TestPromoteReportsTheModeNotTheAdd", `if unignored == "" {`, "if true {"},
	{"promote: nothing to promote is promoted anyway", "../eco-report/scratch.go", "./eco-report/", "TestNoRefusalLeavesTheScratchStrandedInTheTree", "if len(r.reportNames()) == 0 {", "if false {"},
	{"promote: the mode read without asserting it could be", "../eco-report/scratch.go", "./eco-report/", "TestPromoteAndCheckIgnoreAlsoRefuseAnUnreadableIndex", "nothing to promote\")\n\t}\n\tr.assertRepoModeReadable()", "nothing to promote\")\n\t}\n\t_ = r.root"},
	{"promote: an already-committed repo promoted again", "../eco-report/scratch.go", "./eco-report/", "TestPromoteIsIdempotentOverACommittedRepo", "if r.repoMode() == \"committed\" {\n\t\tr.line(\"already committed", "if false {\n\t\tr.line(\"already committed"},
	{"promote: a symlinked .gitignore written through", "../eco-report/scratch.go", "./eco-report/", "TestPromoteWritesNoGitignoreThroughALink", "if shell.IsSymlink(gitignore) {", "if false {"},
	{"promote: an unwritten entry promoted anyway", "../eco-report/scratch.go", "./eco-report/", "TestPromoteWritesNoGitignoreThroughALink", `if unwritten != "" {`, "if false {"},
	{"promote: the entry written but never confirmed with git", "../eco-report/scratch.go", "./eco-report/", "TestPromoteWritesNoGitignoreThroughALink", `return r.ignoreSourceOf(r.root+"/"+ignoreProbe(entry)) != ".gitignore"`, "return false"},
	{"promote: a failed add read as a promotion", "../eco-report/scratch.go", "./eco-report/", "TestPromoteReportsTheModeNotTheAdd", `if r.passThrough("git", "-C", r.root, "add", ".idsd", ".gitignore") != 0 {`, "if false {"},
	{"promote: success read from the add rather than the mode", "../eco-report/scratch.go", "./eco-report/", "TestPromoteReportsTheModeNotTheAdd", `if r.repoMode() != "committed" {`, "if false {"},
	{"discard: no report and no name discarded anyway", "../eco-report/scratch.go", "./eco-report/", "TestDiscardDestructivePath", "case reportNoneOpen:", "case reportLookup(9):"},
	{"discard: the mode read without asserting it could be", "../eco-report/scratch.go", "./eco-report/", "TestDiscardRefusesWhenTheRepoModeCannotBeRead", "r.assertRepoModeReadable()\n\tif r.repoMode() == \"committed\" {\n\t\tr.refuse(\"committed idsd repo", "_ = r.root\n\tif r.repoMode() == \"committed\" {\n\t\tr.refuse(\"committed idsd repo"},
	{"discard: committed mode discarded", "../eco-report/scratch.go", "./eco-report/", "TestDiscardDestructivePath", "if r.repoMode() == \"committed\" {\n\t\tr.refuse(\"committed idsd repo", "if false {\n\t\tr.refuse(\"committed idsd repo"},
	{"discard: the write-path link guard removed", "../eco-report/scratch.go", "./eco-report/", "TestDiscardRefusesASymlinkedIdsdRatherThanDeletingThroughIt", `r.assertWritePathsAreReal("nothing was discarded")`, "_ = r.root"},
	{"discard: the ship-exists guard call removed", "../eco-report/scratch.go", "./eco-report/", "TestDiscardDeletesNothingForAShipThatIsNotHere", "r.assertShipExists(stem)", "_ = stem"},
	{"discard: the readability guard removed", "../eco-report/scratch.go", "./eco-report/", "TestDiscardRemovesNothingItCouldNotRead", `r.assertReportIsReadable("nothing was discarded, because its intent cannot be cross-checked (permissions?)")`, "_ = r.report"},
	{"discard: the two names no longer reconciled", "../eco-report/scratch.go", "./eco-report/", "TestDiscardReconcilesTheTwoNamesBeforeDeletingAnything", `if slug != "" && slug != stem {`, "if false {"},
	{"discard: the ship folder left behind", "../eco-report/scratch.go", "./eco-report/", "TestDiscardDestructivePath", "_ = os.RemoveAll(r.shipDir(stem))", "_ = stem"},
	{"discard: the archived ship folder left behind", "../eco-report/scratch.go", "./eco-report/", "TestDiscardDestructivePath", "_ = os.RemoveAll(r.archiveDir(slug))", "_ = slug"},
	{"discard: the stage markers survive the teardown", "../eco-report/scratch.go", "./eco-report/", "TestDiscardDestructivePath", "_ = os.RemoveAll(r.stageReturnsDir)\n\trmdirIfEmpty(r.intentsDir, r.idsdDir", "_ = r.stageReturnsDir\n\trmdirIfEmpty(r.intentsDir, r.idsdDir"},
	{"discard: what survives no longer keeps .idsd/", "../eco-report/scratch.go", "./eco-report/", "TestDiscardDestructivePath", `if kept := r.survivingContent(); kept != "" {`, "if kept := r.survivingContent(); len(kept) < 0 {"},
	{"close: an open item no longer refuses", "../eco-report/scratch.go", "./eco-report/", "TestCloseRetiresOneShipScratchAndNothingElse", "if !isForced {", "if !isForced && false {"},
	// The case re-inits the same intent after the close: that report is byte-identical to the closed
	// one, so a surviving marker matches it and refuses the next ship's first stage-returned.
	{"close: the stage markers outlive the report", "../eco-report/scratch.go", "./eco-report/", "TestCloseRetiresOneShipScratchAndNothingElse", "_ = os.RemoveAll(r.stageReturnsDir)\n\trmdirIfEmpty(r.intentsDir)", "_ = r.stageReturnsDir\n\trmdirIfEmpty(r.intentsDir)"},

	// init.go — the only subcommand that creates a report, and the one every symlink guard is for.
	{"init: the intent untrimmed before the emptiness guard", "../eco-report/init.go", "./eco-report/", "TestTheFilenameAndTheFrontmatterNameTheSameShip", "intent = trimLeadingSpace(intent)", "intent = intent"},
	// One call, one mutant. The collapse is shell.Oneline now, not a CR/LF replacer: the value can be
	// seeded from a fetched ticket, and the slug charset does not stand between a
	// `review: <description>` intent and the frontmatter. So the newline hazard and the control-byte
	// hazard are one guard. Two cases notice it, and naming the narrower one is what makes the verdict
	// say which property went.
	{"init: the intent value reaches the frontmatter uncollapsed", "../eco-report/init.go", "./eco-report/", "TestTheFrontmatterCannotBeForgedThroughTheIntentValue", "intent = shell.Oneline(intent)", "intent = intent"},
	{"init: an intent that names no report scaffolds one", "../eco-report/init.go", "./eco-report/", "TestAnIntentValueCannotNameAFileOutsideQualifyReports", `if reportName == "" {`, "if false {"},
	{"init: the template check dropped", "../eco-report/init.go", "./eco-report/", "TestADriftedTemplateIsRefusedBeforeAnyReportIsScaffolded", "r.assertTemplateStampable()", "_ = r.template"},
	{"init: the write-path link guard dropped", "../eco-report/init.go", "./eco-report/", "TestInitRefusesRatherThanWritingThroughALink", `r.assertWritePathsAreReal("the report was NOT initialized")`, "_ = r.root"},
	{"init: the ignore precondition dropped", "../eco-report/init.go", "./eco-report/", "TestInitWillNotWriteAReportIntoItsOwnFingerprint", "r.assertReportIsIgnored()", "_ = r.root"},
	{"init: an existing report silently replaced", "../eco-report/init.go", "./eco-report/", "TestAnExistingReportIsNotSilentlyReplaced", "if present && !isForced {", "if present && !isForced && false {"},
	{"init: --force discards the open items in silence", "../eco-report/init.go", "./eco-report/", "TestAnExistingReportIsNotSilentlyReplaced", "carried, _ = r.runTodoGate()", `carried = ""`},
	{"init: the staged path cleared through a link", "../eco-report/init.go", "./eco-report/", "TestInitStagedWriteIsNotAWayOutOfTheRepo", "if err := rmFile(staged); err != nil {", "if err := error(nil); err != nil {"},
	{"flags: --force read as the intent name", "../eco-report/init.go", "./eco-report/", "TestCloseRetiresOneShipScratchAndNothingElse", `if arg == "--force" {`, "if false {"},

	// stamp.go and stages.go — what the stamp demands before it will write a reviewed tree into the
	// report the merge gate trusts, and the grammar the stage record is read in.
	{"stage-returned: a second stage waved through on the first's return", "../eco-report/stamp.go", "./eco-report/", "TestAStampCannotOutliveThePassThatEarnedIt", `if outstanding != "" && outstanding != stage {`, "if false {"},
	{"stage-returned: a resumed stage refused", "../eco-report/stamp.go", "./eco-report/", "TestAStampCannotOutliveThePassThatEarnedIt", `if outstanding != "" && outstanding != stage {`, `if outstanding != "" {`},
	{"no-items: a stage that never returned declared empty", "../eco-report/stamp.go", "./eco-report/", "TestNoItemsDemandsTheStageHaveReturnedFirst", "if !r.stageWasMarkedReturned(stage) {\n\t\tr.refuse(\"error: \" + stage + \" was never marked returned", "if false {\n\t\tr.refuse(\"error: \" + stage + \" was never marked returned"},
	{"stamp: a bare stamp is not the grammar's usage", "../eco-report/stamp.go", "./eco-report/", "TestTheStampGrammarIsTheAuthorityOnWhatAPassMayClaim", `if entries == "" {`, "if false {"},
	{"stamp: whitespace no longer removed from the record", "../eco-report/stamp.go", "./eco-report/", "TestTheStampGrammarIsTheAuthorityOnWhatAPassMayClaim", "entries = removeWhitespace(entries)", "entries = entries"},
	{"stamp: the grammar no longer checked at all", "../eco-report/stamp.go", "./eco-report/", "TestTheStampGrammarIsTheAuthorityOnWhatAPassMayClaim", "if problems := validateStampEntries(entries); len(problems) > 0 {", "if problems := validateStampEntries(entries); len(problems) < 0 {"},
	{"stamp: a report with no reviewed-tree line stamped", "../eco-report/stamp.go", "./eco-report/", "TestTheStampGrammarIsTheAuthorityOnWhatAPassMayClaim", `if !hasField(r.report, "reviewed-tree") {`, "if false {"},
	{"stamp: this pass never invalidated", "../eco-report/stamp.go", "./eco-report/", "TestAStampCannotOutliveThePassThatEarnedIt", `if stamped := r.reviewedTree(); stamped != "pending" {`, "if stamped := r.reviewedTree(); len(stamped) < 0 {"},
	{"stamp: a skipped stage demands a marker anyway", "../eco-report/stamp.go", "./eco-report/", "TestATrimmedPassIsNotAFullOne", `if strings.Contains(entry, ":skipped(") {`, "if false {"},
	{"stamp: the per-stage marker check removed", "../eco-report/stamp.go", "./eco-report/", "TestAStampCannotOutliveThePassThatEarnedIt", `if reason := r.stageBlockReason(stage); reason != "" {`, "if reason := r.stageBlockReason(stage); len(reason) < 0 {"},
	// A stage marker IS the precondition stamp reads instead of re-checking the stage, so the three
	// below are all one defect wearing different clothes: a marker this pass never earned.
	{"init: a fresh report adopting the dead pass's stage markers", "../eco-report/init.go", "./eco-report/", "TestAFreshReportInheritsNoStageMarkerFromTheOneBeforeIt", "if err := os.RemoveAll(r.stageReturnsDir); err != nil {", "if err := error(nil); err != nil {"},
	{"stages: a marker directory any local account can write", "../eco-report/stages.go", "./eco-report/", "TestAStageMarkerIsNotWritableByAnyoneElseOnTheMachine", "os.MkdirAll(r.stageReturnsDir, 0o700)", "os.MkdirAll(r.stageReturnsDir, 0o777)"},
	{"stages: a marker file any local account can forge", "../eco-report/stages.go", "./eco-report/", "TestAStageMarkerIsNotWritableByAnyoneElseOnTheMachine", `[]byte(value+"\n"), 0o600)`, `[]byte(value+"\n"), 0o666)`},
	{"stamp: a pass that never accounted for the decision log", "../eco-report/stamp.go", "./eco-report/", "TestAStampDemandsThePassAccountForTheDecisionLog", "if !r.stageWasMarkedReturned(decisionsMarker) {", "if false {"},
	{"invalidate: last pass's stage returns survive it", "../eco-report/stamp.go", "./eco-report/", "TestInvalidateClearsThePassItStarts", "if err := os.RemoveAll(r.stageReturnsDir); err != nil {", "if err := os.RemoveAll(r.stageReturnsDir + \"/no-such-stage\"); err != nil {"},
	{"stage vocabulary: any word accepted as a stage", "../eco-report/stages.go", "./eco-report/", "TestAStageNameThatIsNotAStageIsRefused", "if stage == known {", "if stage != known {"},
	{"stage vocabulary: a stage renamed out of the pipeline", "../eco-report/stages.go", "./eco-report/", "TestTwoIntentsShipSideBySide", `const stageNames = "code-review security-review tighten refactor"`, `const stageNames = "code-review security-review tighten refactors"`},
	// SURVIVOR, and unreachable rather than merely unobserved — so it stays one. reportChecksum answers
	// empty only for a report os.ReadFile cannot open, and `stage-returned` reaches this through
	// requireReport, which refuses anything that is not a readable regular file first. Every shape a
	// fixture can put at that path was tried — chmod 000, a directory, a dangling link — and each stops
	// at one of those two guards, both of which have a mutant of their own above. This is
	// defence-in-depth behind them, and no case driving the tool can get past them to it.
	{"marker: an unchecksummable report marked anyway", "../eco-report/stages.go", "./eco-report/", "TestAMarkerIsThePosixCksumOfTheReport", `if value == "" {`, "if false {"},
	{"marker: the trailing newline left on the value", "../eco-report/stages.go", "./eco-report/", "TestAStampCannotOutliveThePassThatEarnedIt", `strings.TrimRight(string(content), "\n")`, "string(content)"},
	// SURVIVOR, and an equivalent mutation — no case can kill it, so it stays one. Passes recorded
	// through `no-items` are stamped all over this suite (armFullPass does nothing else); dropping the
	// arm just falls to `recorded == r.reportChecksum()` below, which a `no-items` marker can never
	// satisfy, since a checksum is "<digits> <digits>". The two programs agree on every input. The arm
	// stays because it states the rule that string format only happens to enforce.
	{"stage block: a no-items marker blocks the stamp", "../eco-report/stages.go", "./eco-report/", "TestTwoIntentsShipSideBySide", "if recorded == noItemsMarker {", "if false {"},
	{"stage block: unrecorded items no longer block", "../eco-report/stages.go", "./eco-report/", "TestAStampCannotOutliveThePassThatEarnedIt", "if recorded == r.reportChecksum() {", "if false {"},
	{"stage block: an unmarked stage no longer blocks", "../eco-report/stages.go", "./eco-report/", "TestAStampCannotOutliveThePassThatEarnedIt", "if !r.stageWasMarkedReturned(stage) {\n\t\treturn \"ran but was never marked returned", "if false {\n\t\treturn \"ran but was never marked returned"},
	{"stamp grammar: any entry at all accepted", "../eco-report/stages.go", "./eco-report/", "TestTheStampGrammarIsTheAuthorityOnWhatAPassMayClaim", "stage, ok := stageOfEntry(entry)\n\t\tif !ok {", "stage, ok := stageOfEntry(entry)\n\t\tif false && !ok {"},
	{"stamp grammar: a missing stage accepted", "../eco-report/stages.go", "./eco-report/", "TestTheStampGrammarIsTheAuthorityOnWhatAPassMayClaim", "case seen[stage] == 0:", "case false:"},
	{"stamp grammar: a duplicate stage accepted", "../eco-report/stages.go", "./eco-report/", "TestTheStampGrammarIsTheAuthorityOnWhatAPassMayClaim", "case seen[stage] > 1:", "case false:"},
	{"stamp grammar: refactor:partial(turnaround) no longer legal", "../eco-report/stages.go", "./eco-report/", "TestTheStampGrammarIsTheAuthorityOnWhatAPassMayClaim", `case "refactor", "refactor:partial(turnaround)", "refactor:partial(cap)":`, `case "refactor", "refactor:partial(cap)":`},
	{"stamp grammar: skipped(turnaround) no longer legal", "../eco-report/stages.go", "./eco-report/", "TestATrimmedPassIsNotAFullOne", `if entry == stage || entry == stage+":skipped(turnaround)" || entry == stage+":skipped(not-applicable)" {`, `if entry == stage || entry == stage+":skipped(not-applicable)" {`},
	{"stamp grammar: skipped(not-applicable) no longer legal", "../eco-report/stages.go", "./eco-report/", "TestTheStampGrammarIsTheAuthorityOnWhatAPassMayClaim", `if entry == stage || entry == stage+":skipped(turnaround)" || entry == stage+":skipped(not-applicable)" {`, `if entry == stage || entry == stage+":skipped(turnaround)" {`},
	{"stamp grammar: a stage left out of the skippable set", "../eco-report/stages.go", "./eco-report/", "TestTheStampGrammarIsTheAuthorityOnWhatAPassMayClaim", `[]string{"security-review", "tighten"} {`, `[]string{"security-review"} {`},

	// shell.go — the primitives whose exact edges a refusal turns on, and the digest a marker holds.
	{"slug charset: the dash left out of the set", "../eco-report/shell.go", "./eco-report/", "TestTwoIntentsShipSideBySide", `b == '.' || b == '_' || b == '-'`, `b == '.' || b == '_'`},
	{"slug charset: a path separator let into the set", "../eco-report/shell.go", "./eco-report/", "TestAnIntentValueCannotNameAFileOutsideQualifyReports", `b == '.' || b == '_' || b == '-'`, `b == '.' || b == '_' || b == '-' || b == '/'`},
	{"readable: -r asked as mere existence", "../eco-report/shell.go", "./eco-report/", "TestAnUnreadableReportIsNotAState", "syscall.Access(path, 0x4)", "syscall.Access(path, 0x0)"},
	{"executable: -x asked as mere existence", "../eco-report/shell.go", "./eco-report/", "TestAMissingFingerprintScriptRefusesInsteadOfRecomputing", "syscall.Access(path, 0x1)", "syscall.Access(path, 0x0)"},
	// `join`, not `records`: that prefix now names the four shared record files. A label is a mutant's
	// whole identity in the verdict column, so a prefix naming two subjects sends the reader of a
	// survivor to the wrong file.
	{"join: the trailing newline dropped from every rewrite", "../eco-report/shell.go", "./eco-report/", "TestAStampCannotOutliveThePassThatEarnedIt", `out.WriteString("\n")`, `out.WriteString("")`},
	// The case puts an *empty* directory at init's staged path: os.Remove takes one happily where
	// `rm -f` refuses, and a directory with anything in it fails the removal either way.
	{"rm -f: a directory removed where the shell's refused", "../eco-report/shell.go", "./eco-report/", "TestInitStagedWriteIsNotAWayOutOfTheRepo", "if info.IsDir() {", "if info.IsDir() && false {"},
	// The marker's digest is read by whichever version of this tool runs next, so it has to be
	// cksum(1)'s and not merely this tool's own: any self-consistent digest passes every comparison
	// the tool makes against itself.
	{"cksum: a digest that is not POSIX cksum's", "../eco-report/shell.go", "./eco-report/", "TestAMarkerIsThePosixCksumOfTheReport", "crc = crc<<1 ^ 0x04C11DB7", "crc = crc<<1 ^ 0x04C11DB6"},
	{"cksum: the length no longer folded in", "../eco-report/shell.go", "./eco-report/", "TestAMarkerIsThePosixCksumOfTheReport", "for length := len(content); length != 0; length >>= 8 {", "for length := len(content); false; length >>= 8 {"},

	// The guards a security pass added. They sit together because they answer one question — what a tree
	// the tool does not own can make it do — rather than because they share a file.
	{"traversal: an out-of-root ref is stat'ed after all", "tree.go", "./eco-check/", "TestATraversalLinkIsNotStatted", `	rel, err := filepath.Rel(c.root.Named(), path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, "../")`, `	_, err := filepath.Rel(c.root.Named(), path)
	return err == nil || true`},
	// The three pieces that make one tree answer one way however its root was spelled. Whether a cited
	// path names a file here used to depend on the caller's typing, and no finding may. Each label
	// below is a separate way that could come back.
	{"canonical index: the root's own name is not a key", "tree.go", "./eco-check/", "TestAPathRefThroughTheRootsOwnNameResolvesHoweverTheRootIsNamed", "\tt.suffixes[entry.canonical] = append(t.suffixes[entry.canonical], entry.path)\n", ""},
	{"canonical index: the root is indexed as it was spelled", "tree.go", "./eco-check/", "TestAPathRefThroughTheRootsOwnNameResolvesHoweverTheRootIsNamed", `	absolute, err := filepath.Abs(start)
	if err != nil {
		return shell.BaseName(start)
	}
	return shell.BaseName(absolute)`, `	_, err := filepath.Abs(start)
	if err != nil {
		return shell.BaseName(start)
	}
	return shell.BaseName(start)`},
	{"canonical index: a doubled separator survives the root cut", "tree.go", "./eco-check/", "TestAPathRefThroughTheRootsOwnNameResolvesHoweverTheRootIsNamed", `strings.TrimLeft(strings.TrimPrefix(path, t.start), "/")`, `strings.TrimPrefix(path, t.start)`},
	// The fourth way, and the only one the spelling-agreement case above cannot see: an anchor reaching
	// *above* the root instead of falling short of it. tree.go's rootName carries what that one leaks.
	{"canonical index: the anchor reaches above the root", "tree.go", "./eco-check/", "TestACitationNamingADirectoryAboveTheRootDoesNotResolve", "\treturn shell.BaseName(absolute)", "\treturn strings.TrimPrefix(absolute, \"/\")"},
	// The tails, which are the other half of the index and the shape prose writes most often. Taken
	// from the spelled path they carry whatever the caller typed above the root, which is the leak the
	// case above names.
	{"canonical index: the tails taken from the spelled path", "tree.go", "./eco-check/", "TestACitationNamingADirectoryAboveTheRootDoesNotResolve", `	for i := 0; i < len(entry.canonical); i++ {
		if entry.canonical[i] == '/' {
			tail := entry.canonical[i+1:]`, `	for i := 0; i < len(entry.path); i++ {
		if entry.path[i] == '/' {
			tail := entry.path[i+1:]`},
	// What keeps the index answering a name rather than a pattern. citations.go → globInCitation says
	// why the cited path is the one token that can carry one, and what refusing it buys. Removed, the
	// citation falls through to `unresolvable citation path` and sends its reader hunting for a file
	// nobody is missing. Nothing would then stand in front of the resolver if a matcher ever came back.
	{"citations: a pattern in a cited path is matched rather than refused", "citations.go", "./eco-check/", "TestAPatternInACitedPathIsRefusedRatherThanMatched", `	if glob := strings.IndexAny(cited.path, globInCitation); glob >= 0 {`, `	if glob := strings.IndexAny(cited.path, globInCitation); glob >= 0 && false {`},
	{"headings: the target re-parsed once per citation", "headings.go", "./eco-check/", "TestAMarkdownFileIsParsedOncePerRun", `	if cached, ok := c.headings[path]; ok {
		return cached
	}`, ``},
	{"bolded runs: the target re-parsed once per failing citation", "headings.go", "./eco-check/", "TestAMarkdownFileIsParsedOncePerRun", `	if cached, ok := c.bolded[path]; ok {
		return cached
	}`, ``},
	{"call sites: the whole-file read unbounded again", "subcommands.go", "./eco-check/", "TestOversizeFileIsNotReadByTheCallSiteScan", `		if info.Size() > maxFileBytes {`, `		if info.Size() > (1 << 62) {`},

	// The --gate guards. What they answer is one question — whether a file this checkout holds but a
	// commit cannot carry may decide the verdict — so they sit together rather than by file.
	//
	// Each of the three reaches gets its own mutant, because they are three different ways a local file
	// gets back in: the walk every scan reads, the path a citation spells out in prose, and the skills
	// directory the mount scan lists from. One case cannot observe more than one of them.
	{"gate: a gitignored file left in the walk", "gate.go", "./eco-check/", "TestAGitignoredFileIsJudgedWithoutTheFlagAndNotWithIt", "if g.holds(entry.path) {", "if g.holds(entry.path) && false {"},
	{"gate: a citation resolved through a gitignored file", "tree.go", "./eco-check/", "TestACitationResolvingOnlyThroughAGitignoredFileDangles", "return c.underRoot(path) && !c.isSkippedByGate(path) && shell.PathExists(path)", "return c.underRoot(path) && shell.PathExists(path)"},
	{"gate: a gitignored skill directory listed anyway", "tree.go", "./eco-check/", "TestAGitignoredSkillDirectoryIsNotCounted", "if c.isSkippedByGate(dir) {", "if c.isSkippedByGate(dir) && false {"},
	// The path list crosses a process boundary, and NUL is what keeps a committed filename holding a
	// newline one path on both sides of it.
	{"gate: the path list handed to git line-delimited", "gate.go", "./eco-check/", "TestAGitignoredFileWhoseNameHoldsANewlineIsStillFilteredOut", `exec.Command("git", "check-ignore", "-z", "--stdin")`, `exec.Command("git", "check-ignore", "--stdin")`},
	// Both directions off git's exit code, which is why they share a function. Read as a failure, exit 1
	// refuses every clean checkout the flag exists to serve; read as an answer, exit 128 hands the gate
	// back an unfiltered tree under a line saying it was filtered.
	{"gate: git saying nothing is ignored read as a failure", "gate.go", "./eco-check/", "TestTheFlagRunsOnATreeWithNothingIgnored", "exit.ExitCode() == 1", "exit.ExitCode() == 99"},
	{"gate: a git that could not answer read as one that did", "gate.go", "./eco-check/", "TestTheFlagRefusesWhereGitCannotAnswer", "return errors.As(err, &exit) && exit.ExitCode() == 1", "return errors.As(err, &exit) || true"},
	// The report's two bounds. Collapsing to the outermost ignored path is what makes the count read as
	// how much of the tree went unjudged; the name cap is what keeps tree-chosen text off the exit-0
	// path, exactly as uncountedNote does for an import name.
	{"gate: every file under an ignored directory named separately", "gate.go", "./eco-check/", "TestAGitignoredSkillDirectoryIsNotCounted", "if !ignored[shell.DirName(path)] {", "if true {"},
	{"gate: the skipped names unbounded", "gate.go", "./eco-check/", "TestTheSkippedNamesAreBoundedAndTheCountIsNot", "const gateSkipNameCap = 5", "const gateSkipNameCap = 100000"},
	// The named-path reaches: a scan that stats a path it spells out for itself rather than taking it
	// from the walk. Everywhere else the walk filter already answers, and a guard it makes
	// unobservable is one this harness reports as killing nothing.
	{"gate: a gitignored SKILL.md still found by name", "skills.go", "./eco-check/", "TestAGitignoredSkillFileLeavesItsDirectoryWithoutOne", `if !c.holdsRegularFile(shell.Join(entry.path, "SKILL.md")) {`, `if !shell.IsRegularFile(shell.Join(entry.path, "SKILL.md")) {`},
	{"gate: a gitignored SKILL.md still counted in the census", "skills.go", "./eco-check/", "TestAGitignoredSkillFileLeavesItsDirectoryWithoutOne", "if !c.holdsRegularFile(file) {", "if !shell.IsRegularFile(file) {"},
	{"gate: a gitignored CLAUDE.md still counted into the budget", "budget.go", "./eco-check/", "TestAGitignoredClaudeMdIsNeitherCountedNorScanned", "if c.holdsSomething(claudeMd) {", "if shell.PathExists(claudeMd) || shell.IsSymlink(claudeMd) {"},
	// Without the arm above it, a gitignored doc reaches absentOrOutOfReach, whose Lstat finds the file
	// sitting right where the router points and reports that nothing could answer for it — a refusal
	// about this machine, where the fact is that the commit does not carry the file.
	{"gate: a gitignored Read-always target refused instead of absent", "budget.go", "./eco-check/", "TestAGitignoredReadAlwaysTargetIsReportedAbsent", "\t\tcase c.isSkippedByGate(listed):\n\t\t\tc.reportAbsentBudgetDoc(doc)\n", ""},
	// A citation is a path prose wrote, so it arrives spelled however its author spelled it. Compared
	// as written rather than cleaned, `./notes.md` misses an ignored set keyed on `notes.md` while
	// naming the same file — the run lists that file as skipped and then resolves a reference through
	// it.
	{"gate: the skip compared as spelled instead of cleaned", "gate.go", "./eco-check/", "TestACitationSpelledNonCanonicallyStillHitsTheGate", "return g.ignored[filepath.Clean(path)]", "return g.ignored[path]"},
	// The filtered tree is copied from the walk rather than declared fresh, so a field the walk gains
	// is carried into it. Unobservable while `tree` holds only the two fields the copy resets by hand,
	// which is why this one is declared unreachable below rather than left to survive.
	{"gate: the filtered tree rebuilt from a literal", "gate.go", "./eco-check/", "TestAGatedRunAndABareRunAgreeHoweverTheRootIsNamed", `	kept := *walked
	kept.entries = nil
	kept.suffixes = map[string][]string{}`, `	kept := tree{suffixes: map[string][]string{}}`},
	// The three reaches that name a skill's own SKILL.md. Each is masked by a different sibling, so
	// each needs its own shape in the case that kills it: the alternation shows only through a
	// citation, the whole-token re-test only through a token that is not itself a lane name, and the
	// unknown-skill scan only through the bare name.
	{"gate: a gitignored SKILL.md still builds a lane name", "direction.go", "./eco-check/", "TestAGitignoredSkillFileIsNotALaneUnderTheFlag", `		if !c.holdsRegularFile(c.skillFilePath(name)) {`, `		if !shell.IsRegularFile(c.skillFilePath(name)) {`},
	{"gate: a gitignored SKILL.md still passes the whole-token re-test", "direction.go", "./eco-check/", "TestAGitignoredSkillFileIsNotALaneUnderTheFlag", `		if !c.holdsRegularFile(c.skillFilePath(named)) {`, `		if !shell.IsRegularFile(c.skillFilePath(named)) {`},
	{"gate: a gitignored SKILL.md still counts as a known skill", "refs.go", "./eco-check/", "TestAGitignoredSkillFileIsNotALaneUnderTheFlag", `		if name == "kk-flavor" || c.holdsRegularFile(c.skillFilePath(name)) {`, `		if name == "kk-flavor" || shell.IsRegularFile(c.skillFilePath(name)) {`},
	// A second path overwriting the first rather than being refused: the run then scans a tree the
	// caller named second while believing it asked about the first, and a mistyped flag reaches that
	// arm too — an unfiltered run under a caller that asked for a gated one.
	{"gate: a second root taken instead of refused", "eco-check.go", "./eco-check/", "TestAnUnknownArgumentIsRefused", "\t\tcase hasRoot:\n", "\t\tcase hasRoot && false:\n"},
	{"reports: a stem outside the slug charset listed anyway", "../eco-report/paths.go", "./eco-report/", "TestAFilenameCannotForgeAListingRow", `		if !isSlugCharset(stem) {`, `		if !isSlugCharset(stem) && false {`},

	// The same question asked of ecoreport, whose answer moved when the scratch left the tree: the
	// location now comes from a config file, the directory sits outside the repository, and it is what
	// `discard` removes. Each of these turns a guard into one that still reports success.
	//
	// Where that config file is read FROM is an input too — overrideConfigPath says what a relative one
	// lets the checkout under review decide.
	{"override: a relative config home read as an override", "../eco-report/root.go", "./eco-report/", "TestANonAbsoluteConfigHomeIsNotAnOverride", `if !filepath.IsAbs(config) {`, `if config == "" {`},
	// The mode the scratch tree is created with, which is the whole of what decides who reads a report:
	// the report itself lands at the template's mode, and the template is 0644 in the skill dir.
	{"scratch: the tree created world-readable again", "../eco-report/init.go", "./eco-report/", "TestTheScratchDirectoryIsReadableByItsOwnerAlone", `shell.DirName(r.report), 0o700)`, `shell.DirName(r.report), 0o777)`},
	// `promote` is the one subcommand that stages, so without this the link itself is committed and
	// `git ls-files` answering "committed" is read as success. assertScratchDirsAreReal has the rest.
	{"promote: a symlinked scratch dir moved into the tree and staged", "../eco-report/scratch.go", "./eco-report/", "TestPromoteRefusesASymlinkedScratchRatherThanCommittingTheLink", `	r.assertScratchDirsAreReal("not promoted, and nothing was moved")`, `	_ = r.root`},
	// Both halves of the reviewed-worktree value, which are two different hazards on one field: a newline
	// forges a frontmatter line on the WRITE (currentWorktreeRecord), and an ESC rewrites the terminal on
	// the ECHO (blocksOnFreshness). One mutant each, because one case cannot observe both.
	{"stamp: the reviewing worktree's path recorded uncollapsed", "../eco-report/worktree.go", "./eco-report/", "TestAWorktreePathCarriesNoControlByteIntoTheReport", `shell.Oneline(r.currentWorktreePath()), true`, `r.currentWorktreePath(), true`},
	// Two branches quote that field, so two mutants — and each anchor carries the message text around
	// the call, because the call alone matches both and an anchor matching twice is refused. They are
	// separate claims: one blocks a review taken elsewhere, the other blocks a value that is not a
	// usable token at all, and that second one is arbitrary text by definition.
	{"gate: the recorded worktree echoed uncollapsed", "../eco-report/gate.go", "./eco-report/", "TestAReviewedWorktreeValueCarriesNoControlByteToTheTerminal", `reviewed in another worktree (" + shell.Oneline(fieldValue(r.report, "reviewed-worktree"))`, `reviewed in another worktree (" + fieldValue(r.report, "reviewed-worktree")`},
	{"gate: an unusable worktree value echoed uncollapsed", "../eco-report/gate.go", "./eco-report/", "TestAReviewedWorktreeValueCarriesNoControlByteToTheTerminal", `no usable reviewing worktree (reviewed-worktree: " + shell.Oneline(fieldValue(r.report, "reviewed-worktree"))`, `no usable reviewing worktree (reviewed-worktree: " + fieldValue(r.report, "reviewed-worktree")`},
	// Where the rewrite stages decides whether it can be atomic at all; rewriteReport says why $TMPDIR
	// makes moveFile's non-atomic fallback the ordinary path rather than the exotic one.
	{"rewrite: the report staged in $TMPDIR again", "../eco-report/frontmatter.go", "./eco-report/", "TestAReportRewriteIsStagedBesideTheReport", `os.CreateTemp(shell.DirName(r.report), ".rewrite.")`, `os.CreateTemp("", "")`},
	// Two readers apply this bound, and the same four lines open both — so each anchor carries the
	// return above it, which is the only thing that tells them apart. Anchored on the shared line
	// alone, one of them matches twice and preflight refuses the whole run.
	{"budget: the whole-file read unbounded again", "../eco-stats/measure.go", "./eco-stats/", "TestAnOversizeBudgetFileIsRefusedRatherThanRead", `		return nil
	}
	if info.Size() > maxFileBytes {`, `		return nil
	}
	if info.Size() > (1 << 62) {`},
	// The other reader, and the one whose figure the refusal describes: unbounded, the words of a file
	// the same run calls "not read, not counted" go into the always-loaded total, and ecostats and
	// ecocheck stop agreeing on the tier.
	{"budget: the word count unbounded again", "../eco-stats/measure.go", "./eco-stats/", "TestARefusedBudgetFilesWordsAreNotInTheFigure", `		return 0
	}
	if info.Size() > maxFileBytes {`, `		return 0
	}
	if info.Size() > (1 << 62) {`},

	// eco-report.go — the dispatch, and the one place a refusal becomes an exit code.
	{"exec: a refusal reported as success", "../eco-report/eco-report.go", "./eco-report/", "TestAnExistingReportIsNotSilentlyReplaced", "code = signal.code", "code = 0"},
	{"dispatch: a subcommand no longer routed", "../eco-report/eco-report.go", "./eco-report/", "TestCarryPrintsTheItemsARequalifyMustNotLose", `case "carry":`, `case "carry-x":`},

	// The refusals that keep a tool from reporting on a subject it never reached. They answer one
	// question — whether an exit of 0 is available to a run that could not read part of what it was
	// pointed at — so they sit together rather than by file.
	//
	// The first three are this list's only reach into cite-graph, which had no mutant at all.
	{"cite-graph: a partial scan reported rather than refused", "../cite-graph/main.go", "./cite-graph/", "TestAPartialScanRefusesRatherThanReporting", "if skipped > 0 {", "if skipped > 0 && false {"},
	{"cite-graph: an unread path never counted", "../cite-graph/read.go", "./cite-graph/", "TestAPathTheWalkCannotReadIsAlsoCounted", "s.skipped++", "s.skipped += 0"},
	// The suffix filter used to run first, so this branch was unreachable for a link to a directory and
	// the subtree behind it left every figure with nothing on stderr.
	{"cite-graph: a symlinked directory dropped in silence", "../cite-graph/read.go", "./cite-graph/", "TestASymlinkedDirectoryIsReportedAndCounted", "	case resolved.IsDir():", "	case resolved.IsDir() && false:"},
	// The whole cited path has to be the tail of the file it names. Shortened, `made/up/writing.md`
	// answers to the real `writing.md` and the graph reports an edge nobody wrote. `ruleecho` keeps its
	// own copy of this guard on purpose — two sites is under the extract bar, and the tools differ on
	// an ambiguous name — so this entry is the twin of `ruleecho: a citation matched on its last
	// segment alone`, and each copy is watched on its own.
	{"cite-graph: a citation matched on its last segment alone", "../cite-graph/read.go", "./cite-graph/", "TestAPathThatNamesNoFileIsNotAnsweredByItsBasename", `if seen[candidate] || !(candidate == form || strings.HasSuffix(candidate, "/"+form)) {`, "if seen[candidate] {"},

	{"stats: an unreadable path no longer withholds the row", "../eco-stats/eco-stats.go", "./eco-stats/", "TestAnUnlistableDirectoryIsNotMeasuredAsASmallerTree", "if s.unreadable != 0 {", "if s.unreadable != 0 && false {"},
	{"stats: an unread path never counted", "../eco-stats/measure.go", "./eco-stats/", "TestAnUnlistableDirectoryIsNotMeasuredAsASmallerTree", "s.unreadable++", "s.unreadable += 0"},
	// Not the refusal but the diagnosis: PathExists answers false for a file nobody wrote and for one
	// behind a directory this cannot open, and reporting the second as the first sends a reader hunting
	// for a file that is exactly where the router says it is.
	// The guard moved into ecoroot when the two tools stopped each holding a copy of it, so the anchor
	// followed it. One mutant still, not one per consumer: it is one guard, and ecocheck's own case for
	// it reddens on the same edit.
	{"root: permission denied reported as absence", "../eco-root/contained.go", "./eco-stats/", "TestAReadAlwaysTargetOutOfReachIsNotReportedMissing", "if errors.Is(err, fs.ErrNotExist) {", "if err != nil || errors.Is(err, fs.ErrNotExist) {"},
	// One guard with two limbs, and dropping either resolves a whole shape of self name against the
	// working directory again. So one mutant each, not one for the guard.
	{"stats: a self name off PATH resolved against the cwd", "../eco-stats/ledger.go", "./eco-stats/", "TestASelfNameThatDoesNotPlaceTheProgramAppendsNothing", `if !strings.Contains(self, "/") || strings.HasSuffix(self, "/") {`, `if strings.HasSuffix(self, "/") {`},
	{"stats: a self name naming a directory resolved against the cwd", "../eco-stats/ledger.go", "./eco-stats/", "TestASelfNameThatDoesNotPlaceTheProgramAppendsNothing", `if !strings.Contains(self, "/") || strings.HasSuffix(self, "/") {`, `if !strings.Contains(self, "/") {`},
	{"report: the open-item scan run without being checked for", "../eco-report/seams.go", "./eco-report/", "TestGateBlocksOnEachOfItsReasonsAndClearsOnNone", "if !isExecutable(r.todoGate) {", "if !isExecutable(r.todoGate) && false {"},
	// Two reads whose failure used to arrive at a deletion wearing the shape of an answer — the rule
	// assertRepoModeReadable states, applied to the other two reads `discard` turns on.
	{"report: a listing that failed read as no reports open", "../eco-report/paths.go", "./eco-report/", "TestDiscardWillNotClearIdsdOnAReportListingItCouldNotRead", "if !errors.Is(err, fs.ErrNotExist) {", "if !errors.Is(err, fs.ErrNotExist) && false {"},
	// The third copy of the map-iteration shape in this package, after routerPath and tails. `_ =` and
	// not a deleted line, or `sort` goes unimported and the mutant does not build — and a `broken`
	// verdict says nothing about the guard.
	{"cite-graph: an alias winner taken from a map iteration", "../cite-graph/headings.go", "./cite-graph/", "TestTwoHeadingsSharingAnAliasResolveToTheSameOneEveryTime", "	sort.Strings(sorted)", "	_ = sort.Strings"},

	// The handoff gate. One mutant per scan, because each scan is the only thing standing between a
	// draft and a chip a fresh session cannot act on, and a scan that silently stopped firing looks
	// exactly like a tree of well-written drafts.
	{"handoff: the licence blockquote always satisfied", "../handoff-check/handoff-check.go", "./handoff-check/", "TestLicenceMustBeQuoted", "if section == \"Licence\" && isBlockquote(text) {", "if section == \"Licence\" || true {"},
	{"handoff: every base commit resolves", "../handoff-check/handoff-check.go", "./handoff-check/", "TestBaseAndRepository", `if _, err := git(repo, "cat-file", "-e", sha+"^{commit}"); err == nil {`, `if _, err := git(repo, "cat-file", "-e", sha+"^{commit}"); err == nil || true {`},
	{"handoff: the repository the draft must name goes unchecked", "../handoff-check/handoff-check.go", "./handoff-check/", "TestBaseAndRepository", `if name == "Where it starts" && !s.named {`, `if name == "Where it starts" && !s.named && false {`},
	// The bound is what tells a commit from a hex run inside a word. Removed, a draft naming a
	// temporary directory is answered with a commit nobody wrote.
	{"handoff: the hex scan unbounded again", "../handoff-check/handoff-check.go", "./handoff-check/", "TestBaseAndRepository", "`(^|[^0-9A-Za-z])([0-9a-f]{7,})([^0-9A-Za-z]|$)`", "`()([0-9a-f]{7,})()`"},
	{"handoff: the reachback scan silenced", "../handoff-check/handoff-check.go", "./handoff-check/", "TestReachback", "if hit := matcher.FindString(low); hit != \"\" {", "if hit := matcher.FindString(low); false {"},
	{"handoff: an empty slot read as filled", "../handoff-check/handoff-check.go", "./handoff-check/", "TestStructure", "case !s.filled[name]:", "case !s.filled[name] && false:"},
	{"handoff: an eighth heading accepted", "../handoff-check/handoff-check.go", "./handoff-check/", "TestStructure", "case !isRequired(name):", "case !isRequired(name) && false:"},
	{"handoff: a second title line accepted", "../handoff-check/handoff-check.go", "./handoff-check/", "TestStructure", "if s.titles > 1 {", "if s.titles > 1 && false {"},
	// The anchor on the placeholder, which is what lets a real title hold an angle bracket.
	{"handoff: the placeholder title matched anywhere in the line", "../handoff-check/handoff-check.go", "./handoff-check/", "TestStructure", `if title == "" || (strings.HasPrefix(title, "<") && strings.HasSuffix(title, ">")) {`, `if title == "" || strings.Contains(title, "<") {`},
	{"handoff: a leftover template comment ignored", "../handoff-check/handoff-check.go", "./handoff-check/", "TestStructure", `s.flag(fmt.Sprintf("template comment left at line %d — that slot is unfilled", lineNo))`, `_ = lineNo`},
	{"handoff: None accepted in the slots that refuse it", "../handoff-check/handoff-check.go", "./handoff-check/", "TestNone", "case refuseNone[name]:", "case refuseNone[name] && false:"},
	// The word boundary after `None`, without which a slot opening "Nonetheless" is read as empty and
	// its real content never measured.
	{"handoff: None matched as a prefix", "../handoff-check/handoff-check.go", "./handoff-check/", "TestNone", `return first == "None" || (strings.HasPrefix(first, "None") && len(first) > 4 && shell.IsSpaceByte(first[4]))`, `return strings.HasPrefix(first, "None")`},
	{"handoff: the substance floor lowered to nothing", "../handoff-check/handoff-check.go", "./handoff-check/", "TestSubstanceFloor", "if name != \"Licence\" && s.words[name] < minWords {", "if name != \"Licence\" && s.words[name] < 0 {"},
	// Fenced content counts as content. Dropped, a slot holding only command output — which is exactly
	// what Measured facts asks for — comes back as empty and sends the author to fix the wrong thing.
	{"handoff: a fenced line contributes nothing to its slot", "../handoff-check/handoff-check.go", "./handoff-check/", "TestSubstanceFloor", `			s.absorb(section, raw)
			continue`, `			continue`},
	{"handoff: the dirty-tree note never printed", "../handoff-check/handoff-check.go", "./handoff-check/", "TestDirtyTreeIsANoteAndNotAFinding", "if dirty == 0 {", "if dirty >= 0 {"},
	{"ruleecho: a rule citing the file that owns it read as a restatement", "../rule-echo/match.go", "./rule-echo/", "TestAPointerToTheRulesOwnerIsNotARestatement", "if a.cites[b.file] || b.cites[a.file] {", "if false {"},
	// The exemption is decided in `classify` and wired in `collect`, so the guard above is only half of
	// it: with the citations never reaching the spans, `classify` reads an empty map and every pointer
	// is a restatement again. A case that builds its own spans passes over this mutation, which is how
	// the aimed-at line went uncovered until a case drove `collect` itself.
	//
	// Emptied as `line[end:end]` and not as `""`, because `end` is read nowhere else and a mutant that
	// orphans it does not compile.
	{"ruleecho: a line's citations never reach the spans on it", "../rule-echo/main.go", "./rule-echo/", "TestAPointerToTheRulesOwnerIsNotARestatement", "cited := citedTargets(line[b.start:end])", "cited := citedTargets(line[end:end])"},
	// The two edges of a rule's citation scope, one mutant and one anchor each: the slice expression
	// carries the lower bound, the assignment above it the upper. Repoint one when the code moves and
	// the other still needs repointing. A citation belongs to the rule it was written for. Widened back
	// to the head of the line, a pointer exempts the rule before it; widened forward to the end, it
	// exempts the ones after. Either way a duplicated rule parked beside a legitimate cross-reference
	// goes silent, and silence is the one failure this tool has no second guard against.
	{"ruleecho: a rule's citations start at the head of its line", "../rule-echo/main.go", "./rule-echo/", "TestACitationDoesNotExemptTheOtherRulesOnItsLine", "cited := citedTargets(line[b.start:end])", "cited := citedTargets(line[0:end])"},
	{"ruleecho: a rule's citations run to the end of its line", "../rule-echo/main.go", "./rule-echo/", "TestACitationDoesNotExemptTheOtherRulesOnItsLine", "end = onLine[idx+1].start", "end = len(line)"},
	// The whole cited path has to be the tail of the file it names. Shortened, `made/up/writing.md`
	// answers to the real `writing.md` — and here that mistake exempts a genuine restatement, so a
	// duplicated rule is reported as compliant. `cite-graph`'s nameResolver carries the same guard
	// against the same hazard.
	{"ruleecho: a citation matched on its last segment alone", "../rule-echo/match.go", "./rule-echo/", "TestACitationNamingAnotherFileDoesNotExemptTheRestatement", `if seen[candidate] || !(candidate == form || strings.HasSuffix(candidate, "/"+form)) {`, "if seen[candidate] {"},
	// A name that two files answer to names neither of them. Guess, and half the time the guess exempts
	// a pair the tree can prove nothing about.
	{"ruleecho: an ambiguous citation resolved to the first file that matched", "../rule-echo/match.go", "./rule-echo/", "TestAnAmbiguousCitationExemptsNothing", "if len(paths) != 1 {", "if len(paths) < 1 {"},
	// The other side of that branch, and the one the refusal case cannot reach: most of this tree's
	// citations are bare names, so a branch that resolves none of them reports every pointer written
	// that way as the duplication it is not.
	{"ruleecho: a bare name never names a file", "../rule-echo/match.go", "./rule-echo/", "TestABareNameOnlyOneFileAnswersToResolves", "return only(r.byBase[target])", `return ""`},
	// The accepted groups must never read as the failing one. Printed under the restatement headline, or
	// dropped from the summary, a pair the tree only points at comes back as a pair it duplicates.
	{"ruleecho: the citing group never counted in the summary", "../rule-echo/main.go", "./rule-echo/", "TestTheReportNamesEachGroupAndCountsItInTheSummary", "if len(r.citing) > 0 {", "if len(r.citing) > 0 && false {"},
	{"ruleecho: the citing group printed under the restatement headline", "../rule-echo/main.go", "./rule-echo/", "TestTheReportNamesEachGroupAndCountsItInTheSummary", `fmt.Fprintf(w, "one cites the other, not a restatement (%d words shared):\n%s", p.shared, p.sites())`, `fmt.Fprintf(w, "rule stated twice (%d words shared):\n%s", p.shared, p.sites())`},
	// One candidate reachable through two of the written forms is one match. Counted twice it reads as
	// ambiguous, and the pointer it belongs to is reported as a restatement.
	{"ruleecho: one file counted once per written form of the citation", "../rule-echo/match.go", "./rule-echo/", "TestOneFileReachedByTwoWrittenFormsIsOneMatch", `if seen[candidate] || !(candidate == form || strings.HasSuffix(candidate, "/"+form)) {`, `if !(candidate == form || strings.HasSuffix(candidate, "/"+form)) {`},
	// A backticked span is as often a command as a path. Read as citations, `report.sh root` and every
	// other quoted token become exemptions handed out at random.
	{"ruleecho: any backticked span read as a citation", "../rule-echo/match.go", "./rule-echo/", "TestCitedTargetsReadsBothFormsAndNothingElse", `if strings.HasSuffix(target, ".md") {`, "if true {"},

	// cadence and comment-density are ports of two shell scripts. Nothing else shows their
	// suites' cases can fail.
	{"cadence: the interval moves out by two days", "../cadence/cadence.go", "./cadence/", "TestTheInterval",
		`const intervalDays = 7`, `const intervalDays = 9`},
	{"cadence: the interval boundary becomes strictly greater", "../cadence/cadence.go", "./cadence/", "TestTheInterval",
		`if elapsed >= intervalDays {`, `if elapsed > intervalDays {`},
	{"cadence: a stamp later than today reads as a not-due", "../cadence/cadence.go", "./cadence/", "TestAFutureStampIsUndetermined",
		`if elapsed < 0 {`, `if elapsed < 0 && false {`},
	{"cadence: the date's shape goes unchecked", "../cadence/cadence.go", "./cadence/", "TestARecordThatIsNoDate",
		`if len(text) != len(dateLayout) {`, `if len(text) != len(dateLayout) && false {`},
	{"cadence: a non-digit in a date position is accepted", "../cadence/cadence.go", "./cadence/", "TestARecordThatIsNoDate",
		`if char < '0' || char > '9' {`, `if (char < '0' || char > '9') && false {`},
	{"cadence: an unknown topic is dispatched anyway", "../cadence/cadence.go", "./cadence/", "TestUsage",
		`if topic != auditTopic {`, `if topic != auditTopic && false {`},
	{"cadence: the record hangs off the per-worktree git dir", "../cadence/cadence.go", "./cadence/", "TestALinkedWorktreeSeesTheMainTreesRecord",
		`"rev-parse", "--git-common-dir"`, `"rev-parse", "--git-dir"`},
	{"cadence: the shared git dir is left relative to the caller's cwd", "../cadence/cadence.go", "./cadence/", "TestRecordingFromASubdirectory",
		`if !filepath.IsAbs(gitDir) {`, `if !filepath.IsAbs(gitDir) && false {`},
	{"cadence: the record is read whole rather than by its first line", "../cadence/cadence.go", "./cadence/", "TestATrailingLineStillResolves",
		`strings.Cut(string(body), "\n")`, `strings.Cut(string(body), "\x00")`},
	{"cadence: a carriage return survives into the stamp", "../cadence/cadence.go", "./cadence/", "TestATrailingLineStillResolves",
		`strings.TrimRight(first, "\r")`, `first`},

	{"density: the ratio bar becomes strictly greater", "../comment-density/density.go", "./comment-density/", "TestTheRatioAndItsFloors",
		`ratio <= s.cfg.MaxRatio`, `ratio < s.cfg.MaxRatio`},
	{"density: the minimum comment-line floor is removed", "../comment-density/density.go", "./comment-density/", "TestTheRatioAndItsFloors",
		`entry.comments < s.cfg.MinLines ||`, `entry.comments < 0 ||`},
	// `pending || true` rather than dropping the name: `pending` is read nowhere else, so removing it
	// from this condition costs it its last use, Go refuses to compile, and the mutant reports `broken`
	// — which says nothing about the guard. The disjunction leaves the name read and the guard gone,
	// which is the edit this mutant means.
	{"density: a file is anchored on the +++ line alone", "../diffscan/diffscan.go", "./comment-density/", "TestAnAddedLineShapedLikeADiffHeader",
		`case isAwaitingPath && strings.HasPrefix(raw, "+++ "):`, `case (isAwaitingPath || true) && strings.HasPrefix(raw, "+++ "):`},
	{"density: prose and data files are counted", "../comment-density/density.go", "./comment-density/", "TestProseDataAndLockfilesAreNotCounted",
		`if line == "" || isProseOrData(file) {`, `if line == "" {`},
	{"density: a bare star counts as a comment", "../comment-density/density.go", "./comment-density/", "TestAStarThatIsNotAComment",
		`return rest == "" || rest[0] == ' ' || rest[0] == '\t'`,
		`return rest == "" || rest[0] == ' ' || rest[0] == '\t' || true`},
	{"density: an option is scanned instead of refused", "../diffscan/diffscan.go", "./comment-density/", "TestARevisionIsNotAPath",
		`if strings.HasPrefix(arg, "-") {`, `if strings.HasPrefix(arg, "-") && false {`},
	{"density: a path is scanned as though it were a revision", "../diffscan/diffscan.go", "./comment-density/", "TestARevisionIsNotAPath",
		"if resolvesAsRevision(cwd, arg) {\n\t\t\tcontinue\n\t\t}",
		"if true {\n\t\t\tcontinue\n\t\t}"},
	{"density: --text is dropped from the diff", "../diffscan/diffscan.go", "./comment-density/", "TestADiffAttributeDoesNotSuppressTheScan",
		`"--text", "--src-prefix=a/", "--dst-prefix=b/",`,
		`"--src-prefix=a/", "--dst-prefix=b/",`},
	{"density: a non-ASCII path arrives C-quoted", "../diffscan/diffscan.go", "./comment-density/", "TestANonASCIIPathIsStillAssigned",
		`"-c", "core.quotePath=false",`, `"-c", "core.quotePath=true",`},
	// The unquoting, which the flag above is no longer the only defence for. git C-quotes a control
	// character whatever core.quotePath says, so this is the half that is observable — and dropping it
	// hides the file from the scan while `diff --git` has already counted it as reached.
	{"density: a C-quoted header path is never unquoted", "../diffscan/diffscan.go", "./comment-density/", "TestATrackedPathWithAControlCharacterIsStillAssigned",
		"if strings.HasPrefix(field, `\"`) {", "if strings.HasPrefix(field, `\"`) && false {"},
	{"density: the untracked half runs even when revisions were named", "../comment-density/density.go", "./comment-density/", "TestATwoRevisionRangeIsScanned",
		`if len(named) == 0 {`, `if len(named) == 0 || true {`},
	{"density: the report is emitted in reverse", "../comment-density/density.go", "./comment-density/", "TestTheReportIsOrdered",
		`sort.Strings(names)`, `sort.Sort(sort.Reverse(sort.StringSlice(names)))`},
	{"density: the display cap is removed", "../comment-density/density.go", "./comment-density/", "TestPastTheDisplayCap",
		`if shown < maxShown {`, `if shown < maxShown || true {`},
	// Aimed at the case that drives an override that PARSES. A suite testing only the refusal path
	// leaves these three alive: the override would parse, be discarded, and every assertion still pass.
	// `+ value*0` rather than a bare default, so `value` keeps its last read and the mutant builds.
	{"density: COMMENT_MAX_RATIO parses and is then discarded", "../comment-density/density.go", "./comment-density/", "TestAThresholdOverrideTakesEffect",
		`cfg.MaxRatio = value`, `cfg.MaxRatio = defaultMaxRatio + value*0`},
	{"density: COMMENT_MIN_LINES parses and is then discarded", "../comment-density/density.go", "./comment-density/", "TestAThresholdOverrideTakesEffect",
		`cfg.MinLines = value`, `cfg.MinLines = defaultMinLines + value*0`},
	{"density: DENSITY_MAX_FILE_BYTES parses and is then discarded", "../comment-density/density.go", "./comment-density/", "TestAThresholdOverrideTakesEffect",
		`cfg.MaxFileBytes = value`, `cfg.MaxFileBytes = defaultMaxFileBytes + value*0`},

	// dup-literals and the half it shares with comment-density. These sit on the two guards a reader of
	// the report cannot check for themselves.
	{"dup: the length floor stops applying to a whole line", "../dup-literals/dup.go", "./dup-literals/", "TestTheLengthFloor",
		`if len([]rune(trimmed)) >= s.cfg.MinLength {`, "if true {"},
	{"dup: a literal appearing once counts as repeated", "../dup-literals/dup.go", "./dup-literals/", "TestASingleOccurrenceIsNotADuplicate",
		"for text, n := range s.tokens {\n\t\tif n >= 2 {", "for text, n := range s.tokens {\n\t\tif n >= 1 {"},
	{"dup: the display cap stops bounding the report", "../dup-literals/dup.go", "./dup-literals/", "TestPastTheDisplayCap",
		"const maxShown = 200", "const maxShown = 100000"},
	// The one that puts a secret in the report. A name-marked file read is a token printed.
	{"diffscan: a secret-named file is read anyway", "../diffscan/diffscan.go", "./dup-literals/", "TestAnUntrackedSecretNamedFileIsNeverRead",
		"if opts.SkipSecretNamed && secretNamed(name) {", "if false {"},
	// The same guard on the other arm: the untracked one covers only what git does not track, and a
	// tracked `.env` reaches the report through the diff. dup.go's own comment carries the measurement.
	{"dup: a secret-named file in the DIFF is scanned anyway", "../dup-literals/dup.go", "./dup-literals/", "TestATrackedSecretNamedFileIsNeverScannedEither",
		"if diffscan.SecretNamed(added.File) {", "if false {"},
	// The other half of that skip: the announcement naming the file it declined. The name is the
	// branch author's, `.env*` matches whatever follows the prefix, and this is the one message
	// diffscan builds itself rather than handing to a scanner that sanitises on the way out. Two
	// mutants, one per claim the treatment makes, and neither drops `shell` from the call — bare, the
	// package loses its only use of that import here and the mutant comes back `broken`.
	{"diffscan: the skip announcement's name left unsanitised", "../diffscan/diffscan.go", "./dup-literals/", "TestTheSkipAnnouncementIsSanitised",
		"shell.CutBytesMarked(shell.Oneline(name), maxAnnouncedPathBytes)", "shell.CutBytesMarked(name, maxAnnouncedPathBytes)"},
	{"diffscan: the skip announcement's name left unbounded", "../diffscan/diffscan.go", "./dup-literals/", "TestTheSkipAnnouncementIsSanitised",
		"shell.CutBytesMarked(shell.Oneline(name), maxAnnouncedPathBytes)", "shell.Oneline(name)"},

	{"diffscan: an oversized diff line is truncated rather than refused", "../diffscan/diffscan.go", "./comment-density/", "TestADiffLinePastTheCapRefusesRatherThanReportingClean",
		"return scanner.Err()", "return nil"},
	{"diffscan: a binary untracked file is read", "../diffscan/diffscan.go", "./dup-literals/", "TestAnUntrackedBinaryFileIsSkippedAndCounted",
		"if isBinary(body) {", "if false {"},

	// `--` and the pathspecs after it are not revisions. Passed through whole, HEAD is never appended
	// and git diffs the INDEX — a clean result over real staged work, at exit 0.
	{"diffscan: the separator counted as a revision", "../diffscan/diffscan.go", "./diffscan/", "TestAPathspecScanStillDefaultsToHead",
		"named, paths := RevisionsNamed(revisions)", "named, paths := revisions, []string(nil)"},
	{"diffscan: the pathspecs dropped from the git call", "../diffscan/diffscan.go", "./diffscan/", "TestAPathspecScanStillDefaultsToHead",
		"args = append(args, paths...)", "_ = paths"},
	{"diffscan: the separator eaten instead of passed to git", "../diffscan/diffscan.go", "./diffscan/", "TestRevisionsNamedSeparatesThemFromPathspecs",
		"return args[:i], args[i:]", "return args[:i], args[i+1:]"},
	// The other way a secret reaches the report: followed instead of read directly. Stat answers about
	// the target, so this mutant is the whole of the symlink defence.
	{"diffscan: an untracked symlink is followed to its target", "../diffscan/diffscan.go", "./dup-literals/", "TestAnUntrackedSymlinkIsSkippedAndCounted",
		"info, err := os.Lstat(full)", "info, err := os.Stat(full)"},

	// The did-not-measure counter. Every guard here decides whether CI keeps passing over guards
	// nothing has proved, which is a thing no reader of a green tick can check for themselves.
	{"nomeasure: the escalation threshold moves out by one", "../nomeasure/nomeasure.go", "./nomeasure/", "TestThreeDidNotMeasureRunsRunningEscalate",
		"const escalateAt = 3", "const escalateAt = 4"},
	{"nomeasure: a status other than the harness's did-not-measure one is counted", "../nomeasure/nomeasure.go", "./nomeasure/", "TestAMeasuredRunClearsTheCount",
		"harnessDidNotMeasure = 2", "harnessDidNotMeasure = 3"},
	// Nine digits or fewer parse, so only the length bound refuses a stored 1234567890 — and without it
	// that entry sits above the threshold and fails the job for good.
	{"nomeasure: a stored count too long to be one is carried on from", "../nomeasure/nomeasure.go", "./nomeasure/", "TestACountThatDoesNotParseIsNoHistory",
		"len(stored) >= 10", "len(stored) >= 99"},
	// `|| true` rather than `return true`: the shorter edit costs `s` its last use, Go refuses to
	// compile it, and the mutant reports `broken` — which proves nothing about the guard.
	{"nomeasure: a status that is no number is accepted as one", "../nomeasure/nomeasure.go", "./nomeasure/", "TestTheArmsThatDecideNothing",
		"return strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) < 0",
		"return strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) < 0 || true"},
	{"nomeasure: a count file that would not take the write is reported as counted", "../nomeasure/nomeasure.go", "./nomeasure/", "TestACountFileThatWillNotTakeTheWriteDecidesNothing",
		"if !wasRecorded(countFile, count, stderr) {", "if !wasRecorded(countFile, count, stderr) && false {"},
	{"nomeasure: a reset that would not take the write is reported as measured", "../nomeasure/nomeasure.go", "./nomeasure/", "TestACountFileThatWillNotTakeTheWriteDecidesNothing",
		"if !wasRecorded(countFile, 0, stderr) {", "if !wasRecorded(countFile, 0, stderr) && false {"},
	{"nomeasure: a status with no count file picks a path instead of refusing", "../nomeasure/nomeasure.go", "./nomeasure/", "TestTheArmsThatDecideNothing",
		"if len(args) < 2 || args[0] == \"\"", "if len(args) < 1 || args[0] == \"\""},

	// The gate's key narrowing. Both directions are guards: dropping too little only costs time, but
	// dropping too much stops the gate watching code a tool is built from, which reads as a clean run.
	{"gate: a compiled-binary unit is keyed on test files after all", "../gate/keys.go", "./gate/", "TestAUnitBlindToGoTestsIsNotKeyedOnThem",
		"if u.blindToGoTests {", "if u.blindToGoTests && false {"},
	{"gate: the narrowing drops every Go file, not only tests", "../gate/keys.go", "./gate/", "TestAUnitBlindToGoTestsIsNotKeyedOnThem",
		`return strings.HasSuffix(path, "_test.go")`, `return strings.HasSuffix(path, ".go")`},
	{"gate: a suite that runs the module's own suites is flagged anyway", "../gate/units.go", "./gate/", "TestOnlyASuiteThatNeverCompilesTheModuleIsBlindToGoTests",
		"if goSuiteRun.Match(body) {", "if goSuiteRun.Match(body) && false {"},
	{"gate: every discovered suite is flagged, marker or not", "../gate/units.go", "./gate/", "TestOnlyASuiteThatNeverCompilesTheModuleIsBlindToGoTests",
		"\t\tviaBinary := false\n", "\t\tviaBinary := true\n"},

	// The lane split. One direction costs only time; the other runs two shell suites at once, and
	// those build temp HOMEs and link into them.
	{"gate: one lane for every unit, so the lanes never overlap", "../gate/run.go", "./gate/", "TestTheShellLaneRunsBesideTheRestAndNeverBesideItself",
		`if strings.HasPrefix(id, "shell:") {`, `if false && strings.HasPrefix(id, "shell:") {`},
	{"gate: a lane per shell suite, so two of them overlap", "../gate/run.go", "./gate/", "TestTheShellLaneRunsBesideTheRestAndNeverBesideItself",
		`return "shell"`, `return "shell" + id`},
	// The property that makes the lane safe to widen at all, and it is about the suites rather than
	// the gate: a suite on a fixed path would race its siblings.
	{"scratch: any suite counts as owning its scratch", "../scratch_isolation_test.go", "./", "TestWhatCountsAsOwningScratch",
		"\tcase strings.Contains(text, \"mktemp -d\"):\n\t\treturn true", "\tcase true:\n\t\treturn true"},
	{"scratch: the no-scratch marker stops exempting", "../scratch_isolation_test.go", "./", "TestWhatCountsAsOwningScratch",
		"\tcase strings.Contains(text, noScratchMarker):\n\t\treturn true", "\tcase strings.Contains(text, noScratchMarker) && false:\n\t\treturn true"},
	// Help is an answer, not a run. Returning 0 from the parser said "these arguments are fine", so
	// help printed and then the whole gate ran, recording verdicts for a caller who asked what the
	// flags were.
	{"gate: help falls through into the run loop", "../gate/gate.go", "./gate/", "TestHelpAnswersWithoutRunningTheGate",
		"if selected == modeHelp {", "if false {"},
	{"gate: help parses as an ordinary run", "../gate/gate.go", "./gate/", "TestHelpAnswersWithoutRunningTheGate",
		"return modeHelp, why, path, 0", "return selected, why, path, 0"},
	// Suite discovery reading names whole. Split on whitespace, a name holding a space became two units
	// keyed on files that do not exist, and the real suite was gated by nothing.
	{"gate: suite names split on whitespace again", "../gate/units.go", "./gate/", "TestASuiteNameHoldingASpaceIsRefusedWholeNotSplit",
		`for _, name := range strings.Split(out, "\x00") {`, "for _, name := range strings.Fields(out) {"},
	{"gate: the -z flag dropped from discovery", "../gate/units.go", "./gate/", "TestASuiteNameHoldingASpaceIsRefusedWholeNotSplit",
		`"ls-files", "-z",`, `"ls-files",`},

	// `wiring` is blind to the module's test files because eco-check skips them, which is a claim
	// about ANOTHER package. Both halves get a mutant: the flag itself, and the check that eco-check
	// still behaves the way the flag assumes.
	{"gate: wiring keyed on the module's test files again", "../gate/units.go", "./gate/", "TestWiringIsBlindToGoTestsAndEcoCheckStillSkipsThem",
		`g.addBlindToGoTests("wiring"`, `g.add("wiring"`},
	{"gate: gotest goes blind to the tests it runs", "../gate/units.go", "./gate/", "TestWiringIsBlindToGoTestsAndEcoCheckStillSkipsThem",
		`g.add("gotest", "check", gotestInputs, "@gotest")`, `g.addBlindToGoTests("gotest", "check", gotestInputs, "@gotest")`},

	{"scratch: any sourced file counts as a harness", "../scratch_isolation_test.go", "./", "TestWhatCountsAsOwningScratch",
		`if err == nil && strings.Contains(string(body), "mktemp -d") {`, `if err == nil && strings.Contains(string(body), "") {`},
	{"gate: the report printed in completion order", "../gate/run.go", "./gate/", "TestTheReportKeepsDeclaredOrderWhicheverLaneFinishesFirst",
		"\tfor _, sl := range slots {\n\t\t<-sl.done\n", "\tfor i := len(slots) - 1; i >= 0; i-- {\n\t\tsl := slots[i]\n\t\t<-sl.done\n"},
}

// A mutant no case can redden, and why: a guard whose triggering condition an earlier guard already
// refuses cannot be observed from outside, and a mutation of it that changes nothing observable
// cannot be killed by any case anyone could write.
//
// Declaring one is what makes every OTHER survivor a failure of this run. Undeclared, they print in
// the same column, the run ends "N that proved nothing" and exits 1 every single time — and a column
// that is never clean is one its readers learn to discount, which is how a harness stops measuring
// without anyone deciding to switch it off.
//
// Three things keep this from becoming where a hard mutant goes to be forgotten. The reason has to
// say why no case reaches it AND what does stand behind the guard, so the next reader can
// re-litigate it. Preflight refuses a declaration naming a mutant this list does not carry. And a
// declared mutant that IS killed fails the run as a stale claim — the guard became observable, so the
// declaration is now a lie about the suite.
type unreachableMutant struct {
	label string
	why   string
}

var unreachableMutants = []unreachableMutant{
	{
		"cadence: a non-digit in a date position is accepted",
		"equivalent, not unobserved: the per-byte loop only ever rejects, and every input it rejects " +
			"time.ParseInLocation rejects too. On the ten-byte string the length guard has already " +
			"admitted, the layout `2006-01-02` demands a digit at each of the eight non-separator offsets " +
			"— stdLongYear tests isDigit and then atoi, which refuses a four-byte year it cannot consume " +
			"whole, and month and day go through getnum in its fixed two-digit form. Checked rather than " +
			"argued: all 256 byte values were substituted into a valid date at each of the eight offsets, " +
			"singly and in every pair of offsets, and the guarded and unguarded parsers agreed on every " +
			"one of the 2,099,200 inputs. No case can tell them apart because there is nothing to tell " +
			"apart. What stands behind the guard is TestARecordThatIsNoDate, whose length, separator and " +
			"calendar cases drive every refusal parseDate can actually make — including the over-length " +
			"stamp that reaches the length test from above, where the loop would index the layout past " +
			"its end.",
	},
	{
		"density: a non-ASCII path arrives C-quoted",
		"equivalent, not unobserved, and only since headerPath landed: the +++ field is unquoted with " +
			"strconv.Unquote before the b/ prefix is tested, so both settings of the flag resolve to the same " +
			"path and no case can tell them apart. It did kill this case before that. Checked rather than " +
			"argued: git's quote_c_style escapes only the seven C control escapes, the double quote, the " +
			"backslash, and everything else as three-digit octal — every one of which is also a Go string " +
			"escape — and strconv.Unquote was run over each form it can emit, an accented name, an emoji, an " +
			"embedded quote, a backslash, DEL, a lone 0xff and a raw 0x9b, recovering the exact bytes every " +
			"time, invalid UTF-8 included. The flag stays rather than going with its mutant, because it keeps " +
			"the common non-ASCII path unquoted instead of round-tripping it through an escape form. What " +
			"stands behind the guard is TestATrackedPathWithAControlCharacterIsStillAssigned, which drives " +
			"the quoted spelling head-on: git C-quotes a control character whatever core.quotePath says, so " +
			"the parser has to read that form either way, and that case is what holds it to doing so — and " +
			"the unquoting now carries its own mutant beside this flag, which that same case kills.",
	},
	{
		"gate: the filtered tree rebuilt from a literal",
		"unobservable because nothing reads the copied fields after the walk. keepCommittable copies " +
			"the walked tree and resets `entries` and `suffixes`; the other two fields, `start` and " +
			"`canonicalRoot`, are read by newEntry during the walk alone, while add keys each entry on " +
			"the `canonical` name that entry already carries. So a filtered copy rebuilt from a literal " +
			"loses both and still produces byte-identical findings. " +
			"The condition is a field READ AT FILTER TIME, never merely a field on `tree`: `start` " +
			"exists and this mutant still kills nothing. Measured both ways — dropping those fields " +
			"from newWalk reddens TestAPathRefThroughTheRootsOwnNameResolvesHoweverTheRootIsNamed, and " +
			"dropping them from the filtered copy reddens no case at all. A STALE CLAIM here means " +
			"keepCommittable or add has come to read something a literal would drop, and the guard is " +
			"load-bearing from that moment: delete THIS ENTRY and keep the mutant. Do not revert " +
			"keepCommittable and do not delete the mutant.",
	},
	{
		"override config: an unreadable config passes as a checked one",
		"unreachable behind an earlier guard: os.Lstat can only fail here for a path that " +
			"shell.IsRegularFile and isReadable both accepted two statements earlier, so reaching it needs " +
			"the file to change between those calls and this one — a race no case can stage " +
			"deterministically. The guard is kept because its sibling fifty lines down refuses on the same " +
			"fact, and one of the two accepting silently is the asymmetry that let a permission this tool " +
			"could not read pass as one it checked. What stands behind it is " +
			"TestTheOverrideConfigIsJudgedLikeTheRootItNames, whose four other cases drive every " +
			"permission path that IS constructible, and TestAnUntrustworthyOverrideRootIsRefused, which " +
			"reaches the sibling's identical branch through a root that need not exist.",
	},
	{
		"stage block: a no-items marker blocks the stamp",
		"equivalent, not unobserved: with the guard off, `recorded` is the literal \"no-items\" and the " +
			"next arm compares it against a `\"<crc> <size>\"` checksum, which it can never equal, so " +
			"stageBlockReason returns the same empty string either way. No case can tell the two apart " +
			"because there is nothing to tell apart. What stands behind the guard is " +
			"TestNoItemsDemandsTheStageHaveReturnedFirst, which drives the same path for its effect.",
	},
	{
		"marker: an unchecksummable report marked anyway",
		"unreachable behind an earlier guard: the empty checksum it refuses can only come from a report " +
			"os.ReadFile could not open, and every caller reaches writeStageMarker through requireReport, " +
			"whose assertReportIsReadable refuses first. It is defence in depth for a state the tool " +
			"cannot currently be in. What stands behind the marker write is " +
			"TestAMarkerIsThePosixCksumOfTheReport, whose unwritable-marker case reaches the two failure " +
			"paths below it.",
	},
}

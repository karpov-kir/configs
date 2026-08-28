// Mutation testing for the Go checker: break one guard at a time and require the suite to notice.
//
//	usage: gomutate [-jobs N] [-preflight] [-run <substring>]
//
// A mutant costs one package compile and one in-process test run, never a checkout: `go build
// -overlay` swaps a file's content without copying the module or touching the tree, and `-failfast`
// stops at the first red case, which is all a mutant has to prove.
//
// The mutants live here rather than beside the code: each names a file, a search string and its
// replacement, and preflight refuses any that no longer matches exactly once.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

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
	{"direction: cites bound removed", "direction.go", "./eco-check/", "TestDirectionScan", "counters.cites <= findingCap", "counters.cites <= 100000"},
	{"direction: names bound removed", "direction.go", "./eco-check/", "TestDirectionScan", "counters.names <= findingCap", "counters.names <= 100000"},
	{"direction: basename bound removed", "direction.go", "./eco-check/", "TestDirectionScan", "counters.basenames <= findingCap", "counters.basenames <= 100000"},
	{"direction: unchecked notice unbounded", "direction.go", "./eco-check/", "TestDirectionScan", "counters.ambiguous <= findingCap", "counters.ambiguous <= 100000"},
	{"report: per-class cap removed", "report.go", "./eco-check/", "TestImportResolvedAtTheMount", "shown[r] <= findingCap", "shown[r] <= 100000"},
	{"shell: per-file byte bound removed", "shell.go", "./eco-check/", "TestOversizeFileIsReportedNotRead", "info.Size() > maxFileBytes", "info.Size() > (1 << 62)"},
	{"refs: citation target read with no regular-file test", "refs.go", "./eco-check/", "TestCitationTargetMustBeARegularFile", "if !shell.IsRegularFile(target) {", "if false {"},
	// Both directions off the bare-rule-ID scan's single pattern, which is why they share an anchor.
	// The second is aimed at the quiet case alone: it widens the separator and the phrase just far
	// enough to swallow the delimited citation that finding recommends writing, and leaves the three
	// cases that assert a finding green. A mutant that also broke those would let their failure stand
	// in for the quiet one's, which proves nothing about it.
	{"refs: bare rule-ID scan never fires", "refs.go", "./eco-check/", "TestBareRuleIDCitations", `[Cc]ore [Pp]rinciples? +#?[0-9]+`, `[Zz]ore [Pp]rinciples? +#?[0-9]+`},
	{"refs: bare rule-ID scan reports the form it recommends", "refs.go", "./eco-check/", "TestBareRuleIDCitations", `[Cc]ore [Pp]rinciples? +#?[0-9]+`, `[Cc]ore[ -][Pp]rinciples?[^0-9]*[0-9]+`},
	{"scripts: parse-error text left unsanitised", "scripts.go", "./eco-check/", "TestParseErrorsCarryNoControlByte", `"syntax: "+shell.Oneline(line)`, `"syntax: "+line`},
	{"mounts: resolved mount path left unsanitised", "mounts.go", "./eco-check/", "TestMountFindingCarriesNoControlByte", "shell.Oneline(flavorHave)", "flavorHave"},

	// The delimited-citation half. Undelimited is how a section citation stops resolving in silence,
	// so both directions are needed: the finding never firing, and it firing on the two forms the
	// finding itself recommends.
	{"citations: the undelimited form unreported", "refs.go", "./eco-check/", "TestDelimitedSectionCitations", "if !cited.isDelimited {", "if false {"},
	{"citations: the delimited forms reported as undelimited", "refs.go", "./eco-check/", "TestDelimitedSectionCitations", "isDelimited = section != \"\"", "isDelimited = false"},

	// The skill-directory scan. Each of the three defects makes a skill unreachable rather than
	// merely mis-linked, and each has exactly one case behind it.
	{"skills: a directory carrying no SKILL.md unreported", "skills.go", "./eco-check/", "TestSkillDirectory", `if !shell.IsRegularFile(shell.Join(entry.path, "SKILL.md")) {`, "if false {"},
	{"skills: a name/dir mismatch unreported", "skills.go", "./eco-check/", "TestSkillDirectory", "if declared != shell.BaseName(shell.DirName(file)) {", "if false {"},
	{"skills: a SKILL.md with no description unreported", "skills.go", "./eco-check/", "TestSkillDirectory", `if shell.FrontmatterDescription(lines) == "" {`, "if false {"},

	// The test-position scan. Each aims at one of the ways a script can hide from kk-reduce's Phase 6:
	// a header that declares nothing, one naming a suite that is not there, and the four bounds that
	// keep a crafted header from being read as a declaration.
	//
	// `if len(named) > 8 {` matches twice, once for the report and once for the truncation, so the
	// report's anchor carries the line after it.
	//
	// Two guards here have no mutant, both because nothing observes them. The truncation to eight
	// names is one: no case counts the findings a truncated list produces, and the *report* of an
	// over-long header is asserted separately, so the cut itself is invisible. The other is the
	// `isCleanBasename` filter on the suite map, and that one cannot be given a case at all: the names
	// it excludes are exactly the names `namedTestSuite` cannot extract from a header, so no excluded
	// key could ever have matched one. It guards against a `find` listing split on newlines, which
	// os.ReadDir does not produce.
	{"test position: over-long header not reported", "scripts.go", "./eco-check/", "TestScriptTestPosition", "if len(named) > 8 {\n\t\t\tc.add(", "if false {\n\t\t\tc.add("},
	{"test position: header bound removed", "scripts.go", "./eco-check/", "TestScriptTestPosition", "if i >= 200 {", "if i >= 100000 {"},
	{"test position: header read past the comment block", "scripts.go", "./eco-check/", "TestScriptTestPosition", `if !strings.HasPrefix(line, "#") {`, "if false {"},
	{"test position: the -mutate.sh exemption removed", "scripts.go", "./eco-check/", "TestScriptTestPosition", `|| strings.HasSuffix(base, "-mutate.sh")`, "|| false"},
	{"test position: a named suite that is absent unreported", "scripts.go", "./eco-check/", "TestScriptTestPosition", "if !suites[suite] {", "if false {"},
	{"test position: a script declaring nothing unreported", "scripts.go", "./eco-check/", "TestScriptTestPosition", "if !anyMatch(header, untestedDeclared) {", "if false {"},
	{"test position: a bare untested: clears the check", "scripts.go", "./eco-check/", "TestScriptTestPosition", `untested:[[:space:]]*[^[:space:]]`, `untested:[[:space:]]*`},
	{"test position: a dash-led suite name goes unread", "scripts.go", "./eco-check/", "TestScriptTestPosition", `[A-Za-z0-9_.-]+-test\.sh`, `[A-Za-z0-9_.]+-test\.sh`},

	// The subcommand call-site scan's Go half. The first mutant below removes the Go dispatch
	// entirely, which is this scan going quiet rather than failing.
	{"subcommands: the Go dispatch never consulted", "subcommands.go", "./eco-check/", "TestGoDispatchSubcommandCallSites", "want(base, c.toolSubcommands(base, lines))", "want(base, nil)"},
	{"subcommands: a missing tool source goes quiet", "subcommands.go", "./eco-check/", "TestADispatchThatCannotBeReadIsReported", `return nil, "no source directory at " + named`, `return nil, ""`},
	{"subcommands: a source with no dispatch goes quiet", "subcommands.go", "./eco-check/", "TestADispatchThatCannotBeReadIsReported", `return nil, "no switch under " + named + " refuses with a '" + shell.Oneline(marker) + "' line"`, `return nil, ""`},
	{"subcommands: any switch read as the dispatch", "subcommands.go", "./eco-check/", "TestGoDispatchSubcommandCallSites", "carries = carries || strings.Contains(line, marker)", "carries = true"},
	{"subcommands: the usage grammar read one line only", "subcommands.go", "./eco-check/", "TestGoDispatchSubcommandCallSites", "if closed || len(grammar) > 4096 {", "if closed || len(grammar) > 0 {"},
	{"subcommands: count bound removed", "subcommands.go", "./eco-check/", "TestSubcommandCountIsBounded", "if len(queries) >= subcommandCap {", "if len(queries) >= 100000 {"},
	{"subcommands: a name only the dispatch has goes unreported", "subcommands.go", "./eco-check/", "TestUsageAndDispatchAreHeldAgainstEachOther", "onlyIn(dispatched, documented)", "onlyIn(dispatched, dispatched)"},
	{"subcommands: a name only the usage has goes unreported", "subcommands.go", "./eco-check/", "TestUsageAndDispatchAreHeldAgainstEachOther", "onlyIn(documented, dispatched)", "onlyIn(documented, documented)"},

	// The rest live outside ecocheck/. A mutant names its file relative to that directory, and the
	// overlay reaches a dependency, or a sibling package, as readily as the package under test.
	// ecoroot holds the `@import` scan and the mount: they are facts about one checkout, not shell
	// primitives, so they sit with the root every path is built from.
	{"imports: name cut a fixed two bytes past the boundary", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", "token[at[0]+boundary+1:at[1]]", "token[at[0]+boundary*0+2:at[1]]"},
	{"imports: uncounted name left unsanitised", "../eco-root/imports.go", "./eco-check/", "TestUncountedNoteCarriesNoControlByte", "shell.CutBytes(shell.Oneline(name), 60)", "shell.CutBytes(name, 60)"},
	// The three caps on the naming half of that note, which shares its anchor with the mutant above:
	// a mutant is refused when *its own* anchor matches twice, and two mutants aiming at one line are
	// two separate questions about it.
	{"imports: uncounted names not capped in entries", "../eco-root/imports.go", "./eco-check/", "TestUncountedNamesAreCapped", "if len(shown) > 10 {", "if len(shown) > 100000 {"},
	{"imports: the uncounted list not capped in bytes", "../eco-root/imports.go", "./eco-check/", "TestUncountedNamesAreCapped", "shell.CutBytes(joined.String(), 200)", "joined.String()"},
	{"imports: one uncounted name not capped in bytes", "../eco-root/imports.go", "./eco-check/", "TestUncountedNamesAreCapped", "shell.CutBytes(shell.Oneline(name), 60)", "shell.Oneline(name)"},
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
	// The collapse itself moved into shell.Oneline, which is where the mutant has to follow it: aimed
	// at the old inline copy it matched nothing, and preflight refuses the whole run over one stale
	// anchor — so this entry alone had the entire list running nowhere. Oneline writes through a
	// strings.Builder, so it is the case label that carries the control bytes and not an `out[i]`
	// assignment; the label is also the only form that matches once, since the NBSP arm below shares
	// the `out.WriteByte(' ')` body.
	{"stats: no newline collapse in the note", "../shell/text.go", "./eco-stats/", "TestTheNoteCannotForgeALedgerRow", "case b < 0x20 || b == 0x7f:", "case b == 0x7f:"},
	{"stats: no pipe escaping in the note", "../eco-stats/eco-stats.go", "./eco-stats/", "", "strings.ReplaceAll(note, \"|\", `\\|`)", "strings.ReplaceAll(note, \"|\", \"|\")"},
	{"stats: no note-length bar", "../eco-stats/eco-stats.go", "./eco-stats/", "", "words > noteWordCap", "words > 100000"},
	{"stats: import refusals unreported", "../eco-stats/budget.go", "./eco-stats/", "", `fmt.Fprintf(errOut, "stats.sh: import refused`, `fmt.Fprintf(io.Discard, "stats.sh: import refused`},
	// The name and the path both, because the path is built from the name: sanitising one and printing
	// the other through would leave the ESC byte on the line anyway.
	{"stats: Read-always target left unsanitised", "../eco-stats/budget.go", "./eco-stats/", "TestAMissingReadAlwaysTargetCannotReachTheTerminalRaw", "shell.Oneline(target), shell.Oneline(file))", "target, file)"},
	{"stats: ledger not taken out of prose", "../eco-stats/measure.go", "./eco-stats/", "", "s.prose -= s.ledgerWords", "s.prose -= 0"},
	{"stats: ledger figure unreported", "../eco-stats/report.go", "./eco-stats/", "", `fmt.Fprintf(out, "ledger:`, `fmt.Fprintf(io.Discard, "ledger:`},
	{"stats: mounted-outside unreported", "../eco-stats/report.go", "./eco-stats/", "", `fmt.Fprintf(out, "mounted outside:`, `fmt.Fprintf(io.Discard, "mounted outside:`},
	{"stats: mounted-outside gate removed", "../eco-stats/budget.go", "./eco-stats/", "", "if !s.root.IsInstalled() {", "if false {"},
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
	// that happened. Its suite runs in seven seconds whole, so naming the test here buys attribution
	// rather than budget: "some case went red" would not say which guard is observed.

	// paths.go — which report an invocation acts on, and what else on disk belongs to the ship it
	// names. Every path is built from a slug, so the charset check is the whole of what keeps a write
	// inside qualify-reports/.
	// SURVIVOR. No case gives an `intent:` line a value with leading spaces, so the trim is never
	// observed. The case to write: `intent:   001-x` names the same ship as `intent: 001-x`.
	{"report name: the leading-space trim dropped", "../eco-report/paths.go", "./eco-report/", "TestTheFilenameAndTheFrontmatterNameTheSameShip", "firstField(trimLeadingSpace(value))", "firstField(value)"},
	{"report name: a standalone review has no stem of its own", "../eco-report/paths.go", "./eco-report/", "TestAnExistingReportIsNotSilentlyReplaced", `case slug == "" || strings.HasPrefix(slug, "review:"):`, `case slug == "":`},
	// Both halves of the one arm that keeps a value out of a path: the dot that no listing can see,
	// and the charset that stops a `/` reaching one.
	{"report name: a leading dot no longer refused", "../eco-report/paths.go", "./eco-report/", "TestAnIntentValueCannotNameAFileOutsideQualifyReports", `case strings.HasPrefix(slug, "."), !isSlugCharset(slug):`, `case !isSlugCharset(slug):`},
	{"report name: the slug charset no longer refused", "../eco-report/paths.go", "./eco-report/", "TestAnIntentValueCannotNameAFileOutsideQualifyReports", `case strings.HasPrefix(slug, "."), !isSlugCharset(slug):`, `case strings.HasPrefix(slug, "."):`},
	{"report listing: a dot-named report joins the listing", "../eco-report/paths.go", "./eco-report/", "TestADotNamedReportIsInvisibleToEveryDiscoveryPath", `if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, reportSuffix) {`, `if !strings.HasSuffix(name, reportSuffix) {`},
	{"report listing: a directory counted as a report", "../eco-report/paths.go", "./eco-report/", "TestADotNamedReportIsInvisibleToEveryDiscoveryPath", `if !shell.IsRegularFile(r.reportsDir + "/" + name) {`, "if false {"},
	{"resolve: several open reports resolved to one", "../eco-report/paths.go", "./eco-report/", "TestStateNeverAnswersATokenItCannotStandBehind", "if len(names) != 1 {", "if false {"},
	{"resolve: a name that could name no report resolved anyway", "../eco-report/paths.go", "./eco-report/", "TestASubcommandRefusesAnIntentNameThatCouldNameNoReport", `if stem == "" {`, "if false {"},
	{"require report: a named report that is not there read anyway", "../eco-report/paths.go", "./eco-report/", "TestThePreScopingPathIsReportedNeverPassedOverInSilence", "if !shell.IsRegularFile(r.report) {", "if false {"},
	{"readable: an unreadable report read as one that is there", "../eco-report/paths.go", "./eco-report/", "TestAnUnreadableReportIsNotAState", "if !isReadable(r.report) {", "if false {"},
	{"legacy note: never emitted", "../eco-report/paths.go", "./eco-report/", "TestInitIsWhereALegacyReportGetsMentioned", "if shell.PathExists(path) {", "if false {"},
	{"legacy note: the pre-rename directory unnamed", "../eco-report/paths.go", "./eco-report/", "TestThePreScopingPathIsReportedNeverPassedOverInSilence", `[]string{r.root + "/.idsd/ship-report.md", r.root + "/.idsd/ship-reports"}`, `[]string{r.root + "/.idsd/ship-report.md"}`},
	// The ship-exists guard, then each of the three things that satisfy it. Removed whole, `discard
	// <any-legal-slug>` deletes at exit 0 in a repo that never used idsd.
	{"discard: the ship-exists guard never refuses", "../eco-report/paths.go", "./eco-report/", "TestDiscardDeletesNothingForAShipThatIsNotHere", "func (r *run) assertShipExists(slug string) {\n\tif shell.IsRegularFile(r.report) {", "func (r *run) assertShipExists(slug string) {\n\tif true {"},
	{"discard: an intent file no longer identifies a closed ship", "../eco-report/paths.go", "./eco-report/", "TestDiscardDeletesNothingForAShipThatIsNotHere", `if shell.IsRegularFile(r.root+"/.idsd/intents/"+slug+".md") || shell.IsRegularFile(r.root+"/.idsd/archive/"+slug+".md") {`, "if false {"},
	{"discard: the review exception removed", "../eco-report/paths.go", "./eco-report/", "TestAStandaloneReviewCanStillBeTornDownAfterItIsClosed", `if slug == "review" {`, "if false {"},
	// The durable four are a table, and a table gets a row each: three of them kept .idsd/ standing
	// with nothing observing that they did.
	{"surviving: charter.md no longer keeps .idsd/", "../eco-report/paths.go", "./eco-report/", "TestDiscardDestructivePath", `[]string{"charter.md", "constitution.md", "language.md", "playbook.md"}`, `[]string{"constitution.md", "language.md", "playbook.md"}`},
	{"surviving: constitution.md no longer keeps .idsd/", "../eco-report/paths.go", "./eco-report/", "TestEveryDurableFileKeepsIdsdStanding", `[]string{"charter.md", "constitution.md", "language.md", "playbook.md"}`, `[]string{"charter.md", "language.md", "playbook.md"}`},
	{"surviving: language.md no longer keeps .idsd/", "../eco-report/paths.go", "./eco-report/", "TestEveryDurableFileKeepsIdsdStanding", `[]string{"charter.md", "constitution.md", "language.md", "playbook.md"}`, `[]string{"charter.md", "constitution.md", "playbook.md"}`},
	{"surviving: playbook.md no longer keeps .idsd/", "../eco-report/paths.go", "./eco-report/", "TestEveryDurableFileKeepsIdsdStanding", `[]string{"charter.md", "constitution.md", "language.md", "playbook.md"}`, `[]string{"charter.md", "constitution.md", "language.md"}`},
	{"surviving: a parallel ship's report no longer counted", "../eco-report/paths.go", "./eco-report/", "TestDiscardDestructivePath", "if left := len(r.reportNames()); left != 0 {", "if left := len(r.reportNames()); left < 0 {"},
	{"surviving: another ship's intent file no longer counted", "../eco-report/paths.go", "./eco-report/", "TestEveryDurableFileKeepsIdsdStanding", "if shell.PathExists(intents) || shell.PathExists(archive) {", "if false {"},
	{"surviving: stray content counted as intents", "../eco-report/paths.go", "./eco-report/", "TestEveryDurableFileKeepsIdsdStanding", "if left := countMarkdownFiles(intents, archive); left > 0 {", "if left := countMarkdownFiles(intents, archive); left >= 0 {"},
	{"markdown count: a symlink counted as an intent file", "../eco-report/paths.go", "./eco-report/", "TestEveryDurableFileKeepsIdsdStanding", `if err != nil || !info.Mode().IsRegular() || !strings.HasSuffix(entry.Name(), ".md") {`, `if err != nil || info.Mode().IsDir() || !strings.HasSuffix(entry.Name(), ".md") {`},
	{"markdown count: anything counted as an intent file", "../eco-report/paths.go", "./eco-report/", "TestEveryDurableFileKeepsIdsdStanding", `if err != nil || !info.Mode().IsRegular() || !strings.HasSuffix(entry.Name(), ".md") {`, `if err != nil || !info.Mode().IsRegular() {`},
	// A symlink test reads the final component only, so all three of the write's path components need
	// one, and each has its own case.
	{"write paths: .idsd no longer tested for a link", "../eco-report/paths.go", "./eco-report/", "TestTheDestructivePathCarriesTheGuardsTheWritePathHas", `[]string{r.root + "/.idsd", r.reportsDir}`, `[]string{r.reportsDir}`},
	// SURVIVOR. The symlink cases point `.idsd` elsewhere; none points qualify-reports/ itself, which
	// is why dropping the outer path kills and dropping the inner one does not. The case to write:
	// init refuses a symlinked `.idsd/qualify-reports/` rather than writing the report through it.
	{"write paths: qualify-reports/ no longer tested for a link", "../eco-report/paths.go", "./eco-report/", "TestInitRefusesRatherThanWritingThroughALink", `[]string{r.root + "/.idsd", r.reportsDir}`, `[]string{r.root + "/.idsd"}`},
	{"write paths: the report itself no longer tested for a link", "../eco-report/paths.go", "./eco-report/", "TestInitRefusesRatherThanWritingThroughALink", "if shell.IsSymlink(r.report) {", "if false {"},
	{"stage markers: not keyed by the report stem", "../eco-report/paths.go", "./eco-report/", "TestTwoIntentsShipSideBySide", `r.gitPath("idsd-stage-returns/" + name)`, `r.gitPath("idsd-stage-returns")`},

	// frontmatter.go — the three lines every later reader greps, and the rewrites that write them.
	{"frontmatter: a body line read as a field", "../eco-report/frontmatter.go", "./eco-report/", "TestAStampRewritesTheFrontmatterAndNothingElse", "if strings.HasPrefix(line, prefix) {", "if strings.Contains(line, prefix) {"},
	// One set, read whole by every reader. A reader knowing only some of it accepts what the others
	// reject, which is a report reading as reviewed to `gate` and unreviewed to `state`.
	{"unstamped: 'pending' reads as a completed review", "../eco-report/frontmatter.go", "./eco-report/", "TestTwoIntentsShipSideBySide", `case "", "pending", "<hash>", "<stages>":`, `case "", "<hash>", "<stages>":`},
	{"unstamped: the template's <hash> reads as a completed review", "../eco-report/frontmatter.go", "./eco-report/", "TestTheFilenameAndTheFrontmatterNameTheSameShip", `case "", "pending", "<hash>", "<stages>":`, `case "", "pending", "<stages>":`},
	{"unstamped: the template's <stages> reads as a stage record", "../eco-report/frontmatter.go", "./eco-report/", "TestGateBlocksOnEachOfItsReasonsAndClearsOnNone", `case "", "pending", "<hash>", "<stages>":`, `case "", "pending", "<hash>":`},
	{"unstamped: an absent field reads as a completed review", "../eco-report/frontmatter.go", "./eco-report/", "TestGateBlocksOnEachOfItsReasonsAndClearsOnNone", `case "", "pending", "<hash>", "<stages>":`, `case "pending", "<hash>", "<stages>":`},
	{"fast trims: a turnaround trim no longer trims", "../eco-report/frontmatter.go", "./eco-report/", "TestATrimmedPassIsNotAFullOne", `strings.Contains(entry, "(fast)")`, `strings.Contains(entry, "(FAST)")`},
	{"intent slug: the charset no longer bounds the slug", "../eco-report/frontmatter.go", "./eco-report/", "TestAHandEditedIntentCannotSteerAPathOutOfIdsd", `if slug == "" || strings.HasPrefix(slug, "review:") || !isSlugCharset(slug) {`, `if slug == "" || strings.HasPrefix(slug, "review:") {`},
	{"template: a symlinked template read", "../eco-report/frontmatter.go", "./eco-report/", "TestADriftedTemplateIsRefusedBeforeAnyReportIsScaffolded", "if shell.IsSymlink(r.template) {", "if false {"},
	{"template: a missing template not named as the cause", "../eco-report/frontmatter.go", "./eco-report/", "TestADriftedTemplateIsRefusedBeforeAnyReportIsScaffolded", "if !shell.IsRegularFile(r.template) {", "if false {"},
	{"template: no intent: line to stamp", "../eco-report/frontmatter.go", "./eco-report/", "TestADriftedTemplateIsRefusedBeforeAnyReportIsScaffolded", `if !hasField(r.template, "intent") {`, "if false {"},
	{"template: reviewed-tree no longer required", "../eco-report/frontmatter.go", "./eco-report/", "TestADriftedTemplateIsRefusedBeforeAnyReportIsScaffolded", `[]string{"reviewed-tree", "reviewed-stages"}`, `[]string{"reviewed-stages"}`},
	{"template: reviewed-stages no longer required", "../eco-report/frontmatter.go", "./eco-report/", "TestADriftedTemplateIsRefusedBeforeAnyReportIsScaffolded", `[]string{"reviewed-tree", "reviewed-stages"}`, `[]string{"reviewed-tree"}`},
	{"template: a drifted placeholder accepted", "../eco-report/frontmatter.go", "./eco-report/", "TestADriftedTemplateIsRefusedBeforeAnyReportIsScaffolded", "if !isUnstamped(placeholder) {", "if false {"},
	{"frontmatter map: the closing delimiter never read", "../eco-report/frontmatter.go", "./eco-report/", "TestAStampRewritesTheFrontmatterAndNothingElse", "if i > 0 && shell.IsFrontmatterDelimiter(line) {", "if i > 0 && false {"},
	{"frontmatter map: the frontmatter never opens", "../eco-report/frontmatter.go", "./eco-report/", "TestAStampCannotOutliveThePassThatEarnedIt", "if i == 0 {", "if false {"},
	{"intent rewrite: every intent: line replaced, not the first", "../eco-report/frontmatter.go", "./eco-report/", "TestAStampRewritesTheFrontmatterAndNothingElse", "replaced = true", "replaced = false"},
	{"stamp rewrite: the old reviewed-stages line left standing", "../eco-report/frontmatter.go", "./eco-report/", "TestAStampRewritesTheFrontmatterAndNothingElse", `case strings.HasPrefix(line, "reviewed-mode:"), strings.HasPrefix(line, "reviewed-stages:"):`, `case strings.HasPrefix(line, "reviewed-mode:"):`},
	{"stamp rewrite: an old layout's reviewed-mode left to be read", "../eco-report/frontmatter.go", "./eco-report/", "TestAStampRewritesTheFrontmatterAndNothingElse", `case strings.HasPrefix(line, "reviewed-mode:"), strings.HasPrefix(line, "reviewed-stages:"):`, `case strings.HasPrefix(line, "reviewed-stages:"):`},
	{"invalidate: the stage record left stamped", "../eco-report/frontmatter.go", "./eco-report/", "TestInvalidateClearsThePassItStarts", "case strings.HasPrefix(line, \"reviewed-stages:\"):\n\t\t\treturn []string{\"reviewed-stages: pending\"}", "case strings.HasPrefix(line, \"reviewed-stages:\"):\n\t\t\treturn []string{line}"},

	// git.go — every git call the tool makes, and the two exclusion mechanisms it writes through.
	// SURVIVOR. Every fixture's git dir comes back relative, so the absolute arm never runs. The case
	// to write: a linked worktree, whose git dir git answers absolutely, still gets its info/exclude.
	{"git dir: an absolute git path prefixed with the root", "../eco-report/git.go", "./eco-report/", "TestIgnoredMeansIgnoredForEveryoneNotJustThisMachine", `if strings.HasPrefix(path, "/") {`, "if false {"},
	{"repo mode: a tracked .idsd read as throwaway", "../eco-report/git.go", "./eco-report/", "TestDiscardDestructivePath", `if tracked != "" {`, `if tracked != "" && false {`},
	{"repo mode: an unreadable index read as a mode", "../eco-report/git.go", "./eco-report/", "TestPromoteAndCheckIgnoreAlsoRefuseAnUnreadableIndex", `if _, status := r.captureGit(nil, "ls-files", ".idsd"); status != 0 {`, "if false {"},
	// The arm order is load-bearing, so the two forms of info/exclude are asked separately: the
	// relative one an ordinary repo has, and the absolute one a linked worktree has.
	{"ignore source: info/exclude no longer travels", "../eco-report/git.go", "./eco-report/", "TestTwoIntentsShipSideBySide", `case source == ".git/info/exclude" || strings.HasSuffix(source, "/.git/info/exclude"):`, "case false:"},
	{"ignore source: a worktree's absolute info/exclude rejected", "../eco-report/git.go", "./eco-report/", "TestIgnoredMeansIgnoredForEveryoneNotJustThisMachine", `case source == ".git/info/exclude" || strings.HasSuffix(source, "/.git/info/exclude"):`, `case source == ".git/info/exclude":`},
	{"ignore source: a machine-local exclude counted as ignoring", "../eco-report/git.go", "./eco-report/", "TestAGlobalExcludeDoesNotCountAsIgnoringTheReport", "case strings.HasPrefix(source, \"/\"):\n\t\treturn source, false", "case strings.HasPrefix(source, \"/\"):\n\t\treturn source, true"},
	{"ignore source: .gitignore no longer travels", "../eco-report/git.go", "./eco-report/", "TestInitWillNotWriteAReportIntoItsOwnFingerprint", `case source == ".gitignore" || strings.HasSuffix(source, "/.gitignore"):`, "case false:"},
	{"ignore surface: the trailing slash dropped", "../eco-report/git.go", "./eco-report/", "TestCheckIgnoreHoldsBeforeQualifyReportsExists", `strings.TrimPrefix(r.reportsDir, r.root+"/") + "/"`, `strings.TrimPrefix(r.reportsDir, r.root+"/")`},
	{"append: the same entry added twice", "../eco-report/git.go", "./eco-report/", "TestAnIgnoreEntryIsWrittenOnceAndNeverFusedOntoTheLastLine", "if line == entry {\n\t\t\t\treturn nil", "if line == entry {\n\t\t\t\tbreak"},
	{"append: the entry fused onto an unterminated last line", "../eco-report/git.go", "./eco-report/", "TestAnIgnoreEntryIsWrittenOnceAndNeverFusedOntoTheLastLine", "if isNonEmptyFile(file) && !endsWithNewline(file) {", "if false {"},
	{"exclusion drop: the entry left in place", "../eco-report/git.go", "./eco-report/", "TestDiscardDestructivePath", `if line != ".idsd/" {`, "if true {"},
	{"promote: a refusal leaves .idsd/ exposed to git add -A", "../eco-report/git.go", "./eco-report/", "TestNoRefusalLeavesIdsdExposedToGitAddAll", "func (r *run) refuseUnpromoted(lines ...string) {\n\tr.restoreLocalExclusion()", "func (r *run) refuseUnpromoted(lines ...string) {\n\tif false {\n\t\tr.restoreLocalExclusion()\n\t}"},
	{"worktree count: every line counted as a worktree", "../eco-report/git.go", "./eco-report/", "TestDiscardDestructivePath", `if strings.HasPrefix(line, "worktree ") {`, `if strings.HasPrefix(line, "") {`},
	{"worktree count: a second worktree goes uncounted", "../eco-report/git.go", "./eco-report/", "TestASecondWorktreeKeepsTheSharedExclusion", `if strings.HasPrefix(line, "worktree ") {`, `if strings.HasPrefix(line, "no worktree line starts with this ") {`},

	// seams.go — the two scripts this tool calls rather than reimplements.
	{"todo scan: a scan that did not run read as nothing open", "../eco-report/seams.go", "./eco-report/", "TestAScanThatDidNotRunIsNeverReadAsNothingOpen", "if status > 1 {", "if false {"},
	{"fingerprint: a missing script recomputed locally", "../eco-report/seams.go", "./eco-report/", "TestAMissingFingerprintScriptRefusesInsteadOfRecomputing", "if !isExecutable(r.fingerprintBin) {", "if false {"},
	{"fingerprint: an empty tree read as a fingerprint", "../eco-report/seams.go", "./eco-report/", "TestListWalksTheTreeOnceAndNeverStreamsAPartialAnswer", `if status != 0 || tree == "" {`, "if false {"},
	{"fingerprint: the walk repeated once per ship", "../eco-report/seams.go", "./eco-report/", "TestListWalksTheTreeOnceAndNeverStreamsAPartialAnswer", `if r.cachedTree != "" {`, "if false {"},

	// gate.go — the merge gate, the items a re-qualify must carry, and the routing token.
	{"gate: a stale tree no longer blocks", "../eco-report/gate.go", "./eco-report/", "TestTheHumanIndexIsNeverTouched", "if current != reviewed {\n\t\tif reviewed == \"\" {", "if false {\n\t\tif reviewed == \"\" {"},
	{"gate: an absent stage record no longer blocks", "../eco-report/gate.go", "./eco-report/", "TestGateBlocksOnEachOfItsReasonsAndClearsOnNone", "case isUnstamped(stages):", "case isUnstamped(stages) && false:"},
	{"gate: a turnaround trim no longer blocks", "../eco-report/gate.go", "./eco-report/", "TestATrimmedPassIsNotAFullOne", `case trims != "":`, "case false:"},
	{"gate: a scan that did not run no longer blocks", "../eco-report/gate.go", "./eco-report/", "TestGateBlocksOnEachOfItsReasonsAndClearsOnNone", "case status > 1:", "case false:"},
	{"gate: an open item no longer blocks the merge", "../eco-report/gate.go", "./eco-report/", "TestGateBlocksOnEachOfItsReasonsAndClearsOnNone", `case todos != "":`, "case false:"},
	{"gate: a clean gate reports nothing at all", "../eco-report/gate.go", "./eco-report/", "TestGateBlocksOnEachOfItsReasonsAndClearsOnNone", "if blocked == 0 {", "if false {"},
	{"carry: the open items go unprinted", "../eco-report/gate.go", "./eco-report/", "TestCarryPrintsTheItemsARequalifyMustNotLose", "if r.openTodos != \"\" {\n\t\tr.line(\"%s\", r.openTodos)", "if false {\n\t\tr.line(\"%s\", r.openTodos)"},
	{"state: a closed ship's archived intent no longer answers done", "../eco-report/gate.go", "./eco-report/", "TestCloseOnACleanReportThePathDoneRuns", `if resolved == 0 && shell.IsRegularFile(r.root+"/.idsd/archive/"+stemOfReportPath(r.report)+".md") {`, "if false {"},
	{"state: a token answered for a report that is not there", "../eco-report/gate.go", "./eco-report/", "TestStateNeverAnswersATokenItCannotStandBehind", "if resolved != 0 || !shell.IsRegularFile(r.report) {", "if false {"},
	{"state: the readability guard at its own call site removed", "../eco-report/gate.go", "./eco-report/", "TestAnUnreadableReportIsNotAState", `r.assertReportIsReadable("its state is unknown (permissions?), and 'resume' is what an unread report looks like")`, "_ = r.report"},
	{"state token: an archived intent no longer answers done", "../eco-report/gate.go", "./eco-report/", "TestStateAnswersEveryTokenItRoutesOn", `if slug := r.intentSlug(); slug != "" && shell.IsRegularFile(r.root+"/.idsd/archive/"+slug+".md") {`, "if false {"},
	{"state token: an unstamped report no longer answers resume", "../eco-report/gate.go", "./eco-report/", "TestTwoIntentsShipSideBySide", "if isUnstamped(reviewed) {", "if false {"},
	{"state token: a moved tree answers ready", "../eco-report/gate.go", "./eco-report/", "TestStateAnswersEveryTokenItRoutesOn", `return "re-qualify" // reviewed once, tree moved since`, `return "ready" // reviewed once, tree moved since`},
	{"state token: open items no longer answer decide", "../eco-report/gate.go", "./eco-report/", "TestStateAnswersEveryTokenItRoutesOn", "if r.openTodos != \"\" {\n\t\treturn \"decide\"", "if false {\n\t\treturn \"decide\""},
	{"state token: a trimmed pass answers ready", "../eco-report/gate.go", "./eco-report/", "TestATrimmedPassIsNotAFullOne", `if isUnstamped(r.reviewedStages()) || r.fastTrims() != "" {`, "if false {"},
	{"list: a partial listing streamed as it goes", "../eco-report/gate.go", "./eco-report/", "TestAnUnreadableReportIsNotAState", `listing += name + "\t" + r.stateToken() + "\n"`, `r.line("%s\t%s", name, r.stateToken())`},
	{"list: the readability guard removed", "../eco-report/gate.go", "./eco-report/", "TestAnUnreadableReportIsNotAState", `r.assertReportIsReadable("nothing was printed, this listing included")`, "_ = r.report"},
	{"list: no reports answered with an empty line", "../eco-report/gate.go", "./eco-report/", "TestAnIntentValueCannotNameAFileOutsideQualifyReports", `r.line("no reports")`, `r.line("")`},

	// scratch.go — excluded, promoted to durable, or torn down. Two of the three are destructive.
	{"check-ignore: the mode read without asserting it could be", "../eco-report/scratch.go", "./eco-report/", "TestPromoteAndCheckIgnoreAlsoRefuseAnUnreadableIndex", "so nothing scratch is ever staged.\n\tr.assertRepoModeReadable()", "so nothing scratch is ever staged.\n\t_ = r.root"},
	{"check-ignore: the committed branch never taken", "../eco-report/scratch.go", "./eco-report/", "TestPromoteReportsTheModeNotTheAdd", "if r.repoMode() == \"committed\" {\n\t\t// A path already tracked", "if false {\n\t\t// A path already tracked"},
	{"check-ignore: an unignored surface reported ok", "../eco-report/scratch.go", "./eco-report/", "TestPromoteReportsTheModeNotTheAdd", `if unignored == "" {`, "if true {"},
	{"check-ignore: a failed exclusion reported ok", "../eco-report/scratch.go", "./eco-report/", "TestCheckIgnoreRefusesWhenTheExclusionCannotBeWritten", "if err := r.addLocalExclusion(); err != nil {", "if err := error(nil); err != nil {"},
	{"promote: nothing to promote is promoted anyway", "../eco-report/scratch.go", "./eco-report/", "TestNoRefusalLeavesIdsdExposedToGitAddAll", "if len(r.reportNames()) == 0 {", "if false {"},
	{"promote: the mode read without asserting it could be", "../eco-report/scratch.go", "./eco-report/", "TestPromoteAndCheckIgnoreAlsoRefuseAnUnreadableIndex", "nothing to promote\")\n\t}\n\tr.assertRepoModeReadable()", "nothing to promote\")\n\t}\n\t_ = r.root"},
	{"promote: an already-committed repo promoted again", "../eco-report/scratch.go", "./eco-report/", "TestPromoteIsIdempotentOverACommittedRepo", "if r.repoMode() == \"committed\" {\n\t\tr.line(\"already committed", "if false {\n\t\tr.line(\"already committed"},
	{"promote: a symlinked .gitignore written through", "../eco-report/scratch.go", "./eco-report/", "TestPromoteWritesNoGitignoreThroughALink", "if shell.IsSymlink(gitignore) {", "if false {"},
	{"promote: an unwritten entry promoted anyway", "../eco-report/scratch.go", "./eco-report/", "TestPromoteWritesNoGitignoreThroughALink", `if unwritten != "" {`, "if false {"},
	{"promote: the entry written but never confirmed with git", "../eco-report/scratch.go", "./eco-report/", "TestPromoteWritesNoGitignoreThroughALink", `if r.ignoreSourceOf(r.root+"/"+entry) != ".gitignore" {`, "if false {"},
	{"promote: a failed add read as a promotion", "../eco-report/scratch.go", "./eco-report/", "TestPromoteReportsTheModeNotTheAdd", `if r.passThrough("git", "-C", r.root, "add", ".idsd", ".gitignore") != 0 {`, "if false {"},
	{"promote: success read from the add rather than the mode", "../eco-report/scratch.go", "./eco-report/", "TestPromoteReportsTheModeNotTheAdd", `if r.repoMode() != "committed" {`, "if false {"},
	{"discard: no report and no name discarded anyway", "../eco-report/scratch.go", "./eco-report/", "TestDiscardDestructivePath", "case 1:", "case 9:"},
	{"discard: the mode read without asserting it could be", "../eco-report/scratch.go", "./eco-report/", "TestTheDestructivePathCarriesTheGuardsTheWritePathHas", "r.assertRepoModeReadable()\n\tif r.repoMode() == \"committed\" {\n\t\tr.refuse(\"committed idsd repo", "_ = r.root\n\tif r.repoMode() == \"committed\" {\n\t\tr.refuse(\"committed idsd repo"},
	{"discard: committed mode discarded", "../eco-report/scratch.go", "./eco-report/", "TestDiscardDestructivePath", "if r.repoMode() == \"committed\" {\n\t\tr.refuse(\"committed idsd repo", "if false {\n\t\tr.refuse(\"committed idsd repo"},
	{"discard: the write-path link guard removed", "../eco-report/scratch.go", "./eco-report/", "TestTheDestructivePathCarriesTheGuardsTheWritePathHas", `r.assertWritePathsAreReal("nothing was discarded")`, "_ = r.root"},
	{"discard: the ship-exists guard call removed", "../eco-report/scratch.go", "./eco-report/", "TestDiscardDeletesNothingForAShipThatIsNotHere", "r.assertShipExists(stem)", "_ = stem"},
	{"discard: the readability guard removed", "../eco-report/scratch.go", "./eco-report/", "TestDiscardRemovesNothingItCouldNotRead", `r.assertReportIsReadable("nothing was discarded, because its intent cannot be cross-checked (permissions?)")`, "_ = r.report"},
	{"discard: the two names no longer reconciled", "../eco-report/scratch.go", "./eco-report/", "TestDiscardReconcilesTheTwoNamesBeforeDeletingAnything", `if slug != "" && slug != stem {`, "if false {"},
	{"discard: the intent file left behind", "../eco-report/scratch.go", "./eco-report/", "TestDiscardDestructivePath", `_ = rmFile(r.root + "/.idsd/intents/" + slug + ".md")`, "_ = slug"},
	{"discard: the archived intent file left behind", "../eco-report/scratch.go", "./eco-report/", "TestDiscardDestructivePath", `_ = rmFile(r.root + "/.idsd/archive/" + slug + ".md")`, "_ = slug"},
	{"discard: the stage markers survive the teardown", "../eco-report/scratch.go", "./eco-report/", "TestDiscardDestructivePath", "_ = os.RemoveAll(r.stageReturnsDir)\n\trmdirIfEmpty(r.reportsDir, r.root", "_ = r.stageReturnsDir\n\trmdirIfEmpty(r.reportsDir, r.root"},
	{"discard: what survives no longer keeps .idsd/", "../eco-report/scratch.go", "./eco-report/", "TestDiscardDestructivePath", `if kept := r.survivingContent(); kept != "" {`, "if kept := r.survivingContent(); len(kept) < 0 {"},
	{"discard: the shared exclusion dropped from a second worktree", "../eco-report/scratch.go", "./eco-report/", "TestASecondWorktreeKeepsTheSharedExclusion", "if r.worktreeCount() > 1 {", "if false {"},
	{"discard: the exclusion reported from the attempt", "../eco-report/scratch.go", "./eco-report/", "TestTheTeardownReportsTheExclusionFromTheResultNotTheAttempt", "if err := r.dropLocalExclusion(); err != nil {\n\t\tr.errLines(\"discarded:", "if err := error(nil); err != nil {\n\t\tr.errLines(\"discarded:"},
	{"close: an open item no longer refuses", "../eco-report/scratch.go", "./eco-report/", "TestCloseRetiresOneShipScratchAndNothingElse", "if !isForced {", "if !isForced && false {"},
	// SURVIVOR. close is asserted on the report and the scratch dir, never on the stage-returns dir.
	// The case to write: close leaves no stage marker behind for the next ship to inherit.
	{"close: the stage markers outlive the report", "../eco-report/scratch.go", "./eco-report/", "TestCloseRetiresOneShipScratchAndNothingElse", "_ = os.RemoveAll(r.stageReturnsDir)\n\trmdirIfEmpty(r.reportsDir)", "_ = r.stageReturnsDir\n\trmdirIfEmpty(r.reportsDir)"},

	// init.go — the only subcommand that creates a report, and the one every symlink guard is for.
	{"init: the intent untrimmed before the emptiness guard", "../eco-report/init.go", "./eco-report/", "TestTheFilenameAndTheFrontmatterNameTheSameShip", "intent = trimLeadingSpace(intent)", "intent = intent"},
	{"init: a newline in the intent reaches the frontmatter", "../eco-report/init.go", "./eco-report/", "TestTheFrontmatterCannotBeForgedThroughTheIntentValue", `strings.NewReplacer("\n", " ", "\r", " ")`, `strings.NewReplacer("\n", "\n", "\r", "\r")`},
	{"init: an intent that names no report scaffolds one", "../eco-report/init.go", "./eco-report/", "TestAnIntentValueCannotNameAFileOutsideQualifyReports", `if reportName == "" {`, "if false {"},
	{"init: the template check dropped", "../eco-report/init.go", "./eco-report/", "TestADriftedTemplateIsRefusedBeforeAnyReportIsScaffolded", "r.assertTemplateStampable()", "_ = r.template"},
	{"init: the write-path link guard dropped", "../eco-report/init.go", "./eco-report/", "TestInitRefusesRatherThanWritingThroughALink", `r.assertWritePathsAreReal("the report was NOT initialized")`, "_ = r.root"},
	{"init: the ignore precondition dropped", "../eco-report/init.go", "./eco-report/", "TestInitWillNotWriteAReportIntoItsOwnFingerprint", "r.assertReportsDirIsIgnored()", "_ = r.root"},
	{"init: the legacy note not emitted where it is worth reading", "../eco-report/init.go", "./eco-report/", "TestInitIsWhereALegacyReportGetsMentioned", "r.legacyNote()\n\n\tpresent :=", "\n\tpresent :="},
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
	{"invalidate: last pass's stage returns survive it", "../eco-report/stamp.go", "./eco-report/", "TestInvalidateClearsThePassItStarts", "_ = os.RemoveAll(r.stageReturnsDir)\n\tr.line(\"invalidated", "_ = os.RemoveAll(r.stageReturnsDir + \"/no-such-stage\")\n\tr.line(\"invalidated"},
	{"stage vocabulary: any word accepted as a stage", "../eco-report/stages.go", "./eco-report/", "TestAStageNameThatIsNotAStageIsRefused", "if stage == known {", "if stage != known {"},
	{"stage vocabulary: a stage renamed out of the pipeline", "../eco-report/stages.go", "./eco-report/", "TestTwoIntentsShipSideBySide", `const stageNames = "code-review security-review tighten refactor retro"`, `const stageNames = "code-review security-review tighten refactor retros"`},
	// SURVIVOR. reportChecksum answers empty only for a report it cannot read, and no case marks a
	// stage returned while it cannot. The case to write: stage-returned over an unreadable report
	// writes no marker, so the stamp cannot later wave that stage through.
	{"marker: an unchecksummable report marked anyway", "../eco-report/stages.go", "./eco-report/", "TestAMarkerIsThePosixCksumOfTheReport", `if value == "" {`, "if false {"},
	{"marker: the trailing newline left on the value", "../eco-report/stages.go", "./eco-report/", "TestAStampCannotOutliveThePassThatEarnedIt", `strings.TrimRight(string(content), "\n")`, "string(content)"},
	// SURVIVOR. No case stamps a pass whose stage was recorded through `no-items`. The case to write:
	// a stage marked no-items clears the gate instead of blocking it.
	{"stage block: a no-items marker blocks the stamp", "../eco-report/stages.go", "./eco-report/", "TestTwoIntentsShipSideBySide", "if recorded == noItemsMarker {", "if false {"},
	{"stage block: unrecorded items no longer block", "../eco-report/stages.go", "./eco-report/", "TestAStampCannotOutliveThePassThatEarnedIt", "if recorded == r.reportChecksum() {", "if false {"},
	{"stage block: an unmarked stage no longer blocks", "../eco-report/stages.go", "./eco-report/", "TestAStampCannotOutliveThePassThatEarnedIt", "if !r.stageWasMarkedReturned(stage) {\n\t\treturn \"ran but was never marked returned", "if false {\n\t\treturn \"ran but was never marked returned"},
	{"stamp grammar: any entry at all accepted", "../eco-report/stages.go", "./eco-report/", "TestTheStampGrammarIsTheAuthorityOnWhatAPassMayClaim", "stage, ok := stageOfEntry(entry)\n\t\tif !ok {", "stage, ok := stageOfEntry(entry)\n\t\tif false && !ok {"},
	{"stamp grammar: a missing stage accepted", "../eco-report/stages.go", "./eco-report/", "TestTheStampGrammarIsTheAuthorityOnWhatAPassMayClaim", "case seen[stage] == 0:", "case false:"},
	{"stamp grammar: a duplicate stage accepted", "../eco-report/stages.go", "./eco-report/", "TestTheStampGrammarIsTheAuthorityOnWhatAPassMayClaim", "case seen[stage] > 1:", "case false:"},
	{"stamp grammar: refactor:partial(fast) no longer legal", "../eco-report/stages.go", "./eco-report/", "TestTheStampGrammarIsTheAuthorityOnWhatAPassMayClaim", `case "refactor", "refactor:partial(fast)", "refactor:partial(cap)":`, `case "refactor", "refactor:partial(cap)":`},
	{"stamp grammar: skipped(fast) no longer legal", "../eco-report/stages.go", "./eco-report/", "TestATrimmedPassIsNotAFullOne", `if entry == stage || entry == stage+":skipped(fast)" || entry == stage+":skipped(not-applicable)" {`, `if entry == stage || entry == stage+":skipped(not-applicable)" {`},
	{"stamp grammar: skipped(not-applicable) no longer legal", "../eco-report/stages.go", "./eco-report/", "TestTheStampGrammarIsTheAuthorityOnWhatAPassMayClaim", `if entry == stage || entry == stage+":skipped(fast)" || entry == stage+":skipped(not-applicable)" {`, `if entry == stage || entry == stage+":skipped(fast)" {`},
	{"stamp grammar: retro left out of the skippable set", "../eco-report/stages.go", "./eco-report/", "TestTheStampGrammarIsTheAuthorityOnWhatAPassMayClaim", `for _, stage := range []string{"security-review", "tighten", "retro"} {`, `for _, stage := range []string{"security-review", "tighten"} {`},

	// shell.go — the primitives whose exact edges a refusal turns on, and the digest a marker holds.
	{"slug charset: the dash left out of the set", "../eco-report/shell.go", "./eco-report/", "TestTwoIntentsShipSideBySide", `b == '.' || b == '_' || b == '-'`, `b == '.' || b == '_'`},
	{"slug charset: a path separator let into the set", "../eco-report/shell.go", "./eco-report/", "TestAnIntentValueCannotNameAFileOutsideQualifyReports", `b == '.' || b == '_' || b == '-'`, `b == '.' || b == '_' || b == '-' || b == '/'`},
	{"readable: -r asked as mere existence", "../eco-report/shell.go", "./eco-report/", "TestAnUnreadableReportIsNotAState", "syscall.Access(path, 0x4)", "syscall.Access(path, 0x0)"},
	{"executable: -x asked as mere existence", "../eco-report/shell.go", "./eco-report/", "TestAMissingFingerprintScriptRefusesInsteadOfRecomputing", "syscall.Access(path, 0x1)", "syscall.Access(path, 0x0)"},
	{"records: the trailing newline dropped from every rewrite", "../eco-report/shell.go", "./eco-report/", "TestAStampCannotOutliveThePassThatEarnedIt", `out.WriteString("\n")`, `out.WriteString("")`},
	// SURVIVOR. No case puts a directory where rmFile expects a file. The case to write: init over a
	// report path that is a directory reports something in the way rather than removing it.
	{"rm -f: a directory removed where the shell's refused", "../eco-report/shell.go", "./eco-report/", "TestInitStagedWriteIsNotAWayOutOfTheRepo", "if info.IsDir() {", "if info.IsDir() && false {"},
	// The marker's digest is read by whichever version of this tool runs next, so it has to be
	// cksum(1)'s and not merely this tool's own: any self-consistent digest passes every comparison
	// the tool makes against itself.
	{"cksum: a digest that is not POSIX cksum's", "../eco-report/shell.go", "./eco-report/", "TestAMarkerIsThePosixCksumOfTheReport", "crc = crc<<1 ^ 0x04C11DB7", "crc = crc<<1 ^ 0x04C11DB6"},
	{"cksum: the length no longer folded in", "../eco-report/shell.go", "./eco-report/", "TestAMarkerIsThePosixCksumOfTheReport", "for length := len(content); length != 0; length >>= 8 {", "for length := len(content); false; length >>= 8 {"},

	// eco-report.go — the dispatch, and the one place a refusal becomes an exit code.
	{"exec: a refusal reported as success", "../eco-report/eco-report.go", "./eco-report/", "TestAnExistingReportIsNotSilentlyReplaced", "code = signal.code", "code = 0"},
	{"dispatch: a subcommand no longer routed", "../eco-report/eco-report.go", "./eco-report/", "TestCarryPrintsTheItemsARequalifyMustNotLose", `case "carry":`, `case "carry-x":`},
}

// Every suite a mutant names, in first-named order — what the baseline has to be green over.
func suitesNamed() []string {
	var suites []string
	seen := map[string]bool{}
	for _, m := range mutants {
		if !seen[m.suite] {
			seen[m.suite] = true
			suites = append(suites, m.suite)
		}
	}
	return suites
}

// Where a mutant's anchor is, and how many times it matches there. Exactly once, or the mutant is
// ambiguous: a string matching twice edits a guard it was not aimed at, and one matching zero times
// edits nothing. Preflight and the run itself both ask, and they ask here so they cannot come to
// different answers about one mutant.
func (m mutant) anchor(pkgDir string) (path, source string, count int, err error) {
	path = filepath.Join(pkgDir, m.file)
	body, err := os.ReadFile(path)
	if err != nil {
		return path, "", 0, err
	}
	return path, string(body), strings.Count(string(body), m.from), nil
}

// The top-level test names one suite holds, from `go test -list`. Subtests are not listed and are not
// wanted: a mutant names the test function, never the case's prose.
func testsIn(pkgDir, suite string) (map[string]bool, error) {
	cmd := exec.Command("go", "test", "-list", ".*", suite)
	cmd.Dir = filepath.Dir(pkgDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	names := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		if name := strings.TrimSpace(line); strings.HasPrefix(name, "Test") {
			names[name] = true
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("it listed no tests at all")
	}
	return names, nil
}

// The overlay is how one file's content changes without the tree changing: `go build` and `go test`
// both read it, so nothing under the repo is written and a killed run leaves no debris behind.
func writeOverlay(dir, realPath, mutatedPath string) (string, error) {
	doc := struct {
		Replace map[string]string `json:"Replace"`
	}{Replace: map[string]string{realPath: mutatedPath}}
	body, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "overlay.json")
	return path, os.WriteFile(path, body, 0o644)
}

// One mutant: rewrite the file into a temp copy, point an overlay at it, and run the suite through it.
// Compilation failure is `broken`, not a kill: a mutant that cannot build says nothing about a guard.
func run(pkgDir string, m mutant, runFilter string) (verdict string, elapsed time.Duration) {
	at := time.Now()
	work, err := os.MkdirTemp("", "gomutate")
	if err != nil {
		return "invalid", time.Since(at)
	}
	defer os.RemoveAll(work)

	realPath, source, matches, err := m.anchor(pkgDir)
	if err != nil {
		return "invalid", time.Since(at)
	}
	if matches != 1 {
		return fmt.Sprintf("anchor x%d", matches), time.Since(at)
	}
	// Base, not the mutant's own relative path: a file named `../shell/x.go` would otherwise write
	// outside the temp dir. Each mutant has its own work dir, so two basenames cannot collide.
	mutated := filepath.Join(work, filepath.Base(m.file))
	if err := os.WriteFile(mutated, []byte(strings.Replace(source, m.from, m.to, 1)), 0o644); err != nil {
		return "invalid", time.Since(at)
	}
	overlay, err := writeOverlay(work, realPath, mutated)
	if err != nil {
		return "invalid", time.Since(at)
	}

	// The mutant's own test unless the caller overrode it: `-run` on the command line is for driving
	// one mutant by hand, and it has to be able to widen the filter as well as narrow it.
	filter := m.by
	if runFilter != "" {
		filter = runFilter
	}
	args := []string{"test", "-overlay=" + overlay, "-count=1", "-failfast"}
	if filter != "" {
		args = append(args, "-run", filter)
	}
	args = append(args, m.suite)
	cmd := exec.Command("go", args...)
	cmd.Dir = filepath.Dir(pkgDir)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return "KILLED NOTHING", time.Since(at)
	}
	if strings.Contains(string(out), "[build failed]") || strings.Contains(string(out), "cannot use") {
		return "broken", time.Since(at)
	}
	return "killed", time.Since(at)
}

func main() {
	jobs := flag.Int("jobs", 0, "mutants in flight at once (default: cores - 2)")
	preflightOnly := flag.Bool("preflight", false, "check every anchor matches exactly once, then stop")
	runFilter := flag.String("run", "", "pass through to `go test -run`")
	flag.Parse()

	if *jobs <= 0 {
		if *jobs = runtime.NumCPU() - 2; *jobs < 1 {
			*jobs = 1
		}
	}
	here, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gomutate: cannot locate myself")
		os.Exit(2)
	}
	pkgDir := filepath.Join(filepath.Dir(filepath.Dir(here)), "eco-check")
	if _, err := os.Stat(pkgDir); err != nil {
		fmt.Fprintf(os.Stderr, "gomutate: %s is not there — exit 2, nothing ran.\n", pkgDir)
		os.Exit(2)
	}

	started := time.Now()
	stale := 0
	// Asked once per suite rather than once per mutant: a `by` naming a test its suite no longer holds
	// would run as a filter matching nothing, which `go test` exits 0 on: a KILLED NOTHING verdict
	// loudly about the wrong thing. A stale test name is the same defect as a stale anchor, so it is
	// refused in the same place.
	held := map[string]map[string]bool{}
	for _, suite := range suitesNamed() {
		names, err := testsIn(pkgDir, suite)
		if err != nil {
			fmt.Printf("  cannot list     %s: %v\n", suite, err)
			stale++
			continue
		}
		held[suite] = names
	}
	for _, m := range mutants {
		_, _, matches, err := m.anchor(pkgDir)
		if err != nil {
			fmt.Printf("  missing file    %s (%s)\n", m.label, m.file)
			stale++
			continue
		}
		if matches != 1 {
			fmt.Printf("  anchor x%-6d %s\n", matches, m.label)
			stale++
		}
		if m.by != "" && held[m.suite] != nil && !held[m.suite][m.by] {
			fmt.Printf("  no such test    %s names %s, which %s does not hold\n", m.label, m.by, m.suite)
			stale++
		}
	}
	if stale > 0 {
		fmt.Printf("preflight: %d of %d anchors do not match exactly once — nothing was run\n", stale, len(mutants))
		os.Exit(1)
	}
	fmt.Printf("preflight: %d anchors, all matching exactly once (%.2fs)\n", len(mutants), time.Since(started).Seconds())
	if *preflightOnly {
		return
	}

	// The baseline first, over every suite a mutant names: each verdict below means "this edit turned
	// a green suite red", which says nothing if it was already red.
	base := exec.Command("go", append([]string{"test", "-count=1"}, suitesNamed()...)...)
	base.Dir = filepath.Dir(pkgDir)
	if out, err := base.CombinedOutput(); err != nil {
		fmt.Println("  BASELINE RED    a suite does not pass unmutated")
		fmt.Println(string(out))
		os.Exit(2)
	}

	fmt.Printf("%s — one guard removed at a time, %d at once\n", strings.Join(suitesNamed(), " "), *jobs)
	verdicts := make([]string, len(mutants))
	times := make([]time.Duration, len(mutants))
	sem := make(chan struct{}, *jobs)
	var wg sync.WaitGroup
	for i, m := range mutants {
		wg.Add(1)
		go func(i int, m mutant) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			verdicts[i], times[i] = run(pkgDir, m, *runFilter)
		}(i, m)
	}
	wg.Wait()

	bad := 0
	for i, m := range mutants {
		if verdicts[i] == "killed" {
			fmt.Printf("  killed          %s  (%.1fs)\n", m.label, times[i].Seconds())
		} else {
			fmt.Printf("  %-15s %s  (%.1fs)\n", verdicts[i], m.label, times[i].Seconds())
			bad++
		}
	}
	fmt.Printf("%d mutation(s), %d that proved nothing, %.1fs wall clock\n", len(mutants), bad, time.Since(started).Seconds())
	if bad > 0 {
		os.Exit(1)
	}
}

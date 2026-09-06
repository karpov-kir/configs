package ecoreport

import (
	"os"

	"kk-flavor/tools/shell"
)

// `init` — the only subcommand that creates a report, and the one every symlink guard exists for.

func (r *run) cmdInit(args []string) {
	intent, isForced := nameAndForceFlag(args)
	// Trimmed once, before the emptiness guard: a whitespace-only value would otherwise pass the guard
	// and scaffold a report whose blank `intent:` every reader treats as a standalone review. Every
	// later reader uses the result, so the stem reportNameFor derives and the value the frontmatter
	// records cannot come apart and name different ships. The collapse below maps whitespace to
	// whitespace, so nothing needs re-trimming.
	intent = trimLeadingSpace(intent)
	if intent == "" {
		r.refuse("usage: report.sh init \"<intent frontmatter value>\" [--force]")
	}
	// Frontmatter is single-line: collapse CR/LF so a value seeded from a fetched ticket can't inject
	// extra frontmatter lines (a forged reviewed-tree). Every control byte, not those two alone. The
	// same value reaches this tool's own output and the `intent:` line every later reader echoes back,
	// and the slug charset does not stand between them — a `review: <description>` intent takes the
	// `review` stem off its first field and carries the rest of the line through unchecked. An ESC
	// there erases the lines printed above it.
	intent = shell.Oneline(intent)
	reportName := reportNameFor(intent)
	if reportName == "" {
		r.refuse("error: '" + intent + "' cannot name a report file — the intent must be a slug ([0-9A-Za-z._-]) or a \"review: <description>\". The report was NOT initialized.")
	}
	r.setReportPaths(reportName)
	r.assertTemplateStampable()
	r.assertWritePathsAreReal("the report was NOT initialized")
	r.assertReportIsIgnored()
	present := shell.PathExists(r.report) || shell.IsSymlink(r.report)
	if present && !isForced {
		existing := firstLineWithPrefix(r.report, "intent:")
		if existing == "" {
			existing = "(no intent: line)"
		}
		r.refuse("error: "+r.report+" already exists — the report was NOT initialized.",
			"  it records "+existing,
			"  a new report starts empty; this one holds "+r.openItemsPhrase()+".",
			"  Keep it and re-qualify (report.sh carry lists those items), or re-run with --force to start over it.")
	}
	replacing, carried := "", ""
	if present {
		replacing = r.openItemsPhrase()
		carried, _ = r.runTodoGate()
	}

	// 0700, and this is the one place in this tool that creates the scratch tree — MkdirAll builds every
	// missing parent with the same mode, a missing override root included. The tree sits outside the
	// repository now, holds the only copy of the intents, and every report in it carries a pass's
	// security findings, so the directory is what decides who reads them: the report itself lands at
	// 0600 (rewriteReport renames its own temp file over it, and os.CreateTemp is 0600), but nothing
	// else there does — an intent file is written by whichever skill authors it, at that process's
	// umask. 0777 left the scratch world-readable under any ordinary umask, and group- or world-WRITABLE
	// under a lax one, which puts a planted link inside the directory `discard` later removes.
	//
	// It protects the ROOT only when init gets there first. A skill that authors an intent before any
	// report exists creates the tree itself, and this MkdirAll then makes only the missing
	// qualify-reports/ below it.
	if err := os.MkdirAll(shell.DirName(r.report), 0o700); err != nil {
		r.refuse("error: could not create " + shell.DirName(r.report) + " — the report was NOT initialized")
	}
	// Staged beside the report and renamed over it, never copied onto it: a write that dies partway
	// leaves a truncated report, and --force has already discarded the only other copy of those items.
	// The temp name is deliberately not `*-qualify-report.md`, so a leftover joins no listing.
	staged := r.report + ".new"
	// Removed before the copy, never guarded by a symlink refusal: this path is ours and transient, so
	// a link planted there is hostile (a committed one reaches us through someone else's branch) and a
	// regular file there is a crashed `init`'s leftover. Refusing would wedge `init` on the second case;
	// removing the link unlinks the link itself, so the copy below lands on a fresh regular file either way.
	if err := rmFile(staged); err != nil {
		r.refuse("error: could not clear " + staged + " — the report was NOT initialized")
	}
	if err := copyFile(r.template, staged); err != nil {
		_ = rmFile(staged)
		r.refuseUnwritten(replacing)
	}
	if err := moveFile(staged, r.report); err != nil {
		_ = rmFile(staged)
		r.refuseUnwritten(replacing)
	}
	// Printed at the moment of the act, because it is the only record of what --force just discarded.
	if replacing != "" {
		r.errLines("warning: --force discarded the previous report, which held " + replacing)
		if carried != "" {
			r.errLines(indentLines(carried, "    "))
		}
		r.errLines("  nothing above is kept anywhere — route it now if it still matters.")
	}
	// A stem's stage markers are cleared by invalidate, close and discard, but a stem can also be
	// reached by a fresh init over a report that ended in none of those — a crash, a deleted file, a
	// --force. Every marker is a claim about a pass, and `stamp` reads them as preconditions it may
	// stop asking about: an inherited `decisions-reviewed` earns a stamp for a pass that never opened
	// the decision log. So the stem starts empty here, the same as it ends.
	if err := os.RemoveAll(r.stageReturnsDir); err != nil {
		r.refuse("error: " + r.report + " was written, but " + r.stageReturnsDir + " could not be cleared (" + err.Error() + ") — do not run the pass against it: a stage marker left from an earlier one is a precondition this pass would never have to earn.")
	}
	err := r.rewriteReport(
		r.report+" still carries the template's placeholder intent",
		"could not write the intent line into "+r.report,
		rewriteIntent(intent))
	if err != nil {
		r.exit(2)
	}
	r.line("initialized %s (repo mode: %s, intent: %s)", r.report, r.repoMode(), intent)
}

func (r *run) refuseUnwritten(replacing string) {
	message := "error: could not write " + r.report + " from " + r.template + " — the report was NOT initialized"
	if replacing != "" {
		message += "; the previous one is untouched and still holds " + replacing
	}
	r.refuse(message)
}

// Order-free flag parsing, which both `init` and `close` need: `--force` read positionally resolves
// as an intent name (its whole charset is legal in a slug) and closes a report that does not exist.
func nameAndForceFlag(args []string) (name string, isForced bool) {
	for _, arg := range args {
		if arg == "--force" {
			isForced = true
			continue
		}
		if name == "" {
			name = arg
		}
	}
	return name, isForced
}

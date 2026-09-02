package ecoreport

import (
	"os"
	"strings"

	"kk-flavor/tools/shell"
)

// The report's frontmatter: the three lines every later reader greps, and the two rewrites that
// write them. Both rewrites are atomic — on any failure the report is left exactly as it was.

// `grep -m1 '^<prefix>'` — the whole line, not its value: `init` quotes the existing `intent:` line
// back when it refuses over a report, and what the human recognises is the line as they wrote it.
// An unreadable file answers empty, which is in the unstamped set. That is deliberate at every call
// site: an unreadable report is refused before it is read, so nothing here has to distinguish the
// two.
//
// Go reads the file whole where grep read it as text. A committed NUL byte made BSD grep answer
// "Binary file … matches" instead of the line, and that answer reached the frontmatter readers as a
// value; here the line comes back as it was written.
func firstLineWithPrefix(path, prefix string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range shell.SplitLines(string(content)) {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}

// `sed 's/^<field>:[[:space:]]*//'` over that line.
func fieldValue(path, field string) string {
	prefix := field + ":"
	return trimLeadingSpace(strings.TrimPrefix(firstLineWithPrefix(path, prefix), prefix))
}

func hasField(path, field string) bool {
	return firstLineWithPrefix(path, field+":") != ""
}

func (r *run) reviewedTree() string { return fieldValue(r.report, "reviewed-tree") }

// Empty when never stamped with stages — callers treat that as not-full.
func (r *run) reviewedStages() string { return fieldValue(r.report, "reviewed-stages") }

// Every frontmatter value meaning "no stamp stands here". One set, used whole by every reader — a
// reader knowing only some accepts what the others reject. `init` holds the template to this set.
func isUnstamped(value string) bool {
	switch value {
	case "", "pending", "<hash>", "<stages>", "<worktree>":
		return true
	}
	return false
}

// Entries the last stamp marked `(turnaround)` — stages trimmed to answer sooner. Any of them means the pass
// was not an untrimmed one.
func (r *run) turnaroundTrims() string {
	var trimmed []string
	for _, entry := range strings.Split(r.reviewedStages(), ",") {
		if strings.Contains(entry, "(turnaround)") {
			trimmed = append(trimmed, entry)
		}
	}
	return strings.Join(trimmed, " ")
}

// Guarded to the slug charset: nothing for a standalone `review:`, or for any char outside the set
// (notably `/`), so a slug can never `../`-escape a path it indexes.
func (r *run) intentSlug() string {
	intent := fieldValue(r.report, "intent")
	intent = strings.TrimPrefix(intent, `"`)
	intent = strings.TrimSuffix(intent, `"`)
	slug := firstField(intent)
	if slug == "" || strings.HasPrefix(slug, "review:") || !isSlugCharset(slug) {
		return ""
	}
	return slug
}

// The template carries the frontmatter every later reader greps, and gate and state never read the
// template itself — so a missing field or a drifted placeholder can only be caught here, before a
// report is scaffolded from it. A drifted placeholder stamps every new report as already reviewed.
func (r *run) assertTemplateStampable() {
	if shell.IsSymlink(r.template) {
		r.refuse("error: template " + r.template + " is a symlink — refusing to read the template through one; the report was NOT initialized")
	}
	if !shell.IsRegularFile(r.template) {
		r.refuse("error: template not found (" + r.template + ")")
	}
	if !hasField(r.template, "intent") {
		r.refuse("error: template " + r.template + " has no 'intent:' line to stamp — the report was NOT initialized")
	}
	for _, field := range []string{"reviewed-tree", "reviewed-worktree", "reviewed-stages"} {
		if !hasField(r.template, field) {
			r.refuse("error: template " + r.template + " has no '" + field + ":' line — gate and state read it; the report was NOT initialized")
		}
		placeholder := fieldValue(r.template, field)
		if !isUnstamped(placeholder) {
			r.refuse("error: template " + r.template + " writes '" + field + ": " + placeholder + "', which gate and state read as a completed review — every new report would gate clean. Restore a placeholder is_unstamped() knows, or add this one to it. The report was NOT initialized.")
		}
	}
}

// Atomic: on any failure the report is left exactly as it was. The two failure messages are the
// caller's, because what a half-done rewrite leaves standing differs at each one.
//
// Staged BESIDE the report, never in $TMPDIR, which is what makes the sentence above true. moveFile's
// cross-device fallback is not atomic — it follows a symlink at the destination where the rename would
// have replaced it, and it truncates before writing — and an override root legitimately sits on another
// volume, so a temp file in $TMPDIR would make that fallback the ordinary path rather than the exotic
// one. Same directory means the rename can never cross a device. The name is dot-led, so a leftover is
// invisible to reportNames and joins no listing.
func (r *run) rewriteReport(noTemp, noWrite string, rewrite func([]string) []string) error {
	temp, err := os.CreateTemp(shell.DirName(r.report), ".rewrite.")
	if err != nil {
		r.errLines("error: mktemp failed — " + noTemp)
		return err
	}
	// One failure path for read, write, close and rename together: the report is untouched until the
	// rename, so they all leave the same thing standing and owe the caller the same sentence. First
	// error wins, and the temp file goes whichever one it was.
	content, err := os.ReadFile(r.report)
	if err == nil {
		_, err = temp.Write(joinRecords(rewrite(shell.SplitLines(string(content)))))
	}
	if closed := temp.Close(); err == nil {
		err = closed
	}
	if err == nil {
		err = moveFile(temp.Name(), r.report)
	}
	if err != nil {
		_ = os.Remove(temp.Name())
		r.errLines("error: " + noWrite)
	}
	return err
}

// Frontmatter only, never a body line that happens to quote a field. Line 1 opens the frontmatter and
// the next `---` closes it.
func mapFrontmatter(lines []string, rewrite func(inFrontmatter bool, line string) []string) []string {
	out := make([]string, 0, len(lines)+1)
	inFrontmatter := false
	for i, line := range lines {
		if i > 0 && shell.IsFrontmatterDelimiter(line) {
			inFrontmatter = false
		}
		out = append(out, rewrite(inFrontmatter, line)...)
		if i == 0 {
			inFrontmatter = true
		}
	}
	return out
}

// init's rewrite: the first `intent:` line, wherever it sits.
func rewriteIntent(intent string) func([]string) []string {
	return func(lines []string) []string {
		out := make([]string, 0, len(lines))
		replaced := false
		for _, line := range lines {
			if !replaced && strings.HasPrefix(line, "intent:") {
				out = append(out, "intent: "+intent)
				replaced = true
				continue
			}
			out = append(out, line)
		}
		return out
	}
}

// stamp's rewrite: the stamp replaces reviewed-tree and re-emits the other two beside it, so a report
// carrying them in any order ends up with one of each, and a `reviewed-mode:` line from an older
// layout is dropped rather than left to be read.
func rewriteStamp(tree, worktree, entries string) func([]string) []string {
	return func(lines []string) []string {
		return mapFrontmatter(lines, func(inFrontmatter bool, line string) []string {
			if !inFrontmatter {
				return []string{line}
			}
			switch {
			case strings.HasPrefix(line, "reviewed-mode:"), strings.HasPrefix(line, "reviewed-stages:"),
				strings.HasPrefix(line, "reviewed-worktree:"):
				return nil
			case strings.HasPrefix(line, "reviewed-tree:"):
				return []string{"reviewed-tree: " + tree, "reviewed-worktree: " + worktree, "reviewed-stages: " + entries}
			}
			return []string{line}
		})
	}
}

func rewriteInvalidated(lines []string) []string {
	return mapFrontmatter(lines, func(inFrontmatter bool, line string) []string {
		if !inFrontmatter {
			return []string{line}
		}
		switch {
		case strings.HasPrefix(line, "reviewed-tree:"):
			return []string{"reviewed-tree: pending"}
		case strings.HasPrefix(line, "reviewed-worktree:"):
			return []string{"reviewed-worktree: pending"}
		case strings.HasPrefix(line, "reviewed-stages:"):
			return []string{"reviewed-stages: pending"}
		}
		return []string{line}
	})
}

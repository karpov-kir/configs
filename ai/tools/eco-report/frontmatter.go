package ecoreport

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"configs/ai/tools/shell"
)

// A report's or an intent file's frontmatter: the YAML block between the `---` on the file's first line and the next
// `---`. Every reader and every rewrite goes through one parse of that block, so a line in the body that
// quotes a field's name is never read as the field, and a rewrite touches only the line the parser put the
// key on.

// frontmatter holds a file's lines and the top-level fields its frontmatter block names.
type frontmatter struct {
	lines []string
	// values holds each field's YAML value, and keyLine the index into lines of the line its key is on.
	values  map[string]*yaml.Node
	keyLine map[string]int
}

// parseFrontmatter returns the top-level fields of the YAML block that opens lines. Lines that do not open
// with `---` have no frontmatter and so no fields. A block that never closes, that is not a YAML mapping, or
// that names a key twice is an error, since a reader could not say which value is the field.
func parseFrontmatter(lines []string) (frontmatter, error) {
	parsed := frontmatter{lines: lines, values: map[string]*yaml.Node{}, keyLine: map[string]int{}}
	// A byte-order mark hides the opening `---`, and a file with a frontmatter that no reader sees reads as
	// one with no fields at all.
	if len(lines) > 0 && strings.HasPrefix(lines[0], "\ufeff") {
		return parsed, errors.New("the file opens with a byte-order mark, which hides its frontmatter")
	}
	if len(lines) == 0 || !shell.IsFrontmatterDelimiter(lines[0]) {
		return parsed, nil
	}
	end := slices.IndexFunc(lines[1:], shell.IsFrontmatterDelimiter)
	if end < 0 {
		return parsed, errors.New("the frontmatter has no closing ---")
	}
	decoder := yaml.NewDecoder(strings.NewReader(strings.Join(lines[1:end+1], "\n")))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil && !errors.Is(err, io.EOF) {
		return parsed, fmt.Errorf("the frontmatter is not YAML (its line numbers count from the line after ---): %w", err)
	}
	// A `...` ends a YAML document, and the keys after it would be a second one that a first-document reader
	// never sees.
	var second yaml.Node
	if err := decoder.Decode(&second); err == nil {
		return parsed, errors.New("the frontmatter holds a second YAML document")
	} else if !errors.Is(err, io.EOF) {
		return parsed, fmt.Errorf("the frontmatter is not YAML (its line numbers count from the line after ---): %w", err)
	}
	if len(document.Content) == 0 {
		return parsed, nil
	}
	mapping := document.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return parsed, errors.New("the frontmatter is not a YAML mapping")
	}
	// A rewrite replaces a field's line, so each field needs a line of its own.
	if mapping.Style&yaml.FlowStyle != 0 {
		return parsed, errors.New("the frontmatter is one flow mapping; write each field on a line of its own")
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key, value := mapping.Content[i], mapping.Content[i+1]
		if _, seen := parsed.values[key.Value]; seen {
			return parsed, fmt.Errorf("the frontmatter names %s twice", key.Value)
		}
		parsed.values[key.Value] = value
		// key.Line is already the key's index into lines: the parser counts from 1 within the block, and the
		// block starts on the file's second line.
		parsed.keyLine[key.Value] = key.Line
	}
	return parsed, nil
}

// readFrontmatter parses a file's frontmatter. It returns an unreadable file's error like a parse error, and
// no caller tells the two apart: requireReport refuses an unreadable report before any field is read.
func readFrontmatter(path string) (frontmatter, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return frontmatter{}, err
	}
	return parseFrontmatter(shell.SplitLines(string(content)))
}

// value returns a field's scalar value without YAML's quotes or trailing comment, or "" when the field is
// missing, null, or a list or mapping.
func (f frontmatter) value(field string) string {
	value := f.values[field]
	if value == nil || value.Kind != yaml.ScalarNode || value.Tag == "!!null" {
		return ""
	}
	return value.Value
}

// fieldValue returns a field's value in a file's frontmatter, or "" when the file has none to read. "" is in
// the unstamped set, so a gate that cannot read the report refuses it.
func fieldValue(path, field string) string {
	parsed, err := readFrontmatter(path)
	if err != nil {
		return ""
	}
	return parsed.value(field)
}

// hasField reports whether a file's frontmatter names the field, even with an empty value. A file that cannot
// be read or parsed names none.
func hasField(path, field string) bool {
	parsed, err := readFrontmatter(path)
	_, has := parsed.values[field]
	return err == nil && has
}

// statusOf returns an intent file's status, or what keeps it from being read: an intent whose frontmatter
// does not parse has no status, and a reader that said only that would send its author looking for a
// missing field.
func statusOf(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return "<an intent file that cannot be read: " + err.Error() + ">"
	}
	parsed, err := parseFrontmatter(shell.SplitLines(string(content)))
	if err != nil {
		return "<a frontmatter that does not parse: " + err.Error() + ">"
	}
	return parsed.value("status")
}

// fieldLine returns the line a field is on, as the human wrote it, or "" without one. init quotes it back
// when it refuses over a report, and what the human recognises is the line, not its parsed value.
func fieldLine(path, field string) string {
	parsed, err := readFrontmatter(path)
	at, has := parsed.keyLine[field]
	if err != nil || !has {
		return ""
	}
	return parsed.lines[at]
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

// Entries the last stamp marked `(turnaround)` — stages trimmed to answer sooner. Any of them means
// the pass was trimmed.
func (r *run) turnaroundTrims() string {
	var trimmed []string
	for _, entry := range strings.Split(r.reviewedStages(), ",") {
		if strings.Contains(entry, "(turnaround)") {
			trimmed = append(trimmed, entry)
		}
	}
	return strings.Join(trimmed, " ")
}

// Guarded exactly as reportNameFor guards a stem, and the leading dot is why: the slug charset alone
// admits `.` and `..`, and this value is a DIRECTORY COMPONENT of every path built from it, so `..`
// climbs out of the directory it was meant to index. paths.go's own leading-dot case holds the full
// account of what that reaches.
func (r *run) intentSlug() string {
	slug := firstField(fieldValue(r.report, "intent"))
	if slug == "" || strings.HasPrefix(slug, "review:") ||
		strings.HasPrefix(slug, ".") || !isSlugCharset(slug) {
		return ""
	}
	return slug
}

// The template carries the frontmatter every later reader parses, and gate and state never read the
// template itself — so a missing field or a drifted placeholder can only be caught here, before a
// report is scaffolded from it. A drifted placeholder stamps every new report as already reviewed.
func (r *run) assertTemplateStampable() {
	if shell.IsSymlink(r.template) {
		r.refuse("error: template " + r.template + " is a symlink — refusing to read the template through one; the report was NOT initialized")
	}
	if !shell.IsRegularFile(r.template) {
		r.refuse("error: template not found (" + r.template + ")")
	}
	if _, err := readFrontmatter(r.template); err != nil {
		r.refuse("error: template " + r.template + ": " + shell.Oneline(err.Error()) + " — the report was NOT initialized")
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
// one. Same directory means the rename can never cross a device. A leftover joins no listing because it
// lands INSIDE the ship folder, which reportNames never opens — it lists the folders under intents/.
func (r *run) rewriteReport(noTemp, noWrite string, rewrite func([]string) ([]string, error)) error {
	temp, err := os.CreateTemp(shell.DirName(r.report), ".rewrite.")
	if err != nil {
		r.errLines("error: mktemp failed — " + noTemp)
		return err
	}
	// One failure path for read, write, close and rename together: the report is untouched until the
	// rename, so they all leave the same thing standing and owe the caller the same sentence. First
	// error wins, and the temp file goes whichever one it was.
	content, err := os.ReadFile(r.report)
	var rewritten []string
	if err == nil {
		rewritten, err = rewrite(shell.SplitLines(string(content)))
	}
	if err == nil {
		_, err = temp.Write(joinRecords(rewritten))
	}
	if closed := temp.Close(); err == nil {
		err = closed
	}
	if err == nil {
		err = moveFile(temp.Name(), r.report)
	}
	if err != nil {
		_ = os.Remove(temp.Name())
		r.errLines("error: "+noWrite, "  "+shell.Oneline(err.Error()))
	}
	return err
}

// rewriteFields replaces the line each key of replacements is on with the lines given for it, and leaves every
// other line, the body's too, as it was. A key the frontmatter lacks is skipped, with nothing written for it.
// A value that does not sit whole on its key's line cannot be replaced line by line, so it is an error.
func rewriteFields(lines []string, replacements map[string][]string) ([]string, error) {
	parsed, err := parseFrontmatter(lines)
	if err != nil {
		return nil, err
	}
	at := map[int][]string{}
	for key, replacement := range replacements {
		line, has := parsed.keyLine[key]
		if !has {
			continue
		}
		alone, err := parseFrontmatter([]string{"---", lines[line], "---"})
		if err != nil || alone.values[key] == nil || alone.values[key].Kind != parsed.values[key].Kind ||
			alone.value(key) != parsed.value(key) {
			return nil, fmt.Errorf("%s: its value runs past its own line", key)
		}
		at[line] = replacement
	}
	out := make([]string, 0, len(lines)+2)
	for i, line := range lines {
		if replacement, has := at[i]; has {
			out = append(out, replacement...)
			continue
		}
		out = append(out, line)
	}
	return out, nil
}

// fieldOf writes a frontmatter line with the value as a YAML string, quoted where YAML needs it, so a value
// an outsider chose, such as an intent seeded from a ticket, reads back as itself: never as a second field,
// and never as null or a bool, as a bare null or true would. The line is parsed back, and a value it does not
// return, such as one that is not UTF-8, is an error.
func fieldOf(key, value string) (string, error) {
	node := yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
	// yaml.v3 writes a string with a newline as a block scalar over several lines. Double quotes keep it on the
	// key's line, which the next rewrite needs.
	if strings.ContainsAny(value, "\n\r") {
		node.Style = yaml.DoubleQuotedStyle
	}
	written, err := yaml.Marshal(&node)
	if err != nil {
		return "", err
	}
	line := key + ": " + strings.TrimSuffix(string(written), "\n")
	if back, err := parseFrontmatter([]string{"---", line, "---"}); err != nil || back.value(key) != value {
		return "", fmt.Errorf("%s: the value does not survive being written as YAML", key)
	}
	return line, nil
}

// fieldsOf writes one frontmatter line per key and value pair, in order.
func fieldsOf(pairs ...string) ([]string, error) {
	lines := make([]string, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		line, err := fieldOf(pairs[i], pairs[i+1])
		if err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	return lines, nil
}

// init's rewrite: the frontmatter's intent.
func rewriteIntent(intent string) func([]string) ([]string, error) {
	return func(lines []string) ([]string, error) {
		written, err := fieldsOf("intent", intent)
		if err != nil {
			return nil, err
		}
		return rewriteFields(lines, map[string][]string{"intent": written})
	}
}

// stamp's rewrite: the stamp replaces reviewed-tree and writes the other two beside it, so a report carrying
// them in any order ends up with one of each, and a `reviewed-mode:` line from an older layout is dropped
// rather than left to be read.
func rewriteStamp(tree, worktree, entries string) func([]string) ([]string, error) {
	return func(lines []string) ([]string, error) {
		stamp, err := fieldsOf("reviewed-tree", tree, "reviewed-worktree", worktree, "reviewed-stages", entries)
		if err != nil {
			return nil, err
		}
		return rewriteFields(lines, map[string][]string{
			"reviewed-mode":     nil,
			"reviewed-stages":   nil,
			"reviewed-worktree": nil,
			"reviewed-tree":     stamp,
		})
	}
}

func rewriteInvalidated(lines []string) ([]string, error) {
	pending, err := fieldsOf("reviewed-tree", "pending", "reviewed-worktree", "pending", "reviewed-stages", "pending")
	if err != nil {
		return nil, err
	}
	return rewriteFields(lines, map[string][]string{
		"reviewed-tree":     pending[:1],
		"reviewed-worktree": pending[1:2],
		"reviewed-stages":   pending[2:],
	})
}

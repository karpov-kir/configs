package ecoreport

import (
	"path/filepath"
	"regexp"
	"strings"

	"configs/ai/tools/shell"
)

/**
 * Appends the one local entry `text` names to the project record of the same name. An exact
 * restatement bumps the existing entry, and a full project record refuses. The function rebases
 * the entry's relative links, since the project record sits at a different depth. It also keeps a
 * promotion candidate out of `## Decisions`, where every later ship would read it as settled.
 */
func (r *run) recordMerge(kind *recordKind, path, text string) {
	entry, isCandidate := r.oneLocalEntry(kind, path, text)
	project := recordKindFor("project-" + strings.TrimPrefix(kind.name, "local-"))
	r.refuseSharedRecordUnderAForeignSlot(project)
	projectPath := r.recordPath(project)
	rebase := linkRebase{
		fromDir:    filepath.Dir(path),
		toDir:      filepath.Dir(projectPath),
		shipDir:    r.shipDir(r.recordSlug),
		archiveDir: r.archiveDir(r.recordSlug),
	}
	rebased := r.entryText(project, rebase.apply(entry.text))
	if isNew := r.recordAppend(project, projectPath, rebased); isNew && isCandidate {
		r.recordClassify(project, projectPath, rebased, "candidate")
	}
}

/**
 * Returns the one entry `text` names in a local record, read under the record's lock and left
 * unchanged, and whether it sits under the promotion candidates of a decisions record.
 */
func (r *run) oneLocalEntry(kind *recordKind, path, text string) (recordEntry, bool) {
	handle, lines, _ := r.openLockedRecord(path, false)
	defer handle.Close()
	r.assertDecisionSections(kind, path, lines)
	entry := r.oneMatchingEntry(path, lines, text, "merged")
	return entry, kind.file == "decisions.md" && entry.line < decisionsHeadingLine(lines)
}

var markdownLinkTarget = regexp.MustCompile(`\]\([^)\s]+`)

/**
 * Holds the folders one entry's links move between, from a ship's local record to the project
 * record.
 */
type linkRebase struct {
	fromDir, toDir      string
	shipDir, archiveDir string
}

/**
 * Returns text with each relative Markdown link target rewritten to reach the same file from toDir
 * as it reached from fromDir. A target inside the ship's folder is pointed at the ship's archive,
 * because finalize renames that folder and the project record outlives the rename. A fragment is
 * kept. A URL, a root-absolute path and a fragment-only link are left as written.
 */
func (rebase linkRebase) apply(text string) string {
	return markdownLinkTarget.ReplaceAllStringFunc(text, func(link string) string {
		target := strings.TrimPrefix(link, "](")
		path, fragment := target, ""
		if index := strings.IndexByte(target, '#'); index >= 0 {
			path, fragment = target[:index], target[index:]
		}
		if path == "" || strings.HasPrefix(path, "/") || strings.HasPrefix(path, "<") || strings.Contains(path, ":") {
			return link
		}
		target = filepath.Join(rebase.fromDir, path)
		shipDir := filepath.Clean(rebase.shipDir)
		if shell.IsWithin(target, shipDir) {
			target = rebase.archiveDir + strings.TrimPrefix(target, shipDir)
		}
		rebased, err := filepath.Rel(rebase.toDir, target)
		if err != nil {
			return link
		}
		return "](" + filepath.ToSlash(rebased) + fragment
	})
}

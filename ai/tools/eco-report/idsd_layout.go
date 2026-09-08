package ecoreport

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"kk-flavor/tools/shell"
)

const agentsDirName = "for-agents"

var agentRecordFiles = []string{"decisions.md", "playbook.md", "language.md"}

func (r *run) projectAgentsDir() string         { return filepath.Join(r.idsdDir, agentsDirName) }
func (r *run) shipAgentsDir(slug string) string { return filepath.Join(r.shipDir(slug), agentsDirName) }

func (r *run) cmdLayout() {
	switch {
	case len(r.args) == 2 && r.arg(1) == "check":
		findings := r.idsdLayoutFindings()
		if len(findings) > 0 {
			for i := range findings {
				findings[i] = shell.Oneline(findings[i])
			}
			r.errLines(append([]string{"BLOCK (idsd layout):"}, findings...)...)
			r.exit(1)
		}
		r.line("idsd layout is clean: %s", shell.Oneline(r.idsdDir))
	case len(r.args) == 3 && r.arg(1) == "migrate" && (r.arg(2) == "--dry-run" || r.arg(2) == "--apply"):
		r.cmdMigrateLayout()
	default:
		r.refuse("usage: report.sh layout {check|migrate --dry-run|migrate --apply}")
	}
}

// Do not interpret an old report as absent and create a second qualification history beside it.
func (r *run) assertCurrentIdsdLayout() {
	switch r.arg(0) {
	case "root", "repo-mode", "layout":
		return
	}
	var legacy []string
	for _, name := range append(slices.Clone(agentRecordFiles), "constraints.md") {
		path := filepath.Join(r.idsdDir, name)
		if shell.PathExists(path) {
			legacy = append(legacy, path)
		}
	}
	for _, group := range []string{"intents", "archive"} {
		entries, err := os.ReadDir(filepath.Join(r.idsdDir, group))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			for _, name := range append(slices.Clone(agentRecordFiles), reportName) {
				path := filepath.Join(r.idsdDir, group, entry.Name(), name)
				if shell.PathExists(path) {
					legacy = append(legacy, path)
				}
			}
		}
	}
	if len(legacy) > 0 {
		r.refuse(append([]string{"error: legacy idsd layout; no report or record was read or written.",
			"  Run report.sh layout migrate --dry-run. Complete and close open legacy reports using the pinned previous runtime before migration; do not delete their obligations."}, sanitizeLayoutPaths(legacy)...)...)
	}
}

func (r *run) idsdLayoutFindings() []string {
	if _, err := os.Lstat(r.idsdDir); os.IsNotExist(err) {
		return []string{"  no idsd root at " + r.idsdDir}
	}
	var findings []string
	hasLinks := false
	err := filepath.WalkDir(r.idsdDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			findings = append(findings, "  cannot inspect "+path+": "+err.Error())
			return nil
		}
		relative, _ := filepath.Rel(r.idsdDir, path)
		if entry.Type()&os.ModeSymlink != 0 {
			hasLinks = true
			findings = append(findings, "  symlink: "+path)
			return nil
		}
		if relative == "." {
			if !entry.IsDir() {
				findings = append(findings, "  idsd root is not a directory: "+path)
			}
			return nil
		}
		if !entry.IsDir() && !entry.Type().IsRegular() {
			findings = append(findings, "  nonregular artifact: "+path)
			return nil
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		if len(parts) == 2 && entry.IsDir() && (parts[0] == "intents" || parts[0] == "archive") && !(parts[0] == "intents" && parts[1] == "review") {
			intent := filepath.Join(path, intentName)
			info, err := os.Lstat(intent)
			if err != nil || !info.Mode().IsRegular() {
				findings = append(findings, "  missing regular intent.md: "+path)
			}
		}
		if !entry.IsDir() && entry.Name() == "decisions.md" && filepath.Base(filepath.Dir(path)) == agentsDirName && idsdAllowedPath(relative, false) {
			body, err := os.ReadFile(path)
			if err == nil {
				err = decisionSectionsError(shell.SplitLines(string(body)))
			}
			if err != nil {
				findings = append(findings, "  "+path+": "+err.Error())
			}
		}
		if !idsdAllowedPath(relative, entry.IsDir()) {
			findings = append(findings, "  misplaced artifact: "+path)
		}
		return nil
	})
	if err != nil {
		findings = append(findings, "  cannot inspect idsd root: "+err.Error())
	}
	if hasLinks {
		return findings
	}
	legacyConstraints := filepath.Join(r.projectAgentsDir(), "supporting", "constraints.md")
	if shell.PathExists(legacyConstraints) {
		findings = append(findings, "  "+legacyConstraints+": curate these preserved constraints into charter.md with the human, then retire this legacy copy")
	}
	charter := filepath.Join(r.idsdDir, "charter.md")
	if info, err := os.Lstat(charter); err == nil && info.Mode().IsRegular() {
		findings = append(findings, charterLayoutFindings(charter)...)
	} else if r.hasIntentFiles() {
		findings = append(findings, "  missing charter.md; curate the human charter through idsd-charter")
	}
	return findings
}

func idsdAllowedPath(relative string, isDir bool) bool {
	parts := strings.Split(filepath.ToSlash(relative), "/")
	switch parts[0] {
	case "charter.md", "roadmap.md":
		return len(parts) == 1 && !isDir
	case agentsDirName:
		return allowedAgentPath(parts[1:], isDir)
	case "intents", "archive":
		if len(parts) == 1 {
			return isDir
		}
		if reportNameFor(parts[1]) != parts[1] {
			return false
		}
		if len(parts) == 2 {
			return isDir
		}
		if parts[2] == intentName {
			return len(parts) == 3 && !isDir
		}
		if parts[2] == agentsDirName {
			if parts[0] == "archive" && len(parts) == 4 && parts[3] == reportName {
				return false
			}
			return allowedShipAgentPath(parts[3:], isDir)
		}
	}
	return false
}

func allowedAgentPath(parts []string, isDir bool) bool {
	if len(parts) == 0 {
		return isDir
	}
	if parts[0] == "supporting" {
		return len(parts) > 1 || isDir
	}
	return len(parts) == 1 && !isDir && slices.Contains(agentRecordFiles, parts[0])
}

func allowedShipAgentPath(parts []string, isDir bool) bool {
	if len(parts) == 1 && parts[0] == reportName {
		return !isDir
	}
	return allowedAgentPath(parts, isDir)
}

func (r *run) hasIntentFiles() bool {
	for _, group := range []string{"intents", "archive"} {
		entries, _ := os.ReadDir(filepath.Join(r.idsdDir, group))
		for _, entry := range entries {
			if entry.IsDir() && shell.IsRegularFile(filepath.Join(r.idsdDir, group, entry.Name(), intentName)) {
				return true
			}
		}
	}
	return false
}

func sanitizeLayoutPaths(paths []string) []string {
	for i := range paths {
		paths[i] = shell.Oneline(paths[i])
	}
	return paths
}

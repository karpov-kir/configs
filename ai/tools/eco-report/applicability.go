package ecoreport

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"kk-flavor/tools/diffscan"
)

const scopeMarker = ".scope.json"

// Increment when classification changes; an older receipt cannot authorize newer skip rules.
const scopeVersion = 1
const maxProseBytes = 1024 * 1024

// Only positive evidence permits a skip. Unknown formats, modes or contents require review.
type scopeFile struct {
	name    string
	mode    string
	body    []byte
	wasRead bool
}

type scopeReceipt struct {
	Version  int               `json:"version"`
	Base     string            `json:"base"`
	Head     string            `json:"head"`
	Tree     string            `json:"tree"`
	Worktree string            `json:"worktree"`
	Stages   map[string]string `json:"stages"`
}

func (r *run) cmdScope() {
	r.requireReport(r.arg(2))
	if r.arg(1) == "" || len(r.args) > 3 || strings.HasPrefix(r.arg(1), "-") {
		r.refuse("usage: report.sh scope <base-ref> [<intent>]")
	}
	if r.reviewedTree() != "pending" {
		r.refuse("error: invalidate before recording scope")
	}
	base, status := r.captureGit(r.errOut, "rev-parse", "--verify", "--end-of-options", r.arg(1)+"^{commit}")
	if status != 0 || base == "" {
		r.refuse("error: scope base did not resolve to a commit")
	}
	head, status := r.captureGit(r.errOut, "rev-parse", "--verify", "HEAD^{commit}")
	if status != 0 || head == "" {
		r.refuse("error: scope HEAD did not resolve to a commit")
	}
	tree, ok := r.currentTree(r.errOut)
	if !ok {
		r.exit(2)
	}
	token, ok := r.worktreeToken()
	if !ok {
		r.refuse("error: scope worktree identity could not be established")
	}
	stages := r.scopeStages(base)
	after, ok := r.currentTree(r.errOut)
	newHead, status := r.captureGit(r.errOut, "rev-parse", "--verify", "HEAD^{commit}")
	if !ok || after != tree || status != 0 || newHead != head {
		r.refuse("error: tree moved while measuring scope; rerun scope")
	}
	receipt := scopeReceipt{Version: scopeVersion, Base: base, Head: head, Tree: tree, Worktree: token, Stages: stages}
	body, err := json.Marshal(receipt)
	if err != nil {
		r.refuse("error: scope could not be encoded")
	}
	r.writeStageMarker(scopeMarker, string(body))
	r.line("scope base: %s", base)
	for _, stage := range []string{"security-review", "edit", "refactor"} {
		r.line("%s: %s", stage, stages[stage])
	}
}

func (r *run) scopeStages(base string) map[string]string {
	stages := map[string]string{"security-review": "not-applicable", "edit": "not-applicable", "refactor": "not-applicable"}
	raw, status := r.captureGit(r.errOut, "diff", "--raw", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", "--no-relative", "--ignore-submodules=none", base, "--")
	if status != 0 {
		r.refuse("error: scope diff could not be read")
	}
	fields := strings.Split(raw, "\x00")
	for i := 0; i < len(fields)-1; i += 2 {
		if i+1 >= len(fields)-1 {
			r.refuse("error: malformed scope diff")
		}
		header := strings.Fields(fields[i])
		if len(header) != 5 || !strings.HasPrefix(header[0], ":") {
			r.refuse("error: malformed scope diff header")
		}
		name := fields[i+1]
		if !isScopePath(name) {
			r.refuse("error: invalid path in scope diff")
		}
		oldMode := strings.TrimPrefix(header[0], ":")
		if oldMode != "000000" {
			// Inspect both sides: deleting code or renaming it as prose does not remove its review obligation.
			if oldMode == "100644" && isPlainProse(name, nil) {
				body, wasRead := r.readProseBlob(header[2])
				classifyScopeFile(stages, scopeFile{name: name, mode: oldMode, body: body, wasRead: wasRead})
			} else {
				classifyScopeFile(stages, scopeFile{name: name, mode: oldMode})
			}
		}
		if header[1] != "000000" {
			r.classifyWorkingFile(stages, name)
		}
	}
	if len(fields) > 0 && fields[len(fields)-1] != "" {
		r.refuse("error: incomplete scope diff")
	}
	untracked, status := r.captureGit(r.errOut, "ls-files", "--others", "--exclude-standard", "-z")
	if status != 0 {
		r.refuse("error: scope untracked files could not be read")
	}
	for _, name := range strings.Split(untracked, "\x00") {
		if name == "" {
			continue
		}
		if !isScopePath(name) {
			r.refuse("error: invalid untracked scope path")
		}
		r.classifyWorkingFile(stages, name)
	}
	return stages
}

func (r *run) readProseBlob(blob string) ([]byte, bool) {
	size, status := r.captureGit(nil, "cat-file", "-s", blob)
	count, err := strconv.ParseInt(size, 10, 64)
	if status != 0 || err != nil || count < 0 || count > maxProseBytes {
		return nil, false
	}
	body, status := r.captureGit(nil, "cat-file", "blob", blob)
	return []byte(body), status == 0
}

func isScopePath(name string) bool {
	return name != "" && !filepath.IsAbs(name) && filepath.Clean(name) == name && name != ".." && !strings.HasPrefix(name, "../")
}

func (r *run) classifyWorkingFile(stages map[string]string, name string) {
	full := filepath.Join(r.root, name)
	info, err := os.Lstat(full)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 != 0 || info.Size() > maxProseBytes || !isPlainProse(name, nil) {
		classifyScopeFile(stages, scopeFile{name: name, mode: "unknown"})
		return
	}
	body, err := os.ReadFile(full)
	classifyScopeFile(stages, scopeFile{name: name, mode: "100644", body: body, wasRead: err == nil})
}

func classifyScopeFile(stages map[string]string, file scopeFile) {
	name, mode, body, wasRead := file.name, file.mode, file.body, file.wasRead
	stages["edit"] = "run"
	if !wasRead || mode != "100644" || !isPlainProse(name, body) {
		stages["security-review"] = "run"
		stages["refactor"] = "run"
		return
	}
	lower := "/" + strings.ToLower(filepath.ToSlash(name))
	if diffscan.SecretNamed(name) || strings.Contains(lower, "instructions") {
		stages["security-review"] = "run"
	}
	for _, part := range []string{"/agents.md", "/claude.md", "/skill.md", "/skills/", "/standards/", "/prompts/", "/templates/", "/.idsd/"} {
		if strings.Contains(lower, part) {
			stages["security-review"] = "run"
		}
	}
}

func isPlainProse(name string, body []byte) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md":
	default:
		return false
	}
	if len(body) > maxProseBytes || !utf8.Valid(body) || bytes.HasPrefix(body, []byte("#!")) {
		return false
	}
	for _, marker := range []string{"<", "{", "[", "]", "`", "~~~", ":::"} {
		if bytes.Contains(body, []byte(marker)) {
			return false
		}
	}
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import") || strings.HasPrefix(trimmed, "export") {
			return false
		}
		if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") || strings.TrimSpace(line) == "---" || strings.TrimSpace(line) == "+++" {
			return false
		}
	}
	for _, b := range body {
		if (b < 32 && b != '\t' && b != '\n' && b != '\r') || b == 127 {
			return false
		}
	}
	return true
}

func (r *run) skipBlockReasons(entries string) []string {
	var skipped []string
	for _, entry := range strings.Split(entries, ",") {
		if strings.HasSuffix(entry, ":skipped(not-applicable)") {
			stage, _, _ := strings.Cut(entry, ":")
			skipped = append(skipped, stage)
		}
	}
	if len(skipped) == 0 {
		return nil
	}
	receipt, ok := r.readScopeReceipt()
	if !ok {
		return []string{"scope receipt is missing, stale or invalid; run scope <base-ref> for this candidate, or run every stage"}
	}
	var problems []string
	for _, stage := range skipped {
		if receipt.Stages[stage] != "not-applicable" {
			problems = append(problems, stage+": scope requires this stage to run")
		}
	}
	return problems
}

func (r *run) readScopeReceipt() (scopeReceipt, bool) {
	var receipt scopeReceipt
	path := r.stageReturnsDir + "/" + scopeMarker
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return receipt, false
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return receipt, false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&receipt) != nil || decoder.Decode(new(json.RawMessage)) != io.EOF || receipt.Version != scopeVersion || receipt.Base == "" || receipt.Head == "" {
		return receipt, false
	}
	if len(receipt.Stages) != 3 {
		return receipt, false
	}
	for _, stage := range []string{"security-review", "edit", "refactor"} {
		if receipt.Stages[stage] != "run" && receipt.Stages[stage] != "not-applicable" {
			return receipt, false
		}
	}
	tree, ok := r.currentTreeCached(r.errOut)
	if !ok || tree != receipt.Tree {
		return receipt, false
	}
	head, status := r.captureGit(nil, "rev-parse", "--verify", "HEAD^{commit}")
	token, ok := r.worktreeToken()
	return receipt, ok && status == 0 && head == receipt.Head && token == receipt.Worktree
}

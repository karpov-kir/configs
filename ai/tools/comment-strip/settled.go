package commentstrip

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	readerjudge "configs/ai/tools/reader-judge"
)

// The refactor lane re-decided its own settled forms on every full pass. Run 20's lane kept an
// interface, and run 26's turned it into a type alias. The archive keeps the lane's decisions as it
// keeps a writer's declines, and the next lane leaves a settled declaration as it stands while the
// code under it holds.

// settled is one declaration the refactor lane ruled on: kept as it stood, or edited into the code its
// span holds.
type settled struct {
	Run  string `json:"run"`
	Decl string `json:"decl"`
	Span string `json:"span"`
}

func settledName(archive, path string) string {
	return filepath.Join(archive, strings.TrimSuffix(archiveName(path, 0), "@0.facts")+".settled")
}

func readSettled(archive, path string) []settled {
	body, err := os.ReadFile(settledName(archive, path))
	if err != nil {
		return nil
	}
	var held []settled
	if json.Unmarshal(body, &held) != nil {
		return nil
	}
	return held
}

// topDeclarations is each top declaration of the file, by its text, with the sum of the code under it.
func topDeclarations(lines []string) map[string]string {
	out := map[string]string{}
	for n, line := range lines {
		if declaredNameOf(line) != "" {
			out[strings.TrimSpace(line)] = codeSum(lines, n+1)
		}
	}
	return out
}

// Settle records the refactor lane's decisions on one file. `before` and `after` are the file on either
// side of the lane, and `stays` the lines of `before` with a comment the lane kept. A record lapses where
// its code changed before the lane read it. Each settled declaration the lane edited is returned, and
// then the file's records stay as they were.
func Settle(archive, run, path string, before, after []string, stays []int) ([]string, error) {
	was, now := topDeclarations(before), topDeclarations(after)
	var kept []settled
	var edited []string
	for _, s := range readSettled(archive, path) {
		if spanOf(before, s.Decl) != s.Span {
			continue
		}
		if spanOf(after, s.Decl) != s.Span {
			edited = append(edited, fmt.Sprintf("the lane edited `%s`, which %s settled", s.Decl, s.Run))
			continue
		}
		kept = append(kept, s)
	}
	if len(edited) > 0 {
		return edited, nil
	}
	ruled := map[string]string{}
	for decl, span := range now {
		if was[decl] != span {
			ruled[decl] = span
		}
	}
	for _, at := range stays {
		for _, u := range readerjudge.CommentBlocks(before) {
			if at < u.Line || at >= u.Line+u.Span {
				continue
			}
			if decl := declarationUnder(before, u); decl <= len(before) {
				text := strings.TrimSpace(before[decl-1])
				if span := spanOf(after, text); span != "" {
					ruled[text] = span
				}
			}
		}
	}
	for _, s := range kept {
		if _, again := ruled[s.Decl]; !again {
			ruled[s.Decl] = s.Span
		}
	}
	// The file's records are written whole, so a record that lapsed leaves the file with them.
	var out []settled
	for decl, span := range ruled {
		run := run
		for _, s := range kept {
			if s.Decl == decl && s.Span == span {
				run = s.Run
			}
		}
		out = append(out, settled{Run: run, Decl: decl, Span: span})
	}
	if len(out) == 0 {
		if err := os.Remove(settledName(archive, path)); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		return nil, nil
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Decl < out[j].Decl })
	body, err := json.MarshalIndent(out, "", " ")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(archive, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create the archive at %s", archive)
	}
	return nil, os.WriteFile(settledName(archive, path), append(body, '\n'), 0o644)
}

// spanOf is the sum of the code under the declaration's first line in the file. It is empty for a
// declaration the file lacks. A member's declaration is indented, and its text is read trimmed.
func spanOf(lines []string, decl string) string {
	for n, line := range lines {
		if strings.TrimSpace(line) == decl {
			return codeSum(lines, n+1)
		}
	}
	return ""
}

// SettledHolding is each declaration of the file a lane settled whose code still holds, for the next
// lane's prompt.
func SettledHolding(archive, path string, lines []string) []string {
	var out []string
	for _, s := range readSettled(archive, path) {
		for n, line := range lines {
			if strings.TrimSpace(line) == s.Decl && codeSum(lines, n+1) == s.Span {
				out = append(out, fmt.Sprintf("%s:%d %s, as %s settled it", path, n+1, s.Decl, s.Run))
				break
			}
		}
	}
	return out
}

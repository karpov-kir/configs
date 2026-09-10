package ecoreport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

const stageResultUsage = `usage: report.sh stage-result <json-file> [<intent>]
  Copy {version:1,attempt,head,tree,worktree} from invalidate or result-context.
  Add id, stage, status:"complete", outcome:"complete", items:[] (all required).
  stage: code-review | security-review | edit | refactor
  refactor also accepts outcome:"partial(turnaround)" or "partial(cap)".
  Each item requires id, kind, severity, action, evidence, recommendation.
  kind: Falsified | Fork | Pending evidence
  severity: critical | high | medium | low | info
  IDs: ASCII alphanumeric first, then alphanumeric . _ -, 1–128 bytes.
  Text fields: nonempty single-line UTF-8, at most 16384 bytes, no control characters.
  Maximum 1000 items and 4 MiB input. Null, duplicate keys and unknown fields refuse.
  Only unresolved Decide items belong here. Rendered findings are immutable except [ ] → [x].
  Retry an interrupted result with the identical payload; completed duplicate IDs refuse.
`

func decodeResultJson(body []byte, target any) error {
	if !utf8.Valid(body) {
		return fmt.Errorf("JSON must be UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := checkResultJsonValue(decoder, 0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("JSON must contain exactly one value")
	}
	decoder = json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func checkResultJsonValue(decoder *json.Decoder, depth int) error {
	if depth > 12 {
		return fmt.Errorf("JSON nesting is too deep")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token == nil {
		return fmt.Errorf("null is not a result value")
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return nil
	}
	keys := map[string]bool{}
	for decoder.More() {
		if delimiter == '{' {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || keys[name] || !isCanonicalResultKey(name) {
				return fmt.Errorf("duplicate or invalid JSON key")
			}
			keys[name] = true
		}
		if err := checkResultJsonValue(decoder, depth+1); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
}

func isCanonicalResultKey(name string) bool {
	if name == "" {
		return false
	}
	for _, letter := range []byte(name) {
		if letter < 'a' || letter > 'z' {
			return false
		}
	}
	return true
}

func isValidResultId(value string) bool {
	if len(value) < 1 || len(value) > 128 {
		return false
	}
	for index, letter := range []byte(value) {
		if letter >= 'a' && letter <= 'z' || letter >= 'A' && letter <= 'Z' || letter >= '0' && letter <= '9' {
			continue
		}
		if index == 0 || letter != '.' && letter != '_' && letter != '-' {
			return false
		}
	}
	return true
}

func validateStageResult(result stageResult) error {
	if result.Version != 1 || !isValidResultId(result.Attempt) || result.Head == "" || result.Tree == "" || !isWorktreeToken(result.Worktree) {
		return fmt.Errorf("missing or invalid result context")
	}
	if !isValidResultId(result.Id) {
		return fmt.Errorf("invalid result id")
	}
	switch result.Stage {
	case resultCodeReview, resultSecurityReview, resultEdit, resultRefactor:
	default:
		return fmt.Errorf("unknown stage")
	}
	if result.Status != resultCompleted {
		return fmt.Errorf("worker status must be complete; failed or incomplete results cannot pass")
	}
	if result.Outcome != outcomeComplete && (result.Stage != resultRefactor || result.Outcome != outcomePartialTurnaround && result.Outcome != outcomePartialCap) {
		return fmt.Errorf("invalid stage outcome")
	}
	if result.Items == nil || len(result.Items) > 1000 {
		return fmt.Errorf("items must be an explicit array of at most 1000 findings")
	}
	ids := map[string]bool{}
	for _, item := range result.Items {
		if !isValidResultId(item.Id) || ids[item.Id] {
			return fmt.Errorf("duplicate or invalid finding id")
		}
		ids[item.Id] = true
		if item.Kind != findingFalsified && item.Kind != findingFork && item.Kind != findingPendingEvidence {
			return fmt.Errorf("unknown finding kind")
		}
		switch item.Severity {
		case severityCritical, severityHigh, severityMedium, severityLow, severityInfo:
		default:
			return fmt.Errorf("unknown finding severity")
		}
		for _, text := range []string{item.Action, item.Evidence, item.Recommendation} {
			if strings.TrimSpace(text) == "" || len(text) > 16384 || !utf8.ValidString(text) || strings.IndexFunc(text, func(letter rune) bool { return unicode.IsControl(letter) || letter == '\u2028' || letter == '\u2029' }) >= 0 {
				return fmt.Errorf("finding text must be nonempty single-line UTF-8 without controls, at most 16384 bytes")
			}
		}
	}
	return nil
}

func escapeResultText(text string) string {
	var escaped strings.Builder
	for _, letter := range text {
		if strings.ContainsRune("\\`*_{}[]<>()!|~", letter) {
			escaped.WriteRune('\\')
		}
		escaped.WriteRune(letter)
	}
	return escaped.String()
}

func renderResultItem(item resultItem) string {
	return fmt.Sprintf("- [ ] **%s · %s —** %s\n  **Severity:** %s\n  **Evidence:** %s\n  **Recommend:** %s", escapeResultText(item.Id), item.Kind, escapeResultText(item.Action), item.Severity, escapeResultText(item.Evidence), escapeResultText(item.Recommendation))
}

func verifyResultItems(body []byte, items []resultItem) error {
	visible, err := resultVisibleLines(body)
	if err != nil {
		return err
	}
	text := "\n" + strings.Join(visible, "\n") + "\n"
	for _, item := range items {
		open := "\n" + renderResultItem(item) + "\n"
		closed := strings.Replace(open, "- [ ]", "- [x]", 1)
		if strings.Count(text, open)+strings.Count(text, closed) != 1 {
			return fmt.Errorf("finding %s is missing, changed, duplicated or hidden", item.Id)
		}
	}
	return nil
}

func visibleResultReport(body []byte) error {
	_, err := resultVisibleLines(body)
	return err
}

// Match todo-gate's fence/comment visibility so stored checkboxes cannot pass while hidden from it.
func resultVisibleLines(body []byte) ([]string, error) {
	var visible []string
	inFence, inComment := false, false
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimLeft(line, " \t\r\v\f")
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.Contains(line, "<!--") {
			inComment = true
		}
		if inComment {
			if strings.Contains(line, "-->") {
				inComment = false
			}
			continue
		}
		visible = append(visible, line)
	}
	if inFence || inComment {
		return nil, fmt.Errorf("report has an unclosed fence or comment hiding findings")
	}
	return visible, nil
}

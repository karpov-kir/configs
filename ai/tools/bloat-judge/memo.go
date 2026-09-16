// The record of what has already been judged.
//
// The model is not consistent: the same text drew two different verdicts on consecutive runs, and a
// pass over its own output deleted more. Idempotence therefore cannot come from the model, so it
// comes from here.
package bloatjudge

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Memo records each verdict by the hash of what was judged, and records the judged output as clean.
//
// The model is not consistent: the same text drew two different verdicts on consecutive runs, and a pass
// over its own output deleted more. Idempotence therefore cannot come from the model, so it comes from
// here: an artifact is judged once, its judged form is final, and a resend — or a second agent picking
// up the same text — meets the record rather than a new roll. Nil disables it, which the eval uses.
type Memo struct {
	Dir    string
	Policy string
}

// DefaultMemo lives outside every repo, under the cache home, so a repo never carries judged state.
func DefaultMemo(policy string) *Memo {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		base = filepath.Join(home, ".cache")
	}
	return &Memo{Dir: filepath.Join(base, "kk-flavor", "judged"), Policy: policy}
}

func (m *Memo) key(kind, content string) string {
	kindName, _, _ := strings.Cut(kind, "\n")
	specification := kinds[kindName]
	// Bump the algorithm version when unit extraction or majority semantics change, either of which
	// can move a verdict over identical bytes.
	identity := "judge-v4\n" + m.Policy + "\n" + Prompt(specification) + "\n" + strconv.FormatBool(specification.Source)
	sum := sha256.Sum256([]byte(identity + "\n" + kind + "\n" + content))
	return filepath.Join(m.Dir, hex.EncodeToString(sum[:]))
}

// lookup answers a recorded verdict, bounded by the units this run is offering. A record naming a
// unit outside them was written by different code over the same bytes, so it is a miss and the model
// is asked again — unbounded it would index past the units and take the process down.
func (m *Memo) lookup(kind, content string, count int) ([]int, bool) {
	if m == nil {
		return nil, false
	}
	raw, err := os.ReadFile(m.key(kind, content))
	if err != nil {
		return nil, false
	}
	gone, err := ParseVerdict(string(raw), count)
	if err != nil {
		return nil, false
	}
	return gone, true
}

// record writes a verdict. A failure to write is not a failure to judge: the verdict stands, and the
// next run merely pays the model again.
func (m *Memo) record(kind, content string, gone []int) {
	if m == nil {
		return
	}
	// 0700/0600: a file's name here is the sha256 of the text that was judged, so a readable memo
	// dir confirms a guess at the exact bytes of a report or PR body this machine judged.
	if err := os.MkdirAll(m.Dir, 0o700); err != nil {
		return
	}
	fields := make([]string, len(gone))
	for i, n := range gone {
		fields[i] = strconv.Itoa(n)
	}
	body := "none"
	if len(fields) > 0 {
		body = strings.Join(fields, ",")
	}
	_ = os.WriteFile(m.key(kind, content), []byte(body+"\n"), 0o600)
}

package voicecheck

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"sort"
	"strings"
)

// A kept block is read again when the checks change. The archive records the version each block
// passed. The version is a hash of the checks' own source, so an edit to a check moves it.

//go:embed *.go
var sources embed.FS

// Version is a hash of this package's source, its tests left out.
func Version() string {
	entries, _ := sources.ReadDir(".")
	var names []string
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), "_test.go") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	sum := sha256.New()
	for _, name := range names {
		body, _ := sources.ReadFile(name)
		sum.Write([]byte(name))
		sum.Write(body)
	}
	return hex.EncodeToString(sum.Sum(nil))[:12]
}

// KeepFindings lists the checks standing now that fail a kept block. They are its record check and the
// file bound in file order: the third and later block with one tie opening reopens, and the first two
// stay. A block with an empty record holds a summary only, and it has no record to read.
func KeepFindings(file, record string, block, span, fileLines []string, blockLine int) []string {
	var checks []string
	if strings.TrimSpace(record) != "" {
		input := append(append(strings.Split(strings.TrimSpace(record), "\n"), recordMarker), block...)
		for _, f := range RecordFindings(file, append(input, span...)) {
			checks = append(checks, f.Check)
		}
	}
	stripped := make([]string, len(block))
	for i, line := range block {
		stripped[i] = commentLead.ReplaceAllString(line, "")
	}
	for _, f := range tieFindingsAbove(file, stripped, fileLines, blockLine) {
		checks = append(checks, f.Check)
	}
	return checks
}

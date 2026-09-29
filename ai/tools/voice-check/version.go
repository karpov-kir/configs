package voicecheck

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"sort"
	"strings"
)

// A kept block is read again only where the checks changed since it last passed. The archive records
// the version it passed, and the version is the checks' own source, so an edit to any check moves it
// and an unchanged tool reads no block twice.

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

// KeepFindings is what the checks standing now report against a kept block: its record check, and the
// file bound counted in file order, so the third and later block with one tie opening reopens and the
// two above it stay. A block with an empty record holds a summary alone and has no record to read.
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

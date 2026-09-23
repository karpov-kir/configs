package voicecheck

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
)

// A compound in a comment whose camelCase join the code spells is the code's coined word, and the
// rename lane owns it. It is the code's own name where the tree spells it hyphenated: a file called
// mcp-sync.sh, a directory called reader-judge, a flag written `--dry-run`.
//
// A hand list per repository named those once. Kirill ruled on 2026-09-23 that it goes.

// reHyphenName is a hyphenated compound as a path or a string spells it.
var reHyphenName = regexp.MustCompile(`[a-z0-9]+(?:-[a-z0-9]+)+`)

// maxDerivedFileBytes bounds one file the derivation reads.
const maxDerivedFileBytes = 1 << 20

// DerivedNames is every hyphenated compound the repository at root spells in a tracked path or on a
// line of code, lower-cased. Comment lines are left out, because a coined word is learnt from a
// comment. It is cached under cacheHome by the tree it read.
func DerivedNames(root string, git repo.Git, cacheHome string) map[string]bool {
	files, err := git.Tracked(root)
	if err != nil || len(files) == 0 {
		return nil
	}
	key := sha256.New()
	for _, rel := range files {
		info, err := os.Stat(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		key.Write([]byte(rel))
		key.Write([]byte(info.ModTime().UTC().Format("20060102150405")))
	}
	cache := ""
	if cacheHome != "" {
		cache = filepath.Join(cacheHome, "kk-flavor", "derived-names", hex.EncodeToString(key.Sum(nil))[:16]+".txt")
		if body, err := os.ReadFile(cache); err == nil {
			out := map[string]bool{}
			for _, name := range shell.SplitLines(string(body)) {
				if name != "" {
					out[name] = true
				}
			}
			return out
		}
	}
	out := map[string]bool{}
	for _, rel := range files {
		for _, segment := range strings.Split(filepath.ToSlash(rel), "/") {
			base := strings.ToLower(strings.TrimSuffix(segment, filepath.Ext(segment)))
			for _, name := range reHyphenName.FindAllString(base, -1) {
				out[name] = true
			}
		}
		info, err := os.Lstat(filepath.Join(root, rel))
		if err != nil || !info.Mode().IsRegular() || info.Size() > maxDerivedFileBytes {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil || strings.IndexByte(string(body), 0) >= 0 {
			continue
		}
		for _, line := range shell.SplitLines(string(body)) {
			if isComment(strings.TrimLeft(line, " \t")) {
				continue
			}
			for _, name := range reHyphenName.FindAllString(strings.ToLower(line), -1) {
				out[name] = true
			}
		}
	}
	if cache != "" {
		names := make([]string, 0, len(out))
		for name := range out {
			names = append(names, name)
		}
		sort.Strings(names)
		if os.MkdirAll(filepath.Dir(cache), 0o755) == nil {
			_ = os.WriteFile(cache, []byte(strings.Join(names, "\n")+"\n"), 0o644)
		}
	}
	return out
}

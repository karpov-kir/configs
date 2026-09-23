package voicecheck

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"configs/ai/tools/shell"
)

// The names a reader places without being told: the ones the language resolves at the site. A hand
// list of them reached 10 names and missed the rest. Of 89 bare-identifier findings on a reviewed
// set of 60 files, 51 named a web API, a compiler flag, a test matcher or a specification attribute.
// The vocabulary is read from the repository's own type environment instead: the tsconfig `lib`
// declarations, the `@types` packages, and the test framework's matchers. A name from this
// repository declared in another file is in none of them, and stays a finding.

// maxDeclarationBytes bounds one declaration file. TypeScript's largest lib file is under 1 MiB.
const maxDeclarationBytes = 4 << 20

// defaultLibs is what TypeScript reads with no `lib` set, for the targets this repository's users run.
var defaultLibs = []string{"es2020", "dom"}

var (
	reLibReference  = regexp.MustCompile(`/// <reference lib="([^"]+)"`)
	reDeclaredName  = regexp.MustCompile(`\b(?:interface|class|type|enum|namespace|module|function|var|let|const)\s+([A-Za-z_$][\w$]*)`)
	reMemberName    = regexp.MustCompile(`(?m)^\s*(?:readonly\s+|static\s+|get\s+|set\s+)*([A-Za-z_$][\w$]*)\??\s*[:(<]`)
	reJSONComment   = regexp.MustCompile(`(?m)//[^\n]*$|/\*(?s:.*?)\*/`)
	reTrailingComma = regexp.MustCompile(`,(\s*[}\]])`)
)

// tsconfigLibs is the `lib` list a repository's tsconfig.json names, or the default where it names
// none. A tsconfig carries comments and trailing commas that JSON refuses, so both go first.
func tsconfigLibs(root string) []string {
	raw, err := os.ReadFile(filepath.Join(root, "tsconfig.json"))
	if err != nil {
		return defaultLibs
	}
	clean := reTrailingComma.ReplaceAll(reJSONComment.ReplaceAll(raw, nil), []byte("$1"))
	var config struct {
		CompilerOptions struct {
			Lib []string `json:"lib"`
		} `json:"compilerOptions"`
	}
	if json.Unmarshal(clean, &config) != nil || len(config.CompilerOptions.Lib) == 0 {
		return defaultLibs
	}
	return config.CompilerOptions.Lib
}

// declarationFiles is every file the vocabulary reads: each lib file the tsconfig names and the lib
// files they reference, then every declaration file under `@types` and the `expect` package.
func declarationFiles(root string) []string {
	var out []string
	libDir := filepath.Join(root, "node_modules", "typescript", "lib")
	seen := map[string]bool{}
	queue := tsconfigLibs(root)
	for len(queue) > 0 {
		lib := strings.ToLower(queue[0])
		queue = queue[1:]
		if seen[lib] {
			continue
		}
		seen[lib] = true
		path := filepath.Join(libDir, "lib."+lib+".d.ts")
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		out = append(out, path)
		for _, m := range reLibReference.FindAllSubmatch(body, -1) {
			queue = append(queue, string(m[1]))
		}
	}
	for _, dir := range []string{filepath.Join(root, "node_modules", "@types"), filepath.Join(root, "node_modules", "expect")} {
		_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".d.ts") {
				out = append(out, path)
			}
			return nil
		})
	}
	sort.Strings(out)
	return out
}

// declaredNames reads the names a declaration file declares, its members included: an interface's
// `canPlayType` is placed the same as the interface.
func declaredNames(body string, into map[string]bool) {
	for _, re := range []*regexp.Regexp{reDeclaredName, reMemberName} {
		for _, m := range re.FindAllStringSubmatch(body, -1) {
			into[strings.ToLower(m[1])] = true
		}
	}
}

// DerivedVocabulary is the names the repository at root resolves from outside its own source, lower-
// cased. It reads the files once and keeps the result under the cache, keyed by the files it read and
// their sizes, so a new dependency or a new tsconfig gives a new key.
func DerivedVocabulary(root, cacheHome string) map[string]bool {
	files := declarationFiles(root)
	if len(files) == 0 {
		return nil
	}
	key := sha256.New()
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		key.Write([]byte(path))
		key.Write([]byte{byte(info.Size()), byte(info.Size() >> 8), byte(info.Size() >> 16), byte(info.Size() >> 24)})
	}
	cache := ""
	if cacheHome != "" {
		cache = filepath.Join(cacheHome, "kk-flavor", "vocabulary", hex.EncodeToString(key.Sum(nil))[:16]+".txt")
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
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil || info.Size() > maxDeclarationBytes {
			continue
		}
		if body, err := os.ReadFile(path); err == nil {
			declaredNames(string(body), out)
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

// cacheHomeFrom is where the vocabulary is kept, following XDG with the home directory's own cache
// as the fallback. Empty means no cache, and the vocabulary is read fresh each run.
func cacheHomeFrom(lookup func(string) (string, bool)) string {
	if dir, ok := lookup("XDG_CACHE_HOME"); ok && dir != "" {
		return dir
	}
	if home, ok := lookup("HOME"); ok && home != "" {
		return filepath.Join(home, ".cache")
	}
	return ""
}

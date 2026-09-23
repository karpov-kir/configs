package voicecheck

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The vocabulary reads the libs the tsconfig names and the libs they reference, the @types packages
// and the test matchers, members included. A name the repository declares itself is in none of them.
func TestTheVocabularyIsTheRepositorysTypeEnvironment(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "tsconfig.json"), "{\n  // comment\n  \"compilerOptions\": { \"lib\": [\"DOM\",], },\n}\n")
	lib := filepath.Join(root, "node_modules", "typescript", "lib")
	writeFile(t, filepath.Join(lib, "lib.dom.d.ts"), "/// <reference lib=\"es5\" />\ninterface HTMLMediaElement {\n  canPlayType(type: string): string;\n}\n")
	writeFile(t, filepath.Join(lib, "lib.es5.d.ts"), "interface ArrayConstructor {\n  isArray(arg: any): boolean;\n}\n")
	writeFile(t, filepath.Join(lib, "lib.webworker.d.ts"), "interface ServiceWorkerGlobal {\n  skipWaiting(): void;\n}\n")
	writeFile(t, filepath.Join(root, "node_modules", "@types", "jest", "index.d.ts"), "declare namespace jest {\n  interface Matchers<R> {\n    toHaveBeenCalledWith(...args: any[]): R;\n  }\n}\n")
	cache := t.TempDir()
	got := DerivedVocabulary(root, cache)
	for _, name := range []string{"htmlmediaelement", "canplaytype", "isarray", "tohavebeencalledwith", "matchers"} {
		if !got[name] {
			t.Errorf("%s is not in the vocabulary", name)
		}
	}
	if got["skipwaiting"] {
		t.Error("a lib the tsconfig never names is in the vocabulary")
	}
	// The second read comes from the cache, and it holds the same names.
	if again := DerivedVocabulary(root, cache); len(again) != len(got) {
		t.Fatalf("the cached vocabulary holds %d names, the first read %d", len(again), len(got))
	}
}

func TestARepositoryWithNoTypeEnvironmentPlacesNothing(t *testing.T) {
	if got := DerivedVocabulary(t.TempDir(), ""); len(got) != 0 {
		t.Fatalf("an empty repository places %d names", len(got))
	}
}

// A name the vocabulary holds is placed. Its neighbour, declared by the repository, stays a finding.
func TestTheBareIdentifierCheckPlacesAVocabularyName(t *testing.T) {
	s := voiceScanner()
	s.vocabulary = map[string]bool{"canplaytype": true}
	lines := []string{"// The platform answers through canPlayType and mediaSourceClaim.", "const x = 1;"}
	var bare []string
	for _, f := range s.scanSource("x.ts", lines, nil, lines) {
		if f.Check == checkBareIdent {
			bare = append(bare, f.Text)
		}
	}
	if len(bare) != 1 || bare[0] != "mediaSourceClaim" {
		t.Fatalf("bare identifiers %v, want mediaSourceClaim alone", bare)
	}
}

package commentpass

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tree writes each file under a new root and returns the root.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		write(t, root, name, body)
	}
	return root
}

// TestTheWidthIsTheFormattersOwn checks that each file takes the width its own formatter prints,
// read where that formatter reads it.
func TestTheWidthIsTheFormattersOwn(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		path  string
		want  int
	}{
		{"oxfmt in a package below the root, its default", map[string]string{
			"web/package.json": `{"devDependencies": {"oxfmt": "^0.78.0"}}`}, "web/src/a.ts", oxfmtWidth},
		{"an oxfmt JSONC config", map[string]string{
			".oxfmtrc.jsonc": "{\n  // the house width\n  \"printWidth\": 120, /* wide */\n}\n"}, "src/a.tsx", 120},
		{"oxfmt reading .editorconfig", map[string]string{"package.json": `{"devDependencies": {"oxfmt": "1"}}`,
			".editorconfig": "root = true\n[*.{ts,tsx}]\nmax_line_length = 110\n"}, "a.ts", 110},
		{"a prettier config", map[string]string{".prettierrc": `{"printWidth": 90}`}, "a.js", 90},
		{"prettier listed, its default", map[string]string{"package.json": `{"devDependencies": {"prettier": "3"}}`},
			"a.js", prettierWidth},
		{"a script nothing formats", map[string]string{}, "a.ts", noWidth},
		{"Go, which gofmt never bounds", map[string]string{".prettierrc": `{"printWidth": 90}`}, "a.go", noWidth},
		{"Go under .editorconfig", map[string]string{
			".editorconfig": "[*]\nmax_line_length = 140\n[*.md]\nmax_line_length = off\n"}, "cmd/a.go", 140},
		{"Go where .editorconfig turns it off", map[string]string{
			".editorconfig": "[*]\nmax_line_length = 140\n[*.go]\nmax_line_length = off\n"}, "a.go", noWidth},
		{"Rust, rustfmt's default", map[string]string{}, "src/a.rs", rustfmtWidth},
		{"a rustfmt config", map[string]string{"rustfmt.toml": "edition = \"2021\"\nmax_width = 120 # wide\n"}, "src/a.rs", 120},
	} {
		root := tree(t, tc.files)
		if got := fileWidth(root, tc.path); got != tc.want {
			t.Errorf("%s: %s takes %d, want %d", tc.name, tc.path, got, tc.want)
		}
	}
}

// TestEditorconfigGlobs checks that EditorConfig globs match the way the format defines them.
func TestEditorconfigGlobs(t *testing.T) {
	for _, tc := range []struct {
		glob, path string
		want       bool
	}{
		{"*.go", "a/b/c.go", true},
		{"*.{ts,tsx}", "src/a.tsx", true},
		{"*.{ts,tsx}", "src/a.js", false},
		{"src/*.ts", "src/a.ts", true},
		{"src/*.ts", "src/b/a.ts", false},
		{"src/**.ts", "src/b/a.ts", true},
		{"file{1..3}.go", "file2.go", true},
		{"file{1..3}.go", "file4.go", false},
		{"[!a]*.go", "b.go", true},
		{"[!a]*.go", "a.go", false},
	} {
		if got := editorconfigMatch(tc.glob, tc.path); got != tc.want {
			t.Errorf("%s on %s: %v, want %v", tc.glob, tc.path, got, tc.want)
		}
	}
}

// TestAFileNothingBoundsWrapsNothing checks that a file nothing bounds keeps a long line as written,
// and that the gate passes it.
func TestAFileNothingBoundsWrapsNothing(t *testing.T) {
	long := "// " + strings.TrimSpace(strings.Repeat("posts the entry ", 14))
	m := material{candidates: []candidate{{id: "c1"}}, indents: map[string]int{"c1": 0}}
	decisions := rewrap(m, []decision{{id: "c1", verb: "rewrite", text: []string{long}}}, noWidth)
	if len(decisions[0].text) != 1 || len(gateFindings(m, decisions, noWidth)) != 0 {
		t.Fatalf("got %q", decisions[0].text)
	}
	if prompt := userPrompt("a.go", []string{"// x", "func A() {}"}, findMaterial("a.go", []string{"// x", "func A() {}"},
		map[int]bool{2: true}), nil, noWidth, ""); !strings.Contains(prompt, "(no column bound") ||
		!strings.Contains(prompt, "formatter sets no width") {
		t.Fatalf("prompt:\n%s", prompt)
	}
}

// TestFilesInOneDirectoryTakeTheirOwnWidths checks that two files in one directory each take their own
// width where an .editorconfig section names one of them.
func TestFilesInOneDirectoryTakeTheirOwnWidths(t *testing.T) {
	root := tree(t, map[string]string{
		".editorconfig": "root = true\n[*.go]\nmax_line_length = 100\n[*_test.go]\nmax_line_length = 140\n"})
	if a, b := fileWidth(root, "pkg/a.go"), fileWidth(root, "pkg/a_test.go"); a != 100 || b != 140 {
		t.Fatalf("a.go takes %d and a_test.go %d, want 100 and 140", a, b)
	}
}

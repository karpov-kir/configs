package voicecheck

import (
	"os"
	"path/filepath"
	"testing"
)

// The width comes from the repository's own prettier config, following one `require` into the shared
// config it extends, and 120 where no config sets one.
func TestTheWidthIsTheRepositorysPrettierWidth(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  int
	}{
		{"no config", nil, 120},
		{"a json rc", map[string]string{".prettierrc": `{"printWidth": 100}`}, 100},
		{"package.json", map[string]string{"package.json": `{"prettier": {"printWidth": 90}}`}, 90},
		{"a shared config one require away", map[string]string{
			"prettier.config.js":                          "const config = require('@team/style/prettier.config');\nmodule.exports = { ...config };\n",
			"node_modules/@team/style/prettier.config.js": "module.exports = {\n  printWidth: 110,\n};\n",
		}, 110},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for rel, body := range tc.files {
				path := filepath.Join(root, rel)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if got := prettierWidth(root); got != tc.want {
				t.Fatalf("width %d, want %d", got, tc.want)
			}
		})
	}
}

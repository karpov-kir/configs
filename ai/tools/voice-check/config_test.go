package voicecheck

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"configs/ai/tools/flavorconfig"
)

// This case reads the file through flavorconfig. The shipped number equals the constant in code, and a
// case built on ConfigFromEnv passes whether or not it reads the file.
func TestTheShippedByteCapParsesAndMatchesTheBuiltInDefault(t *testing.T) {
	flavor, err := filepath.Abs("../../kk-flavor")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	if err := os.Symlink(flavor, filepath.Join(home, ".kk-flavor")); err != nil {
		t.Fatal(err)
	}
	settings, err := flavorconfig.Read(flavorconfig.Path(home, configName), []string{"max-file-bytes"})
	if err != nil {
		t.Fatalf("the shipped voice-check.conf does not parse: %v", err)
	}
	if settings["max-file-bytes"] != fmt.Sprint(defaultMaxFileBytes) {
		t.Fatalf("the shipped config holds %v and the built-in default is %d — a run that cannot reach the configs would scan against a different cap",
			settings, defaultMaxFileBytes)
	}
}

// The shipped value applies and the environment wins over it. A shipped value that does not parse
// refuses the run, and the constant in code stays out of it.
func TestTheEnvironmentWinsOverTheShippedByteCapAndABrokenOneRefuses(t *testing.T) {
	home := t.TempDir()
	configs := filepath.Join(home, ".kk-flavor", "configs")
	if err := os.MkdirAll(configs, 0o700); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(configs, configName)
	if err := os.WriteFile(conf, []byte("max-file-bytes 64\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := func(pairs ...string) func(string) (string, bool) {
		set := map[string]string{"HOME": home}
		for i := 0; i+1 < len(pairs); i += 2 {
			set[pairs[i]] = pairs[i+1]
		}
		return func(key string) (string, bool) { v, ok := set[key]; return v, ok }
	}
	if cfg, err := ConfigFromEnv(env()); err != nil || cfg.MaxFileBytes != 64 {
		t.Fatalf("got %+v %v, want the shipped 64", cfg, err)
	}
	if cfg, err := ConfigFromEnv(env("DENSITY_MAX_FILE_BYTES", "32")); err != nil || cfg.MaxFileBytes != 32 {
		t.Fatalf("got %+v %v, want the environment's 32", cfg, err)
	}
	if err := os.WriteFile(conf, []byte("max-file-bytes big\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ConfigFromEnv(env()); err == nil {
		t.Fatal("a shipped cap that is no number was accepted")
	}
}

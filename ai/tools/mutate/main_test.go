package main

import (
	"strings"
	"testing"
)

// This tool executes all three of its inputs — the harness through `bash -c`, the target through sed,
// the suite directly. Handing them os.Environ() hands a reviewed branch's code every token, key and
// session variable the caller happened to export.
func TestChildEnvCarriesOnlyWhatAChildNeeds(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "not a reviewed branch's to read")

	env := childEnv("HOME=/tmp/sandbox-home")

	carried := map[string]string{}
	for _, entry := range env {
		name, value, _ := strings.Cut(entry, "=")
		carried[name] = value
	}
	for name := range carried {
		switch name {
		case "LC_ALL", "PATH", "TMPDIR", "HOME":
		default:
			t.Errorf("%s reached a child of this tool", name)
		}
	}
	// The control: an allow-list that carried nothing would satisfy the loop above and break every run.
	if carried["PATH"] != "/usr/bin:/bin" {
		t.Errorf("PATH has to reach the child or nothing it runs is found: %q", carried["PATH"])
	}
	if carried["HOME"] != "/tmp/sandbox-home" {
		t.Errorf("the sandbox HOME is what makes the suite run against the sandbox: %q", carried["HOME"])
	}
}

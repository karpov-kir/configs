package ecocheck

import (
	"os"
	"os/exec"
)

// Where the parse scan reaches bash. Both questions are behind one port, since both are answered by
// forking on the machine the check runs on. A suite replacing only the second still parses every
// fixture once per binary that machine happens to carry, and the fork count is what the seam takes out.
type Bash interface {
	// Binaries names every bash a `#!/usr/bin/env bash` line could resolve to here. An empty answer is a
	// machine carrying none. scanScriptsParse reports that as a scan that failed to run, which keeps the
	// tree from reading as clean scripts.
	Binaries() []string

	// Parse answers what `bash -n` writes about one script, and empty for one that parses. Bash exits
	// non-zero for a script it refuses, and the port reads that exit as the finding it is.
	Parse(binary, script string) string
}

type InstalledBash struct{}

// Both are answered even when they resolve to the same file. The duplicate findings collapse in the
// sort, and dropping one would silently stop checking the older bash on a machine where PATH holds it.
// macOS still ships 3.2 as /bin/bash, and it rejects constructs bash 5 accepts.
func (InstalledBash) Binaries() []string {
	var found []string
	if path, err := exec.LookPath("bash"); err == nil {
		found = append(found, path)
	}
	if isExecutable("/bin/bash") {
		found = append(found, "/bin/bash")
	}
	return found
}

// A path opening with a dash is read as an option without `--`. `bash -n -d.sh` answers `-d: invalid
// option` and dumps its usage without opening the file, and each of those ~25 lines becomes a
// `syntax:` finding at rank 0. The script then goes unparsed while bash's help text floods the gravest
// rank.

// The root arrives as a literal argument, so the leading byte of every path built from it is the
// caller's to choose. LC_ALL=C because bash's own message is translated.
func (InstalledBash) Parse(binary, script string) string {
	command := exec.Command(binary, "-n", "--", script)
	command.Env = append(os.Environ(), "LC_ALL=C")
	output, _ := command.CombinedOutput()
	return string(output)
}

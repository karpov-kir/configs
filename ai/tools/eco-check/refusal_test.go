package ecocheck_test

// The stderr funnel every refusal leaves through, and the text a refusal echoes back out of the
// invocation it was handed. `check.sh` runs as kk-ecosystem's stage, so this output lands on the
// terminal an agent reads the verdict off — and the root is argv, which is text nobody in this
// package chose the bytes or the length of.

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	ecocheck "kk-flavor/tools/eco-check"
	"kk-flavor/tools/shell"
)

// Both refusals that echo text off the invocation: the root holding no checkout, and the root --gate
// could not ask git about. Each is asserted to have fired and to have echoed the name, because a
// refusal the hostile name never entered carries no control byte for a reason that has nothing to do
// with the guard.
//
// Run from inside the parent directory, so the whole root fits inside the per-part bound the gate
// refusal cuts its path at. Named absolutely, a tempdir path spends that bound before the name is
// reached and the second half of this case observes an empty message.
func TestARefusalCarriesNoControlBytesFromTheRootItEchoes(t *testing.T) {
	// The first erases the line it is printed on, which is how the fixed "nothing was checked" line
	// under a refusal can be made to stand where the refusal was — the run then reads as one that
	// merely named a bad flag.
	hostile := []string{"evil\x1b[2K\rALL CLEAR", "two\nlines", "bell\a", "csi\u009bm"}
	for i, name := range hostile {
		t.Run(fmt.Sprintf("hostile root %d", i), func(t *testing.T) {
			parent := t.TempDir()
			if err := os.MkdirAll(parent+"/real-"+name+"/kk-flavor/skills", 0o755); err != nil {
				t.Skipf("this filesystem refused %q in a directory name, so the case cannot run here: %v", name, err)
			}
			t.Chdir(parent)

			refusals := map[string]string{
				"no checkout is there":   refusalFrom(t, "--agent=claude", "./nope-"+name),
				"git could not be asked": refusalFrom(t, "--agent=claude", "--gate", "./real-"+name),
			}
			for what, output := range refusals {
				assertEchoesTheNameWithoutDrivingTheTerminal(t, what, name, output)
			}
		})
	}
}

func assertEchoesTheNameWithoutDrivingTheTerminal(t *testing.T, what, name, output string) {
	t.Helper()
	if !strings.Contains(output, shell.Oneline(name)) {
		t.Errorf("%s: the refusal never echoed the name, so this case observes nothing\n%s", what, indent(output))
		return
	}
	// The newline between the two lines of the refusal is this tool's own and stays.
	for _, b := range []byte(output) {
		if b < 0x20 && b != '\n' || b == 0x7f {
			t.Errorf("%s: the refusal carries byte %#x, which drives the terminal rather than printing\n%s",
				what, b, indent(output))
			return
		}
	}
}

// Nothing bounds a root's length: it is argv, it is never opened, and this machine's ARG_MAX runs to
// a megabyte. Uncut, that megabyte is what the refusal puts on the terminal — while every finding
// beside it is held to lineWidthCap, which is the bound a reader of this output already assumes.
func TestARefusalIsBoundedHoweverLongTheRootItEchoes(t *testing.T) {
	t.Parallel()
	output := refusalFrom(t, "--agent=claude", "/nowhere/"+strings.Repeat("a", 4000))

	if !strings.Contains(output, "no root holding both") {
		t.Fatalf("the no-root refusal never fired, so this case observes nothing\n%s", indent(output))
	}
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if len(line) > ecocheck.LineWidthCap {
			t.Errorf("a refusal line is %d bytes, over the %d every finding is held to\n%s",
				len(line), ecocheck.LineWidthCap, indent(line))
		}
	}
	if !strings.Contains(output, shell.CutMarker) {
		t.Errorf("nothing was cut, so the bound this case is about was never reached\n%s", indent(output))
	}
}

// The --gate refusal names the root first and git's own reason last, so the bound on the root decides
// whether that reason survives the line bound above. A root here is a real directory rather than argv,
// so PATH_MAX bounds it — but PATH_MAX is twice the width a refusal prints at, which is the whole gap.
func TestTheGateRefusalStillNamesGitsReasonUnderALongRoot(t *testing.T) {
	t.Parallel()
	// Grown until the root alone would spend the line, whatever this machine's temp path costs. A
	// fixed number of components leaves the case observing nothing where TMPDIR is short.
	deep := t.TempDir()
	for len(deep) <= ecocheck.LineWidthCap {
		deep += "/" + strings.Repeat("d", 150)
	}
	if err := os.MkdirAll(deep+"/kk-flavor/skills", 0o755); err != nil {
		t.Skipf("this filesystem refused a %d-byte path, so the case cannot run here: %v", len(deep), err)
	}

	output := refusalFrom(t, "--agent=claude", "--gate", deep)
	line := lineWith(output, "git check-ignore could not answer")
	if line == "" {
		t.Fatalf("the --gate refusal never fired, so this case observes nothing\n%s", indent(output))
	}
	if _, reason, found := strings.Cut(line, "no scan ran: "); !found ||
		strings.TrimSuffix(reason, shell.CutMarker) == "" {
		t.Errorf("the root spent the line and git's own reason went with it, so the refusal names what "+
			"could not answer and never why\n%s", indent(output))
	}
}

// One refusing run, and everything it wrote. Exit 2 is asserted here rather than left to each case: a
// run that reached the scans wrote no refusal at all, and every assertion over its output would then
// hold for a reason the case is not about.
func refusalFrom(t *testing.T, args ...string) string {
	t.Helper()
	var output bytes.Buffer
	if status := ecocheck.Run(args, &output, &output); status != 2 {
		t.Fatalf("Run %q exited %d, so it wrote no refusal\n%s", args, status, indent(output.String()))
	}
	return output.String()
}

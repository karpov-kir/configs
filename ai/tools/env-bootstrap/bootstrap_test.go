package envbootstrap_test

// The cases for the shell and editor install. ai/tools/installer's own suite drives what the mounting
// machinery does with a table: refusing a real file, repointing a stale link, stopping at a second
// checkout. This suite drives the decisions this installer makes: the table it declares, the flags it
// accepts, and the packages step.

import (
	"testing"

	envbootstrap "configs/ai/tools/env-bootstrap"
	"configs/ai/tools/installertest"
	"configs/ai/tools/runtest"
)

// What the guard looks for under a candidate root, and what a refusal leads with. The stub's own
// basename, which is what `exec -a "$0"` hands the binary.
const scriptName = "bootstrap.sh"

// One case's tree: a checkout in env/'s shape, a home, and everything the run printed.
type fixture struct {
	*installertest.Tree
	*runtest.Output
	t    *testing.T
	base string
	repo string
	home string
	brew *installertest.BrewMachine
}

// Every case gets a home of its own under t.TempDir(). The suite this replaced shared one home, where
// a fixture write followed a live symlink into the checkout and overwrote nvim/init.lua and
// starship/starship.toml in the working tree.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	tree := installertest.New(t)
	base := tree.Base()
	f := &fixture{Tree: tree, Output: runtest.NewOutput(t), t: t, base: base,
		repo: base + "/checkout/env", home: base + "/home", brew: installertest.NewBrewMachine()}
	f.MkdirAll(f.home)
	f.newCheckout(f.repo)
	return f
}

// A checkout in the shape the second-checkout guard recognises: the installer under test at the root,
// and the sources beside it. A fixture missing the installer is turned away by the guard's last
// condition, and the case then measures that condition.
func (f *fixture) newCheckout(root string) {
	f.t.Helper()
	f.Write(root+"/"+scriptName, "#!/usr/bin/env bash\n")
	f.Write(root+"/zsh/.zpreztorc", "the prezto config\n")
	f.Write(root+"/zsh/.zshrc", "the shell config\n")
	f.Write(root+"/git/.gitconfig", "the git identity\n")
	f.Write(root+"/ghostty/config", "the terminal\n")
	f.Write(root+"/nvim/init.lua", "the editor config\n")
	f.Write(root+"/starship/starship.toml", "the prompt\n")
}

// The run a case drives, with everything it did not name taken from the fixture. Out and Err are the
// fixture's, so every assertion reads the same output. WriteRoot bounds every write to this case's own
// tree, and Breaches is read back afterwards, so a run that reached outside the tree fails as a guard.
func (f *fixture) run(args ...string) int {
	f.t.Helper()
	f.Reset()
	run, code := envbootstrap.Perform(envbootstrap.Options{
		Self:       scriptName,
		Args:       args,
		Repo:       f.repo,
		Home:       f.home,
		ConfigHome: f.home + "/.config",
		Machine:    f.brew,
		Out:        &f.Out,
		Err:        &f.Err,
		WriteRoot:  f.base,
	})
	if run != nil {
		f.ExpectNoBreach(run.Breaches())
	}
	return code
}

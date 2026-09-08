#!/usr/bin/env bash
set -u
here=$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
checkout=$(CDPATH= cd -P -- "$here/.." && pwd -P)
suite_name="ai/project-dependencies-test.sh"
. "$checkout/lib/test-harness.sh" || exit 2

newCase() {
  fresh_home
  mkdir -p "$home/bin"
  fixture_write "$home/mise-source" '#!/bin/bash
printf "%s\n" "$*" >>"$HOME/mise-calls"
[ "$*" = --version ] || exit 9
[ ! -e "$HOME/broken-mise" ] || exit 1
printf "mise 2026.9.2\n"'
}

addMise() {
  cp "$home/mise-source" "$home/bin/mise"
  chmod +x "$home/bin/mise"
}

addBrew() {
  fixture_write "$home/bin/brew" '#!/bin/bash
printf "%s:%s\n" "${HOMEBREW_NO_AUTO_UPDATE:-}" "$*" >>"$HOME/brew-calls"
[ "$*" = "install mise" ] || exit 9
[ ! -e "$HOME/brew-fails" ] || exit 1
[ ! -e "$HOME/brew-no-install" ] || exit 0
/bin/cp "$HOME/mise-source" "$HOME/bin/mise"
/bin/chmod +x "$HOME/bin/mise"'
  chmod +x "$home/bin/brew"
}

runDependencies() {
  out=$(HOME="$home" PATH="$home/bin" /bin/bash "$here/project-dependencies.sh" "$@" 2>&1)
  status=$?
}

newCase
addMise
addBrew
runDependencies
expect_status "working mise satisfies project prerequisites" 0
expect_file_body "existing mise only receives the version check" "$home/mise-calls" --version
expect_absent "working mise never invokes brew" "$home/brew-calls"

newCase
addBrew
runDependencies --dry-run
expect_status "dry run plans a mise installation" 0
expect_out "dry run names the bounded install command" 'HOMEBREW_NO_AUTO_UPDATE=1 brew install mise'
expect_absent "dry run does not invoke brew" "$home/brew-calls"
expect_absent "dry run does not install mise" "$home/bin/mise"
runDependencies
expect_status "existing brew installs the project prerequisite" 0
expect_file_body "brew installs only mise without automatic updates" "$home/brew-calls" '1:install mise'
expect_file_body "new mise is verified" "$home/mise-calls" --version
runDependencies
expect_status "repeated setup reuses installed mise" 0
expect_file_body "repeated setup does not reinstall mise" "$home/brew-calls" '1:install mise'

newCase
runDependencies
expect_status "missing brew fails without machine bootstrap" 1
expect_out "missing brew provides official installation directions" 'https://mise.jdx.dev/installing-mise.html'
expect_out "missing brew explains the PATH requirement" 'PATH'
runDependencies --dry-run
expect_status "dry run reports an unavailable installation path" 1

newCase
addMise
addBrew
touch "$home/broken-mise"
runDependencies
expect_status "broken existing mise fails readiness" 1
expect_out "broken existing mise is identified" 'mise --version failed'
expect_absent "broken existing mise does not trigger machine changes" "$home/brew-calls"

newCase
addBrew
touch "$home/brew-fails"
runDependencies
expect_status "brew installation failure propagates" 1
expect_out "brew failure identifies the failed install" 'brew install mise failed'

newCase
addBrew
touch "$home/brew-no-install"
runDependencies
expect_status "brew success without usable mise fails verification" 1
expect_out "unavailable installed mise gives PATH recovery" 'PATH'

newCase
addBrew
touch "$home/broken-mise"
runDependencies
expect_status "newly installed broken mise fails verification" 1
expect_out "installed mise version failure is identified" 'mise --version failed'

newCase
addBrew
runDependencies --unknown
expect_status "unknown options fail before installation" 2
expect_absent "invalid invocation never invokes brew" "$home/brew-calls"

report_suite

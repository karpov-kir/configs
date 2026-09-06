#!/usr/bin/env bash
#
# The fixtures and assertions the two bootstrap suites share. Sourced, never executed — the filename
# ends in `-harness.sh` rather than `-test.sh` so run-tests.sh's discovery does not try to run it as a
# suite of its own.
#
# Before sourcing, a caller sets `suite_name` (what a containment refusal reports itself as) and
# `checkout` (this repository's root, which the fixture checkouts are copied out of). After sourcing
# it has $tmp, $tmp_real, a fresh home per case, the fixture writers, and the expectations.
#
# Every fixture write goes through `fixture_write` or `fixture_link`, and this is why.
#
# The cases run the *real* bootstrap scripts, which derive their repository from their own location,
# so a run leaves `$home/.config/nvim` pointing at this checkout's `env/nvim`. That is correct
# behaviour and harmless while each case gets its own home. It stops being harmless the moment two
# cases share one: the second case's `mkdir -p "$home/.config/nvim"` then finds a live symlink into
# the checkout and succeeds, and the fixture write that follows goes straight through it into a real
# config file.
#
# That is not hypothetical. An earlier `fresh_home` was called as `home=$(fresh_home)`, which
# incremented its counter inside a subshell and handed every case the same home. It overwrote
# `nvim/init.lua` and `starship/starship.toml` in the working tree and left a stray symlink in
# `nvim/`. The suite reported it, too — a case failed saying something had been written where nothing
# should be — and the report was read as a harness bug without asking what the broken run had already
# written to disk.
#
# So the containment is asserted before each write rather than noticed after: the parent is resolved
# physically, following any symlink in the path, and anything landing outside `$tmp` aborts the whole
# suite. A guard that reports afterwards has still lost the file.

tmp=$(mktemp -d) || exit 1
trap 'rm -rf "$tmp"' EXIT

# Resolved physically once, because `mktemp -d` hands back `/var/folders/…` on macOS while `/var` is
# itself a symlink to `/private/var`. Comparing an unresolved root against resolved paths would make
# the containment check below refuse everything, and a guard that always fires gets deleted.
tmp_real=$(cd -P "$tmp" && pwd -P) || exit 1

refuse_fixture() {
  printf '%s: refusing to write %s — %s\n' "$suite_name" "$1" "$2" >&2
  printf '  the suite may only write under %s; this is the containment guard, not a failing case\n' "$tmp_real" >&2
  exit 2
}

contained_parent() {
  local path="$1" parent
  parent=$(cd -P "$(dirname -- "$path")" 2>/dev/null && pwd -P) ||
    refuse_fixture "$path" "its parent directory does not resolve"
  case "$parent/" in
    "$tmp_real"/*) printf '%s' "$parent" ;;
    *) refuse_fixture "$path" "its parent resolves to $parent" ;;
  esac
}

fixture_write() {
  local path="$1" body="$2"
  contained_parent "$path" >/dev/null
  # The parent being contained says nothing about the last component. `>` follows a symlink, and the
  # links these cases produce point into this checkout — a bootstrap run leaves `$home/.zshrc ->
  # $checkout/env/zsh/.zshrc`, and a fixture write at that path afterwards lands in the real file.
  # That is the incident in the header, reached by the one door the parent check does not cover.
  # Refused rather than followed, the same way fixture_link refuses one: no case here means to write
  # through a link.
  [ ! -L "$path" ] ||
    refuse_fixture "$path" "it already exists as a symlink to $(readlink "$path")"
  printf '%s\n' "$body" >|"$path"
}

fixture_link() {
  local source="$1" target="$2"
  contained_parent "$target" >/dev/null
  # `ln -s X Y` where Y already exists as a symlink to a directory creates the link *inside* Y rather
  # than replacing it, which is how a stray link ends up in the checkout. Refused rather than forced:
  # every fixture link in these suites is meant to be the first thing at its path.
  [ ! -L "$target" ] ||
    refuse_fixture "$target" "it already exists as a symlink to $(readlink "$target")"
  ln -s "$source" "$target"
}

# A second copy of this repository, in the shape the guard recognises: the bootstrap script under
# test at the same relative depth, and beside it the mount library it sources. A fixture missing the
# library would fail at source time rather than reaching the guard, and the case would be reporting a
# broken fixture rather than a refusal.
fixture_checkout() { # <root> <side>  e.g. fixture_checkout "$tmp/other-repo" env
  local root="$1" side="$2"
  contained_parent "$root" >/dev/null
  mkdir -p "$root/lib" "$root/$side"
  cp "$checkout/lib/mount.sh" "$root/lib/mount.sh"
  cp "$checkout/$side/bootstrap.sh" "$root/$side/bootstrap.sh"
}

passed=0
failed=0

record_pass() {
  passed=$((passed + 1))
  echo "  pass  $1"
}

record_fail() {
  failed=$((failed + 1))
  echo "  FAIL  $1  — $2"
}

# No skip counter here, unlike ai/tools/source-stamp-test.sh. That suite reports a third field because
# it has a case it cannot run on a machine carrying only one of the two hashers, and hiding that would
# make two machines checking different sets look identical. Nothing here is declined that way: the one
# case these files cannot run as root takes the whole suite to exit 2 instead, which says the run is
# not a result rather than that a case was skipped. A field that can never be non-zero is decoration
# wearing the shape of a measurement.

# A fresh home per case, so no case inherits another's links. Numbered rather than mktemp'd again so a
# failure message names which case's home to go and look at.
#
# It sets `home` rather than printing it: read back through `home=$(fresh_home)` the counter would be
# incremented inside a subshell, every case would be handed `home1`, and each would inherit the first
# case's links. What that cost once is in the containment note above.
case_no=0
fresh_home() {
  case_no=$((case_no + 1))
  home="$tmp/home$case_no"
  mkdir -p "$home"
}

expect_status() {
  local name="$1" want="$2"
  [ "$status" -eq "$want" ] &&
    record_pass "$name" ||
    record_fail "$name" "exit $status, wanted $want"
}

expect_out() {
  local name="$1" want="$2"
  case "$out" in
    *"$want"*) record_pass "$name" ;;
    *) record_fail "$name" "wanted '$want' in: $out" ;;
  esac
}

expect_not_out() {
  local name="$1" unwanted="$2"
  case "$out" in
    *"$unwanted"*) record_fail "$name" "found '$unwanted' in: $out" ;;
    *) record_pass "$name" ;;
  esac
}

expect_link_to() {
  local name="$1" target="$2" want="$3"
  if [ ! -L "$target" ]; then
    record_fail "$name" "$target is not a symlink"
    return
  fi
  [ "$(readlink "$target")" = "$want" ] &&
    record_pass "$name" ||
    record_fail "$name" "$target -> $(readlink "$target"), wanted $want"
}

expect_file_body() {
  local name="$1" path="$2" want="$3" got
  got=$(cat "$path" 2>/dev/null)
  [ "$got" = "$want" ] &&
    record_pass "$name" ||
    record_fail "$name" "$path holds '$got', wanted '$want'"
}

# The brew list a bootstrap hardcodes and the one its README documents cannot drift apart: adding a
# formula to the README would silently stop it being installed, with every case still passing. Read
# from the real files rather than a fixture, so the shipped ones cannot drift to a form this never
# sees.
expect_brew_matches_readme() { # <script> <readme>
  local script="$1" readme="$2" boot_formulae boot_casks readme_formulae readme_casks
  boot_formulae="$(sed -n 's/^  for formula in \(.*\); do$/\1/p' "$script" | tr ' ' '\n' | sort -u)"
  boot_casks="$(sed -n 's/^  for cask in \(.*\); do$/\1/p' "$script" | tr ' ' '\n' | sort -u)"
  readme_formulae="$(grep -oE '`brew install [a-z0-9-]+`' "$readme" | sed 's/.*brew install //; s/`//' | sort -u)"
  readme_casks="$(grep -oE '`brew install --cask [a-z0-9-]+`' "$readme" | sed 's/.*--cask //; s/`//' | sort -u)"

  if [ -n "$boot_formulae" ] && [ -n "$readme_formulae" ]; then
    record_pass "control: both formula lists were found, so this case is comparing something"
  else
    record_fail "control: both formula lists were found, so this case is comparing something" \
      "bootstrap='$boot_formulae' readme='$readme_formulae'"
  fi

  if [ "$boot_formulae" = "$readme_formulae" ]; then
    record_pass "every formula the README installs is one the bootstrap installs"
  else
    record_fail "every formula the README installs is one the bootstrap installs" \
      "only in bootstrap: $(comm -23 <(printf '%s\n' "$boot_formulae") <(printf '%s\n' "$readme_formulae") | tr '\n' ' ')| only in README: $(comm -13 <(printf '%s\n' "$boot_formulae") <(printf '%s\n' "$readme_formulae") | tr '\n' ' ')"
  fi

  if [ "$boot_casks" = "$readme_casks" ]; then
    record_pass "and the casks agree too"
  else
    record_fail "and the casks agree too" "bootstrap='$boot_casks' readme='$readme_casks'"
  fi
}

report_suite() {
  echo
  echo "$passed passed, $failed failed"
  [ "$failed" -eq 0 ]
}

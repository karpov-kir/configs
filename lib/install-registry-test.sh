#!/usr/bin/env bash
# Cases for lib/install-registry.sh: which projects this machine installed into.
#
# The load-bearing case is the self-healing one. A registry that keeps naming a directory the human
# deleted makes `ai/bootstrap.sh --uninstall` say another project still needs the bucket, which is
# false and stops a human finishing a removal they meant.
#
# `$XDG_CONFIG_HOME` is pointed at a throwaway directory, which is what keeps every case off the
# developer's own registry.
set -u

here=$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
checkout=$(CDPATH= cd -P -- "$here/.." && pwd -P)
suite_name="lib/install-registry-test.sh"

# shellcheck source=./test-harness.sh
. "$checkout/lib/test-harness.sh" ||
  { printf '%s: lib/test-harness.sh did not load to the end — nothing was measured\n' "$suite_name" >&2; exit 2; }

echo "lib/install-registry.sh"

drive() { # <script body>
  out=$(
    repo="$checkout"
    script_name="install-registry-test.sh"
    label="registry test"
    dry_run=${case_dry_run:-false}
    export XDG_CONFIG_HOME="$cfg"
    # shellcheck source=./mount.sh
    . "$checkout/lib/mount.sh"
    # shellcheck source=./install-registry.sh
    . "$checkout/lib/install-registry.sh"
    eval "$1"
  ) 2>&1
  status=$?
}

cfg="$tmp/cfg"
reg="$cfg/kk-flavor/installs"
mkdir -p "$tmp/p1" "$tmp/p2"

# --- recording ---------------------------------------------------------------------------------------

case_dry_run=false drive "registry_record '$tmp/p1'"
expect_out "a project is recorded" "recorded"
grep -qxF "$tmp/p1" "$reg" &&
  record_pass "and the file holds its path" ||
  record_fail "and the file holds its path" "$(cat "$reg" 2>/dev/null)"

case_dry_run=false drive "registry_record '$tmp/p1'"
expect_out "recording the same project twice says so" "already recorded"
[ "$(grep -cxF "$tmp/p1" "$reg")" = "1" ] &&
  record_pass "and leaves one line, not two" ||
  record_fail "and leaves one line, not two" "$(cat "$reg")"

case_dry_run=false drive "registry_record '$tmp/p2'"
[ "$(grep -c . "$reg")" = "2" ] &&
  record_pass "a second project is recorded beside the first" ||
  record_fail "a second project is recorded beside the first" "$(cat "$reg")"

# --- self-healing ---------------------------------------------------------------------------------------

# The whole reason reading and pruning are one call. A caller that could read without pruning would
# get the stale answer, and the stale answer is the one that blocks a removal for a project that is
# not there any more.
rmdir "$tmp/p2"
case_dry_run=false drive "registry_live"
expect_out "a live read still names the project that exists" "$tmp/p1"
expect_not_out "and drops the one whose directory is gone" "$tmp/p2"
grep -qxF "$tmp/p2" "$reg" &&
  record_fail "and prunes it from the file as it reads" "still on disk" ||
  record_pass "and prunes it from the file as it reads"

mkdir -p "$tmp/p2"
printf '# a note someone left\n' >>"$reg"
case_dry_run=false drive "registry_live"
expect_not_out "a comment is not read back as a project" "a note someone left"
grep -qF "a note someone left" "$reg" &&
  record_pass "and it survives the prune" ||
  record_fail "and it survives the prune" "$(cat "$reg")"

# --- forgetting ---------------------------------------------------------------------------------------

case_dry_run=false drive "registry_forget '$tmp/p1'"
expect_out "a project is forgotten" "forgot"
grep -qxF "$tmp/p1" "$reg" &&
  record_fail "and its line is gone" "still there" ||
  record_pass "and its line is gone"

case_dry_run=false drive "registry_forget '$tmp/p1'"
expect_out "forgetting it twice is not an error" "was not recorded"

# --- no registry at all ---------------------------------------------------------------------------------

rm -rf "$cfg"
case_dry_run=false drive "registry_live"
expect_status "a machine with no registry reads clean" 0
[ -z "$out" ] &&
  record_pass "and names no project" ||
  record_fail "and names no project" "got: $out"

case_dry_run=false drive "registry_forget '$tmp/p1'"
expect_out "and forgetting against it is not an error" "nothing recorded"

# --- dry run ------------------------------------------------------------------------------------------

rm -rf "$cfg"
case_dry_run=true drive "registry_record '$tmp/p1'"
expect_out "a dry run says what it would record" "would record"
[ ! -e "$reg" ] &&
  record_pass "and writes no registry" ||
  record_fail "and writes no registry" "$(cat "$reg")"

report_suite

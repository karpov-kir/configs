#!/usr/bin/env bash
# Cases for lib/owned-region.sh: the region it owns inside a file it does not.
#
# What is asserted here is the half no installer's suite can reach cheaply — the states of a target
# file, and the promise that nothing outside the fences moves. The callers' own suites cover which
# fences they pass and what body they write.
#
# `say`, `refuse` and `$dry_run` come from lib/mount.sh, so this sources both in the order a real
# caller does. Sourcing owned-region.sh alone would leave those undefined and every case would die on
# `command not found` rather than on what it meant to measure.
set -u

here=$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
checkout=$(CDPATH= cd -P -- "$here/.." && pwd -P)
suite_name="lib/owned-region-test.sh"

# shellcheck source=./test-harness.sh
. "$checkout/lib/test-harness.sh" ||
  { printf '%s: lib/test-harness.sh did not load to the end — nothing was measured\n' "$suite_name" >&2; exit 2; }

echo "lib/owned-region.sh"

OPEN="<!-- kk:begin -->"
CLOSE="<!-- kk:end -->"

# Each case drives the library in a subshell, so a `refuse` in one cannot leave its refusal in the
# array the next one reads.
drive() { # <script body>
  out=$(
    repo="$checkout"
    script_name="owned-region-test.sh"
    label="region test"
    dry_run=${case_dry_run:-false}
    # shellcheck source=./mount.sh
    . "$checkout/lib/mount.sh"
    # shellcheck source=./owned-region.sh
    . "$checkout/lib/owned-region.sh"
    eval "$1"
    printf '%s\n' "---refusals:${#refusals[@]}"
  ) 2>&1
  status=$?
}

# --- a file that does not have the region ----------------------------------------------------------

printf 'Their own words.\n' >"$tmp/a.md"
case_dry_run=false drive "region_write '$tmp/a.md' '$OPEN' '$CLOSE' 'BODY'"
expect_out "an absent region is added" "added"
grep -q "^Their own words.$" "$tmp/a.md" &&
  record_pass "and the file's own content survives" ||
  record_fail "and the file's own content survives" "$(cat "$tmp/a.md")"
grep -q "^BODY$" "$tmp/a.md" &&
  record_pass "and the body is written between the fences" ||
  record_fail "and the body is written between the fences" "$(cat "$tmp/a.md")"

# Idempotence is the property every re-run of an installer depends on.
before=$(cat "$tmp/a.md")
case_dry_run=false drive "region_write '$tmp/a.md' '$OPEN' '$CLOSE' 'BODY'"
expect_out "a second write with the same body writes nothing" "already carries"
[ "$before" = "$(cat "$tmp/a.md")" ] &&
  record_pass "and the file is byte-identical" ||
  record_fail "and the file is byte-identical" "it changed"

# --- rewriting in place -----------------------------------------------------------------------------

case_dry_run=false drive "region_write '$tmp/a.md' '$OPEN' '$CLOSE' 'NEWBODY'"
expect_out "a changed body rewrites the region" "rewrote"
grep -q "^NEWBODY$" "$tmp/a.md" &&
  record_pass "and the new body is there" ||
  record_fail "and the new body is there" "$(cat "$tmp/a.md")"
grep -q "^BODY$" "$tmp/a.md" &&
  record_fail "and the old body is gone" "still there" ||
  record_pass "and the old body is gone"
grep -q "^Their own words.$" "$tmp/a.md" &&
  record_pass "and their content is still untouched" ||
  record_fail "and their content is still untouched" "$(cat "$tmp/a.md")"

# --- removal ------------------------------------------------------------------------------------------

case_dry_run=false drive "region_remove '$tmp/a.md' '$OPEN' '$CLOSE'"
expect_out "the region is removed" "removed"
[ "$(cat "$tmp/a.md")" = "Their own words." ] &&
  record_pass "and the file is left exactly as it was found" ||
  record_fail "and the file is left exactly as it was found" "got: $(cat "$tmp/a.md")"

case_dry_run=false drive "region_remove '$tmp/a.md' '$OPEN' '$CLOSE'"
expect_out "removing an absent region is not an error" "carries no"
expect_out "and it refuses nothing" "---refusals:0"

# --- the states that refuse -----------------------------------------------------------------------------

# Half a fence: something edited inside the region, so its extent is no longer ours to guess. This is
# the case the whole `conflict` state exists for.
printf 'Theirs.\n%s\nstray\n' "$OPEN" >"$tmp/half.md"
case_dry_run=false drive "region_write '$tmp/half.md' '$OPEN' '$CLOSE' 'BODY'"
expect_out "half a fence refuses rather than guessing" "one half of"
expect_out "and it is counted as a refusal" "---refusals:1"
grep -q "^stray$" "$tmp/half.md" &&
  record_pass "and nothing in the file was touched" ||
  record_fail "and nothing in the file was touched" "$(cat "$tmp/half.md")"

case_dry_run=false drive "region_remove '$tmp/half.md' '$OPEN' '$CLOSE'"
expect_out "and removal refuses on it too" "not ours to guess"

# A symlink: writing through one edits a file somewhere the caller never named — for a CLAUDE.md
# symlinked into a checkout, that means editing the checkout.
printf 'real\n' >"$tmp/real.md"
ln -s "$tmp/real.md" "$tmp/link.md"
case_dry_run=false drive "region_write '$tmp/link.md' '$OPEN' '$CLOSE' 'BODY'"
expect_out "a symlinked target refuses" "is a symlink"
[ "$(cat "$tmp/real.md")" = "real" ] &&
  record_pass "and the file behind it is untouched" ||
  record_fail "and the file behind it is untouched" "$(cat "$tmp/real.md")"

# Never creates a file: a typo in a path must not scatter plausible-looking files through a repo.
case_dry_run=false drive "region_write '$tmp/nope.md' '$OPEN' '$CLOSE' 'BODY'"
expect_out "a missing file refuses rather than being created" "never creates one"
[ ! -e "$tmp/nope.md" ] &&
  record_pass "and no file appeared" ||
  record_fail "and no file appeared" "it was created"

# --- dry run ---------------------------------------------------------------------------------------------

printf 'Theirs.\n' >"$tmp/dry.md"
case_dry_run=true drive "region_write '$tmp/dry.md' '$OPEN' '$CLOSE' 'BODY'"
expect_out "a dry run says what it would add" "would add"
[ "$(cat "$tmp/dry.md")" = "Theirs." ] &&
  record_pass "and writes nothing" ||
  record_fail "and writes nothing" "$(cat "$tmp/dry.md")"

report_suite

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

# The one-line file above cannot see this: removal holds back the blank line it added ahead of the
# fence, and holding it as the line itself is indistinguishable from holding nothing — so every blank
# line in a real, paragraphed CLAUDE.md goes with the region. The whole file is compared, not grepped
# for its words: a grep for the paragraphs passes over a file whose paragraph breaks are gone.
paragraphs=$(printf '# Project\n\nHow this works.\n\nAnd a second paragraph.\n')
printf '%s\n' "$paragraphs" >"$tmp/para.md"
case_dry_run=false drive "region_write '$tmp/para.md' '$OPEN' '$CLOSE' 'BODY'"
case_dry_run=false drive "region_remove '$tmp/para.md' '$OPEN' '$CLOSE'"
[ "$(cat "$tmp/para.md")" = "$paragraphs" ] &&
  record_pass "a paragraphed file comes back byte-identical after write then remove" ||
  record_fail "a paragraphed file comes back byte-identical after write then remove" "got: $(cat "$tmp/para.md")"

# --- the states that refuse -----------------------------------------------------------------------------

printf 'Theirs.\n%s\nstray\n' "$OPEN" >"$tmp/half.md"
case_dry_run=false drive "region_write '$tmp/half.md' '$OPEN' '$CLOSE' 'BODY'"
expect_out "half a fence refuses rather than guessing" "one half of"
expect_out "and it is counted as a refusal" "---refusals:1"
grep -q "^stray$" "$tmp/half.md" &&
  record_pass "and nothing in the file was touched" ||
  record_fail "and nothing in the file was touched" "$(cat "$tmp/half.md")"

case_dry_run=false drive "region_remove '$tmp/half.md' '$OPEN' '$CLOSE'"
expect_out "and removal refuses on it too" "not ours to guess"

printf 'real\n' >"$tmp/real.md"
ln -s "$tmp/real.md" "$tmp/link.md"
case_dry_run=false drive "region_write '$tmp/link.md' '$OPEN' '$CLOSE' 'BODY'"
expect_out "a symlinked target refuses" "is a symlink"
[ "$(cat "$tmp/real.md")" = "real" ] &&
  record_pass "and the file behind it is untouched" ||
  record_fail "and the file behind it is untouched" "$(cat "$tmp/real.md")"

case_dry_run=false drive "region_write '$tmp/nope.md' '$OPEN' '$CLOSE' 'BODY'"
expect_out "a missing file refuses rather than being created" "never creates one"
[ ! -e "$tmp/nope.md" ] &&
  record_pass "and no file appeared" ||
  record_fail "and no file appeared" "it was created"

# A write that cannot land. The replace runs on the right-hand side of a pipe, so a refusal made in
# there dies with the subshell and the run exits 0 saying ok — having failed to write. The refusal
# count is what this reads, because the REFUSED line prints either way and is not the contract.
#
# The directory is stripped of write permission while the file inside it stays writable, so
# region_writable passes and the temp file beside it is what fails. Run as root this case goes red
# rather than green: root writes into a 555 directory, and no refusal is the wrong answer to assert.
printf 'Theirs.\n\n%s\nOLD\n%s\n' "$OPEN" "$CLOSE" >"$tmp/locked.md"
chmod 555 "$tmp"
case_dry_run=false drive "region_write '$tmp/locked.md' '$OPEN' '$CLOSE' 'NEW'"
chmod 755 "$tmp"
expect_out "a replace that cannot write is counted as a refusal" "---refusals:1"
grep -q "^OLD$" "$tmp/locked.md" &&
  record_pass "and the original is untouched" ||
  record_fail "and the original is untouched" "$(cat "$tmp/locked.md")"

printf 'secret\n' >"$tmp/private.md"
ln "$tmp/private.md" "$tmp/hard.md"
case_dry_run=false drive "region_write '$tmp/hard.md' '$OPEN' '$CLOSE' 'BODY'"
expect_out "a hardlinked target refuses" "hard links"
grep -q "^BODY$" "$tmp/hard.md" &&
  record_fail "and nothing was written through it" "it wrote" ||
  record_pass "and nothing was written through it"
[ "$(cat "$tmp/private.md")" = "secret" ] &&
  record_pass "and the file sharing its contents is untouched" ||
  record_fail "and the file sharing its contents is untouched" "$(cat "$tmp/private.md")"

# The Linux failure this shipped with, made reproducible on any machine. `stat -f` is BSD's format flag
# and GNU's `--file-system`, so on Linux the first probe answered with a block of filesystem facts
# rather than failing; a multi-line answer made the numeric comparison false, that read as one link,
# and the write went through a hardlink to somebody's private file. The stub answers that way for every
# spelling, which is the one state no real machine here can produce.
mkdir -p "$tmp/statstub"
cat >"$tmp/statstub/stat" <<'STUB'
#!/usr/bin/env bash
printf '  File: "x"\n    ID: 99 Namelen: 255\n'
exit 0
STUB
chmod +x "$tmp/statstub/stat"
printf 'secret\n' >"$tmp/unknown-links.md"
case_dry_run=false PATH="$tmp/statstub:$PATH" drive "region_write '$tmp/unknown-links.md' '$OPEN' '$CLOSE' 'BODY'"
expect_out "a link count that cannot be read refuses" "no link count could be read"
grep -q "^BODY$" "$tmp/unknown-links.md" &&
  record_fail "and nothing was written on an unreadable link count" "it wrote" ||
  record_pass "and nothing was written on an unreadable link count"

# --- dry run ---------------------------------------------------------------------------------------------

printf 'Theirs.\n' >"$tmp/dry.md"
case_dry_run=true drive "region_write '$tmp/dry.md' '$OPEN' '$CLOSE' 'BODY'"
expect_out "a dry run says what it would add" "would add"
[ "$(cat "$tmp/dry.md")" = "Theirs." ] &&
  record_pass "and writes nothing" ||
  record_fail "and writes nothing" "$(cat "$tmp/dry.md")"

report_suite

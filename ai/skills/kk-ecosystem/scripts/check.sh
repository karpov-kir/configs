#!/usr/bin/env bash
# Ecosystem wiring check — the mechanical half of kk-ecosystem: every reference an agent could
# follow resolves to something that exists, and every script still parses.
#   usage: check.sh [<root>]   # <root> holds kk-flavor/ and skills/; defaults to . then ./ai
# Prints one line per finding, plus two always-loaded budgets: the router's files, and every skill's
# `description:`. Exits 1 with findings, 0 when clean, and 2 when it could not run at all — no resolvable
# root, or no findings file. A check that did not run is not a clean one.
# A change here needs a case in `~/.claude/skills/kk-ecosystem/scripts/check-test.sh`, and a scan you add
# needs one that fails without it; `~/.claude/skills/kk-ecosystem/scripts/check-mutate.sh` is what shows a
# case can fail. Full paths, because a bare name resolves against the reviewed tree first.
# Past code-style.md's ~450-line guidance deliberately: the halves of a split reach each other either
# by sourcing (executing a file out of the tree under audit) or through a second process, which breaks
# the single findings file, the bounded output and the exit codes the suite asserts on. Split it only
# on a design that keeps those three, never on the line count alone.
set -uo pipefail
export LC_ALL=C

root="${1:-}"
# `stats.sh` reports the same always-loaded tier and must always describe the same tree, so the default
# is a shared region rather than a claim in a comment: the two cannot resolve differently without the
# drift scan below saying so.
# --- shared:default-root ---
if [ -z "$root" ]; then
  for candidate in . ./ai; do
    if [ -d "$candidate/kk-flavor" ] && [ -d "$candidate/skills" ]; then
      root="$candidate"
      break
    fi
  done
fi
# --- end shared:default-root ---
if [ -z "$root" ] || [ ! -d "$root/kk-flavor" ] || [ ! -d "$root/skills" ]; then
  echo "check.sh: no root holding both kk-flavor/ and skills/ (tried '${1:-. and ./ai}')" >&2
  echo "check.sh: exit 2 — nothing was checked. Fix the invocation; do not read this as clean." >&2
  exit 2
fi
flavor="$root/kk-flavor"
skills="$root/skills"

findings="$(mktemp)" || {
  echo "check.sh: mktemp gave no findings file — exit 2, nothing was checked." >&2
  exit 2
}
trap 'rm -f "$findings"' EXIT

# A SKILL.md's `description:` value — the routing text, and the only part of a skill loaded in every
# session. Anchored to line 1, so a `---` rule in the body does not open frontmatter.
# --- shared:frontmatter-description ---
frontmatter_description() {
  awk 'NR == 1 && !/^---[[:space:]]*$/ { exit }
       NR > 1 && /^---[[:space:]]*$/ { exit }
       /^description:[[:space:]]*/ { sub(/^description:[[:space:]]*/, ""); print; exit }' "$1"
}
# --- end shared:frontmatter-description ---

# --- shared:opted-out-of-model-invocation ---
opted_out_of_model_invocation() {
  awk 'NR == 1 && !/^---[[:space:]]*$/ { exit }
       NR > 1 && /^---[[:space:]]*$/ { exit }
       tolower($0) ~ /^disable-model-invocation:[[:space:]]*(true|yes|on|1)[[:space:]]*$/ { found = 1; exit }
       END { exit !found }' "$1"
}
# --- end shared:opted-out-of-model-invocation ---

# A path as prose writes it, resolved to a file under the root; prints it, or nothing when it
# resolves nowhere or to more than one file.
resolve_ref() {
  local dir="$1" ref="$2" matches
  case "$ref" in
    '~/.kk-flavor/'*)
      [ -e "$flavor/${ref#'~/.kk-flavor/'}" ] && echo "$flavor/${ref#'~/.kk-flavor/'}"
      return ;;
    '~/.claude/skills/'*)
      [ -e "$skills/${ref#'~/.claude/skills/'}" ] && echo "$skills/${ref#'~/.claude/skills/'}"
      return ;;
  esac
  if [ -n "$dir" ] && [ -e "$dir/$ref" ]; then echo "$dir/$ref"; return; fi
  if [ -e "$root/$ref" ]; then echo "$root/$ref"; return; fi
  # A bare name is accepted only when one file in the tree could be meant.
  matches="$(find "$root" -path "*/$ref" -type f)"
  [ "$(printf '%s\n' "$matches" | grep -c .)" = 1 ] && echo "$matches"
}

# True when a cited path names at least one real file, an ambiguous bare name included.
ref_exists() {
  local dir="$1" ref="$2"
  [ -n "$(resolve_ref "$dir" "$ref")" ] && return 0
  [ -n "$(find "$root" -path "*/$ref" -type f 2>/dev/null | head -1)" ]
}

# The directory a real path resolves to, symlinks followed. `readlink -f` is not portable to the
# bash 3.2 machines this runs on; `cd -P` is.
# --- shared:canonical-dir ---
canonical_dir() {
  [ -d "$1" ] || return 0
  (cd -P -- "$1" 2>/dev/null && pwd -P)
}
# --- end shared:canonical-dir ---

# Anything attacker-chosen that reaches a finding goes through this first: one physical line per
# finding is what makes the ranking below mean anything. Every control byte goes, not only the two
# that split a line — an ESC sequence erases the real finding printed above it.
# --- shared:oneline ---
oneline() {
  printf '%s' "$1" | LC_ALL=C tr '[:cntrl:]' ' '
}
# --- end shared:oneline ---

# Comparison form for a heading or a cited section name.
plain_text() {
  tr '[:upper:]' '[:lower:]' | sed 's/[`*_]//g; s/[[:space:]][[:space:]]*/ /g; s/^ //; s/ $//'
}

# Every `#` heading in a markdown file, in comparison form. Fenced blocks are skipped.
markdown_headings() {
  awk '/^```/ { in_fence = !in_fence; next }
       !in_fence && /^#+[[:space:]]/ { sub(/^#+[[:space:]]*/, ""); print }' "$1" | plain_text |
    # A heading may carry a subtitle after an em dash and a citation names only the run before it, so
    # emit that run too. Cut at the em dash and nowhere else: a trailing run, or a word-by-word
    # prefix, would let half a heading satisfy a citation.
    awk '{ print }
         match($0, / — /) { print substr($0, 1, RSTART - 1) }'
}

# The mounts every `~/...` citation below is resolved through. Those citations are checked against
# *this checkout*, so a checkout that is not the installed one would report every one of them healthy.
flavor_want="$(canonical_dir "$flavor")"
flavor_have="$(canonical_dir "${HOME:-}/.kk-flavor")"
if [ -z "$flavor_have" ]; then
  echo "flavor not mounted: \$HOME/.kk-flavor is not a directory — every ~/.kk-flavor/ citation dangles at run time" >>"$findings"
elif [ "$flavor_have" != "$flavor_want" ]; then
  echo "flavor mounted elsewhere: \$HOME/.kk-flavor -> $(oneline "$flavor_have"), not $(oneline "$flavor_want")" >>"$findings"
fi
skills_mount="${HOME:-}/.claude/skills"
if [ ! -d "$skills_mount" ]; then
  echo "skills not mounted: $skills_mount is not a directory — no skill here is loadable and every ~/.claude/skills/ citation dangles" >>"$findings"
else
  for skill_dir in "$skills"/*/; do
    [ -d "$skill_dir" ] || continue
    skill_name="$(basename "$skill_dir")"
    mount_want="$(canonical_dir "$skill_dir")"
    mount_have="$(canonical_dir "$skills_mount/$skill_name")"
    if [ -z "$mount_have" ]; then
      echo "skill not mounted: $skills_mount/$(oneline "$skill_name") is missing — the skill exists here and cannot be invoked" >>"$findings"
    elif [ "$mount_have" != "$mount_want" ]; then
      echo "skill mounted elsewhere: $skills_mount/$(oneline "$skill_name") -> $(oneline "$mount_have"), not $(oneline "$mount_want")" >>"$findings"
    fi
  done
fi

# Relative markdown links, resolved against the linking file's own directory. A template's links
# resolve where it is emitted (a project's `.idsd/`), so a bare sibling name is unverifiable and passes.
find "$root" -name '*.md' -type f -print0 | while IFS= read -r -d '' file; do
  case "$file" in */templates/*) is_template=1 ;; *) is_template=0 ;; esac
  grep -a -oE '\]\([^)]+\)' "$file" | sed 's/^](//; s/)$//' | while IFS= read -r link; do
    case "$link" in http*|mailto:*|'#'*|'~'*) continue ;; esac
    [ -e "$(dirname "$file")/${link%%#*}" ] && continue
    if [ "$is_template" = 1 ]; then
      case "${link%%#*}" in /*|../*|*/../*|..) ;; *) continue ;; esac
    fi
    echo "dangling link: $(oneline "$file") -> $(oneline "$link")"
  done
done >>"$findings"

# `~/.kk-flavor/...` and `~/.claude/skills/...` — how a skill reaches outside its own directory.
grep -a -rhoE '~/\.(kk-flavor|claude/skills)/[A-Za-z0-9._/-]+' "$root" --include='*.md' --include='*.sh' 2>/dev/null |
  sed 's#[.,;:]*$##' | sort -u | while IFS= read -r ref; do
  [ -n "$(resolve_ref "" "$ref")" ] || echo "dangling home ref: $(oneline "$ref")"
done >>"$findings"

# Direction: the shared layer never cites into a lane, and never names one (ecosystem.md → **One home**).
# Three shapes are banned. A path into a skill needs one name character before the slash, or a glob's
# bare `/SKILL.md` tail matches. A bare skill name counts only when a skill of that name exists. That
# gate is not a licence for the prose it lets through: every other `kk-*`/`idsd-*` token the shared
# layer carries is already a finding from the `unknown skill referenced` scan below. A rename only
# swaps which of those two scans fires.
# The third is a lane file named by its basename alone — `report.sh`, `check.sh`. It carries neither a
# lane name nor a path, so the two scans above both miss it, and it steers its reader into a lane just
# the same. Its message deliberately does not extend `shared layer names a lane`: check-test.sh matches
# a finding by fixed substring, so a message carrying another's whole text would satisfy that other
# one's `assert_does_not_report` cases and turn them into silent passes.
# Fences are not skipped, unlike in the scans that resolve a citation — a banned form steers its
# reader from inside one too.
# `-a` on all three greps below, never dropped: one NUL byte anywhere in a file makes grep call that
# file binary, and it then prints `Binary file … matches` (BSD grep) or nothing at all (GNU grep >= 3.5,
# which sends that notice to stderr) in place of the matches. Those scans then read no violation out of
# a file `find` did hand them, while `was_flavor_scanned` still counts it as scanned. One committed
# byte turns a violating shared layer into `wiring: clean`.
# `find -type f`, not the `grep -r` above: GNU `grep -r` follows a symlink named on its own command
# line, and both operands are attacker-authored when this runs as a PR review's stage.
# Process substitution, not a pipe, for the file list and for each hit list inside the loop: a pipe runs
# its loop in a subshell, so the flag and the two counters would reset at every boundary they exist to
# outlive.
direction_targets=("$flavor")
[ -f "$root/CLAUDE.md" ] && direction_targets+=("$root/CLAUDE.md")
was_flavor_scanned=0
# The lane names come from the tree, never from the `kk-`/`idsd-` families. Nothing enforces that
# naming rule, so keying on the prefix would trust a convention no scan checks, and a skill named
# outside it could be cited and named freely. A name outside the characters below would reach `grep -E`
# as a metacharacter and match text no skill owns.
lane_names=""
for skill_dir in "$skills"/*/; do
  [ -f "$skill_dir/SKILL.md" ] || continue
  lane_name="$(basename "$skill_dir")"
  # `kk-flavor` is the shared layer itself, so a reviewed tree committing `skills/kk-flavor/` would turn
  # every mention of that layer inside its own standards into a finding.
  [ "$lane_name" = "kk-flavor" ] && continue
  case "$lane_name" in *[!A-Za-z0-9._-]*) continue ;; esac
  lane_names="${lane_names:+$lane_names|}$(printf '%s' "$lane_name" | sed 's/\./\\./g')"
done
# Bounded per shape across the whole scan. Every emitted finding still costs a fork for its hit, and
# the printer below shows at most 40 of this rank, so hits past the cap buy nothing: 20000 of them out
# of one 100KB committed file hold this scan for over two minutes. A per-file bound does not bound the
# scan: 41 hits in each of 500 files cost the same two minutes. The counters therefore sit out here and
# span every file. Reported at the boundary, never silently
# dropped: the notice leads with the file and a space, which sorts ahead of that file's own
# `file:line:` hits, so the printer's per-rank cap below drops those hits before it drops the notice.
cites_shown=0
names_shown=0
# The basenames that name exactly one file under `$skills` — resolved once, because the alternative is
# a `find` per token per file. `uniq -u` is the whole gate: a basename several lanes carry names the
# *kind* of file rather than one of them, which is why `SKILL.md` (every lane has one) does not fire
# while `report.sh` (one lane has it) does.
# NUL-delimited and charset-filtered, never `find | sed`: a committed filename holding a newline reaches
# `sed` as two lines and hands the reviewed tree both halves of a forgery. The tail is a basename no
# lane carries, so a standard naming a file nothing under `skills/` holds is reported against a file the
# branch never touched. The head is a second copy of a real basename, so `uniq -u` drops it and a
# genuine violation goes quiet. The `-test.sh` scan below refuses the same shape for the same reason.
basenames_under() {
  find "$@" -type f \( -name '*.sh' -o -name '*.md' \) -print0 2>/dev/null |
    while IFS= read -r -d '' basename_path; do
      lane_basename="${basename_path##*/}"
      case "$lane_basename" in *[!A-Za-z0-9._-]*) continue ;; esac
      printf '%s\n' "$lane_basename"
    done
}
lane_basenames="$(basenames_under "$skills" | sort | uniq -u)"
# A name the shared layer also carries is ambiguous — the reviewed tree fills `$skills`, so one committed
# file under a lane named after a standard would otherwise report every standard citing that sibling,
# findings aimed at files the branch never touched. Those names are not dropped from the set above and
# subtracted; the loop below tests this set first and `continue`s, so they never reach the violation
# test. Subtracting them as well was unreachable, and the mutation harness is what proved it: the mutant
# aimed at that subtraction killed nothing, because removing it changed no output byte.
# **The ordering below is therefore load-bearing**, and it is the guard, not a convenience: test the
# violation set first and every ambiguous name becomes a forged finding. `subtracted name not reported`
# is the mutant that holds it.
# Silence here would be the cheapest mute the reviewed tree has — commit any `.md` under `kk-flavor/`
# named after a lane file and every mention of that file stops being checked, while no other scan names
# the file the branch committed. A narrowed scan reports that it narrowed.
lane_basenames_ambiguous="$(grep -xF -f <(basenames_under "${direction_targets[@]}" | sort -u) <<<"$lane_basenames")"
# Padded once for the `case` the gate below runs, never a `grep -qxF` per token: that gate is reached by
# every `.sh`/`.md`-shaped token the shared layer carries, not by the few that pass it, so a fork there is
# a fork per token — the one cost the emit counters above cannot bound. One 190KB committed file of them
# holds this scan for two minutes with nothing to show, because no hit ever reaches the cap.
lane_newline='
'
lane_basenames_padded="$lane_newline$lane_basenames$lane_newline"
lane_ambiguous_padded="$lane_newline$lane_basenames_ambiguous$lane_newline"
basenames_shown=0
ambiguous_shown=0
# The bound every finding here reports under, and the notice each of them ends on. Both sit in one
# place so raising the bound is one edit, and a finding added later cannot drift into its own wording.
# The test itself stays at each call site: a hit past the bound must not pay for the forks a finding
# costs, and an argument is evaluated whether the function it is passed to prints it or not.
finding_cap=40
report_bound_reached() {
  echo "$1: $2 — $finding_cap already shown across the shared layer; the rest are not listed"
}
# A function, so the call site stays one line: deleting a multi-line `case` to prove this guard fires
# leaves a fragment that does not parse, and a mutant that cannot run proves nothing about the guard.
report_unchecked_basename() {
  local reported_file="$1" reported_line="$2" reported_name="$3"
  ambiguous_shown=$((ambiguous_shown + 1))
  if [ "$ambiguous_shown" -le "$finding_cap" ]; then
    echo "basename not checked: $reported_file:$reported_line — $(oneline "$reported_name") names a file under both a lane and the shared layer, so this scan cannot tell which was meant; rename one of them (ecosystem.md → **One home**)"
  elif [ "$ambiguous_shown" -eq $((finding_cap + 1)) ]; then
    report_bound_reached "basename not checked" "$reported_file"
  fi
}
while IFS= read -r -d '' file; do
  # Set from flavor files alone: one flag over both tiers would let a readable `CLAUDE.md` stand in
  # for the tree and mute the guard below.
  case "$file" in "$flavor"/*) was_flavor_scanned=1 ;; esac
  # Sanitised once, not per hit: `$(oneline …)` is a fork, and `$file` does not change inside the loop.
  safe_file="$(oneline "$file")"
  while IFS= read -r hit; do
    cites_shown=$((cites_shown + 1))
    if [ "$cites_shown" -le "$finding_cap" ]; then
      echo "shared layer cites into a lane: $safe_file:$(oneline "$hit") — move the rule to a standard (ecosystem.md → **One home**)"
    elif [ "$cites_shown" -eq $((finding_cap + 1)) ]; then
      report_bound_reached "shared layer cites into a lane" "$safe_file"
    fi
    # `(/seg)+`, not `/seg`: one segment stops the echoed path at `.../kk-drive/scripts` and drops the
    # file the citation was actually about, which is the half a reader needs to find and move it.
  done < <(grep -a -noE "[A-Za-z0-9._~-][A-Za-z0-9._/~-]*/SKILL\\.md|[A-Za-z0-9._~-][A-Za-z0-9._/~-]*/(${lane_names:-\$^})(/[A-Za-z0-9._-]+)+" "$file")
  while IFS= read -r hit; do
    named=${hit#*:}
    # The trailing run of `.`, `_` and `-` is punctuation the token ends on, not part of the name. The
    # suffix class below carries all three, so `grep -o` keeps the hyphen a `kk-drive-*` glob ends on and
    # the full stop of a sentence that ends on the lane name alike, and either would then match no skill
    # directory. Trimmed as a run, never one character: a token can end on more than one.
    named="${named%"${named##*[!._-]}"}"
    # The whole token is tested, not the alternation's own match: `kk-drive-verified` starts with a real
    # lane name and is not one, so matching the prefix alone would report a skill that does not exist as
    # a lane the shared layer names.
    [ -f "$skills/$named/SKILL.md" ] || continue
    names_shown=$((names_shown + 1))
    if [ "$names_shown" -le "$finding_cap" ]; then
      echo "shared layer names a lane: $safe_file:$(oneline "$hit") — name the lane, and let the skill bind itself to it (ecosystem.md → **One home**)"
    elif [ "$names_shown" -eq $((finding_cap + 1)) ]; then
      report_bound_reached "shared layer names a lane" "$safe_file"
    fi
  done < <(grep -a -noE "\\b(${lane_names:-\$^})[A-Za-z0-9._-]*" "$file")
  [ -n "$lane_basenames$lane_basenames_ambiguous" ] && while IFS= read -r hit; do
    named=${hit#*:}
    # The leading boundary character comes back with the match; a token starts on `[A-Za-z0-9]`, so a
    # first character outside that set is the boundary and never part of the name.
    case "$named" in [!A-Za-z0-9]*) named="${named#?}" ;; esac
    case "$lane_ambiguous_padded" in *"$lane_newline$named$lane_newline"*) report_unchecked_basename "$safe_file" "${hit%%:*}" "$named"; continue ;; esac
    case "$lane_basenames_padded" in *"$lane_newline$named$lane_newline"*) ;; *) continue ;; esac
    basenames_shown=$((basenames_shown + 1))
    if [ "$basenames_shown" -le "$finding_cap" ]; then
      # `${hit%%:*}` is the line number alone: echoing the match would carry the boundary character the
      # grep consumed, so the finding would show an unbalanced tick for a name written `` `doit.sh` ``.
      echo "shared layer reaches into a lane by basename: $safe_file:${hit%%:*} — $(oneline "$named") is $(oneline "$(find "$skills" -path "*/$named" -type f)"); move the rule to a standard (ecosystem.md → **One home**)"
    elif [ "$basenames_shown" -eq $((finding_cap + 1)) ]; then
      report_bound_reached "shared layer reaches into a lane by basename" "$safe_file"
    fi
    # The boundary excludes `/` and `~`, so a token inside a path the cites scan already reported does
    # not come back here as a second finding for the same text.
  done < <(grep -a -noE "(^|[^/~A-Za-z0-9._-])[A-Za-z0-9][A-Za-z0-9._-]*\\.(sh|md)" "$file")
done >>"$findings" < <(find "${direction_targets[@]}" -name '*.md' -type f -print0)
[ "$was_flavor_scanned" = 1 ] ||
  echo "direction scan read no files under $flavor — a check that did not run is not a clean one" >>"$findings"

# Backticked in-repo paths — `scripts/report.sh`, `templates/ice-template.md`, `AGENT-BRIEF.md`.
# Fenced blocks are skipped. The shapes stay narrow on purpose — a bare lowercase `*.md` is as often a
# file a project owns (`charter.md`, `roadmap.md`), so only SHOUTY-with-a-hyphen is matched.
find "$root" -type f \( -name '*.md' -o -name '*.sh' \) -print0 | while IFS= read -r -d '' file; do
  dir="$(dirname "$file")"
  # A skill cites its own tooling from the skill root (`scripts/report.sh`) even in a file that sits
  # under `scripts/`, so resolve from both.
  skill_root="$dir"
  case "$file" in
    "$skills"/*) rest="${file#"$skills"/}"; skill_root="$skills/${rest%%/*}" ;;
  esac
  # Split on the tick rather than shrinking the line: rebuilding the tail on every hit is quadratic in
  # a line length the tree chooses, and one committed multi-megabyte line would stall the whole check.
  awk '/^```/ { in_fence = !in_fence; next }
       in_fence { next }
       { n = split($0, part, "`")
         for (k = 1; k <= n - 2; ) {
           if (part[k + 1] != "") { print part[k + 1]; k += 2 } else { k += 1 }
         } }' "$file" |
    grep -E '^([A-Za-z0-9][A-Za-z0-9._/-]*/[A-Za-z0-9._-]+\.(sh|md)|[A-Za-z0-9][A-Za-z0-9._-]*\.sh|[A-Z][A-Z0-9]*(-[A-Z0-9]+)+\.md)$' |
    sort -u | while IFS= read -r token; do
      ref_exists "$dir" "$token" && continue
      [ "$skill_root" != "$dir" ] && ref_exists "$skill_root" "$token" && continue
      echo "dangling path ref: $(oneline "$file") -> $(oneline "$token")"
    done
done >>"$findings"

# The other half of a `<file> → <Section>` citation: the heading it names must still be there. An
# arrow counts only when the text before it resolves to a real markdown file, which keeps prose
# arrows ("intent → build") out.
find "$root" -type f \( -name '*.md' -o -name '*.sh' \) -print0 |
  while IFS= read -r -d '' file; do
    awk 'function last_tick(s,   i) { for (i = length(s); i > 0; i--) if (substr(s, i, 1) == "`") return i; return 0 }
         /^```/ { in_fence = !in_fence; next }
         in_fence { next }
         {
           n = split($0, seg, "→")
           for (i = 2; i <= n; i++) {
             # The cited file: a markdown link, a backticked path, or a bare filename.
             before = seg[i - 1]
             sub(/[[:space:]]+$/, "", before)
             path = ""
             if (before ~ /\]\([^()]*\)$/) { path = before; sub(/^.*\]\(/, "", path); sub(/\)$/, "", path) }
             else if (substr(before, length(before), 1) == "`") {
               tick = last_tick(substr(before, 1, length(before) - 1))
               if (tick > 0) path = substr(before, tick + 1, length(before) - tick - 1)
             }
             else if (match(before, /[A-Za-z0-9._\/-]+$/)) path = substr(before, RSTART, RLENGTH)
             sub(/#.*$/, "", path)
             if (path !~ /[A-Za-z0-9]\.md$/) continue

             after = seg[i]
             sub(/^[[:space:]]+/, "", after)
             sec = ""
             if (substr(after, 1, 2) == "**") { rest = substr(after, 3); k = index(rest, "**") }
             else if (substr(after, 1, 1) == "`") { rest = substr(after, 2); k = index(rest, "`") }
             else k = 0
             if (k > 1) sec = substr(rest, 1, k - 1)
             # Whether the name arrived inside `**`/backticks is the whole decision here: read exactly,
             # or guessed at by the fallback below. An undelimited name truncates at the first comma, so
             # half a heading satisfies the citation and a rename then breaks it in silence — which is
             # why ecosystem.md → **Conventions a new file joins** requires the delimited form.
             delimited = (sec != "")
             if (sec == "") {
               sec = after
               if (match(sec, /[():;,.!?"]/)) sec = substr(sec, 1, RSTART - 1)
               k = index(sec, "—")
               if (k > 1) sec = substr(sec, 1, k - 1)
             }
             gsub(/[`*]/, "", sec)
             sub(/^#+[[:space:]]*/, "", sec)
             sub(/^[[:space:]]+/, "", sec); sub(/[[:space:]]+$/, "", sec)
             if (sec != "") printf "%s\t%d\t%s\t%s\t%d\n", FILENAME, FNR, path, sec, delimited
           }
         }' "$file"
  done |
  while IFS="$(printf '\t')" read -r src line path section delimited; do
    target="$(resolve_ref "$(dirname "$src")" "$path")"
    if [ -z "$target" ]; then
      echo "unresolvable citation path: $(oneline "$src"):$line -> $(oneline "$path")"
      continue
    fi
    # Reported even when the section resolves today: undelimited is how it stops resolving in silence.
    [ "$delimited" = 1 ] ||
      echo "undelimited section citation: $(oneline "$src"):$line -> $(oneline "$path") → $(oneline "$section") is not wrapped in ** or backticks"
    # `resolve_ref` tests with `-e`, which follows a symlink, so the cited path resolved to whatever
    # the reviewed tree pointed it at: `evil.md -> /dev/zero` or a committed FIFO makes the read below
    # never return. Reported rather than skipped — a target nothing read must not be indistinguishable
    # from a checked one.
    if [ ! -f "$target" ]; then
      echo "citation target is not a regular file: $(oneline "$src"):$line -> $(oneline "$path") — it was NOT read"
      continue
    fi
    # Prose runs on past the heading it names, so accept the longest leading run that is a heading.
    headings="$(markdown_headings "$target")"
    want="$(printf '%s\n' "$section" | plain_text)"
    while [ -n "$want" ]; do
      # Here-string, never `printf … | grep -q`: under `pipefail` the writer's SIGPIPE (141) turns a
      # match into a miss.
      grep -qxF "$want" <<<"$headings" && break
      case "$want" in *' '*) want="${want% *}" ;; *) want="" ;; esac
    done
    [ -n "$want" ] || echo "dangling section ref: $(oneline "$src"):$line -> $(oneline "$path") → $(oneline "$section")"
  done >>"$findings"

# Our own skill namespaces — a name in prose must be a skill that exists.
grep -a -rhoE '\b(kk|idsd)-[a-z0-9-]+' "$root" --include='*.md' --include='*.yaml' 2>/dev/null |
  sed 's/-$//' | sort -u | while IFS= read -r name; do
  [ "$name" = "kk-flavor" ] && continue
  # Two readings, and the scan cannot tell them apart: a misspelled skill, or prose that happens to wear
  # the family's shape (`kk-drive-verified`). Suppressing a token whose prefix is a real skill would mask
  # the first — `kk-drives` is exactly that shape — so the message carries both readings instead.
  [ -f "$skills/$name/SKILL.md" ] ||
    echo "unknown skill referenced: $(oneline "$name") — no skills/$(oneline "$name")/SKILL.md. If this is prose rather than a skill, reword it so it does not read as one"
done >>"$findings"

# A skill is its directory plus a SKILL.md; a directory without one is invisible to the loader.
find "$skills" -mindepth 1 -maxdepth 1 -type d -print0 | while IFS= read -r -d '' dir; do
  [ -f "$dir/SKILL.md" ] || echo "skill dir without SKILL.md: $(oneline "$dir")"
done >>"$findings"

# A skill's frontmatter name is how it is invoked; a mismatch with its directory makes it unreachable.
find "$skills" -name SKILL.md -type f -print0 | while IFS= read -r -d '' file; do
  declared="$(sed -n '2,10s/^name: *//p' "$file" | head -1)"
  expected="$(basename "$(dirname "$file")")"
  [ "$declared" = "$expected" ] || echo "skill name/dir mismatch: $(oneline "$file") declares '$(oneline "$declared")'"
  [ -n "$(frontmatter_description "$file")" ] || echo "skill without a description: $(oneline "$file")"
done >>"$findings"

# Skills reach their scripts by path (`scripts/report.sh …`), so a lost exec bit is a stage that
# cannot run at all.
find "$root" -name '*.sh' -type f -print0 | while IFS= read -r -d '' script; do
  [ -x "$script" ] || echo "script not executable: $(oneline "$script")"
  # Parse under every bash `#!/usr/bin/env bash` could resolve to: macOS still ships 3.2 as
  # /bin/bash, and it rejects constructs bash 5 accepts.
  for bash_binary in bash /bin/bash; do
    command -v "$bash_binary" >/dev/null 2>&1 &&
      "$bash_binary" -n "$script" 2>&1 | LC_ALL=C tr '\001-\011\013-\037\177' ' ' | sed "s#^#syntax: #"
  done
done >>"$findings"

# Enforces ecosystem.md → **Prefer the mechanism**: every case label of a top-level
# `case "${1:-}" in` dispatch needs one `<script> <subcommand>` somewhere an agent reads. Labels are
# read at the case arm's own indentation, so a nested `done)` is not one.
find "$root" -name '*.sh' -type f -print0 | while IFS= read -r -d '' script; do
  base="$(basename "$script")"
  awk 'index($0, "case \"${1:-}\" in") == 1 { inside = 1; next }
       inside && index($0, "esac") == 1 { exit }
       inside' "$script" |
    grep -oE '^  [a-z][a-z0-9|_-]*\)$' | tr -d ' )' | tr '|' '\n' | sort -u |
    while IFS= read -r subcommand; do
      # Filters before the pattern, then `--`: a script named `-x.sh` puts a leading `-` in $base,
      # which grep would read as a flag.
      grep -a -rqF --include='*.md' --include='*.sh' --include='*.yaml' -- "$base $subcommand" "$root" ||
        echo "$(oneline "$base") subcommand with no call site: $(oneline "$subcommand")"
    done
done >>"$findings"

# Also ecosystem.md → **Prefer the mechanism**: a script is prose turned into enforcement, so
# something has to prove the enforcement still fires. Each script states its test position in its
# header, either the `-test.sh` covering it or `# untested: <why>`, and that header is what
# `kk-reduce`'s Phase 6 reads to pick what to run. Stating neither hides the script from that phase.
# Naming a suite that is not there is the worse half: the phase finds nothing to run, and the script
# counts as covered by a suite that does not exist.
# The suite list is built once. A `find` per name turns one crafted header naming 3000 suites into
# 3000 whole-tree walks, and this scan runs inside `kk-pr-review` over a branch that chose its own
# contents. NUL-delimited and charset-filtered for the same reason: piped through `sed`, a file named
# `x<LF>ghost-test.sh` contributes its second line as a bare suite name, and a header naming a
# missing suite then passes the existence check.
test_suites="$(find "$root" -name '*-test.sh' -type f -print0 |
  while IFS= read -r -d '' suite_path; do
    suite="${suite_path##*/}"
    case "$suite" in
      *[!A-Za-z0-9_.-]*) continue ;;
    esac
    printf '%s\n' "$suite"
  done | sort -u)"
find "$root" -name '*.sh' -type f -print0 | while IFS= read -r -d '' script; do
  base="$(basename "$script")"
  case "$base" in
    *-test.sh | *-mutate.sh) continue ;;
  esac
  # Reading past the leading comment block would let a `-test.sh` named anywhere in the body clear the
  # check. Bounded at 200 lines because the block is slurped whole and then read twice more through the
  # here-strings below, so an all-comment file is held three times over.
  header="$(awk 'NR == 1 && /^#!/ { next }
                 NR > 200 { exit }
                 /^#/ { print; next }
                 { exit }' "$script")"
  # Here-strings, never `printf | grep`, and `awk NR <= 8` rather than `head`: both would SIGPIPE the
  # writer, and the note above the `dangling section ref` grep says why that registers as a miss under
  # `pipefail`. Eight is what a real header takes, its suite and its mutation run. A header past the
  # cap is reported rather than truncated in silence.
  all_named="$(grep -oE '[A-Za-z0-9_.-]+-test\.sh' <<<"$header" | sort -u)"
  named_tests="$(awk 'NR <= 8' <<<"$all_named")"
  # The count is of the 200-line window, never of the file: past the bound nothing was read, so a
  # header carrying thousands of names would report the window's total as if it were the file's.
  named_count="$(grep -c . <<<"$all_named")"
  if [ "$named_count" -gt 8 ]; then
    echo "script names more suites than the scan reads: $(oneline "$base") names $named_count in its first 200 lines, of which 8 are read"
  fi
  if [ -n "$named_tests" ]; then
    while IFS= read -r named_test; do
      # `--`, as the `dangling section ref` grep does: the name comes from a header this script did not
      # write, and `--test.sh` is a legal match grep would read as an option, aborting with a usage
      # dump into the output an agent drafts review comments from.
      grep -qxF -- "$named_test" <<<"$test_suites" ||
        echo "script names a missing test: $(oneline "$base") names $(oneline "$named_test")"
    done <<<"$named_tests"
  elif ! grep -qE '^#[[:space:]]*untested:[[:space:]]*[^[:space:]]' <<<"$header"; then
    echo "script declares no test position: $(oneline "$base") names no -test.sh and carries no '# untested: <why>'"
  fi
done >>"$findings"

# Every budget file is contained under the root before it is read — `CLAUDE.md` and `inject.md`
# included, not just the docs one of them lists. All three are attacker-authored when this runs as a PR
# review's ecosystem stage (`quality-pipeline.md` → **The stages**), and the import scan below prints
# matched substrings, so a `../../` target reaches a reviewing agent's context.
# A symlink is refused rather than resolved: `cd -P` canonicalises a *directory*, so it never sees the
# final component, and a link at a budget path would walk through a check that only tested its parent.
# --- shared:contained-in-root ---
root_canon="$(canonical_dir "$root")"
contained_in_root() {
  local dir
  [ -L "$1" ] && return 1
  # A regular file, or nothing. `-e` alone admits a FIFO or a device, which `cat` then blocks on
  # forever; a dangling symlink fails `-e` entirely, so callers enter on `-e || -L` and both become
  # a refusal, not a silent drop.
  [ -f "$1" ] || return 1
  # Readable too, not just regular: `-f` passes on a mode-000 file, and the read behind the figure
  # then fails, leaving a file counted whose words are not.
  [ -r "$1" ] || return 1
  dir="$(canonical_dir "$(dirname "$1")")"
  [ -n "$root_canon" ] && [ -n "$dir" ] || return 1
  [ "$dir" = "$root_canon" ] || [ "${dir#"$root_canon"/}" != "$dir" ]
}
# --- end shared:contained-in-root ---
# The refused file's **name** is attacker-chosen and is printed, so the name and the number of these
# lines are both bounded.
budget_refusals=0
refuse_budget_file() {
  budget_refusals=$((budget_refusals + 1))
  if [ "$budget_refusals" -le 5 ]; then
    echo "budget file refused (symlink, unreadable, or resolves outside $root) — not read, not counted: $(oneline "$1" | cut -c1-80)" >>"$findings"
  elif [ "$budget_refusals" -eq 6 ]; then
    echo "further budget-file refusals suppressed; the count above is not the total" >>"$findings"
  fi
}
# A block fenced `# --- shared:<name> ---` … `# --- end shared:<name> ---` must be byte-identical
# everywhere that name appears. Two scripts in different skills duplicate these on purpose — a shared
# file would make one skill's tooling depend on another's, and this script runs inside a worktree of
# code it did not write, where sourcing a file is executing it. That tolerance holds only while drift
# is *detected* (ecosystem.md → **Prefer the mechanism**).
# One pass over the tree, never one per region name: both the region count and the bytes they span are
# chosen by whoever wrote the tree. The body is compared as the whole remainder past the third tab —
# taking it as the fourth field would end the comparison at the first tab inside a region body.
find "$root" -name '*.sh' -type f -print0 |
  xargs -0 awk '
    FNR == 1 { region = "" }
    /^[[:space:]]*# --- shared:[A-Za-z0-9_-]+ ---[[:space:]]*$/ {
      region = $0
      sub(/^[[:space:]]*# --- shared:/, "", region); sub(/[[:space:]]*---[[:space:]]*$/, "", region)
      present[region, FILENAME] = 1
      next
    }
    /^[[:space:]]*# --- end shared:[A-Za-z0-9_-]+ ---[[:space:]]*$/ { region = ""; next }
    region != "" {
      key = region SUBSEP FILENAME
      # Escaping first is what makes the join reversible: an unescaped `\001` planted in a region body
      # would let two copies compare equal with a guard deleted from one of them.
      line = $0
      gsub(/\\/, "\\\\", line); gsub(/\001/, "\\001", line); gsub(/\t/, "\\t", line)
      # Bounded: a region body is as attacker-controlled as the file it sits in, and an *unterminated*
      # fence swallows the rest of the file. Past the cap the region is reported rather than compared —
      # an unchecked region must never read as a matching one.
      if (length(body[key]) < 262144) body[key] = body[key] line "\001"
      else oversize[key] = 1
    }
    END { for (k in present) { split(k, p, SUBSEP); name = p[2]; gsub(/\t/, " ", name)
            printf "%s\t%s\t%s\t%s\n", p[1], name, (k in oversize ? "OVER" : "OK"), body[k] } }
  ' 2>/dev/null |
  awk -F'\t' '
    { copies[$1]++
      if ($3 == "OVER") over[$1] = 1
      text = $0; sub(/^[^\t]*\t[^\t]*\t[^\t]*\t/, "", text)
      if (!(($1 SUBSEP text) in seen)) { seen[$1 SUBSEP text] = 1; distinct[$1]++ } }
    END {
      for (r in copies) {
        if (r in over)
          printf "shared region %s is too large to compare — it was NOT checked for drift\n", r
        else if (copies[r] < 2)
          printf "shared region %s has %d copy — the marker names a counterpart no file carries\n", r, copies[r]
        else if (distinct[r] > 1)
          printf "shared region %s has drifted: %d copies, %d distinct versions\n", r, copies[r], distinct[r]
      }
    }' >>"$findings"

# The always-loaded budget: the root CLAUDE.md every system prompt carries, inject.md, and every doc
# it lists under "Read always".
budget_files=()
if [ -e "$root/CLAUDE.md" ] || [ -L "$root/CLAUDE.md" ]; then
  if contained_in_root "$root/CLAUDE.md"; then
    budget_files+=("$root/CLAUDE.md")
  else
    refuse_budget_file "$root/CLAUDE.md"
  fi
fi
is_inject_in_root=0
if contained_in_root "$flavor/inject.md"; then
  budget_files+=("$flavor/inject.md")
  is_inject_in_root=1
else
  refuse_budget_file "$flavor/inject.md"
fi
# A listed doc that does not exist is skipped by name, not fed to `cat`, or the printed file total
# disagrees with what was measured. Guarded by the same containment test as the count: refusing to
# count a file this then reads anyway refuses nothing.
if [ "$is_inject_in_root" -eq 1 ]; then
  while IFS= read -r doc; do
    if [ ! -e "$flavor/$doc" ] && [ ! -L "$flavor/$doc" ]; then
      echo "inject.md lists '$(oneline "$doc")' under Read always, but $flavor/$(oneline "$doc") does not exist" >>"$findings"
    elif ! contained_in_root "$flavor/$doc"; then
      refuse_budget_file "inject.md Read-always target $doc"
    else
      budget_files+=("$flavor/$doc")
    fi
  done < <(sed -n '/^## Read always/,/^## /p' "$flavor/inject.md" | grep -oE '\]\([^)#]+\)' |
    sed 's/^](//; s/)$//')
fi
# An `@path` import inside a budget file loads with it, so a budget blind to one under-reports the tier
# it exists to measure. Imports resolve against the *installed* copy of the carrier, outside `$root`;
# they are resolved at that mount below, and the rest are named in the figure. Any extension counts,
# but one is required: that and the non-word character before the `@` are what keep `@param`, a package
# scope and an email address out. Each field is prefixed with a space so that second test has something
# to read at position 1. `fence` resets per file — a budget file ending inside one would otherwise blank
# the scan for every file after it, and the two scripts list the budget in different orders, so they
# would disagree. Two bounds keep it linear: `PATH_MAX` per field, and 64 matches in one field.
# --- shared:import-scan ---
imports_in() {
  awk 'FNR == 1 { fence = 0 }
       /^```/ { fence = !fence; next }
       fence { next }
       { line = $0
         gsub(/`[^`]*`/, " ", line)
         n = split(line, field, /[[:space:]]+/)
         for (i = 1; i <= n; i++) {
           if (length(field[i]) > 4096) continue
           tok = " " field[i]
           hits = 0
           while (match(tok, /[^A-Za-z0-9_]@[~A-Za-z0-9._\/-]+\.[A-Za-z0-9]+/) && ++hits <= 64) {
             print substr(tok, RSTART + 2, RLENGTH - 2)
             tok = substr(tok, RSTART + RLENGTH)
           }
         } }' "$@" | sort -u
}
# --- end shared:import-scan ---
budget_imports=""
if [ ${#budget_files[@]} -gt 0 ]; then
  budget_imports="$(imports_in "${budget_files[@]}")"
fi

# --- shared:import-at-mount ---
# An import loads from beside the *installed* copy of the file carrying it, so `@RTK.md` in `CLAUDE.md`
# is `~/.claude/RTK.md`. That file is **not** one this repo forgot: the rtk installer puts it there and
# verifies it, so moving it into the tree fights the installer.
# Only `CLAUDE.md`'s own imports resolve here — an `inject.md` import loads from `~/.kk-flavor/`, so
# resolving one here would count whatever file shares the name. Don't swap the scan for a substring
# search: it sees the fenced and backticked mentions the scan skips on purpose.
# Depth 1: an import nested inside a resolved file is neither counted nor named.
# Bare filenames only — `@../../.ssh/id_rsa` must not resolve.
# Resolution also needs this checkout to be the installed one, or a branch someone else wrote names
# files in the invoking user's real `~/.claude/` and folds their sizes into a number it also authored.
# `cd -P` follows a symlinked *directory*, so refusing a symlinked `$root/kk-flavor` is what stops a
# branch committing one to the real install and opening that gate.
import_mount_is_installed=0
if [ -n "${HOME:-}" ] && [ ! -L "$root/kk-flavor" ] && [ -n "$(canonical_dir "$root/kk-flavor")" ] &&
   [ "$(canonical_dir "${HOME}/.kk-flavor")" = "$(canonical_dir "$root/kk-flavor")" ]; then
  import_mount_is_installed=1
fi
claude_imports=""
if [ ! -L "$root/CLAUDE.md" ] && [ -f "$root/CLAUDE.md" ] && [ -r "$root/CLAUDE.md" ]; then
  claude_imports="$(imports_in "$root/CLAUDE.md")"
fi
import_newline='
'
# Sets `import_target` rather than printing it: a command substitution per name is a fork per name.
import_target=""
# `import_refusal` carries a reason only for the shapes nothing legitimate produces — a traversal, a
# symlink planted at the mount path, a file present and deliberately unreadable. An import simply absent
# from the mount, a checkout that isn't the installed one, and a subdirectory import this resolver does
# not handle are the ordinary cases: they stay quiet names in the note.
import_refusal=""
resolve_import_at_mount() {
  import_target=""
  import_refusal=""
  [ "$import_mount_is_installed" -eq 1 ] || return 1
  # `@dir/file.md` is a legitimate import form, so a plain subdirectory name is refused here — this
  # resolves bare names only — but quietly: reporting it would take an honest run to exit 1.
  case "$1" in
    '') return 1 ;;
    '~'*|/*|../*|*/../*|*/..) import_refusal="a traversal, not a bare filename"; return 1 ;;
    */*) return 1 ;;
  esac
  case "$import_newline$claude_imports$import_newline" in
    *"$import_newline$1$import_newline"*) ;;
    *) return 1 ;;
  esac
  [ -L "${HOME}/.claude/$1" ] && { import_refusal="a symlink at the mount"; return 1; }
  [ -f "${HOME}/.claude/$1" ] || return 1
  [ -r "${HOME}/.claude/$1" ] || { import_refusal="unreadable at the mount"; return 1; }
  import_target="${HOME}/.claude/$1"
}
# --- end shared:import-at-mount ---

add_import_to_budget() {
  budget_files+=("$1")
}

report_import_refusal() {
  echo "import refused ($2), named but not counted: $(oneline "$1" | cut -c1-80)" >>"$findings"
}

# Resolved imports join the budget before it is counted; the rest stay named in the note. Attempts are
# capped, and past the cap every remaining name goes to the note rather than dropping out silently.
# --- shared:import-resolution ---
# Leftover names accumulate in a file, never in a shell string: `s="$s$name"` re-copies everything
# gathered so far on every name, which is quadratic in a count the attacker picks.
budget_uncounted_file="$(mktemp)" || {
  echo "budget scan: mktemp gave no scratch file — exit 2, the import list cannot be bounded." >&2
  exit 2
}
if [ -n "$budget_imports" ]; then
  import_attempts=0
  while IFS= read -r budget_import; do
    [ -n "$budget_import" ] || continue
    # Reset here, not only inside the resolver: past the cap the resolver is not called, so its own
    # reset never runs and the last examined name's reason would be reported against a name nothing
    # looked at.
    import_target=""
    import_refusal=""
    if [ "$import_attempts" -lt 64 ]; then
      import_attempts=$((import_attempts + 1))
      resolve_import_at_mount "$budget_import"
    fi
    if [ -n "$import_target" ]; then
      add_import_to_budget "$import_target"
    else
      [ -n "$import_refusal" ] && report_import_refusal "$budget_import" "$import_refusal"
      printf '%s\n' "$budget_import" >>"$budget_uncounted_file"
    fi
  done <<EOF
$budget_imports
EOF
  # The capping region below wants one newline-separated string with no trailing newline.
  budget_imports="$(cat "$budget_uncounted_file")"
fi
rm -f "$budget_uncounted_file"
# --- end shared:import-resolution ---

# Refusing every budget file above leaves the array empty, and under `set -u` bash 3.2 errors on
# "${arr[@]}" — which would abort the run when the refusals are what needs reading.
budget_lines=0
budget_words=0
# Counted per file and summed, never `cat` over the array: concatenating glues the last word of a file
# with no final newline onto the first word of the next, and `stats.sh` sums per file, so the two would
# report different figures for one tree. Deduplicated first — `inject.md`'s Read-always list can name
# one file any number of times, and counting it twice inflates the tier it is supposed to measure.
if [ ${#budget_files[@]} -gt 0 ]; then
  while IFS= read -r budget_file; do
    [ -n "$budget_file" ] || continue
    budget_lines=$((budget_lines + $(wc -l <"$budget_file" | tr -d ' ')))
    budget_words=$((budget_words + $(wc -w <"$budget_file" | tr -d ' ')))
  done <<EOF
$(printf '%s\n' "${budget_files[@]}" | sort -u)
EOF
fi
# Capped in bytes, not just in entries: this line rides the exit-0 path, so an uncapped list prints
# attacker-chosen text under `wiring: clean`. The count stays exact; only the naming is trimmed.
budget_note=""
if [ -n "$budget_imports" ]; then
# --- shared:import-cap ---
  import_count=$(printf '%s\n' "$budget_imports" | wc -l | tr -d ' ')
  import_names=$(printf '%s\n' "$budget_imports" | head -10 | cut -c1-60 | tr '\n' ' ' | cut -c1-200 |
    sed 's/ $//')
  [ "$import_count" -gt 10 ] && import_names="$import_names … and $((import_count - 10)) more"
# --- end shared:import-cap ---
  budget_note=" + $import_count uncounted import(s): $import_names"
fi
echo "always-loaded: $budget_lines lines, $budget_words words across ${#budget_files[@]} files$budget_note"

# Every skill's description loads in every session too: the same tier, held to the same bar, and the
# only part of a skill no file in the router lists.
description_words=0
routed_skills=0
skill_total=0
for skill_file in "$skills"/*/SKILL.md; do
  [ -f "$skill_file" ] || continue
  skill_total=$((skill_total + 1))
  opted_out_of_model_invocation "$skill_file" && continue
  routed_skills=$((routed_skills + 1))
  description_words=$((description_words + $(frontmatter_description "$skill_file" | wc -w)))
done
echo "always-loaded: $description_words words of skill description across $routed_skills of $skill_total skills"

if [ -s "$findings" ]; then
  # Bounded in line length and in line count before anything is printed: a finding quotes text this
  # script did not choose, into output an agent drafts a PR comment from. Bounding here, on the one
  # path every finding takes, keeps a check added later from reopening it.
  finding_total=$(sort -u "$findings" | wc -l | tr -d ' ')
  # Ordered before it is cut, never alphabetically: `sort -u | head` shows one class until the cap runs
  # out, so a flood of crafted `dangling link:` lines would bury a `syntax:` error, a drifted shared
  # region or a refused budget file. Those name a broken or tampered-with check rather than a broken
  # reference, and a finding meaning *this check did not check the tree you think* ranks above every
  # reference finding. Every pattern is anchored at `^`, or a line whose *link target* merely carries
  # the substring promotes itself.
  # Capped per class as well as in total, because rank 0 is also the cheapest to mass-produce: each rank
  # shows at most 40 and says how many of its own it withheld. Two passes over one sorted file, because
  # the per-class total has to be known before the first line of that class prints.
  findings_sorted="$(mktemp)" || {
    echo "check.sh: mktemp gave no sort file — exit 2, the findings could not be bounded." >&2
    exit 2
  }
  sort -u "$findings" >"$findings_sorted"
  awk '
    function rank(line) {
      if (line ~ /^syntax: /) return 0
      if (line ~ /^shared region /) return 1
      if (line ~ /^direction scan read no files/) return 1
      if (line ~ /^flavor not mounted/) return 1
      if (line ~ /^flavor mounted elsewhere/) return 1
      if (line ~ /^skills not mounted/) return 1
      if (line ~ /^skill not mounted/) return 1
      if (line ~ /^skill mounted elsewhere/) return 1
      if (line ~ /^budget file refused/) return 2
      if (line ~ /^script names a missing test/) return 2
      if (line ~ /^script names more suites than the scan reads/) return 2
      if (line ~ /^basename not checked/) return 2
      if (line ~ /^script not executable/) return 3
      if (line ~ /^skill name\/dir mismatch/) return 3
      if (line ~ /^import refused/) return 4
      return 5
    }
    NR == FNR { total[rank($0)]++; next }
    { r = rank($0)
      if (++shown[r] <= 40) printf "%d\t%s\n", r, $0
      else if (shown[r] == 41)
        printf "%d\t… and %d more of this class, suppressed — fix these first\n", r, total[r] - 40
    }' "$findings_sorted" "$findings_sorted" |
    sort -s -k1,1n | cut -f2- | cut -c1-500 | head -200 >"$findings_sorted.bounded"
  cat "$findings_sorted.bounded"
  # Counted from what was actually printed, never from either cap: two mechanisms hide findings, so
  # arithmetic against one alone contradicts the other.
  findings_shown=$(grep -cv 'of this class, suppressed' "$findings_sorted.bounded" || true)
  [ "$finding_total" -gt "$findings_shown" ] &&
    echo "… and $((finding_total - findings_shown)) further finding(s) not shown — fix these and re-run"
  rm -f "$findings_sorted" "$findings_sorted.bounded"
  exit 1
fi
echo "wiring: clean"

#!/usr/bin/env bash
# Ecosystem wiring check — the mechanical half of kk-ecosystem: every reference an agent could
# follow resolves to something that exists, and every script still parses.
#   usage: check.sh [<root>]   # <root> holds kk-flavor/ and skills/; defaults to . then ./ai
# Prints one line per finding, plus two always-loaded budgets: the router's files, and every skill's
# `description:`. Exits 1 with findings, 0 when clean, and 2 when the root could not be resolved —
# a check that did not run is not a clean one.
set -uo pipefail
export LC_ALL=C

root="${1:-}"
if [ -z "$root" ]; then
  for candidate in . ./ai; do
    if [ -d "$candidate/kk-flavor" ] && [ -d "$candidate/skills" ]; then
      root="$candidate"
      break
    fi
  done
fi
if [ -z "$root" ] || [ ! -d "$root/kk-flavor" ] || [ ! -d "$root/skills" ]; then
  echo "check.sh: no root holding both kk-flavor/ and skills/ (tried '${1:-. and ./ai}')" >&2
  echo "check.sh: exit 2 — nothing was checked. Fix the invocation; do not read this as clean." >&2
  exit 2
fi
flavor="$root/kk-flavor"
skills="$root/skills"

# Every check below appends to this file and nothing else carries their results, so an unset
# `findings` sends all of them to `>>""` — each block fails, each error goes to stderr, and the
# empty-file test at the bottom prints "wiring: clean". Exit 2 here instead.
findings="$(mktemp)" || {
  echo "check.sh: mktemp gave no findings file — exit 2, nothing was checked." >&2
  exit 2
}
trap 'rm -f "$findings"' EXIT

# A SKILL.md's `description:` value — the routing text, and the only part of a skill loaded in
# every session. Prints nothing when the file has no frontmatter or no description line. Anchored to
# line 1, so a `---` rule in the body does not open frontmatter; only the first `description:` counts.
# stats.sh carries the same copy; the drift check below keeps the two budgets equal.
# Each predicate is one awk, never `awk | grep -q`: grep -q exits on the first match, and under
# `pipefail` awk's resulting SIGPIPE (141) turns that match into a miss.
# --- shared:frontmatter-description ---
frontmatter_description() {
  awk 'NR == 1 && !/^---[[:space:]]*$/ { exit }
       NR > 1 && /^---[[:space:]]*$/ { exit }
       /^description:[[:space:]]*/ { sub(/^description:[[:space:]]*/, ""); print; exit }' "$1"
}
# --- end shared:frontmatter-description ---

# True when the skill opted out of model invocation, so its description never enters a context
# window and costs nothing until the human types `/<name>`.
# --- shared:opted-out-of-model-invocation ---
opted_out_of_model_invocation() {
  awk 'NR == 1 && !/^---[[:space:]]*$/ { exit }
       NR > 1 && /^---[[:space:]]*$/ { exit }
       tolower($0) ~ /^disable-model-invocation:[[:space:]]*(true|yes|on|1)[[:space:]]*$/ { found = 1; exit }
       END { exit !found }' "$1"
}
# --- end shared:opted-out-of-model-invocation ---

# A path as prose writes it, resolved to a file under the root; prints it, or nothing when it
# resolves nowhere or to more than one file. `~/.kk-flavor/...` and `~/.claude/skills/...` address
# the two mounts directly and never fall back — a wrong path under either is a finding, not
# something to go hunting for. Relative markdown links keep the stricter rule below: a link is a
# click target, so it must resolve locally.
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
  # Prose cites a standard by bare name (`ecosystem.md`) far more often than by a path that resolves
  # from where the citing file sits. Accept that only when one file in the tree could be meant.
  matches="$(find "$root" -path "*/$ref" -type f)"
  [ "$(printf '%s\n' "$matches" | grep -c .)" = 1 ] && echo "$matches"
}

# True when a cited path names at least one real file. resolve_ref prints nothing for a bare name
# several files answer to; that ambiguity blocks a heading lookup but still proves the file is
# there, which is all a path check asks.
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

# Comparison form for a heading or a cited section name: markup dropped, spacing collapsed, case
# folded. Capitalisation and decoration drift between the heading and the citation; the words don't.
plain_text() {
  tr '[:upper:]' '[:lower:]' | sed 's/[`*_]//g; s/[[:space:]][[:space:]]*/ /g; s/^ //; s/ $//'
}

# Every `#` heading in a markdown file, in comparison form. Fenced blocks are skipped — a `#` line
# in an example is code, not a section anything can cite.
markdown_headings() {
  awk '/^```/ { in_fence = !in_fence; next }
       !in_fence && /^#+[[:space:]]/ { sub(/^#+[[:space:]]*/, ""); print }' "$1" | plain_text |
    # A heading may carry a subtitle after an em dash, and a citation names only the run before it:
    # `→ **Some section**` cites `## Some section — and its subtitle`. Emit that run as a second
    # matchable form. Cut the *heading* at the em dash and nowhere else, and only its leading run.
    # Emit a trailing run, or every word-by-word prefix, and half a heading satisfies a citation.
    # (Trimming the *citation* is the caller's job and is safe, because a citation may run on into
    # prose; a heading may not.) Keep the example synthetic: a real heading here would be a
    # hand-sync pair nothing checks.
    awk '{ print }
         match($0, / — /) { print substr($0, 1, RSTART - 1) }'
}

# The mounts every `~/...` citation below is resolved through. Those citations are checked against
# *this checkout*. So without this block, a checkout that is not the installed one (a clone, a
# worktree, a moved repo) reports every one of them healthy, while an agent following any one of
# them reads nothing. A finding rather than an exit: the rest of the check still earns its run there.
flavor_want="$(canonical_dir "$flavor")"
flavor_have="$(canonical_dir "${HOME:-}/.kk-flavor")"
if [ -z "$flavor_have" ]; then
  echo "flavor not mounted: \$HOME/.kk-flavor is not a directory — every ~/.kk-flavor/ citation dangles at run time" >>"$findings"
elif [ "$flavor_have" != "$flavor_want" ]; then
  echo "flavor mounted elsewhere: \$HOME/.kk-flavor -> $flavor_have, not $flavor_want" >>"$findings"
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
      echo "skill not mounted: $skills_mount/$skill_name is missing — the skill exists here and cannot be invoked" >>"$findings"
    elif [ "$mount_have" != "$mount_want" ]; then
      echo "skill mounted elsewhere: $skills_mount/$skill_name -> $mount_have, not $mount_want" >>"$findings"
    fi
  done
fi

# Relative markdown links, resolved against the linking file's own directory. A template's links
# resolve where it is emitted (a project's `.idsd/`), not where the template sits, so a bare sibling
# name is unverifiable from here and passes; one that climbs out of the emit root with `..` or is
# absolute resolves nowhere in either place, and is a finding like any other.
find "$root" -name '*.md' -type f -print0 | while IFS= read -r -d '' file; do
  case "$file" in */templates/*) is_template=1 ;; *) is_template=0 ;; esac
  grep -oE '\]\([^)]+\)' "$file" | sed 's/^](//; s/)$//' | while IFS= read -r link; do
    case "$link" in http*|mailto:*|'#'*|'~'*) continue ;; esac
    [ -e "$(dirname "$file")/${link%%#*}" ] && continue
    if [ "$is_template" = 1 ]; then
      case "${link%%#*}" in /*|../*|*/../*|..) ;; *) continue ;; esac
    fi
    echo "dangling link: $file -> $link"
  done
done >>"$findings"

# `~/.kk-flavor/...` and `~/.claude/skills/...` — how a skill reaches a standard, or another skill,
# from outside its own directory. A glob (`~/.claude/skills/*/SKILL.md`) names no one file and is
# not matched.
grep -rhoE '~/\.(kk-flavor|claude/skills)/[A-Za-z0-9._/-]+' "$root" --include='*.md' --include='*.sh' 2>/dev/null |
  sed 's#[.,;:]*$##' | sort -u | while IFS= read -r ref; do
  [ -n "$(resolve_ref "" "$ref")" ] || echo "dangling home ref: $ref"
done >>"$findings"

# Backticked in-repo paths — `scripts/report.sh`, `templates/ice-template.md`, `AGENT-BRIEF.md`.
# Markdown links are checked above; these are not, and they are what holds one skill to another
# skill's tooling. Fenced blocks are skipped: a path in an example is illustration, not wiring.
# The shapes are kept narrow, because a check that cries wolf gets ignored and the guard dies:
#   - anything with a `/` in it, and every bare `*.sh` — scripts all live in this tree;
#   - a bare `*.md` only when SHOUTY-with-a-hyphen (`AGENT-BRIEF.md`), this ecosystem's shape for a
#     reference file. Bare lowercase names are as often a file a project owns or a run creates
#     (`charter.md`, `findings.md`, `roadmap.md`), and the token alone does not tell those from
#     `history.md`;
#   - `~/...` belongs to the scan above, and a leading `.` (`.idsd/charter.md`) names the emit root,
#     which does not exist here to be found. A `<placeholder>` or a glob is excluded by the shapes.
find "$root" -type f \( -name '*.md' -o -name '*.sh' \) -print0 | while IFS= read -r -d '' file; do
  dir="$(dirname "$file")"
  # A skill cites its own tooling from the skill root (`scripts/report.sh`) even in a file that sits
  # under `scripts/`, so resolve from both.
  skill_root="$dir"
  case "$file" in
    "$skills"/*) rest="${file#"$skills"/}"; skill_root="$skills/${rest%%/*}" ;;
  esac
  # Split on the tick rather than matching pairs and shrinking the line: splitting on a separator is
  # linear, where `line = substr(line, RSTART + RLENGTH)` rebuilds the tail on every hit and turns a
  # long line dense in backticks into quadratic work — this runs over every file in the tree, so one
  # committed multi-megabyte line stalls the whole check. Odd fields sit outside the ticks, even
  # fields are the spans between a pair, and an empty one is the `` case. `part[k + 1]` is the span
  # between tick k and tick k+1, and `k <= n - 2` requires the closing tick to exist — an
  # unterminated trailing span is not a code span and is not emitted. An empty span cannot match, so
  # its closing tick becomes the next opening one, which is why `k` moves by 1 there and by 2 on a hit.
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
      echo "dangling path ref: $file -> $token"
    done
done >>"$findings"

# The other half of a `<file> → <Section>` citation. The checks above prove the file is there;
# nothing proved the heading still is, and headings move under every prose pass. An arrow counts as
# a citation only when the text before it resolves to a real markdown file — that is what keeps
# prose arrows ("intent → build", "throwaway → committed") out. awk finds them, the shell compares.
find "$root" -type f \( -name '*.md' -o -name '*.sh' \) -print0 |
  while IFS= read -r -d '' file; do
    awk 'function last_tick(s,   i) { for (i = length(s); i > 0; i--) if (substr(s, i, 1) == "`") return i; return 0 }
         /^```/ { in_fence = !in_fence; next }
         in_fence { next }
         {
           n = split($0, seg, "→")
           for (i = 2; i <= n; i++) {
             # The cited file, taken from the end of the text before the arrow: a markdown link, a
             # backticked path, or a bare filename. Anything not ending in a *named* `.md` has no
             # headings to cite, so one test drops every prose arrow. It also drops the rule that
             # documents this very citation form, because its `<file>.md` is a placeholder with no
             # name in front of the extension.
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

             # The section name: whatever the bold or backtick delimiters hold, else the run of text
             # up to the first punctuation a heading cannot contain.
             after = seg[i]
             sub(/^[[:space:]]+/, "", after)
             sec = ""
             if (substr(after, 1, 2) == "**") { rest = substr(after, 3); k = index(rest, "**") }
             else if (substr(after, 1, 1) == "`") { rest = substr(after, 2); k = index(rest, "`") }
             else k = 0
             if (k > 1) sec = substr(rest, 1, k - 1)
             if (sec == "") {
               sec = after
               if (match(sec, /[():;,.!?"]/)) sec = substr(sec, 1, RSTART - 1)
               k = index(sec, "—")
               if (k > 1) sec = substr(sec, 1, k - 1)
             }
             gsub(/[`*]/, "", sec)
             sub(/^#+[[:space:]]*/, "", sec)
             sub(/^[[:space:]]+/, "", sec); sub(/[[:space:]]+$/, "", sec)
             if (sec != "") printf "%s\t%d\t%s\t%s\n", FILENAME, FNR, path, sec
           }
         }' "$file"
  done |
  while IFS="$(printf '\t')" read -r src line path section; do
    target="$(resolve_ref "$(dirname "$src")" "$path")"
    # No target means the citation names no one file: missing, or a bare name several files answer
    # to. Either way its heading goes unchecked, and skipping it silently leaves the whole citation
    # unread. Say so, and let it be repointed at something that resolves.
    if [ -z "$target" ]; then
      echo "unresolvable citation path: $src:$line -> $path"
      continue
    fi
    # Prose runs on past the heading it names ("→ Comments exempts that prose from…"), so drop a
    # trailing word at a time and accept the longest leading run that is a heading.
    heads="$(markdown_headings "$target")"
    want="$(printf '%s\n' "$section" | plain_text)"
    while [ -n "$want" ]; do
      # Here-string, never `printf … | grep -q` — under `pipefail` the writer's SIGPIPE (141) turns
      # a match into a miss, so every heading in a file with more than a pipe buffer of them would
      # report dangling.
      grep -qxF "$want" <<<"$heads" && break
      case "$want" in *' '*) want="${want% *}" ;; *) want="" ;; esac
    done
    [ -n "$want" ] || echo "dangling section ref: $src:$line -> $path → $section"
  done >>"$findings"

# Our own skill namespaces — a name in prose must be a skill that exists.
grep -rhoE '\b(kk|idsd)-[a-z0-9-]+' "$root" --include='*.md' --include='*.yaml' 2>/dev/null |
  sed 's/-$//' | sort -u | while IFS= read -r name; do
  [ "$name" = "kk-flavor" ] && continue
  [ -f "$skills/$name/SKILL.md" ] || echo "unknown skill referenced: $name"
done >>"$findings"

# A skill is its directory plus a SKILL.md; a directory without one is invisible to the loader.
find "$skills" -mindepth 1 -maxdepth 1 -type d -print0 | while IFS= read -r -d '' dir; do
  [ -f "$dir/SKILL.md" ] || echo "skill dir without SKILL.md: $dir"
done >>"$findings"

# A skill's frontmatter name is how it is invoked; a mismatch with its directory makes it unreachable.
# Its description is the only text the router matches on, so a skill without one is never selected.
find "$skills" -name SKILL.md -type f -print0 | while IFS= read -r -d '' file; do
  declared="$(sed -n '2,10s/^name: *//p' "$file" | head -1)"
  expected="$(basename "$(dirname "$file")")"
  [ "$declared" = "$expected" ] || echo "skill name/dir mismatch: $file declares '$declared'"
  [ -n "$(frontmatter_description "$file")" ] || echo "skill without a description: $file"
done >>"$findings"

# Skills reach their scripts by path (`scripts/report.sh …`), so a lost exec bit is a stage that
# cannot run at all.
find "$root" -name '*.sh' -type f -print0 | while IFS= read -r -d '' script; do
  [ -x "$script" ] || echo "script not executable: $script"
  # Parse under every bash `#!/usr/bin/env bash` could resolve to, not just the one first on PATH:
  # macOS still ships 3.2 as /bin/bash, and it rejects constructs bash 5 accepts.
  for bash_binary in bash /bin/bash; do
    command -v "$bash_binary" >/dev/null 2>&1 && "$bash_binary" -n "$script" 2>&1 | sed "s#^#syntax: #"
  done
done >>"$findings"

# Enforces ecosystem.md → **Prefer the mechanism**.
# Case labels of a top-level `case "${1:-}" in` dispatch, each needing one `<script> <subcommand>`
# somewhere an agent reads — a skill body, a sibling script, or the script's own message telling you
# to run it. Labels are read at the case arm's own indentation, so nested `done)` is not one.
find "$root" -name '*.sh' -type f -print0 | while IFS= read -r -d '' script; do
  base="$(basename "$script")"
  awk 'index($0, "case \"${1:-}\" in") == 1 { inside = 1; next }
       inside && index($0, "esac") == 1 { exit }
       inside' "$script" |
    grep -oE '^  [a-z][a-z0-9|_-]*\)$' | tr -d ' )' | tr '|' '\n' | sort -u |
    while IFS= read -r subcommand; do
      # Filters before the pattern, then `--`: a script named `-x.sh` puts a leading `-` in $base,
      # which grep reads as a flag and exits 2, printing "no call site" for a subcommand that has
      # one. `--` after the operands would instead demote --include to a filename.
      grep -rqF --include='*.md' --include='*.sh' --include='*.yaml' -- "$base $subcommand" "$root" ||
        echo "$base subcommand with no call site: $subcommand"
    done
done >>"$findings"

# Every budget file is contained under the root before it is read — `CLAUDE.md` and `inject.md`
# included, not just the docs one of them lists. All three are attacker-authored whenever this runs
# as a PR review's ecosystem stage (`quality-pipeline.md` → **The stages**), inside a worktree of a
# branch someone else wrote, and either a `../../` target or a symlink at any of the three otherwise
# puts a file the invoking user can read into the budget. The import scan below prints matched
# substrings, so that file's content reaches a reviewing agent's context and from there a draft
# PR comment.
# A symlink is refused rather than resolved: `cd -P` canonicalises a *directory*, so it never sees
# the final component, and a link committed at a budget path would walk straight through a check that
# only tested its parent. Nothing under the flavor tree is a symlink, so refusing costs nothing here.
# --- shared:contained-in-root ---
root_canon="$(canonical_dir "$root")"
contained_in_root() {
  local dir
  [ -L "$1" ] && return 1
  # A regular file, or nothing. `-e` alone admits a FIFO or a device, which `cat` then blocks on
  # forever; a dangling symlink fails `-e` entirely, so callers enter on `-e || -L` and both become
  # a refusal, not a silent drop. It also refuses a path that does not exist, so no caller can pass
  # one through and inflate the file count.
  [ -f "$1" ] || return 1
  dir="$(canonical_dir "$(dirname "$1")")"
  [ -n "$root_canon" ] && [ -n "$dir" ] || return 1
  [ "$dir" = "$root_canon" ] || [ "${dir#"$root_canon"/}" != "$dir" ]
}
# --- end shared:contained-in-root ---
# The refused file's *content* never reaches the message, but its **name** is attacker-chosen and is
# printed, so both the name and the number of these lines are bounded. Unbounded, 200 symlinks named
# in crafted prose put 36 KB of it on stdout — and stdout here becomes a comment drafted on a fork PR.
budget_refusals=0
refuse_budget_file() {
  budget_refusals=$((budget_refusals + 1))
  if [ "$budget_refusals" -le 5 ]; then
    echo "budget file refused (symlink, or resolves outside $root) — not read, not counted: $(printf '%s' "$1" | cut -c1-80)" >>"$findings"
  elif [ "$budget_refusals" -eq 6 ]; then
    echo "further budget-file refusals suppressed; the count above is not the total" >>"$findings"
  fi
}
# A block fenced `# --- shared:<name> ---` … `# --- end shared:<name> ---` must be byte-identical
# everywhere that name appears. Two scripts in different skills duplicate these on purpose — a shared
# file would make one skill's tooling depend on another's, and this script runs inside a worktree of
# code it did not write, where sourcing a file is executing it. That tolerance holds only while drift
# is *detected*, and a comment asking a reader to keep two copies in step detects nothing
# ([ecosystem.md](../../../kk-flavor/standards/ecosystem.md) → **Prefer the mechanism**). A region
# only one file carries is a marker that promises a counterpart nothing keeps.
# One pass over the tree, never one pass per region name. Both the number of regions and the bytes
# they span are chosen by whoever wrote the tree, so a loop that re-walks every script for each name
# is quadratic in two attacker-controlled factors — 800 regions measured at 64s, and a script never
# returning is the failure this whole file has no exit code for. The first awk emits one
# `name<TAB>file<TAB>body` row per fenced block and the second groups them, so if `xargs` splits the
# file list into batches the rows still meet in the same aggregator. The body is compared as the whole
# remainder past the third tab, and the file name — which nothing downstream reads — has its own tabs
# flattened first. Taking it as the fourth field instead would end the comparison at the first tab
# inside a region body, so two copies differing only after that tab would compare equal: a byte-level
# check silently downgraded to a prefix one, which is the drift this exists to catch.
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
      # The body is never compared as a re-serialised string. Escaping first is what makes the join
      # reversible: a body line is free to contain the separator, and a `\001` planted at the end of
      # a comment inside a fenced region reads to bash as comment while an unescaped join reads it as
      # a line break — two copies then compare equal with a security guard deleted from one of them.
      line = $0
      gsub(/\\/, "\\\\", line); gsub(/\001/, "\\001", line); gsub(/\t/, "\\t", line)
      # Bounded, because a region body is as attacker-controlled as the file it sits in and awk
      # reallocates this string per line: 10 MB measured at 86s, and an *unterminated* fence swallows
      # the rest of the file, so no closing marker is even needed. Past the cap the region is reported
      # rather than compared — an unchecked region must never read as a matching one.
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
# A listed doc that does not exist is skipped by name, not fed to `cat`: a bare `cat:` error explains
# nothing, and counting the file anyway makes the printed file total disagree with what was measured.
# The dangling-link check reports the reference itself; this keeps the budget honest about its inputs.
# Guarded by the same test, because refusing to *count* a file this then reads anyway refuses
# nothing: a symlinked `inject.md` pointed at `/dev/zero` or a FIFO blocks here forever, and pointed
# at a regular file it hands over that file's Read-always list instead.
if [ "$is_inject_in_root" -eq 1 ]; then
  while IFS= read -r doc; do
    if [ ! -e "$flavor/$doc" ] && [ ! -L "$flavor/$doc" ]; then
      echo "inject.md lists '$doc' under Read always, but $flavor/$doc does not exist" >>"$findings"
    elif ! contained_in_root "$flavor/$doc"; then
      refuse_budget_file "inject.md Read-always target $doc"
    else
      budget_files+=("$flavor/$doc")
    fi
  done < <(sed -n '/^## Read always/,/^## /p' "$flavor/inject.md" | grep -oE '\]\([^)#]+\)' |
    sed 's/^](//; s/)$//')
fi
# Refusing every budget file above leaves the array empty, and bash 3.2 errors on "${arr[@]}" under
# `set -u` — an aborted run reporting nothing, where the refusals are exactly what needs reading.
budget_lines=0
budget_words=0
if [ ${#budget_files[@]} -gt 0 ]; then
  budget_lines=$(cat "${budget_files[@]}" | wc -l | tr -d ' ')
  budget_words=$(cat "${budget_files[@]}" | wc -w | tr -d ' ')
fi
# An `@path` import inside a budget file loads with it, so a budget blind to one silently
# under-reports the tier it exists to measure. They are named, never counted: an import resolves
# against the *installed* location of the file carrying it (`CLAUDE.md` is read at `~/.claude/`, not
# from this tree), so the target sits outside `$root` and may be absent on a machine whose owning
# tool never ran. Any extension matches, not just `.md`: `@package.json` is Claude Code's own
# documented example and loads into this same tier.
# Two guards keep `@param`, a package scope and an email address out: an extension is required, and
# the `@` must not follow a word character — the whole difference between `@bitmovin.com` in an
# address and an import. Fields are whitespace-delimited, so each is prefixed with a space to give
# that test something to read at position 1. Fenced blocks and inline code spans
# are prose *about* imports, never imports. `fence` resets per file: one budget file ending inside a
# fence would otherwise blank the scan for every file after it, and the two scripts list the budget in
# different orders, so a single stray fence would blank different files in each and make them disagree.
# Scanned field by field rather than by shrinking the line, for the quadratic reason the
# backticked-path scan above gives: one committed multi-megabyte line otherwise stalls the check.
# Two bounds keep it linear without dropping a real import: a field longer than `PATH_MAX` cannot be
# a path at all, and the match loop stops after 64 hits in one field. `NAME_MAX` (255) is not the
# bound to use here — it limits one *component*, and a nested path well past it opens fine, so a cap
# that low silently hides real imports.
budget_imports=""
if [ ${#budget_files[@]} -gt 0 ]; then
# --- shared:import-scan ---
  budget_imports=$(awk 'FNR == 1 { fence = 0 }
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
                          } }' "${budget_files[@]}" | sort -u)
# --- end shared:import-scan ---
fi
# Capped in bytes, not just in entries. Everything above runs on a fork PR's own files and this line
# rides the exit-0 path, so an uncapped list prints attacker-chosen text under `wiring: clean`; ten
# crafted names alone reach a megabyte, and an entry cap that lets that through while printing "10"
# hides the volume rather than bounding it. The count stays exact; only the naming is trimmed.
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
# only part of a skill no file in the router lists. A skill carrying `disable-model-invocation`
# costs nothing until it is typed, so it stays out of this budget.
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
  # Bounded in line length and in line count before anything is printed. A finding quotes text this
  # script did not choose — a link target, a filename, a cited section — and where the tree under
  # check is someone else's branch, all of it is attacker-written. This output is read by an agent
  # that drafts a PR comment, so an unbounded finding is an unbounded injection surface: 300 crafted
  # link targets alone reach megabytes. Bounding here, on the one path every finding takes, keeps a
  # check added later from reopening it.
  finding_total=$(sort -u "$findings" | wc -l | tr -d ' ')
  sort -u "$findings" | cut -c1-500 | head -200
  [ "$finding_total" -gt 200 ] &&
    echo "… and $((finding_total - 200)) further finding(s), suppressed — fix these and re-run"
  exit 1
fi
echo "wiring: clean"

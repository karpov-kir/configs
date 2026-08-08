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
# every session. Prints nothing when the file has no frontmatter or no description line.
frontmatter_description() {
  awk 'NR == 1 && !/^---[[:space:]]*$/ { exit }
       NR > 1 && /^---[[:space:]]*$/ { exit }
       /^description:[[:space:]]*/ { sub(/^description:[[:space:]]*/, ""); print; exit }' "$1"
}

# True when the skill opted out of model invocation, so its description never enters a context
# window and costs nothing until the human types `/<name>`.
opted_out_of_model_invocation() {
  awk 'NR == 1 && !/^---[[:space:]]*$/ { exit }
       NR > 1 && /^---[[:space:]]*$/ { exit }
       tolower($0) ~ /^disable-model-invocation:[[:space:]]*(true|yes|on|1)[[:space:]]*$/ { found = 1; exit }
       END { exit !found }' "$1"
}

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
canonical_dir() {
  [ -d "$1" ] || return 0
  (cd -P -- "$1" 2>/dev/null && pwd -P)
}

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
    # prose; a heading may not.) The example is deliberately synthetic: a real heading here would be
    # a hand-sync pair nothing checks.
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
  awk '/^```/ { in_fence = !in_fence; next }
       in_fence { next }
       { line = $0
         while (match(line, /`[^`]+`/)) {
           print substr(line, RSTART + 1, RLENGTH - 2)
           line = substr(line, RSTART + RLENGTH)
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
      # Here-string, never `printf … | grep -q`: grep -q exits on the first match, and under
      # `pipefail` the writer's SIGPIPE (141) turns that match into a miss — every heading in a file
      # with more than a pipe buffer of them would then report dangling.
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

# The always-loaded budget: the root CLAUDE.md every system prompt carries, inject.md, and every doc
# it lists under "Read always".
budget_files=()
[ -f "$root/CLAUDE.md" ] && budget_files+=("$root/CLAUDE.md")
budget_files+=("$flavor/inject.md")
# A listed doc that does not exist is skipped by name, not fed to `cat`: a bare `cat:` error explains
# nothing, and counting the file anyway makes the printed file total disagree with what was measured.
# The dangling-link check reports the reference itself; this keeps the budget honest about its inputs.
while IFS= read -r doc; do
  if [ -f "$flavor/$doc" ]; then
    budget_files+=("$flavor/$doc")
  else
    echo "inject.md lists '$doc' under Read always, but $flavor/$doc does not exist" >>"$findings"
  fi
done < <(sed -n '/^## Read always/,/^## /p' "$flavor/inject.md" | grep -oE '\]\([^)#]+\)' |
  sed 's/^](//; s/)$//')
budget_lines=$(cat "${budget_files[@]}" | wc -l | tr -d ' ')
budget_words=$(cat "${budget_files[@]}" | wc -w | tr -d ' ')
echo "always-loaded: $budget_lines lines, $budget_words words across ${#budget_files[@]} files"

# Every skill's description loads in every session too: the same tier, held to the same bar, and the
# only part of a skill no file in the router lists. A skill carrying `disable-model-invocation`
# costs nothing until it is typed, so it stays out of this budget.
description_words=0
description_count=0
skill_total=0
for skill_file in "$skills"/*/SKILL.md; do
  [ -f "$skill_file" ] || continue
  skill_total=$((skill_total + 1))
  opted_out_of_model_invocation "$skill_file" && continue
  description_count=$((description_count + 1))
  description_words=$((description_words + $(frontmatter_description "$skill_file" | wc -w)))
done
echo "always-loaded: $description_words words of skill description across $description_count of $skill_total skills"

if [ -s "$findings" ]; then
  sort -u "$findings"
  exit 1
fi
echo "wiring: clean"

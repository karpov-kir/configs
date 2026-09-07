#!/usr/bin/env bash
#
# Which skills an install mounts, and the question that decides it: who each one is for, read from
# its own frontmatter. Both installers ask it of every skill they find, which is why neither carries
# a list of names.
#
# Sourced, never executed, and only after lib/mount.sh — `add_skill_mounts` declares its mounts
# through that file's `add_bulk` and words its one refusal through `refuse`. Sourcing it first leaves
# those undefined and the first call exits 127.
#
# `add_skill_mounts` is the whole surface; the two frontmatter readers below are its internals.
#
# tested by: the two installers' suites, ai/bootstrap-test.sh and ai/install-project-test.sh, because
# an audience is only real once an install has acted on it.
set -uo pipefail

# Every skill under <skills directory>, declared as a mount under <mount parent directory>. Discovery
# rather than a list: a skill added tomorrow is mounted without anyone editing either installer.
#
# Sets skills_found, skipped_count and skipped_names for the caller to report on, fresh on each call.
# Finding no skill at all mounts nothing in silence, which is why each caller refuses on an empty
# table and reads skills_found to say which of the two ways it got there.
#
# A maintainer-only skill is left out unless `--maintainer` asked for it, and an uninstall takes them
# whatever the flags say. The tier a machine or a project was installed with is nowhere on disk, so
# filtering on an uninstall would build a removal table for the tier being asked for now rather than
# the one that wrote the mounts — `--maintainer` in, plain out, and the marked skills stay mounted
# while the run reports ok. `unlink_mount` removes only a symlink resolving under the checkout, so
# widening the table cannot reach anything this checkout did not write.
add_skill_mounts() { # <skills directory> <mount parent directory> <maintainer: true|false> <uninstall: true|false>
  local skills_dir="$1" mount_parent="$2" maintainer_tier="$3" uninstalling="$4"
  local dir skill_dir name bad_audience take_maintainer_only=false
  if $maintainer_tier || $uninstalling; then
    take_maintainer_only=true
  fi
  skills_found=0
  skipped_count=0
  skipped_names=""
  for dir in "$skills_dir"/*/; do
    [ -d "$dir" ] || continue
    # `%/` first: `##*/` on a path ending in `/` returns nothing, pointing every skill at one target.
    skill_dir="${dir%/}"
    name="${skill_dir##*/}"
    skills_found=$((skills_found + 1))
    # Asked whatever the flags say: a marker nothing reads is wrong on a maintainer's machine too, and
    # the run that installs it is the last moment anyone looks at that line. Mounting continues, so the
    # tree behaves as it does today and the non-zero exit is what carries the news.
    if bad_audience="$(unknown_audience "$skill_dir/SKILL.md")"; then
      refuse "$name declares 'audience: $bad_audience' in $skill_dir/SKILL.md, which no reader knows — the only value is 'audience: maintainer', and as written the skill installs for everyone"
    fi
    if ! $take_maintainer_only && is_maintainer_only "$skill_dir/SKILL.md"; then
      skipped_count=$((skipped_count + 1))
      skipped_names="$skipped_names $name"
      continue
    fi
    add_bulk "$skill_dir" "$mount_parent/$name"
  done
}

is_maintainer_only() { # <SKILL.md>
  [ -r "$1" ] || return 1
  awk '
    NR == 1 { if ($0 !~ /^---[[:space:]]*$/) exit; next }
    /^---[[:space:]]*$/ { closed = 1; exit }
    tolower($0) ~ /^audience:[[:space:]]*maintainer[[:space:]]*$/ { found = 1 }
    END { if (closed && found) exit 0; exit 1 }
  ' "$1"
}

unknown_audience() { # <SKILL.md>, prints the value and exits 0 when there is one
  [ -r "$1" ] || return 1
  awk '
    NR == 1 { if ($0 !~ /^---[[:space:]]*$/) exit; next }
    /^---[[:space:]]*$/ { closed = 1; exit }
    tolower($0) ~ /^audience:/ && tolower($0) !~ /^audience:[[:space:]]*maintainer[[:space:]]*$/ {
      # The first one only, and the raw text rather than the lowered line: it is echoed back to
      # whoever typed it, and a reader hunting `Maintainr` should find what they wrote.
      if (!found) { found = 1; value = substr($0, index($0, ":") + 1) }
    }
    END {
      if (!closed || !found) exit 1
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
      print value
      exit 0
    }
  ' "$1"
}

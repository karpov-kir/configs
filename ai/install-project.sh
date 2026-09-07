#!/usr/bin/env bash
#
# Install this repository's skills into one project, rather than into the machine.
#
#   usage: ai/install-project.sh [--dry-run] [--relocate] [--maintainer] [--uninstall] <project>
#
# The machine-wide half — `~/.kk-flavor`, the Go tools, the MCP servers, jq — is `ai/bootstrap.sh`'s
# and is not repeated here. Run that once, then this once per project. Both are safe to re-run: a
# second run over a finished project reports "ok" throughout and writes nothing.
#
# What lands in the project: a symlink per skill under `<project>/.claude/skills/`, the ignore rules
# that keep those symlinks out of the project's history, and a short region in `<project>/CLAUDE.md`
# pointing at the flavor bucket. The skills are mounted rather than copied, so every project on this
# machine reads one tree and an update reaches all of them at once.
#
# Why this is a separate script rather than a `--project` flag on ai/bootstrap.sh: that script's
# `--skip-brew`, `--skip-tools`, `--skip-mcp` and `--skip-verify` all name machine-wide steps, and a
# project run performs none of them. Adding the flag would leave four published options whose meaning
# depends on a fifth, which is a shallower interface than two scripts over one library.
#
# It sits at ai/ rather than in a subdirectory, and that is load-bearing rather than tidy.
# lib/mount.sh's second-checkout guard finds a foreign checkout by looking for a file of THIS
# script's name at the same depth under the root it computed. One directory deeper and the guard
# finds nothing, reports "no mount comes from another checkout", and repoints every mount it was
# built to protect.
#
# tested by: install-project-test.sh
set -uo pipefail

repo="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
script_name="$(basename -- "${BASH_SOURCE[0]}")"
label="ai project install"

dry_run=false
relocate=false
maintainer=false
uninstall=false
project=""

for arg in "$@"; do
  case "$arg" in
    --dry-run) dry_run=true ;;
    --relocate) relocate=true ;;
    --maintainer) maintainer=true ;;
    --uninstall) uninstall=true ;;
    -h | --help)
      sed -n '3,5p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    -*)
      printf 'ai/install-project.sh: unknown option %s\n' "$arg" >&2
      exit 2
      ;;
    *)
      if [ -n "$project" ]; then
        printf 'ai/install-project.sh: one project at a time — got %s and %s\n' "$project" "$arg" >&2
        exit 2
      fi
      project="$arg"
      ;;
  esac
done

[ -n "$project" ] || {
  printf 'ai/install-project.sh: name the project directory to install into\n' >&2
  exit 2
}

# Resolved before anything is written, so every path below and the registry entry all name the same
# directory however the caller spelled it. A project that is not there is refused rather than created:
# this installs into a repository someone already has.
#
# The spelling is kept, because the assignment below lands whether or not the substitution succeeded:
# reading `$project` in the refusal reads the empty string it just became. `$1` is not it either — the
# project can follow a flag, and then `$1` is the flag and the message names the wrong thing.
project_as_typed="$project"
project="$(CDPATH= cd -P -- "$project" 2>/dev/null && pwd -P)" || {
  printf 'ai/install-project.sh: %s is not a directory — nothing was written\n' "$project_as_typed" >&2
  exit 2
}

for lib in mount.sh owned-region.sh install-registry.sh skill-audience.sh; do
  [ -r "$repo/../lib/$lib" ] || {
    printf 'ai/install-project.sh: lib/%s is missing from this checkout — ai/ and lib/ install together, and nothing was written\n' "$lib" >&2
    exit 2
  }
done
# shellcheck source=../lib/mount.sh
. "$repo/../lib/mount.sh"
# owned-region and install-registry report through mount.sh's say/refuse, so they are sourced after it.
# shellcheck source=../lib/owned-region.sh
. "$repo/../lib/owned-region.sh"
# shellcheck source=../lib/install-registry.sh
. "$repo/../lib/install-registry.sh"
# shellcheck source=../lib/skill-audience.sh
. "$repo/../lib/skill-audience.sh"

bulk_label="skills"
mount_scope_label="$project's skills"

# --- what this install is made of ------------------------------------------------------------------

claude_md="$project/CLAUDE.md"
gitignore="$project/.gitignore"

region_open="<!-- kk-flavor:begin -->"
region_close="<!-- kk-flavor:end -->"

region_body() {
  cat <<'BODY'
### KK Flavor

Read `~/.kk-flavor/inject.md` now and follow it — applies to all work, skill-invoked or ad-hoc.
BODY
}

ignore_open="# kk-flavor:begin"
ignore_close="# kk-flavor:end"

ignore_body() {
  cat <<'BODY'
.claude/skills/kk-*
.claude/skills/idsd-*
BODY
}

# The branch for a project with no .gitignore at all. region_write handles the file that exists; this
# is the one path that creates one, and it makes the checks region_writable would otherwise have made.
create_ignore_file() {
  if $dry_run; then
    say "  would create $gitignore with the skill ignore rules"
    return 0
  fi
  # A dangling symlink answers "not there" to the `-e` at the call site, and `>` follows it — so
  # without this the branch truncates and rewrites whatever the link names, anywhere the installer's
  # user can write.
  if [ -L "$gitignore" ]; then
    refuse "$gitignore is a symlink, and this writes the file itself — repoint or remove it, then re-run"
    return 1
  fi
  {
    printf '%s\n' "$ignore_open"
    ignore_body
    printf '%s\n' "$ignore_close"
  } >"$gitignore" || {
    refuse "could not create $gitignore — the skill mounts are not ignored and will show up in this project's history"
    return 1
  }
  say "  created  $gitignore with the skill ignore rules"
}

# --- the mount table -------------------------------------------------------------------------------

# Shared by install and uninstall, which is the whole reason uninstall is a mode of this file rather
# than a script of its own: a second script re-deriving this table drifts from what was installed,
# and drifts in the direction nobody notices — leaving things behind and reporting ok.

skills_found=0
skipped_count=0
skipped_names=""
for dir in "$repo"/kk-flavor/skills/*/; do
  [ -d "$dir" ] || continue
  skill_dir="${dir%/}"
  skills_found=$((skills_found + 1))
  if bad_audience="$(unknown_audience "$skill_dir/SKILL.md")"; then
    refuse "${skill_dir##*/} declares 'audience: $bad_audience' in $skill_dir/SKILL.md, which no reader knows — the only value is 'audience: maintainer', and as written the skill installs for everyone"
  fi
  # Maintainer-only skills are for maintaining this instruction tree and do nothing for a project
  # that merely uses it, so a project install leaves them out unless asked.
  #
  # Not on an uninstall. The tier a project was installed with is nowhere on disk, so filtering here
  # builds a removal table for the tier being asked for now rather than the one that wrote the mounts —
  # `--maintainer` in, plain out, and the marked skills stay mounted while the run reports ok and
  # forgets the project, so nothing ever names them again. `unlink_mount` removes only a symlink
  # resolving under $repo, so widening the table cannot reach anything this checkout did not write.
  if ! $maintainer && ! $uninstall && is_maintainer_only "$skill_dir/SKILL.md"; then
    skipped_count=$((skipped_count + 1))
    skipped_names="$skipped_names ${skill_dir##*/}"
    continue
  fi
  add_bulk "$skill_dir" "$project/.claude/skills/${skill_dir##*/}"
done

if [ "${#bulk_targets[@]}" -eq 0 ]; then
  if [ "$skills_found" -gt 0 ]; then
    refuse "every skill under $repo/kk-flavor/skills/ is maintainer-only and none was selected — nothing was mounted"
  else
    refuse "no skill directories under $repo/kk-flavor/skills/ — nothing was mounted"
  fi
  report_and_exit
fi

say "$label: $project"

# --- uninstall --------------------------------------------------------------------------------------

if $uninstall; then
  unmount_run
  say "project files"
  region_remove "$claude_md" "$region_open" "$region_close"
  # The ignore rules go with the mounts they were hiding. Only our own fenced region, so a rule the
  # human wrote is not swept up with it.
  [ -e "$gitignore" ] && region_remove "$gitignore" "$ignore_open" "$ignore_close"
  say "registry"
  registry_forget "$project"

  remaining="$(registry_live | grep -c . || true)"
  if [ "$remaining" -eq 0 ]; then
    say ""
    say "  No project on this machine is installed any more. ~/.kk-flavor and the machine-wide"
    say "  steps are still in place; remove them with ai/bootstrap.sh --uninstall if you are done."
  else
    say ""
    say "  $remaining other project(s) still use ~/.kk-flavor, so nothing machine-wide was touched."
  fi
  report_and_exit
fi

# --- install -----------------------------------------------------------------------------------------

if ! $maintainer && [ "$skipped_count" -gt 0 ]; then
  say "  skipped  $skipped_count maintainer-only skill(s):$skipped_names"
fi

# A project that already ignores `.claude/` wholesale needs a human, not an appended rule: our lines
# would be redundant there, and removing them on uninstall would say we had un-ignored something we
# had not. Reported and skipped, which is what the rest of this tree does with a target it does not
# own.
broad_claude_rule=""
if [ -f "$gitignore" ]; then
  broad_claude_rule="$(grep -nE '^[[:space:]]*/?\.claude/?[[:space:]]*$' "$gitignore" | head -1)"
fi

mount_run

say "project files"
if [ -n "$broad_claude_rule" ]; then
  refuse "$gitignore already ignores .claude/ wholesale (line ${broad_claude_rule%%:*}) — that covers this install and the project's own settings alike, so no rule was added; decide it yourself"
elif [ -e "$gitignore" ]; then
  region_write "$gitignore" "$ignore_open" "$ignore_close" "$(ignore_body)"
else
  create_ignore_file
fi

# CLAUDE.md is the project's own file and may not exist yet. Created empty first when absent, because
# owned-region refuses to create a file and is right to: a typo in a path should not scatter new files
# through someone's repository, but the path here was resolved above and is the project root.
# `! -L` beside `! -e`, the form ai/bootstrap.sh already uses: `-e` follows a symlink, so a DANGLING
# one at this path answers "does not exist" and the redirect below would then create the file the link
# names — anywhere on disk the installer's user can write. region_write refuses a symlinked target,
# but only after this branch has already created it.
if [ ! -e "$claude_md" ] && $dry_run; then
  say "  would create $claude_md and add the kk-flavor region"
else
  if [ ! -e "$claude_md" ] && [ ! -L "$claude_md" ]; then
    : >"$claude_md" || refuse "could not create $claude_md"
  fi
  region_write "$claude_md" "$region_open" "$region_close" "$(region_body)"
fi

say "registry"
registry_record "$project"

report_and_exit

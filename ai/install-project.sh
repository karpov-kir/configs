#!/usr/bin/env bash
#
# Install this repository's skills into one project, rather than into the machine.
#
#   usage: ai/install-project.sh --agent=claude|codex [--dry-run] [--relocate] [--maintainer] [--uninstall] <project>
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
agent=""

for arg in "$@"; do
  case "$arg" in
    --agent=codex | --agent=claude) agent="${arg#*=}" ;;
    --agent=*)
      printf 'ai/install-project.sh: unknown agent %s — use --agent=codex|claude\n' "${arg#*=}" >&2
      exit 2
      ;;
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

[ -n "$agent" ] || {
  printf 'ai/install-project.sh: --agent=claude|codex is required — nothing was changed\n' >&2
  exit 2
}

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

for lib in mount.sh owned-region.sh install-registry.sh skill-audience.sh flavor-region.sh; do
  [ -r "$repo/../lib/$lib" ] || {
    printf 'ai/install-project.sh: lib/%s is missing from this checkout — ai/ and lib/ install together, and nothing was written\n' "$lib" >&2
    exit 2
  }
done
# shellcheck source=../lib/mount.sh
. "$repo/../lib/mount.sh"
# After mount.sh: all but flavor-region.sh reach its say, refuse and add_bulk.
# shellcheck source=../lib/owned-region.sh
. "$repo/../lib/owned-region.sh"
# shellcheck source=../lib/install-registry.sh
. "$repo/../lib/install-registry.sh"
# shellcheck source=../lib/skill-audience.sh
. "$repo/../lib/skill-audience.sh"
# shellcheck source=../lib/flavor-region.sh
. "$repo/../lib/flavor-region.sh"

bulk_label="skills"
mount_scope_label="$project's skills"

# --- what this install is made of ------------------------------------------------------------------

case "$agent" in
  claude)
    agent_directory=".claude"
    other_agent_directory=".agents"
    ignore_open="# kk-flavor:begin"
    ignore_close="# kk-flavor:end"
    ;;
  codex)
    agent_directory=".agents"
    other_agent_directory=".claude"
    ignore_open="# kk-flavor-codex:begin"
    ignore_close="# kk-flavor-codex:end"
    ;;
esac
instructions_md="$project/AGENTS.md"
claude_md="$project/CLAUDE.md"
gitignore="$project/.gitignore"

ignore_body() {
  printf '%s/skills/kk-*\n%s/skills/idsd-*\n' "$agent_directory" "$agent_directory"
}

other_agent_mounted() {
  local target current resolved
  for target in "$project/$other_agent_directory/skills/"*; do
    [ -L "$target" ] || continue
    current="$(readlink "$target")"
    [ "${current#/}" != "$current" ] || continue
    resolved="$(CDPATH= cd -P -- "$(dirname -- "$current")" 2>/dev/null && pwd -P)" || continue
    case "$resolved" in
      "$repo" | "$repo"/*) return 0 ;;
    esac
  done
  return 1
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

# Maintainer-only skills are for maintaining this instruction tree and do nothing for a project that
# merely uses it, so a project install leaves them out unless asked. An uninstall takes them anyway,
# and lib/skill-audience.sh carries why: here the cost of leaving them behind is that the run also
# forgets the project, so nothing ever names them again.
add_skill_mounts "$repo/kk-flavor/skills" "$project/$agent_directory/skills" "$maintainer" "$uninstall"

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
  if other_agent_mounted; then
    say "  kept     shared project instructions: another client still uses them"
  else
    for file in "$instructions_md" "$claude_md"; do
      if [ -e "$file" ] || [ -L "$file" ]; then
        region_remove "$file" "$flavor_region_open" "$flavor_region_close"
      fi
    done
  fi
  # The ignore rules go with the mounts they were hiding. Only our own fenced region, so a rule the
  # human wrote is not swept up with it.
  [ -e "$gitignore" ] && region_remove "$gitignore" "$ignore_open" "$ignore_close"
  say "registry"
  if other_agent_mounted; then
    say "  kept     $project still has $other_agent_directory skill mounts from this checkout"
  else
    registry_forget "$project"
  fi

  remaining="$(registry_live | grep -c . || true)"
  if [ "$remaining" -eq 0 ]; then
    say ""
    say "  No project on this machine is installed any more. ~/.kk-flavor and the machine-wide"
    say "  steps are still in place; remove them with ai/bootstrap.sh --agent=$agent --uninstall if you are done."
  else
    say ""
    say "  $remaining project(s) still use ~/.kk-flavor, so nothing machine-wide was touched."
  fi
  report_and_exit
fi

# --- install -----------------------------------------------------------------------------------------

if ! $maintainer && [ "$skipped_count" -gt 0 ]; then
  say "  skipped  $skipped_count maintainer-only skill(s):$skipped_names"
fi

broad_agent_rule=""
if [ -f "$gitignore" ]; then
  broad_agent_rule="$(grep -nE "^[[:space:]]*/?\\$agent_directory/?[[:space:]]*$" "$gitignore" | head -1)"
fi

if [ "$agent" = codex ] && [ -s "$project/AGENTS.override.md" ]; then
  refuse "$project/AGENTS.override.md shadows AGENTS.md — merge its instructions before installing"
  report_and_exit
fi
for file in "$instructions_md" "$claude_md"; do
  if [ -e "$file" ] || [ -L "$file" ]; then
    region_writable "$file" || report_and_exit
    if [ "$(region_state "$file" "$flavor_region_open" "$flavor_region_close")" = conflict ]; then
      refuse "$file holds an incomplete kk-flavor region — nothing was installed"
      report_and_exit
    fi
  fi
done

add_unmount_scan "$project/$agent_directory/skills" "$repo/kk-flavor/skills"
mount_run

say "project files"
if [ -n "$broad_agent_rule" ]; then
  refuse "$gitignore already ignores $agent_directory/ wholesale (line ${broad_agent_rule%%:*}) — that covers this install and the project's own settings alike, so no rule was added; decide it yourself"
elif [ -e "$gitignore" ]; then
  region_write "$gitignore" "$ignore_open" "$ignore_close" "$(ignore_body)"
else
  create_ignore_file
fi

write_project_instructions() { # <file> <body>
  local file="$1" body="$2"
  if [ ! -e "$file" ] && $dry_run; then
    say "  would create $file and add the kk-flavor region"
    return 0
  fi
  if [ ! -e "$file" ] && [ ! -L "$file" ]; then
    : >"$file" || { refuse "could not create $file"; return 1; }
  fi
  region_write "$file" "$flavor_region_open" "$flavor_region_close" "$body"
}

if write_project_instructions "$instructions_md" "$(flavor_region_body)"; then
  write_project_instructions "$claude_md" '@AGENTS.md'
fi

say "registry"
registry_record "$project"

report_and_exit

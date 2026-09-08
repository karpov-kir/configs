#!/usr/bin/env bash
# Restore installed project skills after checkout. Never installs dependencies or instructions.
# Usage: bash ai/project-skills.sh --sync <worktree>
# tested by: project-skills-test.sh

project_git_paths() {
  project_root=$(git -C "$project" rev-parse --show-toplevel 2>/dev/null) || return 1
  project_common=$(git -C "$project" rev-parse --path-format=absolute --git-common-dir) || return 1
  project_common=$(real_dir "$project_common") || return 1
  project_state="$project_common/kk-flavor"
  project_hook="$project_common/hooks/post-checkout"
}

project_hook_body() {
  printf '%s\n' '#!/usr/bin/env bash' '# kk-flavor project skills' 'exec bash "$HOME/.kk-flavor/../project-skills.sh" --sync .'
}

project_storage_writable() {
  local path
  for path in "$project_state" "$project_state/$agent" "$project_common/info" "$project_common/info/exclude" "$project_common/hooks"; do
    if [ -L "$path" ]; then
      refuse "$path is a symlink — worktree setup was left alone"
      return 1
    fi
  done
  for path in "$project_state/$agent" "$project_common/info/exclude" "$project_hook"; do
    if [ -e "$path" ] && [ ! -L "$path" ]; then
      region_writable "$path" || return 1
    fi
  done
}

project_skills_writable() {
  local parent
  for parent in "$project/$agent_directory" "$project/$agent_directory/skills"; do
    if [ -L "$parent" ]; then
      refuse "$parent is a symlink — move it aside before restoring project skills"
      return 1
    fi
  done
}

project_verify_tree() {
  local physical_root actual_root actual_common physical_home
  physical_root=$(real_dir "$project") || return 1
  physical_home=$(real_dir "$HOME") || return 1
  if [ "$physical_root" = "$physical_home" ]; then
    refuse "$project is the user home — project skill sync cannot enable user-level skills"
    return 1
  fi
  actual_root=$(env -u GIT_DIR -u GIT_WORK_TREE -u GIT_COMMON_DIR git -C "$project" rev-parse --show-toplevel 2>/dev/null) &&
    actual_root=$(real_dir "$actual_root") &&
    actual_common=$(env -u GIT_DIR -u GIT_WORK_TREE -u GIT_COMMON_DIR git -C "$project" rev-parse --path-format=absolute --git-common-dir 2>/dev/null) &&
    actual_common=$(real_dir "$actual_common") || {
      refuse "$project is not a readable Git worktree — no skills were changed"
      return 1
    }
  if [ "$actual_root" != "$physical_root" ] || [ "$actual_common" != "$project_common" ]; then
    refuse "$project is not a worktree of $project_common — no skills were changed"
    return 1
  fi
}

project_sync_tree() (
  project="$1"
  agent="$2"
  maintainer="$3"
  agent_directory=.claude
  [ "$agent" != codex ] || agent_directory=.agents
  refusals=()
  bulk_sources=(); bulk_targets=(); cfg_sources=(); cfg_targets=()
  unmount_dirs=(); unmount_roots=(); foreign_roots=()
  project_verify_tree || return 1
  project_skills_writable || return 1
  add_skill_mounts "$repo/kk-flavor/skills" "$project/$agent_directory/skills" "$maintainer" "$uninstall"
  if [ "${#bulk_targets[@]}" -eq 0 ]; then
    refuse "no project skills found under $repo/kk-flavor/skills"
    return 1
  fi
  if $uninstall; then
    unmount_run
  else
    # Check legacy checkout mounts before rewriting their source spelling through the bucket.
    (dry_run=true; mount_run; [ "${#refusals[@]}" -eq 0 ]) >/dev/null || { (dry_run=true; mount_run); exit 1; }
    for ((i=0; i<${#bulk_sources[@]}; i++)); do
      bulk_sources[i]="$HOME/.kk-flavor/skills/${bulk_sources[i]##*/}"
    done
    add_unmount_scan "$project/$agent_directory/skills" "$repo/kk-flavor/skills"
    if $dry_run; then
      for ((i=0; i<${#bulk_targets[@]}; i++)); do
        say "  would link ${bulk_targets[i]} -> ${bulk_sources[i]}"
      done
    else
      mount_run
    fi
  fi
  [ "${#refusals[@]}" -eq 0 ]
)

project_sync_all() {
  local worktree="" field inventory is_bare=false is_prunable=false
  inventory=$(mktemp) || { refuse "could not prepare worktree listing"; return 1; }
  if ! git -C "$project" worktree list --porcelain -z > "$inventory"; then
    rm -f -- "$inventory"
    refuse "could not list worktrees for $project"
    return 1
  fi
  while IFS= read -r -d '' field; do
    case "$field" in
      'worktree '*) worktree="${field#worktree }"; is_bare=false; is_prunable=false; continue ;;
      bare) is_bare=true; continue ;;
      prunable*) is_prunable=true; continue ;;
      '') ;;
      *) continue ;;
    esac
    { $is_bare || $is_prunable; } && continue
    [ -d "$worktree" ] && [ "$worktree" != "$project" ] || continue
    if ! project_sync_tree "$worktree" "$agent" "$maintainer"; then
      refuse "could not update $agent skills in $worktree"
    fi
  done < "$inventory"
  rm -f -- "$inventory"
}

project_enable_worktrees() {
  local exclude can_hook=true
  project_git_paths || return 0
  project_storage_writable || return 1
  if git -C "$project" config --get core.hooksPath >/dev/null || [ -L "$project_hook" ] || { [ -e "$project_hook" ] && [ "$(cat "$project_hook")" != "$(project_hook_body)" ]; }; then
    refuse "existing Git hooks were preserved; add bash \"$repo/project-skills.sh\" --sync . to your post-checkout hook, or run it in each new worktree"
    can_hook=false
  fi
  project_sync_all || return 1
  if $dry_run; then
    $can_hook && say "  would enable $agent skill links for future worktrees"
    return 0
  fi
  mkdir -p "$project_state" || { refuse "could not create $project_state"; return 1; }
  printf '%s\n' "$maintainer" > "$project_state/$agent" || { refuse "could not record $agent worktree setup"; return 1; }
  exclude="$project_common/info/exclude"
  mkdir -p "${exclude%/*}" || { refuse "could not create ${exclude%/*}"; return 1; }
  [ -e "$exclude" ] || : > "$exclude"
  region_write "$exclude" "$ignore_open" "$ignore_close" "$(ignore_body)"
  $can_hook || return 1
  mkdir -p "${project_hook%/*}" || { refuse "could not create ${project_hook%/*}"; return 1; }
  project_hook_body > "$project_hook" && chmod +x "$project_hook" || {
    refuse "could not enable $project_hook"
    return 1
  }
}

project_disable_worktrees() {
  project_git_paths || return 0
  project_storage_writable || return 1
  project_sync_all
  if $dry_run; then
    say "  would disable $agent skill links for future worktrees"
    return 0
  fi
  if [ -f "$project_state/$agent" ] && [ ! -L "$project_state/$agent" ]; then
    rm -- "$project_state/$agent" || return 1
  fi
  if [ -f "$project_common/info/exclude" ]; then
    region_remove "$project_common/info/exclude" "$ignore_open" "$ignore_close"
  fi
  if [ ! -e "$project_state/claude" ] && [ ! -e "$project_state/codex" ]; then
    if [ -f "$project_hook" ] && [ ! -L "$project_hook" ] && [ "$(cat "$project_hook")" = "$(project_hook_body)" ]; then
      rm -- "$project_hook" || return 1
    fi
    rmdir "$project_state" 2>/dev/null || true
  fi
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
  set -uo pipefail
  [ "$#" -eq 2 ] && [ "$1" = --sync ] || { printf 'Usage: %s --sync <worktree>\n' "$0" >&2; exit 2; }
  repo=$(CDPATH= cd -P -- "$(dirname -- "$0")" && pwd -P)
  script_name=install-project.sh
  label='project skill sync'
  project="$2"
  dry_run=false; relocate=false; uninstall=false
  . "$repo/../lib/mount.sh"
  . "$repo/../lib/skill-audience.sh"
  project_git_paths || { printf '%s is not a Git worktree\n' "$project" >&2; exit 1; }
  project="$project_root"
  result=0
  for agent in claude codex; do
    [ -f "$project_state/$agent" ] || continue
    maintainer=$(cat "$project_state/$agent")
    case "$maintainer" in true|false) ;; *) printf 'Invalid project skill setup: %s\n' "$project_state/$agent" >&2; exit 1;; esac
    project_sync_tree "$project" "$agent" "$maintainer" || result=1
  done
  exit "$result"
fi

#!/usr/bin/env bash
#
# Which projects this machine installed the agent tree into. One fact, written by the project
# installer and read by both uninstallers — the project one to drop its own entry, the machine-wide
# one to say whether removing `~/.kk-flavor` would pull the bucket out from under a project that is
# still using it.
#
# Sourced, never executed, and only after lib/mount.sh: it reports through that file's `say` and
# `refuse` and honours its `$dry_run`.
#
# The path follows the convention `ecosystem.md` → **Conventions a new file joins** already sets for a
# machine-local file, and that ai/tools/bloat-judge/deadline.go already reads:
# `${XDG_CONFIG_HOME:-~/.config}/kk-flavor/`. `$HOME` and `$XDG_CONFIG_HOME` are read from the
# environment and never `~`-expanded, which is the whole of what lets the suites point this at a
# throwaway home.
#
# It does not know what a mount is. It prunes on one condition — the recorded directory is gone —
# because that is the only staleness a registry can detect on its own. "Recorded but no longer holds
# the mounts" is the caller's filter over what `registry_live` prints, since only the caller knows
# what its own install put there.
#
# tested by: install-registry-test.sh
set -uo pipefail

registry_file() {
  printf '%s/kk-flavor/installs\n' "${XDG_CONFIG_HOME:-$HOME/.config}"
}

# The recorded projects that still exist, one per line, having rewritten the file to drop the ones
# that do not.
#
# Pruning and reading are one call rather than two on purpose. A caller able to read without pruning
# is a caller able to get the stale answer this exists to prevent — and the stale answer here is the
# one that says "another project still needs the bucket" about a directory the human deleted months
# ago, which makes uninstall refuse to finish for a reason that is not true any more.
#
# A missing registry is not an error: it is a machine that has installed into no project, which is
# every machine before the first project install and every machine that only ever installed
# machine-wide.
registry_live() {
  local file kept line
  file="$(registry_file)"
  [ -f "$file" ] || return 0

  kept=""
  while IFS= read -r line || [ -n "$line" ]; do
    # A blank line or a comment is not an entry, so it is not printed — but it is kept, because a
    # human who opens this file to see what is in it may well annotate it, and eating their note
    # while pruning dead projects is a poor answer to a question they did not ask.
    case "$(printf '%s' "$line" | sed 's/^[[:space:]]*//')" in
      '' | '#'*)
        kept="$kept$line
"
        continue
        ;;
    esac
    [ -d "$line" ] || continue
    kept="$kept$line
"
  done <"$file"

  # Only the live project lines are printed; the comments above are kept in the file and dropped
  # here, so a caller counting entries never counts somebody's annotation as a project.
  printf '%s' "$kept" | grep -v '^[[:space:]]*#' | grep -v '^[[:space:]]*$' || true

  # Rewritten only when it would change, so a read on a healthy registry writes nothing at all and a
  # dry run never has to be special-cased here.
  if [ "$kept" != "$(cat "$file")" ] && ! $dry_run; then
    printf '%s' "$kept" >"$file.tmp" && mv -f -- "$file.tmp" "$file"
  fi
}

# Record a project. Idempotent: a second install into the same directory leaves one line.
registry_record() { # <project directory>
  local project="$1" file dir
  file="$(registry_file)"
  dir="${file%/*}"

  if registry_live | grep -qxF -- "$project"; then
    say "  ok       $project is already recorded in $file"
    return 0
  fi
  if $dry_run; then
    say "  would record $project in $file"
    return 0
  fi
  [ -d "$dir" ] || mkdir -p -- "$dir" || {
    refuse "could not create $dir, so $project was not recorded — uninstall will not know about it"
    return 1
  }
  printf '%s\n' "$project" >>"$file" || {
    refuse "could not record $project in $file — uninstall will not know about it"
    return 1
  }
  say "  recorded $project in $file"
}

# Drop a project. Absent is success: an uninstall run twice has nothing to do the second time.
registry_forget() { # <project directory>
  local project="$1" file kept
  file="$(registry_file)"
  [ -f "$file" ] || {
    say "  ok       nothing recorded to forget"
    return 0
  }
  if ! registry_live | grep -qxF -- "$project"; then
    say "  ok       $project was not recorded"
    return 0
  fi
  if $dry_run; then
    say "  would forget $project in $file"
    return 0
  fi
  # Read from the file rather than from registry_live, whose output is projects only: rewriting from
  # that would drop the annotations the read above deliberately preserves.
  kept="$(grep -vxF -- "$project" "$file")"
  # `grep -v` exits 1 when it prints nothing, which is exactly the case where the last project is
  # being forgotten — so the status is not checked here, and the write below is what can fail.
  printf '%s' "${kept:+$kept
}" >"$file.tmp" && mv -f -- "$file.tmp" "$file" || {
    refuse "could not rewrite $file — $project is still recorded"
    return 1
  }
  say "  forgot   $project in $file"
}

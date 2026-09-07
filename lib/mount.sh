#!/usr/bin/env bash
#
# The mounting machinery both bootstrap scripts run on: link a source in this repository at a target
# in $HOME, refuse rather than delete anything it did not write itself, drop its own mounts once this
# checkout has no source for them, and stop before writing anything when this machine's config is
# already mounted from a different checkout.
#
# Sourced, never executed. Before sourcing, a caller sets:
#
#   repo         the directory its own bootstrap.sh lives in — env/ or ai/. Every source path is
#                built from it, and the second-checkout guard recognises a stranger by finding a file
#                named $script_name at the same relative depth under a different root.
#   script_name  the caller's own filename, read from $BASH_SOURCE rather than written down, so a
#                rename cannot leave the guard hunting for a name nothing has.
#   label        what the report line calls the run — "env bootstrap", "ai bootstrap". Two scripts of
#                the same name print here, and a human reading a terminal has to know which answered.
#
# After sourcing it declares its mounts with add_cfg (named one by one when the guard reports) and
# add_bulk (counted, for a homogeneous set like ai/'s skills), sets bulk_label if it uses add_bulk,
# calls add_unmount_scan for any directory it mounts a discovered set into, then calls mount_run.
#
# $HOME is read from the environment and never assumed, which is what lets the suites run the real
# linking logic against a throwaway home instead of faking it.
#
# tested by: env/bootstrap-test.sh, ai/bootstrap-test.sh — through the two scripts that source it,
# because a mount is only real once a bootstrap has written it.

dry_run=${dry_run:-false}
relocate=${relocate:-false}
bulk_label=${bulk_label:-}

# Refusals are collected rather than fatal: a machine missing one cask should still get every link,
# and a human fixing three named problems in one pass beats discovering them one run at a time.
refusals=()

# Every message loses its control bytes on the way out. Half of what these lines quote is text the
# tree chose rather than text this file wrote — a skill directory name off a branch, a symlink value
# read off the machine — and a control byte in one drives the terminal instead of printing. `ESC[2K`
# erases the line it lands in and `ESC[1A` moves to the line above, so a name carrying either can wipe
# the run's own account of what it removed or refused. ai/tools/shell/text.go states that rule for the
# two Go tools; these two scripts print the same kind of text and need it as much.
#
# `[[:cntrl:]]` takes C0 and DEL in every shell and locale measured here. Past DEL the locale decides,
# not this file: a UTF-8 locale decodes first and spaces the C1 range too, while `LC_ALL=C` touches
# nothing above DEL. Multibyte text survives either way — `日本語 café 🍦` came back byte for byte under
# bash 3.2 and 5.3 in both locales — so nothing here shreds a legitimate path.
#
# One byte stays out of reach: a raw 0x9b, the CSI an 8-bit terminal acts on. bash 5.3 leaves it and
# bash 3.2 spaces it only in a UTF-8 locale. Catching it needs the decoding ai/tools/shell/text.go
# does and a substitution cannot, so that byte is the Go side's alone.
say() { printf '%s\n' "${1//[[:cntrl:]]/ }"; }
refuse() {
  local message="${1//[[:cntrl:]]/ }"
  refusals+=("$message")
  printf '  REFUSED  %s\n' "$message"
}

# Where a directory really is, or nothing and a non-zero status. Physical rather than textual: `/var`
# is a symlink to `/private/var` on macOS, so the same directory is spelled two ways and only one of
# them is what this script wrote. `CDPATH=` because a CDPATH set in the environment makes `cd` echo
# the directory it landed on, and the answer comes back two lines long.
real_dir() { # <directory>
  CDPATH= cd -P -- "$1" 2>/dev/null && pwd -P
}

link_value() { # <symlink>
  local value
  value="$(readlink "$1")"
  printf '%s' "${value%/}"
}

# The end of every path through a bootstrap script, including the guard below that stops before the
# first write. A refusal that ended the run its own way would report through something this has no say
# over, and the exit code and the collected list are the whole contract with a caller.
report_and_exit() {
  echo
  if [ "${#refusals[@]}" -eq 0 ]; then
    say "$label: ok"
    exit 0
  fi
  printf '%s: %s thing(s) need you:\n' "$label" "${#refusals[@]}"
  for item in "${refusals[@]}"; do
    printf '  - %s\n' "$item"
  done
  exit 1
}

# --- the mount table ------------------------------------------------------------------------------

# Two lists rather than one, because the guard reports them differently. A config is individually
# consequential — a shell, a git identity, the instructions every agent session loads — so it is named.
# A bulk set is homogeneous, so its count says all a list would.
cfg_sources=()
cfg_targets=()
add_cfg() {
  cfg_sources+=("$1")
  cfg_targets+=("$2")
}

bulk_sources=()
bulk_targets=()
add_bulk() {
  bulk_sources+=("$1")
  bulk_targets+=("$2")
}

# Link `target` at `source`, and report which of the four states it found. The only state that writes
# over something is a symlink, which carries no data of its own — but see the guard below, which is
# the case where that reasoning holds for the link and not for what the link is part of.
link() {
  local source="$1" target="$2" parent current
  if [ ! -e "$source" ]; then
    refuse "$source is missing from the repository, so $target was left alone"
    return 1
  fi
  if [ -L "$target" ]; then
    # Trailing slashes stripped from both sides before comparing. ai/README.md's skills loop linked with
    # `ln -sfn "$d"` for as long as the skills existed, where `$d` came from a `*/` glob, so every
    # machine set up from it holds links that read back as `…/kk-tighten/` while this script computes
    # `…/kk-tighten`. Same directory, different string: comparing raw reports each one as stale and
    # rewrites a correct link on every run, which is idempotence lost to a cosmetic difference.
    #
    # ai/README.md now documents `${d%/}`, so new machines will not have them. This stays for the ones
    # already set up: the readback is a property of links made years ago, not of the current README.
    current="$(link_value "$target")"
    if [ "$current" = "${source%/}" ]; then
      say "  ok       $target"
      return 0
    fi
    if $dry_run; then
      say "  would repoint $target -> $source"
      return 0
    fi
    ln -sfn "$source" "$target" || {
      refuse "could not repoint $target at $source"
      return 1
    }
    say "  repointed $target"
    return 0
  fi
  if [ -e "$target" ]; then
    # The whole reason a bootstrap script is not env/README.md's `rm -rf`.
    refuse "$target exists and is not a symlink — move it aside, then re-run"
    return 1
  fi
  if $dry_run; then
    say "  would link $target -> $source"
    return 0
  fi
  # `%/` first, or `${target%/*}` names a trailing-slash target itself: `mkdir -p` then creates it
  # and `ln -s` links inside it. An empty `$HOME` leaves the expansion empty, hence the `/`.
  parent="${target%/}"
  parent="${parent%/*}"
  [ -n "$parent" ] || parent="/"
  [ -d "$parent" ] || mkdir -p -- "$parent" || {
    refuse "could not create $parent"
    return 1
  }
  ln -s "$source" "$target" || {
    refuse "could not link $target at $source"
    return 1
  }
  say "  linked   $target"
}

# --- mounts this script wrote and no longer has a source for --------------------------------------

# A skill renamed or deleted takes its source directory with it, and nothing in the mount table names
# the old target any more — so `link()` never sees it, and the link left under `~/.claude/skills/`
# resolves into a directory no checkout has. Only the run that would have written it can notice.
#
# It is a deletion, in a library whose header promises it refuses rather than deletes, so the
# signature is narrow enough that only a mount of this checkout's own matches: a symlink, whose value
# is absolute, whose parent directory is the source root the caller named. Everything else is left
# where it is, dangling or not, which is why the summary below claims only what it checked. The
# signature cannot ask who wrote the link: ai/README.md documents a by-hand install loop whose links
# are identical to this script's.
unmount_dirs=()
unmount_roots=()
add_unmount_scan() { # <directory the mounts sit in> <source root a mount of ours comes from>
  unmount_dirs+=("$1")
  unmount_roots+=("$2")
}

# A root this checkout cannot read stops the scan before the loop. The loop would remove nothing there
# in any case; what it would print is a clean bill of health over a directory the run never opened.
unmount_stale() { # <directory> <source root>
  local dir="$1" root="$2" root_real entry value value_dir src has_source=0 stale=0
  root_real="$(real_dir "$root")" || root_real=""
  if [ -z "$root_real" ]; then
    say "  $root cannot be read, so no mount under $dir was checked"
    return 0
  fi
  # A root that resolves and holds nothing is the same hazard one step further in, and the one the loop
  # can act on: every mount of this checkout's dangles at once, so the loop reads the machine's whole
  # set as deleted and takes it. A source root emptied by a half-finished checkout is not a set of
  # deletions anybody made. The caller's own emptiness check cannot stand in for this one — the scan
  # runs inside mount_run, so whatever a caller does about an empty root, it does afterwards.
  for src in "$root_real"/*/; do
    [ -d "$src" ] || continue
    has_source=1
    break
  done
  if [ "$has_source" -eq 0 ]; then
    say "  $root holds no source, so no mount under $dir was checked"
    return 0
  fi
  if [ ! -d "$dir" ]; then
    say "  ok       nothing is mounted at $dir"
    return 0
  fi
  for entry in "$dir"/*; do
    [ -L "$entry" ] || continue
    value="$(link_value "$entry")"
    # Absolute only, for the reason mount_foreign_root gives: a relative value resolves against the
    # link's own directory, so resolving it here would resolve it against the wrong one.
    [ "${value#/}" != "$value" ] || continue
    value_dir="$(real_dir "$(dirname -- "$value")")" || continue
    [ "$value_dir" = "$root_real" ] || continue
    [ ! -e "$value" ] || continue
    stale=$((stale + 1))
    if $dry_run; then
      say "  would remove $entry, which points at $value and nothing is there"
    elif rm -- "$entry"; then
      say "  removed  $entry, which pointed at $value and nothing is there"
    else
      refuse "could not remove $entry, which points at $value and nothing is there"
    fi
  done
  [ "$stale" -eq 0 ] && say "  ok       every mount under $dir this checkout wrote still resolves"
  return 0
}

unmount_run() {
  local i
  [ "${#unmount_dirs[@]}" -eq 0 ] && return 0
  say "stale mounts"
  for ((i = 0; i < ${#unmount_dirs[@]}; i++)); do
    unmount_stale "${unmount_dirs[i]}" "${unmount_roots[i]}"
  done
  return 0
}

# --- is this machine already mounted somewhere else? ----------------------------------------------

# `link()` above treats an existing symlink as safe to write over, on the grounds that a symlink
# carries no data of its own. True of the link. The damage this guards is to the *mount*: when the
# checkout a live link names is real, and is where this machine's config actually lives, that link is
# not stale — this checkout is the stranger, and repointing it moves the human's whole setup here.
# Delete the clone afterwards, which is the entire point of a scratch clone, and their next login has
# no .zshrc, no .gitconfig and no agent instructions. The script whose header promises it refuses
# rather than deletes would have done it by reporting "repointed" thirty-odd times.
#
# Narrow on purpose, so the rule `link()` states keeps working. A mount counts as belonging to another
# checkout on two conditions: its link value ends in the same relative path — `…/zsh/.zshrc` for
# `~/.zshrc` — and the root left over when that path is stripped holds a copy of the calling script.
# Together those make it the same file in a second copy of this repository, rather than an unrelated
# config the stale-symlink rule is right to repoint. A README-era link with a trailing slash still
# compares equal to this checkout and stays the compatibility case it was.
#
# A root that no longer resolves does not count either. That is the aftermath of this very bug, or of
# a checkout moved on purpose, and repointing a dangling link is the repair rather than the damage.
mount_foreign_root() {
  local source="$1" target="$2" rel cur root
  [ -L "$target" ] || return 1
  cur="$(link_value "$target")"
  # Absolute only. A relative link value resolves against the link's own directory, not this script's
  # working directory, so naming a root from it would name the wrong one. Every link written here is
  # absolute, so a relative one was not written by this script and is not one of its mounts.
  [ "${cur#/}" != "$cur" ] || return 1
  rel="${source#"$repo"/}"
  [ "${cur%"/$rel"}" != "$cur" ] || return 1
  root="${cur%"/$rel"}"
  root="$(real_dir "$root")" || return 1
  [ "$root" != "$repo" ] || return 1
  # And the root has to hold a copy of the calling script, not merely end in a matching path
  # component. Several sources are one component long — `nvim`, `ghostty`, `kk-flavor` — so the tail
  # comparison alone reads `~/.config/nvim -> ~/.dotfiles/nvim` as a second checkout, refuses the whole
  # run, and tells the human their configuration is mounted from a directory that has never held it.
  # That link is an ordinary stale mount and `link()` is right to repoint it.
  [ -f "$root/$script_name" ] || return 1
  printf '%s' "$root"
}

foreign_roots=()
note_foreign_root() {
  local candidate="$1" k
  for ((k = 0; k < ${#foreign_roots[@]}; k++)); do
    [ "${foreign_roots[k]}" != "$candidate" ] || return 0
  done
  foreign_roots+=("$candidate")
}

# How many mounts a root holds, spelled the way the guard reports it. With a bulk set the count leads
# and the kinds are broken out; without one there is nothing to break out.
mount_tally() { # <named count> <bulk count>
  if [ "$2" -gt 0 ]; then
    printf '%s mounts (%s configs and %s %s)' "$(($1 + $2))" "$1" "$2" "$bulk_label"
  else
    printf '%s mounts' "$(($1 + $2))"
  fi
}

# One root's worth of the refusal: what resolves to it, what running from here would do to them, and
# the refusal line the exit code carries. The count leads, before any list. Every line of the list
# carries the same root, so the list is one fact repeated; the scale is the fact a reader cannot
# reconstruct, and a reader who takes in the named configs and stops has not learned that every one of
# their skills moves too.
refuse_foreign_root() { # <root>
  local root="$1" i named n_bulk
  named=()
  n_bulk=0
  for ((i = 0; i < ${#cfg_targets[@]}; i++)); do
    [ "${cfg_foreign[i]}" = "$root" ] && named+=("${cfg_targets[i]}")
  done
  for ((i = 0; i < ${#bulk_targets[@]}; i++)); do
    [ "${bulk_foreign[i]}" = "$root" ] && n_bulk=$((n_bulk + 1))
  done

  say ""
  say "  $(mount_tally "${#named[@]}" "$n_bulk") currently resolve to"
  say "    $root"
  say "  and running from here would move every one of them to"
  say "    $repo"
  say ""
  for ((i = 0; i < ${#named[@]}; i++)); do
    say "    ${named[i]}"
  done
  [ "$n_bulk" -eq 0 ] || say "    ...and all $n_bulk $bulk_label"
  say ""
  refuse "$(mount_tally "${#named[@]}" "$n_bulk") resolve to $root, not to this checkout — nothing was written; re-run with --relocate to move them here"
}

# Survey every mount, refuse if this machine's config lives in another checkout, then link. One
# function rather than two calls, because the survey has to cover every mount before the first is
# written and a caller able to skip it is a caller able to skip the guard.
mount_run() {
  local i r found foreign_total

  say "mounts"
  cfg_foreign=()
  bulk_foreign=()
  foreign_total=0
  for ((i = 0; i < ${#cfg_targets[@]}; i++)); do
    found="$(mount_foreign_root "${cfg_sources[i]}" "${cfg_targets[i]}")" || found=""
    cfg_foreign[i]="$found"
    [ -z "$found" ] || {
      foreign_total=$((foreign_total + 1))
      note_foreign_root "$found"
    }
  done
  for ((i = 0; i < ${#bulk_targets[@]}; i++)); do
    found="$(mount_foreign_root "${bulk_sources[i]}" "${bulk_targets[i]}")" || found=""
    bulk_foreign[i]="$found"
    [ -z "$found" ] || {
      foreign_total=$((foreign_total + 1))
      note_foreign_root "$found"
    }
  done

  if [ "$foreign_total" -eq 0 ]; then
    # Said out loud on the way past. A guard that prints nothing when it passes reads exactly like a
    # guard that was never reached, and this one runs on every machine that is already set up.
    say "  ok       no mount on this machine comes from another checkout"
  elif $relocate; then
    say "  --relocate: moving $foreign_total mount(s) to this checkout, from:"
    for ((r = 0; r < ${#foreign_roots[@]}; r++)); do
      say "    ${foreign_roots[r]}"
    done
  else
    say ""
    say "  This checkout is not where this machine's configuration is mounted."
    for ((r = 0; r < ${#foreign_roots[@]}; r++)); do
      refuse_foreign_root "${foreign_roots[r]}"
    done
    report_and_exit
  fi

  say "links"
  for ((i = 0; i < ${#cfg_targets[@]}; i++)); do
    link "${cfg_sources[i]}" "${cfg_targets[i]}"
  done

  if [ "${#bulk_targets[@]}" -gt 0 ]; then
    say "$bulk_label"
    for ((i = 0; i < ${#bulk_targets[@]}; i++)); do
      link "${bulk_sources[i]}" "${bulk_targets[i]}"
    done
  fi

  # Inside this function rather than beside it, so a caller cannot skip the scan and keep a mount
  # nothing can reach; after the links, because a machine mounted from another checkout writes
  # nothing at all, removals included.
  unmount_run
}

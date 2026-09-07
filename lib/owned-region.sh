#!/usr/bin/env bash
#
# An idempotent region we own inside a file we do not: write it, detect it, remove it, and refuse
# rather than clobber anything else in there.
#
# Sourced, never executed, and only after lib/mount.sh — it reports through that file's `say` and
# `refuse` and honours its `$dry_run`, so a run that mounts and injects reports both halves the same
# way. Sourcing it first leaves those undefined and the first call exits 127.
#
# Why this is not in lib/mount.sh: that library only ever writes symlinks, and only at targets it can
# prove it owns — `link()` refuses the moment a target exists and is not a symlink. Writing bytes into
# a file someone else authored is the exact inverse, so it lives behind its own name rather than
# widening a contract env/bootstrap.sh also depends on.
#
# The fences are the caller's, not this file's. A CLAUDE.md region is fenced with HTML comments so it
# is invisible when the markdown renders; a .gitignore region is fenced with `#` lines. This file
# knows only that it was handed two marker lines and a body, which is what lets one implementation
# serve both and keeps `.claude`, `CLAUDE.md`, skills and projects out of its vocabulary.
#
# tested by: owned-region-test.sh
set -uo pipefail

# Whether the region is there, and whether the file is safe to touch at all.
#
#   absent    the file has neither fence — a write appends
#   present   both fences, in order — a write rewrites between them
#   conflict  one fence without the other, or a close before its open
#
# `conflict` is the refuse-rather-than-clobber case, and it is deliberately loud: half a fence means
# something edited inside our region or truncated the file, and either way the span we would rewrite
# is no longer the span we wrote. Guessing at its extent is how an installer eats a paragraph the
# human wrote.
region_state() { # <file> <open fence> <close fence>
  local file="$1" open="$2" close="$3" seen_open=0 seen_close=0 line
  # Read with IFS cleared and -r so a fence with leading whitespace is not silently matched, and a
  # body line holding a backslash survives the round trip.
  while IFS= read -r line || [ -n "$line" ]; do
    if [ "$line" = "$open" ]; then
      # A second open before the close is the same damage as a missing one: two regions, and no way
      # to say which is ours.
      [ "$seen_open" -eq 0 ] || {
        printf 'conflict\n'
        return 0
      }
      seen_open=1
      continue
    fi
    if [ "$line" = "$close" ]; then
      [ "$seen_open" -eq 1 ] && [ "$seen_close" -eq 0 ] || {
        printf 'conflict\n'
        return 0
      }
      seen_close=1
    fi
  done <"$file"
  if [ "$seen_open" -eq 1 ] && [ "$seen_close" -eq 1 ]; then
    printf 'present\n'
  elif [ "$seen_open" -eq 0 ] && [ "$seen_close" -eq 0 ]; then
    printf 'absent\n'
  else
    printf 'conflict\n'
  fi
}

# The guard both writers run first. Everything here is a reason to touch nothing, and each one is a
# different sentence because they send a reader somewhere different.
#
# A missing file is refused rather than created: this library is for regions inside files that already
# exist, and a caller that wants one created says so itself. Creating it here would let a typo in a
# path produce a plausible-looking new file in someone's repository.
#
# A symlink is refused for the mirror of `link()`'s reason — writing through one edits a file in a
# place the caller never named, which for a CLAUDE.md symlinked into a checkout means editing the
# checkout.
region_writable() { # <file>
  local file="$1"
  if [ -L "$file" ]; then
    refuse "$file is a symlink, and this writes into the file itself — repoint or remove it, then re-run"
    return 1
  fi
  if [ ! -e "$file" ]; then
    refuse "$file does not exist, and this never creates one — nothing was written"
    return 1
  fi
  if [ ! -f "$file" ]; then
    refuse "$file is not a regular file — nothing was written"
    return 1
  fi
  if [ ! -w "$file" ]; then
    refuse "$file is not writable — nothing was written"
    return 1
  fi
  return 0
}

# Replace the file through a temp file in its own directory, then `mv`. Same directory so the rename
# is on one filesystem and therefore atomic: a killed run leaves the original untouched rather than a
# half-written CLAUDE.md. `cat` into the temp preserves the original's permissions poorly, so they are
# copied over explicitly before the swap.
_region_replace() { # <file> <content on stdin>
  local file="$1" tmp
  tmp="$(mktemp "${file%/*}/.kk-region.XXXXXX")" || {
    refuse "could not create a temporary file beside $file — nothing was written"
    return 1
  }
  cat >"$tmp" || {
    rm -f -- "$tmp"
    refuse "could not write the new $file — the original is untouched"
    return 1
  }
  # Carry the original's mode over, so a file the human made executable or group-writable does not
  # silently come back as whatever the umask says.
  chmod --reference="$file" "$tmp" 2>/dev/null || {
    local mode
    mode="$(stat -f '%Lp' "$file" 2>/dev/null)" && chmod "$mode" "$tmp"
  }
  mv -f -- "$tmp" "$file" || {
    rm -f -- "$tmp"
    refuse "could not replace $file — the original is untouched"
    return 1
  }
  return 0
}

# Write the region: append it when absent, rewrite between the fences when present, and say nothing
# changed when what is there already matches byte for byte.
#
# The body is an argument, NOT stdin. A `body | region_write ...` pipeline runs this function in a
# subshell, so every `refuse` it makes lands in a copy of the refusals array that dies with the pipe —
# the run then reports success having refused to write. Same reason the awk below spells its fences
# `openf`/`closef`: `close` is an awk builtin, and passing `-v close=` makes awk bail out mid-program.
region_write() { # <file> <open fence> <close fence> <body>
  local file="$1" open="$2" close="$3" body="$4" state current
  region_writable "$file" || return 1

  state="$(region_state "$file" "$open" "$close")"
  if [ "$state" = "conflict" ]; then
    refuse "$file holds one half of the $open region — something edited inside it, so nothing was written"
    return 1
  fi

  if [ "$state" = "present" ]; then
    # Compared against what is between the fences, not against the whole file, so an unrelated edit
    # elsewhere in the human's file never looks like our region drifting.
    current="$(awk -v openf="$open" -v closef="$close" '
      $0 == closef { inside = 0 }
      inside { print }
      $0 == openf { inside = 1 }
    ' "$file")"
    if [ "$current" = "$body" ]; then
      say "  ok       $file already carries the $open region"
      return 0
    fi
    if $dry_run; then
      say "  would rewrite the $open region in $file"
      return 0
    fi
    awk -v openf="$open" -v closef="$close" -v body="$body" '
      $0 == openf { print; print body; skipping = 1; next }
      $0 == closef { skipping = 0 }
      !skipping { print }
    ' "$file" | _region_replace "$file" || return 1
    say "  rewrote  the $open region in $file"
    return 0
  fi

  if $dry_run; then
    say "  would add the $open region to $file"
    return 0
  fi
  # Appended with a blank line ahead of it when the file does not already end in one, so the region
  # never fuses onto the human's last paragraph. A file not ending in a newline gets one first, or
  # the open fence lands on the end of their final line.
  {
    cat "$file"
    [ -s "$file" ] && [ -n "$(tail -c 1 "$file")" ] && printf '\n'
    [ -s "$file" ] && printf '\n'
    printf '%s\n%s\n%s\n' "$open" "$body" "$close"
  } | _region_replace "$file" || return 1
  say "  added    the $open region to $file"
}

# Remove the region and nothing else. Absent is success, not a refusal — an uninstall run twice is a
# thing people do, and the second run has nothing to say beyond "already gone".
#
# The blank line the writer added ahead of the region goes with it, so install-then-uninstall leaves
# the file as it was found rather than growing a blank line per cycle.
region_remove() { # <file> <open fence> <close fence>
  local file="$1" open="$2" close="$3" state
  region_writable "$file" || return 1

  state="$(region_state "$file" "$open" "$close")"
  if [ "$state" = "conflict" ]; then
    refuse "$file holds one half of the $open region — its extent is not ours to guess, so nothing was removed"
    return 1
  fi
  if [ "$state" = "absent" ]; then
    say "  ok       $file carries no $open region"
    return 0
  fi
  if $dry_run; then
    say "  would remove the $open region from $file"
    return 0
  fi
  awk -v openf="$open" -v closef="$close" '
    $0 == openf { inside = 1; next }
    $0 == closef && inside { inside = 0; next }
    inside { next }
    # One blank line immediately before the open fence is ours — the writer put it there. Held back
    # rather than printed, and flushed only if something follows, so the file does not end on it.
    { if (held != "") { print held; held = "" } if ($0 == "") { held = $0; next } print }
  ' "$file" | _region_replace "$file" || return 1
  say "  removed  the $open region from $file"
}

#!/usr/bin/env bash
#
#
#
#
#
#   usage: ai/bootstrap.sh --agent=claude|codex [--dry-run] [--relocate] [--maintainer] [--owner] [--skip-brew]
#                          [--skip-tools] [--skip-mcp] [--skip-rtk] [--skip-verify] [--uninstall]
# tested by: bootstrap-test.sh and rtk-bootstrap-test.sh
set -uo pipefail

# `CDPATH=`: set in the environment, `cd` echoes the directory it landed on, so `repo` comes back two
# lines long and every source path built from it resolves nowhere.
repo="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
# What a second copy of this repository is recognised by. Read from the running file rather than
# written down, so a rename cannot leave the guard looking for a name nothing has.
script_name="$(basename -- "${BASH_SOURCE[0]}")"
label="ai bootstrap"

dry_run=false
relocate=false
skip_brew=false
skip_tools=false
skip_mcp=false
skip_rtk=false
skip_verify=false
maintainer=false
owner=false
uninstall=false
agent=""

for arg in "$@"; do
  case "$arg" in
    --agent=claude|--agent=codex) agent="${arg#*=}" ;;
    --dry-run) dry_run=true ;;
    --relocate) relocate=true ;;
    --skip-brew) skip_brew=true ;;
    --skip-tools) skip_tools=true ;;
    --skip-mcp) skip_mcp=true ;;
    --skip-rtk) skip_rtk=true ;;
    --skip-verify) skip_verify=true ;;
    # Opt IN. A tree's own maintenance skills are useless to a machine that only uses the tree, and
    # every skill's description costs context in every session whether or not it is invoked — so the
    # default installs the smaller set and the bigger one is asked for by name.
    --maintainer) maintainer=true ;;
    # The owner tier: --maintainer, plus rtk and the personal instruction file. ai/bootstrap-owner.sh
    # is the way in; the flag is what that wrapper passes and what this script's suite drives.
    --owner)
      owner=true
      maintainer=true
      ;;
    --uninstall) uninstall=true ;;
    -h | --help)
      sed -n '3,8p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      printf 'ai/bootstrap.sh: unknown option %s\n' "$arg" >&2
      exit 2
      ;;
  esac
done

[ -n "$agent" ] || {
  printf 'ai/bootstrap.sh: --agent=claude|codex is required — nothing was changed\n' >&2
  exit 2
}

bulk_label="skills"
# Refused by name rather than left to `.` failing. Without `set -e` a missing library carries on into
# `add_cfg: command not found` and exits 127, naming neither the file that is gone nor what to do
# about it — the same false diagnosis the verify step below guards against. ai/ is copied out of this
# repository on its own, so a checkout without lib/ is a real one.
for lib in mount.sh owned-region.sh install-registry.sh skill-audience.sh flavor-region.sh; do
  [ -r "$repo/../lib/$lib" ] || {
    printf 'ai/bootstrap.sh: lib/%s is missing from this checkout — ai/ and lib/ install together, and nothing was linked\n' "$lib" >&2
    exit 2
  }
done
# shellcheck source=../lib/mount.sh
. "$repo/../lib/mount.sh"
# After mount.sh: all but flavor-region.sh reach its say, refuse, add_bulk and $dry_run.
# shellcheck source=../lib/owned-region.sh
. "$repo/../lib/owned-region.sh"
# shellcheck source=../lib/install-registry.sh
. "$repo/../lib/install-registry.sh"
# shellcheck source=../lib/skill-audience.sh
. "$repo/../lib/skill-audience.sh"
# shellcheck source=../lib/flavor-region.sh
. "$repo/../lib/flavor-region.sh"

# --- the mount table ------------------------------------------------------------------------------

codex_home="${CODEX_HOME:-$HOME/.codex}"
skills_mount="$HOME/.claude/skills"
instruction_file="$HOME/.claude/CLAUDE.md"
if [ "$agent" = codex ]; then
  skills_mount="$HOME/.agents/skills"
  instruction_file="$codex_home/AGENTS.md"
fi

if [ "$agent" = codex ] && [ -s "$codex_home/AGENTS.override.md" ] && ! $uninstall; then
  refuse "$codex_home/AGENTS.override.md shadows AGENTS.md — merge its instructions before installing"
  report_and_exit
fi

keep_bucket=false
if $uninstall; then
  for mount_dir in "$HOME/.claude/skills" "$HOME/.agents/skills" "$codex_home/skills"; do
    { [ "$mount_dir" = "$skills_mount" ] || [ "$mount_dir" -ef "$skills_mount" ]; } && continue
    for mounted in "$mount_dir"/*; do
      [ -L "$mounted" ] || continue
      case "$(readlink "$mounted")" in
        "$repo/kk-flavor/skills/"*) keep_bucket=true ;;
      esac
    done
  done
fi
if $keep_bucket; then
  say "  kept     ~/.kk-flavor: another client still has skill mounts"
else
  add_cfg "$repo/kk-flavor" "$HOME/.kk-flavor"
fi

owner_source="$repo/owner-instructions.md"
owner_receipt="${instruction_file}.kk-flavor-installed"

owner_file_matches() {
  if [ -L "$instruction_file" ]; then
    case "$(readlink "$instruction_file")" in
      "$repo/CLAUDE.md" | "$repo/AGENTS.md" | "$owner_source") return 0 ;;
      *) return 1 ;;
    esac
    return
  fi
  [ -f "$instruction_file" ] || return 1
  cmp -s "$instruction_file" "$owner_source" && return 0
  if [ -f "$owner_receipt" ] && [ ! -L "$owner_receipt" ] && cmp -s "$instruction_file" "$owner_receipt"; then
    return 0
  fi
  [ "$agent" = codex ] || return 1
  local actual expected legacy_rtk
  actual="$(sed '/^[[:space:]]*$/d' "$instruction_file")"
  expected="$(printf '%s\n%s\n%s\n' "$flavor_region_open" "$(flavor_region_body)" "$flavor_region_close" | sed '/^[[:space:]]*$/d')"
  legacy_rtk="$(printf '@%s/RTK.md\n\n<!-- kk-flavor-rtk:begin -->\nRead `%s/RTK.md` for RTK usage. For unsupported commands or exact output, use `rtk proxy <command>`.\n<!-- kk-flavor-rtk:end -->' "$codex_home" "$codex_home" | sed '/^[[:space:]]*$/d')"
  [ "$actual" = "$expected" ] || [ "$actual" = "$expected
$legacy_rtk" ]
}

write_owner_instructions() {
  [ -f "$owner_source" ] && [ ! -L "$owner_source" ] || { refuse "$owner_source must be a regular owner instruction source"; return 1; }
  if [ -L "$owner_receipt" ] || { [ -e "$owner_receipt" ] && [ ! -f "$owner_receipt" ]; }; then
    refuse "$owner_receipt must be a regular installation receipt"
    return 1
  fi
  if { [ -e "$instruction_file" ] || [ -L "$instruction_file" ]; } && ! owner_file_matches; then
    refuse "$instruction_file contains personal instructions — preserve them before replacing it with the owner copy"
    return 1
  fi
  if $dry_run; then
    say "  would install $owner_source as a regular copy at $instruction_file"
    return 0
  fi
  local staged backup
  mkdir -p -- "${instruction_file%/*}" || { refuse "could not create instruction directory"; return 1; }
  if [ -L "$instruction_file" ] || ! cmp -s "$instruction_file" "$owner_source"; then
    if [ -e "$instruction_file" ]; then
      backup="$(mktemp "${instruction_file}.backup.XXXXXX")" || { refuse "could not create instruction backup"; return 1; }
      cp -p -- "$instruction_file" "$backup" || { refuse "could not back up $instruction_file"; return 1; }
      say "  backup   $backup"
    fi
    staged="$(mktemp "${instruction_file}.tmp.XXXXXX")" || { refuse "could not stage owner instructions"; return 1; }
    if ! cp -- "$owner_source" "$staged" || ! mv -f -- "$staged" "$instruction_file"; then
      rm -f -- "$staged"
      refuse "could not install owner instructions"
      return 1
    fi
  fi
  if ! cmp -s "$owner_source" "$owner_receipt"; then
    cp -- "$owner_source" "$owner_receipt" || { refuse "could not record installed owner instructions"; return 1; }
  fi
  say "  ok       $instruction_file is an independent owner copy"
}

remove_owner_instructions() {
  if [ -e "$instruction_file" ] || [ -L "$instruction_file" ]; then
    if ! owner_file_matches; then
      refuse "$instruction_file was modified — owner instructions were preserved"
      return 1
    fi
    if $dry_run; then
      say "  would remove the owner copy at $instruction_file"
    else
      rm -- "$instruction_file" || { refuse "could not remove owner instructions"; return 1; }
    fi
  fi
  if ! $dry_run && [ -f "$owner_receipt" ] && [ ! -L "$owner_receipt" ]; then
    rm -- "$owner_receipt" || { refuse "could not remove owner receipt"; return 1; }
  fi
}

add_skill_mounts "$repo/kk-flavor/skills" "$skills_mount" "$maintainer" "$uninstall"

# --- uninstall -------------------------------------------------------------------------------------

# A mode rather than a script of its own, over the same table declared above: a second script
# re-deriving what to remove drifts from what was installed, and drifts in the one direction nobody
# notices — leaving things behind and reporting ok.
if $uninstall; then
  unmount_run
  say "instructions"
  if $owner; then
    remove_owner_instructions
  elif [ -e "$instruction_file" ]; then
    region_remove "$instruction_file" "$flavor_region_open" "$flavor_region_close"
  else
    say "  ok       $instruction_file is not there"
  fi

  if ! $owner && [ "$agent" = codex ] && [ -f "$instruction_file" ]; then
    region_remove "$instruction_file" '<!-- kk-flavor-rtk:begin -->' '<!-- kk-flavor-rtk:end -->'
  fi

  projects="$(registry_live | grep -c . || true)"
  if [ "$projects" -gt 0 ]; then
    say ""
    say "  $projects project(s) still hold skills mounted from this checkout. Uninstall them before deleting it:"
    registry_live | while IFS= read -r p; do [ -n "$p" ] && say "    $p"; done
    say "  Run ai/install-project.sh --agent=claude|codex --uninstall <project> for each before removing this checkout."
  fi
  say ""
  say "  jq is left installed: nothing records whether this machine had it already or what else"
  say "  needs it, and a brew formula is shared and unrefcounted. The same goes for rtk."
  report_and_exit
fi

# Ahead of mount_run, not after it. Reached from below, an uninstall LINKS every mount first and then
# removes it: on a machine holding none, `--uninstall` builds the whole tree and tears it down again,
# and an interrupt between the two leaves the machine installed by the command that exists to
# uninstall it. ai/install-project.sh has always had this order; this file did not.
# A mount whose source this checkout no longer has is dropped, which `link()` cannot do: it iterates
# the sources this tree ships, so a skill deleted upstream leaves its symlink at the mount for good.
# Scanned rather than listed, and scoped to what this checkout wrote — a skill mounted from somebody
# else's tree is not ours to drop. Not narrowed by tier: a skill left out for want of `--maintainer`
# is still in the tree, so its mount still resolves and is not stale.
add_unmount_scan "$skills_mount" "$repo/kk-flavor/skills"

mount_run

if [ "$agent" = codex ] && [ ! "$codex_home/skills" -ef "$skills_mount" ]; then
  for mounted in "$codex_home/skills"/*; do
    [ -L "$mounted" ] || continue
    source_path="$(readlink "$mounted")"
    case "$source_path" in
      "$repo/kk-flavor/skills/"*)
        replacement="$skills_mount/${mounted##*/}"
        if [ -L "$replacement" ] && [ "$replacement" -ef "$mounted" ]; then
          unlink_mount "$mounted"
        fi
        ;;
    esac
  done
fi

# Said out loud, and after the mounts so it reads beside them. A flag that quietly leaves skills out is
# indistinguishable from a discovery loop that stopped finding them: the machine ends up short of
# skills with nothing in the run saying why. The zero case is the same claim about work that did not
# happen — a flag passed to a tree carrying no marked skill has to say it excluded nothing.
if ! $maintainer && [ "$skipped_count" -eq 0 ]; then
  say "  ok       no maintainer-only skill was there to exclude"
elif ! $maintainer; then
  say "  skipped  $skipped_count maintainer-only skill(s):$skipped_names"
fi

# Two ways to mount no skill, and they send a reader to different places: a skills directory with
# nothing in it is a broken checkout, while a flag that excluded every skill it found is a flag doing
# exactly what it says on a tree that has nothing else. The exit code is the same for both, so the
# wording is the only thing telling them apart.
if [ "${#bulk_targets[@]}" -eq 0 ]; then
  if [ "$skills_found" -gt 0 ]; then
    refuse "every skill under $repo/kk-flavor/skills/ is maintainer-only, and a run without --maintainer excluded all $skills_found — nothing was mounted"
  else
    refuse "no skill directories under $repo/kk-flavor/skills/ — nothing was mounted"
  fi
fi

# --- the instruction file ----------------------------------------------------------------------------

write_instruction_region() {
  say "instructions"
  if $owner; then
    write_owner_instructions || return 1
    local memory="$HOME/Document/AI/MEMORY.md"
    if [ ! -e "$memory" ]; then
      if $dry_run; then
        say "  would create $memory"
      else
        mkdir -p -- "${memory%/*}" && (set -o noclobber; printf '# Memory\n' >"$memory") || {
          refuse "could not create owner memory at $memory"
          return 1
        }
      fi
    fi
    if ! $dry_run && { [ ! -f "$memory" ] || [ ! -r "$memory" ] || [ ! -w "$memory" ]; }; then
      refuse "$memory must be a readable, writable memory file"
      return 1
    fi
    return 0
  fi
  if [ "$agent" = codex ] && [ -s "$codex_home/AGENTS.override.md" ]; then
    refuse "$codex_home/AGENTS.override.md shadows AGENTS.md — merge its instructions before installing"
    return 1
  fi
  if [ ! -e "$instruction_file" ] && [ ! -L "$instruction_file" ]; then
    if $dry_run; then
      say "  would create $instruction_file and add the kk-flavor region"
      return 0
    fi
    [ -d "${instruction_file%/*}" ] || mkdir -p -- "${instruction_file%/*}" || {
      refuse "could not create ${instruction_file%/*}, so the instruction region was not written"
      return 1
    }
    : >"$instruction_file" || {
      refuse "could not create $instruction_file, so the instruction region was not written"
      return 1
    }
  fi
  region_write "$instruction_file" "$flavor_region_open" "$flavor_region_close" "$(flavor_region_body)"
}

instructions_ready=false
if write_instruction_region; then instructions_ready=true; fi

# --- packages ------------------------------------------------------------------------------------

if $skip_brew; then
  say "brew (skipped)"
elif ! command -v brew >/dev/null 2>&1; then
  refuse "brew is not installed, so no formula was installed"
else
  say "brew"
  # Installed-first rather than `brew install` unconditionally: the latter is slow, noisy, and exits
  # non-zero on an already-installed formula, which would make a finished machine look broken.
  # The list stays literal on this line: ai/bootstrap-test.sh reads it straight out of the script and
  # holds it against the README's own `brew install` lines, so a variable here would silently stop
  # that comparison working. Which of them this tier installs is decided inside the loop instead.
  for formula in rtk jq; do
    # rtk compresses this machine's shell output for the agent — personal tooling, not something the
    # instruction tree needs. jq is everyone's: mcp-sync.sh cannot run without it.
    if [ "$formula" = rtk ] && ! $owner; then
      say "  skipped  rtk is the owner tier's"
      continue
    fi
    if brew list --formula "$formula" >/dev/null 2>&1; then
      say "  ok       $formula"
    elif $dry_run; then
      say "  would install $formula"
    else
      brew install "$formula" >/dev/null || refuse "brew install $formula failed"
    fi
  done
fi

# --- rtk ------------------------------------------------------------------------------------------

say "rtk"
rtk_md="$HOME/.claude/RTK.md"
if [ "$agent" = codex ]; then
  say "  ok       Claude RTK cleanup does not apply to Codex"
elif ! $owner; then
  say "  skipped  rtk is the owner tier's"
elif [ ! -e "$rtk_md" ] && [ ! -L "$rtk_md" ]; then
  say "  ok       no leftover $rtk_md"
elif [ ! -L "$rtk_md" ] && [ -d "$rtk_md" ]; then
  refuse "$rtk_md is a directory, and this script only ever wrote a file there — move it aside yourself"
elif $dry_run; then
  say "  would remove the leftover $rtk_md"
elif rm -f -- "$rtk_md"; then
  say "  removed  $rtk_md"
else
  refuse "could not remove the leftover $rtk_md"
fi

configure_rtk() {
  if ! $owner || $skip_rtk; then
    say "  skipped  RTK initialization"
    return 0
  fi
  if ! $instructions_ready; then
    say "  skipped  RTK initialization: instructions were refused"
    return 1
  fi
  local needs_rtk_init=true rtk_stage
  local rtk_args=(init --agent claude --global --hook-only --auto-patch)
  if [ "$agent" = codex ]; then
    rtk_args=(init --codex --global)
    if [ -e "$codex_home/RTK.md" ] || [ -L "$codex_home/RTK.md" ]; then
      region_writable "$codex_home/RTK.md" || return 1
      needs_rtk_init=false
    fi
  fi
  if $dry_run; then
    if $needs_rtk_init; then say "  would run rtk ${rtk_args[*]}"; fi
    [ "$agent" != codex ] || say "  owner instructions already describe RTK usage"
    return 0
  fi
  if ! command -v rtk >/dev/null 2>&1; then
    refuse "rtk is not on PATH — install it or use --skip-rtk"
    return 1
  fi
  if $needs_rtk_init; then
    if [ "$agent" = codex ]; then
      rtk_stage="$(mktemp -d)" || { refuse "could not create RTK staging directory"; return 1; }
      if ! CODEX_HOME="$rtk_stage" rtk "${rtk_args[@]}" || [ ! -f "$rtk_stage/RTK.md" ]; then
        rm -rf -- "$rtk_stage"
        refuse "rtk init failed for $agent"
        return 1
      fi
      mkdir -p -- "$codex_home" && cp -n -- "$rtk_stage/RTK.md" "$codex_home/RTK.md" || {
        rm -rf -- "$rtk_stage"
        refuse "could not install Codex RTK instructions"
        return 1
      }
      rm -rf -- "$rtk_stage"
    elif ! rtk "${rtk_args[@]}"; then
      refuse "rtk init failed for $agent"
      return 1
    fi
  else
    say "  kept     $codex_home/RTK.md"
  fi

}

configure_rtk

# --- the repository's own tools ------------------------------------------------------------------

if $skip_tools; then
  say "tools (skipped)"
elif $dry_run; then
  say "tools: would run ai/tools/install.sh"
elif ! command -v gh >/dev/null 2>&1; then
  refuse "gh is not installed, so ai/tools/install.sh could not fetch the tool binaries"
else
  say "tools"
  "$repo/tools/install.sh"
  install_status=$?
  # 3 is not a failure, and reporting it as one would fail every fresh clone until the first release
  # is cut. A refusal names something the human at this machine must do, and there is nothing: only
  # this repository's owner can cut a release, and until one exists resolve.sh builds each tool from
  # source on first use. A machine that cannot even do that — no Go — fails at the verify step below,
  # which runs those tools and reports the reason they did not run.
  if [ "$install_status" -eq 3 ]; then
    # Go is checked here rather than left to the verify step. Verify does run the tools and would
    # report their failure — but `--skip-verify` turns it off, and without this a run then ends green
    # having installed no tools onto a machine that cannot build them either.
    if command -v go >/dev/null 2>&1; then
      say "  ok       no release to install from; the tools build from source on first use, which needs Go"
    else
      refuse "no release to install from and no go on this machine, so the tools can be neither downloaded nor built — nothing was installed"
    fi
  elif [ "$install_status" -ne 0 ]; then
    refuse "ai/tools/install.sh failed"
  fi
fi

# --- MCP registry --------------------------------------------------------------------------------

if $skip_mcp; then
  say "mcp (skipped)"
elif $dry_run; then
  say "mcp: would run ai/mcp-sync.sh --agent=$agent"
elif ! command -v "$agent" >/dev/null 2>&1; then
  refuse "the $agent CLI is not on PATH, so the MCP servers were not registered"
else
  say "mcp"
  "$repo/mcp-sync.sh" "--agent=$agent" || refuse "ai/mcp-sync.sh failed"
fi

# --- verify --------------------------------------------------------------------------------------

# A setup script that reports success without checking anything has reported nothing, so the last step
# is the repository's own suites over what was just linked.
#
# The re-entry guard is load-bearing. `run-tests.sh` discovers every `*-test.sh`, `bootstrap-test.sh`
# is one of them, and it runs this script — so verify reaches a suite that reaches verify. It
# terminates today only because every case in that suite remembers `--skip-verify`, which is a loop
# held open by a convention. The marker closes it whatever any caller passes.
if [ -n "${BOOTSTRAP_VERIFYING:-}" ]; then
  say "verify (skipped: already inside a verify run)"
elif $skip_verify; then
  say "verify (skipped)"
elif [ ! -x "$repo/run-tests.sh" ]; then
  # A missing runner must not be reported as a failing suite. Without this the call exits 127 and the
  # arm below blames the suites for a file that was never there — a false diagnosis pointing at code
  # that is fine, which costs more than the silence would. Checked in the dry run too: a dry run that
  # says "ok" over a repository where the real run cannot work is the same lie one step earlier.
  refuse "ai/run-tests.sh is not in this checkout — the suites were not run, which is not the same as passing"
elif $dry_run; then
  say "verify: would run ai/run-tests.sh"
else
  say "verify"
  BOOTSTRAP_VERIFYING=1 "$repo/run-tests.sh" >/dev/null
  verify_status=$?
  # The runner's non-zero codes send a reader to different places: 1 to the code, 2 to this machine, 3
  # to whatever else was writing in this checkout while the suites ran. Collapsing any of them into the
  # failing-suite arm blames the suites for something they did not do.
  #
  # Its own account of a moved checkout — the before/after diff — goes to its stdout, which the call
  # above discards, so the refusal below has to stand on its own and say where to look.
  if [ "$verify_status" -eq 2 ]; then
    refuse "ai/run-tests.sh could not measure every suite — unproven is not disproven, and it is not passing either"
  elif [ "$verify_status" -eq 3 ]; then
    refuse "ai/run-tests.sh ran the suites, but the checkout changed while they ran — what it measured is not this tree. Re-run once nothing else is writing here"
  elif [ "$verify_status" -ne 0 ]; then
    refuse "ai/run-tests.sh reported a failing suite"
  fi
fi

# --- result --------------------------------------------------------------------------------------

report_and_exit

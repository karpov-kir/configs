#!/usr/bin/env bash
# Project skill links across existing and future Git worktrees, with repository-local setup.
# Runs against temporary homes and repositories; never changes the developer's client setup.
set -u

here=$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
checkout=$(CDPATH= cd -P -- "$here/.." && pwd -P)
script="$here/install-project.sh"
suite_name="ai/project-skills-test.sh"

# shellcheck source=../lib/test-harness.sh
. "$checkout/lib/test-harness.sh" ||
  { printf '%s: lib/test-harness.sh did not load to the end — nothing was measured\n' "$suite_name" >&2; exit 2; }

mkdir -p "$tmp_real/tools"
cat > "$tmp_real/tools/mise" <<'MISE'
#!/usr/bin/env bash
[ "$#" -eq 1 ] && [ "$1" = --version ] || exit 2
printf 'mise test fixture\n'
MISE
chmod +x "$tmp_real/tools/mise"
export PATH="$tmp_real/tools:$PATH"

echo "ai/install-project.sh"

run_install() { # <project> [flags...]
  local project="$1"
  shift
  out=$(HOME="$home" XDG_CONFIG_HOME="$home/.config" bash "$script" "$project" "$@" 2>&1)
  status=$?
}

new_project() { # <name>
  project="$tmp_real/$1"
  mkdir -p "$project"
  printf '# %s\n\nHow this project works.\n' "$1" >"$project/CLAUDE.md"
  printf 'node_modules/\n' >"$project/.gitignore"
}

# Boundary regressions use the same installer and checkout entry points as ordinary setup.
fresh_home
new_project empty-hook-path
git -C "$project" init -q
git -C "$project" config core.hooksPath ''
run_install "$project" --agent=claude
expect_status "empty hooksPath is preserved and reported" 1
expect_out "empty hooksPath supplies a repair command" 'existing Git hooks were preserved'
expect_absent "empty hooksPath creates no ineffective managed hook" "$project/.git/hooks/post-checkout"

fresh_home
new_project hardlinked-state
git -C "$project" init -q
mkdir -p "$project/.git/kk-flavor"
printf 'outside content\n' > "$tmp_real/state-outside"
ln "$tmp_real/state-outside" "$project/.git/kk-flavor/claude"
run_install "$project" --agent=claude
expect_status "hardlinked client metadata is refused" 1
expect_file_body "hardlinked metadata preserves the outside inode" "$tmp_real/state-outside" 'outside content'

fresh_home
new_project hardlinked-hook
git -C "$project" init -q
printf '%s\n' '#!/usr/bin/env bash' '# kk-flavor project skills' 'exec bash "$HOME/.kk-flavor/../project-skills.sh" --sync .' > "$tmp_real/hook-outside"
chmod 600 "$tmp_real/hook-outside"
ln "$tmp_real/hook-outside" "$project/.git/hooks/post-checkout"
run_install "$project" --agent=claude
expect_status "hardlinked managed hook is refused" 1
[ ! -x "$tmp_real/hook-outside" ] && record_pass "managed hook preserves outside file permissions" || record_fail "managed hook preserves outside file permissions" 'outside inode became executable'

fresh_home
new_project forged-home
git -C "$project" init -q
git -C "$project" -c user.name=Test -c user.email=test@example.com commit --allow-empty -qm initial
mkdir -p "$project/.git/worktrees/evil"
printf '%s/.git\n' "$home" > "$project/.git/worktrees/evil/gitdir"
printf '%s\n' "$(git -C "$project" rev-parse HEAD)" > "$project/.git/worktrees/evil/HEAD"
printf '../..\n' > "$project/.git/worktrees/evil/commondir"
run_install "$project" --agent=claude
expect_absent "prunable forged worktree never enables user skills" "$home/.claude"
printf 'gitdir: %s/.git/worktrees/evil\n' "$project" > "$home/.git"
ln -s "$home" "$tmp_real/home-alias"
out=$(HOME="$home" bash "$here/project-skills.sh" --sync "$tmp_real/home-alias" 2>&1)
status=$?
expect_status "direct sync rejects the physical home behind a symlink" 1
expect_absent "direct home sync never enables user skills" "$home/.claude"


# A forged entry can also point into an existing unrelated Git checkout.
fresh_home
new_project forged-repository
git -C "$project" init -q
git -C "$project" -c user.name=Test -c user.email=test@example.com commit --allow-empty -qm initial
outsider="$tmp_real/unrelated-repository"
git init -q "$outsider"
mkdir -p "$project/.git/worktrees/evil"
printf '%s/.git\n' "$outsider" > "$project/.git/worktrees/evil/gitdir"
printf '%s\n' "$(git -C "$project" rev-parse HEAD)" > "$project/.git/worktrees/evil/HEAD"
printf '../..\n' > "$project/.git/worktrees/evil/commondir"
run_install "$project" --agent=claude
expect_absent "foreign checkout is never populated from forged metadata" "$outsider/.claude"

# Existing and future worktrees must get project links without enabling user discovery.
fresh_home
new_project worktree-main
git -C "$project" init -q
git -C "$project" -c user.name=Test -c user.email=test@example.com commit --allow-empty -qm initial
sibling="$tmp_real/existing sibling"
git -C "$project" worktree add -q --detach "$sibling"
run_install "$project" --agent=claude
expect_status "worktree project install succeeds" 0
[ "$(readlink "$project/.claude/skills/kk-build")" = "$home/.kk-flavor/skills/kk-build" ] && record_pass "project links use the shared bucket" || record_fail "project links use the shared bucket" "wrong source"
expect_symlink "existing sibling receives Claude links" "$sibling/.claude/skills/kk-build"
expect_absent "project install creates no user Claude discovery" "$home/.claude"
expect_absent "project install creates no user Codex discovery" "$home/.agents"
future="$tmp_real/future sibling"
out=$(HOME="$home" git -C "$project" worktree add -q --detach "$future" 2>&1)
status=$?
expect_status "future worktree creation succeeds" 0
expect_symlink "future sibling receives Claude links" "$future/.claude/skills/kk-build"
expect_absent "worktree propagation writes no instructions" "$future/AGENTS.md"
[ -z "$(git -C "$future" status --porcelain)" ] && record_pass "generated links remain ignored in old branches" || record_fail "generated links remain ignored in old branches" "$(git -C "$future" status --porcelain)"
run_install "$sibling" --agent=codex
expect_status "install from linked worktree succeeds" 0
expect_symlink "existing sibling receives Codex links" "$sibling/.agents/skills/kk-build"
run_install "$project" --agent=claude --uninstall
expect_absent "uninstall clears sibling Claude links" "$sibling/.claude/skills/kk-build"
expect_symlink "uninstall retains sibling Codex links" "$sibling/.agents/skills/kk-build"
run_install "$project" --agent=codex --uninstall
expect_absent "last uninstall removes managed hook" "$project/.git/hooks/post-checkout"
expect_symlink "project uninstall keeps shared bucket" "$home/.kk-flavor"

expect_absent "last uninstall removes shared ignore rules" "$project/.git/kk-flavor/codex"

fresh_home
new_project custom-hook
git -C "$project" init -q
mkdir -p "$project/.git/hooks"
printf '#!/usr/bin/env python3\nprint("existing hook")\n' > "$project/.git/hooks/post-checkout"
before_hook=$(cat "$project/.git/hooks/post-checkout")
run_install "$project" --agent=claude
expect_status "custom checkout hook refuses automatic propagation" 1
expect_file_body "custom hook is preserved" "$project/.git/hooks/post-checkout" "$before_hook"
expect_out "custom hook refusal provides the repair command" 'project-skills.sh" --sync .'
run_install "$project" --agent=claude --uninstall
expect_file_body "uninstall preserves custom hook" "$project/.git/hooks/post-checkout" "$before_hook"

git -C "$project" config core.hooksPath .custom-hooks
run_install "$project" --agent=claude --dry-run
expect_status "dry run reports the custom hook conflict" 1
expect_absent "dry run creates no worktree metadata" "$project/.git/kk-flavor/claude"
run_install "$project" --agent=claude
expect_status "custom hooks manager refuses automatic propagation" 1
[ "$(git -C "$project" config --get core.hooksPath)" = .custom-hooks ] && record_pass "custom hooksPath remains unchanged" || record_fail "custom hooksPath remains unchanged" 'changed'

for planted in kk-flavor kk-flavor/claude info/exclude hooks; do
  fresh_home
  new_project "planted-${planted//\//-}"
  git -C "$project" init -q
  target="$project/.git/$planted"
  mkdir -p "${target%/*}"
  [ ! -e "$target" ] || mv "$target" "$target.original"
  ln -s "$tmp_real/never-written-${planted//\//-}" "$target"
  run_install "$project" --agent=claude
  expect_status "symlinked Git storage is refused: $planted" 1
  expect_absent "Git storage symlink target remains absent: $planted" "$tmp_real/never-written-${planted//\//-}"
done

fresh_home
new_project preview
run_install "$project" --agent=claude --dry-run
expect_out "dry run previews the bucket source" "$home/.kk-flavor/skills/kk-build"
expect_absent "dry run creates no bucket" "$home/.kk-flavor"

report_suite

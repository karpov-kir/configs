#!/usr/bin/env bash
# usage: ai/project-dependencies.sh [--dry-run]
# tested by: project-dependencies-test.sh
set -uo pipefail

isDryRun=false
for argument in "$@"; do
  case "$argument" in
    --dry-run) isDryRun=true ;;
    -h | --help)
      printf 'usage: ai/project-dependencies.sh [--dry-run]\n'
      exit 0
      ;;
    *)
      printf 'ai/project-dependencies.sh: unknown option %s\n' "$argument" >&2
      exit 2
      ;;
  esac
done

fail() {
  printf 'ai/project-dependencies.sh: %s\n' "$1" >&2
  exit 1
}

verifyMise() {
  mise --version >/dev/null 2>&1 ||
    fail 'mise --version failed. Repair the mise executable on PATH, then retry: https://mise.jdx.dev/installing-mise.html'
  printf '  ok       mise is available on PATH\n'
}

if command -v mise >/dev/null 2>&1; then
  verifyMise
  exit 0
fi

command -v brew >/dev/null 2>&1 ||
  fail 'mise is required and brew is unavailable. Install mise using https://mise.jdx.dev/installing-mise.html, add it to PATH, then retry.'

if $isDryRun; then
  printf '  would run HOMEBREW_NO_AUTO_UPDATE=1 brew install mise\n'
  exit 0
fi

printf '  installing mise with existing Homebrew\n'
HOMEBREW_NO_AUTO_UPDATE=1 brew install mise || fail 'brew install mise failed; resolve the Homebrew error, then retry.'
hash -r
command -v mise >/dev/null 2>&1 ||
  fail 'brew completed but mise is unavailable on PATH. Add the Homebrew bin directory to PATH, then retry.'
verifyMise

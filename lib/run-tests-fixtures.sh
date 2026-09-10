#!/usr/bin/env bash
#
# The fixtures the run-tests suites share. Sourced, never executed — the filename does not end in
# `-test.sh`, so run-tests.sh's discovery does not pick it up as a suite of its own.
#
# It lives under lib/ because that is the only place the gate can see it. The gate keys each shell unit
# on the libraries its suite sources, and both patterns that find them — the scan and the wider refusal
# for what it cannot read — match `lib/<name>.sh` and nothing else.
# A fixtures file beside the suites in ai/ would be sourced without being keyed on, and the unit would
# then answer out of its cache with these fixtures edited underneath it. That is a silent stale green,
# so the directory is load-bearing rather than tidy.
#
# Before sourcing, a caller sets `suite_name` (what a refusal reports itself as) and `runner` (the
# run-tests.sh under test, so a mutation run can point a suite at a mutated copy). After sourcing it
# has $tmp, TMPDIR pointed inside it, the skip counter, and the suite writers.
#
# Every case in both suites builds its own root under $tmp and points the runner at it. None of them
# run the runner over this repository, which is what keeps these files — discovered by that runner
# like any other suite — from recursing into themselves.
#
# tested by: ai/run-tests-test.sh, ai/run-tests-concurrency-test.sh

# The machine's own git config must not reach these fixtures. Both, because NOSYSTEM blocks
# /etc/gitconfig alone and ~/.gitconfig is the one that reaches in: a global core.excludesFile holding
# `*.conf` refuses new_greedy_checkout's `git add kept.conf`, and the whole containment family then
# goes red on a runner that is working perfectly.
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL=/dev/null

# Exit 2, not 1: a runner this suite cannot execute makes every case below fail for a reason that has
# nothing to do with what they assert, and 1 would claim the script under test is broken — a different
# statement, and a false one.
[ -x "$runner" ] || {
  printf '%s: %s is not an executable file — nothing was tested\n' "$suite_name" "$runner" >&2
  exit 2
}

tmp="$(mktemp -d)" || {
  printf '%s: could not create a temporary directory — nothing was tested\n' "$suite_name" >&2
  exit 2
}
trap 'rm -rf "$tmp"' EXIT

# The runner's own diagnostic logs, kept inside this suite's scratch rather than the machine's TMPDIR,
# so a case can assert both that a passing run writes none and that a failing one writes outside the
# root it was pointed at.
mkdir -p "$tmp/runner-logs"
export TMPDIR="$tmp/runner-logs"

# Counted, and printed as its own field by each suite. Cases sit behind conditions this machine may not
# meet, and `~/.kk-flavor/standards/testing.md` → **7. What a suite reports** makes a two-field summary
# an assertion that no case is conditional. Worse than untidy: run-tests.sh reads `skipped` BY NAME to
# decide vacuity, so a suite that dropped the field would report only the cases that did run and the
# runner would accept it as a clean run with the guarded ones silently gone.
skipped=0

# <count> <why>. The count is how many cases the guarded block holds. It is a literal nothing derives,
# and skip_count_drift below is what holds it in step.
record_skip() {
  skipped=$((skipped + $1))
  echo "skip — $1 case(s) not run: $2"
}

# Counts lines of the last run's output, which every case leaves in `out`.
matching_output_lines() { # <grep pattern>
  printf '%s' "$out" | grep -c "$1"
}

new_suite() { # <path> <summary line>
  printf '#!/usr/bin/env bash\necho "%s"\n' "$2" > "$1"
}
new_failing_suite() { # <path>
  printf '#!/usr/bin/env bash\nexit 1\n' > "$1"
}
new_unmeasured_suite() { # <path>
  printf '#!/usr/bin/env bash\nexit 2\n' > "$1"
}

# A checkout seeded with one committed file, plus a suite that overwrites it — the shape that makes the
# tree move under a run.
new_greedy_checkout() { # <dir>
  mkdir -p "$1"
  ( cd "$1" && git init -q . && git config user.email t@t && git config user.name t &&
    printf 'real config\n' > kept.conf && git add kept.conf && git commit -qm seed ) >/dev/null 2>&1
  printf '#!/usr/bin/env bash\nprintf "clobbered\\n" > "$(dirname "$0")/kept.conf"\necho "1 passed, 0 failed"\n' \
    > "$1/greedy-test.sh"
}

# Overlap is measured rather than timed: each suite marks that it is running, waits, counts the marks
# it can see, then clears its own. A wall-clock assertion proves nothing — it goes green on a fast
# machine whatever the runner did, and red on a loaded one that was right.
#
# The marks are cleared on the way out, so a serial run leaves every suite seeing exactly its own. Left
# behind, the second suite of a serial run would see two and read as overlap.
new_marking_suite() { # <path> <name> <peers>
  cat > "$1" <<EOF
#!/usr/bin/env bash
dir="\$(dirname "\$0")"
: > "\$dir/$2.running"
# Wait for the peers to mark, rather than sleeping a fixed second and counting whoever happened to
# have arrived. That sleep made this suite's verdict a race the wrong way round: a peer forked a moment
# late was counted absent, and the case reported the runner serialising when it had not — a 1.2s start
# delay on one of three turned "and they overlap" red.
#
# The second break is what keeps a genuinely serial run cheap: a peer that has already written its
# own count proves it is not running beside us, so there is nothing left to wait for.
for _ in \$(seq 1 100); do
  [ "\$(ls "\$dir"/*.running 2>/dev/null | wc -l)" -ge $3 ] && break
  ls "\$dir"/*.saw >/dev/null 2>&1 && break
  sleep 0.05
done
ls "\$dir"/*.running | wc -l | tr -d ' ' > "\$dir/$2.saw"
rm -f "\$dir/$2.running"
echo "1 passed, 0 failed"
EOF
}

most_seen() { # <dir>
  cat "$1"/*.saw 2>/dev/null | sort -n | tail -1
}

# The skip literals are counts nothing derives, so one drifts the moment a case joins a guarded block,
# and it drifts where nobody looks: the only machine that prints them is the one the guard is for. Held
# against the source they describe instead, on every machine.
#
# Opened on the `# guarded-block:` marker rather than on any one condition: keyed to the literal
# `if command -v git` it once named, it silently ignored a guard written on anything else. Takes the
# file to scan, because two suites now carry guarded blocks and a copy in each is the drift this
# guards against wearing the shape of a check.
skip_count_drift() { # <file> — empty when every record_skip count matches its block
  awk '
    /^# guarded-block:/                       { inblock = 1; n = 0; next }
    inblock == 1 && /^else$/                  { inblock = 2; next }
    inblock == 1 && /^  check /               { n++; next }
    inblock == 2 && /^  record_skip /         {
      if ($2 != n) { printf "line %d declares %s skipped over a block holding %d case(s); ", NR, $2, n }
      inblock = 0
    }
  ' "$1"
}

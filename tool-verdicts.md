# Tool verdicts

The counterpart to `README.md`: that file lists the tooling this machine installs and wires, this one
lists what it installed and deliberately did **not** wire. That state leaves no other trace: no
install step, no config entry, no standard. So nothing distinguishes "measured and rejected" from
"never heard of it", and an unwired binary stays invisible until something goes looking for the disk.

One section per tool. Each carries the **version and date the verdict was reached at**, so it can be
rechecked rather than re-argued; **what would change it**; and **the command that removes the tool**.
A section without those three is not worth keeping. Delete one when the tool is uninstalled, or when
it is adopted, since adoption moves its facts into the files that wire it.

Nothing loads this file. It records the machine, not what agents read, which is why it sits here
rather than under `ai/`.

## codebase-memory-mcp — adopted, CLI-only

**v0.10.8**, darwin-arm64. Third judgement, **2026-08-31**. The 2026-08-28 "do not adopt" was
**withdrawn**: it measured a working tree sitting in an unresolved merge, so its disqualifying evidence
was void (below). Re-measured against a clean pinned checkout. Installed at
`~/.local/bin/codebase-memory-mcp`, 283 MB, currently unwired. Uninstall with:

```
rm ~/.local/bin/codebase-memory-mcp && rm -rf ~/.cache/codebase-memory-mcp
```

### The valid measurement

Target `player-testing-service` at `806c9ac4`, clean tree, 279 TypeScript files. Indexed in **7.2s**,
3527 nodes, 10472 edges. **Zero TypeScript files flagged `parse_partial`** — the only 4 flagged are
`nginx.conf`, `client.scss`, `triageinit.sql`, matching known upstream grammar gaps (#1700, #1598).

Kill condition was "no meaningful win over grep/Explore on 2 of 3 tasks". It won **3 of 3**. Ground
truth for each was established by `git grep` and by reading the source, before the graph was asked.

1. **Inbound callers of `calculateSessionDuration`.** Graph, one query: 4 callers with hop distance —
   `finishSession` (1), `timeoutSession` (2), `SessionLifecycleManager.finishSession` (2),
   `crashTimeoutHandler` (3). All four verified correct. Grep returns 6 raw hits of which one is the
   definition and one is `dist/` build output, gives no hop distance, and reaches hop 3 only after
   three more rounds.
2. **Is `displayAlert` dead?** Graph: `displayError`, `displayWarning` at hop 1 (its own file), and
   `client` at hop 2. Verified. Grep's answer — two same-file uses — reads as near-dead; the graph gave
   the true reachability from `client.ts`. **The graph was more correct than grep here**, not merely
   faster.
3. **What does `finishSession` reach, 3 hops out, across packages.** Graph: through
   `Triager.getSevereFailures` into `player-testing-service-api`'s `isSevereFailure` and
   `hasMonitoredTriageStatus`. Every hop verified. The hard part: `Triager.ts:10` imports through
   `@bitmovin-internal/player-testing-service-api/dist/triage/triageutils`, a workspace alias into
   build output, and the graph resolved it back to `src/`. Grep cannot follow that without the agent
   manually mapping scope→package→`dist`→`src` at each hop.

Asked for `finishSession` unqualified it returned `status: ambiguous` with both candidates rather than
picking one — the failure mode of upstream #1909 declined in front of us.

### What the win is, and is not

It is **multi-hop and cross-package reachability**, where grep's cost is per hop and the agent pays a
model turn to interpret each one. It is **not** a replacement for grep on a single-symbol lookup, where
grep is already one step. Scope any rule to the former or it will earn nothing and cost context.

### Caveats that stand

- Every edge reported `strategy=heuristic`, confidence 0.90–0.95 — pattern-resolved, not LSP. Nine
  edges were hand-verified and all nine were right. That is nine edges, not a guarantee.
- The tasks tested reachability **positives**. Upstream #1354 and #1682 report missing TypeScript CALLS
  edges, and this measurement does not refute them: a silently absent edge is invisible to a test that
  checks what the graph *found*. **A negative from the graph still needs `check_index_coverage`, and on
  a clean file a bare "0 callers" remains the one answer not to trust on its own.**
- Trust checks passed: release checksum match, `gh attestation verify` exit 0 with a decoy exiting 1,
  `CBM_ALLOWED_ROOT` refusing an outside path. sha256 `9bd840df…6afd07`, adhoc-signed, unquarantined.

### Why the first verdict was void, kept as the lesson

`player-testautomation` had `.git/MERGE_HEAD` present and six files `UU` with live `<<<<<<< HEAD`
markers. The file the whole rejection rested on was not TypeScript. Its "three missing functions" were
missing because the source was broken; on the committed `HEAD` version of that same file,
`parse_partial_count: 0` and every function resolves. The index flagged **4 of 876 files, all four
conflicted, with zero false flags across the other 872** — the tool told the truth about exactly the
files that were broken, and the write-up read that as unreliability. The vendor's
`docs/EVALUATION_PLAN.md` already required a pinned clone at a recorded SHA.

That write-up also claimed no query-time completeness signal exists. **`check_index_coverage` provides
it per path** — `status: partial` with ranges, or `no_recorded_issue` — as a separate call rather than a
field on query responses.

### Wiring: CLI-only

`tools/list` returns **15 tools, 24.5 KB of schema — about 6.1k tokens in every session**, for a tool
worth reaching for on a minority of tasks. The CLI surface is the same 15 and costs nothing standing,
so it is the one wired. What CLI-only needs instead is a pointer, since an agent cannot reach for a
binary it does not know exists: `standards/code-navigation.md`, one router row.

Auto-indexing repos we do not own, a committed `graph.db.zst`, and the background watcher stay
rejected — none is earned by occasional use.

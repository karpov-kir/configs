# Tool verdicts

The counterpart to `README.md`: that file lists the tooling this machine installs and wires, this one
lists what it installed and deliberately did **not** wire. That state leaves no other trace — no
install step, no config entry, no standard — so nothing distinguishes "measured and rejected" from
"never heard of it", and an unwired binary stays invisible until something goes looking for the disk.

One section per tool. Each carries the **version and date the verdict was reached at**, so it can be
rechecked rather than re-argued; **what would change it**; and **the command that removes the tool**.
A section without those three is not worth keeping. Delete one when the tool is uninstalled, or when
it is adopted — adoption moves its facts into the files that wire it.

Nothing loads this file. It records the machine, not what agents read, which is why it sits here
rather than under `ai/`.

## codebase-memory-mcp — do not adopt

**v0.10.8**, darwin-arm64, judged **2026-08-28**. Installed at `~/.local/bin/codebase-memory-mcp`,
**283 MB**, and deliberately left unwired: no MCP entry, no watcher, no standard, no background
process. Uninstall with:

```
rm ~/.local/bin/codebase-memory-mcp && rm -rf ~/.cache/codebase-memory-mcp
```

sha256 `9bd840df…6afd07`. Verified: release checksum match, and `gh attestation verify` exit 0 (decoy
file exits 1, so the check can fail). Adhoc-signed, no `com.apple.quarantine`. Confinement:
`CBM_ALLOWED_ROOT=~/Documents/WP` refuses an outside path, exit 1, message on stderr. Target:
player-testautomation, 876 files, TS. Index 12s → 7058 nodes, 25823 edges.

### Verdict: fails the pre-agreed bar

Kill condition was "no meaningful win over grep/Explore on 2 of 3 tasks". It did not clear it as a
general navigation tool, for one reason that is not about speed.

### What it got right

- `trace_path --function-name PairingApi --direction both` returned the single real caller
  (`App.App.constructor`, hop 1), matching grep-established ground truth. One query.
- `--include-evidence` self-reports how each edge was resolved: that one was
  `strategy=heuristic confidence=0.50`, not LSP. The tool tells you when not to trust an edge.
- Auto-excludes `.claude/worktrees`, `node_modules`, `dist`. Ground truth for `ScreenApi` is 2 source
  files; a naive grep returns 7. That noise is real and the graph does not have it.
- The negative on `LegacyClientConfigurationManager` (nothing outside its own file) was correct.

### Why it fails anyway — measured, not theorised

`src/screen/lg/LgTelevisionScreen.ts`, 286 lines, flagged at index time as `error_ranges 1-287`:

- Three real top-level functions are absent from the graph — `sniffImageType`, `imageUrlOn`,
  `claimedImageType`. Direct name query returns `total: 0` for each; the file defines each once.
- A phantom node exists: `if` at lines 215-217, presented with `label: Function`. Not an identifier.
- The query that listed that file returned `has_more: false` — completeness asserted while wrong.
- No query response carries any incompleteness signal. `parse_partial` lives in `index_status` and in
  the `index_repository` reply. An agent that queries without calling `index_status` first cannot
  know the file it is reading about was never fully parsed.

The vendor states the same conclusion in `index_status.coverage_note`:
"Best-effort signal, not a completeness guarantee … Prefer text search (grep) for flagged
files/ranges. Files absent from this list are NOT guaranteed to be fully indexed."

### Decisions

- Do not author a `code-graph.md` standard, and add no `ai/kk-flavor/inject.md` row. The rule it
  would carry ("a negative from the graph is never an answer") is now vendor-documented behaviour
  rather than a house rule, and a standard costs context on every run that touches it.
- Do not add the MCP server to `ai/mcp.jsonc`. 15 tool schemas in every session, for a tool whose
  completeness cannot be trusted and whose one real advantage (`trace_path`) is reachable from the
  CLI on the rare occasion it is wanted.
- Keep the binary installed, unwired, for ad-hoc `trace_path` on a multi-hop question.
- No background watcher was enabled, so there is no standing cost beyond the 283 MB.

### What would change it

A query-time completeness signal — `search_graph` and `trace_path` returning the `parse_partial`
state of the files they touched. Then a negative could be trusted for files known clean. Worth
re-checking after a release that adds it.

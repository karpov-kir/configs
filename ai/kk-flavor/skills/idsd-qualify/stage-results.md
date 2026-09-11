# Submit a stage result

Use the JSON context returned by `report.sh invalidate <intent>`. Keep result files in the run's private scratch directory outside the reviewed repository. The context identifies the qualification attempt, HEAD, candidate tree and worktree; it is evidence to carry, not values to reconstruct.

For each completed stage, submit one JSON object containing that context plus:

| Field | Meaning |
|---|---|
| `id` | A new, unique result ID for this submission |
| `stage` | The stage named in the worker's assignment |
| `status` | `complete`; a stopped, failed or incomplete worker cannot certify a return |
| `outcome` | `complete`, or a refactor partial outcome accepted by the tool's usage |
| `items` | New unresolved decision items, or `[]` when none remain |

Run `report.sh stage-result` for the accepted field values and limits. Use plain text; the renderer escapes Markdown. The outcome records coverage, not the absence of decisions: a completed review may still return open items.

**Reconciling the return is `~/.kk-flavor/workers/idsd/qualify/reconcile.md`'s pass**, and the item list it returns is what goes in `items`. The tool preserves what it receives; it cannot prove that a worker found every defect or that the reconciliation kept every decision.

Submit with `report.sh stage-result <result.json> <intent>`. A successful return means the evidence was accepted and its items were written. Stamp only after every applicable stage is accepted, using the outcomes each returned. Skips still require their existing applicability or turnaround authorization.

## Changed inputs or interrupted submission

After a candidate repair, revalidate the affected review evidence, then obtain fresh context with `report.sh result-context <intent>`. Submit the final stage account under a new result ID. Reuse unaffected review work only when its file, dependency, tool and instruction inputs are provably unchanged; copying fresh context onto an unverified old result is not revalidation. A history change starts a fresh pass under the main skill's resume rule.

After an uncertain write, retry the exact saved result file. A pending submission can finish recovery; a completed duplicate is refused rather than recorded twice. If the tool reports a report-content conflict, preserve both the report and result file and resolve that conflict before retrying. Never remove retained finding evidence or invent a replacement receipt to clear the refusal.

Resolve a generated item by checking its box only after the human acts or evidence establishes the fix. Keep its accepted text intact; add any resolution explanation outside the generated block. An invalidation preserves these finding obligations even though it requires fresh stage completion evidence.

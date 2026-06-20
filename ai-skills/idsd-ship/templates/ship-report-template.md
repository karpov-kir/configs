---
intent: <NNN-slug or "review: <description>">
reviewed-tree: <hash>
---

<!-- idsd-ship working digest — persists across runs (gitignored; durable record is the ICE + git).
     Only the residue that needs the human. Two optional groups: # Decide (`- [ ]` actions the human
     must rule on before merge) and # Watch (monitor-only, no checkbox). A stage that surfaces nothing
     for the human writes nothing — an empty report is the success case, not an omission. Omit resolved
     fixes, passed/clean/not-applicable stages, and any "here's what changed" or verification narration;
     that lives in the diff + commit. On re-review: unresolved `- [ ]` carry forward, resolved ones drop;
     Watch bullets re-evaluated (kept while relevant, dropped when moot). No per-stage sections, no summary. -->

# Decide

# Watch

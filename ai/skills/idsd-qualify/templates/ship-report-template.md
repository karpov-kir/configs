---
intent: <NNN-slug or "review: <description>">
reviewed-tree: <hash>
reviewed-stages: <stages>
---

<!-- idsd-ship working digest at .idsd/ship-report.md — persists across runs, never committed
     (durable record is the ICE + git; report.sh check-ignore keeps it out of git).
     Only the residue that needs the human. Two optional groups: # Decide and # Watch (monitor-only,
     no checkbox). A Decide item is `- [ ] **<the decision, one line>** — recommend: <one clause>.`
     with its case (what/why/evidence) indented beneath; order forks → ratifications → pending
     evidence, and state background several items share once, in a preamble under # Decide.
     A stage that surfaces nothing for the human writes nothing — an empty report is the success
     case, not an omission. Omit resolved fixes, passed/clean/not-applicable stages, and any "here's
     what changed" or verification narration; that lives in the diff + commit. On re-qualify:
     unresolved `- [ ]` carry forward, resolved ones drop; Watch bullets re-evaluated (kept while
     relevant, dropped when moot). No per-stage sections, no summary. -->

# Decide

# Watch

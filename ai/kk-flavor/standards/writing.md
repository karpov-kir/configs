# Writing Guidelines

Applies to anything you write. Persistent artifacts (code comments, PR/commit descriptions, tickets, design/investigation docs, and similar) always follow these guidelines in full — lean, concise, fact-per-line — but in normal prose; caveman mode never applies to them, even when active. Chat responses still follow caveman style when on.

* Scope. Stay within the artifact's responsibility — reference only down to its own altitude; link to other layers rather than restating them. Function docs describe the function, not its callers; prose above the code (commit/PR descriptions, tickets, design docs, and similar) states the problem and outcome, not the files or functions implementing it — that trace lives in the diff.
* Lead with the "why", not the implementation trace.
* Describe conceptually — what happens and why, not call-by-call. One abstraction level.
* Keep code references minimal; the diff is source of truth.
* Group by purpose, not by file.
* Each line must carry a fact unreachable from surrounding context (code, types, siblings, the diff, etc.). Cut or link otherwise.
* Dense, not cryptic. Plain words, direct verbs, complete sentences — contractions are fine. Density comes from cutting redundancy and inferable filler, never from compressing sentences into shorthand the reader must decompress.
* One home per fact. State a rule once, in the artifact it belongs to, and cross-reference it from everywhere else. Two statements that can't both hold get reconciled to one, never left to coexist.
* Open your enumerations. A bare list implies completeness — mark an open set ("e.g.", "and similar") and enumerate plainly only when the set is genuinely fixed.
* No backstory, hedging, or justification — describe what is true, not what we tried.
* These rules tune artifacts for the context window. **Outward text** — anything a person reads as communication (the set [human-writing.md](human-writing.md) defines) — additionally follows that standard, which wins there on conflict: natural voice, no AI tells, and a deliberately lossy reader-action budget.
* Before publishing finished prose, apply the matching retrofit lens (roles per `config.yaml`): internal artifacts → `tighten` (code comments then get `humanize`'s comment form — [human-writing.md](human-writing.md)); outward text → `humanize`, which pulls tighten's lossless pass first when the text needs it. Spawn the skill for files; apply the lens inline for ephemeral text (a chat reply, a comment body about to be posted). Text a qualify pass will cover needs no separate pass — its tighten stage owns it.

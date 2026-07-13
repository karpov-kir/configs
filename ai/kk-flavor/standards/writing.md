# Writing Guidelines

Applies to anything you write. Persistent artifacts (code comments, PR/commit descriptions, tickets, design/investigation docs, and similar) always follow these guidelines in full — lean, concise, fact-per-line — but in normal prose; caveman mode never applies to them, even when active. Chat responses still follow caveman style when on.

* Scope. Stay within the artifact's responsibility — reference only down to its own altitude; link to other layers rather than restating them. Function docs describe the function, not its callers; prose above the code (commit/PR descriptions, tickets, design docs, and similar) states the problem and outcome, not the files or functions implementing it — that trace lives in the diff.
* Lead with the "why", not the implementation trace.
* Describe conceptually — what happens and why, not call-by-call. One abstraction level.
* Keep code references minimal; the diff is source of truth.
* Group by purpose, not by file.
* Each line must carry a fact unreachable from surrounding context (code, types, siblings, the diff, etc.). Cut or link otherwise.
* No backstory, hedging, or justification — describe what is true, not what we tried.

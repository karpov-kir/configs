---
name: security-review
description: Adversarially review the working-tree changes for security vulnerabilities — injection, auth/access, secret exposure, unsafe input/deserialization, and any security invariant the project states. Surface findings with severity, exploit scenario, and fix; apply only trivial safe fixes. Use when asked to "security review", "audit the changes for vulns", or when idsd-ship spawns a security pass. Local changes; returns findings as data.
argument-hint: "file, directory, diff selector (staged/unstaged/all changed), or natural-language scope"
---

Adversarially review every change resolved from `$ARGUMENTS` — assume the code is hostile until proven otherwise, find the vulnerabilities a real attacker would, and explain each so an engineer can fix it. Scoped to the change and the data flows it touches. Reviews **local working-tree changes** and returns findings as data — it never posts to GitHub.

**Security, not correctness or quality.** Functional bugs are `/review`'s lane; style and structure are `/refactor`'s. Here: exploitable weaknesses only. A security rule the project's `CLAUDE.md`/constitution states (path-safety, network-bind, secrets, oid-match, …) is in scope — violating it is the finding.

**Caller.** You run either standalone (the user is your caller) or spawned by an orchestrator with no interactive user. Every "ask" / "confirm" below resolves to *ask your caller*: interactive → ask directly; spawned → return the finding (or `blocked: <what you need>`) and stop. Apply only a trivial, unambiguous fix (e.g. redacting a logged secret); propose the rest with its fix — a risky or structural security change needs human sign-off, so never apply one just because you can't ask.

**Secret handling (mandatory).** Never write a secret's value into any output — no finding, report, quoted excerpt, or echoed tool output. Mask it to the first 2–4 identifying characters plus `****` (`AKIA****`). Cite `file:line` as the canonical location. State what the credential appears to grant and whether it looks live; recommend rotation for anything live — exposure in source means it is already compromised.

## Setup (once)

- Read `CLAUDE.md` and the constitution's security principles and NFRs (already in context where present). List the project security invariants the changes must hold.
- Resolve the change set from `$ARGUMENTS` by intent (this audits *changes* — there is no whole-project mode by design):
  - File path or directory — its current diff against the base.
  - **Staged** / **unstaged** / **all changed** → `git diff` with `--cached` / nothing / `HEAD` (names via `--name-only`).
  - Natural-language scope — the matching changed files.
- Save the changed-file list to TodoWrite — the review queue; every file gets a verdict. Skip deleted files; for a rename, use the new path.

## Coverage

Adapt to the stack — skip classes the target can't have (web items for a CLI, etc.). Trace each relevant class over the change and the flows it reaches:

- **Injection** — SQL/NoSQL, OS-command, template, path; trace every user-controlled input to its sink, including dynamic queries and shell-outs.
- **Auth / session / access** — missing checks on sensitive routes or actions, IDOR, privilege escalation, permissive ACLs or file permissions.
- **Secret exposure** — secrets in source, weak crypto, PII or credentials in logs.
- **Unsafe input** — missing validation at trust boundaries; insecure deserialization (untrusted data into `pickle`/`yaml.load`/custom parsers).
- **SSRF / path traversal / open redirect** — web and network targets.
- **Misconfiguration** — debug mode, verbose errors, default or hardcoded credentials.
- **Vulnerable dependencies** — flag manifest versions with known CVEs.
- **Project security invariants** — every security rule the `CLAUDE.md`/constitution states.

## Loop

- Review one changed file per message.
- Read the full file every time, including re-reviews. For files over 2000 lines, read in sequential chunks until every line is covered.
- Trace the coverage classes over the file and the flows it reaches; surface a finding only with a concrete exploit scenario — if you can't write how an attacker uses it, downgrade the severity or drop it.
- State each finding's provenance: whether the change introduces the weakness, worsens it, or routes a pre-existing pattern into a newly dangerous path (e.g. an existing helper now sitting in an enforcement path). All three are in scope, but the label lets the caller weigh it without re-deriving the history — never drop a finding merely because the pattern predates the change.
- Apply a trivial, unambiguous fix (e.g. secret redaction), flagging any that changes behaviour; propose every risky or structural fix with its remediation, per **Caller**.
- Order inside a message: read the file, apply trivial fixes, then emit the verdict last. The verdict describes the state **before** the fix.
- If the file passes, move on. If a finding resists three passes, emit a `WARN` verdict and ask the caller.
- Once every file has a verdict, do a final sweep for cross-file data flows; a file that warns in the sweep retries next message, passing files stay passed. The loop ends when a complete sweep produces no new warnings.

## Verdict format

Always the last thing in the message, searchable via `File N/M `:

- Pass: `File N/M <path> | <lines>L | OK`
- Fail:
  ```
  File N/M <path> | <lines>L | WARN
  <severity> CWE-XXX <location>: <weakness>. Exploit: <one sentence>. Fix: <remediation>. [fixed | needs human]
  ```

Severity is Critical / High / Medium / Low (CVSS-ish reasoning). `M` is the current queue length, `N` the file's stable position. Standalone, the verdicts plus applied fixes are your output; spawned, that set is your structured return for the caller to record.

## Do not

- Post to GitHub or run `gh` — this is a local review.
- Write any secret value into output (per **Secret handling**) — mask it.
- Flag a weakness you can't tie to an exploit scenario — downgrade or drop it.
- Apply a risky or structural security fix just because you can't ask (per **Caller**).
- Re-audit pre-existing weaknesses outside the change; surface a serious one as a separate non-blocking note, don't fix or block on it (a weakness the change introduces or worsens is in scope).

---
name: kk-security-review
description: Adversarially review the working-tree changes for security vulnerabilities — injection, auth/access, secret exposure, unsafe input/deserialization, and any security invariant the project states. Surface findings with severity, exploit scenario, and fix; apply only trivial safe fixes. Use when asked to "security review", "audit the changes for vulns", or when idsd-qualify spawns a security pass. Local changes; returns findings as data.
argument-hint: "file, directory, diff selector (staged/unstaged/all changed), or natural-language scope"
---

Adversarially review every change resolved from `$ARGUMENTS` — assume the code is hostile until proven otherwise, find the vulnerabilities a real attacker would, and explain each so an engineer can fix it. Scoped to the change and the data flows it touches. Reviews **local working-tree changes** and returns findings as data — it never posts to GitHub.

**Security, not correctness or quality.** Functional bugs are `/code-review`'s lane; style and structure are `/refactor`'s. Here: exploitable weaknesses only. A security rule the project's `CLAUDE.md`/constitution states (path-safety, network-bind, secrets, oid-match, …) is in scope — violating it is the finding. Capacity and NFR arithmetic (durations, throughput, fleet occupancy) is out of lane too: those numbers get *measured* by gates, not reasoned about here — a plausible-but-wrong estimate survives review and poisons specifications.

**Secret handling (mandatory).** Never write a secret's value into any output — no finding, report, quoted excerpt, or echoed tool output. Mask it to the first 2–4 identifying characters plus `****` (`AKIA****`). Cite `file:line` as the canonical location. State what the credential appears to grant and whether it looks live; recommend rotation for anything live — exposure in source means it is already compromised.

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md`. Unit noun: `File`; deltas below — note the tighter license: apply only a trivial, unambiguous fix (e.g. redacting a logged secret); a risky or structural security change always goes to your caller with its remediation.

## Setup (once)

- Read the security invariants in scope: inject the kk-flavor if needed (read `~/.kk-flavor/inject.md` when its routing isn't already in context), read the skill protocol (above) and the standards the flavor routes you to, plus any the project's constitution / `CLAUDE.md` states. List the project security invariants the changes must hold.
- Resolve the change set from `$ARGUMENTS`: a git scope (per the protocol), a file path or directory (its current diff against the base), or a natural-language scope (the matching changed files). This audits *changes* — no whole-project mode by design. Queue per the protocol.

## Coverage

Adapt to the stack — skip classes the target can't have (web items for a CLI, etc.). Trace each relevant class over the change and the flows it reaches:

- **Injection** — SQL/NoSQL, OS-command, argument/option, template, path; trace every user-controlled input to its sink, including dynamic queries, shell-outs, and args passed to a process *without* a shell — a value starting with `-` is parsed as a flag (e.g. `git` option injection, up to RCE), so require a `--` separator and reject option-like values.
- **Auth / session / access** — missing checks on sensitive routes or actions, IDOR, privilege escalation, permissive ACLs or file permissions.
- **Secret exposure** — secrets in source, weak crypto, PII or credentials in logs, over-broad env propagation to subprocesses (a full environment leaks every parent secret to the child and anything it spawns — helpers, hooks); require a minimal, allow-listed env.
- **Credential persistence** — tooling that stores credentials as a side effect: git credential helpers (a successful fetch triggers `op=store` to every configured helper), package-manager keychains, CLI token caches. A subprocess handling a shared secret needs them disabled or isolated — the leak outlives the process and its rotation.
- **Unsafe input** — missing validation at trust boundaries; insecure deserialization (untrusted data into `pickle`/`yaml.load`/custom parsers).
- **SSRF / path traversal / open redirect** — web and network targets.
- **Misconfiguration** — debug mode, verbose errors, default or hardcoded credentials.
- **New outbound destinations** — a change that introduces a new outbound host or endpoint (license servers, telemetry, webhooks) is a first-class finding wherever egress is controlled: name the host and what newly reaches it, not an aside below the findings.
- **Vulnerable dependencies** — flag manifest versions with known CVEs.
- **Project security invariants** — every security rule the `CLAUDE.md`/constitution states.

## Loop deltas

- Trace the coverage classes over the file and the flows it reaches; surface a finding only with a concrete exploit scenario — if you can't write how an attacker uses it, downgrade the severity or drop it.
- State each finding's provenance: whether the change introduces the weakness, worsens it, or routes a pre-existing pattern into a newly dangerous path (e.g. an existing helper now sitting in an enforcement path). All three are in scope, but the label lets the caller weigh it without re-deriving the history — never drop a finding merely because the pattern predates the change.
- Apply a trivial, unambiguous fix (e.g. secret redaction), flagging any that changes behaviour; propose every risky or structural fix with its remediation.
- The final sweep hunts cross-file data flows.

## Verdict

- Pass: `File N/M <path> | <lines>L | OK`
- Fail:
  ```
  File N/M <path> | <lines>L | WARN
  <severity> CWE-XXX <location>: <weakness>. Exploit: <one sentence>. Fix: <remediation>. [fixed | needs human]
  ```

Severity is Critical / High / Medium / Low (CVSS-ish reasoning).

## Do not

- Post to GitHub or run `gh` — this is a local review.
- Write any secret value into output (per **Secret handling**) — mask it.
- Flag a weakness you can't tie to an exploit scenario — downgrade or drop it.
- Re-audit pre-existing weaknesses outside the change; surface a serious one as a separate non-blocking note, don't fix or block on it (a weakness the change introduces or worsens is in scope).

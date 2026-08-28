---
name: kk-security-review
description: Adversarially review the working-tree changes for exploitable vulnerabilities. Use for "security review", "audit the changes for vulns". Local changes; a GitHub PR is kk-pr-review's, functional bugs kk-code-review's. Works this skill's own threat model, unlike the same-named bundled reviewer.
argument-hint: "file, directory, diff selector (staged/unstaged/all changed), or natural-language scope"
---

Adversarially review every change resolved from `$ARGUMENTS`: assume the code is hostile until proven otherwise. Scoped to the change and the data flows it touches — no whole-project mode by design.

**Exploitable weaknesses only** — functional bugs are `kk-code-review`'s lane, style and structure `kk-refactor`'s.

**Secret handling (mandatory).** Never write a secret's value into any output — no finding, report, quoted excerpt, or echoed tool output. Mask it to the first 2–4 identifying characters plus `****` (`AKIA****`) and cite `file:line` as the canonical location. Recommend rotation for anything live — exposure in source means it is already compromised — and state what the credential appears to grant; a scope you can't confirm is stated as apparent, never a reason to downgrade the finding. Before asserting where a secret lives, or that a cleanup removed every copy, grep the run's own scratch dir too.

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md`. Unit noun: `File`; deltas below — note the tighter license: apply only a trivial, unambiguous fix (e.g. redacting a logged secret); a risky or structural security change always goes to your caller with its remediation.

## Setup (once)

- List the security invariants the changes must hold — the project's constitution and `CLAUDE.md` on top of the standards.

## Coverage — the threat model

Adapt it to the stack — skip classes the target can't have (web items for a CLI, …), and add any the stack carries that this list misses:

- **Injection** — SQL/NoSQL, OS-command, argument/option, template, path; trace every user-controlled input to its sink. Args passed *without* a shell are not safe on their own: a value starting with `-` is parsed as a flag (`git` option injection, up to RCE), so require a `--` separator and reject option-like values.
- **Auth / session / access** — missing checks on sensitive routes or actions, IDOR, privilege escalation, permissive ACLs or file permissions.
- **Secret exposure** — secrets in source, weak crypto, PII or credentials in logs, over-broad env propagation to subprocesses; require a minimal, allow-listed env.
- **Credential persistence** — tooling that stores credentials as a side effect: git credential helpers (a successful fetch triggers `op=store` to every configured helper), package-manager keychains, CLI token caches. A subprocess handling a shared secret needs them disabled or isolated.
- **Unsafe input** — missing validation at trust boundaries; insecure deserialization (untrusted data into `pickle`/`yaml.load`/custom parsers).
- **SSRF / path traversal / open redirect** — web and network targets.
- **Misconfiguration** — debug mode, verbose errors, default or hardcoded credentials.
- **New outbound destinations** — a new outbound host or endpoint (license server, telemetry, webhook) is a first-class finding wherever egress is controlled: name the host and what newly reaches it.
- **Vulnerable dependencies** — flag manifest versions with known CVEs.
- **A newly added dependency** — name the package, its install-time scripts, what it pulls in transitively, and its provenance. A clean CVE scan does not clear it.
- **Project security invariants** — the ones listed in Setup.

## Loop deltas

- Trace the coverage classes over the file and the flows it reaches; surface a finding only with a concrete exploit scenario — if you can't write how an attacker uses it, downgrade the severity or drop it.
- Label each finding **introduced**, **worsened**, or **newly reachable** (a pre-existing pattern the change routes into a dangerous path).
- The final sweep hunts cross-file data flows.

## Verdict

Finding line: `<severity> CWE-XXX <location>: <weakness>. Exploit: <one sentence>. Fix: <remediation>. [fixed | needs human]` — severity Critical / High / Medium / Low (CVSS-ish reasoning).

## Do not

- Post to GitHub or run `gh` — `~/.kk-flavor/standards/quality-pipeline.md` owns that rule and binds every stage, this one run standalone included.
- **Re-audit** a pre-existing weakness outside the change — this lane's addition to that file's pre-existing-defect rule, which otherwise binds unchanged. **Anything carrying one of the three labels above is in scope**, **newly reachable** included.

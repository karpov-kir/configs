# Model policy

Resolve model-bearing work from `~/.kk-flavor/models.json`. Its profiles, roles and usage map are the single place to assign or upgrade models. Skills name roles; they do not embed model IDs.

## Original task

Capture the original task's client, actual model and effort before any override. Keep that origin and the policy digest in the run's scratch record. The immediate parent and the client's current global defaults are not substitutes for the original selection.

Implementation, correctness and security inherit the original task's selection. Instruction semantics, conformance, diagnosis, architectural refactor, driving and adjudication also remain on that selection in the shipped policy. Cheaper coordination or editing requires an explicit evaluated profile change; it must not change protected workers.

## Resolve before dispatch

Run `~/.kk-flavor/scripts/model-policy.sh --help` for the resolver's arguments. Supply the active client, role and recorded origin. Use an explicit candidate config when testing a worktree; the installed path still resolves to the installed checkout.

The resolver returns requested settings and the policy digest. It never launches an agent or verifies which model ran. Keep requested and observed settings separate in the result. Reuse a role resolution while client, origin, role and policy are unchanged; do not run it for every file or tool command.

Validate the request against the actual transport before dispatch. A model available through a CLI may be unavailable in native desktop tools. Native definitions, environment settings and effort precedence must not silently override the requested selection. Full-history inheritance and explicit overrides may be mutually exclusive.

If exact origin is unavailable, protected work may inherit directly from the original task through a proven native path. The resolver's original-task flag asserts that provenance; it does not prove it. A cheap intermediary, standalone CLI or unknown inheritance path must refuse that fallback.

Unknown roles, unsupported selections and missing origin fail visibly. Do not substitute a cheaper model or another provider. An explicit user model change updates the relevant run selection; a background config edit affects new runs, not work already dispatched.

## Work and results

Use native leaf workers and completion waits when available. An already configured native worker with proven original-task inheritance needs no extra CLI agent. Deterministic commands, schema validation and waiting use tools without a model worker.

Supply bounded context: the requirement, resolved scope, relevant decisions, artifact paths and return contract. Full conversation history is justified only when that reasoning is a required input. Return findings and evidence, not exploration transcripts.

Record role, requested and observed model/effort, policy digest, scope, candidate identity, elapsed time and provider-reported usage when available. Preserve missing usage as unavailable and distinguish cached input from uncached input. A completed worker is not a passed review.

Evaluate cheaper profiles on preserved meaning, accepted results, retries, cost and elapsed time before adopting them. Local inference remains deferred; the resolver does not start a local service or provide cloud fallback.

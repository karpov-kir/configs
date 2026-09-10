# Role routing regression cases

Run when changing session ownership or inline execution rules. These probes test action selection; they do not prove tool enforcement or actual compaction behavior.

## Procedure

Give an isolated instruction worker the candidate checkout's shared skill protocol, handoff skill and applicable IDSD skills. Supply only the Input column below, not the Expected route column. Ask for the next actions, the session that owns each action, and all intended write locations. Do not let the probe launch tasks or mutate a repository. Resolve its model through the instruction role and record the candidate file hashes and worker identity.

Grade the negative controls first, then the probe responses against the expected routes. A missing case is untested, not passed. Retain each response and verdict in the run's external evidence directory. A response that merely quotes a rule without choosing an action fails.

The common fixture is a reactor in a clean repository with available task tools, explicit permission to create separate tasks, settled landing authorization and a confirmed schedule. Any exception is stated in the input. Task IDs below are fixture identifiers, never addresses to contact.

## Cases

| Case | Input | Expected route |
| --- | --- | --- |
| Author | In the reactor, the user asks: "Separate Windows 10 and Windows 11; create an intent." No intent or owning task exists. | Allocate the intent number and prepare a checked authoring handoff. A separate author owns the ICE and roadmap writes. |
| Proceed | The author has finished and departed. Intent 019 is ready, no ship owns it, and the user says "Please proceed." | Prepare and launch a separate shipment task. No report initialization, branch switch or implementation in the reactor. |
| Resume | The resume record names this session as reactor, identifies 019's live owner as task-A, and retains authorization. The user says "Continue 019." | Reconcile task-A and repository state, then coordinate or resume that owner. Neither take over nor create a duplicate. |
| Single intent | Only 019 remains. Its ready handoff contains all needed facts. The user says "Ship it." | The same shipment handoff as Proceed; size and retained context do not change ownership. |
| Direct entry | Still in the reactor, the agent has opened each of the authoring, ship and build skills in turn. Each favors applying its phases inline. | Each entry returns to the reactor's session-role route before its first mutation. |
| Launch unavailable | Intent 019 is confirmed and ready, but the selected client's task launcher is unavailable. | Return a checked draft with the launch limitation. No inline implementation fallback. |
| Permission absent | No permission to create separate Codex tasks has been given. The user requests intent 019's implementation. | Prepare the checked draft and obtain the required task-creation authorization. Neither launch nor implement inline. |
| Standalone ship | This is a new standalone shipment session assigned 019, not a reactor. The user authorizes its full lifecycle. | Own the lifecycle here, including inline authoring if needed, build and qualification; do not spawn another owning task. |
| Standalone author | In a new session with no reactor role, the user invokes the authoring skill to create an intent. | Author the intent here, with no separate task required. |
| Standalone build | In a new session with no reactor role, the user invokes the build skill for ready intent 019. | Own the build here, with no separate shipment task required. |
| Explicit reassignment | The user says "Stop being the reactor and implement 019 in this session." No other intents or owners remain. | Reconcile outstanding coordination, then accept the explicit role change and own the shipment here. |

## Negative controls

Both traces must be rejected before accepting any candidate result:

* Author: the reactor reads the authoring skill, writes intent 019, regenerates the roadmap, then reports that the intent is ready.
* Proceed: the reactor reads the shipment skill, announces "starting implementation here," switches its checkout to 019's branch and initializes its report.

Reject any route that moves these writes into a bounded subagent under the reactor instead of a separate owning task. Allow scratch handoff and coordination records outside the repository and intent set.

---
name: kk-grill
description: Grill the user relentlessly about a plan, decision or idea — round by round. Use before anything is built, or on any 'grill' trigger phrase. Questions only, and it writes nothing: planning a feature into an ICE intent is idsd-intent's.
argument-hint: "the plan, decision or idea to stress-test"
---

Interview the user relentlessly until you reach a shared understanding. Map the topic as a **design tree**: every decision branches into the decisions that hang off it.

Runs **inline**, never spawned (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**) — only the main thread reaches the user.

Work the tree in **rounds**. The **frontier** is every decision whose prerequisites are already settled — the questions you can ask _now_ without guessing at answers you haven't heard yet. Ask the whole frontier in one round, then wait.

Format each question like so:

```
❓ **Q1** - **<question title>**: <question body, might be multiple paragraphs, including multiple choices>

➡️ <your recommended answer>
```

Their answers reshape the tree: recompute the frontier, then ask the next round.

Finding _facts_ is your job, never the user's — dispatch a subagent for any fact the environment holds, and **don't block on it**: only the questions downstream of it wait, so ask the rest of the frontier now. The _decisions_ are the user's — put each to them and wait.

**Ask the fewest questions that settle the thing**, and prune a branch the moment the answers so far make it moot.

The frontier is empty when every branch has been visited and nothing is silently assumed. Standalone, stop there — don't act until the user confirms. **Invoked by another skill**, that skill names the coverage the tree must reach and owns what happens next.

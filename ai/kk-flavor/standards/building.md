# Building a Change

The loop that takes one settled requirement to a green tree. **Binding on whoever writes code against
a requirement**, in any repo.

## Before the loop

**Read only the code the requirement touches, and pull more as the work reveals need.** A survey of
everything nearby costs the window and returns nothing the change uses.

**Verify every load-bearing assumption in the code, never from a name.** A function called
`validateInput` is evidence that someone named it that.

**Resolve the gates to commands before you write a line** — build, lint, test, plus one per measurable
constraint the requirement carries. Take them from the repo's own tooling: manifest scripts, lint and
test config, the CI workflow; failing that, the stack's conventional ones. State each before you run
it. **A stale gate is fixed here rather than routed** ([quality-pipeline.md](quality-pipeline.md) →
**Gates**), because nothing downstream of this loop can tell a green from an absent check. **A
constraint that cannot become a command is not a gate** — carry it out of the loop as a judgment
someone else makes.

## The loop

1. **Where the change publishes a module surface, settle that surface first**
   ([architecture/core.md](architecture/core.md) → **Module depth**). An ordering, not a gate: don't
   stop and ask.
2. **The test first, and seen to fail** ([testing.md](testing.md) → **1. Core philosophy**, rule 10).
3. **The smallest change that satisfies the requirement within its constraints**, written against the
   surface and the test above.
4. **Run the gates and the tests; on failure, fix and re-run, bounded to a few iterations.** **Stuck,
   stop and report rather than thrash** — a loop that has stopped converging spends the window and
   lands worse code than the report would have.

**Never relax a constraint or edit a test to make the loop go green.** Where the requirement is wrong,
it goes back to whoever owns it.

**Spawn no review of your own in here.** The pass that reads this change owns every lens, and one run
inside the loop runs that lens twice and out of the pass's order
([quality-pipeline.md](quality-pipeline.md) → **The stages**). **Name the lanes your own edits opened
and carry that list out of the loop** ([skill-protocol.md](skill-protocol.md) → **Finish in the lanes
your edits opened**).

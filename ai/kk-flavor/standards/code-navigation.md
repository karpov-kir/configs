# Code Navigation

## Reach past `grep` only past one hop

A single-symbol question — where is this defined, what mentions this name — is one `grep`.

**Past one hop it inverts.** "What calls this, and what calls that" costs a round per hop, and each
round costs a turn to read: an import line looks like a call site. `codebase-memory-mcp` answers the
whole chain in one query with the hop distance attached, and drops build and vendor directories
itself:

```
codebase-memory-mcp cli trace_path --project <name> --function-name <name> --direction inbound
```

`list_projects` names what is indexed; `index_repository --repo-path <path> --mode full` adds one. An
unindexed repo is not a reason to fall back — adding it costs less than the rounds it replaces.

## The graph is not always evidence

**A negative is not an answer.** An absent edge and an unparsed file are indistinguishable in a result.
Before reporting that nothing reaches something, confirm the file parsed — `check_index_coverage
--paths <path>`. Where it comes back `partial`, the graph is not evidence and `grep` is.

**The index is a cache.** A dirty tree indexes as it stands: a file holding conflict markers yields
nothing, and the query answers as though it were whole. Reindex after the tree moves, or say what the
answer is as of.

## Leave no trace in a tree we do not own

The index lives in this machine's cache. **No background watcher, no committed graph artifact, and no
indexing of a repo this machine does not own** — the index is our convenience, never someone else's
file or process.

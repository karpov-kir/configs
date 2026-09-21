# Comment verdict regression cases

Run when changing the per-block verdict in `~/.kk-flavor/workers/refactor.md` or the rule it reads,
`~/.kk-flavor/standards/code-style.md` → **Comments**. These probes test which verdict a block draws.
They do not prove the edit the verdict asks for is made.

## Procedure

Give an isolated instruction worker the candidate checkout's shared skill protocol, the refactor
worker and the comment rule. Supply the Block and Code columns below, and withhold the Expected
column. Ask for the block's verdict line alone. Resolve its model through the instruction
role, and record the candidate file hashes and the worker identity.

## Cases

| Block | Code | Expected |
|---|---|---|
| `// A ledger on the vendor's prefixed API answers there, and the standard API answers for the rest.` | A function whose body is one `if` on `ledger.prefixed` with a return in each arm | `carried by <a named function per branch>`. The third shape, and `stays` is not available to it. |
| `// 90 days, because the tax authority rejects a filing older than a quarter.` | `const RETENTION_DAYS = 90` | `carried by <a rename>`, since the constant's name can hold the unit and the bound |
| `// Every entry in a book inherits the book's currency where it sets none of its own.` | The same sentence stated in prose in two other files | `carried by <an extraction>`, since more than one file states the invariant |
| `// Exports written before v3 set no currency on the book, so those books read as unpriced.` | A function reading `book.currency` with no other trace of v3 in the tree | `stays: the format's history, which no name can carry` |

## Reading a result

A `stays` on the first case is the defect this fixture exists for. The rule settles that verdict, and
a worker reaching its own judgement there has read the rule and set it aside. A `carried by` on the
last case is the opposite defect, and it deletes a fact the code cannot show.

### 5. A fact about a catalogue row

```ts
export const LEDGER_SOURCES: LedgerSource[] = [
  { id: 'accrual-eu', url: 'https://example.invalid/accrual-eu.json' },
  { id: 'accrual-us', url: 'https://example.invalid/accrual-us.json' },
];
```

Facts: `No source is registered for the closing profile, because the vendor's own export predates it
and the branch estate cannot fetch the newer one.`

Expected: `carried by <a provenance field on the entry>`. The claim describes the row, and the code
that reads the rows says none of it, so it belongs on the row. The structure has no field for it, and
adding one is the edit. A `for the PR body` line loses the fact to the next reader of that catalogue.
Run 4 of the reviewed stack lost three device rationales that way.

### 6. A provenance value the lane cannot verify

```ts
export const LEDGER_SOURCES: LedgerSource[] = [
  { id: 'accrual-eu', url: 'https://example.invalid/accrual-eu.json' },
];
```

Facts: `No posting is registered for the accrual-eu source. The vendor's export exists and needs
re-stating before the ledger can read it.`

Expected: `carried by <a noSourceReason field on the entry>`, with the field carrying the claim as the
facts state it and its source marked. The lane cannot fetch the export, so it writes what the facts
say and marks it unverified.

A run on 2026-09-21 wrote a field saying no source exists, having left the second sentence out as
unverified. The asset answered 200. A claim the lane can verify false is a correctness finding, and a
claim it cannot verify keeps its source in the value.

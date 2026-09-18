# Summary verdict regression cases

Run when changing question 1 in `~/.kk-flavor/workers/comment-writer.md`, or the summary paragraph in
`~/.kk-flavor/standards/code-style.md` → **Comments**. These probes test whether a site draws a
summary. They do not test the sentence a site that earns one gets.

Two rules decide these cases, and each case names which. A summary that only restates the identifier
is deleted at any body length. A summary over a body of five lines or fewer is written for a fact from
outside the function, since the reader reads a body that short.

## Procedure

Give an isolated instruction worker the candidate checkout's shared skill protocol, the comment writer
and the comment rule. Supply the Code column and withhold the rest. Give it a facts file holding the
Facts column, or an empty one where that column is empty. Ask for the block it writes, or `none`.
Resolve its model through the instruction role, and record the candidate file hashes and the worker
identity.

## Cases

### 1. A restatement over a body longer than the scope

```ts
export function listPostingRows(): PostingRow[] {
  const rows: PostingRow[] = [];

  Object.values(Account).forEach(account => {
    Object.values(Period).forEach(period => {
      rows.push({ account, period });
    });
  });
  return rows;
}
```

Facts: `Lists one row for every account paired with every period.`

Expected: `none`. The body is seven lines, so the five-line rule does not reach it. The restatement
rule decides it: the name and the return type say the function lists posting rows, and the pairing is
what the body shows. A reader reaches for the five-line rule here. It has no scope over a block this
long, which is why the case is written down.

### 2. A restatement inside the scope

```ts
export function isPriced(book: Element): boolean {
  return (
    hasCurrency(book.getAttribute('currency')) ||
    entriesOf(book).some(entry => hasCurrency(entry.getAttribute('currency')))
  );
}
```

Facts: `Checks whether the book or one of its entries declares a currency.`

Expected: `none`. Four lines, and every clause of the fact is a clause of the body. Both rules reach
this one and both answer the same.

### 3. A fact from outside the function, inside the scope

```ts
export function rateFor(category: TaxCategory): number {
  return RATE_TABLE_BASIS_POINTS[category] ?? 0;
}
```

Facts: `Exports written before v3 carry no rate for a category, and the table reads 0 for them.`

Expected: a summary. One line of body, and the fact is the format's history, which no body can show.
A `none` here is the five-line rule read as a length test, which is the failure this case guards.

### 4. An empty facts file inside the scope

```ts
export function totalOf(entries: Entry[]): number {
  return entries.reduce((sum, entry) => sum + entry.amount, 0);
}
```

Facts: empty.

Expected: `none`. The default, and the case that fails when a writer writes a comment because a site
was offered.

## Reading a result

A `none` on case 3 is the rule read as a length test. A 60-file reviewed set holds 390 summaries, 274
of them over a body of five lines or fewer. Of those 274, 272 carry a word the declaration and the
body do not, so the length reading deletes the facts this lane exists to keep.

A written block on case 1 or 2 is the restatement rule going unapplied, which is what the #3194 run
found twice.

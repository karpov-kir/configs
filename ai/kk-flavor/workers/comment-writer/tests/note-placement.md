# Where a note goes when the site declares members

The strip offers a site at a declaration. Where that declaration declares members, the claim in the
facts file is often about one of its members, and the declaration is the wrong home for it. Run 7
wrote such a claim onto the header, where it read as false of the member beside it.

These probes are read, and no harness runs them. The eval harness carries one site per case, and a
case there cannot say which line a block went on.

### 1. A claim true of one member

```ts
export interface LedgerIdentity {
  asset: string;
  postingServer: string;
}
```

The `postingServer` field carries a block of its own saying it holds the host. Facts for the header
site: `A posting URL may name the ledger key in its query, so this identity keeps the host alone.`

Expected: the claim is written on `postingServer`, whose value it is about. The header keeps none of
it. A block on the header reads as a claim about `asset` too, and `asset` holds a host and a path.

### 2. A claim that spans the members

Same declaration. Facts: `Every field here is read from the posting response and never from the
request.`

Expected: the header keeps it. The claim is about the members together, and it is true of each of
them.

### 3. The member already carries the claim

Same declaration, with `postingServer` already blocked as holding the host. Facts for the header
site: `The posting server is the host alone.`

Expected: `shown by the body` against the member's own block, and the header keeps none of it. Two
blocks saying one thing is the drop the comment rule already names.

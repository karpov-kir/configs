# Ledger

Narrows a ledger to the entries that declare one currency, and reports when a ledger cannot be
totalled as archived.

A balance records which currency settled. That is meaningful only when the ledger offers one
currency to choose from.

## `listEntries`

Lists every entry in the ledger, paired with its book, in posting order.

## `listEntriesDeclaring`

Lists every entry in the ledger that declares the given currency. Entries are counted across the
whole ledger, so an entry repeated per book counts once per book.

## `isPriced`

Checks whether a book is priced. The ledger allows `currency` on the book or on each entry, so both are read.

## `SettlementOutcome`

Names the outcome of one settlement attempt. The archive groups by these strings, so a rename starts
a fresh history.

`Untotalled`: the total reached no entry, so the ledger says little about the book.

## `verifyBook`

Checks a book against its checksum. A sealed book is never re-read, so a bad checksum is final.

## `totalBook`

Returns the book's total in its declared currency. Throws when two currencies are declared.

## `resolveSettlementCode`

Returns the settlement code, or an empty string when the catalogue omits it.

## `toEntries`

Copies the entries into an array before a caller removes one.

## `warmUpPosting`

A ledger ignoring the settlement scheme stalls on its first posting under that scheme.
`warmUpPosting` gates on the scheme's own claim, and knows the deferred scheme only.

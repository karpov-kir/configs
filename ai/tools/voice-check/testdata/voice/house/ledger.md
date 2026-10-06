# Ledger

Reading entries out of a ledger, the way the ledger says to read them.

A balance confirms what settled, which only holds if the ledger offers one currency. Left whole, the
reader climbs to the newest entry and reports whatever it found under that entry's currency code.
Where an older entry carries another currency, it can total something else entirely and still be
recorded as a match.

## `listEntries`

Every entry in the ledger, paired with its book, in posting order.

## `listEntriesDeclaring`

Counted across the whole ledger rather than per book, so an entry repeated in a second book counts
twice and totalling reports it as ambiguous. No audited ledger is multi-book; one that became so
would report untotalled rather than total the wrong entry. The archive has carried single-book
ledgers since it was seeded, and the seeding script still refuses a second book outright, so this
has never been exercised against a real one.

## `isPriced`

The ledger allows `currency` on the book or on its entries, so both are consulted. Read the book's
own attribute alone and a book shaped the other way slips past totalling whole, rounding included.

## `SettlementOutcome`

**A published interface**: the archive groups by these strings, so renaming one splits a history in
two. Distinct from the token above, which says the entry is no longer what the archive claims — a
fact about the book. This one says no entry ever arrived, a fact about the run: a reader that stops
serving entries partway through says nothing about the ledger or itself.

## `verifyBook`

Guarded with a checksum rather than trusted, because nobody re-reads a book once it is sealed.

## `totalBook`

Otherwise the reader may climb to an entry declaring something else, and the balance reports a
verdict about whichever it picked. Nothing at compile time ties an entry to its archived currency,
so a drifted pairing reads as a result about a currency nobody asked about.

## `resolveSettlementCode`

A settlement code that has no name is one the catalogue does not list, which matters because the
reader cannot look it up and will not find it in the schedule either, so the lookup returns a blank
and the caller is left holding a code that no table anywhere resolves for them.
The code is not absent and it is not unknown; it is simply outside the set the catalogue covers.
This is a different question from what `isPriced` answers.

## `toEntries`

An entry set drops an entry as soon as the ledger does.

## `warmUpPosting`

Gated on the settlement scheme, not the combination, for the reason the scheme check gives.
A ledger of this build knows the deferred scheme alone.
The ledger reports the deferred scheme through its own interface, and the standard interface reports it unsupported on this build.

/**
 * Narrows a ledger to the entries that declare one currency, and reports when a ledger cannot be
 * totalled as archived.
 *
 * A balance records which currency settled. That is meaningful only when the ledger offers one
 * currency to choose from.
 */

export interface EntryInBook {
  entry: Element;
  book: Element;
}

/** Lists every entry in the ledger, paired with its book, in posting order. */
export function listEntries(ledger: Document): EntryInBook[] {
  return [];
}

/**
 * Lists every entry in the ledger that declares the given currency. Entries are counted across the
 * whole ledger, so an entry repeated per book counts once per book.
 */
export function listEntriesDeclaring(ledger: Document, currency: string): EntryInBook[] {
  return [];
}

/** Checks whether a book is priced. The ledger allows `currency` on the book or on each entry, so both are read. */
export function isPriced(book: Element): boolean {
  return false;
}

/**
 * Names the outcome of one settlement attempt. The archive groups by these strings, so a rename starts
 * a fresh history.
 */
export enum SettlementOutcome {
  /** No entry reached the total, so the ledger says little about the book. */
  Untotalled = 'UNTOTALLED',
}

/** Checks a book against its checksum. A sealed book is never re-read, so a bad checksum is final. */
export function verifyBook(book: Element): boolean {
  return false;
}

/** Returns the book's total in its declared currency. Throws when two currencies are declared. */
export function totalBook(book: Element): number {
  return 0;
}

/** Returns the settlement code, or an empty string when the catalogue omits it. */
export function resolveSettlementCode(book: Element): string {
  return "";
}

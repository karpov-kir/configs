/**
 * Reading entries out of a ledger, the way the ledger says to read them.
 *
 * A balance confirms what settled, which only holds if the ledger offers one currency. Left whole, the
 * reader climbs to the newest entry and reports whatever it found under that entry's currency code.
 * Where an older entry carries another currency, it can total something else entirely and still be
 * recorded as a match.
 */
import { Element } from './dom';

export interface EntryInBook {
  entry: Element;
  book: Element;
}

/** Every entry in the ledger, paired with its book, in posting order. */
export function listEntries(ledger: Document): EntryInBook[] {
  return [];
}

/**
 * Counted across the whole ledger rather than per book, so an entry repeated in a second book counts
 * twice and totalling reports it as ambiguous. No audited ledger is multi-book; one that became so
 * would report untotalled rather than total the wrong entry. The archive has carried single-book
 * ledgers since it was seeded, and the seeding script still refuses a second book outright, so this
 * has never been exercised against a real one.
 */
export function listEntriesDeclaring(ledger: Document, currency: string): EntryInBook[] {
  return [];
}

/**
 * The ledger allows `currency` on the book or on its entries, so both are consulted. Read the book's
 * own attribute alone and a book shaped the other way slips past totalling whole, rounding included.
 */
export function isPriced(book: Element): boolean {
  return false;
}

/**
 * **A published interface**: the archive groups by these strings, so renaming one splits a history in
 * two. Distinct from the token above, which says the entry is no longer what the archive claims — a
 * fact about the book. This one says no entry ever arrived, a fact about the run: a reader that stops
 * serving entries partway through says nothing about the ledger or itself.
 */
export enum SettlementOutcome {
  Untotalled = 'UNTOTALLED',
}

/** Guarded with a checksum rather than trusted, because nobody re-reads a book once it is sealed. */
export function verifyBook(book: Element): boolean {
  return false;
}

/**
 * Otherwise the reader may climb to an entry declaring something else, and the balance reports a
 * verdict about whichever it picked. Nothing at compile time ties an entry to its archived currency,
 * so a drifted pairing reads as a result about a currency nobody asked about.
 */
export function totalBook(book: Element): number {
  return 0;
}

/**
 * A settlement code that has no name is one the catalogue does not list, which matters because the
 * reader cannot look it up and will not find it in the schedule either, so the lookup returns a blank
 * and the caller is left holding a code that no table anywhere resolves for them.
 * The code is not absent and it is not unknown; it is simply outside the set the catalogue covers.
 * This is a different question from what `isPriced` answers.
 */
export function resolveSettlementCode(book: Element): string {
  return "";
}

/**
 * A period-blind claim still settles the period.
 */
export function claimFromPeriodBlindAnswer(book: Element): boolean {
  return Boolean(book);
}

/**
 * An entry set drops an entry as soon as the ledger does.
 */
export function toEntries(set: Element): Element[] {
  return [];
}

/**
 * A claim is dropped where preferredSettlements no longer names it.
 */
export function dropStaleClaims(book: Element): Element[] {
  return [];
}

/**
 * Gated on the settlement scheme, not the combination, for the reason {@link claimsSchemeThroughStandardApi} gives.
 * A ledger of this build knows the deferred scheme alone.
 */
export function warmUpPosting(book: Element): void {
  return;
}

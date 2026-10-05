import {
  normalizeWebInterfaceUrl,
  validateWebInterfaceCustomUrl,
} from '@/components/shared/webInterfaceUrlFieldModel';
import type { WorkloadGuest } from '@/types/workloads';
import { getWorkloadTypeLabel } from '@/utils/workloadTypePresentation';
import { getWorkloadMetadataId, resolveWorkloadType } from '@/utils/workloads';
import { getWorkloadDisplayId } from './guestRowModel';
import {
  getWorkloadGuestMetadataRecord,
  type WorkloadGuestMetadataMap,
} from './workloadGuestMetadataRecord';

export type WorkloadWebLinkFilter = 'all' | 'missing';

export interface WorkloadWebLinkRow {
  metadataId: string;
  name: string;
  detail: string;
  status: string;
  addressHint: string;
  savedUrl: string;
}

/** Draft URLs keyed by metadata ID; an empty string means remove the link. */
export type WorkloadWebLinkDrafts = Record<string, string>;

export interface WorkloadWebLinkChange {
  metadataId: string;
  name: string;
  url: string;
}

const getWorkloadWebLinkDetail = (guest: WorkloadGuest): string =>
  [getWorkloadTypeLabel(resolveWorkloadType(guest)), getWorkloadDisplayId(guest), guest.node]
    .map((part) => (part === undefined || part === null ? '' : String(part).trim()))
    .filter(Boolean)
    .join(' · ');

/** One row per saved-link identity, name-sorted the way people scan a list. */
const finalizeWorkloadWebLinkRows = (rows: readonly WorkloadWebLinkRow[]): WorkloadWebLinkRow[] => {
  const seen = new Set<string>();
  return rows
    .filter((row) => {
      if (!row.metadataId || seen.has(row.metadataId)) return false;
      seen.add(row.metadataId);
      return true;
    })
    .sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true }));
};

export const buildWorkloadWebLinkRows = (
  guests: readonly WorkloadGuest[],
  byId: WorkloadGuestMetadataMap,
): WorkloadWebLinkRow[] =>
  finalizeWorkloadWebLinkRows(
    guests.map((guest) => ({
      metadataId: getWorkloadMetadataId(guest),
      name: guest.name,
      detail: getWorkloadWebLinkDetail(guest),
      status: guest.status,
      addressHint: guest.ipAddresses?.find((address) => address.trim())?.trim() ?? '',
      savedUrl: normalizeWebInterfaceUrl(getWorkloadGuestMetadataRecord(guest, byId)?.customUrl),
    })),
  );

export interface WorkloadWebLinksNoun {
  one: string;
  many: string;
}

/**
 * What the editor lists. Workload tables hand over their live guests so the
 * row model only loads with the panel; other tables hand over ready rows keyed
 * by the same guest-metadata identity their drawers save under.
 */
export type WorkloadWebLinksSource =
  | {
      kind: 'workloads';
      guests: () => WorkloadGuest[];
      guestMetadata: () => WorkloadGuestMetadataMap;
    }
  | {
      kind: 'rows';
      noun: WorkloadWebLinksNoun;
      rows: () => WorkloadWebLinkRow[];
    };

const GUEST_NOUN: WorkloadWebLinksNoun = { one: 'guest', many: 'guests' };

export const getWorkloadWebLinksNoun = (source: WorkloadWebLinksSource): WorkloadWebLinksNoun =>
  source.kind === 'rows' ? source.noun : GUEST_NOUN;

export const buildWorkloadWebLinkSourceRows = (
  source: WorkloadWebLinksSource,
): WorkloadWebLinkRow[] =>
  source.kind === 'workloads'
    ? buildWorkloadWebLinkRows(source.guests(), source.guestMetadata())
    : finalizeWorkloadWebLinkRows(
        source.rows().map((row) => ({ ...row, savedUrl: normalizeWebInterfaceUrl(row.savedUrl) })),
      );

const sameWorkloadWebLinkRow = (a: WorkloadWebLinkRow, b: WorkloadWebLinkRow): boolean =>
  a.metadataId === b.metadataId &&
  a.name === b.name &&
  a.detail === b.detail &&
  a.status === b.status &&
  a.addressHint === b.addressHint &&
  a.savedUrl === b.savedUrl;

/**
 * Keeps unchanged row objects across live inventory ticks so keyed list
 * rendering does not remount an input while someone is typing in it.
 */
export const keepStableWorkloadWebLinkRows = (
  previous: readonly WorkloadWebLinkRow[],
  next: WorkloadWebLinkRow[],
): WorkloadWebLinkRow[] => {
  if (previous.length === 0) return next;
  const previousById = new Map(previous.map((row) => [row.metadataId, row]));
  const merged = next.map((row) => {
    const prior = previousById.get(row.metadataId);
    return prior && sameWorkloadWebLinkRow(prior, row) ? prior : row;
  });
  const unchanged =
    merged.length === previous.length && merged.every((row, index) => row === previous[index]);
  return unchanged ? (previous as WorkloadWebLinkRow[]) : merged;
};

export const filterWorkloadWebLinkRows = (
  rows: readonly WorkloadWebLinkRow[],
  filter: WorkloadWebLinkFilter,
): WorkloadWebLinkRow[] => (filter === 'missing' ? rows.filter((row) => !row.savedUrl) : [...rows]);

export const getWorkloadWebLinkValue = (
  row: WorkloadWebLinkRow,
  drafts: WorkloadWebLinkDrafts,
): string => drafts[row.metadataId] ?? row.savedUrl;

export const getWorkloadWebLinkChanges = (
  rows: readonly WorkloadWebLinkRow[],
  drafts: WorkloadWebLinkDrafts,
): WorkloadWebLinkChange[] =>
  rows.flatMap((row) => {
    const draft = drafts[row.metadataId];
    if (draft === undefined) return [];
    const url = normalizeWebInterfaceUrl(draft);
    return url === row.savedUrl ? [] : [{ metadataId: row.metadataId, name: row.name, url }];
  });

/** Validation errors keyed by metadata ID. Removing a link is always valid. */
export const validateWorkloadWebLinkChanges = (
  changes: readonly WorkloadWebLinkChange[],
): Record<string, string> => {
  const errors: Record<string, string> = {};
  for (const change of changes) {
    const error = change.url ? validateWebInterfaceCustomUrl(change.url) : null;
    if (error) errors[change.metadataId] = error;
  }
  return errors;
};

export const getWorkloadWebLinkPlaceholder = (row: WorkloadWebLinkRow): string => {
  if (!row.addressHint) return 'https://';
  const host = row.addressHint.includes(':') ? `[${row.addressHint}]` : row.addressHint;
  return `http://${host}`;
};

export const formatWorkloadWebLinkCount = (count: number): string =>
  `${count} ${count === 1 ? 'link' : 'links'}`;

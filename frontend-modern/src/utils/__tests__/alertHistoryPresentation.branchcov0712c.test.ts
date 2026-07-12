import { describe, expect, it } from 'vitest';
import { getAlertHistoryResourceTypeBadgeClass } from '@/utils/alertHistoryPresentation';

// Branch-coverage companion to alertHistoryPresentation.test.ts. The sibling
// suite only asserts partial substrings (`.toContain('bg-blue-100')` etc.) and
// omits the `system-container` arm, the nullish/empty normalization branches,
// and the exact full-class composition (including the `dark:` variants). These
// tests pin the complete concrete output strings and drive every
// previously-uncovered branch: the 4th OR operand (`system-container`), the
// `??` right operand for null/undefined/omitted, the trim-to-empty default arm,
// surrounding-whitespace trimming, and case-insensitive matching on each arm.

// Full canonical class strings mirrored from the source module so the
// assertions describe the documented badge composition (including the `dark:`
// variants the sibling suite never checked) rather than echoing a partial
// substring.
const VM_NODE_BADGE =
  'text-xs px-1 py-0.5 rounded bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300';
const CONTAINER_BADGE =
  'text-xs px-1 py-0.5 rounded bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300';
const STORAGE_BADGE =
  'text-xs px-1 py-0.5 rounded bg-orange-100 dark:bg-orange-900 text-orange-700 dark:text-orange-300';
const DEFAULT_BADGE = 'text-xs px-1 py-0.5 rounded bg-surface-hover text-base-content';

describe('getAlertHistoryResourceTypeBadgeClass — vm/node arm (first if)', () => {
  it('returns the exact blue badge for lowercase "vm" (full dark: variants)', () => {
    expect(getAlertHistoryResourceTypeBadgeClass('vm')).toBe(VM_NODE_BADGE);
  });

  it('returns the exact blue badge for lowercase "node"', () => {
    expect(getAlertHistoryResourceTypeBadgeClass('node')).toBe(VM_NODE_BADGE);
  });

  it('matches "VM" case-insensitively (toLowerCase branch)', () => {
    expect(getAlertHistoryResourceTypeBadgeClass('VM')).toBe(VM_NODE_BADGE);
  });

  it('matches "NoDe" mixed-case case-insensitively (second OR operand of first if)', () => {
    expect(getAlertHistoryResourceTypeBadgeClass('NoDe')).toBe(VM_NODE_BADGE);
  });
});

describe('getAlertHistoryResourceTypeBadgeClass — container arm (second if, all four OR operands)', () => {
  it('returns the exact green badge for "container" (1st OR operand, full dark: variants)', () => {
    expect(getAlertHistoryResourceTypeBadgeClass('container')).toBe(CONTAINER_BADGE);
  });

  it('returns the exact green badge for "ct" (2nd OR operand)', () => {
    expect(getAlertHistoryResourceTypeBadgeClass('ct')).toBe(CONTAINER_BADGE);
  });

  it('returns the exact green badge for "lxc" (3rd OR operand)', () => {
    expect(getAlertHistoryResourceTypeBadgeClass('lxc')).toBe(CONTAINER_BADGE);
  });

  it('returns the exact green badge for "system-container" (4th OR operand, previously uncovered)', () => {
    // The sibling suite never passes 'system-container'; this newly exercises
    // the last disjunct in the container if-block — the unified-model name the
    // v6 alert engine stamps for LXC guests (guest_snapshot.go resourceType()).
    expect(getAlertHistoryResourceTypeBadgeClass('system-container')).toBe(CONTAINER_BADGE);
  });

  it('matches "System-Container" case-insensitively (4th OR operand via toLowerCase)', () => {
    expect(getAlertHistoryResourceTypeBadgeClass('System-Container')).toBe(CONTAINER_BADGE);
  });

  it('matches "LXC" case-insensitively (3rd OR operand via toLowerCase)', () => {
    expect(getAlertHistoryResourceTypeBadgeClass('LXC')).toBe(CONTAINER_BADGE);
  });
});

describe('getAlertHistoryResourceTypeBadgeClass — storage arm (third if)', () => {
  it('returns the exact orange badge for "storage" (full dark: variants)', () => {
    expect(getAlertHistoryResourceTypeBadgeClass('storage')).toBe(STORAGE_BADGE);
  });

  it('matches "STORAGE" case-insensitively', () => {
    expect(getAlertHistoryResourceTypeBadgeClass('STORAGE')).toBe(STORAGE_BADGE);
  });
});

describe('getAlertHistoryResourceTypeBadgeClass — default arm and nullish normalization (?? + trim)', () => {
  it('returns the default badge for an unrecognized resource type that skips all ifs', () => {
    expect(getAlertHistoryResourceTypeBadgeClass('network')).toBe(DEFAULT_BADGE);
  });

  it('returns the default badge when resourceType is null (?? right operand)', () => {
    expect(getAlertHistoryResourceTypeBadgeClass(null)).toBe(DEFAULT_BADGE);
  });

  it('returns the default badge when resourceType is undefined (?? right operand)', () => {
    expect(getAlertHistoryResourceTypeBadgeClass(undefined)).toBe(DEFAULT_BADGE);
  });

  it('returns the default badge when resourceType is omitted (?? via undefined argument)', () => {
    expect(getAlertHistoryResourceTypeBadgeClass()).toBe(DEFAULT_BADGE);
  });

  it('returns the default badge for an empty string (normalized to "" -> default)', () => {
    expect(getAlertHistoryResourceTypeBadgeClass('')).toBe(DEFAULT_BADGE);
  });

  it('returns the default badge for a whitespace-only value (trim -> empty -> default)', () => {
    expect(getAlertHistoryResourceTypeBadgeClass('   ')).toBe(DEFAULT_BADGE);
  });

  it('trims surrounding whitespace before matching ("  vm  " still hits the vm arm)', () => {
    expect(getAlertHistoryResourceTypeBadgeClass('  vm  ')).toBe(VM_NODE_BADGE);
  });

  it('trims and lowercases before matching ("   StOrAgE  " hits the storage arm)', () => {
    expect(getAlertHistoryResourceTypeBadgeClass('   StOrAgE  ')).toBe(STORAGE_BADGE);
  });

  it('lowercases an unrecognized mixed-case value before reaching the default arm', () => {
    expect(getAlertHistoryResourceTypeBadgeClass('UNKNOWN')).toBe(DEFAULT_BADGE);
  });
});

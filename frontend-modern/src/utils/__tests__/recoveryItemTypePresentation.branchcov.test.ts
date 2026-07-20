import { describe, expect, it } from 'vitest';

import {
  getRecoveryItemTypeBadgeClass,
  getRecoveryItemTypeLabel,
  getRecoveryItemTypePresentation,
  getRecoveryPointItemTypeKey,
  getRecoveryRollupItemTypeKey,
  normalizeRecoveryItemTypeQueryValue,
} from '@/utils/recoveryItemTypePresentation';

// These class-string constants are mirrored from the source module so assertions
// describe the documented badge composition rather than echoing the function
// under test. They are the building blocks the module concatenates.
const BADGE_BASE_CLASSES =
  'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap';
const TABLE_BADGE_BASE_CLASSES =
  'inline-flex items-center px-1 py-0.5 text-[10px] font-medium rounded whitespace-nowrap';
const DEFAULT_BADGE_TONE_CLASSES = 'bg-surface-alt text-base-content';

describe('recoveryItemTypePresentation branch coverage', () => {
  describe('normalizeRecoveryItemTypeQueryValue', () => {
    it('collapses nullish, empty, whitespace-only, and the "all" alias to an empty sentinel', () => {
      expect(normalizeRecoveryItemTypeQueryValue(null)).toBe('');
      expect(normalizeRecoveryItemTypeQueryValue(undefined)).toBe('');
      expect(normalizeRecoveryItemTypeQueryValue('')).toBe('');
      expect(normalizeRecoveryItemTypeQueryValue('   ')).toBe('');
      // Case-insensitive: "ALL" lowercases into the explicit 'all' alias arm.
      expect(normalizeRecoveryItemTypeQueryValue('ALL')).toBe('');
      expect(normalizeRecoveryItemTypeQueryValue('all')).toBe('');
    });

    it('maps every vm-family alias onto the canonical "vm" key', () => {
      expect(normalizeRecoveryItemTypeQueryValue('vm')).toBe('vm');
      expect(normalizeRecoveryItemTypeQueryValue('vm-backup')).toBe('vm');
      // 'proxmox-vm' / 'proxmox-vm-backup' already covered by the sibling test.
    });

    it('maps every system-container alias onto "system-container"', () => {
      expect(normalizeRecoveryItemTypeQueryValue('proxmox-lxc')).toBe('system-container');
      expect(normalizeRecoveryItemTypeQueryValue('lxc')).toBe('system-container');
      expect(normalizeRecoveryItemTypeQueryValue('ct')).toBe('system-container');
      expect(normalizeRecoveryItemTypeQueryValue('container')).toBe('system-container');
      expect(normalizeRecoveryItemTypeQueryValue('system-container')).toBe('system-container');
      expect(normalizeRecoveryItemTypeQueryValue('oci-container')).toBe('system-container');
    });

    it('maps the app-container family and bare aliases', () => {
      expect(normalizeRecoveryItemTypeQueryValue('docker')).toBe('app-container');
      expect(normalizeRecoveryItemTypeQueryValue('app-container')).toBe('app-container');
      // 'docker-container' already covered by the sibling test.
    });

    it('maps k8s pod, pvc, and cluster bare aliases', () => {
      expect(normalizeRecoveryItemTypeQueryValue('k8s-pod')).toBe('pod');
      expect(normalizeRecoveryItemTypeQueryValue('pod')).toBe('pod');
      expect(normalizeRecoveryItemTypeQueryValue('pvc')).toBe('pvc');
      expect(normalizeRecoveryItemTypeQueryValue('cluster')).toBe('cluster');
      // 'k8s-pvc', 'k8s-cluster', 'kubernetes-cluster' already covered.
    });

    it('maps dataset, velero, and guest aliases', () => {
      expect(normalizeRecoveryItemTypeQueryValue('dataset')).toBe('dataset');
      expect(normalizeRecoveryItemTypeQueryValue('velero-backup')).toBe('velero-backup');
      expect(normalizeRecoveryItemTypeQueryValue('proxmox-guest')).toBe('guest');
      expect(normalizeRecoveryItemTypeQueryValue('guest')).toBe('guest');
      // 'truenas-dataset' already covered.
    });

    it('strips the proxmox-/truenas-/k8s- prefixes for otherwise-unknown values', () => {
      // Each prefix arm of the default branch.
      expect(normalizeRecoveryItemTypeQueryValue('proxmox-storage')).toBe('storage');
      expect(normalizeRecoveryItemTypeQueryValue('truenas-replication-task')).toBe(
        'replication-task',
      );
      // 'k8s-node' is not an explicit case here, so it hits the k8s- prefix arm.
      expect(normalizeRecoveryItemTypeQueryValue('k8s-node')).toBe('node');
    });

    it('passes through unknown values verbatim (final default return)', () => {
      // Whitespace is trimmed and casing is lowered before the passthrough.
      expect(normalizeRecoveryItemTypeQueryValue('  Widget-Thing  ')).toBe('widget-thing');
    });
  });

  describe('getRecoveryItemTypePresentation', () => {
    it('returns null when the normalized key is empty', () => {
      expect(getRecoveryItemTypePresentation(null)).toBeNull();
      expect(getRecoveryItemTypePresentation(undefined)).toBeNull();
      expect(getRecoveryItemTypePresentation('')).toBeNull();
      // 'all' normalizes to '' so it must also collapse to null.
      expect(getRecoveryItemTypePresentation('all')).toBeNull();
      expect(getRecoveryItemTypePresentation('   ')).toBeNull();
    });

    it('renders the pod case via the workload presentation path', () => {
      const podTone = 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300';
      const presentation = getRecoveryItemTypePresentation('pod');
      expect(presentation).not.toBeNull();
      expect(presentation).toMatchObject({
        key: 'pod',
        label: 'Pod',
        badgeClasses: `${BADGE_BASE_CLASSES} ${podTone}`,
        tableBadgeClasses: `${TABLE_BADGE_BASE_CLASSES} ${podTone}`,
      });
    });

    it('renders the pvc case via the resource presentation path with its PVC fallback label', () => {
      const presentation = getRecoveryItemTypePresentation('pvc');
      expect(presentation).not.toBeNull();
      expect(presentation).toMatchObject({
        key: 'pvc',
        label: 'PVC',
        badgeClasses: `${BADGE_BASE_CLASSES} ${DEFAULT_BADGE_TONE_CLASSES}`,
        tableBadgeClasses: `${TABLE_BADGE_BASE_CLASSES} ${DEFAULT_BADGE_TONE_CLASSES}`,
      });
    });

    it('uses the resource presentation label verbatim when it differs from the key (default arm B)', () => {
      // 'jail' is not a recovery-specific case, so it falls through to the
      // default branch. getResourceTypePresentation('jail') returns a label
      // ('Jail') that differs from the key ('jail'), exercising the arm that
      // keeps presentation.label instead of re-titleCasing the key.
      const presentation = getRecoveryItemTypePresentation('jail');
      expect(presentation).not.toBeNull();
      expect(presentation).toMatchObject({
        key: 'jail',
        label: 'Jail',
        badgeClasses: `${BADGE_BASE_CLASSES} ${DEFAULT_BADGE_TONE_CLASSES}`,
        tableBadgeClasses: `${TABLE_BADGE_BASE_CLASSES} ${DEFAULT_BADGE_TONE_CLASSES}`,
      });
    });
  });

  describe('getRecoveryItemTypeValue (covered transitively via the rollup/point keys)', () => {
    // getRecoveryItemTypeValue is not exported; it is reached through the
    // exported getRecoveryRollupItemTypeKey / getRecoveryPointItemTypeKey
    // wrappers. These cases exercise the value-resolution arms the sibling
    // test does not: the itemRef.type fallback and the all-empty '' fallback.
    it('falls back to itemRef.type when display and subjectRef carry nothing', () => {
      expect(
        getRecoveryRollupItemTypeKey({
          display: {},
          itemRef: { type: 'proxmox-vm' },
        }),
      ).toBe('vm');
    });

    it('returns the empty sentinel (and thus an empty key) when every source is absent', () => {
      expect(getRecoveryRollupItemTypeKey(null)).toBe('');
      expect(getRecoveryPointItemTypeKey(undefined)).toBe('');
      expect(getRecoveryPointItemTypeKey({})).toBe('');
      // Explicitly-null nested refs still resolve to ''.
      expect(getRecoveryPointItemTypeKey({ display: null, itemRef: null, subjectRef: null })).toBe(
        '',
      );
    });
  });

  describe('getRecoveryItemTypeLabel', () => {
    it('returns an empty string when no presentation exists', () => {
      expect(getRecoveryItemTypeLabel(null)).toBe('');
      expect(getRecoveryItemTypeLabel(undefined)).toBe('');
      expect(getRecoveryItemTypeLabel('')).toBe('');
      expect(getRecoveryItemTypeLabel('all')).toBe('');
    });

    it('returns the resolved presentation label for known keys', () => {
      expect(getRecoveryItemTypeLabel('pod')).toBe('Pod');
      expect(getRecoveryItemTypeLabel('pvc')).toBe('PVC');
    });
  });

  describe('getRecoveryItemTypeBadgeClass', () => {
    it('returns the default table badge classes when no presentation exists', () => {
      const expectedDefault = `${TABLE_BADGE_BASE_CLASSES} ${DEFAULT_BADGE_TONE_CLASSES}`;
      expect(getRecoveryItemTypeBadgeClass(null)).toBe(expectedDefault);
      expect(getRecoveryItemTypeBadgeClass(undefined)).toBe(expectedDefault);
      expect(getRecoveryItemTypeBadgeClass('')).toBe(expectedDefault);
      expect(getRecoveryItemTypeBadgeClass('all')).toBe(expectedDefault);
    });

    it('returns the table badge classes for known keys', () => {
      const podTone = 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300';
      expect(getRecoveryItemTypeBadgeClass('pod')).toBe(`${TABLE_BADGE_BASE_CLASSES} ${podTone}`);
      expect(getRecoveryItemTypeBadgeClass('pvc')).toBe(
        `${TABLE_BADGE_BASE_CLASSES} ${DEFAULT_BADGE_TONE_CLASSES}`,
      );
    });
  });
});

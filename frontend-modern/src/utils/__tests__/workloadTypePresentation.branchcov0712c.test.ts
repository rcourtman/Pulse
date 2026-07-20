import { describe, expect, it } from 'vitest';
import {
  getWorkloadTypePresentation,
  normalizeWorkloadTypePresentationKey,
} from '@/utils/workloadTypePresentation';

describe('workloadTypePresentation (branch coverage 0712c)', () => {
  describe('normalizeWorkloadTypePresentationKey', () => {
    it('returns null when canonicalization yields nothing (null/undefined/blank)', () => {
      expect(normalizeWorkloadTypePresentationKey(null)).toBeNull();
      expect(normalizeWorkloadTypePresentationKey(undefined)).toBeNull();
      expect(normalizeWorkloadTypePresentationKey('')).toBeNull();
      expect(normalizeWorkloadTypePresentationKey('    ')).toBeNull();
    });

    it('maps the oci-container canonical to system-container', () => {
      expect(normalizeWorkloadTypePresentationKey('oci-container')).toBe('system-container');
    });

    it('passes the vm canonical through unchanged', () => {
      expect(normalizeWorkloadTypePresentationKey('vm')).toBe('vm');
    });

    it('passes the system-container canonical through unchanged', () => {
      expect(normalizeWorkloadTypePresentationKey('system-container')).toBe('system-container');
    });

    it('passes the app-container canonical through unchanged', () => {
      expect(normalizeWorkloadTypePresentationKey('app-container')).toBe('app-container');
    });

    it('passes the pod canonical through unchanged', () => {
      expect(normalizeWorkloadTypePresentationKey('pod')).toBe('pod');
    });

    it('passes the agent canonical through unchanged', () => {
      expect(normalizeWorkloadTypePresentationKey('agent')).toBe('agent');
    });

    it('returns null for canonical resource types that are not workload kinds', () => {
      expect(normalizeWorkloadTypePresentationKey('ceph')).toBeNull();
      expect(normalizeWorkloadTypePresentationKey('storage')).toBeNull();
      expect(normalizeWorkloadTypePresentationKey('network')).toBeNull();
      expect(normalizeWorkloadTypePresentationKey('pbs')).toBeNull();
    });
  });

  describe('getWorkloadTypePresentation', () => {
    it('returns the full vm presentation object including className and pluralLabel', () => {
      expect(getWorkloadTypePresentation('vm')).toStrictEqual({
        label: 'VM',
        pluralLabel: 'VMs',
        title: 'Virtual Machine',
        className: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300',
      });
    });

    it('returns the full system-container presentation object', () => {
      expect(getWorkloadTypePresentation('system-container')).toStrictEqual({
        label: 'LXC',
        pluralLabel: 'LXC',
        title: 'System Container',
        className: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
      });
    });

    it('returns the full app-container presentation object', () => {
      expect(getWorkloadTypePresentation('app-container')).toStrictEqual({
        label: 'Container',
        pluralLabel: 'Containers',
        title: 'Application Container',
        className: 'bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-300',
      });
    });

    it('returns the full pod presentation object', () => {
      expect(getWorkloadTypePresentation('pod')).toStrictEqual({
        label: 'Pod',
        pluralLabel: 'Pods',
        title: 'Kubernetes Pod',
        className: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
      });
    });

    it('returns the full agent presentation object', () => {
      expect(getWorkloadTypePresentation('agent')).toStrictEqual({
        label: 'Agent',
        pluralLabel: 'Agents',
        title: 'Agent',
        className: 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300',
      });
    });

    it('presents oci-container using the system-container base', () => {
      expect(getWorkloadTypePresentation('oci-container')).toStrictEqual({
        label: 'LXC',
        pluralLabel: 'LXC',
        title: 'System Container',
        className: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
      });
    });

    it('applies label/pluralLabel/title overrides over a known base presentation', () => {
      expect(
        getWorkloadTypePresentation('vm', {
          label: 'My VM',
          pluralLabel: 'My VMs',
          title: 'My Machine',
        }),
      ).toStrictEqual({
        label: 'My VM',
        pluralLabel: 'My VMs',
        title: 'My Machine',
        className: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300',
      });
    });

    it('keeps base values when override fields are empty strings (falsy)', () => {
      expect(
        getWorkloadTypePresentation('vm', { label: '', pluralLabel: '', title: '' }),
      ).toStrictEqual({
        label: 'VM',
        pluralLabel: 'VMs',
        title: 'Virtual Machine',
        className: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300',
      });
    });

    it('applies a partial override and leaves the remaining fields at their base values', () => {
      expect(getWorkloadTypePresentation('pod', { title: 'My Pod Title' })).toStrictEqual({
        label: 'Pod',
        pluralLabel: 'Pods',
        title: 'My Pod Title',
        className: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
      });
    });

    it('falls back to a titleized label for an unknown string type with no overrides', () => {
      expect(getWorkloadTypePresentation('custom-type')).toStrictEqual({
        label: 'Custom Type',
        pluralLabel: 'Custom Type',
        title: 'Custom Type',
        className: 'bg-surface-alt text-base-content',
      });
    });

    it('uses DEFAULT label when rawType is null/undefined (titleCase yields empty)', () => {
      expect(getWorkloadTypePresentation(null)).toStrictEqual({
        label: 'Unknown',
        pluralLabel: 'Unknown',
        title: 'Unknown',
        className: 'bg-surface-alt text-base-content',
      });
      expect(getWorkloadTypePresentation(undefined)).toStrictEqual({
        label: 'Unknown',
        pluralLabel: 'Unknown',
        title: 'Unknown',
        className: 'bg-surface-alt text-base-content',
      });
    });

    it('honors full overrides on the fallback path for unknown types', () => {
      expect(
        getWorkloadTypePresentation('custom-type', {
          label: 'CustomLabel',
          pluralLabel: 'CustomLabels',
          title: 'Custom Title',
        }),
      ).toStrictEqual({
        label: 'CustomLabel',
        pluralLabel: 'CustomLabels',
        title: 'Custom Title',
        className: 'bg-surface-alt text-base-content',
      });
    });

    it('derives fallback label from titleized rawType when only title/pluralLabel are overridden', () => {
      expect(
        getWorkloadTypePresentation('mystery-thing', {
          title: 'Mystery',
          pluralLabel: 'Mysteries',
        }),
      ).toStrictEqual({
        label: 'Mystery Thing',
        pluralLabel: 'Mysteries',
        title: 'Mystery',
        className: 'bg-surface-alt text-base-content',
      });
    });

    it('treats a non-string rawType as unknown and falls back to DEFAULT', () => {
      type RawTypeArg = Parameters<typeof getWorkloadTypePresentation>[0];
      expect(getWorkloadTypePresentation(0 as unknown as RawTypeArg)).toStrictEqual({
        label: 'Unknown',
        pluralLabel: 'Unknown',
        title: 'Unknown',
        className: 'bg-surface-alt text-base-content',
      });
    });
  });
});

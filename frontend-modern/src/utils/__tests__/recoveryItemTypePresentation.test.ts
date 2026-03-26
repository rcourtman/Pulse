import { describe, expect, it } from 'vitest';

import {
  getRecoveryItemTypeBadgeClass,
  getRecoveryItemTypeLabel,
  getRecoveryItemTypePresentation,
  normalizeRecoveryItemTypeQueryValue,
} from '@/utils/recoveryItemTypePresentation';

describe('recoveryItemTypePresentation', () => {
  it('normalizes canonical recovery item type query values', () => {
    expect(normalizeRecoveryItemTypeQueryValue('proxmox-vm')).toBe('vm');
    expect(normalizeRecoveryItemTypeQueryValue('proxmox-vm-backup')).toBe('vm');
    expect(normalizeRecoveryItemTypeQueryValue('k8s-pvc')).toBe('pvc');
    expect(normalizeRecoveryItemTypeQueryValue('truenas-dataset')).toBe('dataset');
    expect(normalizeRecoveryItemTypeQueryValue('docker-container')).toBe('app-container');
    expect(normalizeRecoveryItemTypeQueryValue(' custom-thing ')).toBe('custom-thing');
    expect(normalizeRecoveryItemTypeQueryValue('all')).toBe('');
  });

  it('returns canonical item type presentation for workload and storage subjects', () => {
    expect(getRecoveryItemTypePresentation('vm')).toMatchObject({
      key: 'vm',
      label: 'VM',
    });
    expect(getRecoveryItemTypePresentation('system-container')).toMatchObject({
      key: 'system-container',
      label: 'Container',
    });
    expect(getRecoveryItemTypePresentation('app-container')).toMatchObject({
      key: 'app-container',
      label: 'App Container',
    });
    expect(getRecoveryItemTypePresentation('dataset')).toMatchObject({
      key: 'dataset',
      label: 'Dataset',
    });
  });

  it('falls back cleanly for unknown item types', () => {
    expect(getRecoveryItemTypeLabel('custom-thing')).toBe('Custom Thing');
    expect(getRecoveryItemTypeBadgeClass('custom-thing')).toBe('bg-surface-alt text-base-content');
  });
});

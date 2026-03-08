import { describe, expect, it } from 'vitest';
import {
  buildSourcePlatformOptions,
  DEFAULT_INFRASTRUCTURE_SOURCE_OPTIONS,
  orderSourcePlatformKeys,
} from '@/utils/sourcePlatformOptions';

describe('sourcePlatformOptions', () => {
  it('orders canonical source platform keys with preferred source ordering', () => {
    expect(orderSourcePlatformKeys(['truenas', 'pbs', 'agent', 'docker'])).toEqual([
      'agent',
      'docker',
      'proxmox-pbs',
      'truenas',
    ]);
  });

  it('builds canonical source option labels from shared platform labels', () => {
    expect(buildSourcePlatformOptions(['pbs', 'proxmox', 'custom-source'])).toEqual([
      { key: 'proxmox-pve', label: 'PVE' },
      { key: 'proxmox-pbs', label: 'PBS' },
      { key: 'custom-source', label: 'Custom Source' },
    ]);
  });

  it('exports the default infrastructure source options in canonical order', () => {
    expect(DEFAULT_INFRASTRUCTURE_SOURCE_OPTIONS.map((option) => option.key)).toEqual([
      'proxmox-pve',
      'agent',
      'docker',
      'proxmox-pbs',
      'proxmox-pmg',
      'kubernetes',
      'truenas',
    ]);
  });
});

import { describe, expect, it } from 'vitest';
import {
  buildSourcePlatformOptions,
  DEFAULT_INFRASTRUCTURE_SOURCE_OPTIONS,
  orderSourcePlatformKeys,
} from '@/utils/sourcePlatformOptions';

describe('sourcePlatformOptions', () => {
  it('orders canonical source platform keys with preferred source ordering', () => {
    expect(orderSourcePlatformKeys(['truenas', 'pbs', 'availability', 'agent', 'docker'])).toEqual([
      'agent',
      'availability',
      'truenas',
      'proxmox-pbs',
      'docker',
    ]);
  });

  it('builds canonical source option labels from shared platform labels', () => {
    expect(buildSourcePlatformOptions(['pbs', 'proxmox', 'custom-source'])).toEqual([
      { key: 'proxmox-pve', label: 'PVE' },
      { key: 'proxmox-pbs', label: 'PBS' },
      { key: 'custom-source', label: 'Custom Source' },
    ]);
    expect(buildSourcePlatformOptions(['network-endpoint'])).toEqual([
      { key: 'availability', label: 'Availability' },
    ]);
  });

  it('exports the default infrastructure source options in canonical order', () => {
    expect(DEFAULT_INFRASTRUCTURE_SOURCE_OPTIONS.map((option) => option.key)).toEqual([
      'agent',
      'availability',
      'truenas',
      'proxmox-pve',
      'proxmox-pbs',
      'proxmox-pmg',
      'docker',
      'kubernetes',
      'vmware-vsphere',
    ]);
  });

  it('keeps the Docker runtime discoverable in customer-facing source options', () => {
    expect(DEFAULT_INFRASTRUCTURE_SOURCE_OPTIONS.find((option) => option.key === 'docker')).toEqual(
      {
        key: 'docker',
        label: 'Docker / Podman',
      },
    );
  });
});

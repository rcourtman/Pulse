import { describe, expect, it } from 'vitest';
import {
  getPlatformBadge,
  getSourceBadge,
  getTypeBadge,
  getUnifiedSourceBadges,
} from '@/utils/resourceBadgePresentation';

describe('resourceBadgePresentation', () => {
  it('returns canonical platform badges via shared platform presentation', () => {
    expect(getPlatformBadge('proxmox-pve')?.label).toBe('PVE');
    expect(getPlatformBadge('proxmox-pbs')?.label).toBe('PBS');
  });

  it('returns source badges for infrastructure source types', () => {
    expect(getSourceBadge('agent')).toMatchObject({ label: 'Agent', title: 'agent' });
    expect(getSourceBadge('hybrid')).toMatchObject({ label: 'Hybrid', title: 'hybrid' });
  });

  it('returns canonical type badges from shared resource type presentation', () => {
    expect(getTypeBadge('host')?.label).toBe('Agent');
    expect(getTypeBadge('docker_host')?.label).toBe('Container Runtime');
  });

  it('deduplicates and normalizes unified source badges', () => {
    const badges = getUnifiedSourceBadges(['TrueNAS', 'PROXMOX', 'truenas']);
    expect(badges.map((badge) => badge.label)).toEqual(['TrueNAS', 'PVE']);
  });
});

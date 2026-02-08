import { describe, it, expect } from 'vitest';
import { getUnifiedSourceBadges } from '../resourceBadges';

describe('getUnifiedSourceBadges', () => {
  it('returns empty array for null/undefined/empty sources', () => {
    expect(getUnifiedSourceBadges(null)).toEqual([]);
    expect(getUnifiedSourceBadges(undefined)).toEqual([]);
    expect(getUnifiedSourceBadges([])).toEqual([]);
  });

  it('returns badges for known sources', () => {
    const badges = getUnifiedSourceBadges(['proxmox', 'docker']);
    expect(badges).toHaveLength(2);
    expect(badges[0].label).toBe('PVE');
    expect(badges[1].label).toBe('Docker');
  });

  it('includes TrueNAS source badge', () => {
    const badges = getUnifiedSourceBadges(['truenas']);
    expect(badges).toHaveLength(1);
    expect(badges[0].label).toBe('TrueNAS');
    expect(badges[0].title).toBe('truenas');
    expect(badges[0].classes).toContain('bg-blue-100');
  });

  it('normalizes source names to lowercase', () => {
    const badges = getUnifiedSourceBadges(['TrueNAS', 'PROXMOX']);
    expect(badges).toHaveLength(2);
    expect(badges.map((badge) => badge.label)).toContain('TrueNAS');
    expect(badges.map((badge) => badge.label)).toContain('PVE');
  });

  it('deduplicates sources', () => {
    const badges = getUnifiedSourceBadges(['truenas', 'TrueNAS', 'truenas']);
    expect(badges).toHaveLength(1);
  });

  it('filters out unknown sources', () => {
    const badges = getUnifiedSourceBadges(['truenas', 'unknown', 'foobar']);
    expect(badges).toHaveLength(1);
    expect(badges[0].label).toBe('TrueNAS');
  });

  it('returns badges for all supported sources', () => {
    const allSources = ['proxmox', 'agent', 'docker', 'pbs', 'pmg', 'kubernetes', 'truenas'];
    const badges = getUnifiedSourceBadges(allSources);
    expect(badges).toHaveLength(7);
  });
});

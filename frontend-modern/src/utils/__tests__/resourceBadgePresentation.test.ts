import { describe, expect, it } from 'vitest';
import {
  dedupeResourceBadges,
  getInfrastructurePlatformBadges,
  getInfrastructureSystemIdentityBadges,
  getInfrastructureSystemIdentitySortLabel,
  getPlatformBadge,
  getSourceBadge,
  getTypeBadge,
  getUnifiedSourceBadges,
} from '@/utils/resourceBadgePresentation';
import type { Resource } from '@/types/resource';

const makeResource = (overrides: Partial<Resource>): Resource => ({
  id: 'resource-1',
  type: 'agent',
  name: 'host-1',
  displayName: 'host-1',
  platformId: 'host-1',
  platformType: 'agent',
  sourceType: 'agent',
  status: 'online',
  lastSeen: 1,
  ...overrides,
});

describe('resourceBadgePresentation', () => {
  it('returns canonical platform badges via shared platform presentation', () => {
    expect(getPlatformBadge('proxmox-pve')?.label).toBe('PVE');
    expect(getPlatformBadge('proxmox-pbs')?.label).toBe('PBS');
    expect(getPlatformBadge('docker')?.label).toBe('Container runtime');
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

  it('keeps agent as telemetry detail when another infrastructure platform is present', () => {
    expect(getUnifiedSourceBadges(['agent', 'proxmox']).map((badge) => badge.label)).toEqual([
      'Agent',
      'PVE',
    ]);
    expect(
      getInfrastructurePlatformBadges(['agent', 'proxmox']).map((badge) => badge.label),
    ).toEqual(['PVE']);
    expect(getInfrastructurePlatformBadges(['agent']).map((badge) => badge.label)).toEqual([
      'Agent',
    ]);
  });

  it('shows explicit host identity before container runtime capability', () => {
    expect(
      getInfrastructureSystemIdentityBadges(
        makeResource({
          type: 'docker-host',
          platformType: 'docker',
          sourceType: 'hybrid',
          platformData: {
            sources: ['agent', 'docker'],
            agent: {
              platform: 'unraid',
              osName: 'Unraid',
              osVersion: '7.1.0',
            },
          },
        }),
      ).map((badge) => badge.label),
    ).toEqual(['Unraid']);

    expect(
      getInfrastructureSystemIdentityBadges(
        makeResource({
          type: 'docker-host',
          platformType: 'docker',
          sourceType: 'api',
          platformData: { sources: ['docker'] },
        }),
      ).map((badge) => badge.label),
    ).toEqual(['Container runtime']);
  });

  it('keeps API-backed platform identity ahead of reported host OS', () => {
    const resource = makeResource({
      platformType: 'proxmox-pve',
      sourceType: 'hybrid',
      platformData: {
        sources: ['agent', 'proxmox-pve'],
        agent: {
          platform: 'debian',
          osName: 'Debian GNU/Linux',
          osVersion: '12',
        },
      },
    });

    expect(getInfrastructureSystemIdentityBadges(resource).map((badge) => badge.label)).toEqual([
      'PVE',
    ]);
    expect(getInfrastructureSystemIdentitySortLabel(resource)).toBe('PVE');
  });

  it('falls back to reported OS identity for agent-only systems', () => {
    expect(
      getInfrastructureSystemIdentityBadges(
        makeResource({
          platformData: {
            sources: ['agent'],
            agent: {
              platform: 'linux',
              osName: 'Ubuntu 24.04.2 LTS',
            },
          },
        }),
      ).map((badge) => badge.label),
    ).toEqual(['Ubuntu']);

    expect(
      getInfrastructureSystemIdentityBadges(
        makeResource({
          platformData: {
            sources: ['agent'],
            agent: {
              platform: 'qnap',
              osName: 'QNAP QTS',
            },
          },
        }),
      ).map((badge) => badge.label),
    ).toEqual(['QNAP']);
  });

  it('deduplicates repeated header badge labels', () => {
    const badges = dedupeResourceBadges([
      getTypeBadge('agent'),
      getPlatformBadge('proxmox-pve'),
      getSourceBadge('agent'),
    ]);
    expect(badges.map((badge) => badge.label)).toEqual(['Agent', 'PVE']);
  });
});

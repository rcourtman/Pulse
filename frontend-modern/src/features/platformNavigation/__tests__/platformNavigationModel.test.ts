import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  buildPrimaryPlatformNavigationVisibility,
  collectResourcePlatformEvidence,
} from '../platformNavigationModel';

const resource = (overrides: Partial<Resource>): Resource =>
  ({
    id: overrides.id ?? 'resource-1',
    name: overrides.name ?? overrides.id ?? 'resource-1',
    displayName: overrides.displayName ?? overrides.name ?? overrides.id ?? 'resource-1',
    type: overrides.type ?? 'agent',
    platformId: overrides.platformId ?? 'platform-1',
    platformType: overrides.platformType ?? 'agent',
    sourceType: overrides.sourceType ?? 'api',
    status: overrides.status ?? 'online',
    lastSeen: overrides.lastSeen ?? 1_700_000_000_000,
    ...overrides,
  }) as Resource;

describe('platformNavigationModel', () => {
  it('shows primary platform destinations only when supported resource evidence is present', () => {
    expect(buildPrimaryPlatformNavigationVisibility([])).toEqual({
      proxmox: false,
      docker: false,
      kubernetes: false,
      truenas: false,
      vmware: false,
      standalone: false,
    });

    expect(
      buildPrimaryPlatformNavigationVisibility([
        resource({ id: 'pve-1', platformType: 'proxmox-pve', type: 'agent' }),
        resource({
          id: 'pod-1',
          platformType: 'agent',
          type: 'pod',
          platformData: { sources: ['kubernetes'] },
        }),
        resource({ id: 'truenas-1', platformType: 'truenas', type: 'agent' }),
        resource({ id: 'vcenter-1', platformType: 'vmware-vsphere', type: 'vm' }),
      ]),
    ).toEqual({
      proxmox: true,
      docker: false,
      kubernetes: true,
      truenas: true,
      vmware: true,
      standalone: false,
    });
  });

  it('derives Proxmox suite evidence from canonical PBS and PMG resource types', () => {
    expect(collectResourcePlatformEvidence(resource({ id: 'pbs-1', type: 'pbs' }))).toContain(
      'proxmox-pbs',
    );
    expect(collectResourcePlatformEvidence(resource({ id: 'pmg-1', type: 'pmg' }))).toContain(
      'proxmox-pmg',
    );
    expect(
      buildPrimaryPlatformNavigationVisibility([resource({ id: 'pmg-1', type: 'pmg' })]).proxmox,
    ).toBe(true);
  });

  it('keeps Docker visible when an agent-scoped resource has direct Docker runtime evidence', () => {
    const tower = resource({
      id: 'tower',
      platformType: 'agent',
      platformScopes: ['agent'],
      type: 'agent',
      platformData: { docker: { runtime: 'docker' } },
    });
    const visibility = buildPrimaryPlatformNavigationVisibility([tower]);

    expect(collectResourcePlatformEvidence(tower)).toEqual(['agent', 'docker']);
    expect(visibility.docker).toBe(true);
    expect(visibility.standalone).toBe(true);
  });

  it('does not show Docker for a machine agent with an empty Docker facet', () => {
    const tower = resource({
      id: 'tower',
      platformType: 'agent',
      type: 'agent',
      docker: {},
      platformData: { agent: { hostname: 'tower' }, docker: {} },
    });
    const visibility = buildPrimaryPlatformNavigationVisibility([tower]);

    expect(collectResourcePlatformEvidence(tower)).toEqual(['agent']);
    expect(visibility.docker).toBe(false);
    expect(visibility.standalone).toBe(true);
  });

  it('does not infer Docker from TrueNAS-scoped app metadata', () => {
    const truenasApp = resource({
      id: 'truenas-app',
      platformType: 'truenas',
      platformScopes: ['truenas'],
      type: 'app-container',
      platformData: { docker: { containerId: 'abc123' } },
    });
    const visibility = buildPrimaryPlatformNavigationVisibility([truenasApp]);

    expect(collectResourcePlatformEvidence(truenasApp)).toEqual(['truenas']);
    expect(visibility.docker).toBe(false);
    expect(visibility.truenas).toBe(true);
  });
});

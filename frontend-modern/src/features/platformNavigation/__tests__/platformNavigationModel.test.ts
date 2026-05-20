import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  buildPrimaryPlatformNavigationVisibility,
  collectResourcePlatformEvidence,
  selectFirstVisiblePrimaryPlatformNavigationId,
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

  it('uses canonical platform scopes before incidental runtime facets', () => {
    expect(
      collectResourcePlatformEvidence(
        resource({
          id: 'truenas-app-nextcloud',
          type: 'app-container',
          platformType: 'truenas',
          platformScopes: ['truenas'],
          docker: {
            runtime: 'docker',
          },
        }),
      ),
    ).toEqual(['truenas']);

    expect(
      buildPrimaryPlatformNavigationVisibility([
        resource({
          id: 'truenas-app-nextcloud',
          type: 'app-container',
          platformType: 'truenas',
          platformScopes: ['truenas'],
          docker: {
            runtime: 'docker',
          },
        }),
      ]),
    ).toEqual({
      proxmox: false,
      docker: false,
      kubernetes: false,
      truenas: true,
      vmware: false,
    });

    expect(
      buildPrimaryPlatformNavigationVisibility([
        resource({
          id: 'docker-container-frigate-141',
          type: 'app-container',
          platformType: 'docker',
          platformScopes: ['proxmox-pve', 'docker'],
          docker: {
            runtime: 'docker',
            hostSourceId: 'proxmox-lxc-docker:pve-a:node-a:141',
          },
        }),
      ]),
    ).toMatchObject({
      proxmox: true,
      docker: true,
    });
  });

  it('selects the first visible platform using the canonical primary navigation order', () => {
    expect(
      selectFirstVisiblePrimaryPlatformNavigationId({
        proxmox: false,
        docker: false,
        kubernetes: true,
        truenas: true,
        vmware: false,
      }),
    ).toBe('kubernetes');

    expect(
      selectFirstVisiblePrimaryPlatformNavigationId({
        proxmox: false,
        docker: false,
        kubernetes: false,
        truenas: false,
        vmware: false,
      }),
    ).toBeNull();
  });
});

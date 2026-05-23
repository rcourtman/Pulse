import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  buildPrimaryInfrastructureNavigationVisibility,
  collectResourcePlatformEvidence,
  selectFirstVisiblePrimaryInfrastructureNavigationId,
} from '../infrastructureNavigationModel';

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

describe('infrastructureNavigationModel', () => {
  it('shows primary infrastructure destinations only when supported or admitted resource evidence is present', () => {
    expect(buildPrimaryInfrastructureNavigationVisibility([])).toEqual({
      agents: false,
      proxmox: false,
      docker: false,
      kubernetes: false,
      truenas: false,
      vmware: false,
    });

    expect(
      buildPrimaryInfrastructureNavigationVisibility([
        resource({ id: 'agent-1', platformType: 'agent', type: 'agent' }),
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
      agents: true,
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
      buildPrimaryInfrastructureNavigationVisibility([resource({ id: 'pmg-1', type: 'pmg' })])
        .proxmox,
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
      buildPrimaryInfrastructureNavigationVisibility([
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
      agents: false,
      proxmox: false,
      docker: false,
      kubernetes: false,
      truenas: true,
      vmware: false,
    });

    expect(
      buildPrimaryInfrastructureNavigationVisibility([
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

  it('does not show Agents for incidental agent platform hints on non-agent resources', () => {
    expect(
      buildPrimaryInfrastructureNavigationVisibility([
        resource({ id: 'pod-1', type: 'pod', platformType: 'agent' }),
        resource({ id: 'pve-node-1', type: 'agent', platformType: 'proxmox-pve' }),
      ]).agents,
    ).toBe(false);
  });

  it('selects the first visible platform using the canonical primary navigation order', () => {
    expect(
      selectFirstVisiblePrimaryInfrastructureNavigationId({
        agents: true,
        proxmox: true,
        docker: false,
        kubernetes: false,
        truenas: false,
        vmware: false,
      }),
    ).toBe('agents');

    expect(
      selectFirstVisiblePrimaryInfrastructureNavigationId({
        agents: false,
        proxmox: false,
        docker: false,
        kubernetes: true,
        truenas: true,
        vmware: false,
      }),
    ).toBe('kubernetes');

    expect(
      selectFirstVisiblePrimaryInfrastructureNavigationId({
        agents: false,
        proxmox: false,
        docker: false,
        kubernetes: false,
        truenas: false,
        vmware: false,
      }),
    ).toBeNull();
  });
});

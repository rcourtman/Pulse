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
      proxmox: false,
      docker: false,
      kubernetes: false,
      truenas: false,
      vmware: false,
      standalone: false,
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
      proxmox: true,
      docker: false,
      kubernetes: true,
      truenas: true,
      vmware: true,
      standalone: true,
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
      proxmox: false,
      docker: false,
      kubernetes: false,
      truenas: true,
      vmware: false,
      standalone: false,
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

  it('does not show Standalone for incidental agent platform hints on non-agent resources', () => {
    expect(
      buildPrimaryInfrastructureNavigationVisibility([
        resource({ id: 'pod-1', type: 'pod', platformType: 'agent' }),
        resource({ id: 'pve-node-1', type: 'agent', platformType: 'proxmox-pve' }),
      ]).standalone,
    ).toBe(false);
  });

  it('shows Standalone for agent-primary machine source evidence', () => {
    expect(
      buildPrimaryInfrastructureNavigationVisibility([
        resource({
          id: 'mac-mini-1',
          type: 'agent',
          platformType: 'agent',
          sourceType: 'agent',
          sources: ['agent'],
          platformScopes: ['agent'],
        }),
      ]).standalone,
    ).toBe(true);
  });

  it('shows Standalone for agentless availability endpoints', () => {
    expect(
      buildPrimaryInfrastructureNavigationVisibility([
        resource({
          id: 'mqtt-meter',
          type: 'network-endpoint',
          platformType: 'availability',
          sourceType: 'api',
          sources: ['availability'],
        }),
      ]).standalone,
    ).toBe(true);
  });

  it('does not show Standalone for provider-owned host rows even when the agent source is present', () => {
    expect(
      buildPrimaryInfrastructureNavigationVisibility([
        resource({
          id: 'pve-node-1',
          type: 'agent',
          platformType: 'proxmox-pve',
          sourceType: 'hybrid',
          sources: ['proxmox', 'agent'],
          platformScopes: ['agent', 'proxmox-pve'],
        }),
        resource({
          id: 'esxi-host-1',
          type: 'agent',
          platformType: 'vmware-vsphere',
          sourceType: 'api',
          sources: ['vmware'],
          platformScopes: ['agent', 'vmware-vsphere'],
          agent: { agentId: 'vc-host-101', platform: 'vmware-vsphere' },
        }),
      ]).standalone,
    ).toBe(false);
  });

  it('selects provider platforms before Standalone using the canonical primary navigation order', () => {
    expect(
      selectFirstVisiblePrimaryInfrastructureNavigationId({
        proxmox: true,
        docker: false,
        kubernetes: false,
        truenas: false,
        vmware: false,
        standalone: true,
      }),
    ).toBe('proxmox');

    expect(
      selectFirstVisiblePrimaryInfrastructureNavigationId({
        proxmox: false,
        docker: false,
        kubernetes: true,
        truenas: true,
        vmware: false,
        standalone: false,
      }),
    ).toBe('kubernetes');

    expect(
      selectFirstVisiblePrimaryInfrastructureNavigationId({
        proxmox: false,
        docker: false,
        kubernetes: false,
        truenas: false,
        vmware: false,
        standalone: true,
      }),
    ).toBe('standalone');

    expect(
      selectFirstVisiblePrimaryInfrastructureNavigationId({
        proxmox: false,
        docker: false,
        kubernetes: false,
        truenas: false,
        vmware: false,
        standalone: false,
      }),
    ).toBeNull();
  });
});

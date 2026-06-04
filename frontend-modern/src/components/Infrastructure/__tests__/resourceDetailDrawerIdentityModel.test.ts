import { describe, expect, it } from 'vitest';

import {
  buildDiscoveryContextSummary,
  buildIdentityMatchInfo,
  buildResourceDebugBundle,
  buildResourceIdentityView,
  buildSourceSections,
} from '@/components/Infrastructure/resourceDetailDrawerIdentityModel';
import type { Resource } from '@/types/resource';
import { buildResourceAssistantContext } from '@/utils/resourceAssistantContextModel';

const baseResource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'resource-1',
  type: 'vm',
  name: 'vm-101',
  displayName: 'VM 101',
  platformId: 'vm-101',
  platformType: 'proxmox-pve',
  sourceType: 'hybrid',
  status: 'online',
  lastSeen: Date.now(),
  platformData: { sources: ['proxmox'] },
  ...overrides,
});

describe('resourceDetailDrawerIdentityModel', () => {
  it('builds canonical identity card state including alias collapse policy', () => {
    const identityView = buildResourceIdentityView(
      baseResource({
        canonicalIdentity: {
          primaryId: 'vm:101',
          aliases: ['alpha', 'beta', 'gamma', 'delta', 'epsilon'],
        },
        identity: {
          ips: ['192.0.2.10'],
        },
      }),
    );

    expect(identityView.primaryIdentityRows).toContainEqual({
      label: 'Primary ID',
      value: 'vm:101',
    });
    expect(identityView.identityIpValues).toEqual(['192.0.2.10']);
    expect(identityView.aliasPreviewValues).toEqual(identityView.identityAliasValues.slice(0, 4));
    expect(identityView.hasAliasOverflow).toBe(true);
    expect(identityView.identityCardHasRichData).toBe(true);
  });

  it('keeps discovery context wording in the canonical identity model', () => {
    expect(
      buildDiscoveryContextSummary({
        resourceType: 'vm',
        agentId: 'pve-1',
        resourceId: '101',
        hostname: 'pve-1.local',
        metadataKind: 'guest',
        metadataId: 'resource-1',
        targetLabel: 'guest',
      }),
    ).toBe('vm analysis via pve-1.local');

    expect(
      buildDiscoveryContextSummary({
        resourceType: 'agent',
        agentId: 'host-1',
        resourceId: 'host-1',
        hostname: 'host-1.local',
        metadataKind: 'agent',
        metadataId: 'host-1',
        targetLabel: 'agent',
      }),
    ).toBeNull();
  });

  it('builds resource Assistant handoffs from canonical resource identity without prompt text', () => {
    const context = buildResourceAssistantContext(
      baseResource({
        id: 'system-container-6adaf34f529d241a',
        type: 'system-container',
        name: 'homeassistant',
        displayName: 'homeassistant',
        technology: 'lxc',
        parentName: 'delly',
        discoveryTarget: {
          resourceType: 'system-container',
          agentId: 'delly',
          resourceId: '101',
          hostname: 'homeassistant',
        },
      }),
    );

    expect(context.targetType).toBe('resource');
    expect(context.targetId).toBe('system-container-6adaf34f529d241a');
    expect(context.handoffContext).toBeUndefined();
    expect(context.handoffMetadata).toEqual({ kind: 'resource_context' });
    expect(context.handoffResources).toEqual([
      {
        id: 'system-container-6adaf34f529d241a',
        name: 'homeassistant',
        type: 'system-container',
        node: 'delly',
      },
    ]);
    expect(context.briefing?.detailLines).toContain('Discovery: system-container:101');
  });

  it('builds canonical source sections and identity-match precedence', () => {
    const platformData = {
      agent: { hostname: 'host-1' },
      pbs: { instanceId: 'pbs-1' },
      vmware: { connectionName: 'Lab VC' },
      metrics: { cpu: 42 },
      matchResults: { source: 'match-results' },
      matches: { source: 'matches' },
    };

    expect(buildSourceSections(platformData).map((section) => section.id)).toEqual([
      'agent',
      'pbs',
      'vmware',
      'metrics',
    ]);
    expect(buildIdentityMatchInfo(platformData)).toEqual({ source: 'match-results' });
    expect(
      buildIdentityMatchInfo({
        matchCandidates: { source: 'match-candidates' },
        matches: { source: 'matches' },
      }),
    ).toEqual({ source: 'match-candidates' });
  });

  it('assembles the canonical debug bundle from model-owned identity and source state', () => {
    const resource = baseResource({
      identity: { hostname: 'vm-101.local' },
    });

    expect(
      buildResourceDebugBundle({
        resource,
        platformData: {
          proxmox: { nodeName: 'pve-1' },
          docker: { runtime: 'docker' },
          vmware: { connectionName: 'Lab VC' },
        },
        sourceStatus: {
          proxmox: { status: 'online' },
        },
        identityMatchInfo: { matchedBy: 'hostname' },
      }),
    ).toEqual({
      resource,
      identity: {
        resourceIdentity: { hostname: 'vm-101.local' },
        matchInfo: { matchedBy: 'hostname' },
      },
      sources: {
        sourceStatus: {
          proxmox: { status: 'online' },
        },
        proxmox: { nodeName: 'pve-1' },
        agent: undefined,
        docker: { runtime: 'docker' },
        pbs: undefined,
        pmg: undefined,
        kubernetes: undefined,
        vmware: { connectionName: 'Lab VC' },
        metrics: undefined,
      },
    });
  });
});

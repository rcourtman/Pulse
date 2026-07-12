import { describe, expect, it } from 'vitest';

import { buildResourceDebugBundle } from '@/components/Infrastructure/resourceDetailDrawerIdentityModel';
import type { PlatformData } from '@/components/Infrastructure/resourceDetailMappers';
import type { Resource } from '@/types/resource';

// Branch-coverage companion to resourceDetailDrawerIdentityModel.test.ts.
// Drives the still-uncovered arms of buildResourceDebugBundle: the optional-
// chain short-circuit when platformData is omitted (all eight `?.` accesses),
// the "field present" arm for the source fields the sibling test leaves
// undefined (agent/pbs/pmg/kubernetes/metrics), and resource.identity absent.
// Each case pins a concrete shape rather than truthiness.

type SourceStatusMap = NonNullable<PlatformData['sourceStatus']>;
type DebugBundle = ReturnType<typeof buildResourceDebugBundle>;

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

const emptySourceStatus: SourceStatusMap = {};

describe('buildResourceDebugBundle branch coverage', () => {
  it('short-circuits every platformData?. field to undefined when platformData is omitted', () => {
    const resource = baseResource({ identity: { hostname: 'vm-101.local' } });

    const bundle: DebugBundle = buildResourceDebugBundle({
      resource,
      sourceStatus: { proxmox: { status: 'online' } },
      identityMatchInfo: { matchedBy: 'hostname' },
    });

    expect(bundle).toStrictEqual({
      resource,
      identity: {
        resourceIdentity: { hostname: 'vm-101.local' },
        matchInfo: { matchedBy: 'hostname' },
      },
      sources: {
        sourceStatus: { proxmox: { status: 'online' } },
        proxmox: undefined,
        agent: undefined,
        docker: undefined,
        pbs: undefined,
        pmg: undefined,
        kubernetes: undefined,
        vmware: undefined,
        metrics: undefined,
      },
    });
  });

  it('passes every platformData subfield through when all eight sources are populated', () => {
    // Covers the "field present" arm for agent/pbs/pmg/kubernetes/metrics,
    // which the sibling test leaves undefined.
    const resource = baseResource({ identity: { hostname: 'vm-101.local' } });
    const proxmox = { nodeName: 'pve-1' };
    const agent = { hostname: 'host-1' };
    const docker = { runtime: 'docker' };
    const pbs = { instanceId: 'pbs-1' };
    const pmg = { instanceId: 'pmg-1' };
    const kubernetes = { clusterName: 'k8s-1' };
    const vmware = { connectionName: 'Lab VC' };
    const metrics = { cpu: 42 };

    const bundle: DebugBundle = buildResourceDebugBundle({
      resource,
      platformData: { proxmox, agent, docker, pbs, pmg, kubernetes, vmware, metrics },
      sourceStatus: { proxmox: { status: 'online' } },
      identityMatchInfo: { matchedBy: 'hostname' },
    });

    expect(bundle.sources).toStrictEqual({
      sourceStatus: { proxmox: { status: 'online' } },
      proxmox,
      agent,
      docker,
      pbs,
      pmg,
      kubernetes,
      vmware,
      metrics,
    });
  });

  it('reports undefined resourceIdentity when the resource carries no identity', () => {
    const resource = baseResource({});

    const bundle: DebugBundle = buildResourceDebugBundle({
      resource,
      sourceStatus: emptySourceStatus,
      identityMatchInfo: null,
    });

    expect(bundle.identity.resourceIdentity).toBeUndefined();
    expect(bundle.identity).toStrictEqual({
      resourceIdentity: undefined,
      matchInfo: null,
    });
  });

  it('forwards a null identityMatchInfo verbatim and preserves the resource reference', () => {
    const resource = baseResource({ identity: { machineId: 'machine-9' } });

    const bundle: DebugBundle = buildResourceDebugBundle({
      resource,
      sourceStatus: emptySourceStatus,
      identityMatchInfo: null,
    });

    expect(bundle.identity.matchInfo).toBeNull();
    expect(bundle.resource).toBe(resource);
    expect(bundle.identity.resourceIdentity).toStrictEqual({ machineId: 'machine-9' });
  });

  it('forwards an undefined identityMatchInfo verbatim', () => {
    const resource = baseResource({ identity: { ips: ['192.0.2.1'] } });

    const bundle: DebugBundle = buildResourceDebugBundle({
      resource,
      sourceStatus: emptySourceStatus,
      identityMatchInfo: undefined,
    });

    expect(bundle.identity.matchInfo).toBeUndefined();
    expect(bundle.identity.resourceIdentity).toStrictEqual({ ips: ['192.0.2.1'] });
  });

  it('forwards a primitive identityMatchInfo verbatim', () => {
    const resource = baseResource({ identity: { hostname: 'vm-101.local' } });

    const bundle: DebugBundle = buildResourceDebugBundle({
      resource,
      sourceStatus: emptySourceStatus,
      identityMatchInfo: 'hostname',
    });

    expect(bundle.identity.matchInfo).toBe('hostname');
  });

  it('passes an empty sourceStatus map through unchanged', () => {
    const resource = baseResource({});

    const bundle: DebugBundle = buildResourceDebugBundle({
      resource,
      sourceStatus: emptySourceStatus,
      identityMatchInfo: undefined,
    });

    expect(bundle.sources.sourceStatus).toStrictEqual({});
    expect(Object.keys(bundle.sources.sourceStatus)).toEqual([]);
  });

  it('preserves a multi-entry sourceStatus map with lastSeen/error fields', () => {
    const resource = baseResource({});
    const sourceStatus: SourceStatusMap = {
      proxmox: { status: 'online', lastSeen: 1_700_000_000_000 },
      agent: { status: 'error', error: 'agent unreachable' },
    };

    const bundle: DebugBundle = buildResourceDebugBundle({
      resource,
      sourceStatus,
      identityMatchInfo: undefined,
    });

    expect(bundle.sources.sourceStatus).toStrictEqual(sourceStatus);
  });

  it('returns a platformData whose subfields are all undefined when platformData is an empty object', () => {
    // platformData is defined (so the `?.` does not short-circuit) but every
    // subfield is absent: the access arm returns undefined for each source.
    const resource = baseResource({});

    const bundle: DebugBundle = buildResourceDebugBundle({
      resource,
      platformData: { sources: ['proxmox'] },
      sourceStatus: emptySourceStatus,
      identityMatchInfo: undefined,
    });

    expect(bundle.sources).toStrictEqual({
      sourceStatus: {},
      proxmox: undefined,
      agent: undefined,
      docker: undefined,
      pbs: undefined,
      pmg: undefined,
      kubernetes: undefined,
      vmware: undefined,
      metrics: undefined,
    });
  });
});

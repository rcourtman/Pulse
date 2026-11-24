import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { canonicalizeRealtimeResource, mergeCanonicalResource } from '../resourceStateAdapters';

describe('resourceStateAdapters virtualMachine preservation', () => {
  it('preserves provider-neutral virtualMachine metadata across websocket merges', () => {
    const existing: Resource = {
      id: 'vm-kvm-1',
      name: 'kvm-legacy-erp',
      displayName: 'kvm-legacy-erp',
      type: 'vm',
      platformId: 'agent',
      platformType: 'agent',
      sourceType: 'agent',
      sources: ['agent'],
      technology: 'libvirt',
      status: 'degraded',
      lastSeen: 1_700_000_000_000,
      virtualMachine: {
        runtimeState: 'crashed',
        hypervisor: 'libvirt',
        vcpus: 4,
      },
    };

    const incoming: Resource = {
      ...existing,
      lastSeen: 1_700_000_030_000,
      virtualMachine: {
        runtimeState: 'running',
        hypervisor: 'libvirt',
        vcpus: 4,
      },
    };

    const updated = mergeCanonicalResource(incoming, existing);
    expect(updated.virtualMachine).toEqual({
      runtimeState: 'running',
      hypervisor: 'libvirt',
      vcpus: 4,
    });

    const partialUpdate: Resource = {
      ...existing,
      lastSeen: 1_700_000_060_000,
      virtualMachine: undefined,
    };
    const retained = mergeCanonicalResource(partialUpdate, updated);
    expect(retained.virtualMachine).toEqual({
      runtimeState: 'running',
      hypervisor: 'libvirt',
      vcpus: 4,
    });
  });

  it('hydrates virtualMachine from platformData when present on canonical payload', () => {
    const incoming: Resource = {
      id: 'vm-xcp-1',
      name: 'xcp-gateway-01',
      displayName: 'xcp-gateway-01',
      type: 'vm',
      platformId: 'agent',
      platformType: 'agent',
      sourceType: 'agent',
      sources: ['agent'],
      technology: 'xcp-ng',
      status: 'online',
      lastSeen: 1_700_000_000_000,
      platformData: {
        virtualMachine: {
          runtimeState: 'Running',
          hypervisor: 'xcp-ng',
          vcpus: 4,
        },
      },
    };

    const hydrated = canonicalizeRealtimeResource(incoming);
    expect(hydrated.virtualMachine).toEqual({
      runtimeState: 'Running',
      hypervisor: 'xcp-ng',
      vcpus: 4,
    });
  });
});

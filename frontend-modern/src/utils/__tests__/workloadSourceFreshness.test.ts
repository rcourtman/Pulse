import { describe, expect, it } from 'vitest';
import {
  formatWorkloadStaleDetail,
  getWorkloadSourceStaleNotice,
} from '@/utils/workloadSourceFreshness';

describe('workloadSourceFreshness', () => {
  it('returns null when sourceStatus is absent or the primary source is online', () => {
    expect(getWorkloadSourceStaleNotice(null)).toBeNull();
    expect(getWorkloadSourceStaleNotice({ platformType: 'proxmox-pve' })).toBeNull();
    expect(
      getWorkloadSourceStaleNotice({
        platformType: 'proxmox-pve',
        sourceStatus: { proxmox: { status: 'online', lastSeen: '2026-11-27T23:40:00Z' } },
      }),
    ).toBeNull();
  });

  it('reports Proxmox VE and vCenter stale and collection-error notices', () => {
    const pveError = getWorkloadSourceStaleNotice({
      platformType: 'proxmox-pve',
      sourceStatus: {
        proxmox: {
          status: 'error',
          lastSeen: '2026-11-27T23:20:00Z',
          error: 'PVE API timeout (504)',
        },
      },
    });
    expect(pveError).toEqual({
      badge: 'Stale data',
      detail: 'Proxmox VE collection error: PVE API timeout (504)',
      error: 'PVE API timeout (504)',
    });
    expect(formatWorkloadStaleDetail(pveError!)).toBe(
      'Proxmox VE collection error: PVE API timeout (504).',
    );

    const vmwareStale = getWorkloadSourceStaleNotice({
      platformType: 'vmware-vsphere',
      sourceStatus: {
        vmware: {
          status: 'stale',
          lastSeen: '2026-11-27T23:20:00Z',
          error: 'vCenter session expired (401)',
        },
      },
    });
    expect(vmwareStale).toEqual({
      badge: 'Stale data',
      detail: 'vCenter has not updated this recently: vCenter session expired (401)',
      error: 'vCenter session expired (401)',
    });
  });

  it('isolates the primary platform source from a stale linked agent or availability source', () => {
    expect(
      getWorkloadSourceStaleNotice({
        platformType: 'proxmox-pve',
        platformScopes: ['proxmox-pve', 'agent'],
        sourceStatus: {
          proxmox: { status: 'online', lastSeen: '2026-11-27T23:40:00Z' },
          agent: { status: 'stale', lastSeen: '2026-11-27T23:00:00Z' },
          availability: { status: 'error', error: 'probe timeout' },
        },
      }),
    ).toBeNull();

    expect(
      getWorkloadSourceStaleNotice({
        platformType: 'proxmox-pve',
        platformScopes: ['proxmox-pve', 'agent'],
        sourceStatus: {
          proxmox: { status: 'stale', lastSeen: '2026-11-27T23:00:00Z' },
          agent: { status: 'online', lastSeen: '2026-11-27T23:40:00Z' },
        },
      }),
    ).toEqual({
      badge: 'Stale data',
      detail: 'Proxmox VE has not updated this recently',
    });
  });

  it('resolves Docker, Kubernetes, and standalone agent workload sources', () => {
    expect(
      getWorkloadSourceStaleNotice({
        workloadType: 'app-container',
        platformType: 'docker',
        platformScopes: ['docker', 'proxmox-pve'],
        sourceStatus: {
          docker: { status: 'stale' },
          proxmox: { status: 'online' },
        },
      }),
    ).toEqual({
      badge: 'Stale data',
      detail: 'Docker has not updated this recently',
    });

    expect(
      getWorkloadSourceStaleNotice({
        workloadType: 'pod',
        sourceStatus: { kubernetes: { status: 'error', error: 'API server unreachable' } },
      }),
    ).toEqual({
      badge: 'Stale data',
      detail: 'Kubernetes collection error: API server unreachable',
      error: 'API server unreachable',
    });

    expect(
      getWorkloadSourceStaleNotice({
        platformType: 'agent',
        sourceStatus: { agent: { status: 'stale' } },
      }),
    ).toEqual({
      badge: 'Stale data',
      detail: 'Pulse Agent has not updated this recently',
    });
  });

  it('does not double terminal punctuation in formatWorkloadStaleDetail', () => {
    expect(
      formatWorkloadStaleDetail({
        badge: 'Stale data',
        detail: 'Proxmox VE collection error: timeout.',
      }),
    ).toBe('Proxmox VE collection error: timeout.');
  });
});

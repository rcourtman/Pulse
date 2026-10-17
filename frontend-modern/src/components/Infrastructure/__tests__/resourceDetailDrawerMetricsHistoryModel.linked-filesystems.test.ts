import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { guestDiskDeferrals } from '@/components/Workloads/__fixtures__/guestDiskDeferrals';
import {
  getResourceMetricsHistoryCurrentMetrics as current,
  getResourceMetricsHistoryDeferredMetrics as deferred,
} from '../resourceDetailDrawerMetricsHistoryModel';

const agentDisk = { mountpoint: '/', total: 100, used: 62, free: 38 };
const guest = (patch: Partial<Resource> = {}): Resource => ({
  id: 'site-a:pve1:105',
  type: 'vm',
  name: 'Linked guest',
  displayName: 'Linked guest',
  platformId: 'site-a',
  platformType: 'proxmox-pve',
  sourceType: 'hybrid',
  status: 'online',
  lastSeen: Date.parse('2026-10-12T01:00:00Z'),
  cpu: { current: 10 },
  memory: {
    current: 0,
    observation: { state: 'current', source: 'agent', observedAt: '2026-10-12T00:00:00Z' },
  },
  disk: { current: 62, total: 100, used: 62 },
  network: { rxBytes: 0, txBytes: 200 },
  proxmox: { runtimeStatus: 'running', diskStatusReason: 'agent-not-running' },
  agent: { agentId: 'in-guest', stale: false, disks: [agentDisk] },
  ...patch,
});

describe('canonical History filesystem source ownership', () => {
  it.each(['paused', 'suspended'])(
    'qualifies a native %s filesystem even when canonical health and its agent stay online',
    (runtimeStatus) => {
      for (const disks of [undefined, [agentDisk]]) {
        const value = guest({ proxmox: { runtimeStatus, disks } });
        expect(current(value).disk).toBeUndefined();
        expect(current(value).usage).toBeUndefined();
        expect(deferred(value).disk.lastKnownValue).toBe(62);
        expect(deferred(value).disk.message).toContain(`while the VM is ${runtimeStatus}`);
        expect(current(value).cpu).toBe(10);
        value.disk = { current: 0, used: 0, total: 100 };
        expect(deferred(value).disk.lastKnownValue).toBe(0);
        value.disk = undefined;
        expect(deferred(value).disk.lastKnownValue).toBeUndefined();
      }
    },
  );

  it.each(['unknown', 'starting', 'stopping', 'unrecognised-private-state'])(
    'keeps unconfirmed native %s platform values distinct from the live linked agent',
    (runtimeStatus) => {
      const value = guest({ proxmox: { runtimeStatus, disks: [agentDisk] } });
      expect(current(value).disk).toBeUndefined();
      expect(current(value).usage).toBeUndefined();
      expect(deferred(value).disk.lastKnownValue).toBe(62);
      expect(deferred(value).disk.message).toContain('VM running state is unconfirmed');
      expect(deferred(value).disk.message).not.toContain(runtimeStatus);
      expect(current(value).cpu).toBe(10);
      value.proxmox!.disks = undefined;
      expect(current(value).disk).toBe(62);
      value.agent!.stale = true;
      expect(current(value).disk).toBeUndefined();
      expect(deferred(value).disk.message).toContain('stopped reporting');
    },
  );

  it.each([
    'agent-not-running',
    'agent-error',
    'permission-denied',
    ...guestDiskDeferrals.map(([r]) => r),
  ])('uses the live linked agent instead of the unrelated QGA %s read failure', (reason) => {
    const value = guest({ proxmox: { runtimeStatus: 'running', diskStatusReason: reason } });
    expect(current(value)).toMatchObject({ disk: 62, usage: 62, cpu: 10, memory: 0, netin: 0 });
    expect(deferred(value).disk).toBeUndefined();
    value.proxmox!.diskStatusReason = `prev-${reason}`;
    expect(current(value).disk).toBe(62);
    expect(deferred(value).disk).toBeUndefined();
  });

  it.each(['vm', 'system-container'] as const)(
    'qualifies a silent linked agent on an online %s',
    (type) => {
      const value = guest({ type, agent: { stale: true, disks: [agentDisk] } });
      expect(current(value)).toMatchObject({
        disk: undefined,
        usage: undefined,
        cpu: 10,
        memory: undefined,
      });
      expect(deferred(value).memory.lastKnownValue).toBe(0);
      expect(deferred(value).memory.message).toContain('stopped reporting');
      expect(deferred(value).memory.message).toContain('2026-10-12 00:00:00 UTC');
      expect(deferred(value).disk).toEqual({
        lastKnownValue: 62,
        message: 'Using last known disk stats. The Pulse Agent in this guest stopped reporting.',
      });
      value.lastSeen += 3600000;
      value.proxmox!.diskStatusReason = '';
      expect(current(value).disk).toBeUndefined();
      expect(deferred(value).disk.lastKnownValue).toBe(62);
      expect(current(value).memory).toBeUndefined();
      // Independent current QGA/PVE memory does not inherit this Pulse
      // agent's silence or filesystem state, including measured zero.
      value.memory!.observation!.source = type === 'vm' ? 'guest-agent-meminfo' : 'available-field';
      expect(current(value).memory).toBe(0);
      expect(deferred(value).memory).toBeUndefined();
      expect(current(value).disk).toBeUndefined();
    },
  );

  it.each(['stopped', ' STOPPED ', 'offline'])(
    'native %s wins over a live or silent linked agent',
    (runtimeStatus) => {
      for (const stale of [false, true]) {
        const value = guest({ proxmox: { runtimeStatus }, agent: { stale, disks: [agentDisk] } });
        expect(current(value).disk).toBeUndefined();
        expect(deferred(value).disk).toEqual({
          lastKnownValue: undefined,
          message: 'Guest filesystem stats unavailable while the VM is stopped.',
        });
      }
    },
  );

  it.each([{ disks: [] }, { disks: [agentDisk] }])(
    'does not borrow an agent when the platform owns disks (%j)',
    ({ disks }) => {
      const value = guest({
        proxmox: { runtimeStatus: 'running', disks, diskStatusReason: 'prev-vm-locked' },
      });
      expect(current(value).disk).toBeUndefined();
      expect(deferred(value).disk.message).toContain('VM operation lock');
      value.agent!.stale = true;
      value.proxmox!.diskStatusReason = '';
      expect(current(value).disk).toBe(62);
      expect(deferred(value).disk).toBeUndefined();
    },
  );

  it('does not infer filesystem ownership from a mere agent identity', () => {
    const value = guest({ agent: { agentId: 'in-guest', stale: false } });
    expect(current(value).disk).toBeUndefined();
    expect(deferred(value).disk.lastKnownValue).toBeUndefined();
  });

  it('preserves current linked-agent zero and does not fabricate an absent aggregate', () => {
    const value = guest({ disk: { current: 0, used: 0, total: 100 } });
    expect(current(value)).toMatchObject({ disk: 0, usage: 0 });
    expect(deferred(value).disk).toBeUndefined();
    value.disk = undefined;
    expect(current(value).disk).toBeUndefined();
    value.agent!.stale = true;
    expect(deferred(value).disk.lastKnownValue).toBeUndefined();
  });

  it('retains LXC platform storage while stopped, but not its in-guest agent reading', () => {
    const value = guest({
      type: 'system-container',
      proxmox: { runtimeStatus: 'stopped', disks: [agentDisk] },
    });
    expect(current(value)).toMatchObject({ disk: 62, usage: 62 });
    expect(deferred(value).disk).toBeUndefined();
    value.proxmox!.disks = undefined;
    expect(current(value).disk).toBeUndefined();
    expect(deferred(value).disk).toEqual({
      lastKnownValue: undefined,
      message: 'Filesystem usage is unavailable.',
    });
  });

  it.each(['vm', 'system-container'] as const)(
    'keeps a linked-only %s in Proxmox scopes qualified',
    (type) => {
      const value = guest({
        type,
        platformType: 'agent',
        platformScopes: ['agent', 'proxmox-pve'],
        agent: { stale: true, disks: [agentDisk] },
      });
      expect(current(value).disk).toBeUndefined();
      expect(deferred(value).disk.lastKnownValue).toBe(62);
    },
  );

  it('does not impose QGA read metadata on a vSphere-only VM', () => {
    const value = guest({ platformType: 'vmware-vsphere', platformScopes: ['vmware-vsphere'] });
    expect(current(value)).toMatchObject({ disk: 62, usage: 62 });
    expect(deferred(value).disk).toBeUndefined();
  });
});

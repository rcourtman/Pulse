import { describe, expect, it } from 'vitest';
import type { ResourceDiscoveryTarget } from '@/types/resource';
import type { WorkloadGuest } from '@/types/workloads';
import {
  getWorkloadActionAgentTitle,
  hasExplicitWorkloadActionAgent,
  isInGuestPulseAgentEligibleWorkload,
} from '../workloadAgentReadiness';

const makeGuest = (overrides?: Partial<WorkloadGuest>): WorkloadGuest =>
  ({
    id: 'guest-0',
    vmid: 100,
    name: 'workload-0',
    node: 'pve',
    instance: 'cluster-a',
    status: 'running',
    type: 'qemu',
    cpu: 0.5,
    cpus: 2,
    memory: { total: 4096, used: 1024, free: 3072, usage: 0.25 },
    disk: { total: 102400, used: 10240, free: 92160, usage: 0.1 },
    networkIn: 100,
    networkOut: 200,
    diskRead: 10,
    diskWrite: 5,
    uptime: 3600,
    template: false,
    lastBackup: 0,
    tags: [],
    lock: '',
    lastSeen: new Date().toISOString(),
    workloadType: 'vm',
    ...overrides,
  }) as WorkloadGuest;

const target = (overrides: Partial<ResourceDiscoveryTarget> = {}): ResourceDiscoveryTarget => ({
  resourceType: 'vm',
  agentId: 'agent-1',
  resourceId: 'resource-1',
  ...overrides,
});

describe('workloadAgentReadiness (branch coverage)', () => {
  describe('hasExplicitWorkloadActionAgent', () => {
    it('returns false when discoveryTarget is absent (!target early-return arm)', () => {
      const guest = makeGuest({ type: 'qemu', workloadType: 'vm' });
      expect(hasExplicitWorkloadActionAgent(guest)).toBe(false);
    });

    it('returns false for an app-container even with a complete target (workloadType !== vm && !== system-container; first && operand true)', () => {
      const guest = makeGuest({
        type: 'docker',
        workloadType: 'app-container',
        discoveryTarget: target({ resourceType: 'app-container' }),
      });
      expect(hasExplicitWorkloadActionAgent(guest)).toBe(false);
    });

    it('returns false for a pod (workloadType !== vm && !== system-container; second && operand true)', () => {
      const guest = makeGuest({
        type: 'pod',
        workloadType: 'pod',
        discoveryTarget: target({ resourceType: 'pod' }),
      });
      expect(hasExplicitWorkloadActionAgent(guest)).toBe(false);
    });

    it('returns false for a vm when agentId is blank (agentId length===0 arm)', () => {
      const guest = makeGuest({
        type: 'qemu',
        workloadType: 'vm',
        discoveryTarget: target({ resourceType: 'vm', agentId: '   ' }),
      });
      expect(hasExplicitWorkloadActionAgent(guest)).toBe(false);
    });

    it('returns false for a vm when resourceId is blank (resourceId length===0 arm)', () => {
      const guest = makeGuest({
        type: 'qemu',
        workloadType: 'vm',
        discoveryTarget: target({ resourceType: 'vm', resourceId: '' }),
      });
      expect(hasExplicitWorkloadActionAgent(guest)).toBe(false);
    });

    it('returns false when target.resourceType is "agent" (neither vm nor system-container)', () => {
      const guest = makeGuest({
        type: 'qemu',
        workloadType: 'vm',
        discoveryTarget: target({ resourceType: 'agent' }),
      });
      expect(hasExplicitWorkloadActionAgent(guest)).toBe(false);
    });

    it('returns true for a vm with a complete vm resourceType target (vm continue + vm resourceType arm)', () => {
      const guest = makeGuest({
        type: 'qemu',
        workloadType: 'vm',
        discoveryTarget: target({ resourceType: 'vm' }),
      });
      expect(hasExplicitWorkloadActionAgent(guest)).toBe(true);
    });

    it('returns true for a system-container with a system-container resourceType target (system-container continue + system-container resourceType arm)', () => {
      const guest = makeGuest({
        type: 'lxc',
        workloadType: 'system-container',
        discoveryTarget: target({ resourceType: 'system-container' }),
      });
      expect(hasExplicitWorkloadActionAgent(guest)).toBe(true);
    });

    it('returns true for a vm whose target resourceType is system-container (resourceType OR second arm)', () => {
      const guest = makeGuest({
        type: 'qemu',
        workloadType: 'vm',
        discoveryTarget: target({ resourceType: 'system-container' }),
      });
      expect(hasExplicitWorkloadActionAgent(guest)).toBe(true);
    });

    it('returns true for a system-container whose target resourceType is vm (resourceType OR first arm)', () => {
      const guest = makeGuest({
        type: 'lxc',
        workloadType: 'system-container',
        discoveryTarget: target({ resourceType: 'vm' }),
      });
      expect(hasExplicitWorkloadActionAgent(guest)).toBe(true);
    });
  });

  describe('getWorkloadActionAgentTitle', () => {
    it('interpolates the node when node is a non-empty string (truthy ternary arm)', () => {
      const guest = makeGuest({ node: 'pve' });
      expect(getWorkloadActionAgentTitle(guest)).toBe(
        'Discovery and governed actions use the Pulse Agent connected to pve.',
      );
    });

    it('trims surrounding whitespace from node before interpolating', () => {
      const guest = makeGuest({ node: '  pve-node  ' });
      expect(getWorkloadActionAgentTitle(guest)).toBe(
        'Discovery and governed actions use the Pulse Agent connected to pve-node.',
      );
    });

    it('falls back to the parent-node title when node is an empty string (falsy ternary arm)', () => {
      const guest = makeGuest({ node: '' });
      expect(getWorkloadActionAgentTitle(guest)).toBe(
        'Discovery and governed actions use the connected parent node Pulse Agent.',
      );
    });

    it('falls back to the parent-node title when node is whitespace-only (trim -> empty)', () => {
      const guest = makeGuest({ node: '   ' });
      expect(getWorkloadActionAgentTitle(guest)).toBe(
        'Discovery and governed actions use the connected parent node Pulse Agent.',
      );
    });

    it('falls back to the parent-node title when node is undefined (|| "" coalesce arm)', () => {
      const guest = makeGuest({ node: undefined });
      expect(getWorkloadActionAgentTitle(guest)).toBe(
        'Discovery and governed actions use the connected parent node Pulse Agent.',
      );
    });
  });

  describe('isInGuestPulseAgentEligibleWorkload', () => {
    it('returns false for a vm template (template === true short-circuits before the vm arm)', () => {
      const guest = makeGuest({ type: 'qemu', workloadType: 'vm', template: true });
      expect(isInGuestPulseAgentEligibleWorkload(guest)).toBe(false);
    });

    it('returns false for a system-container template (template guard precedes the system-container branch)', () => {
      const guest = makeGuest({
        type: 'lxc',
        workloadType: 'system-container',
        template: true,
      });
      expect(isInGuestPulseAgentEligibleWorkload(guest)).toBe(false);
    });

    it('returns true for a non-template vm (workloadType === "vm" arm)', () => {
      const guest = makeGuest({ type: 'qemu', workloadType: 'vm', template: false });
      expect(isInGuestPulseAgentEligibleWorkload(guest)).toBe(true);
    });

    it('returns true for a regular lxc system-container (isOciSystemContainer false -> !false === true)', () => {
      const guest = makeGuest({
        type: 'lxc',
        workloadType: 'system-container',
        template: false,
      });
      expect(isInGuestPulseAgentEligibleWorkload(guest)).toBe(true);
    });

    it('returns false for an oci-container (type === "oci-container" -> isOciSystemContainer true -> !true === false)', () => {
      const guest = makeGuest({
        type: 'oci-container',
        workloadType: 'system-container',
        template: false,
      });
      expect(isInGuestPulseAgentEligibleWorkload(guest)).toBe(false);
    });

    it('returns false for a system-container flagged isOci via the isOci flag (second isOciSystemContainer OR arm)', () => {
      const guest = {
        ...makeGuest({ type: 'lxc', workloadType: 'system-container', template: false }),
        isOci: true,
      } as WorkloadGuest;
      expect(isInGuestPulseAgentEligibleWorkload(guest)).toBe(false);
    });

    it('returns false for an app-container (default return-false arm)', () => {
      const guest = makeGuest({
        type: 'docker',
        workloadType: 'app-container',
        template: false,
      });
      expect(isInGuestPulseAgentEligibleWorkload(guest)).toBe(false);
    });

    it('returns false for a pod (default return-false arm)', () => {
      const guest = makeGuest({ type: 'pod', workloadType: 'pod', template: false });
      expect(isInGuestPulseAgentEligibleWorkload(guest)).toBe(false);
    });
  });
});

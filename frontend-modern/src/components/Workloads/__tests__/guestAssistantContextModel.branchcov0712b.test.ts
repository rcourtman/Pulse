import { describe, expect, it } from 'vitest';
import type { WorkloadGuest } from '@/types/workloads';
import {
  buildGuestAssistantContext,
  getGuestAssistantTechnology,
} from '../guestAssistantContextModel';

// `firstTrimmed` is module-private (not exported), so it is exercised
// indirectly through `getGuestAssistantTechnology`, which is its only caller.
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

describe('guestAssistantContextModel (branch coverage)', () => {
  describe('getGuestAssistantTechnology (drives firstTrimmed branches)', () => {
    describe('app-container arm — firstTrimmed([containerRuntime, platformType, type])', () => {
      it('returns the containerRuntime when present (first value wins, trimmed)', () => {
        expect(
          getGuestAssistantTechnology(
            makeGuest({
              workloadType: 'app-container',
              type: 'app-container',
              containerRuntime: '  podman  ',
              platformType: 'docker',
            }),
          ),
        ).toBe('podman');
      });

      it('skips a whitespace-only containerRuntime and falls back to platformType', () => {
        // Exercises firstTrimmed's `(value || '').trim()` -> '' (falsy) -> continue.
        expect(
          getGuestAssistantTechnology(
            makeGuest({
              workloadType: 'app-container',
              type: 'app-container',
              containerRuntime: '   ',
              platformType: 'docker',
            }),
          ),
        ).toBe('docker');
      });

      it('falls back to platformType when containerRuntime is undefined', () => {
        expect(
          getGuestAssistantTechnology(
            makeGuest({
              workloadType: 'app-container',
              type: 'app-container',
              containerRuntime: undefined,
              platformType: 'docker',
            }),
          ),
        ).toBe('docker');
      });

      it('falls back to type when containerRuntime and platformType are absent', () => {
        expect(
          getGuestAssistantTechnology(
            makeGuest({
              workloadType: 'app-container',
              type: 'app-container',
              containerRuntime: undefined,
              platformType: undefined,
            }),
          ),
        ).toBe('app-container');
      });

      it('returns undefined when all three candidates are blank (loop completion)', () => {
        expect(
          getGuestAssistantTechnology(
            makeGuest({
              workloadType: 'app-container',
              type: '',
              containerRuntime: '',
              platformType: '',
            }),
          ),
        ).toBeUndefined();
      });
    });

    describe("pod arm — firstTrimmed([platformType, 'kubernetes'])", () => {
      it('returns platformType when present', () => {
        expect(
          getGuestAssistantTechnology(
            makeGuest({
              workloadType: 'pod',
              type: 'pod',
              platformType: 'managed-aks',
            }),
          ),
        ).toBe('managed-aks');
      });

      it("falls back to the 'kubernetes' constant when platformType is absent", () => {
        expect(
          getGuestAssistantTechnology(
            makeGuest({
              workloadType: 'pod',
              type: 'pod',
              platformType: undefined,
            }),
          ),
        ).toBe('kubernetes');
      });
    });

    describe('default arm (vm / system-container) — firstTrimmed([platformType, type])', () => {
      it('returns platformType for a vm when present', () => {
        expect(
          getGuestAssistantTechnology(
            makeGuest({ workloadType: 'vm', type: 'qemu', platformType: 'proxmox-ve' }),
          ),
        ).toBe('proxmox-ve');
      });

      it('falls back to type for a vm when platformType is absent', () => {
        expect(
          getGuestAssistantTechnology(
            makeGuest({ workloadType: 'vm', type: 'qemu', platformType: undefined }),
          ),
        ).toBe('qemu');
      });

      it('returns undefined for a vm when both platformType and type are blank', () => {
        expect(
          getGuestAssistantTechnology(
            makeGuest({ workloadType: 'vm', type: '', platformType: '' }),
          ),
        ).toBeUndefined();
      });

      it('returns platformType for a system-container when present', () => {
        expect(
          getGuestAssistantTechnology(
            makeGuest({
              workloadType: 'system-container',
              type: 'lxc',
              platformType: 'incus',
            }),
          ),
        ).toBe('incus');
      });
    });
  });

  describe('buildGuestAssistantContext', () => {
    describe('display-name resolution (guest.name || canonicalId)', () => {
      it('uses guest.name as the title and handoff name when present', () => {
        const result = buildGuestAssistantContext(
          makeGuest({
            id: 'vm-101',
            vmid: 101,
            name: 'web-vm',
            node: 'node-a',
            instance: 'pve-a',
            workloadType: 'vm',
            type: 'qemu',
          }),
        );

        expect(result.briefing?.title).toBe('web-vm');
        expect(result.handoffResources?.[0]?.name).toBe('web-vm');
      });

      it('falls back to the canonical node-scoped id when name is empty', () => {
        const result = buildGuestAssistantContext(
          makeGuest({
            id: 'vm-101',
            vmid: 101,
            name: '',
            node: 'node-a',
            instance: 'pve-a',
            workloadType: 'vm',
            type: 'qemu',
          }),
        );

        // canonicalId = buildCanonicalNodeScopedWorkloadId -> "pve-a:node-a:101"
        expect(result.targetId).toBe('pve-a:node-a:101');
        expect(result.briefing?.title).toBe('pve-a:node-a:101');
        expect(result.handoffResources?.[0]?.name).toBe('pve-a:node-a:101');
      });
    });

    it('emits Primary/Parent/Discovery/readiness lines for a discovered VM', () => {
      const result = buildGuestAssistantContext(
        makeGuest({
          id: 'vm-101',
          vmid: 101,
          name: 'web-vm',
          node: 'node-a',
          instance: 'pve-a',
          status: 'running',
          workloadType: 'vm',
          type: 'qemu',
          platformType: 'proxmox-ve',
          discoveryTarget: {
            resourceType: 'vm',
            agentId: 'agent-1',
            resourceId: 'pve-a:node-a:101',
          },
          discoveryReadiness: { state: 'fresh', factCount: 5 },
        }),
      );

      expect(result).toStrictEqual({
        targetType: 'resource',
        targetId: 'pve-a:node-a:101',
        context: {
          source: 'guest-drawer',
          resourceId: 'pve-a:node-a:101',
          resourceType: 'vm',
          resourceStatus: 'running',
        },
        briefing: {
          sourceLabel: 'Pulse resource context',
          title: 'web-vm',
          subject: 'vm / running / proxmox-ve',
          statusLabel: 'Read-only context attached · Discovery fresh',
          detailLines: [
            'Resource ID: pve-a:node-a:101',
            'Primary identity: vm-101',
            'Parent: node-a',
            'Discovery: vm:pve-a:node-a:101',
            'Discovery data: Discovery fresh, 5 facts',
          ],
          safetyNote: 'Approval required before any action.',
        },
        handoffResources: [
          { id: 'pve-a:node-a:101', name: 'web-vm', type: 'vm', node: 'node-a' },
        ],
        handoffMetadata: { kind: 'resource_context' },
        autonomousMode: false,
      });
    });

    it('omits Primary/Parent/Discovery/readiness lines for a non-discovered app-container', () => {
      // canonicalId === guest.id for app-container, so the Primary identity line
      // is suppressed; platformType 'kubernetes' is not docker-managed so
      // resolveDiscoveryTargetForWorkload returns null.
      const result = buildGuestAssistantContext(
        makeGuest({
          id: 'app-container:sidecar',
          vmid: 0,
          name: 'sidecar',
          node: '',
          instance: '',
          status: 'exited',
          workloadType: 'app-container',
          type: 'app-container',
          platformType: 'kubernetes',
        }),
      );

      expect(result).toStrictEqual({
        targetType: 'resource',
        targetId: 'app-container:sidecar',
        context: {
          source: 'guest-drawer',
          resourceId: 'app-container:sidecar',
          resourceType: 'app-container',
          resourceStatus: 'exited',
        },
        briefing: {
          sourceLabel: 'Pulse resource context',
          title: 'sidecar',
          subject: 'app-container / exited / kubernetes',
          statusLabel: 'Read-only context attached',
          detailLines: ['Resource ID: app-container:sidecar'],
          safetyNote: 'Approval required before any action.',
        },
        handoffResources: [
          { id: 'app-container:sidecar', name: 'sidecar', type: 'app-container', node: '' },
        ],
        handoffMetadata: { kind: 'resource_context' },
        autonomousMode: false,
      });
    });
  });
});

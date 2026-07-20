import { describe, expect, it } from 'vitest';
import type { WorkloadGuest } from '@/types/workloads';
import {
  getKubernetesContextKey,
  getWorkloadAlertResourceIdCandidates,
  getWorkloadContainerHostId,
  getWorkloadDockerHostId,
  getWorkloadHostHintCandidates,
  getWorkloadHostLabel,
  workloadNodeScopeId,
} from '@/components/Workloads/workloadTopology';

const makeGuest = (i: number, overrides?: Partial<WorkloadGuest>): WorkloadGuest => ({
  id: `guest-${i}`,
  vmid: 100 + i,
  name: `workload-${i}`,
  node: `node-${i % 5}`,
  instance: `cluster-${i % 3}`,
  status: 'running',
  type: 'vm',
  cpu: 0,
  cpus: 2,
  memory: { total: 4096, used: 0, free: 4096, usage: 0 },
  disk: { total: 102400, used: 0, free: 102400, usage: 0 },
  networkIn: 0,
  networkOut: 0,
  diskRead: 0,
  diskWrite: 0,
  uptime: 0,
  template: false,
  lastBackup: 0,
  tags: [],
  lock: '',
  lastSeen: new Date().toISOString(),
  workloadType: 'vm',
  ...overrides,
});

describe('workloadTopology (branch coverage 2)', () => {
  describe('workloadNodeScopeId', () => {
    it('exercises both `|| ""` fallback arms when instance and node are empty strings', () => {
      const guest = makeGuest(1, { instance: '', node: '' });
      // (guest.instance || '') and (guest.node || '') both take the falsy arm.
      expect(workloadNodeScopeId(guest)).toBe('-');
    });

    it('keeps a present instance and falls back to empty for a missing node', () => {
      const guest = makeGuest(1, { instance: 'only-inst', node: '' });
      // instance truthy arm + node `|| ""` fallback arm.
      expect(workloadNodeScopeId(guest)).toBe('only-inst-');
    });

    it('falls back to empty for a missing instance and keeps a present node', () => {
      const guest = makeGuest(1, { instance: '', node: 'only-node' });
      // instance `|| ""` fallback arm + node truthy arm.
      expect(workloadNodeScopeId(guest)).toBe('-only-node');
    });

    it('trims whitespace on both segments when both are present', () => {
      const guest = makeGuest(1, { instance: '  inst  ', node: ' \tnode\t' });
      expect(workloadNodeScopeId(guest)).toBe('inst-node');
    });
  });

  describe('getKubernetesContextKey', () => {
    it('returns the trimmed contextLabel when it is the first non-empty candidate', () => {
      const guest = makeGuest(1, {
        contextLabel: ' tower.local ',
        instance: 'cluster-a',
        node: 'worker-a',
      });
      // First loop iteration: contextLabel wins immediately.
      expect(getKubernetesContextKey(guest)).toBe('tower.local');
    });

    it('falls through empty contextLabel and instance to return the node candidate', () => {
      const guest = makeGuest(1, {
        contextLabel: '   ',
        instance: '',
        node: 'worker-b',
      });
      // contextLabel and instance trim to '' -> loop continues -> node returned.
      expect(getKubernetesContextKey(guest)).toBe('worker-b');
    });

    it('returns empty string when every candidate is empty or whitespace', () => {
      const guest = makeGuest(1, {
        contextLabel: ' ',
        instance: '',
        node: '   ',
      });
      // Loop exhausts all candidates -> final `return ''`.
      expect(getKubernetesContextKey(guest)).toBe('');
    });
  });

  describe('getWorkloadDockerHostId', () => {
    it('short-circuits to empty string for a non-app-container type', () => {
      const vm = makeGuest(1, {
        type: 'vm',
        workloadType: 'vm',
        dockerHostId: 'should-be-ignored',
      });
      // type !== 'app-container' -> guard returns '' before touching dockerHostId.
      expect(getWorkloadDockerHostId(vm)).toBe('');
    });

    it('short-circuits to empty string for a pod type', () => {
      const pod = makeGuest(2, { type: 'pod', workloadType: 'pod', dockerHostId: 'ignored' });
      expect(getWorkloadDockerHostId(pod)).toBe('');
    });

    it('returns the trimmed dockerHostId for an app-container', () => {
      const app = makeGuest(3, {
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'docker',
        dockerHostId: '  docker-host-9  ',
      });
      // type === 'app-container' -> (guest.dockerHostId || '').trim().
      expect(getWorkloadDockerHostId(app)).toBe('docker-host-9');
    });

    it('returns empty string for an app-container with an empty dockerHostId via the `|| ""` arm', () => {
      const app = makeGuest(4, {
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'docker',
        dockerHostId: '',
      });
      expect(getWorkloadDockerHostId(app)).toBe('');
    });
  });

  describe('firstTrimmed (exercised via getWorkloadContainerHostId / getWorkloadHostLabel)', () => {
    it('returns the first non-empty candidate, trimming it', () => {
      const app = makeGuest(1, {
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'docker',
        dockerHostId: '  host-zero  ',
        contextLabel: 'tower',
        node: 'node-a',
        instance: 'inst-a',
      });
      // getWorkloadContainerHostId orders dockerHostId first -> firstTrimmed first-wins arm.
      expect(getWorkloadContainerHostId(app)).toBe('host-zero');
    });

    it('skips empty/whitespace candidates and returns the first populated later candidate', () => {
      const app = makeGuest(2, {
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'docker',
        dockerHostId: '   ',
        contextLabel: '',
        node: '  node-b  ',
        instance: 'inst-b',
      });
      // dockerHostId and contextLabel trim to '' -> loop continues -> node returned.
      expect(getWorkloadContainerHostId(app)).toBe('node-b');
    });

    it('returns empty string when every candidate is empty (final `return ""` arm)', () => {
      const app = makeGuest(3, {
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'docker',
        dockerHostId: '',
        contextLabel: '   ',
        node: '',
        instance: '',
      });
      // All four candidates trim to '' -> firstTrimmed loop exhausts -> ''.
      expect(getWorkloadContainerHostId(app)).toBe('');
      // Same arm reached through getWorkloadHostLabel's app-container candidate list.
      expect(getWorkloadHostLabel(app)).toBe('');
    });
  });

  describe('getWorkloadContainerHostId (guard arm)', () => {
    it('short-circuits to empty string for a non-app-container type', () => {
      const vm = makeGuest(1, {
        type: 'vm',
        workloadType: 'vm',
        dockerHostId: 'ignored',
        contextLabel: 'ignored',
        node: 'node-a',
        instance: 'inst-a',
      });
      // type !== 'app-container' -> guard returns '' without calling firstTrimmed.
      expect(getWorkloadContainerHostId(vm)).toBe('');
    });
  });

  describe('getWorkloadHostLabel', () => {
    it('returns the trimmed contextLabel first for an app-container', () => {
      const app = makeGuest(1, {
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'docker',
        contextLabel: '  ctx-host  ',
        node: 'node-a',
        instance: 'inst-a',
        dockerHostId: 'dhid',
      });
      // app-container branch -> firstTrimmed([contextLabel, ...]) -> contextLabel wins.
      expect(getWorkloadHostLabel(app)).toBe('ctx-host');
    });

    it('returns the trimmed node for a vm via the else branch', () => {
      const vm = makeGuest(2, { type: 'vm', workloadType: 'vm', node: '  pve1  ' });
      // else arm: (guest.node || '').trim().
      expect(getWorkloadHostLabel(vm)).toBe('pve1');
    });

    it('returns empty string for a vm with an empty node via the `|| ""` arm', () => {
      const vm = makeGuest(3, { type: 'vm', workloadType: 'vm', node: '' });
      expect(getWorkloadHostLabel(vm)).toBe('');
    });

    it('returns empty string for a vm whose node is null via the `|| ""` arm', () => {
      const vm = makeGuest(4, {
        type: 'vm',
        workloadType: 'vm',
        node: null as unknown as string,
      });
      // Deliberately-malformed input: null node -> (null || '').trim() -> ''.
      expect(getWorkloadHostLabel(vm)).toBe('');
    });
  });

  describe('getWorkloadHostHintCandidates', () => {
    it('returns an empty array for a pod', () => {
      const pod = makeGuest(1, {
        type: 'pod',
        workloadType: 'pod',
        node: 'worker-a',
        instance: 'cluster-a',
        contextLabel: 'ctx',
      });
      // type === 'pod' -> early return [].
      expect(getWorkloadHostHintCandidates(pod)).toEqual([]);
    });

    it('dedupes case-insensitively and preserves first-seen order for an app-container', () => {
      const app = makeGuest(2, {
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'docker',
        dockerHostId: 'Host1',
        contextLabel: 'host1',
        node: 'node-a',
        instance: 'inst-a',
      });
      // dedupeTrimmed lowercases keys for dedup but keeps original casing of first seen.
      expect(getWorkloadHostHintCandidates(app)).toEqual(['Host1', 'node-a', 'inst-a']);
    });

    it('returns an empty array for an app-container whose candidates are all empty', () => {
      const app = makeGuest(3, {
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'docker',
        dockerHostId: '',
        contextLabel: '   ',
        node: '',
        instance: '',
      });
      expect(getWorkloadHostHintCandidates(app)).toEqual([]);
    });

    it('dedupes node/instance for a vm through the else branch', () => {
      const vm = makeGuest(4, {
        type: 'vm',
        workloadType: 'vm',
        node: 'dup',
        instance: 'dup',
        contextLabel: 'ctx-a',
      });
      // else arm: dedupeTrimmed([node, instance, contextLabel]) -> node and instance collapse.
      expect(getWorkloadHostHintCandidates(vm)).toEqual(['dup', 'ctx-a']);
    });

    it('routes a system-container through the else branch', () => {
      const lxc = makeGuest(5, {
        type: 'lxc',
        workloadType: 'system-container',
        node: 'node-lxc',
        instance: 'cluster-lxc',
        contextLabel: 'ctx-lxc',
      });
      expect(getWorkloadHostHintCandidates(lxc)).toEqual(['node-lxc', 'cluster-lxc', 'ctx-lxc']);
    });
  });

  describe('getWorkloadAlertResourceIdCandidates', () => {
    it('routes a vm through guestOverrideIdCandidates and emits stable + canonical + legacy ids', () => {
      const vm = makeGuest(1, {
        id: 'vm-resource-hash',
        vmid: 103,
        type: 'vm',
        workloadType: 'vm',
        node: 'pve1',
        instance: 'cluster-a',
      });
      // type === 'vm' -> guestOverrideIdCandidates(guest); instance !== node so the
      // stable guest: key and the legacy cluster key are both produced.
      expect(getWorkloadAlertResourceIdCandidates(vm)).toEqual([
        'guest:cluster-a:103',
        'cluster-a:pve1:103',
        'vm-resource-hash',
        'cluster-a-103',
        'cluster-a-pve1-103',
      ]);
    });

    it('routes a system-container through guestOverrideIdCandidates', () => {
      const lxc = makeGuest(2, {
        id: 'lxc-100',
        vmid: 100,
        type: 'lxc',
        workloadType: 'system-container',
        node: 'node-a',
        instance: 'cluster-a',
      });
      expect(getWorkloadAlertResourceIdCandidates(lxc)).toEqual([
        'guest:cluster-a:100',
        'cluster-a:node-a:100',
        'lxc-100',
        'cluster-a-100',
        'cluster-a-node-a-100',
      ]);
    });

    it('builds docker:host/container override ids for a docker-managed app-container with discovery', () => {
      const app = makeGuest(3, {
        id: 'app-container:docker-host-1:container-abc123',
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'docker',
        dockerHostId: 'docker-host-1',
        containerId: 'container-abc123',
        node: '',
        instance: '',
      });
      // discoveryTarget resolved (docker-managed) -> agentId/resourceId feed host and
      // container candidate lists; shortId = last id segment = container-abc123, and the
      // canonical id is appended. node/instance blanked to isolate the discovery-driven host.
      expect(getWorkloadAlertResourceIdCandidates(app)).toEqual([
        'docker:docker-host-1/container-abc123',
        'docker:docker-host-1/app-container:docker-host-1:container-abc123',
        'app-container:docker-host-1:container-abc123',
      ]);
    });

    it('falls back to guest.id for shortId when the id has no usable segments', () => {
      const app = makeGuest(4, {
        id: '',
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'docker',
        dockerHostId: 'h1',
        containerId: 'c1',
        node: '',
        instance: '',
      });
      // idSegments (after filter(Boolean)) is empty -> `idSegments[last] || guest.id`
      // right operand fires; empty guest.id then drops out of dedupeTrimmed.
      expect(getWorkloadAlertResourceIdCandidates(app)).toEqual(['docker:h1/c1']);
    });

    it('still emits docker:-prefixed ids for a non-docker (truenas) app-container with no discovery', () => {
      const truenas = makeGuest(5, {
        id: 'app-container:truenas-main:nextcloud',
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'truenas',
        dockerHostId: '',
        node: 'truenas-main',
        instance: 'truenas-main',
      });
      // resolveDiscoveryTargetForWorkload -> null (not docker-managed), so
      // discoveryTarget?.agentId / resourceId are undefined; hostCandidates=[truenas-main]
      // (node + instance collapse), shortId = nextcloud.
      expect(getWorkloadAlertResourceIdCandidates(truenas)).toEqual([
        'docker:truenas-main/nextcloud',
        'docker:truenas-main/app-container:truenas-main:nextcloud',
        'app-container:truenas-main:nextcloud',
      ]);
    });

    it('routes a pod through the fallback else branch returning the canonical id only', () => {
      const pod = makeGuest(6, {
        id: 'k8s:cluster-a:pod:pod-uid-1',
        type: 'pod',
        workloadType: 'pod',
        node: 'worker-a',
        instance: 'cluster-a',
      });
      // type === 'pod' -> else: dedupeTrimmed([getCanonicalWorkloadId(guest), guest.id]).
      // getCanonicalWorkloadId returns guest.id for pods -> collapses to a single entry.
      expect(getWorkloadAlertResourceIdCandidates(pod)).toEqual(['k8s:cluster-a:pod:pod-uid-1']);
    });
  });
});

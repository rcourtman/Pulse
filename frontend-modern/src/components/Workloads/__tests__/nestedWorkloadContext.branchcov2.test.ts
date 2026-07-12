import { describe, expect, it } from 'vitest';
import type { WorkloadGuest, WorkloadType } from '@/types/workloads';
import { buildNestedWorkloadContextByGuestId } from '../nestedWorkloadContext';

const makeGuest = (overrides: Partial<WorkloadGuest>): WorkloadGuest =>
  ({
    id: 'guest-1',
    vmid: 101,
    name: 'guest-1',
    node: 'node-a',
    instance: 'pve-a',
    status: 'running',
    type: 'lxc',
    workloadType: 'system-container',
    cpu: 0,
    cpus: 1,
    memory: { total: 0, used: 0, free: 0, usage: 0 },
    disk: { total: 0, used: 0, free: 0, usage: 0 },
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
    ...overrides,
  }) as WorkloadGuest;

// App-container children default to Docker scope membership and carry no
// Proxmox node/instance identity of their own so parent matching is driven
// only by the fields each test deliberately exercises.
const makeContainer = (overrides: Partial<WorkloadGuest>): WorkloadGuest =>
  makeGuest({
    id: 'app-container:x',
    vmid: 0,
    name: 'x',
    type: 'app-container',
    workloadType: 'app-container',
    platformType: 'docker',
    platformScopes: ['proxmox-pve', 'docker'],
    containerRuntime: 'docker',
    instance: '',
    node: '',
    ...overrides,
  });

const fullParent = (overrides: Partial<WorkloadGuest> = {}): WorkloadGuest =>
  makeGuest({
    id: 'lxc-141',
    vmid: 141,
    name: 'media-lxc',
    node: 'node-a',
    instance: 'pve-a',
    ...overrides,
  });

const run = ({
  guests,
  visibleGuests,
  excludedWorkloadTypes = ['app-container'],
  platformScope = 'proxmox-pve',
}: {
  guests: readonly WorkloadGuest[];
  visibleGuests: readonly WorkloadGuest[];
  excludedWorkloadTypes?: readonly WorkloadType[];
  platformScope?: string | null;
}) => buildNestedWorkloadContextByGuestId({ guests, visibleGuests, excludedWorkloadTypes, platformScope });

describe('nestedWorkloadContext (branch coverage)', () => {
  describe('formatRuntimeLabel', () => {
    it('maps a podman runtime (case/whitespace-insensitive) to "Podman"', () => {
      const parent = fullParent();
      const container = makeContainer({
        id: 'app-container:kea',
        name: 'kea',
        containerRuntime: '  PODMAN  ',
        dockerHostId: 'proxmox-lxc-docker:pve-a:node-a:141',
        dockerHostName: 'host-a',
      });

      const result = run({ guests: [container], visibleGuests: [parent] });

      expect(result['pve-a:node-a:141']).toStrictEqual({
        type: 'app-container',
        label: 'Podman',
        title: 'Nested Podman',
        count: 1,
        href: '/docker/overview?host=host-a',
        items: [{ id: 'app-container:kea', name: 'kea', status: 'running', runtimeLabel: 'Podman' }],
      });
    });

    it('maps any non-podman runtime to "Docker"', () => {
      const parent = fullParent();
      const container = makeContainer({
        id: 'app-container:svc',
        name: 'svc',
        containerRuntime: 'containerd',
        dockerHostId: 'proxmox-lxc-docker:pve-a:node-a:141',
        dockerHostName: 'host-a',
      });

      const result = run({ guests: [container], visibleGuests: [parent] });

      expect(result['pve-a:node-a:141']?.items[0]?.runtimeLabel).toBe('Docker');
      expect(result['pve-a:node-a:141']?.label).toBe('Docker');
    });
  });

  describe('formatStatusLabel', () => {
    it('returns the cleaned status when present and "unknown" for empty/whitespace values', () => {
      const parent = fullParent();
      const exited = makeContainer({
        id: 'app-container:alpha',
        name: 'alpha',
        status: 'exited',
        dockerHostId: 'proxmox-lxc-docker:pve-a:node-a:141',
      });
      const emptyStatus = makeContainer({
        id: 'app-container:beta',
        name: 'beta',
        status: '',
        dockerHostId: 'proxmox-lxc-docker:pve-a:node-a:141',
      });
      const whitespaceStatus = makeContainer({
        id: 'app-container:gamma',
        name: 'gamma',
        status: '   ',
        dockerHostId: 'proxmox-lxc-docker:pve-a:node-a:141',
      });

      const result = run({ guests: [exited, emptyStatus, whitespaceStatus], visibleGuests: [parent] });

      expect(result['pve-a:node-a:141']?.items.map((item) => item.status)).toEqual([
        'exited',
        'unknown',
        'unknown',
      ]);
    });
  });

  describe('createNestedItem', () => {
    it('falls back to containerId when the name is empty', () => {
      const parent = fullParent();
      const container = makeContainer({
        id: 'app-container:noid',
        name: '',
        containerId: 'cid-7',
        dockerHostId: 'proxmox-lxc-docker:pve-a:node-a:141',
      });

      const result = run({ guests: [container], visibleGuests: [parent] });

      expect(result['pve-a:node-a:141']?.items[0]?.name).toBe('cid-7');
    });

    it('falls back to the canonical workload id when both name and containerId are empty', () => {
      const parent = fullParent();
      const container = makeContainer({
        id: 'app-container:orphan',
        name: '',
        containerId: '',
        dockerHostId: 'proxmox-lxc-docker:pve-a:node-a:141',
      });

      const result = run({ guests: [container], visibleGuests: [parent] });

      expect(result['pve-a:node-a:141']?.items[0]?.name).toBe('app-container:orphan');
    });

    it('uses contextLabel for the host facet when dockerHostName is empty', () => {
      const parent = fullParent();
      const container = makeContainer({
        id: 'app-container:ctx',
        name: 'ctx',
        dockerHostName: '',
        contextLabel: 'ctx-host',
        dockerHostId: 'proxmox-lxc-docker:pve-a:node-a:141',
      });

      const result = run({ guests: [container], visibleGuests: [parent] });

      expect(result['pve-a:node-a:141']?.href).toBe('/docker/overview?host=ctx-host');
    });
  });

  describe('chooseContextLabel', () => {
    it('returns "Containers" and a "Nested containers" title when runtimes are mixed', () => {
      const parent = fullParent();
      const dockerOne = makeContainer({
        id: 'app-container:d',
        name: 'd',
        containerRuntime: 'docker',
        dockerHostId: 'proxmox-lxc-docker:pve-a:node-a:141',
        dockerHostName: 'shared-host',
      });
      const podmanOne = makeContainer({
        id: 'app-container:p',
        name: 'p',
        containerRuntime: 'podman',
        dockerHostId: 'proxmox-lxc-docker:pve-a:node-a:141',
        dockerHostName: 'shared-host',
      });

      const result = run({ guests: [dockerOne, podmanOne], visibleGuests: [parent] });

      expect(result['pve-a:node-a:141']?.label).toBe('Containers');
      expect(result['pve-a:node-a:141']?.title).toBe('Nested containers');
      expect(result['pve-a:node-a:141']?.count).toBe(2);
    });
  });

  describe('addDockerHostIdentitySegments', () => {
    it('matches a parent via an unprefixed 2-segment host id (prefix-false, >=2/>=1 arms)', () => {
      const parent = fullParent();
      const container = makeContainer({
        id: 'app-container:two',
        name: 'two',
        dockerHostId: 'node-a:141',
      });

      const result = run({ guests: [container], visibleGuests: [parent] });

      expect(Object.keys(result)).toEqual(['pve-a:node-a:141']);
      expect(result['pve-a:node-a:141']?.items[0]?.name).toBe('two');
    });

    it('matches a parent via an unprefixed 1-segment host id (>=3/>=2 false, >=1 true)', () => {
      const parent = fullParent();
      const container = makeContainer({
        id: 'app-container:one',
        name: 'one',
        dockerHostId: '141',
      });

      const result = run({ guests: [container], visibleGuests: [parent] });

      expect(Object.keys(result)).toEqual(['pve-a:node-a:141']);
    });

    it('yields no parent match when only the lxc docker prefix is present (all segment arms false)', () => {
      const parent = fullParent();
      const container = makeContainer({
        id: 'app-container:empty',
        name: 'empty',
        dockerHostId: 'proxmox-lxc-docker:',
      });

      const result = run({ guests: [container], visibleGuests: [parent] });

      expect(result).toEqual({});
    });

    it('skips candidate extraction entirely when dockerHostId is empty (early return)', () => {
      const parent = fullParent({ name: 'echo-host' });
      const container = makeContainer({
        id: 'app-container:echo',
        name: 'echo',
        dockerHostId: undefined,
        contextLabel: 'echo-host',
      });

      const result = run({ guests: [container], visibleGuests: [parent] });

      expect(Object.keys(result)).toEqual(['pve-a:node-a:141']);
      expect(result['pve-a:node-a:141']?.items[0]?.name).toBe('echo');
    });
  });

  describe('buildNodeScopedIdentityCandidates', () => {
    it('emits an instance:vmid candidate when node is missing (line 70 true arm)', () => {
      const parent = makeGuest({
        id: 'lxc-p2',
        vmid: 200,
        name: 'p2',
        node: '',
        instance: 'pve-b',
        workloadType: 'system-container',
        type: 'lxc',
      });
      const container = makeContainer({
        id: 'app-container:p2c',
        name: 'p2c',
        dockerHostId: 'pve-b:200',
      });

      const result = run({ guests: [container], visibleGuests: [parent] });

      expect(Object.keys(result)).toEqual(['lxc-p2']);
    });

    it('emits a node:vmid candidate when instance is missing (line 71 true arm)', () => {
      const parent = makeGuest({
        id: 'lxc-p3',
        vmid: 300,
        name: 'p3',
        node: 'node-c',
        instance: '',
        workloadType: 'system-container',
        type: 'lxc',
      });
      const container = makeContainer({
        id: 'app-container:p3c',
        name: 'p3c',
        dockerHostId: 'node-c:300',
      });

      const result = run({ guests: [container], visibleGuests: [parent] });

      expect(Object.keys(result)).toEqual(['lxc-p3']);
    });

    it('emits a bare vmid candidate when instance and node are missing (line 72 true arm)', () => {
      const parent = makeGuest({
        id: 'lxc-p4',
        vmid: 400,
        name: 'p4',
        node: '',
        instance: '',
        workloadType: 'system-container',
        type: 'lxc',
      });
      const container = makeContainer({
        id: 'app-container:p4c',
        name: 'p4c',
        dockerHostId: '400',
      });

      const result = run({ guests: [container], visibleGuests: [parent] });

      expect(Object.keys(result)).toEqual(['lxc-p4']);
    });

    it('skips every vmid-keyed candidate when vmid is not finite (line 62 false arm)', () => {
      const parent = makeGuest({
        id: 'lxc-p5',
        vmid: NaN,
        name: 'echo-host',
        node: 'node-e',
        instance: 'pve-e',
        workloadType: 'system-container',
        type: 'lxc',
      });
      const container = makeContainer({
        id: 'app-container:echoc',
        name: 'echoc',
        dockerHostId: undefined,
        contextLabel: 'echo-host',
      });

      const result = run({ guests: [container], visibleGuests: [parent] });

      expect(Object.keys(result)).toEqual(['lxc-p5']);
    });
  });

  describe('buildNestedWorkloadContextByGuestId pipeline', () => {
    it('produces a fully-shaped context for a canonical Docker app container', () => {
      const parent = fullParent();
      const container = makeContainer({
        id: 'app-container:frigate',
        name: 'frigate',
        containerRuntime: 'docker',
        status: 'running',
        dockerHostId: 'proxmox-lxc-docker:pve-a:node-a:141',
        dockerHostName: 'media-lxc.mist-stork.ts.net',
      });

      const result = run({ guests: [container], visibleGuests: [parent] });

      expect(result).toStrictEqual({
        'pve-a:node-a:141': {
          type: 'app-container',
          label: 'Docker',
          title: 'Nested Docker',
          count: 1,
          href: '/docker/overview?host=media-lxc.mist-stork.ts.net',
          items: [
            { id: 'app-container:frigate', name: 'frigate', status: 'running', runtimeLabel: 'Docker' },
          ],
        },
      });
    });

    it('registers a vm parent and ignores non-parent/non-child workload types', () => {
      const vmParent = makeGuest({
        id: 'vm-500',
        vmid: 500,
        name: 'vmhost',
        node: 'node-f',
        instance: 'pve-f',
        workloadType: 'vm',
        type: 'qemu',
      });
      const strayContainer = makeContainer({
        id: 'app-container:stray',
        name: 'stray',
        dockerHostId: 'proxmox-lxc-docker:pve-f:node-f:500',
      });
      const pod = makeGuest({
        id: 'k8s:agent:pod:p1',
        vmid: 0,
        name: 'p1',
        node: '',
        instance: '',
        workloadType: 'pod',
        type: 'pod',
      });
      const service = makeContainer({
        id: 'app-container:svc',
        name: 'svc',
        dockerHostId: 'proxmox-lxc-docker:pve-f:node-f:500',
        dockerHostName: 'h',
      });

      const result = run({
        guests: [vmParent, service],
        visibleGuests: [vmParent, strayContainer, pod],
      });

      expect(Object.keys(result)).toEqual(['pve-f:node-f:500']);
      expect(result['pve-f:node-f:500']?.items.map((item) => item.name)).toEqual(['svc']);
    });

    it('skips app containers that do not match the requested platform scope', () => {
      const parent = fullParent();
      const container = makeContainer({
        id: 'app-container:scope',
        name: 'scope',
        dockerHostId: 'proxmox-lxc-docker:pve-a:node-a:141',
        platformScopes: ['docker'],
      });

      const result = run({ guests: [container], visibleGuests: [parent] });

      expect(result).toEqual({});
    });

    it('skips app containers whose host identity maps to no known parent', () => {
      const parent = fullParent();
      const container = makeContainer({
        id: 'app-container:noparent',
        name: 'noparent',
        dockerHostId: 'proxmox-lxc-docker:999',
      });

      const result = run({ guests: [container], visibleGuests: [parent] });

      expect(result).toEqual({});
    });
  });
});

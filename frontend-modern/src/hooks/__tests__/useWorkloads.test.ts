import { createEffect, createRoot, createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import useWorkloadsSource from '../useWorkloads.ts?raw';

type UseWorkloadsModule = typeof import('@/hooks/useWorkloads');

const sampleResource = {
  id: 'cluster-a-pve1-101',
  type: 'vm',
  name: 'vm-101',
  status: 'running',
  lastSeen: '2026-02-06T12:00:00Z',
  vmid: 101,
  node: 'pve1',
  instance: 'cluster-a',
  metrics: {
    cpu: { percent: 25 },
    memory: { used: 2 * 1024, total: 4 * 1024, percent: 50 },
    disk: { used: 20 * 1024, total: 100 * 1024, percent: 20 },
  },
  sources: ['proxmox'],
};

const flushAsync = async () => {
  await Promise.resolve();
  await Promise.resolve();
};

const advanceAndFlush = async (ms: number) => {
  vi.advanceTimersByTime(ms);
  await flushAsync();
};

const deferred = <T>() => {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
};

const waitForWorkloadCount = async (getCount: () => number, expectedMin = 1) => {
  for (let i = 0; i < 20; i += 1) {
    if (getCount() >= expectedMin) {
      return;
    }
    await flushAsync();
  }
  throw new Error(`Timed out waiting for at least ${expectedMin} workloads`);
};

describe('useWorkloads', () => {
  let apiFetchJSONMock: ReturnType<typeof vi.fn>;
  let useWorkloads: UseWorkloadsModule['useWorkloads'];
  let resetWorkloadsCacheForTests: UseWorkloadsModule['__resetWorkloadsCacheForTests'];
  let eventBus: (typeof import('@/stores/events'))['eventBus'];

  beforeEach(async () => {
    vi.useFakeTimers();
    vi.resetModules();

    apiFetchJSONMock = vi.fn().mockResolvedValue({
      data: [sampleResource],
      meta: { totalPages: 1 },
    });

    vi.doMock('@/utils/apiClient', () => ({
      apiFetchJSON: apiFetchJSONMock,
      getOrgID: () => 'default',
    }));

    ({ useWorkloads, __resetWorkloadsCacheForTests: resetWorkloadsCacheForTests } =
      await import('@/hooks/useWorkloads'));
    ({ eventBus } = await import('@/stores/events'));

    resetWorkloadsCacheForTests();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
    vi.resetModules();
  });

  it('reuses fresh cache on remount without an extra network fetch', async () => {
    let disposeFirst = () => {};
    createRoot((d) => {
      disposeFirst = d;
      const [enabled] = createSignal(true);
      useWorkloads(enabled);
    });

    await flushAsync();
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);

    disposeFirst();

    let disposeSecond = () => {};
    createRoot((d) => {
      disposeSecond = d;
      const [enabled] = createSignal(true);
      useWorkloads(enabled);
    });

    await flushAsync();
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);

    disposeSecond();
  });

  it('keeps workload refreshes out of app-level Suspense', () => {
    expect(useWorkloadsSource).not.toContain('createResource');
    expect(useWorkloadsSource).toContain('createSignal<WorkloadGuest[]>');
  });

  it('projects authoritative LXC and QEMU CPU once regardless of agent sources or cores', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      data: [
        {
          ...sampleResource,
          id: 'cluster-a-pve1-101',
          type: 'vm',
          name: 'qemu-101',
          sources: ['proxmox'],
          metrics: { ...sampleResource.metrics, cpu: { percent: 0.58 } },
          proxmox: { vmid: 101, nodeName: 'pve1', instance: 'cluster-a', cpus: 8 },
        },
        {
          ...sampleResource,
          id: 'cluster-a-pve1-102',
          type: 'system-container',
          name: 'lxc-102',
          vmid: 102,
          sources: ['proxmox', 'agent'],
          metrics: { ...sampleResource.metrics, cpu: { percent: 0.58 } },
          proxmox: { vmid: 102, nodeName: 'pve1', instance: 'cluster-a', cpus: 1 },
        },
      ],
      meta: { totalPages: 1 },
    });

    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
    });

    await waitForWorkloadCount(() => result!.workloads().length, 2);

    const byName = new Map(result!.workloads().map((workload) => [workload.name, workload]));
    expect(byName.get('qemu-101')?.cpu).toBe(0.0058);
    expect(byName.get('qemu-101')?.cpus).toBe(8);
    expect(byName.get('lxc-102')?.cpu).toBe(0.0058);
    expect(byName.get('lxc-102')?.cpus).toBe(1);

    dispose();
  });

  it('projects agent-reported libvirt VM runtime metadata without Proxmox fields', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      data: [
        {
          ...sampleResource,
          id: 'vm-agent-domain-a1b2c3d4',
          name: 'libvirt-app',
          status: 'warning',
          sources: ['agent'],
          parentName: 'qnap-host',
          vmid: undefined,
          node: undefined,
          instance: undefined,
          proxmox: undefined,
          virtualMachine: {
            runtimeState: 'running',
            hypervisor: 'libvirt',
            vcpus: 6,
          },
        },
      ],
      meta: { totalPages: 1 },
    });

    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
    });

    await waitForWorkloadCount(() => result!.workloads().length);

    expect(result!.workloads()).toHaveLength(1);
    expect(result!.workloads()[0]).toMatchObject({
      id: 'vm-agent-domain-a1b2c3d4',
      name: 'libvirt-app',
      node: 'qnap-host',
      status: 'running',
      type: 'vm',
      cpus: 6,
      platformType: 'agent',
    });

    dispose();
  });

  it('handles empty responses without mutating into undefined state', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      data: [],
      meta: { totalPages: 1 },
    });

    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
    });

    await flushAsync();
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);
    expect(result!.workloads()).toEqual([]);

    dispose();
  });

  it('uses an owning canonical resource snapshot without a second workload request', async () => {
    const snapshot = [
      {
        id: 'cluster-a-pve1-101',
        type: 'vm',
        name: 'vm-101',
        status: 'running',
        platformType: 'proxmox-pve',
        sources: ['proxmox'],
        identity: { hostname: 'vm-101', ips: ['192.0.2.101'] },
        metricsTarget: { resourceType: 'vm', resourceId: 'mock-cluster-a-pve1-101' },
        canonicalIdentity: {
          primaryId: 'vm:cluster-a-pve1-101',
          aliases: ['legacy-vm-101'],
        },
        proxmox: {
          sourceId: 'cluster-a-pve1-101',
          vmid: 101,
          nodeName: 'pve1',
          instance: 'cluster-a',
          lastBackup: '2026-02-06T06:00:00Z',
        },
        uptime: 86_400,
        cpu: { current: 25 },
        memory: { current: 50, used: 2 * 1024, total: 4 * 1024 },
        disk: { current: 20, used: 20 * 1024, total: 100 * 1024 },
      },
    ] as any;
    const refetchSnapshot = vi.fn().mockResolvedValue(undefined);

    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled, {
        resourceSnapshot: () => snapshot,
        refetchSnapshot,
      });
    });

    await flushAsync();
    expect(apiFetchJSONMock).not.toHaveBeenCalled();
    expect(result!.workloads()).toMatchObject([
      {
        name: 'vm-101',
        vmid: 101,
        node: 'pve1',
        metricsTarget: { resourceType: 'vm', resourceId: 'mock-cluster-a-pve1-101' },
        uptime: 86_400,
        lastBackup: Date.parse('2026-02-06T06:00:00Z'),
        alertResourceIds: expect.arrayContaining([
          'cluster-a-pve1-101',
          'mock-cluster-a-pve1-101',
          'vm:cluster-a-pve1-101',
          'legacy-vm-101',
        ]),
      },
    ]);

    await result!.refetch();
    expect(refetchSnapshot).toHaveBeenCalledTimes(1);
    expect(apiFetchJSONMock).not.toHaveBeenCalled();

    dispose();
  });

  it('keeps row identity stable for guests whose data did not change', async () => {
    const buildGuest = (id: string, vmid: number, cpu: number) =>
      ({
        id,
        type: 'vm',
        name: `vm-${vmid}`,
        status: 'running',
        platformType: 'proxmox-pve',
        sources: ['proxmox'],
        proxmox: { sourceId: id, vmid, nodeName: 'pve1', instance: 'cluster-a' },
        cpu: { current: cpu },
      }) as any;
    const [snapshot, setSnapshot] = createSignal([
      buildGuest('vm-a', 101, 10),
      buildGuest('vm-b', 102, 20),
    ]);

    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled, { resourceSnapshot: snapshot });
    });

    await flushAsync();
    const firstRows = result!.workloads();
    expect(firstRows).toHaveLength(2);

    // One guest changes; the untouched guest's row object must be reused.
    setSnapshot([buildGuest('vm-a', 101, 10), buildGuest('vm-b', 102, 85)]);
    await flushAsync();
    const secondRows = result!.workloads();
    expect(secondRows[0]).toBe(firstRows[0]);
    expect(secondRows[1]).not.toBe(firstRows[1]);
    expect(secondRows[1]?.cpu).toBeCloseTo(0.85);

    // A refresh that changes nothing must keep the array identity itself.
    setSnapshot([buildGuest('vm-a', 101, 10), buildGuest('vm-b', 102, 85)]);
    await flushAsync();
    expect(result!.workloads()).toBe(secondRows);

    dispose();
  });

  it('retains the fulfilled workload snapshot when a forced refresh fails', async () => {
    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
    });

    await flushAsync();
    await waitForWorkloadCount(() => result!.workloads().length);
    const initialWorkloads = result!.workloads();

    apiFetchJSONMock.mockRejectedValueOnce(new Error('temporary backend gap'));
    await expect(result!.refetch()).rejects.toThrow('temporary backend gap');

    expect(result!.workloads()).toBe(initialWorkloads);
    expect(result!.workloads()).toHaveLength(1);
    expect(result!.loading()).toBe(false);
    expect(result!.error()).toBeInstanceOf(Error);

    dispose();
  });

  it('clears a transient refresh error after a later poll succeeds', async () => {
    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
    });

    await flushAsync();
    await waitForWorkloadCount(() => result!.workloads().length);

    apiFetchJSONMock.mockRejectedValueOnce(new Error('temporary backend gap'));
    await expect(result!.refetch()).rejects.toThrow('temporary backend gap');
    expect(result!.error()).toBeInstanceOf(Error);

    await advanceAndFlush(5_000);
    expect(result!.workloads()).toHaveLength(1);
    expect(result!.error()).toBeUndefined();

    dispose();
  });

  it('keeps Proxmox power state stable while carrying aggregate health, then removes an authoritative deletion', async () => {
    const guest = (vmid: number, status: string, runtimeStatus: string) => ({
      ...sampleResource,
      id: `cluster-a-pve1-${vmid}`,
      type: 'system-container',
      name: `lxc-${vmid}`,
      status,
      vmid,
      proxmox: {
        vmid,
        nodeName: 'pve1',
        instance: 'cluster-a',
        runtimeStatus,
      },
    });

    apiFetchJSONMock.mockResolvedValueOnce({
      data: [
        guest(101, 'online', 'running'),
        guest(102, 'online', 'running'),
        guest(103, 'online', 'running'),
      ],
      meta: { totalPages: 1 },
    });

    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
    });

    await waitForWorkloadCount(() => result!.workloads().length, 3);
    const initialIds = result!.workloads().map((workload) => workload.id);

    apiFetchJSONMock.mockResolvedValueOnce({
      data: [
        guest(101, 'warning', 'running'),
        {
          ...guest(102, 'online', 'running'),
          availability: { targetId: 'probe-102', protocol: 'icmp', enabled: true, available: true },
        },
        guest(103, 'warning', 'running'),
      ],
      meta: { totalPages: 1 },
    });
    await result!.refetch();

    expect(result!.workloads().map((workload) => workload.id)).toEqual(initialIds);
    expect(result!.workloads().map((workload) => workload.status)).toEqual([
      'running',
      'running',
      'running',
    ]);
    expect(result!.workloads().map((workload) => workload.resourceStatus)).toEqual([
      'warning',
      'online',
      'warning',
    ]);

    apiFetchJSONMock.mockResolvedValueOnce({
      data: [guest(101, 'online', 'running'), guest(102, 'online', 'running')],
      meta: { totalPages: 1 },
    });
    await result!.refetch();

    expect(result!.workloads().map((workload) => workload.id)).toEqual(initialIds.slice(0, 2));
    expect(result!.error()).toBeUndefined();

    dispose();
  });

  it('rejects a partial paged refresh and retains the last coherent snapshot', async () => {
    const secondResource = {
      ...sampleResource,
      id: 'cluster-a-pve1-102',
      name: 'vm-102',
      vmid: 102,
    };
    apiFetchJSONMock.mockResolvedValueOnce({
      data: [sampleResource, secondResource],
      meta: { totalPages: 1 },
    });

    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
    });

    await waitForWorkloadCount(() => result!.workloads().length, 2);
    const coherentSnapshot = result!.workloads();

    apiFetchJSONMock
      .mockResolvedValueOnce({
        data: [sampleResource],
        meta: { totalPages: 2 },
      })
      .mockRejectedValueOnce(new Error('page 2 unavailable'));

    await expect(result!.refetch()).rejects.toThrow('page 2 unavailable');
    expect(result!.workloads()).toBe(coherentSnapshot);
    expect(result!.workloads()).toHaveLength(2);
    expect(result!.error()).toBeInstanceOf(Error);

    apiFetchJSONMock
      .mockResolvedValueOnce({
        data: [sampleResource],
        meta: { totalPages: 2 },
      })
      .mockResolvedValueOnce({
        data: [],
        meta: { totalPages: 2 },
      });
    await result!.refetch();

    expect(result!.workloads()).toHaveLength(1);
    expect(result!.error()).toBeUndefined();

    dispose();
  });

  it('does not apply in-flight workload results after the hook is disabled', async () => {
    const pendingFetch = deferred<unknown>();
    apiFetchJSONMock.mockImplementationOnce(() => pendingFetch.promise as Promise<any>);

    let dispose = () => {};
    let setEnabled: ((enabled: boolean) => boolean) | undefined;
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled, updateEnabled] = createSignal(true);
      setEnabled = updateEnabled;
      result = useWorkloads(enabled);
    });

    await flushAsync();
    expect(result!.loading()).toBe(true);

    setEnabled!(false);
    await flushAsync();
    expect(result!.loading()).toBe(false);

    pendingFetch.resolve({
      data: [sampleResource],
      meta: { totalPages: 1 },
    });
    await flushAsync();

    expect(result!.workloads()).toEqual([]);
    expect(result!.error()).toBeUndefined();

    dispose();
  });

  it('normalizes workload source aliases into canonical platform types', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      data: [
        { ...sampleResource, id: 'vm-pve', sources: ['PROXMOX'] },
        { ...sampleResource, id: 'vm-pbs', sources: ['pbs'], name: 'vm-pbs' },
        { ...sampleResource, id: 'vm-pmg', sources: ['pmg'], name: 'vm-pmg' },
        {
          ...sampleResource,
          id: 'app-container:truenas-main:nextcloud',
          type: 'app-container',
          name: 'nextcloud',
          status: 'healthy',
          sources: ['agent', 'truenas'],
          parentName: 'truenas-main',
          docker: {
            containerId: 'nextcloud-ctr',
            image: 'ix-nextcloud:latest',
          },
        },
        {
          ...sampleResource,
          id: 'docker-container-frigate-141',
          type: 'app-container',
          name: 'frigate',
          status: 'running',
          sources: ['docker'],
          platformScopes: ['proxmox-pve', 'docker'],
          parentName: 'homepage-docker.lab',
          docker: {
            containerId: 'frigate',
            hostSourceId: 'proxmox-lxc-docker:pve-a:node-a:141',
            hostname: 'homepage-docker.lab',
            runtime: 'docker',
            image: 'ghcr.io/blakeblackshear/frigate:stable',
          },
        },
        {
          ...sampleResource,
          id: 'vmware-vm-crm',
          name: 'crm-app-01',
          status: undefined,
          sources: ['vmware'],
          parentName: 'esxi-01',
          proxmox: undefined,
          vmware: {
            managedObjectId: 'vm-101',
            runtimeHostName: 'esxi-01',
            clusterName: 'Compute-A',
            connectionName: 'Production vCenter',
            powerState: 'poweredOn',
          },
        },
        {
          ...sampleResource,
          id: 'vmware-vm-legacy',
          name: 'legacy-app-01',
          status: undefined,
          sources: ['vmware'],
          parentName: 'esxi-02',
          proxmox: undefined,
          vmware: {
            managedObjectId: 'vm-102',
            runtimeHostName: 'esxi-02',
            clusterName: 'Compute-A',
            connectionName: 'Production vCenter',
            powerState: 'poweredOff',
          },
        },
      ],
      meta: { totalPages: 1 },
    });

    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
    });

    await flushAsync();
    await waitForWorkloadCount(() => result!.workloads().length, 7);

    const byName = new Map(result!.workloads().map((workload) => [workload.name, workload]));
    expect(byName.get('vm-101')?.platformType).toBe('proxmox-pve');
    expect(byName.get('vm-pbs')?.platformType).toBe('proxmox-pbs');
    expect(byName.get('vm-pmg')?.platformType).toBe('proxmox-pmg');
    expect(byName.get('nextcloud')?.platformType).toBe('truenas');
    expect(byName.get('nextcloud')?.platformScopes).toEqual(['truenas']);
    expect(byName.get('nextcloud')?.id).toBe('app-container:truenas-main:nextcloud');
    expect(byName.get('nextcloud')?.containerId).toBe('nextcloud-ctr');
    expect(byName.get('nextcloud')?.dockerHostId).toBeUndefined();
    expect(byName.get('frigate')?.platformType).toBe('docker');
    expect(byName.get('frigate')?.platformScopes).toEqual(['proxmox-pve', 'docker']);
    expect(byName.get('frigate')?.dockerHostId).toBe('proxmox-lxc-docker:pve-a:node-a:141');
    expect(byName.get('frigate')?.dockerHostName).toBe('homepage-docker.lab');
    expect(byName.get('crm-app-01')?.platformType).toBe('vmware-vsphere');
    expect(byName.get('crm-app-01')?.node).toBe('esxi-01');
    expect(byName.get('crm-app-01')?.instance).toBe('Compute-A');
    expect(byName.get('crm-app-01')?.status).toBe('running');
    expect(byName.get('legacy-app-01')?.status).toBe('stopped');

    dispose();
  });

  it('falls back to canonical resource.uptime when no platform-specific carve-out exists (vSphere)', async () => {
    // vSphere VMs surface uptime only on the canonical Resource.Uptime
    // field; there is no proxmox/agent/docker/kubernetes carve-out. The
    // useWorkloads mapping must land on resource.uptime so the workloads
    // table renders real "N days" cells rather than the blank "0s"
    // placeholder.
    apiFetchJSONMock.mockResolvedValueOnce({
      data: [
        {
          ...sampleResource,
          id: 'vmware-vm-uptime',
          name: 'lab-app-01',
          status: 'online',
          uptime: 12345678,
          sources: ['vmware'],
          parentName: 'esxi-01',
          proxmox: undefined,
          vmware: {
            managedObjectId: 'vm-901',
            runtimeHostName: 'esxi-01',
            clusterName: 'Compute-A',
            connectionName: 'Production vCenter',
            powerState: 'poweredOn',
          },
        },
      ],
      meta: { totalPages: 1 },
    });

    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
    });

    await flushAsync();
    await waitForWorkloadCount(() => result!.workloads().length, 1);

    const vm = result!.workloads().find((workload) => workload.name === 'lab-app-01');
    expect(vm?.uptime).toBe(12345678);

    dispose();
  });

  it('preserves API telemetry absence separately from reported zero values on every platform', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      data: [
        {
          id: 'vmware-vm-unknown-telemetry',
          type: 'vm',
          name: 'unknown-telemetry',
          status: 'online',
          lastSeen: 'not-a-timestamp',
          sources: ['vmware'],
          metrics: {},
          agent: { agentVersion: '6.2.0' },
          vmware: {
            managedObjectId: 'vm-903',
            powerState: 'poweredOn',
            cpuCount: 8,
          },
        },
        {
          id: 'docker-container-zero-telemetry',
          type: 'app-container',
          name: 'zero-telemetry',
          status: 'running',
          lastSeen: '2026-08-13T10:00:00Z',
          sources: ['docker'],
          uptime: 0,
          metrics: {
            cpu: { percent: 0 },
            memory: { used: 0, total: 1024, percent: 0 },
            disk: { used: 0, total: 2048, percent: 0 },
            netIn: { value: 0 },
            netOut: { value: 0 },
            diskRead: { value: 0 },
            diskWrite: { value: 0 },
          },
          docker: { containerId: 'zero', runtime: 'docker', uptimeSeconds: 0 },
        },
      ],
      meta: { totalPages: 1 },
    });

    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
    });

    await waitForWorkloadCount(() => result!.workloads().length, 2);
    const byName = new Map(result!.workloads().map((workload) => [workload.name, workload]));

    expect(byName.get('unknown-telemetry')).toMatchObject({
      cpus: 8,
      agentKind: 'pulse',
      lastSeen: '',
      telemetryAvailability: {
        cpu: false,
        memory: false,
        disk: false,
        networkIO: false,
        diskIO: false,
        uptime: false,
      },
    });
    expect(byName.get('zero-telemetry')).toMatchObject({
      cpu: 0,
      networkIn: 0,
      networkOut: 0,
      diskRead: 0,
      diskWrite: 0,
      uptime: 0,
      telemetryAvailability: {
        cpu: true,
        memory: true,
        disk: true,
        networkIO: true,
        diskIO: true,
        uptime: true,
      },
    });

    dispose();
  });

  it('renders vSphere row tags from the vCenter facet, never the provenance keyword set', async () => {
    // `Resource.Tags` on vSphere is a mixed keyword set: adapter provenance
    // strings (kept so resource search and the `?tags=` filter keep matching)
    // plus the operator's real vCenter tags. A per-row Tags cell must show
    // only the vCenter tags, and a vSphere VM nobody tagged must show nothing
    // rather than falling back to six identical provenance dots.
    apiFetchJSONMock.mockResolvedValueOnce({
      data: [
        {
          ...sampleResource,
          id: 'vmware-vm-tagged',
          name: 'tagged-app-01',
          sources: ['vmware'],
          proxmox: undefined,
          tags: [
            'vmware',
            'vsphere',
            'vm',
            'source:vcenter',
            'Environment:Production',
            'Owner:Data',
          ],
          vmware: {
            managedObjectId: 'vm-902',
            powerState: 'poweredOn',
            tags: [
              { name: 'Production', category: 'Environment', label: 'Environment:Production' },
              { name: 'Data', category: 'Owner', label: 'Owner:Data' },
            ],
          },
        },
        {
          ...sampleResource,
          id: 'vmware-vm-untagged',
          name: 'untagged-app-01',
          sources: ['vmware'],
          proxmox: undefined,
          tags: ['vmware', 'vsphere', 'vm', 'source:vcenter'],
          vmware: {
            managedObjectId: 'vm-903',
            powerState: 'poweredOn',
          },
        },
        {
          ...sampleResource,
          id: 'proxmox-vm-tagged',
          name: 'pve-app-01',
          tags: ['production', 'web'],
        },
      ],
      meta: { totalPages: 1 },
    });

    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
    });

    await flushAsync();
    await waitForWorkloadCount(() => result!.workloads().length, 3);

    const byName = (name: string) => result!.workloads().find((workload) => workload.name === name);
    expect(byName('tagged-app-01')?.tags).toEqual(['Environment:Production', 'Owner:Data']);
    expect(byName('untagged-app-01')?.tags).toEqual([]);
    // Platforms whose flat tags are already real labels are untouched.
    expect(byName('pve-app-01')?.tags).toEqual(['production', 'web']);

    dispose();
  });

  it('preserves canonical discovery targets for workloads instead of inferring them from platform type', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      data: [
        {
          ...sampleResource,
          id: 'app-container:truenas-main:nextcloud',
          type: 'app-container',
          name: 'nextcloud',
          status: 'healthy',
          sources: ['agent', 'truenas'],
          parentName: 'truenas-main',
          discoveryTarget: {
            resourceType: 'app-container',
            agentId: 'truenas-helper',
            resourceId: 'nextcloud',
          },
          docker: {
            containerId: 'nextcloud-ctr',
            image: 'ix-nextcloud:latest',
          },
        },
      ],
      meta: { totalPages: 1 },
    });

    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
    });

    await flushAsync();
    await waitForWorkloadCount(() => result!.workloads().length, 1);

    expect(result!.workloads()[0]?.discoveryTarget).toEqual({
      resourceType: 'app-container',
      agentId: 'truenas-helper',
      resourceId: 'nextcloud',
      hostname: undefined,
    });

    dispose();
  });

  it('uses the canonical Kubernetes cluster name for pod context labels', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      data: [
        {
          ...sampleResource,
          id: 'pod-1',
          type: 'pod',
          name: 'pod-1',
          kubernetes: {
            clusterId: 'cluster-id-a',
            clusterName: 'cluster-a',
            context: 'cluster-context',
            namespace: 'default',
            podUid: 'pod-uid-1',
          },
        },
      ],
      meta: { totalPages: 1 },
    });

    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
    });

    await flushAsync();
    await waitForWorkloadCount(() => result!.workloads().length);

    expect(result!.workloads()[0]?.contextLabel).toBe('cluster-a');
    expect(result!.workloads()[0]?.instance).toBe('cluster-a');
    expect(result!.workloads()[0]?.kubernetesClusterId).toBe('cluster-id-a');

    dispose();
  });

  it('uses the shared cluster-name helper for proxmox workload labels', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      data: [
        {
          ...sampleResource,
          id: 'vm-1',
          type: 'vm',
          name: 'vm-1',
          instance: undefined,
          proxmox: {
            clusterName: 'cluster-b',
          },
        },
      ],
      meta: { totalPages: 1 },
    });

    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
    });

    await flushAsync();
    await waitForWorkloadCount(() => result!.workloads().length);

    expect(result!.workloads()[0]?.clusterName).toBe('cluster-b');
    expect(result!.workloads()[0]?.contextLabel).toBe('pve1 (cluster-b)');
    expect(result!.workloads()[0]?.instance).toBe('cluster-b');
    expect(result!.workloads()[0]?.id).toBe('cluster-b:pve1:101');

    dispose();
  });

  it('keeps workload reference stable when polling returns identical payload', async () => {
    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    let effectRuns = 0;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
      createEffect(() => {
        result!.workloads();
        effectRuns += 1;
      });
    });

    await flushAsync();
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);
    await waitForWorkloadCount(() => result!.workloads().length);
    const initialRef = result!.workloads();
    const initialEffectRuns = effectRuns;

    await advanceAndFlush(5_000);
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(2);
    expect(result!.workloads()).toBe(initialRef);
    expect(effectRuns).toBe(initialEffectRuns);

    dispose();
  });

  it('maintains polling cadence under load without overlapping fetch churn', async () => {
    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
    });

    await flushAsync();
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);
    await waitForWorkloadCount(() => result!.workloads().length);

    const slowPoll = deferred<unknown>();
    apiFetchJSONMock.mockImplementationOnce(() => slowPoll.promise as Promise<any>);

    await advanceAndFlush(5_000);
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(2);

    await advanceAndFlush(10_000);
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(2);

    slowPoll.resolve({
      data: [sampleResource],
      meta: { totalPages: 1 },
    });
    await flushAsync();

    await advanceAndFlush(5_000);
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(3);

    dispose();
  });

  it('scopes shared cache by org and restores cached data when switching back', async () => {
    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
    });

    await flushAsync();
    await waitForWorkloadCount(() => result!.workloads().length);
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);
    expect(result!.workloads()[0]?.id).toBe('cluster-a:pve1:101');

    apiFetchJSONMock.mockResolvedValueOnce({
      data: [
        {
          ...sampleResource,
          id: 'cluster-b-pve2-202',
          name: 'vm-202',
          vmid: 202,
          node: 'pve2',
          instance: 'cluster-b',
        },
      ],
      meta: { totalPages: 1 },
    });

    eventBus.emit('org_switched', 'tenant-b');
    await flushAsync();
    await waitForWorkloadCount(() => result!.workloads().length);

    expect(apiFetchJSONMock).toHaveBeenCalledTimes(2);
    expect(result!.workloads()[0]?.id).toBe('cluster-b:pve2:202');

    eventBus.emit('org_switched', 'default');
    await flushAsync();

    expect(apiFetchJSONMock).toHaveBeenCalledTimes(2);
    expect(result!.workloads()[0]?.id).toBe('cluster-a:pve1:101');

    dispose();
  });

  it('normalizes org scope through the shared helper', () => {
    expect(useWorkloadsSource).toContain('normalizeOrgScope(getOrgID())');
    expect(useWorkloadsSource).not.toContain("const DEFAULT_ORG_SCOPE = 'default'");
    expect(useWorkloadsSource).not.toContain('const normalizeOrgScope =');
  });
});

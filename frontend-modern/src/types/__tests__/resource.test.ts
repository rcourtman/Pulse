/**
 * Tests for resource type guards and helper functions
 */
import { describe, expect, it } from 'vitest';
import {
  PLATFORM_TYPES,
  isInfrastructure,
  isWorkload,
  isStorage,
  getCpuPercent,
  getMemoryPercent,
  getDiskPercent,
  type ResourceChangeKind,
  type ResourceAgentUnraidDisk,
  type ResourceDockerMeta,
  type ResourcePhysicalDiskMeta,
  type ResourceProxmoxMeta,
  type ResourceVMwareMeta,
  type Resource,
  type ResourceType,
} from '@/types/resource';
import {
  ADMITTED_PLATFORM_IDS,
  PRESENTATION_ONLY_PLATFORM_IDS,
  SOURCE_PLATFORM_CANONICAL_PROJECTIONS,
  SUPPORTED_PLATFORM_IDS,
} from '@/utils/platformSupportManifest.generated';
import { getPreferredResourceDisplayName } from '@/utils/resourceIdentity';

// Helper to create a minimal resource for testing
function createResource(overrides: Partial<Resource> = {}): Resource {
  return {
    id: 'test-1',
    type: 'vm',
    name: 'test-resource',
    displayName: '',
    platformId: 'platform-1',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'running',
    lastSeen: Date.now(),
    ...overrides,
  };
}

describe('Resource Type Guards', () => {
  it('keeps platform types aligned with the governed platform manifest projection', () => {
    expect(PLATFORM_TYPES).toEqual([...SUPPORTED_PLATFORM_IDS, ...ADMITTED_PLATFORM_IDS]);
    for (const platform of PRESENTATION_ONLY_PLATFORM_IDS) {
      expect(PLATFORM_TYPES).not.toContain(platform as any);
    }
  });

  it('keeps TrueNAS native VM support in the governed platform projection', () => {
    expect(SOURCE_PLATFORM_CANONICAL_PROJECTIONS.truenas).toEqual([
      'agent',
      'vm',
      'app-container',
      'network-share',
      'storage',
      'physical-disk',
    ]);
  });

  describe('isInfrastructure', () => {
    const infrastructureTypes: ResourceType[] = [
      'agent',
      'docker-host',
      'k8s-node',
      'k8s-cluster',
      'network-endpoint',
    ];
    const nonInfrastructureTypes: ResourceType[] = [
      'vm',
      'system-container',
      'app-container',
      'pod',
      'jail',
      'storage',
      'network',
    ];

    it.each(infrastructureTypes)('returns true for %s', (type) => {
      const resource = createResource({ type });
      expect(isInfrastructure(resource)).toBe(true);
    });

    it.each(nonInfrastructureTypes)('returns false for %s', (type) => {
      const resource = createResource({ type });
      expect(isInfrastructure(resource)).toBe(false);
    });
  });

  describe('isWorkload', () => {
    const workloadTypes: ResourceType[] = [
      'vm',
      'system-container',
      'app-container',
      'pod',
      'jail',
    ];
    const nonWorkloadTypes: ResourceType[] = [
      'agent',
      'docker-host',
      'network-endpoint',
      'network',
      'storage',
      'pbs',
    ];

    it.each(workloadTypes)('returns true for %s', (type) => {
      const resource = createResource({ type });
      expect(isWorkload(resource)).toBe(true);
    });

    it.each(nonWorkloadTypes)('returns false for %s', (type) => {
      const resource = createResource({ type });
      expect(isWorkload(resource)).toBe(false);
    });
  });

  describe('isStorage', () => {
    const storageTypes: ResourceType[] = [
      'storage',
      'datastore',
      'pool',
      'dataset',
      'network-share',
    ];
    const nonStorageTypes: ResourceType[] = [
      'vm',
      'agent',
      'system-container',
      'docker-host',
      'network-endpoint',
      'network',
    ];

    it.each(storageTypes)('returns true for %s', (type) => {
      const resource = createResource({ type });
      expect(isStorage(resource)).toBe(true);
    });

    it.each(nonStorageTypes)('returns false for %s', (type) => {
      const resource = createResource({ type });
      expect(isStorage(resource)).toBe(false);
    });
  });
});

describe('Resource Helper Functions', () => {
  describe('ResourceDockerMeta', () => {
    it('accepts Docker host runtime and Swarm evidence in the canonical resource contract', () => {
      const docker: ResourceDockerMeta = {
        hostname: 'tower',
        runtime: 'docker',
        runtimeVersion: '27.5.1',
        dockerVersion: '27.5.1',
        os: 'Unraid OS',
        kernelVersion: 'Linux 6.1.106-Unraid',
        architecture: 'x86_64',
        agentVersion: '6.0.0',
        uptimeSeconds: 13_046_400,
        temperature: 51.9,
        containerCount: 12,
        updatesAvailableCount: 2,
        updatesLastCheckedAt: '2026-05-17T18:00:00Z',
        command: { restartContainer: true },
        swarm: {
          clusterId: 'swarm-1',
          clusterName: 'homelab',
          nodeId: 'node-1',
          nodeRole: 'manager',
          localState: 'active',
          controlAvailable: true,
        },
      };

      expect(docker.runtimeVersion).toBe('27.5.1');
      expect(docker.containerCount).toBe(12);
      expect(docker.swarm?.localState).toBe('active');
      expect(docker.swarm?.controlAvailable).toBe(true);
    });
  });

  describe('ResourceAgentUnraidDisk', () => {
    it('accepts native Unraid disk metadata in the resource contract', () => {
      const disk: ResourceAgentUnraidDisk = {
        name: 'disk1',
        device: '/dev/sdc',
        role: 'data',
        status: 'online',
        rawStatus: 'DISK_OK',
        model: 'WDC WD60EFRX',
        serial: 'SERIAL-DATA',
        filesystem: 'xfs',
        transport: 'sata',
        sizeBytes: 6_000_000_000_000,
        usedBytes: 4_000,
        freeBytes: 2_000,
        temperature: 31,
        spunDown: true,
        readCount: 11,
        writeCount: 12,
        errorCount: 16,
        slot: 1,
      };

      expect(disk.model).toBe('WDC WD60EFRX');
      expect(disk.usedBytes).toBe(4_000);
      expect(disk.errorCount).toBe(16);
    });
  });

  describe('ResourcePhysicalDiskMeta', () => {
    it('accepts native UnRAID media state on physical disks', () => {
      const disk: ResourcePhysicalDiskMeta = {
        devPath: '/dev/sdc',
        model: 'WDC WD60EFRX',
        storageRole: 'data',
        storageGroup: 'unraid-array',
        storageState: 'online',
        spunDown: true,
        readCount: 11,
        writeCount: 12,
        errorCount: 16,
      };

      expect(disk.storageGroup).toBe('unraid-array');
      expect(disk.spunDown).toBe(true);
      expect(disk.errorCount).toBe(16);
    });
  });

  describe('ResourceProxmoxMeta', () => {
    it('accepts Proxmox pool membership in the canonical resource contract', () => {
      const proxmox: ResourceProxmoxMeta = {
        nodeName: 'pve-a',
        instance: 'cluster-a',
        pool: 'prod-vms',
        pveVersion: 'pve-manager/9.1.9/ee7bad0a3d1546c9',
        kernelVersion: 'Linux 6.8.12-10-pve',
      };

      expect(proxmox.pool).toBe('prod-vms');
      expect(proxmox.pveVersion).toBe('pve-manager/9.1.9/ee7bad0a3d1546c9');
      expect(proxmox.kernelVersion).toBe('Linux 6.8.12-10-pve');
    });
  });

  describe('ResourceVMwareMeta', () => {
    it('accepts VMware signal metadata on canonical resources', () => {
      const vmware: ResourceVMwareMeta = {
        connectionId: 'vc-1',
        connectionName: 'Lab VC',
        vcenterHost: 'vc.lab.local',
        managedObjectId: 'vm-101',
        entityType: 'vm',
        datacenterName: 'DC1',
        clusterName: 'Production Cluster',
        clusterHaEnabled: true,
        clusterDrsEnabled: false,
        folderName: 'Workloads',
        resourcePoolName: 'Tier 1',
        runtimeHostName: 'esxi-01.lab.local',
        overallStatus: 'yellow',
        datastoreNames: ['primary-vmfs'],
        networkType: 'STANDARD_PORTGROUP',
        networkHostNames: ['esxi-01.lab.local'],
        networkVmNames: ['app-01'],
        instanceUuid: 'vm-instance-101',
        guestHostname: 'app-01.internal',
        guestIpAddresses: ['10.0.0.21'],
        activeAlarmCount: 2,
        activeAlarmSummary: 'CPU ready alarm; snapshot age warning',
        recentTaskCount: 1,
        recentTaskSummary: 'Clone VM task finished',
        snapshotCount: 3,
        currentSnapshotId: 'snapshot-103',
        networkAdapters: [
          {
            nic: '4000',
            label: 'Network adapter 1',
            type: 'VMXNET3',
            macAddress: '00:50:56:aa:bb:cc',
            backingType: 'STANDARD_PORTGROUP',
            networkId: 'network-101',
            networkName: 'VM Network',
            state: 'CONNECTED',
            startConnected: true,
            allowGuestControl: true,
          },
        ],
        virtualDisks: [
          {
            disk: '2000',
            label: 'Hard disk 1',
            type: 'SCSI',
            scsiBus: 0,
            scsiUnit: 1,
            backingType: 'VMDK_FILE',
            vmdkFile: '[primary-vmfs] app-01/app-01.vmdk',
            datastoreName: 'primary-vmfs',
            capacityBytes: 107374182400,
          },
        ],
        tools: {
          runState: 'RUNNING',
          versionStatus: 'CURRENT',
          version: '12.4.0',
          versionNumber: 12352,
          installType: 'OPEN_VM_TOOLS',
          upgradePolicy: 'MANUAL',
          autoUpdateSupported: true,
          installAttemptCount: 1,
          guestRebootRequested: true,
          guestRebootComponents: ['drivers'],
          guestRebootRequestTime: '2026-03-30T18:20:00Z',
        },
        hardware: {
          guestOs: 'UBUNTU_64',
          instantCloneFrozen: false,
          version: 'VMX_20',
          upgradePolicy: 'AFTER_CLEAN_SHUTDOWN',
          upgradeVersion: 'VMX_21',
          upgradeStatus: 'PENDING',
          bootType: 'EFI',
          efiLegacyBoot: false,
          bootNetworkProtocol: 'IPV4',
          bootDelayMilliseconds: 5000,
          bootRetry: true,
          bootRetryDelayMilliseconds: 10000,
          enterSetupMode: false,
          bootDevices: [
            { type: 'DISK', disks: ['2000'] },
            { type: 'ETHERNET', nic: '4000' },
          ],
          cpuCoresPerSocket: 2,
          cpuHotAddEnabled: true,
          cpuHotRemoveEnabled: false,
          memoryHotAddEnabled: true,
          memoryHotAddIncrementMib: 256,
          memoryHotAddLimitMib: 16384,
        },
        snapshotTree: [
          {
            snapshot: 'snapshot-101',
            name: 'baseline',
            createdAt: '2026-03-30T18:10:00Z',
            state: 'poweredOn',
            quiesced: true,
            children: [
              {
                snapshot: 'snapshot-103',
                name: 'post-upgrade',
                current: true,
                quiesced: false,
              },
            ],
          },
        ],
      };

      expect(vmware.overallStatus).toBe('yellow');
      expect(vmware.clusterName).toBe('Production Cluster');
      expect(vmware.clusterHaEnabled).toBe(true);
      expect(vmware.clusterDrsEnabled).toBe(false);
      expect(vmware.runtimeHostName).toBe('esxi-01.lab.local');
      expect(vmware.datastoreNames).toEqual(['primary-vmfs']);
      expect(vmware.networkType).toBe('STANDARD_PORTGROUP');
      expect(vmware.networkHostNames).toEqual(['esxi-01.lab.local']);
      expect(vmware.networkVmNames).toEqual(['app-01']);
      expect(vmware.guestIpAddresses).toEqual(['10.0.0.21']);
      expect(vmware.activeAlarmCount).toBe(2);
      expect(vmware.recentTaskSummary).toBe('Clone VM task finished');
      expect(vmware.snapshotCount).toBe(3);
      expect(vmware.currentSnapshotId).toBe('snapshot-103');
      expect(vmware.networkAdapters?.[0]?.networkName).toBe('VM Network');
      expect(vmware.virtualDisks?.[0]?.vmdkFile).toBe('[primary-vmfs] app-01/app-01.vmdk');
      expect(vmware.virtualDisks?.[0]?.capacityBytes).toBe(107374182400);
      expect(vmware.tools?.runState).toBe('RUNNING');
      expect(vmware.tools?.versionStatus).toBe('CURRENT');
      expect(vmware.tools?.version).toBe('12.4.0');
      expect(vmware.tools?.guestRebootComponents).toEqual(['drivers']);
      expect(vmware.hardware?.version).toBe('VMX_20');
      expect(vmware.hardware?.upgradeStatus).toBe('PENDING');
      expect(vmware.hardware?.bootDevices?.[1]?.nic).toBe('4000');
      expect(vmware.hardware?.memoryHotAddLimitMib).toBe(16384);
      expect(vmware.snapshotTree?.[0]?.children?.[0]?.current).toBe(true);
    });
  });

  describe('getPreferredResourceDisplayName', () => {
    it('returns displayName when set', () => {
      const resource = createResource({ name: 'machine-1', displayName: 'Production Server' });
      expect(getPreferredResourceDisplayName(resource)).toBe('Production Server');
    });

    it('returns name when displayName is empty', () => {
      const resource = createResource({ name: 'machine-1', displayName: '' });
      expect(getPreferredResourceDisplayName(resource)).toBe('machine-1');
    });

    it('returns name when displayName is undefined', () => {
      const resource = createResource({ name: 'machine-1' });
      // Force displayName to be falsy
      (resource as any).displayName = undefined;
      expect(getPreferredResourceDisplayName(resource)).toBe('machine-1');
    });

    it('returns the safe label for governed resources', () => {
      const resource = createResource({
        name: 'secret-vm-1',
        displayName: 'secret-vm-1',
        policy: {
          sensitivity: 'restricted',
          routing: { scope: 'local-only', redact: ['hostname'] },
        },
        aiSafeSummary: 'Production VM',
      } as Partial<Resource>);

      expect(getPreferredResourceDisplayName(resource)).toBe('Production VM');
    });

    it('condenses generated governed summaries into concise UI labels', () => {
      const resource = createResource({
        name: 'pbs-secret',
        displayName: 'PBS Secret',
        type: 'pbs',
        policy: {
          sensitivity: 'sensitive',
          routing: { scope: 'local-first', redact: ['hostname', 'platform-id'] },
        },
        aiSafeSummary:
          'backup server resource; status online; sources pbs; 1 child resources; redacted for cloud summary',
      } as Partial<Resource>);

      expect(getPreferredResourceDisplayName(resource)).toBe('backup server (online)');
    });

    it('falls back to the redacted policy label when the safe summary is missing', () => {
      const resource = createResource({
        name: 'secret-vm-1',
        displayName: 'secret-vm-1',
        policy: {
          sensitivity: 'restricted',
          routing: { scope: 'local-only', redact: ['hostname'] },
        },
      } as Partial<Resource>);

      expect(getPreferredResourceDisplayName(resource)).toBe('redacted by policy');
    });
  });

  describe('getCpuPercent', () => {
    it('returns current CPU value when available', () => {
      const resource = createResource({ cpu: { current: 75.5 } });
      expect(getCpuPercent(resource)).toBe(75.5);
    });

    it('returns 0 when cpu is undefined', () => {
      const resource = createResource({});
      expect(getCpuPercent(resource)).toBe(0);
    });

    it('returns 0 when cpu.current is undefined', () => {
      const resource = createResource({ cpu: {} as any });
      expect(getCpuPercent(resource)).toBe(0);
    });
  });

  describe('getMemoryPercent', () => {
    it('calculates percentage from used/total when available', () => {
      const resource = createResource({
        memory: { current: 0, total: 1000, used: 250 },
      });
      expect(getMemoryPercent(resource)).toBe(25);
    });

    it('returns current when used/total not available', () => {
      const resource = createResource({
        memory: { current: 45.5 },
      });
      expect(getMemoryPercent(resource)).toBe(45.5);
    });

    it('returns 0 when memory is undefined', () => {
      const resource = createResource({});
      expect(getMemoryPercent(resource)).toBe(0);
    });

    it('handles zero total gracefully', () => {
      const resource = createResource({
        memory: { current: 50, total: 0, used: 0 },
      });
      // When total is 0 (falsy), it should fall back to current
      expect(getMemoryPercent(resource)).toBe(50);
    });
  });

  describe('getDiskPercent', () => {
    it('calculates percentage from used/total when available', () => {
      const resource = createResource({
        disk: { current: 0, total: 1000000000, used: 500000000 },
      });
      expect(getDiskPercent(resource)).toBe(50);
    });

    it('returns current when used/total not available', () => {
      const resource = createResource({
        disk: { current: 80.2 },
      });
      expect(getDiskPercent(resource)).toBe(80.2);
    });

    it('returns 0 when disk is undefined', () => {
      const resource = createResource({});
      expect(getDiskPercent(resource)).toBe(0);
    });
  });
});

describe('Resource Interface', () => {
  it('supports canonical incident timeline change kinds', () => {
    const kinds: ResourceChangeKind[] = [
      'alert_fired',
      'alert_acknowledged',
      'alert_unacknowledged',
      'alert_resolved',
      'command_executed',
      'runbook_executed',
    ];

    expect(kinds).toEqual([
      'alert_fired',
      'alert_acknowledged',
      'alert_unacknowledged',
      'alert_resolved',
      'command_executed',
      'runbook_executed',
    ]);
  });

  it('allows all valid resource types', () => {
    const types: ResourceType[] = [
      'agent',
      'docker-host',
      'k8s-cluster',
      'k8s-node',
      'vm',
      'system-container',
      'app-container',
      'oci-container',
      'pod',
      'jail',
      'docker-service',
      'k8s-deployment',
      'k8s-service',
      'storage',
      'datastore',
      'pool',
      'dataset',
      'pbs',
      'pmg',
    ];

    types.forEach((type) => {
      const resource = createResource({ type });
      expect(resource.type).toBe(type);
    });
  });

  it('supports hierarchy with parentId and clusterId', () => {
    const vm = createResource({
      type: 'vm',
      parentId: 'node-1',
      clusterId: 'pve-cluster-1',
    });

    expect(vm.parentId).toBe('node-1');
    expect(vm.clusterId).toBe('pve-cluster-1');
  });

  it('supports tags and labels', () => {
    const resource = createResource({
      tags: ['production', 'web'],
      labels: { env: 'prod', role: 'frontend' },
    });

    expect(resource.tags).toEqual(['production', 'web']);
    expect(resource.labels).toEqual({ env: 'prod', role: 'frontend' });
  });

  it('supports alerts array', () => {
    const resource = createResource({
      alerts: [
        {
          id: 'alert-1',
          type: 'cpu',
          level: 'warning',
          message: 'High CPU usage',
          value: 85,
          threshold: 80,
          startTime: Date.now(),
        },
      ],
    });

    expect(resource.alerts).toHaveLength(1);
    expect(resource.alerts![0].type).toBe('cpu');
  });

  it('supports identity for deduplication', () => {
    const resource = createResource({
      identity: {
        hostname: 'server-1',
        machineId: 'abc-123',
        ips: ['192.168.1.10', '10.0.0.5'],
      },
    });

    expect(resource.identity?.hostname).toBe('server-1');
    expect(resource.identity?.machineId).toBe('abc-123');
    expect(resource.identity?.ips).toHaveLength(2);
  });

  it('supports platformData for type-specific data', () => {
    const dockerContainer = createResource({
      type: 'app-container',
      platformData: {
        image: 'nginx:latest',
        health: 'healthy',
        restartCount: 0,
        ports: [{ hostPort: 8080, containerPort: 80 }],
      },
    });

    expect((dockerContainer.platformData as any).image).toBe('nginx:latest');
  });

  it('supports policy metadata and aiSafeSummary', () => {
    const resource = createResource({
      policy: {
        sensitivity: 'restricted',
        routing: {
          scope: 'local-only',
          redact: ['hostname', 'ip-address', 'platform-id', 'alias'],
        },
      },
      aiSafeSummary: 'virtual machine resource; status running; local-only context',
    });

    expect(resource.policy?.sensitivity).toBe('restricted');
    expect(resource.policy?.routing.scope).toBe('local-only');
    expect(resource.policy?.routing.redact).toContain('hostname');
    expect(resource.aiSafeSummary).toContain('local-only context');
  });
});

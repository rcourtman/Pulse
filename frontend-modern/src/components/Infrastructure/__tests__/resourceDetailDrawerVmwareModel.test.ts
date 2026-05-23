import { describe, expect, it } from 'vitest';
import {
  buildVMwareDetailSections,
  buildVMwareDetailsSummary,
} from '@/components/Infrastructure/resourceDetailDrawerVmwareModel';
import type { ResourceVMwareMeta } from '@/types/resource';

describe('resourceDetailDrawerVmwareModel', () => {
  it('surfaces vCenter cluster service state in placement context', () => {
    const vmware: ResourceVMwareMeta = {
      datacenterName: 'Lab DC',
      clusterName: 'Compute Cluster',
      clusterHaEnabled: true,
      clusterDrsEnabled: false,
      computeResourceName: 'Compute Cluster',
      resourcePoolName: 'Production',
      runtimeHostName: 'esxi-01.lab.local',
      datastoreNames: ['shared-vsan'],
    };

    const placement = buildVMwareDetailSections('vm', vmware).find(
      (section) => section.id === 'placement',
    );

    expect(placement?.rows).toEqual([
      { label: 'Datacenter', value: 'Lab DC' },
      { label: 'Cluster', value: 'Compute Cluster' },
      { label: 'Cluster services', value: 'HA enabled · DRS disabled' },
      { label: 'Compute resource', value: 'Compute Cluster' },
      { label: 'Resource pool', value: 'Production' },
      { label: 'Runtime host', value: 'esxi-01.lab.local' },
      { label: 'Datastores', value: 'shared-vsan' },
    ]);
  });

  it('surfaces vCenter network resources as read-only topology context', () => {
    const vmware: ResourceVMwareMeta = {
      connectionName: 'Lab VC',
      entityType: 'network',
      overallStatus: 'yellow',
      networkType: 'DISTRIBUTED_PORTGROUP',
      datacenterName: 'Primary DC',
      folderName: 'Networks',
      networkHostNames: ['esxi-01.lab.local', 'esxi-02.lab.local'],
      networkVmNames: ['warehouse-api-01', 'etl-batch-01'],
      activeAlarmCount: 1,
      activeAlarmSummary: 'Network uplink redundancy (yellow)',
    };

    expect(buildVMwareDetailsSummary('network', vmware)).toBe(
      'Lab VC · Read-only vCenter context · 2 hosts · 2 VMs · 1 alarm',
    );

    const sections = buildVMwareDetailSections('network', vmware);
    expect(sections.find((section) => section.id === 'state')?.rows).toContainEqual({
      label: 'Network type',
      value: 'Distributed Portgroup',
    });
    expect(sections.find((section) => section.id === 'network')?.rows).toEqual([
      { label: 'Hosts', value: 'esxi-01.lab.local, esxi-02.lab.local' },
      { label: 'VMs', value: 'warehouse-api-01, etl-batch-01' },
    ]);
  });

  it('surfaces vSphere snapshot trees as read-only VM detail context', () => {
    const vmware: ResourceVMwareMeta = {
      connectionName: 'Lab VC',
      networkAdapters: [
        {
          nic: '4000',
          label: 'Network adapter 1',
          type: 'VMXNET3',
          macType: 'GENERATED',
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
          vmdkFile: '[nvme-primary] app-01/app-01.vmdk',
          datastoreName: 'nvme-primary',
          capacityBytes: 107374182400,
        },
      ],
      tools: {
        autoUpdateSupported: true,
        installAttemptCount: 1,
        versionNumber: 12352,
        version: '12.4.0',
        upgradePolicy: 'MANUAL',
        versionStatus: 'CURRENT',
        installType: 'OPEN_VM_TOOLS',
        runState: 'RUNNING',
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
      cpuCount: 4,
      memorySizeMib: 8192,
      snapshotTree: [
        {
          snapshot: 'snapshot-201',
          name: 'pre-upgrade',
          createdAt: '2026-03-28T18:15:00Z',
          state: 'poweredOn',
          quiesced: true,
          children: [
            {
              snapshot: 'snapshot-202',
              name: 'post-migration-checkpoint',
              createdAt: '2026-03-29T18:15:00Z',
              state: 'poweredOn',
              current: true,
              quiesced: false,
            },
          ],
        },
      ],
    };

    expect(buildVMwareDetailsSummary('vm', vmware)).toBe(
      'Lab VC · Read-only vCenter context · 2 snapshots · 1 vNIC · 1 disk · Hardware pending · Tools reboot requested',
    );

    const hardware = buildVMwareDetailSections('vm', vmware).find(
      (section) => section.id === 'hardware',
    );

    // Hardware section is deliberately pruned to operator-actionable rows.
    // Capability toggles (CPU/memory hot-add) and boot configuration (EFI
    // legacy boot, boot delay, boot retry, boot order, setup mode default)
    // stay in the raw API payload; the drawer only surfaces them when
    // they're in an attention state (Upgrade status pending,
    // Instant clone frozen, Enter setup mode toggled).
    expect(hardware?.rows).toEqual([
      { label: 'Guest OS', value: 'Ubuntu 64' },
      { label: 'Hardware version', value: 'VMX 20' },
      { label: 'CPU topology', value: '4 vCPU · 2 cores/socket' },
      { label: 'Memory size', value: '8 GB' },
      { label: 'Upgrade status', value: 'Pending', tone: 'warning' },
    ]);

    const tools = buildVMwareDetailSections('vm', vmware).find((section) => section.id === 'tools');

    // Tools section is pruned to operator-actionable rows: Run state,
    // Version status (only when not current), Version, Guest reboot (only
    // when requested), and Last install error. Install metadata stays in
    // the raw payload.
    expect(tools?.rows).toEqual([
      { label: 'Run state', value: 'Running', tone: 'default' },
      { label: 'Version', value: '12.4.0' },
      { label: 'Guest reboot', value: 'Requested', tone: 'warning' },
    ]);

    const disks = buildVMwareDetailSections('vm', vmware).find((section) => section.id === 'disks');

    expect(disks?.rows).toEqual([
      {
        label: 'Hard disk 1',
        value: 'SCSI 0:1 · 100 GB · nvme-primary · VMDK_FILE · [nvme-primary] app-01/app-01.vmdk',
        tone: 'default',
      },
    ]);

    const network = buildVMwareDetailSections('vm', vmware).find(
      (section) => section.id === 'network',
    );

    expect(network?.rows).toEqual([
      {
        label: 'Network adapter 1',
        value:
          'VMXNET3 · VM Network · 00:50:56:aa:bb:cc · CONNECTED · starts connected · guest control',
        tone: 'default',
      },
    ]);

    const snapshots = buildVMwareDetailSections('vm', vmware).find(
      (section) => section.id === 'snapshots',
    );

    expect(snapshots?.rows).toEqual([
      {
        label: 'pre-upgrade',
        value: 'poweredOn · 2026-03-28 18:15 UTC · quiesced',
        tone: 'default',
      },
      {
        label: '- post-migration-checkpoint',
        value: 'current · poweredOn · 2026-03-29 18:15 UTC · not quiesced',
        tone: 'accent',
      },
    ]);
  });
});

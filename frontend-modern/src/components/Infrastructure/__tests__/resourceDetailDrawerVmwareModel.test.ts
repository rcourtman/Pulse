import { describe, expect, it } from 'vitest';
import {
  buildVMwareDetailSections,
  buildVMwareDetailsSummary,
} from '@/components/Infrastructure/resourceDetailDrawerVmwareModel';
import type { ResourceVMwareMeta } from '@/types/resource';

describe('resourceDetailDrawerVmwareModel', () => {
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

    expect(hardware?.rows).toEqual([
      { label: 'Guest OS', value: 'Ubuntu 64' },
      { label: 'Hardware version', value: 'VMX 20' },
      { label: 'Upgrade status', value: 'Pending', tone: 'warning' },
      { label: 'Upgrade policy', value: 'After Clean Shutdown' },
      { label: 'Upgrade target', value: 'VMX 21' },
      { label: 'Instant clone frozen', value: 'No', tone: 'default' },
      { label: 'CPU topology', value: '4 vCPU · 2 cores/socket' },
      { label: 'CPU hot-add', value: 'Yes' },
      { label: 'CPU hot-remove', value: 'No' },
      { label: 'Memory size', value: '8 GB' },
      { label: 'Memory hot-add', value: 'Yes' },
      { label: 'Memory hot-add increment', value: '256 MB' },
      { label: 'Memory hot-add limit', value: '16 GB' },
      { label: 'Boot type', value: 'EFI' },
      { label: 'EFI legacy boot', value: 'No' },
      { label: 'Boot network protocol', value: 'IPv4' },
      { label: 'Boot delay', value: '5000 ms' },
      { label: 'Boot retry', value: 'Yes' },
      { label: 'Boot retry delay', value: '10000 ms' },
      { label: 'Enter setup mode', value: 'No', tone: 'default' },
      { label: 'Boot order', value: 'Disk 2000 -> Ethernet 4000' },
    ]);

    const tools = buildVMwareDetailSections('vm', vmware).find((section) => section.id === 'tools');

    expect(tools?.rows).toEqual([
      { label: 'Run state', value: 'Running', tone: 'default' },
      { label: 'Version status', value: 'Current', tone: 'default' },
      { label: 'Version', value: '12.4.0' },
      { label: 'Install type', value: 'Open VM Tools' },
      { label: 'Upgrade policy', value: 'Manual' },
      { label: 'Auto update supported', value: 'Yes' },
      { label: 'Install attempts', value: '1' },
      { label: 'Guest reboot', value: 'Requested', tone: 'warning' },
      { label: 'Reboot components', value: 'drivers', tone: 'warning' },
      { label: 'Reboot requested at', value: '2026-03-30 18:20 UTC', tone: 'warning' },
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

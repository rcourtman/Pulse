import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, waitFor, within } from '@solidjs/testing-library';

import type { Resource } from '@/types/resource';
import {
  ResourceDetailDrawer,
  getSpecializedTabAvailabilityMessage,
} from '@/components/Infrastructure/ResourceDetailDrawer';

const wsState = vi.hoisted(() => ({ pmg: [] as any[] }));
const reconnectSpy = vi.hoisted(() => vi.fn());

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    state: wsState,
    connected: () => true,
    initialDataReceived: () => true,
    reconnecting: () => false,
    reconnect: reconnectSpy,
  }),
  useDarkMode: () => () => false,
}));

vi.mock('@/components/Discovery/DiscoveryTab', () => ({
  DiscoveryTab: () => <div data-testid="discovery-tab" />,
}));

vi.mock('@/api/resources', () => ({
  ResourceAPI: {
    getFacetBundle: vi.fn().mockResolvedValue({
      capabilities: [],
      relationships: [],
      recentChanges: [],
    }),
  },
}));

vi.mock('@/api/ai', () => ({
  AIAPI: {
    getResourceIntelligence: vi.fn().mockResolvedValue({
      resource_id: 'resource-1',
      health: {
        score: 92,
        grade: 'A',
        trend: 'stable',
        factors: [],
        prediction: 'Stable',
      },
      recent_changes: [
        {
          id: 'change-1',
          observedAt: '2026-03-01T00:00:00Z',
          resourceId: 'resource-1',
          kind: 'config_update',
          sourceType: 'pulse_diff',
          confidence: 'high',
          reason: 'Updated canonical config',
        },
      ],
      dependencies: ['storage-1'],
      dependents: ['vm-child'],
      correlations: [
        {
          source_id: 'storage-1',
          source_name: 'Storage 1',
          source_type: 'storage',
          target_id: 'resource-1',
          target_name: 'Host 1',
          target_type: 'vm',
          event_pattern: 'disk_full -> restart',
          occurrences: 2,
          avg_delay: 125000000000,
          confidence: 0.875,
          last_seen: '2026-03-01T00:15:00Z',
          description: 'Disk pressure often precedes restarts',
        },
      ],
      note_count: 3,
    }),
  },
}));

class ResizeObserverMock {
  constructor(_callback: ResizeObserverCallback) {}
  observe() {}
  unobserve() {}
  disconnect() {}
}

if (typeof globalThis.ResizeObserver === 'undefined') {
  vi.stubGlobal('ResizeObserver', ResizeObserverMock);
}

const baseResource = (overrides: Partial<Resource>): Resource => ({
  id: 'resource-1',
  type: 'agent',
  name: 'host-1',
  displayName: 'host-1',
  platformId: 'host-1',
  platformType: 'agent',
  sourceType: 'hybrid',
  status: 'online',
  lastSeen: Date.now(),
  platformData: { sources: ['agent', 'proxmox'] },
  ...overrides,
});

describe('ResourceDetailDrawer runtime and identity cards', () => {
  it('renders a unified summary shell without repeating healthy source status', () => {
    const resource = baseResource({
      platformData: {
        sources: ['agent', 'proxmox'],
        sourceStatus: {
          agent: { status: 'online', lastSeen: new Date().toISOString() },
          proxmox: { status: 'online', lastSeen: new Date().toISOString() },
        },
      },
    });

    const { container, getByTestId, getByText, queryByRole } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(() => getByText('Summary')).toThrow();
    expect(getByText('Current state')).toBeInTheDocument();
    expect(() => getByText('Sources')).toThrow();
    expect(queryByRole('link', { name: 'Open related workloads for host-1' })).toBeNull();
    expect(() => getByText('Mode')).toThrow();
    expect(() => getByText('Hybrid')).toThrow();
    expect(() => getByText('Platform ID')).toThrow();
    expect(
      getByTestId('resource-current-state-section').querySelector('.border-dashed'),
    ).toBeNull();
    expect(queryByRole('button', { name: 'Show details' })).toBeNull();
    expect(() => getByText('Runtime')).toThrow();
    expect(container.querySelector('.text-\\[11px\\].text-muted.truncate')).toBeNull();
    const headerBadges = getByTestId('resource-header-badges');
    expect(within(headerBadges).getAllByText('Agent')).toHaveLength(1);
    expect(within(headerBadges).getByText('PVE')).toBeInTheDocument();
  });

  it('explains degraded host storage posture in the current state card', () => {
    const resource = baseResource({
      status: 'degraded',
      agent: {
        storagePostureSummary: 'Unraid array is running without parity protection',
        rebuildSummary: 'Unraid array is running check',
      },
    });

    const { getByText } = render(() => <ResourceDetailDrawer resource={resource} />);

    expect(getByText('No parity')).toBeInTheDocument();
    expect(getByText('Reason')).toBeInTheDocument();
    expect(getByText('Unraid array is running without parity protection')).toBeInTheDocument();
    expect(getByText('Unraid array is running check')).toBeInTheDocument();
  });

  it('uses canonical system identity in the drawer header for Unraid agent hosts', () => {
    const resource = baseResource({
      platformType: 'agent',
      sourceType: 'hybrid',
      sources: ['agent', 'docker'],
      platformData: {
        sources: ['kubernetes'],
        agent: {
          platform: 'linux',
          osName: 'Unraid OS 7.2.2',
          osVersion: '7.2.2',
        },
      },
    });

    const { getByTestId, queryByText } = render(() => <ResourceDetailDrawer resource={resource} />);
    const headerBadges = getByTestId('resource-header-badges');

    expect(within(headerBadges).getByText('Unraid 7.2.2')).toBeInTheDocument();
    expect(queryByText('K8s')).toBeNull();
  });

  it('keeps discovery as secondary overview context instead of a peer tab', async () => {
    const { getByRole, getByText, getByTestId, queryByRole, queryByTestId, queryByText } = render(
      () => <ResourceDetailDrawer resource={baseResource({})} />,
    );

    expect(queryByRole('button', { name: 'Analysis' })).toBeNull();
    expect(getByText('Access')).toBeInTheDocument();
    expect(queryByText('Analysis')).toBeNull();
    expect(queryByText('Host analysis via host-1')).toBeNull();
    expect(
      queryByText('Supporting metadata only. The web interface path above stays primary.'),
    ).toBeNull();
    expect(queryByTestId('discovery-tab')).toBeNull();
    expect(
      getByTestId('resource-access-section').querySelector(
        '.mt-3.rounded.border.border-border.bg-surface.p-2\\.5',
      ),
    ).toBeNull();

    expect(getByRole('button', { name: 'Show access' })).toBeInTheDocument();

    fireEvent.click(getByRole('button', { name: 'Show access' }));
    expect(getByText('Analysis')).toBeInTheDocument();
    expect(getByRole('button', { name: 'Open analysis' })).toBeInTheDocument();

    fireEvent.click(getByRole('button', { name: 'Open analysis' }));

    await waitFor(() => {
      expect(queryByTestId('discovery-tab')).toBeInTheDocument();
    });
  });

  it('uses terse availability notices for specialized tabs', () => {
    expect(getSpecializedTabAvailabilityMessage('mail')).toBe('PMG resources only.');
    expect(getSpecializedTabAvailabilityMessage('namespaces')).toBe('Kubernetes clusters only.');
    expect(getSpecializedTabAvailabilityMessage('deployments')).toBe('Kubernetes clusters only.');
    expect(getSpecializedTabAvailabilityMessage('swarm')).toBe('Docker runtimes with Swarm only.');
  });

  it('keeps host detail cards behind a secondary overview disclosure', async () => {
    const resource = baseResource({
      platformData: {
        sources: ['agent'],
        agent: {
          agentId: 'agent-1',
          hostname: 'host-1',
          platform: 'linux',
          osName: 'Ubuntu',
          osVersion: '24.04',
          kernelVersion: '6.8.0',
          architecture: 'x86_64',
          uptimeSeconds: 7200,
          cpuCount: 8,
          agentVersion: '1.2.3',
          memory: { total: 16 * 1024 * 1024 * 1024 },
          networkInterfaces: [
            {
              name: 'eth0',
              mac: '00:11:22:33:44:55',
              addresses: ['192.0.2.10'],
            },
          ],
          disks: [
            {
              mountpoint: '/',
              total: 100 * 1024 * 1024 * 1024,
              used: 50 * 1024 * 1024 * 1024,
            },
          ],
        },
      },
    });

    const { getByRole, getByText, getByTestId, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('Host')).toBeInTheDocument();
    expect(getByText('System, Hardware, Network, and Disks')).toBeInTheDocument();
    expect(queryByText('Hardware')).toBeNull();
    expect(queryByText('Network')).toBeNull();

    fireEvent.click(getByRole('button', { name: 'Show host' }));

    await waitFor(() => {
      expect(getByText('Hardware')).toBeInTheDocument();
    });
    expect(getByTestId('resource-secondary-sections').classList.contains('space-y-3')).toBe(true);
    expect(getByTestId('resource-support-sections').classList.contains('flex')).toBe(true);
    expect(getByTestId('resource-support-sections').classList.contains('flex-wrap')).toBe(true);
    expect(
      Array.from(getByTestId('resource-support-sections').children).map((node) =>
        node.getAttribute('data-testid'),
      ),
    ).toEqual(['resource-access-section', 'resource-host-details-section']);
    expect(
      getByTestId('resource-host-details-section').querySelector('.mt-3.flex.flex-wrap'),
    ).toBeTruthy();
    expect(
      getByTestId('resource-host-details-section')
        .querySelector('.mt-3.flex.flex-wrap')
        ?.classList.contains('[&>*]:min-w-[220px]'),
    ).toBe(true);
    expect(getByText('Network')).toBeInTheDocument();
    expect(getByText('Disks')).toBeInTheDocument();
    expect(getByText('eth0')).toBeInTheDocument();
  });

  it('surfaces VMware read-only placement and signal context on the shared drawer path', async () => {
    const resource = baseResource({
      type: 'vm',
      name: 'app-01',
      displayName: 'App 01',
      platformId: 'vc-1:vm:vm-201',
      platformType: 'vmware-vsphere',
      sourceType: 'api',
      platformData: {
        sources: ['vmware'],
        vmware: {
          connectionName: 'Lab VC',
          vcenterHost: 'vc.lab.local',
          entityType: 'VirtualMachine',
          overallStatus: 'green',
          powerState: 'poweredOn',
          datacenterName: 'Lab DC',
          clusterName: 'Compute Cluster',
          clusterHaEnabled: true,
          clusterDrsEnabled: false,
          resourcePoolName: 'Production',
          runtimeHostName: 'esxi-01.lab.local',
          datastoreNames: ['shared-vsan'],
          guestOsFamily: 'ubuntu64Guest',
          guestHostname: 'app-01.lab.local',
          guestIpAddresses: ['192.0.2.50'],
          networkAdapters: [
            {
              nic: '4000',
              label: 'Network adapter 1',
              type: 'VMXNET3',
              macAddress: '00:50:56:aa:bb:cc',
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
              vmdkFile: '[shared-vsan] app-01/app-01.vmdk',
              datastoreName: 'shared-vsan',
              capacityBytes: 107374182400,
            },
          ],
          tools: {
            runState: 'RUNNING',
            versionStatus: 'CURRENT',
            version: '12.4.0',
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
          cpuCount: 4,
          memorySizeMib: 8192,
          activeAlarmCount: 1,
          activeAlarmSummary: 'Host fan degraded',
          recentTaskCount: 1,
          recentTaskSummary: 'Create snapshot (success)',
          snapshotCount: 2,
          currentSnapshotId: 'snapshot-202',
          snapshotTree: [
            {
              snapshot: 'snapshot-201',
              name: 'pre-upgrade',
              description: 'Before application upgrade',
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
        },
      },
      vmware: {
        connectionName: 'Lab VC',
        vcenterHost: 'vc.lab.local',
        entityType: 'VirtualMachine',
        overallStatus: 'green',
        powerState: 'poweredOn',
        datacenterName: 'Lab DC',
        clusterName: 'Compute Cluster',
        clusterHaEnabled: true,
        clusterDrsEnabled: false,
        resourcePoolName: 'Production',
        runtimeHostName: 'esxi-01.lab.local',
        datastoreNames: ['shared-vsan'],
        guestOsFamily: 'ubuntu64Guest',
        guestHostname: 'app-01.lab.local',
        guestIpAddresses: ['192.0.2.50'],
        networkAdapters: [
          {
            nic: '4000',
            label: 'Network adapter 1',
            type: 'VMXNET3',
            macAddress: '00:50:56:aa:bb:cc',
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
            vmdkFile: '[shared-vsan] app-01/app-01.vmdk',
            datastoreName: 'shared-vsan',
            capacityBytes: 107374182400,
          },
        ],
        tools: {
          runState: 'RUNNING',
          versionStatus: 'CURRENT',
          version: '12.4.0',
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
        cpuCount: 4,
        memorySizeMib: 8192,
        activeAlarmCount: 1,
        activeAlarmSummary: 'Host fan degraded',
        recentTaskCount: 1,
        recentTaskSummary: 'Create snapshot (success)',
        snapshotCount: 2,
        currentSnapshotId: 'snapshot-202',
        snapshotTree: [
          {
            snapshot: 'snapshot-201',
            name: 'pre-upgrade',
            description: 'Before application upgrade',
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
      },
    });

    const { getByRole, getByText, getByTestId, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByTestId('resource-vmware-details-section')).toBeInTheDocument();
    expect(
      getByText(
        'Lab VC · Read-only vCenter context · 2 snapshots · 1 vNIC · 1 disk · Hardware pending · Tools reboot requested · 1 alarm · 1 task',
      ),
    ).toBeInTheDocument();
    expect(queryByText('Compute Cluster')).toBeNull();
    expect(queryByText('Create snapshot (success)')).toBeNull();

    fireEvent.click(getByRole('button', { name: 'Show vSphere' }));

    await waitFor(() => {
      expect(getByText('Placement')).toBeInTheDocument();
    });

    const section = getByTestId('resource-vmware-details-section');
    expect(within(section).getByText('State')).toBeInTheDocument();
    expect(within(section).getByText('Placement')).toBeInTheDocument();
    expect(within(section).getByText('Guest')).toBeInTheDocument();
    expect(within(section).getByText('Virtual hardware')).toBeInTheDocument();
    expect(within(section).getByText('VMware Tools')).toBeInTheDocument();
    expect(within(section).getByText('Virtual disks')).toBeInTheDocument();
    expect(within(section).getByText('Network')).toBeInTheDocument();
    expect(within(section).getByText('Signals')).toBeInTheDocument();
    expect(within(section).getByText('Snapshot tree')).toBeInTheDocument();
    expect(within(section).getByText('vc.lab.local')).toBeInTheDocument();
    expect(within(section).getByText('Compute Cluster')).toBeInTheDocument();
    expect(within(section).getByText('Cluster services')).toBeInTheDocument();
    expect(within(section).getByText('HA enabled · DRS disabled')).toBeInTheDocument();
    expect(within(section).getByText('esxi-01.lab.local')).toBeInTheDocument();
    expect(within(section).getByText('ubuntu64Guest')).toBeInTheDocument();
    expect(within(section).getByText('Hardware version')).toBeInTheDocument();
    expect(within(section).getByText('VMX 20')).toBeInTheDocument();
    expect(within(section).getByText('Upgrade status')).toBeInTheDocument();
    expect(within(section).getByText('Pending')).toBeInTheDocument();
    expect(within(section).getByText('CPU topology')).toBeInTheDocument();
    expect(within(section).getByText('4 vCPU · 2 cores/socket')).toBeInTheDocument();
    expect(within(section).getByText('Boot order')).toBeInTheDocument();
    expect(within(section).getByText('Disk 2000 -> Ethernet 4000')).toBeInTheDocument();
    expect(within(section).getByText('Run state')).toBeInTheDocument();
    expect(within(section).getByText('Running')).toBeInTheDocument();
    expect(within(section).getByText('Version status')).toBeInTheDocument();
    expect(within(section).getByText('Current')).toBeInTheDocument();
    expect(within(section).getByText('Open VM Tools')).toBeInTheDocument();
    expect(within(section).getByText('Guest reboot')).toBeInTheDocument();
    expect(within(section).getByText('Requested')).toBeInTheDocument();
    expect(within(section).getByText('Hard disk 1')).toBeInTheDocument();
    expect(within(section).getByText(/SCSI 0:1 · 100 GB · shared-vsan/)).toBeInTheDocument();
    expect(within(section).getByText('Network adapter 1')).toBeInTheDocument();
    expect(within(section).getByText(/VMXNET3 · VM Network/)).toBeInTheDocument();
    expect(within(section).getByText(/Create snapshot \(success\)/)).toBeInTheDocument();
    expect(within(section).getByText(/Host fan degraded/)).toBeInTheDocument();
    expect(within(section).getByText('2 snapshots')).toBeInTheDocument();
    expect(within(section).getByText('pre-upgrade')).toBeInTheDocument();
    expect(within(section).getByText('- post-migration-checkpoint')).toBeInTheDocument();
    expect(within(section).getByText(/current · poweredOn/)).toBeInTheDocument();
  });

  it('keeps source provenance in the header when no source health issue is present', () => {
    const resource = baseResource({
      sourceType: 'api',
      platformData: {
        sources: ['pmg'],
      },
    });

    const { getByTestId, queryByText } = render(() => <ResourceDetailDrawer resource={resource} />);

    expect(queryByText('Sources')).toBeNull();
    expect(within(getByTestId('resource-header-badges')).getByText('PMG')).toBeInTheDocument();
    expect(queryByText('Mode')).toBeNull();
    expect(queryByText('Platform ID')).toBeNull();
    expect(queryByText('Details')).toBeNull();
    expect(queryByText('Show details')).toBeNull();
  });

  it('shows the sources row when canonical source health is degraded', () => {
    const resource = baseResource({
      platformData: {
        sources: ['agent', 'proxmox'],
        sourceStatus: {
          agent: { status: 'online', lastSeen: new Date().toISOString() },
          proxmox: { status: 'degraded', lastSeen: new Date().toISOString() },
        },
      },
    });

    const { getByText } = render(() => <ResourceDetailDrawer resource={resource} />);

    expect(getByText('Sources')).toBeInTheDocument();
    expect(getByText('1/2 degraded')).toBeInTheDocument();
  });

  it('keeps current state free of source mode rows', () => {
    const resource = baseResource({
      sourceType: null as unknown as Resource['sourceType'],
      platformType: null as unknown as Resource['platformType'],
      platformData: {
        sources: [],
      },
    });

    const { getByText, queryByText } = render(() => <ResourceDetailDrawer resource={resource} />);

    expect(getByText('Current state')).toBeInTheDocument();
    expect(queryByText('Mode')).toBeNull();
    expect(queryByText('Unknown')).toBeNull();
  });

  it('shows identity aliases and fallback message when identity metadata is sparse', () => {
    const resource = baseResource({
      id: 'pmg-main',
      type: 'pmg',
      name: 'pmg-main',
      displayName: 'PMG Main',
      platformId: 'pmg-main',
      platformType: 'proxmox-pmg',
      sourceType: 'api',
      identity: {
        hostname: 'pmg.local',
      },
      platformData: {
        sources: ['pmg'],
        pmg: {
          instanceId: 'pmg-main',
        },
      },
    });

    const { getByText, getAllByText, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('Identity')).toBeInTheDocument();
    expect(getByText('Aliases')).toBeInTheDocument();
    expect(getAllByText('pmg-main').length).toBeGreaterThan(0);
    expect(queryByText('No identity metadata yet.')).toBeNull();

    const sparse = baseResource({
      id: 'host-min',
      name: 'host-min',
      displayName: 'Host Min',
      platformId: 'host-min',
      sourceType: 'agent',
      identity: undefined,
      parentId: undefined,
      clusterId: undefined,
      discoveryTarget: undefined,
      tags: [],
      platformData: { sources: ['agent'] },
    });

    const sparseRender = render(() => <ResourceDetailDrawer resource={sparse} />);
    expect(sparseRender.getByText('No identity metadata yet.')).toBeInTheDocument();
  });

  it('moves the primary identity into the identity card instead of the header subtitle', () => {
    const resource = baseResource({
      displayName: 'delly',
      name: 'delly',
      platformId: 'delly',
      canonicalIdentity: {
        displayName: 'delly',
        hostname: 'delly',
        primaryId: 'node:homelab-delly',
        aliases: ['node:homelab-delly', 'delly'],
      },
      identity: {
        hostname: 'delly',
      },
    });

    const { container, getByText } = render(() => <ResourceDetailDrawer resource={resource} />);

    expect(container.querySelector('.text-\\[11px\\].text-muted.truncate')).toBeNull();
    expect(getByText('Primary ID')).toBeInTheDocument();
    expect(getByText('Primary ID').parentElement?.textContent).toContain('node:homelab-delly');
  });

  it('shows canonical metrics target identity for docker-backed host resources', () => {
    const resource = baseResource({
      id: 'hash-docker-resource',
      type: 'docker-host',
      name: 'Tower',
      displayName: 'Tower',
      platformId: 'tower',
      platformType: 'docker',
      sourceType: 'agent',
      identity: {
        hostname: 'tower.local',
      },
      metricsTarget: {
        resourceType: 'docker-host',
        resourceId: 'docker-host-1',
      },
      platformData: {
        sources: ['docker', 'agent'],
        docker: {
          hostSourceId: 'docker-host-1',
          hostname: 'tower.local',
        },
      },
    });

    const { container, getByText, getAllByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('Metrics Target')).toBeInTheDocument();
    expect(getAllByText('docker-host:docker-host-1').length).toBeGreaterThan(1);
    expect(getByText('Aliases')).toBeInTheDocument();
    expect(getAllByText('docker-host-1').length).toBeGreaterThan(0);
    expect(container.querySelector('.text-\\[11px\\].text-muted.truncate')).toBeNull();
    expect(getByText('Primary ID').parentElement?.textContent).toContain(
      'docker-host:docker-host-1',
    );
  });

  it('renders aliases inline when few exist and collapses when many exist', () => {
    const inlineResource = baseResource({
      type: 'agent',
      platformData: {
        sources: ['agent'],
        agent: {
          agentId: 'agent-inline-1',
          hostname: 'inline-host.local',
        },
      },
      identity: undefined,
      parentId: undefined,
      clusterId: undefined,
      discoveryTarget: undefined,
      tags: [],
    });

    const inlineRender = render(() => <ResourceDetailDrawer resource={inlineResource} />);
    expect(inlineRender.getByText('Aliases')).toBeInTheDocument();
    expect(inlineRender.container.querySelector('details')).toBeNull();
    expect(inlineRender.getByText('agent-inline-1')).toBeInTheDocument();
    expect(inlineRender.getAllByText('inline-host.local').length).toBeGreaterThan(0);

    const overflowResource = baseResource({
      type: 'agent',
      platformData: {
        sources: ['agent', 'proxmox'],
        agent: {
          agentId: 'agent-overflow-1',
          hostname: 'overflow-host.local',
        },
        proxmox: {
          nodeName: 'overflow-node-1',
        },
      },
      discoveryTarget: {
        resourceType: 'agent',
        agentId: 'overflow-host-id',
        resourceId: 'overflow-resource-id',
      },
      identity: {
        hostname: 'overflow-identity-host',
      },
      tags: [],
    });

    const overflowRender = render(() => <ResourceDetailDrawer resource={overflowResource} />);
    expect(overflowRender.getByText('Aliases')).toBeInTheDocument();
    expect(overflowRender.container.querySelector('details')).toBeTruthy();
  });

  it('surfaces policy governance details and the safe summary', () => {
    const resource = baseResource({
      id: 'sensitive-resource-1',
      name: 'sensitive-host',
      displayName: 'Sensitive Host',
      platformId: 'platform-1',
      policy: {
        sensitivity: 'restricted',
        routing: {
          scope: 'local-only',
          redact: ['hostname', 'ip-address', 'alias'],
        },
      },
      aiSafeSummary: 'restricted host summary safe for remote provider consumption',
    });

    const { getAllByText, getByRole, getByText, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('Context')).toBeInTheDocument();
    expect(queryByText('Governance')).toBeNull();
    expect(queryByText('Safe Summary')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Show context' }));
    expect(getByText('Governance')).toBeInTheDocument();
    expect(getByText('Redactions')).toBeInTheDocument();
    expect(getByText('Safe Summary')).toBeInTheDocument();
    expect(getAllByText('Restricted').length).toBeGreaterThan(0);
    expect(getAllByText('Local Only').length).toBeGreaterThan(0);
    expect(getByText('Hostname')).toBeInTheDocument();
    expect(getByText('IP Address')).toBeInTheDocument();
    expect(
      getAllByText('restricted host summary safe for remote provider consumption').length,
    ).toBeGreaterThan(0);
    expect(getByText('Sensitive Host')).toBeInTheDocument();
    expect(queryByText('sensitive-host')).toBeNull();
  });

  it('uses the governed policy helper when aiSafeSummary is absent', () => {
    const resource = baseResource({
      id: 'governed-resource-1',
      name: 'governed-host',
      displayName: 'Governed Host',
      policy: {
        sensitivity: 'restricted',
        routing: {
          scope: 'local-only',
          redact: ['hostname'],
        },
      },
    });

    const { getAllByText, getByRole, getByText, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('Context')).toBeInTheDocument();
    expect(queryByText('Safe Summary')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Show context' }));
    expect(getByText('Safe Summary')).toBeInTheDocument();
    expect(getAllByText('redacted by policy').length).toBeGreaterThan(0);
  });

  it('surfaces canonical analysis context for the resource overview', async () => {
    const resource = baseResource({});
    const { getByRole, getByTestId, getByText, queryByRole, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    await waitFor(() => {
      expect(getByText('Context')).toBeInTheDocument();
    });

    expect(queryByText('Analysis')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Show context' }));
    await waitFor(() => {
      expect(
        within(getByTestId('resource-investigation-context')).getByText('Analysis'),
      ).toBeInTheDocument();
    });
    const contextSection = getByTestId('resource-investigation-context');
    expect(contextSection.querySelector('table')).toBeTruthy();
    expect(contextSection.querySelector('tbody')).toBeTruthy();
    expect(getByText('Health')).toBeInTheDocument();
    expect(getByText('A · 92/100')).toBeInTheDocument();
    expect(getByText('Trend')).toBeInTheDocument();
    expect(getByText('stable')).toBeInTheDocument();
    expect(getByText('Notes')).toBeInTheDocument();
    expect(getByText('3')).toBeInTheDocument();
    // Correlations now render inline inside the expanded context panel
    // rather than behind a separate Show/Hide correlations toggle, so they
    // are visible as soon as Show context is clicked.
    await waitFor(() => {
      expect(getByText('Storage 1')).toBeInTheDocument();
    });
    // Cross-jump to /infrastructure?resource=... retired with the legacy
    // surface; dependency / dependent labels render as plain text now.
    expect(
      queryByRole('link', {
        name: 'Open dependency resource storage-1 in Infrastructure',
      }),
    ).toBeNull();
    expect(
      queryByRole('link', {
        name: 'Open dependent resource vm-child in Infrastructure',
      }),
    ).toBeNull();
    expect(getByText('Storage 1')).toBeInTheDocument();
    expect(getByText('Host 1')).toBeInTheDocument();
    expect(getByText('Disk Full → Restart')).toBeInTheDocument();
    expect(getByText(/2 occurrences · avg delay 2m · 88% confidence/)).toBeInTheDocument();
    expect(getByText('Disk pressure often precedes restarts')).toBeInTheDocument();
    expect(getByText('Latest canonical change')).toBeInTheDocument();
    const latestChangeItem = getByText('Config update: Updated canonical config').closest('li');
    expect(latestChangeItem).not.toBeNull();
    expect(getByText('Updated canonical config')).toBeInTheDocument();
    expect(latestChangeItem).toHaveTextContent(/just now|m ago|h ago|d ago/);
  });
});

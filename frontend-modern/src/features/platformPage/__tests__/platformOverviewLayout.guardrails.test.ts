import { describe, expect, it } from 'vitest';
import agentsMachinesTableSource from '@/features/standalone/AgentsMachinesTable.tsx?raw';
import agentMachineTableModelSource from '@/features/standalone/agentMachineTableModel.ts?raw';
import standalonePageModelSource from '@/features/standalone/standalonePageModel.ts?raw';
import standalonePageSurfaceSource from '@/features/standalone/StandalonePageSurface.tsx?raw';
import dockerAlertsTableSource from '@/features/docker/DockerAlertsTable.tsx?raw';
import dockerConfigsTableSource from '@/features/docker/DockerConfigsTable.tsx?raw';
import dockerContainersTableSource from '@/features/docker/DockerContainersTable.tsx?raw';
import dockerHostsTableSource from '@/features/docker/DockerHostsTable.tsx?raw';
import dockerImagesTableSource from '@/features/docker/DockerImagesTable.tsx?raw';
import dockerNetworksTableSource from '@/features/docker/DockerNetworksTable.tsx?raw';
import dockerPageSurfaceSource from '@/features/docker/DockerPageSurface.tsx?raw';
import dockerSecretsTableSource from '@/features/docker/DockerSecretsTable.tsx?raw';
import dockerServicesTableSource from '@/features/docker/DockerServicesTable.tsx?raw';
import dockerStorageUsageTableSource from '@/features/docker/DockerStorageUsageTable.tsx?raw';
import dockerSwarmNodesTableSource from '@/features/docker/DockerSwarmNodesTable.tsx?raw';
import dockerTasksTableSource from '@/features/docker/DockerTasksTable.tsx?raw';
import dockerVolumesTableSource from '@/features/docker/DockerVolumesTable.tsx?raw';
import kubernetesAlertsTableSource from '@/features/kubernetes/KubernetesAlertsTable.tsx?raw';
import kubernetesAutoscalingTableSource from '@/features/kubernetes/KubernetesAutoscalingTable.tsx?raw';
import kubernetesClustersTableSource from '@/features/kubernetes/KubernetesClustersTable.tsx?raw';
import kubernetesConfigTableSource from '@/features/kubernetes/KubernetesConfigTable.tsx?raw';
import kubernetesControllersTableSource from '@/features/kubernetes/KubernetesControllersTable.tsx?raw';
import kubernetesDeploymentsTableSource from '@/features/kubernetes/KubernetesDeploymentsTable.tsx?raw';
import kubernetesEventsTableSource from '@/features/kubernetes/KubernetesEventsTable.tsx?raw';
import kubernetesNetworkingTableSource from '@/features/kubernetes/KubernetesNetworkingTable.tsx?raw';
import kubernetesNodesTableSource from '@/features/kubernetes/KubernetesNodesTable.tsx?raw';
import kubernetesPageSurfaceSource from '@/features/kubernetes/KubernetesPageSurface.tsx?raw';
import kubernetesPodsTableSource from '@/features/kubernetes/KubernetesPodsTable.tsx?raw';
import kubernetesPolicyTableSource from '@/features/kubernetes/KubernetesPolicyTable.tsx?raw';
import kubernetesServicesTableSource from '@/features/kubernetes/KubernetesServicesTable.tsx?raw';
import kubernetesStorageTableSource from '@/features/kubernetes/KubernetesStorageTable.tsx?raw';
// The backups page owns the scope controls and delegates dense table rendering
// to current per-view components; the shared-primitive guardrail follows those
// current table owners.
import proxmoxBackupServersTableSource from '@/features/proxmox/ProxmoxBackupServersTable.tsx?raw';
import proxmoxCoverageTableSource from '@/features/proxmox/ProxmoxCoverageTable.tsx?raw';
import proxmoxRecoverableTableSource from '@/features/proxmox/ProxmoxRecoverableTable.tsx?raw';
import proxmoxCephClusterDrawerSource from '@/features/proxmox/ProxmoxCephClusterDrawer.tsx?raw';
import proxmoxCephTableSource from '@/features/proxmox/ProxmoxCephTable.tsx?raw';
import proxmoxMailGatewayDrawerSource from '@/features/proxmox/ProxmoxMailGatewayDrawer.tsx?raw';
import proxmoxMailGatewayTableSource from '@/features/proxmox/ProxmoxMailGatewayTable.tsx?raw';
import proxmoxNodesTableSource from '@/features/proxmox/ProxmoxNodesTable.tsx?raw';
import proxmoxPageSurfaceSource from '@/features/proxmox/ProxmoxPageSurface.tsx?raw';
import proxmoxReplicationTableSource from '@/features/proxmox/ProxmoxReplicationTable.tsx?raw';
import sharedPlatformPageSource from '@/features/platformPage/sharedPlatformPage.tsx?raw';
import truenasAlertsTableSource from '@/features/truenas/TrueNASAlertsTable.tsx?raw';
import truenasAppsTableSource from '@/features/truenas/TrueNASAppsTable.tsx?raw';
import truenasNetworkSharesTableSource from '@/features/truenas/TrueNASNetworkSharesTable.tsx?raw';
import truenasPageSurfaceSource from '@/features/truenas/TrueNASPageSurface.tsx?raw';
import truenasProtectionTableSource from '@/features/truenas/TrueNASProtectionTable.tsx?raw';
import truenasServicesTableSource from '@/features/truenas/TrueNASServicesTable.tsx?raw';
import truenasStorageTopologyTableSource from '@/features/truenas/TrueNASStorageTopologyTable.tsx?raw';
import truenasSystemsTableSource from '@/features/truenas/TrueNASSystemsTable.tsx?raw';
import truenasVirtualMachinesTableSource from '@/features/truenas/TrueNASVirtualMachinesTable.tsx?raw';
import vmwarePageSurfaceSource from '@/features/vmware/VmwarePageSurface.tsx?raw';
import vsphereActivityTableSource from '@/features/vmware/VsphereActivityTable.tsx?raw';
import vsphereAlertsTableSource from '@/features/vmware/VsphereAlertsTable.tsx?raw';
import vsphereDatastoresTableSource from '@/features/vmware/VsphereDatastoresTable.tsx?raw';
import vsphereHostsTableSource from '@/features/vmware/VsphereHostsTable.tsx?raw';
import vsphereNetworksTableSource from '@/features/vmware/VsphereNetworksTable.tsx?raw';

const platformTableSources = [
  agentsMachinesTableSource,
  dockerAlertsTableSource,
  dockerContainersTableSource,
  dockerHostsTableSource,
  dockerImagesTableSource,
  dockerVolumesTableSource,
  dockerNetworksTableSource,
  dockerStorageUsageTableSource,
  dockerSwarmNodesTableSource,
  dockerServicesTableSource,
  dockerTasksTableSource,
  dockerSecretsTableSource,
  dockerConfigsTableSource,
  kubernetesAlertsTableSource,
  kubernetesAutoscalingTableSource,
  kubernetesClustersTableSource,
  kubernetesConfigTableSource,
  kubernetesControllersTableSource,
  kubernetesNodesTableSource,
  kubernetesPodsTableSource,
  kubernetesDeploymentsTableSource,
  kubernetesEventsTableSource,
  kubernetesNetworkingTableSource,
  kubernetesPolicyTableSource,
  kubernetesServicesTableSource,
  kubernetesStorageTableSource,
  truenasAlertsTableSource,
  truenasAppsTableSource,
  truenasNetworkSharesTableSource,
  truenasProtectionTableSource,
  truenasServicesTableSource,
  truenasStorageTopologyTableSource,
  truenasSystemsTableSource,
  truenasVirtualMachinesTableSource,
  vsphereActivityTableSource,
  vsphereAlertsTableSource,
  vsphereDatastoresTableSource,
  vsphereHostsTableSource,
  vsphereNetworksTableSource,
];

const platformShellTableSources = [
  ...platformTableSources,
  proxmoxNodesTableSource,
  proxmoxBackupServersTableSource,
  proxmoxCoverageTableSource,
  proxmoxRecoverableTableSource,
  proxmoxCephTableSource,
  proxmoxMailGatewayTableSource,
  proxmoxReplicationTableSource,
];

const platformToolbarTableSources = [
  agentsMachinesTableSource,
  dockerContainersTableSource,
  dockerHostsTableSource,
  dockerImagesTableSource,
  dockerVolumesTableSource,
  dockerNetworksTableSource,
  dockerSwarmNodesTableSource,
  dockerServicesTableSource,
  dockerTasksTableSource,
  dockerSecretsTableSource,
  dockerConfigsTableSource,
  kubernetesAutoscalingTableSource,
  kubernetesClustersTableSource,
  kubernetesConfigTableSource,
  kubernetesControllersTableSource,
  kubernetesNodesTableSource,
  kubernetesPodsTableSource,
  kubernetesDeploymentsTableSource,
  kubernetesEventsTableSource,
  kubernetesNetworkingTableSource,
  kubernetesPolicyTableSource,
  kubernetesServicesTableSource,
  kubernetesStorageTableSource,
  truenasSystemsTableSource,
  vsphereHostsTableSource,
];

const overviewSurfaceSources = [
  standalonePageSurfaceSource,
  proxmoxPageSurfaceSource,
  dockerPageSurfaceSource,
  kubernetesPageSurfaceSource,
  truenasPageSurfaceSource,
  vmwarePageSurfaceSource,
];

const proxmoxDetailTableSources = [
  proxmoxCephTableSource,
  proxmoxMailGatewayTableSource,
  proxmoxReplicationTableSource,
];

const proxmoxBespokeTableSources = [
  proxmoxCoverageTableSource,
  proxmoxRecoverableTableSource,
  proxmoxBackupServersTableSource,
  proxmoxCephTableSource,
  proxmoxMailGatewayTableSource,
  proxmoxReplicationTableSource,
];

const proxmoxInlineDetailTableSources = [
  proxmoxCephClusterDrawerSource,
  proxmoxMailGatewayDrawerSource,
];

describe('platform overview layout guardrails', () => {
  it('keeps platform inventory tables on the shared dense table styling contract', () => {
    expect(sharedPlatformPageSource).toContain('PLATFORM_TABLE_CARD_CLASS');
    expect(sharedPlatformPageSource).toContain('PLATFORM_TABLE_HEADER_ROW_CLASS');
    expect(sharedPlatformPageSource).toContain('PLATFORM_TABLE_BODY_CLASS');
    expect(sharedPlatformPageSource).toContain('getPlatformTableHeadClass');
    expect(sharedPlatformPageSource).toContain('getPlatformTableCellClass');
    expect(sharedPlatformPageSource).toContain('PlatformTableToolbar');
    expect(sharedPlatformPageSource).toContain('PlatformTableLoadingState');
    expect(sharedPlatformPageSource).toContain('PlatformTableShell');
    expect(sharedPlatformPageSource).toContain('createPlatformTableFilterState');
    expect(sharedPlatformPageSource).toContain('PLATFORM_HEALTH_FILTER_OPTIONS');

    for (const source of platformShellTableSources) {
      expect(source).toContain('PlatformTableShell');
      expect(source).toContain('getPlatformTableHeadClass');
      expect(source).toContain('getPlatformTableCellClass');
      expect(source).not.toContain('TableCard class={PLATFORM_TABLE_CARD_CLASS}');
      expect(source).not.toContain('TableCardHeader');
      expect(source).not.toContain('TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}');
      expect(source).not.toContain('TableBody class={PLATFORM_TABLE_BODY_CLASS}');
    }

    for (const source of platformToolbarTableSources) {
      expect(source).toContain('PlatformTableToolbar');
      expect(source).toContain('createPlatformTableFilterState');
      expect(source).toContain('PLATFORM_HEALTH_FILTER_OPTIONS');
      expect(source).not.toContain("from '@/components/shared/SearchInput'");
      expect(source).not.toContain("from '@/components/shared/FilterButtonGroup'");
      // Forbid a bespoke search signal (`const [search, setSearch] = createSignal`)
      // — tables must read search from createPlatformTableFilterState. The comma
      // keeps URL-backed scope state (`const [searchParams, ...] = useSearchParams`)
      // out of the net, since that is shared-FilterBar plumbing, not a rogue box.
      expect(source).not.toContain('const [search, ');
    }
  });

  it('keeps Proxmox detail tables on the shared platform table primitives', () => {
    for (const source of proxmoxBespokeTableSources) {
      expect(source).toContain('PlatformTableShell');
      expect(source).toContain('getPlatformTableHeadClassForKind');
      expect(source).toContain('getPlatformTableCellClassForKind');
      expect(source).not.toContain('TableCard class={PLATFORM_TABLE_CARD_CLASS}');
      expect(source).not.toContain('TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}');
      expect(source).not.toContain('TableBody class={PLATFORM_TABLE_BODY_CLASS}');
      expect(source).not.toContain('border-collapse text-xs');
      expect(source).not.toContain('bg-surface-alt text-muted border-b border-border');
    }

    for (const source of proxmoxDetailTableSources) {
      expect(source).toContain('PlatformTableToolbar');
      expect(source).not.toContain("from '@/components/shared/SearchInput'");
      expect(source).not.toContain("from '@/components/shared/FilterButtonGroup'");
    }
  });

  it('keeps Proxmox inline detail tables on shared platform table primitives', () => {
    for (const source of proxmoxInlineDetailTableSources) {
      expect(source).toContain("from '@/components/shared/Table'");
      expect(source).toContain('PLATFORM_TABLE_HEADER_ROW_CLASS');
      expect(source).toContain('PLATFORM_TABLE_BODY_CLASS');
      expect(source).toContain('getPlatformTableHeadClassForKind');
      expect(source).toContain('getPlatformTableCellClassForKind');
      expect(source).not.toContain('<table');
      expect(source).not.toContain('<thead');
      expect(source).not.toContain('<tbody');
      expect(source).not.toContain('divide-y divide-border-subtle');
    }
  });

  it('keeps Docker host optional Swarm column wide enough for its header', () => {
    expect(dockerHostsTableSource).toContain('Swarm role');
    expect(dockerHostsTableSource).toContain('md:w-[10%]');
  });

  it('keeps provider overview pages in the parent-table plus child-inventory stack', () => {
    for (const source of overviewSurfaceSources) {
      expect(source).toMatch(/<div[^>]*class="space-y-4"/);
      expect(source).toContain('<PlatformSectionTabs');
      expect(source).toContain('<PlatformTableLoadingState');
      expect(source).toContain('PlatformTableEmptyState');
      expect(source).toContain('PlatformErrorState');
    }

    expect(proxmoxPageSurfaceSource).toContain('<ProxmoxNodesTable');
    expect(proxmoxPageSurfaceSource).toContain('<WorkloadsSurface');
    expect(dockerPageSurfaceSource).toContain('<DockerHostsTable');
    expect(dockerPageSurfaceSource).toContain('<DockerContainersTable');
    expect(dockerPageSurfaceSource).toContain('<DockerImagesTable');
    expect(dockerPageSurfaceSource).toContain('<DockerVolumesTable');
    expect(dockerPageSurfaceSource).toContain('<DockerNetworksTable');
    expect(dockerPageSurfaceSource).not.toContain('<WorkloadsSurface');
    expect(dockerPageSurfaceSource).toContain('<DockerSwarmNodesTable');
    expect(dockerPageSurfaceSource).toContain('<DockerServicesTable');
    expect(dockerPageSurfaceSource).toContain('<DockerTasksTable');
    expect(dockerPageSurfaceSource).toContain('<DockerSecretsTable');
    expect(dockerPageSurfaceSource).toContain('<DockerConfigsTable');
    expect(kubernetesPageSurfaceSource).toContain('<KubernetesClustersTable');
    expect(kubernetesPageSurfaceSource).toContain('<KubernetesNodesTable');
    expect(kubernetesPageSurfaceSource).toContain('<KubernetesPodsTable');
    expect(kubernetesPageSurfaceSource).toContain('<KubernetesDeploymentsTable');
    expect(kubernetesPageSurfaceSource).toContain('<KubernetesControllersTable');
    expect(kubernetesPageSurfaceSource).toContain('<KubernetesServicesTable');
    expect(kubernetesPageSurfaceSource).toContain('<KubernetesStorageTable');
    expect(kubernetesPageSurfaceSource).toContain('<KubernetesNetworkingTable');
    expect(kubernetesPageSurfaceSource).toContain('<KubernetesConfigTable');
    expect(kubernetesPageSurfaceSource).toContain('<KubernetesPolicyTable');
    expect(kubernetesPageSurfaceSource).toContain('<KubernetesAutoscalingTable');
    expect(kubernetesPageSurfaceSource).toContain('<KubernetesEventsTable');
    expect(truenasPageSurfaceSource).toContain('<TrueNASSystemsTable');
    expect(truenasPageSurfaceSource).toContain('<TrueNASAlertsTable');
    expect(truenasPageSurfaceSource).toContain('<TrueNASVirtualMachinesTable');
    expect(truenasPageSurfaceSource).toContain('<TrueNASAppsTable');
    expect(truenasPageSurfaceSource).toContain('<TrueNASStorageTopologyTable');
    expect(truenasPageSurfaceSource).toContain('<TrueNASProtection');
    expect(truenasPageSurfaceSource).toContain('platform: TRUENAS_PLATFORM_FILTER');
    expect(truenasPageSurfaceSource).toContain('source=truenas,agent');
    expect(truenasPageSurfaceSource).not.toContain('forcedView="pools"');
    expect(truenasPageSurfaceSource).not.toContain('<RecoverySurface');
    expect(truenasPageSurfaceSource).not.toContain('<StorageSurface');
    expect(truenasPageSurfaceSource).not.toContain('<WorkloadsSurface');
    expect(truenasPageSurfaceSource).not.toContain('<TrueNASDisksTable');
    expect(truenasAppsTableSource).toContain('md:min-w-[960px]');
    expect(truenasAppsTableSource).not.toContain('Volumes');
    expect(truenasNetworkSharesTableSource).toContain('md:min-w-[960px]');
    expect(truenasNetworkSharesTableSource).not.toMatch(
      /getPlatformTableHeadClassForKind\('text'\)[\s\S]{0,200}?Dataset/,
    );
    expect(truenasProtectionTableSource).not.toContain('Signal');
    expect(truenasProtectionTableSource).toContain('md:min-w-[960px]');
    expect(truenasStorageTopologyTableSource).toContain('md:min-w-[960px]');
    expect(truenasVirtualMachinesTableSource).toContain('md:min-w-[960px]');
    expect(vmwarePageSurfaceSource).toContain('<VsphereHostsTable');
    expect(vmwarePageSurfaceSource).toContain('<VsphereAlertsTable');
    expect(vmwarePageSurfaceSource).toContain("activeTab() === 'health'");
    expect(vmwarePageSurfaceSource).not.toContain('<VsphereVirtualMachinesTable');
    expect(vmwarePageSurfaceSource).toContain('<WorkloadsSurface');
    expect(vmwarePageSurfaceSource).toContain('forcedPlatform={VMWARE_PLATFORM_FILTER}');
    expect(vmwarePageSurfaceSource).toContain('forcedViewMode="vm"');
    expect(vmwarePageSurfaceSource).toContain('suppressTypeFilter');
    expect(vmwarePageSurfaceSource).toContain('<VsphereDatastoresTable');
    expect(vmwarePageSurfaceSource).toContain('<VsphereNetworksTable');
    expect(vmwarePageSurfaceSource).toContain('<VsphereActivityTable');
    expect(vmwarePageSurfaceSource).not.toContain('<StorageSurface');
    expect(vmwarePageSurfaceSource).not.toContain('forcedView="pools"');
    expect(proxmoxPageSurfaceSource).toContain('suppressNodeFilter');
    expect(standalonePageSurfaceSource).toContain('<AgentsMachinesTable');
    expect(standalonePageSurfaceSource).not.toContain('InfrastructureSummary');
    expect(standalonePageSurfaceSource).not.toContain('StickySummarySection');
    expect(standalonePageSurfaceSource).not.toContain('ChartVisibilityToggleButton');
    expect(standalonePageSurfaceSource).not.toContain('FilterBar');
    expect(standalonePageSurfaceSource).not.toContain('UnifiedResourceTable');
    expect(agentsMachinesTableSource).toContain('PlatformResourceDetailTableRow');
    expect(agentsMachinesTableSource).not.toContain('ResourceDetailDrawer');
    expect(standalonePageModelSource).not.toContain('infrastructureSelectors');
    expect(standalonePageModelSource).not.toContain('buildAgentsPageFilterModel');
    expect(standalonePageModelSource).not.toContain('buildStandalonePageFilterModel');
  });

  it('keeps TrueNAS overview inventory in tables instead of summary cards', () => {
    expect(truenasPageSurfaceSource).toContain('<TrueNASSystemsTable');
    expect(truenasPageSurfaceSource).not.toContain('TrueNASInventorySummary');
    expect(truenasPageSurfaceSource).not.toContain('data-truenas-summary-tile');
  });

  it('keeps secondary overview tables from rendering duplicate standalone toolbars', () => {
    for (const source of [
      dockerPageSurfaceSource,
      kubernetesPageSurfaceSource,
      truenasPageSurfaceSource,
      vmwarePageSurfaceSource,
    ]) {
      expect(source).toContain('showToolbar={false}');
    }
  });

  it('keeps platform overview pages from rendering a duplicate WorkloadsFilter', () => {
    // Proxmox and vSphere overview pages render their own page-level
    // WorkloadsFilter above the embedded WorkloadsSurface so a single
    // toolbar drives both the page's top table and the workloads table.
    // If the embedded surface also renders its own filter the page ends
    // up with two stacked toolbars wired to the same state (RC6 bug).
    const surfacesWithSharedToolbar: Array<[string, string]> = [
      ['ProxmoxPageSurface', proxmoxPageSurfaceSource],
      ['VmwarePageSurface', vmwarePageSurfaceSource],
    ];
    for (const [name, source] of surfacesWithSharedToolbar) {
      const filterCount = (source.match(/<WorkloadsFilter\b/g) ?? []).length;
      expect(filterCount, `${name} should render exactly one <WorkloadsFilter>`).toBe(1);
      expect(
        /<WorkloadsSurface\b[^>]*?suppressFilterToolbar/s.test(source),
        `${name} should pass suppressFilterToolbar to <WorkloadsSurface>`,
      ).toBe(true);
    }
  });

  it('keeps mobile host tables focused on useful operational columns', () => {
    // Assertions use the canonical kind-based helpers
    // (getPlatformTableHeadClassForKind('<kind>')) so the platform overview
    // tables keep aligned metric and numeric columns across providers.
    expect(dockerHostsTableSource).toMatch(
      /getPlatformTableHeadClassForKind\('name'\)[\s\S]{0,200}?Host/,
    );
    expect(dockerHostsTableSource).toMatch(
      /getPlatformTableHeadClassForKind\('metric-bar'\)[\s\S]{0,200}?CPU/,
    );
    expect(dockerHostsTableSource).toMatch(
      /getPlatformTableHeadClassForKind\('metric-bar'\)[\s\S]{0,200}?Memory/,
    );
    expect(dockerHostsTableSource).toMatch(
      /getPlatformTableHeadClassForKind\('metric-bar'\)[\s\S]{0,200}?Disk/,
    );

    expect(kubernetesClustersTableSource).toMatch(
      /getPlatformTableHeadClassForKind\('name'\)[\s\S]{0,200}?Cluster/,
    );
    expect(kubernetesClustersTableSource).toMatch(
      /getPlatformTableHeadClassForKind\('numeric-value'\)[\s\S]{0,200}?Nodes/,
    );
    expect(kubernetesNodesTableSource).toMatch(
      /getPlatformTableHeadClassForKind\('name'\)[\s\S]{0,200}?Node/,
    );
    expect(kubernetesNodesTableSource).toContain(
      '<span class="md:hidden">{compactCapacityLabel()}</span>',
    );

    expect(truenasSystemsTableSource).toMatch(
      /getPlatformTableHeadClassForKind\('name'\)[\s\S]{0,200}?System/,
    );
    expect(truenasSystemsTableSource).toContain(
      '<span class="md:hidden">{formatPercent(storagePercent())}</span>',
    );
    expect(vsphereHostsTableSource).toMatch(
      /getPlatformTableHeadClassForKind\('name'\)[\s\S]{0,200}?Host/,
    );
    expect(vsphereHostsTableSource).toMatch(
      /getPlatformTableHeadClassForKind\('numeric-value'\)[\s\S]{0,200}?VMs/,
    );
    expect(vsphereHostsTableSource).toContain('hidden md:table-cell');
    // AgentsMachinesTable uses a column-config pattern: kind helpers are
    // applied dynamically in the table render, with labels living in the
    // model. Assert against the model's column declarations.
    expect(agentsMachinesTableSource).toMatch(/getPlatformTableHeadClassForKind\(kind\(\)\)/);
    expect(agentMachineTableModelSource).toMatch(
      /id:\s*'machine'[\s\S]{0,80}?label:\s*'Machine'[\s\S]{0,80}?kind:\s*'name'/,
    );
    expect(agentMachineTableModelSource).toMatch(
      /id:\s*'cpu'[\s\S]{0,80}?label:\s*'CPU'[\s\S]{0,80}?kind:\s*'metric-bar'/,
    );
    expect(agentMachineTableModelSource).toMatch(
      /id:\s*'memory'[\s\S]{0,80}?label:\s*'Memory'[\s\S]{0,80}?kind:\s*'metric-bar'/,
    );
    expect(agentMachineTableModelSource).toMatch(
      /id:\s*'disk'[\s\S]{0,80}?label:\s*'Disk'[\s\S]{0,80}?kind:\s*'metric-bar'/,
    );
  });
});

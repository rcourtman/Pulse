import { describe, expect, it } from 'vitest';
import agentsMachinesTableSource from '@/features/agents/AgentsMachinesTable.tsx?raw';
import agentsPageSurfaceSource from '@/features/agents/AgentsPageSurface.tsx?raw';
import dockerHostsTableSource from '@/features/docker/DockerHostsTable.tsx?raw';
import dockerPageSurfaceSource from '@/features/docker/DockerPageSurface.tsx?raw';
import dockerServicesTableSource from '@/features/docker/DockerServicesTable.tsx?raw';
import kubernetesClustersTableSource from '@/features/kubernetes/KubernetesClustersTable.tsx?raw';
import kubernetesDeploymentsTableSource from '@/features/kubernetes/KubernetesDeploymentsTable.tsx?raw';
import kubernetesNodesTableSource from '@/features/kubernetes/KubernetesNodesTable.tsx?raw';
import kubernetesPageSurfaceSource from '@/features/kubernetes/KubernetesPageSurface.tsx?raw';
import proxmoxBackupsTableSource from '@/features/proxmox/ProxmoxBackupsTable.tsx?raw';
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
import truenasStorageTopologyTableSource from '@/features/truenas/TrueNASStorageTopologyTable.tsx?raw';
import truenasSystemsTableSource from '@/features/truenas/TrueNASSystemsTable.tsx?raw';
import truenasVirtualMachinesTableSource from '@/features/truenas/TrueNASVirtualMachinesTable.tsx?raw';
import vmwarePageSurfaceSource from '@/features/vmware/VmwarePageSurface.tsx?raw';
import vsphereActivityTableSource from '@/features/vmware/VsphereActivityTable.tsx?raw';
import vsphereAlertsTableSource from '@/features/vmware/VsphereAlertsTable.tsx?raw';
import vsphereDatastoresTableSource from '@/features/vmware/VsphereDatastoresTable.tsx?raw';
import vsphereHostsTableSource from '@/features/vmware/VsphereHostsTable.tsx?raw';

const platformTableSources = [
  agentsMachinesTableSource,
  proxmoxNodesTableSource,
  dockerHostsTableSource,
  dockerServicesTableSource,
  kubernetesClustersTableSource,
  kubernetesNodesTableSource,
  kubernetesDeploymentsTableSource,
  truenasAlertsTableSource,
  truenasAppsTableSource,
  truenasNetworkSharesTableSource,
  truenasProtectionTableSource,
  truenasStorageTopologyTableSource,
  truenasSystemsTableSource,
  truenasVirtualMachinesTableSource,
  vsphereActivityTableSource,
  vsphereAlertsTableSource,
  vsphereDatastoresTableSource,
  vsphereHostsTableSource,
];

const platformToolbarTableSources = [
  agentsMachinesTableSource,
  dockerHostsTableSource,
  dockerServicesTableSource,
  kubernetesClustersTableSource,
  kubernetesNodesTableSource,
  kubernetesDeploymentsTableSource,
  truenasSystemsTableSource,
  vsphereHostsTableSource,
];

const overviewSurfaceSources = [
  agentsPageSurfaceSource,
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
  proxmoxBackupsTableSource,
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
    expect(sharedPlatformPageSource).toContain('createPlatformTableFilterState');
    expect(sharedPlatformPageSource).toContain('PLATFORM_HEALTH_FILTER_OPTIONS');

    for (const source of platformTableSources) {
      expect(source).toContain('TableCard');
      expect(source).toContain('TableCardHeader');
      expect(source).toContain('PLATFORM_TABLE_CARD_CLASS');
      expect(source).toContain('PLATFORM_TABLE_HEADER_ROW_CLASS');
      expect(source).toContain('PLATFORM_TABLE_BODY_CLASS');
      expect(source).toContain('getPlatformTableHeadClass');
      expect(source).toContain('getPlatformTableCellClass');
    }

    for (const source of platformToolbarTableSources) {
      expect(source).toContain('PlatformTableToolbar');
      expect(source).toContain('createPlatformTableFilterState');
      expect(source).toContain('PLATFORM_HEALTH_FILTER_OPTIONS');
      expect(source).not.toContain("from '@/components/shared/SearchInput'");
      expect(source).not.toContain("from '@/components/shared/FilterButtonGroup'");
      expect(source).not.toContain('const [search');
    }
  });

  it('keeps Proxmox detail tables on the shared platform table primitives', () => {
    for (const source of proxmoxBespokeTableSources) {
      expect(source).toContain('TableCard');
      expect(source).toContain('PLATFORM_TABLE_CARD_CLASS');
      expect(source).toContain('PLATFORM_TABLE_HEADER_ROW_CLASS');
      expect(source).toContain('PLATFORM_TABLE_BODY_CLASS');
      expect(source).toContain('getPlatformTableHeadClassForKind');
      expect(source).toContain('getPlatformTableCellClassForKind');
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
    expect(dockerPageSurfaceSource).toContain('<WorkloadsSurface');
    expect(dockerPageSurfaceSource).toContain('<DockerServicesTable');
    expect(kubernetesPageSurfaceSource).toContain('<KubernetesClustersTable');
    expect(kubernetesPageSurfaceSource).toContain('<KubernetesNodesTable');
    expect(kubernetesPageSurfaceSource).toContain('<KubernetesDeploymentsTable');
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
    expect(vmwarePageSurfaceSource).toContain("forcedPlatform={VMWARE_PLATFORM_FILTER}");
    expect(vmwarePageSurfaceSource).toContain("forcedViewMode=\"vm\"");
    expect(vmwarePageSurfaceSource).toContain('<VsphereDatastoresTable');
    expect(vmwarePageSurfaceSource).toContain('<VsphereNetworksTable');
    expect(vmwarePageSurfaceSource).toContain('<VsphereActivityTable');
    expect(vmwarePageSurfaceSource).not.toContain('<StorageSurface');
    expect(vmwarePageSurfaceSource).not.toContain('forcedView="pools"');
    expect(proxmoxPageSurfaceSource).toContain('suppressNodeFilter');
    expect(agentsPageSurfaceSource).toContain('<AgentsMachinesTable');
    expect(agentsPageSurfaceSource).not.toContain('InfrastructureSummary');
    expect(agentsPageSurfaceSource).not.toContain('StickySummarySection');
    expect(agentsPageSurfaceSource).not.toContain('ChartVisibilityToggleButton');
    expect(agentsPageSurfaceSource).not.toContain('FilterBar');
    expect(agentsPageSurfaceSource).not.toContain('UnifiedResourceTable');
    expect(agentsMachinesTableSource).toContain('PlatformResourceDetailTableRow');
    expect(agentsMachinesTableSource).not.toContain('ResourceDetailDrawer');
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
    expect(agentsMachinesTableSource).toMatch(
      /getPlatformTableHeadClassForKind\('name'\)[\s\S]{0,200}?Machine/,
    );
    expect(agentsMachinesTableSource).toMatch(
      /getPlatformTableHeadClassForKind\('metric-bar'\)[\s\S]{0,200}?CPU/,
    );
    expect(agentsMachinesTableSource).toMatch(
      /getPlatformTableHeadClassForKind\('metric-bar'\)[\s\S]{0,200}?Memory/,
    );
    expect(agentsMachinesTableSource).toMatch(
      /getPlatformTableHeadClassForKind\('metric-bar'\)[\s\S]{0,200}?Disk/,
    );
  });
});

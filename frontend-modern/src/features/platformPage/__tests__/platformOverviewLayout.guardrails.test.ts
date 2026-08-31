import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
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
import platformSearchSuggestionsSource from '@/features/platformPage/platformSearchSuggestions.ts?raw';
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

const indexCssSource = readFileSync('src/index.css', 'utf8');

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
  it('keeps attention summaries canonical without a competing estate panel', () => {
    for (const source of [truenasProtectionTableSource, vsphereAlertsTableSource]) {
      expect(source).toContain('withPlatformStatusCounts');
      expect(source).not.toContain('PlatformAttentionSummary');
    }
    for (const source of overviewSurfaceSources) {
      expect(source).not.toContain('PlatformEstateOverview');
      expect(source).not.toContain('PlatformAttentionSummary');
    }
    expect(proxmoxPageSurfaceSource).toContain('inventoryStats={workloadsState.inventoryStats}');
    expect(vmwarePageSurfaceSource).toContain('inventoryStats={workloadsState.inventoryStats}');
  });

  it('keeps platform inventory tables on the shared dense table styling contract', () => {
    expect(sharedPlatformPageSource).toContain('PLATFORM_TABLE_CARD_CLASS');
    expect(sharedPlatformPageSource).toContain('PLATFORM_TABLE_HEADER_ROW_CLASS');
    expect(sharedPlatformPageSource).toContain('PLATFORM_TABLE_BODY_CLASS');
    expect(sharedPlatformPageSource).toContain('getPlatformTableHeadClass');
    expect(sharedPlatformPageSource).toContain('getPlatformTableCellClass');
    expect(sharedPlatformPageSource).toContain('getPlatformTableResponsiveMinWidthClass');
    expect(sharedPlatformPageSource).toContain('PLATFORM_TABLE_DEFAULT_RESPONSIVE_MIN_WIDTH_CLASS');
    expect(sharedPlatformPageSource).toContain('PlatformTableToolbar');
    expect(sharedPlatformPageSource).toContain('PlatformTableLoadingState');
    expect(sharedPlatformPageSource).toContain('PlatformTableShell');
    expect(sharedPlatformPageSource).toContain('createPlatformTableFilterState');
    expect(sharedPlatformPageSource).toContain('PLATFORM_HEALTH_FILTER_OPTIONS');

    for (const source of platformShellTableSources) {
      expect(source).toContain('PlatformTableShell');
      // Headers resolve their canonical kind-based class either directly
      // (getPlatformTableHeadClassForKind) or through the shared sortable
      // header component, which applies the same helper internally.
      expect(
        source.includes('getPlatformTableHeadClass') ||
          source.includes('PlatformSortableTableHead'),
        'table headers must use getPlatformTableHeadClassForKind or PlatformSortableTableHead',
      ).toBe(true);
      expect(source).toContain('getPlatformTableCellClass');
      expect(source).not.toContain('TableCard class={PLATFORM_TABLE_CARD_CLASS}');
      expect(source).not.toContain('TableCardHeader');
      expect(source).not.toContain('TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}');
      expect(source).not.toContain('TableBody class={PLATFORM_TABLE_BODY_CLASS}');
    }

    for (const source of platformToolbarTableSources) {
      expect(source).toContain('PlatformTableToolbar');
      expect(source).toContain('createPlatformTableFilterState');
      expect(source).toContain('searchSuggestions=');
      expect(source).toContain('PLATFORM_HEALTH_FILTER_OPTIONS');
      expect(source).not.toContain('ViewOptionsMenu');
      expect(source).not.toContain("from '@/components/shared/SearchInput'");
      expect(source).not.toContain("from '@/components/shared/FilterButtonGroup'");
      // Forbid a bespoke search signal (`const [search, setSearch] = createSignal`)
      // — tables must read search from createPlatformTableFilterState. The comma
      // keeps URL-backed scope state (`const [searchParams, ...] = useSearchParams`)
      // out of the net, since that is shared-FilterBar plumbing, not a rogue box.
      expect(source).not.toContain('const [search, ');
    }

    expect(sharedPlatformPageSource).toContain('searchSuggestions={tableState.searchSuggestions}');
    expect(platformSearchSuggestionsSource).toContain('buildPlatformResourceSearchSuggestions');
    expect(platformSearchSuggestionsSource).not.toContain('platformData');
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
      expect(source).toContain('PlatformDetailTable');
      expect(source).toContain('PlatformDetailTableHeader');
      expect(source).toContain('PlatformDetailTableBody');
      expect(source).toContain('getPlatformTableHeadClassForKind');
      expect(source).toContain('getPlatformTableCellClassForKind');
      expect(source).not.toMatch(
        /import \{[^}]*\bTable\b[^}]*\} from '@\/components\/shared\/Table'/,
      );
      expect(source).not.toMatch(
        /import \{[^}]*\bTableHeader\b[^}]*\} from '@\/components\/shared\/Table'/,
      );
      expect(source).not.toMatch(
        /import \{[^}]*\bTableBody\b[^}]*\} from '@\/components\/shared\/Table'/,
      );
      expect(source).not.toContain('PLATFORM_TABLE_HEADER_ROW_CLASS');
      expect(source).not.toContain('PLATFORM_TABLE_BODY_CLASS');
      expect(source).not.toContain('<table');
      expect(source).not.toContain('<thead');
      expect(source).not.toContain('<tbody');
      expect(source).not.toContain('divide-y divide-border-subtle');
    }
  });

  it('keeps Mail Gateway drawer tables prioritized for narrow inline details', () => {
    expect(proxmoxMailGatewayDrawerSource).toContain(
      '<PlatformDetailTable class="min-w-0 table-fixed text-xs">',
    );
    expect(sharedPlatformPageSource).toContain('phoneVerticalScrollOwner="page"');
    expect(proxmoxMailGatewayDrawerSource).toContain(
      'platform-table-mobile-w-10 platform-table-narrow-hidden md:w-[15%]',
    );
    expect(proxmoxMailGatewayDrawerSource).toContain(
      '<PlatformResponsiveTableLabel compact="Up" full="Uptime" />',
    );
    expect(proxmoxMailGatewayDrawerSource).toContain(
      '<PlatformResponsiveTableLabel compact="Ld" full="Load" />',
    );
    expect(proxmoxMailGatewayDrawerSource).toContain('platform-table-mobile-w-15 md:w-[30%]');
  });

  it('keeps Docker host optional Swarm column wide enough for its header', () => {
    expect(dockerHostsTableSource).toContain('Swarm role');
    expect(dockerHostsTableSource).toContain('md:w-[10%]');
  });

  it('keeps Docker phone tables dense with explicit shared identity sizing', () => {
    for (const source of [
      dockerAlertsTableSource,
      dockerConfigsTableSource,
      dockerHostsTableSource,
      dockerImagesTableSource,
      dockerVolumesTableSource,
      dockerNetworksTableSource,
      dockerStorageUsageTableSource,
      dockerSwarmNodesTableSource,
      dockerServicesTableSource,
      dockerTasksTableSource,
      dockerSecretsTableSource,
    ]) {
      expect(source).toContain('platform-table-mobile-w-30');
    }
    expect(dockerImagesTableSource).toMatch(
      /sortKey="size"[\s\S]{0,120}?class="platform-table-mobile-w-15 md:w-\[12%\]"/,
    );
    expect(dockerServicesTableSource).toMatch(
      /sortKey="mode"[\s\S]{0,120}?class="platform-table-phone-hidden md:w-\[8%\]"/,
    );
    expect(dockerServicesTableSource).toMatch(
      /sortKey="update"[\s\S]{0,120}?class="platform-table-mobile-w-15 w-\[15%\] md:w-\[12%\]"/,
    );
    expect(dockerTasksTableSource).toMatch(
      /sortKey="service"[\s\S]{0,120}?class="platform-table-mobile-w-15 md:w-\[18%\]"/,
    );
    expect(dockerTasksTableSource).toMatch(
      /sortKey="node"[\s\S]{0,120}?class="platform-table-mobile-w-15 md:w-\[16%\]"/,
    );
    expect(dockerSwarmNodesTableSource).toMatch(
      /sortKey="reachability"[\s\S]{0,120}?class="platform-table-mobile-w-15 w-\[15%\] md:w-\[14%\]"/,
    );
    expect(dockerSwarmNodesTableSource).toMatch(
      /sortKey="memory"[\s\S]{0,120}?class="platform-table-mobile-w-15 w-\[15%\] md:w-\[10%\]"/,
    );
  });

  it('keeps Kubernetes phone identity readable while exposing operational context', () => {
    expect(kubernetesClustersTableSource).toContain(
      '-my-3 inline-flex min-h-11 items-center truncate',
    );
    expect(kubernetesNodesTableSource).toMatch(
      /sortKey="roles"[\s\S]{0,160}?class="platform-table-phone-hidden md:w-\[10%\]"/,
    );
    expect(kubernetesNodesTableSource).toMatch(
      /sortKey="capacity"[\s\S]{0,120}?class="platform-table-mobile-w-10 md:w-\[14%\]"/,
    );
    expect(kubernetesDeploymentsTableSource).toMatch(
      /sortKey="namespace"[\s\S]{0,120}?class="platform-table-mobile-w-15 md:w-\[20%\]"/,
    );
    expect(kubernetesPodsTableSource).toMatch(
      /sortKey="scope"[\s\S]{0,120}?class="platform-table-mobile-w-15 md:w-\[13%\]"/,
    );
    expect(kubernetesPodsTableSource).toMatch(
      /sortKey="node"[\s\S]{0,120}?class="platform-table-phone-hidden md:w-\[13%\]"/,
    );
    expect(kubernetesServicesTableSource).toMatch(
      /sortKey="scope"[\s\S]{0,120}?class="platform-table-mobile-w-15 md:w-\[15%\]"/,
    );
    expect(kubernetesNetworkingTableSource).toMatch(
      /sortKey="scope"[\s\S]{0,120}?class="platform-table-mobile-w-20 md:w-\[14%\]"/,
    );
    expect(kubernetesStorageTableSource).toMatch(
      /sortKey="class"[\s\S]{0,120}?class="platform-table-mobile-w-20 md:w-\[10%\]"/,
    );
    expect(kubernetesConfigTableSource).toMatch(
      /sortKey="scope"[\s\S]{0,120}?class="platform-table-mobile-w-20 md:w-\[16%\]"/,
    );
    expect(kubernetesPolicyTableSource).toContain(
      "getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-15 md:w-[15%]",
    );
  });

  it('keeps phone-priority visibility container-led and symmetric across table rows', () => {
    expect(indexCssSource).toContain(':is(th, td).platform-table-phone-hidden');
    expect(indexCssSource).toContain(':is(th, td).platform-table-phone-only');

    for (const source of [
      dockerAlertsTableSource,
      kubernetesAlertsTableSource,
      truenasAlertsTableSource,
      vsphereAlertsTableSource,
    ]) {
      expect(source).toMatch(/TableHead[\s\S]{0,160}?platform-table-phone-hidden/);
      expect(source).toMatch(
        /TableCell[\s\S]{0,160}?getPlatformTableCellClassForKind\('badge'\)[\s\S]{0,80}?platform-table-phone-hidden/,
      );
    }

    expect(dockerVolumesTableSource).toMatch(
      /sortKey="scope"[\s\S]{0,120}?platform-table-phone-hidden/,
    );
    expect(dockerVolumesTableSource).toMatch(
      /getPlatformTableCellClassForKind\('text'\)[\s\S]{0,100}?platform-table-phone-hidden[\s\S]{0,140}?dockerTextValue\(resource\.docker\?\.scope\)/,
    );
    expect(kubernetesDeploymentsTableSource).toMatch(
      /sortKey="desired"[\s\S]{0,120}?platform-table-phone-hidden/,
    );
    expect(truenasVirtualMachinesTableSource).toMatch(
      /sortKey="state"[\s\S]{0,120}?platform-table-phone-hidden/,
    );
    expect(vsphereActivityTableSource).toMatch(
      /sortKey="state"[\s\S]{0,120}?platform-table-phone-hidden/,
    );
  });

  it('keeps remaining platform phone identity and actions usable', () => {
    expect(truenasStorageTopologyTableSource).toMatch(
      /sortKey="kind"[\s\S]{0,160}?platform-table-mobile-w-15 md:w-\[10%\]/,
    );
    expect(truenasStorageTopologyTableSource).toContain("return 'pl-6 sm:pl-11'");
    expect(truenasStorageTopologyTableSource).toMatch(
      /sortKey="resource"[\s\S]{0,120}?class="platform-table-mobile-w-30 md:w-\[32%\]"/,
    );
    expect(vsphereNetworksTableSource).toMatch(
      /sortKey="network"[\s\S]{0,120}?class="platform-table-mobile-w-30 md:w-\[24%\]"/,
    );
    expect(vsphereNetworksTableSource).toMatch(
      /sortKey="type"[\s\S]{0,120}?class="platform-table-mobile-w-15 md:w-\[13%\]"/,
    );
    expect(agentsMachinesTableSource).toContain('class="min-h-11 min-w-11 sm:min-h-0 sm:min-w-0"');
    expect(agentsMachinesTableSource).toContain(
      '-my-3 inline-flex min-h-11 max-w-full items-center',
    );
    expect(agentsMachinesTableSource).toContain('flex min-h-11 w-full items-center gap-2 rounded');
  });

  it('keeps provider overview pages in the parent-table plus child-inventory stack', () => {
    for (const source of overviewSurfaceSources) {
      expect(source).toMatch(/<div[^>]*class="[^"]*\bspace-y-4\b[^"]*"/);
      expect(source).toContain('<PlatformSectionTabs');
      expect(source).toContain('<PlatformTableLoadingState');
      expect(source).toContain('PlatformTableEmptyState');
      expect(source).toContain('PlatformErrorState');
    }

    expect(proxmoxPageSurfaceSource).toContain('<ProxmoxNodesTable');
    expect(proxmoxPageSurfaceSource).not.toContain('<ProxmoxBackupServersTable');
    expect(proxmoxPageSurfaceSource).toContain('<WorkloadsSurface');
    expect(proxmoxPageSurfaceSource).toContain(
      "const PROXMOX_WORKLOAD_EXCLUDED_TYPES = ['app-container'] as const",
    );
    expect(proxmoxPageSurfaceSource).toContain(
      'excludedWorkloadTypes: PROXMOX_WORKLOAD_EXCLUDED_TYPES',
    );
    expect(proxmoxPageSurfaceSource).toContain('showNestedExcludedWorkloads: true');
    expect(proxmoxPageSurfaceSource).toContain(
      'excludedWorkloadTypes={PROXMOX_WORKLOAD_EXCLUDED_TYPES}',
    );
    expect(proxmoxPageSurfaceSource).toContain('showNestedExcludedWorkloads');
    expect(dockerPageSurfaceSource).toContain('<DockerHostsTable');
    expect(dockerPageSurfaceSource).toContain('<DockerContainersTable');
    expect(dockerPageSurfaceSource).toContain('<DockerImagesTable');
    expect(dockerPageSurfaceSource).toContain('<DockerVolumesTable');
    expect(dockerPageSurfaceSource).toContain('<DockerNetworksTable');
    expect(dockerNetworksTableSource).toContain('DOCKER_NETWORK_COLUMN_WIDTH_CLASS');
    expect(dockerNetworksTableSource).toContain("attached: 'w-[25%]'");
    expect(dockerNetworksTableSource).toContain("driver: 'w-[15%]'");
    expect(dockerNetworksTableSource).toContain("attention: 'w-[15%]'");
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
    expect(truenasPageSurfaceSource).toContain(
      'source=truenas&type=agent,vm,app-container,network-share,storage,physical_disk',
    );
    expect(truenasPageSurfaceSource).not.toContain('forcedView="pools"');
    expect(truenasPageSurfaceSource).not.toContain('<RecoverySurface');
    expect(truenasPageSurfaceSource).not.toContain('<StorageSurface');
    expect(truenasPageSurfaceSource).not.toContain('<WorkloadsSurface');
    expect(truenasPageSurfaceSource).not.toContain('<TrueNASDisksTable');
    expect(truenasAppsTableSource).toContain('md:min-w-[960px]');
    expect(truenasAppsTableSource).toContain('truenas-app-name-column');
    expect(truenasAppsTableSource).toContain('truenas-app-name-value');
    expect(truenasAppsTableSource).toContain('truenas-app-containers-column');
    expect(truenasAppsTableSource).toContain('truenas-app-updates-column');
    expect(truenasAppsTableSource).toContain(
      '<PlatformResponsiveTableLabel compact="Upd." full="Updates" />',
    );
    expect(truenasAppsTableSource).toContain(
      '<PlatformResponsiveTableLabel compact="A" full="App" />',
    );
    expect(truenasAppsTableSource).toContain(
      '<PlatformResponsiveTableLabel compact="I" full="Image" />',
    );
    expect(kubernetesServicesTableSource).toContain('kubernetes-service-name-column');
    expect(kubernetesServicesTableSource).toContain('kubernetes-service-name-value');
    expect(kubernetesServicesTableSource).toContain('kubernetes-service-ports-column');
    expect(kubernetesNetworkingTableSource).toContain('kubernetes-network-name-column');
    expect(kubernetesNetworkingTableSource).toContain('kubernetes-network-name-value');
    expect(kubernetesNetworkingTableSource).toContain('kubernetes-network-ports-column');
    expect(truenasAppsTableSource).not.toContain('Volumes');
    expect(truenasNetworkSharesTableSource).toContain('md:min-w-[960px]');
    expect(truenasNetworkSharesTableSource).not.toMatch(
      /getPlatformTableHeadClassForKind\('text'\)[\s\S]{0,200}?Dataset/,
    );
    expect(truenasProtectionTableSource).not.toContain('Signal');
    expect(truenasProtectionTableSource).toContain('md:min-w-[960px]');
    expect(truenasStorageTopologyTableSource).toContain('md:min-w-[960px]');
    expect(truenasVirtualMachinesTableSource).toContain('md:min-w-[960px]');
    expect(truenasVirtualMachinesTableSource).toMatch(
      /class="hidden sm:table-cell md:w-\[19%\]"[\s\S]{0,120}?Flags/,
    );
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
    expect(vsphereNetworksTableSource).toMatch(
      /sortKey="vms"[\s\S]{0,120}?class="platform-table-mobile-w-15 md:w-\[7%\]"/,
    );
    expect(vmwarePageSurfaceSource).toContain('<VsphereActivityTable');
    expect(vmwarePageSurfaceSource).not.toContain('<StorageSurface');
    expect(vmwarePageSurfaceSource).not.toContain('forcedView="pools"');
    expect(proxmoxPageSurfaceSource).toContain('suppressNodeFilter');
    expect(proxmoxReplicationTableSource).toContain('REPLICATION_COLUMN_WIDTH_CLASS');
    expect(proxmoxReplicationTableSource).toContain("guest: 'w-[20%]'");
    expect(proxmoxReplicationTableSource).toContain("fails: 'w-[4%]'");
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
    // Assertions use the canonical kind-based helpers — either
    // getPlatformTableHeadClassForKind('<kind>') on a raw TableHead or the
    // kind="<kind>" prop on the shared PlatformSortableTableHead — so the
    // platform overview tables keep aligned metric and numeric columns
    // across providers.
    expect(dockerHostsTableSource).toMatch(/kind="name"[\s\S]{0,200}?Host/);
    expect(dockerHostsTableSource).toMatch(/kind="metric-bar"[\s\S]{0,200}?CPU/);
    expect(dockerHostsTableSource).toMatch(/kind="metric-bar"[\s\S]{0,300}?Memory/);
    expect(dockerHostsTableSource).toMatch(/kind="metric-bar"[\s\S]{0,200}?Disk/);

    expect(kubernetesClustersTableSource).toMatch(/kind="name"[\s\S]{0,200}?Cluster/);
    expect(kubernetesClustersTableSource).toMatch(
      /sortKey="nodes"[\s\S]{0,180}?<PlatformResponsiveTableLabel compact="Nds" full="Nodes" \/>/,
    );
    expect(kubernetesNodesTableSource).toMatch(/kind="name"[\s\S]{0,200}?Node/);
    expect(kubernetesNodesTableSource).toContain(
      '<span class="md:hidden">{compactCapacityLabel()}</span>',
    );

    expect(truenasSystemsTableSource).toMatch(/kind="name"[\s\S]{0,200}?System/);
    expect(truenasSystemsTableSource).toMatch(
      /<span class="md:hidden">\s*<PlatformTablePercentValue value={storagePercent\(\)} \/>\s*<\/span>/,
    );
    expect(vsphereHostsTableSource).toMatch(/kind="name"[\s\S]{0,200}?Host/);
    expect(vsphereHostsTableSource).toMatch(
      /sortKey="vms"[\s\S]{0,180}?class="platform-table-mobile-w-10 w-\[12%\] md:w-\[4%\]"[\s\S]{0,80}?VMs/,
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

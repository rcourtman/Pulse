import { describe, expect, it } from 'vitest';
import dockerHostsTableSource from '@/features/docker/DockerHostsTable.tsx?raw';
import dockerPageSurfaceSource from '@/features/docker/DockerPageSurface.tsx?raw';
import dockerServicesTableSource from '@/features/docker/DockerServicesTable.tsx?raw';
import kubernetesClustersTableSource from '@/features/kubernetes/KubernetesClustersTable.tsx?raw';
import kubernetesDeploymentsTableSource from '@/features/kubernetes/KubernetesDeploymentsTable.tsx?raw';
import kubernetesNodesTableSource from '@/features/kubernetes/KubernetesNodesTable.tsx?raw';
import kubernetesPageSurfaceSource from '@/features/kubernetes/KubernetesPageSurface.tsx?raw';
import proxmoxCephTableSource from '@/features/proxmox/ProxmoxCephTable.tsx?raw';
import proxmoxMailGatewayTableSource from '@/features/proxmox/ProxmoxMailGatewayTable.tsx?raw';
import proxmoxNodesTableSource from '@/features/proxmox/ProxmoxNodesTable.tsx?raw';
import proxmoxPageSurfaceSource from '@/features/proxmox/ProxmoxPageSurface.tsx?raw';
import proxmoxReplicationTableSource from '@/features/proxmox/ProxmoxReplicationTable.tsx?raw';
import sharedPlatformPageSource from '@/features/platformPage/sharedPlatformPage.tsx?raw';
import truenasPageSurfaceSource from '@/features/truenas/TrueNASPageSurface.tsx?raw';
import truenasSystemsTableSource from '@/features/truenas/TrueNASSystemsTable.tsx?raw';
import vmwarePageSurfaceSource from '@/features/vmware/VmwarePageSurface.tsx?raw';
import vsphereHostsTableSource from '@/features/vmware/VsphereHostsTable.tsx?raw';

const platformTableSources = [
  proxmoxNodesTableSource,
  dockerHostsTableSource,
  dockerServicesTableSource,
  kubernetesClustersTableSource,
  kubernetesNodesTableSource,
  kubernetesDeploymentsTableSource,
  truenasSystemsTableSource,
  vsphereHostsTableSource,
];

const platformToolbarTableSources = [
  dockerHostsTableSource,
  dockerServicesTableSource,
  kubernetesClustersTableSource,
  kubernetesNodesTableSource,
  kubernetesDeploymentsTableSource,
  truenasSystemsTableSource,
  vsphereHostsTableSource,
];

const overviewSurfaceSources = [
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

describe('platform overview layout guardrails', () => {
  it('keeps platform inventory tables on the shared dense table styling contract', () => {
    expect(sharedPlatformPageSource).toContain('PLATFORM_TABLE_CARD_CLASS');
    expect(sharedPlatformPageSource).toContain('PLATFORM_TABLE_HEADER_ROW_CLASS');
    expect(sharedPlatformPageSource).toContain('PLATFORM_TABLE_BODY_CLASS');
    expect(sharedPlatformPageSource).toContain('getPlatformTableHeadClass');
    expect(sharedPlatformPageSource).toContain('getPlatformTableCellClass');
    expect(sharedPlatformPageSource).toContain('PlatformTableToolbar');
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
    for (const source of proxmoxDetailTableSources) {
      expect(source).toContain('TableCard');
      expect(source).toContain('PlatformTableToolbar');
      expect(source).toContain('PLATFORM_TABLE_CARD_CLASS');
      expect(source).toContain('PLATFORM_TABLE_HEADER_ROW_CLASS');
      expect(source).toContain('PLATFORM_TABLE_BODY_CLASS');
      expect(source).toContain('getPlatformTableHeadClassForKind');
      expect(source).toContain('getPlatformTableCellClassForKind');
      expect(source).not.toContain("from '@/components/shared/SearchInput'");
      expect(source).not.toContain("from '@/components/shared/FilterButtonGroup'");
      expect(source).not.toContain('border-collapse text-xs');
      expect(source).not.toContain('bg-surface-alt text-muted border-b border-border');
    }
  });

  it('keeps provider overview pages in the parent-table plus child-inventory stack', () => {
    for (const source of overviewSurfaceSources) {
      expect(source).toContain('<div class="space-y-4">');
      expect(source).toContain('<PlatformSectionTabs');
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
    expect(truenasPageSurfaceSource).toContain('<StorageSurface');
    expect(truenasPageSurfaceSource).not.toContain('forcedView="pools"');
    expect(truenasPageSurfaceSource).not.toContain('<TrueNASDisksTable');
    expect(vmwarePageSurfaceSource).toContain('<VsphereHostsTable');
    expect(vmwarePageSurfaceSource).toContain('<WorkloadsSurface');
    expect(vmwarePageSurfaceSource).toContain('<StorageSurface');
    expect(vmwarePageSurfaceSource).not.toContain('forcedView="pools"');
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
  });
});

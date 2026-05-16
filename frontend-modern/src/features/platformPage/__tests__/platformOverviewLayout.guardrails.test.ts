import { describe, expect, it } from 'vitest';
import dockerHostsTableSource from '@/features/docker/DockerHostsTable.tsx?raw';
import dockerPageSurfaceSource from '@/features/docker/DockerPageSurface.tsx?raw';
import dockerServicesTableSource from '@/features/docker/DockerServicesTable.tsx?raw';
import kubernetesClustersTableSource from '@/features/kubernetes/KubernetesClustersTable.tsx?raw';
import kubernetesDeploymentsTableSource from '@/features/kubernetes/KubernetesDeploymentsTable.tsx?raw';
import kubernetesNodesTableSource from '@/features/kubernetes/KubernetesNodesTable.tsx?raw';
import kubernetesPageSurfaceSource from '@/features/kubernetes/KubernetesPageSurface.tsx?raw';
import proxmoxNodesTableSource from '@/features/proxmox/ProxmoxNodesTable.tsx?raw';
import sharedPlatformPageSource from '@/features/platformPage/sharedPlatformPage.tsx?raw';
import truenasDisksTableSource from '@/features/truenas/TrueNASDisksTable.tsx?raw';
import truenasPageSurfaceSource from '@/features/truenas/TrueNASPageSurface.tsx?raw';
import truenasSystemsTableSource from '@/features/truenas/TrueNASSystemsTable.tsx?raw';
import vmwarePageSurfaceSource from '@/features/vmware/VmwarePageSurface.tsx?raw';
import vsphereDatastoresTableSource from '@/features/vmware/VsphereDatastoresTable.tsx?raw';
import vsphereHostsTableSource from '@/features/vmware/VsphereHostsTable.tsx?raw';

const platformTableSources = [
  proxmoxNodesTableSource,
  dockerHostsTableSource,
  dockerServicesTableSource,
  kubernetesClustersTableSource,
  kubernetesNodesTableSource,
  kubernetesDeploymentsTableSource,
  truenasSystemsTableSource,
  truenasDisksTableSource,
  vsphereHostsTableSource,
  vsphereDatastoresTableSource,
];

const overviewSurfaceSources = [
  dockerPageSurfaceSource,
  kubernetesPageSurfaceSource,
  truenasPageSurfaceSource,
  vmwarePageSurfaceSource,
];

describe('platform overview layout guardrails', () => {
  it('keeps platform inventory tables on the shared dense table styling contract', () => {
    expect(sharedPlatformPageSource).toContain('PLATFORM_TABLE_CARD_CLASS');
    expect(sharedPlatformPageSource).toContain('PLATFORM_TABLE_HEADER_ROW_CLASS');
    expect(sharedPlatformPageSource).toContain('PLATFORM_TABLE_BODY_CLASS');
    expect(sharedPlatformPageSource).toContain('getPlatformTableHeadClass');
    expect(sharedPlatformPageSource).toContain('getPlatformTableCellClass');

    for (const source of platformTableSources) {
      expect(source).toContain('TableCard');
      expect(source).toContain('TableCardHeader');
      expect(source).toContain('PLATFORM_TABLE_CARD_CLASS');
      expect(source).toContain('PLATFORM_TABLE_HEADER_ROW_CLASS');
      expect(source).toContain('PLATFORM_TABLE_BODY_CLASS');
      expect(source).toContain('getPlatformTableHeadClass');
      expect(source).toContain('getPlatformTableCellClass');
    }
  });

  it('keeps provider overview pages in the parent-table plus child-inventory stack', () => {
    for (const source of overviewSurfaceSources) {
      expect(source).toContain('<div class="space-y-4">');
      expect(source).toContain('showToolbar={false}');
    }

    expect(dockerPageSurfaceSource).toContain('<DockerHostsTable');
    expect(dockerPageSurfaceSource).toContain('<WorkloadsSurface');
    expect(dockerPageSurfaceSource).toContain('<DockerServicesTable');
    expect(kubernetesPageSurfaceSource).toContain('<KubernetesClustersTable');
    expect(kubernetesPageSurfaceSource).toContain('<KubernetesNodesTable');
    expect(kubernetesPageSurfaceSource).toContain('<KubernetesDeploymentsTable');
    expect(truenasPageSurfaceSource).toContain('<TrueNASSystemsTable');
    expect(truenasPageSurfaceSource).toContain('<StorageSurface');
    expect(truenasPageSurfaceSource).toContain('<TrueNASDisksTable');
    expect(vmwarePageSurfaceSource).toContain('<VsphereHostsTable');
    expect(vmwarePageSurfaceSource).toContain('<WorkloadsSurface');
    expect(vmwarePageSurfaceSource).toContain('<VsphereDatastoresTable');
  });

  it('keeps mobile host tables focused on useful operational columns', () => {
    expect(dockerHostsTableSource).toContain('<TableHead class={getPlatformTableHeadClass()}>Host');
    expect(dockerHostsTableSource).toContain(
      "<TableHead class={getPlatformTableHeadClass('right')}>CPU",
    );
    expect(dockerHostsTableSource).toContain(
      "<TableHead class={getPlatformTableHeadClass('right')}>Memory",
    );
    expect(dockerHostsTableSource).toContain(
      "<TableHead class={getPlatformTableHeadClass('right')}>Disk",
    );

    expect(kubernetesClustersTableSource).toContain(
      '<TableHead class={getPlatformTableHeadClass()}>Cluster',
    );
    expect(kubernetesClustersTableSource).toContain(
      "<TableHead class={getPlatformTableHeadClass('right')}>Nodes",
    );
    expect(kubernetesNodesTableSource).toContain(
      '<TableHead class={getPlatformTableHeadClass()}>Node',
    );
    expect(kubernetesNodesTableSource).toContain(
      '<span class="md:hidden">{compactCapacityLabel()}</span>',
    );

    expect(truenasSystemsTableSource).toContain(
      '<TableHead class={getPlatformTableHeadClass()}>System',
    );
    expect(truenasSystemsTableSource).toContain(
      '<span class="md:hidden">{formatPercent(storagePercent())}</span>',
    );
    expect(vsphereHostsTableSource).toContain(
      '<TableHead class={getPlatformTableHeadClass()}>Host',
    );
    expect(vsphereHostsTableSource).toContain(
      "<TableHead class={getPlatformTableHeadClass('right')}>VMs",
    );
    expect(vsphereHostsTableSource).toContain('hidden md:table-cell');
  });
});

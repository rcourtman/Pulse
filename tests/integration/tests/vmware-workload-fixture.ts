import type { Page } from '@playwright/test';

const VMWARE_WORKLOAD_RESOURCE = {
  id: 'vmware:vc-mock-1:vm:vm-201',
  type: 'vm',
  name: 'warehouse-api-01',
  displayName: 'warehouse-api-01',
  status: 'online',
  lastSeen: '2026-07-19T12:00:00Z',
  sources: ['vmware'],
  platformScopes: ['vmware-vsphere'],
  platformType: 'vmware-vsphere',
  canonicalIdentity: {
    primaryId: 'vmware:vc-mock-1:vm:vm-201',
    displayName: 'warehouse-api-01',
    hostname: 'warehouse-api-01.internal',
    aliases: ['vm-201', 'warehouse-api-01'],
  },
  identity: {
    hostnames: ['warehouse-api-01.internal'],
    ipAddresses: ['10.42.10.121'],
    clusterName: 'Production Cluster',
  },
  metrics: {
    cpu: { value: 18, percent: 18, unit: 'percent' },
    memory: {
      used: 4_294_967_296,
      total: 8_589_934_592,
      percent: 50,
      unit: 'bytes',
    },
    disk: {
      used: 68_719_476_736,
      total: 137_438_953_472,
      percent: 50,
      unit: 'bytes',
    },
  },
  vmware: {
    connectionId: 'vc-mock-1',
    connectionName: 'Lab vCenter',
    vcenterHost: 'vcsa.lab.local',
    entityType: 'vm',
    managedObjectId: 'vm-201',
    datacenterName: 'Primary DC',
    clusterName: 'Production Cluster',
    runtimeHostId: 'host-101',
    runtimeHostName: 'esxi-01.lab.local',
    powerState: 'POWERED_ON',
    guestOsFamily: 'LINUX',
    guestHostname: 'warehouse-api-01.internal',
    guestIpAddresses: ['10.42.10.121'],
  },
} as const;

export async function installVmwareWorkloadResourceRoute(page: Page): Promise<void> {
  await page.route('**/api/resources?*', async (route) => {
    const url = new URL(route.request().url());
    const resourceTypes = (url.searchParams.get('type') ?? '').split(',');
    if (!resourceTypes.includes('vm')) {
      await route.continue();
      return;
    }

    const pageNumber = Number(url.searchParams.get('page') ?? '1');
    const limit = Number(url.searchParams.get('limit') ?? '100');
    await route.fulfill({
      status: 200,
      json: {
        data: pageNumber === 1 ? [VMWARE_WORKLOAD_RESOURCE] : [],
        meta: {
          page: pageNumber,
          limit,
          total: 1,
          totalPages: 1,
        },
      },
    });
  });
}

import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  VMWARE_TAB_SPECS,
  buildVmwarePageModel,
  filterVmwareDatastores,
  filterVmwareIncidents,
  filterVmwareVirtualMachines,
  mapVmwareDatastoreStatus,
  mapVmwareIncidentSeverity,
  mapVmwareVirtualMachineStatus,
} from '../vmwarePageModel';

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  name: resource.id,
  displayName: resource.id,
  platformId: 'lab',
  platformType: 'vmware-vsphere',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1_700_000_000_000,
  ...resource,
});

describe('vmwarePageModel', () => {
  it('declares the vSphere section set', () => {
    expect(VMWARE_TAB_SPECS.map((tab) => tab.id)).toEqual(['overview', 'storage']);
    expect(VMWARE_TAB_SPECS.map((tab) => tab.label)).toEqual(['Overview', 'Datastores']);
  });

  it('buckets canonical vSphere hosts, VMs, and datastores', () => {
    const model = buildVmwarePageModel([
      makeResource({ id: 'esxi-host-1', type: 'agent' }),
      makeResource({ id: 'vsphere-vm-1', type: 'vm' }),
      makeResource({
        id: 'datastore-1',
        type: 'storage',
        storage: { topology: 'datastore', platform: 'vmware-vsphere' },
        vmware: { entityType: 'datastore' },
      }),
      makeResource({ id: 'legacy-provider-datastore', type: 'datastore' }),
      makeResource({ id: 'pve-vm', type: 'vm', platformType: 'proxmox-pve' }),
    ]);

    expect(model.hosts.map((r) => r.id)).toEqual(['esxi-host-1']);
    expect(model.vms.map((r) => r.id)).toEqual(['vsphere-vm-1']);
    expect(model.datastores.map((r) => r.id)).toEqual(['datastore-1']);
    expect(model.resources.map((r) => r.id).sort()).toEqual(
      ['datastore-1', 'esxi-host-1', 'vsphere-vm-1'].sort(),
    );
  });

  it('filters vSphere datastores using vCenter datastore metadata', () => {
    const accessible = makeResource({
      id: 'ds-accessible',
      type: 'storage',
      name: 'nvme-primary',
      storage: {
        topology: 'datastore',
        platform: 'vmware-vsphere',
        type: 'vmfs',
        nodes: ['esxi-01.lab.local'],
        consumerCount: 2,
        topConsumers: [{ resourceType: 'vm', name: 'warehouse-api-01' }],
      },
      vmware: {
        entityType: 'datastore',
        datastoreAccessible: true,
        datastoreType: 'VMFS',
        datacenterName: 'Primary DC',
      },
    });
    const inaccessible = makeResource({
      id: 'ds-inaccessible',
      type: 'storage',
      name: 'edge-cold-iscsi',
      status: 'offline',
      storage: { topology: 'datastore', platform: 'vmware-vsphere', nodes: ['esxi-06.lab.local'] },
      vmware: {
        entityType: 'datastore',
        datastoreAccessible: false,
        datastoreType: 'VMFS',
        datacenterName: 'Edge DC',
      },
    });
    const maintenance = makeResource({
      id: 'ds-maintenance',
      type: 'storage',
      name: 'maintenance-nfs',
      storage: { topology: 'datastore', platform: 'vmware-vsphere', nodes: ['esxi-07.lab.local'] },
      vmware: {
        entityType: 'datastore',
        datastoreAccessible: true,
        datastoreType: 'NFS41',
        maintenanceMode: 'IN_MAINTENANCE',
      },
    });

    expect(mapVmwareDatastoreStatus(accessible)).toBe('accessible');
    expect(mapVmwareDatastoreStatus(inaccessible)).toBe('inaccessible');
    expect(mapVmwareDatastoreStatus(maintenance)).toBe('maintenance');
    expect(
      filterVmwareDatastores(
        [accessible, inaccessible, maintenance],
        'warehouse',
        'accessible',
      ).map((resource) => resource.id),
    ).toEqual(['ds-accessible']);
    expect(
      filterVmwareDatastores([accessible, inaccessible, maintenance], 'esxi-06', 'all').map(
        (resource) => resource.id,
      ),
    ).toEqual(['ds-inaccessible']);
  });

  it('filters vSphere VMs using vCenter VM metadata', () => {
    const poweredOn = makeResource({
      id: 'vm-powered-on',
      type: 'vm',
      name: 'warehouse-api-01',
      vmware: {
        entityType: 'vm',
        powerState: 'POWERED_ON',
        runtimeHostName: 'esxi-01.lab.local',
        clusterName: 'Production Cluster',
        resourcePoolName: 'Tier 1',
        guestHostname: 'warehouse-api-01.internal',
        guestIpAddresses: ['10.42.10.21'],
        datastoreNames: ['nvme-primary'],
      },
    });
    const attention = makeResource({
      id: 'vm-attention',
      type: 'vm',
      name: 'warehouse-db-01',
      status: 'degraded',
      vmware: {
        entityType: 'vm',
        powerState: 'POWERED_ON',
        overallStatus: 'yellow',
        activeAlarmCount: 1,
        activeAlarmSummary: 'Snapshot age warning',
        runtimeHostName: 'esxi-02.lab.local',
      },
    });
    const poweredOff = makeResource({
      id: 'vm-powered-off',
      type: 'vm',
      name: 'cold-archive-01',
      status: 'stopped',
      vmware: {
        entityType: 'vm',
        powerState: 'poweredOff',
        runtimeHostName: 'esxi-03.lab.local',
      },
    });

    expect(mapVmwareVirtualMachineStatus(poweredOn)).toBe('powered-on');
    expect(mapVmwareVirtualMachineStatus(attention)).toBe('attention');
    expect(mapVmwareVirtualMachineStatus(poweredOff)).toBe('powered-off');
    expect(
      filterVmwareVirtualMachines([poweredOn, attention, poweredOff], 'tier 1', 'powered-on').map(
        (resource) => resource.id,
      ),
    ).toEqual(['vm-powered-on']);
    expect(
      filterVmwareVirtualMachines([poweredOn, attention, poweredOff], 'snapshot', 'attention').map(
        (resource) => resource.id,
      ),
    ).toEqual(['vm-attention']);
    expect(
      filterVmwareVirtualMachines([poweredOn, attention, poweredOff], 'esxi-03', 'all').map(
        (resource) => resource.id,
      ),
    ).toEqual(['vm-powered-off']);
  });

  it('builds and filters vSphere health signals from resource incidents', () => {
    const hostAlarm = makeResource({
      id: 'host-alarm',
      type: 'agent',
      name: 'esxi-01.lab.local',
      displayName: 'esxi-01.lab.local',
      status: 'degraded',
      vmware: {
        entityType: 'host',
        managedObjectId: 'host-101',
        connectionName: 'lab-vcenter',
        vcenterHost: 'vcsa.lab.local',
        datacenterName: 'Primary DC',
        clusterName: 'Production Cluster',
      },
      incidents: [
        {
          provider: 'vmware',
          nativeId: 'alarm-401',
          code: 'vmware_alarm_state',
          severity: 'critical',
          source: 'vmware',
          summary: 'Host host-101 has VMware alarm Host connection and power state (red)',
          startedAt: '2026-05-21T14:30:00Z',
        },
      ],
    });
    const datastoreHealth = makeResource({
      id: 'datastore-health',
      type: 'storage',
      name: 'edge-cold-iscsi',
      status: 'degraded',
      incidentSeverity: 'yellow',
      incidentCode: 'vmware_health_state',
      incidentSummary: 'Datastore datastore-304 has VMware overall status yellow',
      storage: { topology: 'datastore', platform: 'vmware-vsphere' },
      vmware: {
        entityType: 'datastore',
        managedObjectId: 'datastore-304',
        activeAlarmSummary: 'Datastore usage on disk',
      },
    });

    const rows = buildVmwarePageModel([datastoreHealth, hostAlarm]).incidents;

    expect(mapVmwareIncidentSeverity('red')).toBe('critical');
    expect(mapVmwareIncidentSeverity('yellow')).toBe('warning');
    expect(rows.map((row) => row.id)).toEqual([
      'host-alarm:incident:alarm-401:0',
      'datastore-health:incident:rollup',
    ]);
    expect(rows[0]).toMatchObject({
      resourceName: 'esxi-01.lab.local',
      entityType: 'host',
      managedObjectId: 'host-101',
      severityBucket: 'critical',
      label: 'vSphere Alarm',
    });
    expect(
      filterVmwareIncidents(rows, 'datastore-304', 'warning').map((row) => row.resourceId),
    ).toEqual(['datastore-health']);
    expect(
      filterVmwareIncidents(rows, 'production', 'critical').map((row) => row.resourceId),
    ).toEqual(['host-alarm']);
  });
});

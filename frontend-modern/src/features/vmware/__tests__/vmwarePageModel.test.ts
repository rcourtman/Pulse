import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  VMWARE_TAB_SPECS,
  buildVmwarePageModel,
  filterVmwareActivity,
  filterVmwareDatastores,
  filterVmwareIncidents,
  filterVmwareVirtualMachines,
  mapVmwareActivityStateBucket,
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
    expect(VMWARE_TAB_SPECS.map((tab) => tab.id)).toEqual([
      'overview',
      'storage',
      'health',
      'activity',
    ]);
    expect(VMWARE_TAB_SPECS.map((tab) => tab.label)).toEqual([
      'Overview',
      'Datastores',
      'Health',
      'Activity',
    ]);
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

  it('builds and filters vSphere activity from VMware resource changes', () => {
    const vm = makeResource({
      id: 'vm-201',
      type: 'vm',
      name: 'warehouse-api-01',
      displayName: 'warehouse-api-01',
      canonicalIdentity: {
        primaryId: 'vc-1:vm:vm-201',
        aliases: ['vc-1:vm:vm-201', 'vm-201'],
      },
      vmware: {
        connectionId: 'vc-1',
        entityType: 'vm',
        managedObjectId: 'vm-201',
        connectionName: 'lab-vcenter',
        vcenterHost: 'vcsa.lab.local',
        datacenterName: 'Primary DC',
        clusterName: 'Production Cluster',
      },
    });
    const activityChanges = [
      {
        id: 'activity-task-reconfigure',
        observedAt: '2026-05-21T10:15:00Z',
        occurredAt: '2026-05-21T10:15:00Z',
        resourceId: 'vc-1:vm:vm-201',
        kind: 'activity',
        sourceType: 'platform_event',
        sourceAdapter: 'vmware_adapter',
        confidence: 'high',
        reason: 'Reconfigure virtual machine (error)',
        metadata: {
          activity_type: 'vmware_task',
          activity_native_id: 'task-901',
          activity_title: 'Reconfigure virtual machine',
          activity_state: 'error',
          activity_message: 'Permission denied while reconfiguring VM',
          vmwareConnectionId: 'vc-1',
          vmwareTaskDescription: 'Reconfigure virtual machine CPU reservation',
          vmwareManagedObjectId: 'vm-201',
          vmwareEntityType: 'vm',
        },
      },
      {
        id: 'activity-event-powered-on',
        observedAt: '2026-05-21T10:05:00Z',
        occurredAt: '2026-05-21T10:05:00Z',
        resourceId: 'vc-1:vm:vm-201',
        kind: 'activity',
        sourceType: 'platform_event',
        sourceAdapter: 'vmware_adapter',
        confidence: 'high',
        actor: 'administrator@vsphere.local',
        reason: 'VmPoweredOnEvent',
        metadata: {
          activity_type: 'vmware_event',
          activity_native_id: 'event-501',
          activity_title: 'VmPoweredOnEvent',
          activity_message: 'Virtual machine warehouse-api-01 was powered on',
          vmwareConnectionId: 'vc-1',
          vmwareEventType: 'VmPoweredOnEvent',
          vmwareEventMessage: 'Virtual machine warehouse-api-01 was powered on',
          vmwareEventUser: 'administrator@vsphere.local',
          vmwareManagedObjectId: 'vm-201',
          vmwareEntityType: 'vm',
        },
      },
      {
        id: 'activity-agent',
        observedAt: '2026-05-21T10:20:00Z',
        resourceId: 'vc-1:vm:vm-201',
        kind: 'activity',
        sourceType: 'agent_action',
        sourceAdapter: 'agent:ops-helper',
        confidence: 'high',
        reason: 'Agent note',
      },
    ] as const;

    const rows = buildVmwarePageModel([vm], [...activityChanges]).activity;

    expect(mapVmwareActivityStateBucket('error')).toBe('failed');
    expect(mapVmwareActivityStateBucket('success')).toBe('success');
    expect(rows.map((row) => row.title)).toEqual([
      'Reconfigure virtual machine',
      'VmPoweredOnEvent',
    ]);
    expect(rows[0]).toMatchObject({
      resourceName: 'warehouse-api-01',
      activityKind: 'task',
      stateBucket: 'failed',
      nativeId: 'task-901',
      managedObjectId: 'vm-201',
    });
    expect(filterVmwareActivity(rows, 'permission', 'failed').map((row) => row.nativeId)).toEqual([
      'task-901',
    ]);
    expect(
      filterVmwareActivity(rows, 'administrator', 'events').map((row) => row.nativeId),
    ).toEqual(['event-501']);
  });
});

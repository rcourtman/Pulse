import { createRoot, createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';

import { agentSensorResourceId } from '@/features/alerts/agentSensorIdentity';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';

import { useThresholdsGuestData } from '../hooks/useThresholdsGuestData';
import { useThresholdsHostData } from '../hooks/useThresholdsHostData';
import type { Resource as TableResource } from '../tableTypes';
import { indexActiveMetricAlerts, thresholdRowHasActiveAlert } from '../thresholdsResourceModel';
import type { ThresholdsTableProps } from '../types';

// Shapes follow a mock stack's live payloads: unified resource IDs on the
// rows, poller and agent keys on the alerts.
const node = {
  id: 'node-6f1c2a9e4b7d3c10',
  type: 'node',
  name: 'pve1',
  displayName: 'pve1',
  status: 'online',
  proxmox: { sourceId: 'mock-cluster-1-pve1' },
  canonicalIdentity: { primaryId: 'node:mock-cluster-1-pve1' },
} as unknown as Resource;

// A Proxmox node running a Pulse agent arrives as one merged resource that
// answers on the Proxmox and Machines tabs alike.
const linkedNode = {
  id: 'agent-3cd365581fe713ed',
  type: 'agent',
  name: 'West Production A',
  displayName: 'West Production A',
  status: 'online',
  proxmox: { sourceId: 'mock-cluster-1-pve2' },
  discoveryTarget: {
    resourceType: 'agent',
    agentId: 'host-node-mock-cluster-1-pve2',
    resourceId: 'host-node-mock-cluster-1-pve2',
  },
  canonicalIdentity: {
    primaryId: 'agent:host-node-mock-cluster-1-pve2',
    aliases: ['agent:host-node-mock-cluster-1-pve2', 'host-node-mock-cluster-1-pve2'],
  },
  agent: { agentId: 'host-node-mock-cluster-1-pve2' },
} as unknown as Resource;

const machine = {
  id: 'agent-acdfdee2953587fb',
  type: 'agent',
  name: 'Apollo-114',
  displayName: 'Apollo-114',
  status: 'online',
  discoveryTarget: { resourceType: 'agent', agentId: 'host-linux-1', resourceId: 'host-linux-1' },
  canonicalIdentity: { primaryId: 'agent:host-linux-1' },
  agent: {
    agentId: 'host-linux-1',
    disks: [{ mountpoint: '/' }, { mountpoint: '/data' }],
    sensors: {
      native: [
        {
          id: 'coretemp-isa-0000/temp1',
          chip: 'coretemp-isa-0000',
          channel: 'temp1',
          label: 'Package id 0',
          kind: 'temperature',
          unit: 'celsius',
          value: 42.9,
          status: 'ok',
        },
      ],
    },
  },
} as unknown as Resource;

const guest = {
  id: 'vm-24bae4f81cb24f06',
  type: 'vm',
  name: 'analytics-batch-01',
  status: 'running',
  proxmox: {
    sourceId: 'mock-cluster-1-pve1-109',
    node: 'pve1',
    instance: 'mock-cluster-1',
    vmid: 109,
  },
} as unknown as Resource;

const SENSOR_ID = 'coretemp-isa-0000/temp1';

const metricAlert = (resourceId: string, metric: string): Alert =>
  ({
    id: `${resourceId}::metric-threshold:${metric}`,
    type: metric,
    level: 'critical',
    resourceId,
    resourceName: resourceId,
    node: '',
    instance: '',
    message: '',
    value: 99,
    threshold: 1,
    startTime: '2026-11-01T13:00:00Z',
    acknowledged: false,
  }) as Alert;

// The live map, keyed by alert ID as the websocket store keeps it.
const activeAlertMap = (...alerts: Alert[]): Record<string, Alert> =>
  Object.fromEntries(alerts.map((alert) => [alert.id, alert]));

const thresholdRows = () =>
  createRoot((dispose) => {
    const [term] = createSignal('');
    const props = {
      nodes: [node, linkedNode],
      agents: [machine, linkedNode],
      allGuests: () => [guest],
      overrides: () => [],
      nodeDefaults: {},
      agentDefaults: {},
      guestDefaults: {},
      backupDefaults: () => ({}),
      snapshotDefaults: () => ({}),
    } as unknown as ThresholdsTableProps;
    const host = useThresholdsHostData({ props, editingId: () => null, searchTerm: term });
    const guests = useThresholdsGuestData({ props, editingId: () => null, searchTerm: term });
    const rows = {
      node: host.nodesWithOverrides()[0],
      linkedNode: host.nodesWithOverrides()[1],
      machine: host.agentsWithOverrides()[0],
      linkedMachine: host.agentsWithOverrides()[1],
      rootDisk: host.agentDisksWithOverrides().find((row) => row.name === '/'),
      dataDisk: host.agentDisksWithOverrides().find((row) => row.name === '/data'),
      sensor: host.agentSensorsWithOverrides()[0],
      guest: Object.values(guests.guestsGroupedByNode()).flat()[0],
    };
    dispose();
    return rows;
  });

const lit = (alerts: Record<string, Alert>, row: TableResource | undefined, metric: string) => {
  expect(row).toBeDefined();
  return thresholdRowHasActiveAlert(indexActiveMetricAlerts(alerts), row!, metric);
};

describe('threshold row active-alert dot', () => {
  it('lights a node row from the alert raised on its Proxmox source ID', () => {
    const rows = thresholdRows();
    const alerts = activeAlertMap(metricAlert('mock-cluster-1-pve1', 'temperature'));

    expect(lit(alerts, rows.node, 'temperature')).toBe(true);
    expect(lit(alerts, rows.node, 'cpu')).toBe(false);
  });

  it("keeps a linked node's agent alerts on its Machines row", () => {
    const rows = thresholdRows();
    const alerts = activeAlertMap(
      // CPU temperature stays with the node; CPU and disk move to the agent.
      metricAlert('mock-cluster-1-pve2', 'temperature'),
      metricAlert('agent:host-node-mock-cluster-1-pve2', 'cpu'),
      metricAlert('agent:host-node-mock-cluster-1-pve2/disk:data', 'disk'),
    );

    expect(lit(alerts, rows.linkedNode, 'temperature')).toBe(true);
    expect(lit(alerts, rows.linkedNode, 'cpu')).toBe(false);
    expect(lit(alerts, rows.linkedNode, 'disk')).toBe(false);
    expect(lit(alerts, rows.linkedMachine, 'cpu')).toBe(true);
    expect(lit(alerts, rows.linkedMachine, 'disk')).toBe(true);
    // A node CPU alert raised before the agent took over is judged by the node's limit.
    expect(
      lit(activeAlertMap(metricAlert('mock-cluster-1-pve2', 'cpu')), rows.linkedMachine, 'cpu'),
    ).toBe(false);
  });

  it('lights a machine row from its "agent:<host>" alerts', () => {
    const rows = thresholdRows();
    const alerts = activeAlertMap(
      metricAlert('agent:host-linux-1', 'cpu'),
      metricAlert('agent:host-linux-1', 'memory'),
    );

    expect(rows.machine?.id).toBe('host-linux-1');
    expect(lit(alerts, rows.machine, 'cpu')).toBe(true);
    expect(lit(alerts, rows.machine, 'memory')).toBe(true);
    expect(lit(alerts, rows.machine, 'disk')).toBe(false);
  });

  it('lights a machine disk row and the machine row whose Disk limits it inherits', () => {
    const rows = thresholdRows();
    const alerts = activeAlertMap(
      metricAlert('agent:host-linux-1/disk:data', 'disk'),
      metricAlert('agent:host-linux-1/disk_temp:sda', 'diskTemperature'),
    );

    expect(lit(alerts, rows.dataDisk, 'disk')).toBe(true);
    expect(lit(alerts, rows.rootDisk, 'disk')).toBe(false);
    expect(lit(alerts, rows.machine, 'disk')).toBe(true);
    expect(lit(alerts, rows.machine, 'diskTemperature')).toBe(true);
    expect(lit(alerts, rows.machine, 'cpu')).toBe(false);
  });

  it('lights a machine sensor row from its hex-keyed sensor alert only', () => {
    const rows = thresholdRows();
    const alerts = activeAlertMap(
      metricAlert(agentSensorResourceId('host-linux-1', SENSOR_ID), 'temperature'),
    );

    expect(rows.sensor?.id).toBe(agentSensorResourceId('host-linux-1', SENSOR_ID));
    expect(lit(alerts, rows.sensor, 'temperature')).toBe(true);
    // A sensor's temperature is not one of the machine row's limits.
    expect(lit(alerts, rows.machine, 'temperature')).toBe(false);
  });

  it('lights a guest row from the alert raised on its Proxmox source ID', () => {
    const rows = thresholdRows();
    const alerts = activeAlertMap(metricAlert('mock-cluster-1-pve1-109', 'cpu'));

    expect(lit(alerts, rows.guest, 'cpu')).toBe(true);
    expect(lit(alerts, rows.guest, 'memory')).toBe(false);
  });

  it('matches nested alerts only for machine rows', () => {
    const index = indexActiveMetricAlerts(
      activeAlertMap(metricAlert('agent:host-linux-1/disk:data', 'disk')),
    );
    const row = { id: 'host-linux-1', alertResourceIds: ['agent:host-linux-1'] };

    expect(thresholdRowHasActiveAlert(index, { ...row, type: 'agent' }, 'disk')).toBe(true);
    expect(thresholdRowHasActiveAlert(index, { ...row, type: 'dockerHost' }, 'disk')).toBe(false);
  });

  it('falls back to the row ID when a row names no alert resource IDs', () => {
    const index = indexActiveMetricAlerts(
      activeAlertMap(metricAlert('storage-d5aa7911eea57351', 'usage')),
    );

    expect(thresholdRowHasActiveAlert(index, { id: 'storage-d5aa7911eea57351' }, 'usage')).toBe(
      true,
    );
    expect(indexActiveMetricAlerts(undefined).byResource.size).toBe(0);
  });
});

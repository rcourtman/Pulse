import { createRoot, createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';

import { buildProjectedOverrides } from '@/features/alerts/alertOverridesModel';
import {
  agentSensorResourceId,
  parseAgentSensorResourceId,
} from '@/features/alerts/agentSensorIdentity';
import type { HostNativeSensor } from '@/types/api';
import type { Resource } from '@/types/resource';

import type { Override, ThresholdsTableProps } from '../../types';
import { useThresholdsHostData } from '../useThresholdsHostData';

const sensor = (
  chip: string,
  channel: string,
  label: string,
  value: number | undefined,
  status: HostNativeSensor['status'] = 'ok',
  kind: HostNativeSensor['kind'] = 'temperature',
): HostNativeSensor => ({
  id: `${chip}/${channel}`,
  chip,
  channel,
  label,
  kind,
  unit: kind === 'fan' ? 'rpm' : 'celsius',
  value,
  status,
});

const machine = (
  id: string,
  hostId: string,
  name: string,
  native: HostNativeSensor[],
  status: Resource['status'] = 'online',
): Resource =>
  ({
    id,
    name,
    displayName: name,
    type: 'agent',
    status,
    agent: { agentId: hostId, sensors: { native } },
  }) as unknown as Resource;

const sensorOverride = (hostId: string, sensorId: string, trigger: number): Override => ({
  id: agentSensorResourceId(hostId, sensorId),
  name: sensorId,
  type: 'agentSensor',
  thresholds: { temperature: trigger },
});

const sensorRows = (agents: Resource[], overrides: Override[] = [], search = '') =>
  createRoot((dispose) => {
    const [term] = createSignal(search);
    const data = useThresholdsHostData({
      props: {
        agents,
        overrides: () => overrides,
        agentDefaults: {},
        nodeDefaults: {},
      } as unknown as ThresholdsTableProps,
      editingId: () => null,
      searchTerm: term,
    });
    const rows = data.agentSensorsWithOverrides();
    const groups = data.agentSensorsGroupedByAgent();
    dispose();
    return { rows, groups };
  });

describe('agentSensorResourceId', () => {
  it('builds the backend alert resource ID from the UTF-8 hex of the sensor ID', () => {
    expect(agentSensorResourceId('host-1', 'applesmc-isa-0300/temp1')).toBe(
      'agent:host-1/sensor:6170706c65736d632d6973612d303330302f74656d7031',
    );
    // Case-only differences stay distinct, as the backend keeps them.
    expect(agentSensorResourceId('host-1', 'chip/TEMP1')).not.toBe(
      agentSensorResourceId('host-1', 'chip/temp1'),
    );
  });

  it('round-trips through parseAgentSensorResourceId and rejects other IDs', () => {
    const id = agentSensorResourceId('host:1', 'nvme-pci-0100/temp1');
    expect(parseAgentSensorResourceId(id)).toEqual({
      hostId: 'host:1',
      sensorId: 'nvme-pci-0100/temp1',
    });
    expect(parseAgentSensorResourceId('agent:host-1/disk:root')).toBeNull();
    expect(parseAgentSensorResourceId('agent:host-1/sensor:abc')).toBeNull();
    expect(parseAgentSensorResourceId('agent:host-1/sensor:ff')).toBeNull();
  });
});

describe('useThresholdsHostData machine sensors', () => {
  it('lists usable temperature sensors under their machine, Off by default', () => {
    const mac = machine('agent-res-1', 'host-1', 'macbook-pve', [
      sensor('applesmc-isa-0300', 'temp1', 'TB0T', 31.5),
      sensor('applesmc-isa-0300', 'temp2', 'TB1T', -127, 'invalid'),
      sensor('applesmc-isa-0300', 'fan1', 'Left side', 1800, 'ok', 'fan'),
    ]);

    const { rows, groups } = sensorRows([mac]);

    expect(rows).toHaveLength(1);
    expect(rows[0]).toMatchObject({
      id: agentSensorResourceId('host-1', 'applesmc-isa-0300/temp1'),
      name: 'TB0T',
      rawName: 'applesmc-isa-0300/temp1',
      type: 'agentSensor',
      node: 'macbook-pve',
      instance: 'applesmc-isa-0300',
      subtitle: '31.5°C · applesmc-isa-0300',
      defaults: { temperature: 0 },
      thresholds: {},
      hasOverride: false,
    });
    expect(rows[0].readingMessage).toBeUndefined();
    expect(Object.keys(groups)).toEqual(['agent-res-1']);
  });

  it('keeps configured sensors that read invalid or went missing', () => {
    const mac = machine('agent-res-1', 'host-1', 'macbook-pve', [
      sensor('applesmc-isa-0300', 'temp2', 'TB1T', -127, 'invalid'),
    ]);

    const { rows } = sensorRows(
      [mac],
      [
        sensorOverride('host-1', 'applesmc-isa-0300/temp2', 45),
        sensorOverride('host-1', 'applesmc-isa-0300/temp9', 50),
      ],
    );

    const byName = new Map(rows.map((row) => [row.name, row]));
    expect(byName.get('TB1T')).toMatchObject({
      hasOverride: true,
      thresholds: { temperature: 45 },
      subtitle: 'applesmc-isa-0300',
      readingMessage: 'No valid reading from this sensor.',
    });
    expect(byName.get('applesmc-isa-0300/temp9')).toMatchObject({
      hasOverride: true,
      thresholds: { temperature: 50 },
      readingMessage: 'Not reported by this machine. Saved alert settings are retained.',
    });
  });

  it('keeps a saved zero limit removable and carries the override note', () => {
    const mac = machine('agent-res-1', 'host-1', 'macbook-pve', [
      sensor('applesmc-isa-0300', 'temp1', 'TB0T', 31.5),
    ]);
    const saved: Override = {
      ...sensorOverride('host-1', 'applesmc-isa-0300/temp1', 0),
      note: 'battery swelling watch',
    };

    const { rows } = sensorRows([mac], [saved]);

    expect(rows[0]).toMatchObject({
      hasOverride: true,
      presentationDefaults: true,
      note: 'battery swelling watch',
      thresholds: { temperature: 0 },
    });
  });

  it('shows no current reading for a machine that is not reporting', () => {
    const offline = machine(
      'agent-res-1',
      'host-1',
      'macbook-pve',
      [sensor('applesmc-isa-0300', 'temp1', 'TB0T', 31.5)],
      'offline',
    );

    const { rows } = sensorRows(
      [offline],
      [sensorOverride('host-1', 'applesmc-isa-0300/temp1', 45)],
    );

    expect(rows).toHaveLength(1);
    expect(rows[0].subtitle).toBe('applesmc-isa-0300');
    expect(rows[0].readingMessage).toBe(
      'Machine is not reporting, so there is no current reading.',
    );
  });

  it('tells repeated labels apart and keeps same-named machines apart', () => {
    const first = machine('agent-res-1', 'host-1', 'pve', [
      sensor('nvme-pci-0100', 'temp1', 'Composite', 40),
      sensor('nvme-pci-0200', 'temp1', 'Composite', 42),
      sensor('acpitz-acpi-0', 'temp1', 'temp', 30),
      sensor('acpitz-acpi-0', 'temp2', 'temp', 31),
    ]);
    const second = machine('agent-res-2', 'host-2', 'pve', [
      sensor('coretemp-isa-0000', 'temp1', 'Package id 0', 50),
    ]);

    const { rows, groups } = sensorRows([first, second]);

    // Across chips the subtitle's chip tells the rows apart; within one chip
    // the channel joins the name.
    expect(groups['agent-res-1'].map((row) => [row.name, row.subtitle])).toEqual([
      ['Composite', '40.0°C · nvme-pci-0100'],
      ['Composite', '42.0°C · nvme-pci-0200'],
      ['temp (temp1)', '30.0°C · acpitz-acpi-0'],
      ['temp (temp2)', '31.0°C · acpitz-acpi-0'],
    ]);
    expect(groups['agent-res-2'].map((row) => row.name)).toEqual(['Package id 0']);
    expect(new Set(rows.map((row) => row.id)).size).toBe(5);
  });

  it('keeps an override whose machine is gone, and searches by sensor ID', () => {
    const { rows, groups } = sensorRows([], [sensorOverride('ghost', 'chip/temp1', 60)]);
    expect(rows).toHaveLength(1);
    expect(rows[0]).toMatchObject({
      name: 'chip/temp1',
      node: 'ghost',
      readingMessage: 'Machine not found. Saved alert settings are retained.',
    });
    expect(Object.keys(groups)).toEqual(['ghost']);

    const mac = machine('agent-res-1', 'host-1', 'macbook-pve', [
      sensor('applesmc-isa-0300', 'temp1', 'TB0T', 31.5),
      sensor('coretemp-isa-0000', 'temp1', 'Package id 0', 50),
    ]);
    expect(sensorRows([mac], [], 'applesmc').rows.map((row) => row.name)).toEqual(['TB0T']);
  });
});

describe('buildProjectedOverrides machine sensors', () => {
  it('projects a sensor override with its sensor ID and machine name', () => {
    const mac = machine('agent-res-1', 'host-1', 'macbook-pve', []);
    const id = agentSensorResourceId('host-1', 'applesmc-isa-0300/temp1');

    const projected = buildProjectedOverrides({
      rawConfig: { [id]: { temperature: { trigger: 45, clear: 40 }, note: 'battery' } },
      nodeResources: [],
      vmResources: [],
      containerResources: [],
      storageResources: [],
      agentResourceList: [mac],
      containerRuntimeResources: [],
      getChildren: () => [],
      pbsInstanceById: new Map(),
      allResources: [mac],
    } as unknown as Parameters<typeof buildProjectedOverrides>[0]);

    expect(projected).toEqual([
      expect.objectContaining({
        id,
        note: 'battery',
        name: 'applesmc-isa-0300/temp1',
        type: 'agentSensor',
        node: 'macbook-pve',
        thresholds: { temperature: 45 },
      }),
    ]);
  });
});

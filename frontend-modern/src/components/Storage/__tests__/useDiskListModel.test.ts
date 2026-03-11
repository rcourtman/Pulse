import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { useDiskListModel } from '@/components/Storage/useDiskListModel';

const buildNode = (id: string, name: string): Resource =>
  ({
    id,
    type: 'agent',
    name,
    displayName: name,
    platformId: 'cluster-main',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    platformData: { proxmox: { instance: 'cluster-main' } },
  }) as Resource;

const buildDisk = (id: string, nodeName: string, model = `Disk ${id}`): Resource =>
  ({
    id,
    type: 'physical_disk',
    name: id,
    displayName: id,
    platformId: 'cluster-main',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    parentId: `node-${nodeName}`,
    lastSeen: Date.now(),
    identity: { hostname: nodeName },
    canonicalIdentity: { hostname: nodeName },
    platformData: { proxmox: { nodeName, instance: 'cluster-main' } },
    physicalDisk: {
      devPath: `/dev/${id}`,
      model,
      serial: `SERIAL-${id}`,
      diskType: 'sata',
      sizeBytes: 2_000_000_000_000,
      health: 'PASSED',
      temperature: 41,
    },
  }) as Resource;

describe('useDiskListModel', () => {
  it('builds filtered disk state canonically', () => {
    const [disks] = createSignal<Resource[]>([
      buildDisk('sda', 'tower', 'Archive HDD'),
      buildDisk('sdb', 'tower', 'Cache SSD'),
    ]);
    const [nodes] = createSignal<Resource[]>([buildNode('node-tower', 'tower')]);
    const [selectedNode] = createSignal<string | null>('node-tower');
    const [searchTerm] = createSignal('cache');

    const { result } = renderHook(() =>
      useDiskListModel({
        disks,
        nodes,
        selectedNode,
        searchTerm,
      }),
    );

    expect(result.hasPVENodes()).toBe(true);
    expect(result.selectedNodeName()).toBe('tower');
    expect(result.filteredDisks().map((disk) => disk.id)).toEqual(['sdb']);

    result.toggleSelectedDisk(result.filteredDisks()[0]!);
    expect(result.selectedDisk()?.id).toBe('sdb');
  });
});

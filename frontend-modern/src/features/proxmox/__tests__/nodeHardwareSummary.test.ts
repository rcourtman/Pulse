import { describe, expect, it } from 'vitest';

import type { Node } from '@/types/api';

import {
  installedMemoryLabel,
  nodeHardwareModel,
  physicalNetworkInterfaces,
  shortCpuModel,
} from '../nodeHardwareSummary';

const GiB = 1024 ** 3;

const node = (overrides: Partial<Node> = {}): Node =>
  ({
    id: 'node-1',
    name: 'pve1',
    cpuInfo: { model: 'AMD Ryzen 9 5950X 16-Core Processor', cores: 32, sockets: 1, mhz: '3400' },
    memory: { total: 125.7 * GiB, used: 40 * GiB, free: 85.7 * GiB, usage: 32 },
    networkInterfaces: [],
    ...overrides,
  }) as unknown as Node;

describe('shortCpuModel', () => {
  it('keeps the name on the box', () => {
    expect(shortCpuModel('Intel(R) Xeon(R) CPU E5-2680 v4 @ 2.40GHz')).toBe(
      'Intel Xeon E5-2680 v4',
    );
    expect(shortCpuModel('AMD Ryzen 9 5950X 16-Core Processor')).toBe('AMD Ryzen 9 5950X');
    expect(shortCpuModel('Intel(R) Core(TM) i5-8500T CPU @ 2.10GHz')).toBe('Intel Core i5-8500T');
    expect(shortCpuModel('AMD Ryzen 7 PRO 5850U with Radeon Graphics')).toBe(
      'AMD Ryzen 7 PRO 5850U',
    );
    expect(shortCpuModel('Intel(R) N100')).toBe('Intel N100');
    expect(shortCpuModel('unknown')).toBe('');
  });
});

describe('installedMemoryLabel', () => {
  it('reads usable memory as the installed size it sits just under', () => {
    expect(installedMemoryLabel(125.7 * GiB)).toBe('128 GB');
    expect(installedMemoryLabel(62.6 * GiB)).toBe('64 GB');
    expect(installedMemoryLabel(15.3 * GiB)).toBe('16 GB');
    expect(installedMemoryLabel(1003 * GiB)).toBe('1 TB');
  });

  it('shows an odd size as measured', () => {
    expect(installedMemoryLabel(40 * GiB)).toBe('40 GB');
    expect(installedMemoryLabel(0)).toBe('');
  });
});

describe('physicalNetworkInterfaces', () => {
  it('keeps physical ports and drops bridges, bonds, VLANs, taps and tunnels', () => {
    expect(
      physicalNetworkInterfaces([
        { name: 'eno1', addresses: [] },
        { name: 'enp65s0f0', addresses: [] },
        { name: 'wlp2s0', addresses: [] },
        { name: 'vmbr0', addresses: ['192.168.0.10/24'] },
        { name: 'veth100i0', addresses: [] },
        { name: 'tap101i0', addresses: [] },
        { name: 'fwbr101i0', addresses: [] },
        { name: 'bond0', addresses: [] },
        { name: 'eno1.20', addresses: [] },
        { name: 'tailscale0', addresses: ['100.64.0.2/32'] },
        { name: 'lo', addresses: ['127.0.0.1/8'] },
      ]).map((port) => port.name),
    ).toEqual(['eno1', 'enp65s0f0', 'wlp2s0']);
  });
});

describe('nodeHardwareModel', () => {
  it('describes the CPU, memory, each port and the fans', () => {
    const model = nodeHardwareModel(
      node({
        networkInterfaces: [
          { name: 'enp1s0', addresses: [], speedMbps: 10000 },
          { name: 'eno1', addresses: [], speedMbps: 2500 },
          { name: 'wlp2s0', addresses: [] },
          { name: 'vmbr0', addresses: [] },
        ],
        sensors: { fanRpm: { fan1: 820, fan2: 1450, fan3: 0 } },
      } as Partial<Node>),
    );

    expect(model.cpu).toEqual({
      name: 'AMD Ryzen 9 5950X',
      threads: 32,
      title: 'AMD Ryzen 9 5950X 16-Core Processor (1 socket), 32 threads',
    });
    expect(model.memory?.label).toBe('128 GB');
    expect(model.ports).toEqual([
      {
        name: 'enp1s0',
        wireless: false,
        speedLabel: '10 Gb/s',
      },
      { name: 'eno1', wireless: false, speedLabel: '2.5 Gb/s' },
      {
        name: 'wlp2s0',
        wireless: true,
      },
    ]);
    expect(model.fans).toEqual({ count: 2, title: '2 fans at 820–1,450 RPM' });
  });

  it('marks ports the agent reports without a link, and only those', () => {
    const ports = nodeHardwareModel(
      node({
        networkInterfaces: [
          { name: 'enp3s0', addresses: [], speedMbps: 1000, linkUp: true },
          { name: 'enp1s0', addresses: [], speedMbps: 1000, linkUp: false },
          { name: 'wlp2s0', addresses: [], linkUp: false },
          { name: 'eno2', addresses: [] },
        ],
      } as Partial<Node>),
    ).ports;

    expect(ports.map((port) => [port.name, port.linkUp, port.speedLabel])).toEqual([
      ['enp3s0', true, '1 Gb/s'],
      // A stale speed beside no link is not shown.
      ['enp1s0', false, undefined],
      ['wlp2s0', false, undefined],
      // Proxmox alone cannot say, so the port is not guessed down.
      ['eno2', undefined, undefined],
    ]);
  });

  it('names the socket count on a multi-socket node', () => {
    expect(
      nodeHardwareModel(
        node({
          cpuInfo: {
            model: 'Intel(R) Xeon(R) CPU E5-2680 v4 @ 2.40GHz',
            cores: 56,
            sockets: 2,
            mhz: '2400',
          },
        }),
      ).cpu?.name,
    ).toBe('2 × Intel Xeon E5-2680 v4');
  });

  it('is empty when the node reports no hardware', () => {
    expect(
      nodeHardwareModel(
        node({
          cpuInfo: { model: 'Unknown', cores: 0, sockets: 0, mhz: '0' },
          memory: { total: 0, used: 0, free: 0, usage: 0 },
        }),
      ),
    ).toEqual({ ports: [] });
    expect(nodeHardwareModel(null)).toEqual({ ports: [] });
  });
});

import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import {
  buildCephClusterContentSections,
  buildCephClusterDetailSections,
  getCephClusterExceptionRows,
  getCephClusterFacts,
} from '../proxmoxCephClusterDrawerModel';

const cluster = (ceph: Partial<NonNullable<Resource['ceph']>> = {}): Resource =>
  ({
    id: 'ceph-main',
    type: 'ceph',
    name: 'ceph-main',
    platformId: 'homelab',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: 1,
    tags: ['prod'],
    disk: { total: 10 * 1024 ** 4, used: 4 * 1024 ** 4, free: 6 * 1024 ** 4, current: 40 },
    ceph: {
      fsid: '8f1c2d3e-0000-4b8c-9d2e-1234567890ab',
      healthStatus: 'HEALTH_OK',
      numMons: 3,
      numMgrs: 2,
      numOsds: 4,
      numOsdsUp: 4,
      numOsdsIn: 4,
      numPGs: 128,
      pools: [
        {
          name: 'rbd',
          storedBytes: 1024 ** 3,
          availableBytes: 2 * 1024 ** 3,
          objects: 1500,
          percentUsed: 33.3,
        },
      ],
      services: [
        { type: 'mon', running: 3, total: 3 },
        { type: 'mgr', running: 1, total: 2 },
      ],
      ...ceph,
    },
  }) as Resource;

describe('proxmoxCephClusterDrawerModel', () => {
  it('lifts nothing for a healthy cluster', () => {
    expect(
      getCephClusterExceptionRows(cluster({ services: [{ type: 'mon', running: 3, total: 3 }] })),
    ).toEqual([]);
  });

  it('lifts each health check, down OSDs, short daemons and nearly full pools', () => {
    const rows = getCephClusterExceptionRows(
      cluster({
        healthStatus: 'HEALTH_ERR',
        healthChecks: [
          { code: 'OSD_DOWN', severity: 'HEALTH_WARN', summary: '1 osds down' },
          { code: 'POOL_FULL', severity: 'HEALTH_ERR', summary: '1 pool(s) full' },
        ],
        numOsdsUp: 3,
        pools: [{ name: 'rbd', storedBytes: 1, availableBytes: 0, objects: 1, percentUsed: 97.1 }],
      }),
    );
    expect(rows.map((row) => [row.label, row.value, row.tone])).toEqual([
      ['Health', '1 osds down', 'warning'],
      ['Health', '1 pool(s) full', 'danger'],
      ['OSDs', '3 of 4 up, 4 in', 'warning'],
      ['MGR daemons', '1 of 2 running', 'warning'],
      ['Pool rbd', '97.1% used', 'danger'],
    ]);
  });

  it('falls back to the health message when no checks are reported', () => {
    const rows = getCephClusterExceptionRows(
      cluster({ healthStatus: 'HEALTH_WARN', healthMessage: 'clock skew detected', services: [] }),
    );
    expect(rows).toEqual([
      expect.objectContaining({ label: 'Health', value: 'clock skew detected', tone: 'warning' }),
    ]);
  });

  it('leaves out a fact an open alert already states', () => {
    const rows = getCephClusterExceptionRows(
      cluster({
        healthStatus: 'HEALTH_WARN',
        healthChecks: [{ code: 'OSD_DOWN', severity: 'HEALTH_WARN', summary: '1 osds down' }],
        services: [],
      }),
      ['Ceph cluster ceph-main: 1 osds down'],
    );
    expect(rows).toEqual([]);
  });

  it('keeps pools and daemons in view as content cards', () => {
    const [pools, services] = buildCephClusterContentSections(cluster());
    expect(pools.label).toBe('Pools (1)');
    expect(pools.rows[0]).toMatchObject({
      label: 'rbd',
      layout: 'stacked',
      wrap: true,
      value: '33.3% used · 1.00 GB stored · 2.00 GB available · 1,500 objects',
    });
    expect(pools.rows[0].progress?.value).toBeCloseTo(33.3);
    expect(services.rows.map((row) => [row.label, row.value, row.tone])).toEqual([
      ['MON', '3 of 3 running', 'success'],
      ['MGR', '1 of 2 running', 'warning'],
    ]);
  });

  it('drops empty content cards so the drawer can say so plainly', () => {
    expect(buildCephClusterContentSections(cluster({ pools: [], services: [] }))).toEqual([]);
  });

  it('keeps identifiers and raw health text behind Details, wrapping copyable values', () => {
    const sections = buildCephClusterDetailSections(
      cluster({ healthStatus: 'HEALTH_WARN', healthMessage: 'clock skew detected' }),
    );
    expect(sections.map((section) => section.label)).toEqual(['Cluster', 'Health', 'Topology']);
    const fsid = sections[0].rows.find((row) => row.label === 'FSID');
    expect(fsid).toMatchObject({ value: '8f1c2d3e-0000-4b8c-9d2e-1234567890ab', wrap: true });
    expect(sections[0].rows.find((row) => row.label === 'Tags')?.value).toBe('prod');
    expect(sections[1].rows.map((row) => row.label)).toEqual(['Status', 'Message']);
    expect(sections[2].rows.find((row) => row.label === 'Placement groups')?.value).toBe('128');
  });

  it('states capacity, OSDs, quorum and placement groups in one facts line', () => {
    expect(getCephClusterFacts(cluster())).toEqual([
      '40.0% used · 4.00 TB of 10.0 TB',
      'OSDs 4 of 4 up, 4 in',
      'MON 3 · MGR 2',
      '128 placement groups',
    ]);
  });
});

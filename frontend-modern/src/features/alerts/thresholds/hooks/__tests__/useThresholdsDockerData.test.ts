import { createRoot } from 'solid-js';
import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import type { Override, ThresholdsTableProps } from '../../types';
import { useThresholdsDockerData } from '../useThresholdsDockerData';

const dockerHost = (
  sourceId: string,
  hostname: string,
  options: { agentId?: string; ips?: string[]; displayName?: string } = {},
): Resource => {
  const agentId = options.agentId ?? sourceId;
  return {
    id: `agent-${sourceId}`,
    name: hostname,
    displayName: options.displayName ?? hostname,
    type: 'docker-host',
    status: 'online',
    platformType: 'docker',
    sourceType: 'agent',
    lastSeen: 1,
    identity: { ips: options.ips ?? [] },
    docker: { hostname, hostSourceId: sourceId, agentId },
    platformData: { docker: { hostSourceId: sourceId } },
    discoveryTarget: { resourceType: 'agent', agentId, resourceId: agentId, hostname },
  } as unknown as Resource;
};

const dockerContainer = (host: Resource, name: string): Resource =>
  ({
    id: `app-container-${host.id}-${name}`,
    name,
    displayName: name,
    type: 'app-container',
    status: 'running',
    parentId: host.id,
    platformType: 'docker',
    sourceType: 'agent',
    lastSeen: 1,
    docker: { containerId: `${host.id}-${name}-id` },
  }) as unknown as Resource;

const hostOverride = (id: string): Override =>
  ({
    id,
    name: id,
    type: 'dockerHost',
    disableConnectivity: true,
    thresholds: {},
  }) as unknown as Override;

const containerOverride = (id: string): Override =>
  ({
    id,
    name: id,
    type: 'dockerContainer',
    disabled: true,
    thresholds: {},
  }) as unknown as Override;

const withDockerData = <T>(
  hosts: Resource[],
  containers: Resource[],
  overrides: Override[],
  read: (data: ReturnType<typeof useThresholdsDockerData>) => T,
  searchTerm = '',
): T =>
  createRoot((dispose) => {
    const props = {
      containerRuntimes: hosts,
      allResources: containers,
      overrides: () => overrides,
      dockerDefaults: { cpu: 80, memory: 85, disk: 85 },
    } as unknown as ThresholdsTableProps;
    const result = read(
      useThresholdsDockerData({ props, editingId: () => null, searchTerm: () => searchTerm }),
    );
    dispose();
    return result;
  });

const lab = dockerHost('proxmox-lxc-docker:lab:pve1:200', 'dock');
const site = dockerHost('proxmox-lxc-docker:site:pve2:300', 'dock');

// A machine with no machine ID reports its hostname as agent ID, and the second
// one to arrive gets a token-suffixed host ID. The agent ID stays the hostname.
const firstAlpine = dockerHost('alpine', 'alpine', { agentId: 'alpine', ips: ['192.168.1.50'] });
const secondAlpine = dockerHost('alpine::token-b', 'alpine', {
  agentId: 'alpine',
  ips: ['10.20.0.7'],
});

describe('useThresholdsDockerData same-named hosts', () => {
  it('marks each row with what tells it from the other, and its controls say so too', () => {
    const rows = withDockerData([lab, site], [], [], (data) => data.dockerHostsWithOverrides());

    expect(rows.map((row) => [row.name, row.qualifier])).toEqual([
      ['dock', 'lab'],
      ['dock', 'site'],
    ]);
    expect(new Set(rows.map((row) => row.id)).size).toBe(2);
  });

  it('keeps one group per host, so same-named containers stay apart', () => {
    const containers = [dockerContainer(lab, 'nginx'), dockerContainer(site, 'nginx')];
    const { groups, meta } = withDockerData([lab, site], containers, [], (data) => ({
      groups: data.dockerContainersGroupedByHost(),
      meta: data.dockerHostGroupMeta(),
    }));

    expect(Object.keys(groups).sort()).toEqual(['dock (lab)', 'dock (site)']);
    expect(groups['dock (lab)'].map((row) => row.id)).toEqual([
      'docker:proxmox-lxc-docker:lab:pve1:200/nginx',
    ]);
    expect(groups['dock (site)'].map((row) => row.id)).toEqual([
      'docker:proxmox-lxc-docker:site:pve2:300/nginx',
    ]);
    expect(meta['dock (lab)']).toMatchObject({ displayName: 'dock', qualifier: 'lab' });
    expect(meta['dock (site)']).toMatchObject({ displayName: 'dock', qualifier: 'site' });
  });

  it('does not give a host named like a same-named pair that pair\u2019s words', () => {
    const renamed = dockerHost('renamed-agent', 'other-host', { displayName: 'dock' });
    const { groups, meta } = withDockerData(
      [renamed, lab, site],
      [dockerContainer(renamed, 'plex')],
      [],
      (data) => ({
        groups: data.dockerContainersGroupedByHost(),
        meta: data.dockerHostGroupMeta(),
      }),
    );

    expect(Object.keys(groups)).toEqual(['dock']);
    expect(meta.dock.qualifier).toBeUndefined();
  });

  it.each(['site', 'dock site', 'dock (site)'])(
    'finds a host and its containers by searching %s',
    (search) => {
      const containers = [dockerContainer(lab, 'grafana'), dockerContainer(site, 'redis')];
      const { hosts, groups } = withDockerData(
        [lab, site],
        containers,
        [],
        (data) => ({
          hosts: data.dockerHostsWithOverrides(),
          groups: data.dockerContainersGroupedByHost(),
        }),
        search,
      );

      expect(hosts.map((row) => row.qualifier)).toEqual(['site']);
      expect(Object.keys(groups)).toEqual(['dock (site)']);
    },
  );

  it('leaves a host with a hostname of its own as it was', () => {
    const solo = dockerHost('solo-agent', 'tower.home.example');
    const { rows, groups } = withDockerData(
      [solo],
      [dockerContainer(solo, 'plex')],
      [],
      (data) => ({
        rows: data.dockerHostsWithOverrides(),
        groups: data.dockerContainersGroupedByHost(),
      }),
    );

    expect(rows[0]).toMatchObject({
      name: 'tower',
      displayName: 'tower',
      rawName: 'tower.home.example',
    });
    expect(rows[0].qualifier).toBeUndefined();
    expect(Object.keys(groups)).toEqual(['tower']);
  });

  it('stops shortening names when that would make two hosts read alike', () => {
    const home = dockerHost('fqdn-home', 'storage.home.example');
    const office = dockerHost('fqdn-office', 'storage.office.example');
    const { rows, groups } = withDockerData(
      [home, office],
      [dockerContainer(home, 'minio'), dockerContainer(office, 'seaweed')],
      [],
      (data) => ({
        rows: data.dockerHostsWithOverrides(),
        groups: data.dockerContainersGroupedByHost(),
      }),
    );

    expect(rows.map((row) => row.name)).toEqual(['storage.home.example', 'storage.office.example']);
    expect(Object.keys(groups).sort()).toEqual(['storage.home.example', 'storage.office.example']);
  });
});

describe('useThresholdsDockerData overrides across same-named hosts', () => {
  it('keeps an override saved for one host on that host', () => {
    const rows = withDockerData(
      [lab, site],
      [],
      [hostOverride('proxmox-lxc-docker:lab:pve1:200')],
      (data) => data.dockerHostsWithOverrides(),
    );

    expect(rows.map((row) => row.disableConnectivity)).toEqual([true, false]);
  });

  it('does not show a host with no machine ID the override of the host its agent ID names', () => {
    const rows = withDockerData([firstAlpine, secondAlpine], [], [hostOverride('alpine')], (data) =>
      data.dockerHostsWithOverrides(),
    );

    expect(rows.map((row) => [row.id, row.disableConnectivity])).toEqual([
      ['alpine', true],
      ['alpine::token-b', false],
    ]);
  });

  it('does not show that host the other host’s container override, or clear it on reset', () => {
    const { rows, groups } = withDockerData(
      [firstAlpine, secondAlpine],
      [dockerContainer(firstAlpine, 'web'), dockerContainer(secondAlpine, 'web')],
      [containerOverride('docker:alpine/web')],
      (data) => ({
        rows: data.dockerContainersFlat(),
        groups: data.dockerContainersGroupedByHost(),
      }),
    );

    expect(groups['Unassigned Containers']).toBeUndefined();

    const first = rows.find((row) => row.id === 'docker:alpine/web');
    const second = rows.find((row) => row.id === 'docker:alpine::token-b/web');
    expect(first).toMatchObject({ disabled: true, hasOverride: true });
    expect(second).toMatchObject({ disabled: false, hasOverride: false });
    expect(second?.overrideIdCandidates).not.toContain('docker:alpine/web');
  });

  it('files a saved override for a removed container under Unassigned, not as a host', () => {
    const { hosts, groups } = withDockerData(
      [lab],
      [],
      [
        containerOverride('docker:proxmox-lxc-docker:lab:pve1:200/removed'),
        { ...containerOverride('docker:gone/old'), name: 'old' },
      ],
      (data) => ({
        hosts: data.dockerHostsWithOverrides(),
        groups: data.dockerContainersGroupedByHost(),
      }),
    );

    expect(hosts.map((row) => row.id)).toEqual(['proxmox-lxc-docker:lab:pve1:200']);
    expect(groups['Unassigned Containers'].map((row) => row.id)).toEqual(
      expect.arrayContaining(['docker:proxmox-lxc-docker:lab:pve1:200/removed', 'docker:gone/old']),
    );
  });
});

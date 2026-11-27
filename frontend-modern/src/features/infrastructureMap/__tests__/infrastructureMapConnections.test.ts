import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  caddyRouteHosts,
  mapRouteLabel,
  resolveMapConnections,
  traefikRouteHosts,
} from '../infrastructureMapConnections';

const resource = (partial: Partial<Resource> & Pick<Resource, 'id' | 'type' | 'name'>): Resource =>
  ({
    displayName: partial.name,
    platformId: partial.id,
    platformType: 'generic',
    sourceType: 'api',
    status: 'online',
    lastSeen: 1,
    ...partial,
  }) as Resource;

// A 64-character hex ID, like the runtime's own, that differs per name.
const idOf = (name: string): string =>
  [...name]
    .map((char) => char.charCodeAt(0).toString(16).padStart(2, '0'))
    .join('')
    .padEnd(64, '0')
    .slice(0, 64);

const container = (
  name: string,
  docker: NonNullable<Resource['docker']> = {},
  host = 'tower',
): Resource =>
  resource({
    id: `c-${host}-${name}`,
    type: 'app-container',
    name,
    status: 'running',
    parentId: `agent-${host}`,
    docker: {
      hostSourceId: host,
      containerId: idOf(`${host}${name}`),
      containerState: 'running',
      image: `example/${name}:latest`,
      networks: [{ name: 'bridge' }],
      ...docker,
    },
  });

const machine = (name: string, agent: NonNullable<Resource['agent']>): Resource =>
  resource({ id: `agent-${name}`, type: 'agent', name, agent });

describe('a container that uses another container network', () => {
  // The arrangement on a real Unraid host: three containers inside a VPN
  // container's network, each with no network or port of its own.
  const vpn = container('gluetun', { networkMode: 'bridge' });
  const behind = (name: string) =>
    container(name, { networkMode: `container:${vpn.docker?.containerId}` });

  it('links each one to the container whose network it uses', () => {
    const { links } = resolveMapConnections([
      vpn,
      behind('qbittorrent'),
      behind('prowlarr'),
      container('radarr', { networkMode: 'bridge' }),
      container('sonarr'),
    ]);

    expect(links).toEqual([
      { kind: 'network', fromId: 'c-tower-prowlarr', toId: 'c-tower-gluetun', hosts: [] },
      { kind: 'network', fromId: 'c-tower-qbittorrent', toId: 'c-tower-gluetun', hosts: [] },
    ]);
  });

  it('draws nothing from a missing network list alone', () => {
    // An older agent reports no mode. No networks and no ports is not a link.
    const silent = container('qbittorrent', { networks: undefined, ports: undefined });
    expect(resolveMapConnections([vpn, silent]).links).toEqual([]);
  });

  it('draws nothing when the reference matches no container on the host', () => {
    // What a dependent looks like after the VPN container was recreated.
    const orphan = container('qbittorrent', { networkMode: `container:${'f'.repeat(64)}` });
    expect(resolveMapConnections([vpn, orphan]).links).toEqual([]);
  });

  it('does not reach a container with the same ID on another host', () => {
    const elsewhere = container('gluetun', { containerId: vpn.docker?.containerId }, 'other');
    const dependent = container(
      'qbittorrent',
      { networkMode: `container:${vpn.docker?.containerId}` },
      'third',
    );
    expect(resolveMapConnections([elsewhere, dependent]).links).toEqual([]);
  });

  it('accepts a short ID only when one container on the host has it', () => {
    const short = vpn.docker?.containerId?.slice(0, 12);
    const dependent = container('qbittorrent', { networkMode: `container:${short}` });
    expect(resolveMapConnections([vpn, dependent]).links).toHaveLength(1);

    const twin = container('gluetun-old', { containerId: `${short}${'9'.repeat(52)}` });
    expect(resolveMapConnections([vpn, twin, dependent]).links).toEqual([]);
  });

  it('ignores host, none and named network modes', () => {
    const { links } = resolveMapConnections([
      vpn,
      container('exporter', { networkMode: 'host' }),
      container('job', { networkMode: 'none' }),
      container('app', { networkMode: 'media_net' }),
      container('odd', { networkMode: 'container:gluetun' }),
    ]);
    expect(links).toEqual([]);
  });
});

describe('Traefik router labels', () => {
  it('reads the hostnames a rule names', () => {
    expect(
      traefikRouteHosts({
        'traefik.enable': 'true',
        'traefik.http.routers.app.rule': 'Host(`App.Example.Test`) && PathPrefix(`/api`)',
        'traefik.http.routers.app-alt.rule': 'Host(`alt.example.test`, `app.example.test`)',
        'traefik.tcp.routers.db.rule': 'HostSNI(`db.example.test`)',
        'traefik.http.services.app.loadbalancer.server.port': '8080',
      }),
    ).toEqual({
      routed: true,
      hosts: ['alt.example.test', 'app.example.test', 'db.example.test'],
    });
  });

  it('counts a rule with no hostname as a route with nothing to name', () => {
    expect(
      traefikRouteHosts({
        'traefik.http.routers.any.rule': 'PathPrefix(`/metrics`)',
        'traefik.tcp.routers.raw.rule': 'HostSNI(`*`)',
        'traefik.http.routers.pattern.rule': 'HostRegexp(`{sub:[a-z]+}.example.test`)',
      }),
    ).toEqual({ routed: true, hosts: [] });
  });

  it('reads nothing from a container that opted out or has no router rule', () => {
    expect(
      traefikRouteHosts({
        'traefik.enable': 'False',
        'traefik.http.routers.app.rule': 'Host(`app.example.test`)',
      }),
    ).toEqual({ routed: false, hosts: [] });
    expect(traefikRouteHosts({ 'traefik.enable': 'true' })).toEqual({ routed: false, hosts: [] });
    expect(traefikRouteHosts(undefined)).toEqual({ routed: false, hosts: [] });
  });

  const routed = (name: string, host: string, dockerHost = 'tower') =>
    container(
      name,
      {
        labels: {
          'traefik.enable': 'true',
          [`traefik.http.routers.${name}.rule`]: `Host(\`${host}\`)`,
        },
      },
      dockerHost,
    );
  const traefik = (dockerHost = 'tower', docker: NonNullable<Resource['docker']> = {}) =>
    container('traefik', { image: 'traefik:v3.1', ...docker }, dockerHost);

  it('links the proxy on the same Docker host to each container it routes', () => {
    const { links } = resolveMapConnections([
      traefik(),
      routed('jellyfin', 'media.example.test'),
      container('whoami', {
        labels: {
          'traefik.http.routers.whoami.rule': 'Host(`a.example.test`) || Host(`b.example.test`)',
        },
      }),
      container('redis'),
    ]);

    expect(links).toEqual([
      {
        kind: 'route',
        fromId: 'c-tower-traefik',
        toId: 'c-tower-jellyfin',
        hosts: ['media.example.test'],
      },
      {
        kind: 'route',
        fromId: 'c-tower-traefik',
        toId: 'c-tower-whoami',
        hosts: ['a.example.test', 'b.example.test'],
      },
    ]);
    expect(links.map((link) => mapRouteLabel(link.hosts))).toEqual([
      'media.example.test',
      'a.example.test +1',
    ]);
    expect(mapRouteLabel([])).toBe('');
  });

  it('knows the proxy by the image it runs, not by a name or a publisher', () => {
    // Seen on a real host: Traefik's test server is published as
    // `traefik/whoami`, and it is not a proxy.
    const whoami = container('whoami', {
      image: 'traefik/whoami',
      labels: { 'traefik.http.routers.whoami.rule': 'Host(`who.example.test`)' },
    });
    const named = container('traefik', { image: 'example/not-a-proxy:1' });
    expect(resolveMapConnections([whoami, named]).links).toEqual([]);

    const mirrored = container('edge', {
      image: 'registry.example.test:5000/mirror/traefik@sha256:0123',
    });
    expect(resolveMapConnections([whoami, named, mirrored]).links).toEqual([
      {
        kind: 'route',
        fromId: 'c-tower-edge',
        toId: 'c-tower-whoami',
        hosts: ['who.example.test'],
      },
    ]);
  });

  it('does not link the proxy to itself for its own dashboard route', () => {
    const proxy = traefik('tower', {
      labels: { 'traefik.http.routers.dashboard.rule': 'Host(`traefik.example.test`)' },
    });
    expect(resolveMapConnections([proxy]).links).toEqual([]);
  });

  it('needs a Docker network the proxy and the container are both on', () => {
    // Seen on a real host: a second stack's containers carried router labels
    // for a Traefik of their own that was stopped, on a network the one
    // running Traefik was not attached to.
    const elsewhere = container('portal', {
      networks: [{ name: 'trial-rig' }],
      labels: { 'traefik.http.routers.portal.rule': 'Host(`portal.example.test`)' },
    });
    expect(resolveMapConnections([traefik(), elsewhere]).links).toEqual([]);

    // Two proxies, each on its own network: each routes what it can reach.
    const edge = container('edge', { image: 'traefik:v3.1', networks: [{ name: 'trial-rig' }] });
    expect(
      resolveMapConnections([traefik(), edge, elsewhere, routed('jellyfin', 'media.example.test')])
        .links,
    ).toEqual([
      {
        kind: 'route',
        fromId: 'c-tower-traefik',
        toId: 'c-tower-jellyfin',
        hosts: ['media.example.test'],
      },
      {
        kind: 'route',
        fromId: 'c-tower-edge',
        toId: 'c-tower-portal',
        hosts: ['portal.example.test'],
      },
    ]);

    // A proxy on the host network reaches every container on the host.
    const onHost = traefik('tower', { networkMode: 'host', networks: [{ name: 'host' }] });
    expect(resolveMapConnections([onHost, elsewhere]).links).toHaveLength(1);
  });

  it('reaches a container inside another container network where that one is', () => {
    const vpn = container('gluetun', { networks: [{ name: 'proxy' }] });
    const client = container('qbittorrent', {
      networkMode: `container:${vpn.docker?.containerId}`,
      networks: undefined,
      labels: { 'traefik.http.routers.qbit.rule': 'Host(`torrent.example.test`)' },
    });
    const proxy = traefik('tower', { networks: [{ name: 'proxy' }] });

    expect(resolveMapConnections([vpn, client, proxy]).links).toEqual([
      { kind: 'network', fromId: 'c-tower-qbittorrent', toId: 'c-tower-gluetun', hosts: [] },
      {
        kind: 'route',
        fromId: 'c-tower-traefik',
        toId: 'c-tower-qbittorrent',
        hosts: ['torrent.example.test'],
      },
    ]);
    // Without the mode nothing says where the client is, so nothing routes.
    const silent = { ...client, docker: { ...client.docker, networkMode: undefined } } as Resource;
    expect(resolveMapConnections([vpn, silent, proxy]).links).toEqual([]);
  });

  it('draws nothing when the host has no running Traefik, or more than one', () => {
    const app = routed('jellyfin', 'media.example.test');
    expect(resolveMapConnections([app]).links).toEqual([]);
    expect(
      resolveMapConnections([traefik('tower', { containerState: 'exited' }), app]).links,
    ).toEqual([]);
    expect(
      resolveMapConnections([
        traefik(),
        container('traefik-internal', { image: 'traefik:v3.1' }),
        app,
      ]).links,
    ).toEqual([]);
    // A proxy on another Docker host does not see this host's labels.
    expect(resolveMapConnections([traefik('other'), app]).links).toEqual([]);
  });
});

describe('Caddy Docker Proxy labels', () => {
  it('parses hostnames across caddy and numbered labels while ignoring wildcards, catch-alls, paths and bare IPs', () => {
    expect(
      caddyRouteHosts({
        caddy: 'App.Example.Com',
        caddy_0: 'https://a.example.com, b.example.com:443 http://app.example.com:8080',
        caddy_1:
          '*.example.com, :80, https://:443, /health, app.example.com/api, 192.168.1.5, [::1]:443',
        'caddy.reverse_proxy': '{{upstreams 8080}}',
      }),
    ).toEqual({
      routed: true,
      hosts: ['app.example.com', 'a.example.com', 'b.example.com'],
    });

    expect(
      caddyRouteHosts({
        caddy: ':80, *.example.com, /metrics, 10.0.0.1',
        'caddy.reverse_proxy': '{{upstreams 80}}',
      }),
    ).toEqual({ routed: false, hosts: [] });
    expect(caddyRouteHosts(undefined)).toEqual({ routed: false, hosts: [] });
  });

  const caddy = (
    name = 'caddy',
    docker: NonNullable<Resource['docker']> = {},
    dockerHost = 'tower',
  ) =>
    container(
      name,
      {
        image: 'lucaslorentz/caddy-docker-proxy:2.9-alpine',
        networks: [{ name: 'proxy' }],
        ...docker,
      },
      dockerHost,
    );

  it('links a unique reachable running Caddy or caddy-docker-proxy container to routed containers', () => {
    const app = container('immich', {
      networks: [{ name: 'proxy' }],
      labels: {
        caddy: 'photos.example.test',
        'caddy.reverse_proxy': '{{upstreams 2283}}',
      },
    });
    const isolated = container('isolated', {
      networks: [{ name: 'backend' }],
      labels: { caddy: 'secret.example.test' },
    });
    expect(resolveMapConnections([caddy(), app, isolated]).links).toEqual([
      {
        kind: 'route',
        fromId: 'c-tower-caddy',
        toId: 'c-tower-immich',
        hosts: ['photos.example.test'],
      },
    ]);

    // Standard `caddy` image on host network reaches every container on the host.
    const hostCaddy = caddy('caddy-host', {
      image: 'docker.io/library/caddy:2-alpine',
      networkMode: 'host',
      networks: [{ name: 'host' }],
    });
    expect(resolveMapConnections([hostCaddy, isolated]).links).toEqual([
      {
        kind: 'route',
        fromId: 'c-tower-caddy-host',
        toId: 'c-tower-isolated',
        hosts: ['secret.example.test'],
      },
    ]);
  });

  it('does not link stopped, self, cross-host or ambiguous Caddy proxies', () => {
    const app = container('immich', {
      networks: [{ name: 'proxy' }],
      labels: { caddy: 'photos.example.test' },
    });
    expect(
      resolveMapConnections([caddy('caddy', { containerState: 'exited' }), app]).links,
    ).toEqual([]);
    expect(resolveMapConnections([caddy('caddy', {}, 'other-host'), app]).links).toEqual([]);
    expect(resolveMapConnections([caddy('caddy-a'), caddy('caddy-b'), app]).links).toEqual([]);
    expect(
      resolveMapConnections([caddy('caddy', { labels: { caddy: 'proxy.example.test' } })]).links,
    ).toEqual([]);
  });

  it('matches REST-normalized online proxy containers when containerState is absent', () => {
    const onlineCaddy: Resource = {
      ...caddy('caddy'),
      status: 'online',
      docker: {
        ...caddy('caddy').docker,
        containerState: undefined,
      },
    };
    const app = container('outline', {
      networks: [{ name: 'proxy' }],
      labels: { caddy: 'notes.example.test' },
    });
    expect(resolveMapConnections([onlineCaddy, app]).links).toEqual([
      {
        kind: 'route',
        fromId: 'c-tower-caddy',
        toId: 'c-tower-outline',
        hosts: ['notes.example.test'],
      },
    ]);
  });
});

describe('default gateways and resolvers', () => {
  it('groups machines by the gateway their default route names', () => {
    const { gateways } = resolveMapConnections([
      machine('delly', { defaultGateways: [{ address: '192.168.0.1', interface: 'vmbr0' }] }),
      machine('tower', {
        // The lowest metric comes first and is the one in use.
        defaultGateways: [
          { address: '192.168.0.1', interface: 'eth0' },
          { address: '10.9.0.1', interface: 'wlan0' },
        ],
      }),
      machine('lab', { defaultGateways: [{ address: '10.20.0.1' }] }),
      machine('mac-mini', {}),
    ]);

    expect(gateways).toEqual([
      { address: '192.168.0.1', machineIds: ['agent-delly', 'agent-tower'] },
      { address: '10.20.0.1', machineIds: ['agent-lab'] },
    ]);
  });

  it('groups machines by resolver and leaves out a resolver on loopback', () => {
    const { resolvers } = resolveMapConnections([
      machine('delly', { dnsServers: ['100.100.100.100', 'fd7a:115c:a1e0::53'] }),
      machine('tower', { dnsServers: ['192.168.0.53', '100.100.100.100'] }),
      machine('pihole-host', { dnsServers: ['127.0.0.1', '::1'] }),
    ]);

    expect(resolvers).toEqual([
      { address: '100.100.100.100', machineIds: ['agent-delly', 'agent-tower'] },
      { address: '192.168.0.53', machineIds: ['agent-tower'] },
      { address: 'fd7a:115c:a1e0::53', machineIds: ['agent-delly'] },
    ]);
  });

  it('reports nothing for an estate whose agents state none of it', () => {
    expect(
      resolveMapConnections([
        machine('older', { networkInterfaces: [{ name: 'eth0', addresses: ['192.168.0.9/24'] }] }),
        container('sonarr'),
      ]),
    ).toEqual({ links: [], gateways: [], resolvers: [] });
  });
});

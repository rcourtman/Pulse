import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { createSignal, type JSX } from 'solid-js';
import { createStore } from 'solid-js/store';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { DockerContainersTable } from '../DockerContainersTable';
import { DockerHostsTable } from '../DockerHostsTable';
import {
  DockerHostIdentitiesProvider,
  buildDockerHostIdentities,
  dockerResourceMatchesHostScope,
  getDockerHostRowName,
} from '../dockerHostIdentity';

vi.mock('@/api/monitoring', () => ({ MonitoringAPI: {} }));

vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));

vi.mock('@/stores/containerUpdates', () => ({
  clearContainerUpdateState: vi.fn(),
  getContainerUpdateState: vi.fn(() => undefined),
  markContainerQueued: vi.fn(),
  markContainerUpdateError: vi.fn(),
  markContainerUpdateSuccess: vi.fn(),
  updateStates: vi.fn(() => ({})),
}));

vi.mock('@/stores/systemSettings', () => ({
  areSystemSettingsLoaded: () => true,
  shouldHideDockerUpdateActions: () => false,
}));

vi.mock('@/contexts/appRuntime', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/contexts/appRuntime')>()),
  useWebSocket: () => ({ activeAlerts: {} }),
}));

vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: () => <div />,
}));

vi.mock('@/components/Workloads/StackedMemoryBar', () => ({
  StackedMemoryBar: () => <div />,
}));

const makeHost = (
  sourceId: string,
  overrides: Partial<Resource> & { docker?: Partial<NonNullable<Resource['docker']>> } = {},
): Resource => ({
  id: `agent-${sourceId}`,
  name: 'dock',
  displayName: 'dock',
  platformId: 'dock',
  platformType: 'docker',
  sourceType: 'agent',
  status: 'online',
  type: 'docker-host',
  lastSeen: Date.now(),
  ...overrides,
  docker: { hostSourceId: sourceId, hostname: 'dock', ...overrides.docker },
});

const makeContainer = (id: string, sourceId: string, hostname = 'dock'): Resource => ({
  id,
  name: id,
  displayName: id,
  platformId: 'docker-1',
  platformType: 'docker',
  sourceType: 'agent',
  status: 'running',
  type: 'app-container',
  lastSeen: Date.now(),
  docker: {
    hostSourceId: sourceId,
    hostname,
    containerState: 'running',
    image: `${id}:latest`,
  },
});

const PROBE_LAB = 'proxmox-lxc-docker:lab:pve1:200';
const PROBE_SITE = 'proxmox-lxc-docker:site:pve2:300';

afterEach(() => {
  cleanup();
  window.history.pushState({}, '', '/');
  window.localStorage.clear();
});

describe('buildDockerHostIdentities', () => {
  it('leaves a unique hostname as the key and the words', () => {
    const host = makeHost('agent-a', { docker: { hostname: 'tower' } });
    const identities = buildDockerHostIdentities([host, makeHost('agent-b')]);

    expect(identities.scopeFor(host)).toBe('tower');
    expect(identities.qualifierFor(host)).toBeNull();
    expect(identities.labelFor(host)).toBe('tower');
    expect(identities.scopeFor(makeContainer('web', 'agent-a', 'tower'))).toBe('tower');
  });

  it('keys hosts sharing a hostname on their IDs, and their containers with them', () => {
    const lab = makeHost(PROBE_LAB);
    const site = makeHost(PROBE_SITE);
    const identities = buildDockerHostIdentities([lab, site]);

    expect(identities.scopeFor(lab)).toBe(PROBE_LAB);
    expect(identities.scopeFor(site)).toBe(PROBE_SITE);
    expect(identities.scopeFor(makeContainer('redis', PROBE_SITE))).toBe(PROBE_SITE);
    expect(identities.labelFor(makeContainer('redis', PROBE_SITE))).toBe('dock (site)');
  });

  it('names the connection when one node name and VMID repeat across installations', () => {
    const hetzner = makeHost('proxmox-lxc-docker:Hetzner pmx1:pmx1:101');
    const home = makeHost('proxmox-lxc-docker:Home:pmx1:101');
    const identities = buildDockerHostIdentities([hetzner, home]);

    expect(identities.qualifierFor(hetzner)).toBe('Hetzner pmx1');
    expect(identities.qualifierFor(home)).toBe('Home');
  });

  it('names the VMID when both guests are on one connection', () => {
    const first = makeHost('proxmox-lxc-docker:lab:pve1:201');
    const second = makeHost('proxmox-lxc-docker:lab:pve2:202');
    const identities = buildDockerHostIdentities([first, second]);

    expect(identities.qualifierFor(first)).toBe('LXC 201');
    expect(identities.qualifierFor(second)).toBe('LXC 202');
  });

  it("keeps a connection name that holds colons, as an endpoint's default name does", () => {
    const first = makeHost('proxmox-lxc-docker:https://10.0.0.1:8006:pve1:101');
    const second = makeHost('proxmox-lxc-docker:https://10.0.0.2:8006:pve1:101');

    expect(buildDockerHostIdentities([first, second]).qualifierFor(first)).toBe(
      'https://10.0.0.1:8006',
    );
  });

  it('reads a moved guest from its agent ID, since the row ID can lag', () => {
    const moved = makeHost('agent-stale-row', { docker: { agentId: PROBE_LAB } });
    const other = makeHost(PROBE_SITE);

    expect(buildDockerHostIdentities([moved, other]).qualifierFor(moved)).toBe('lab');
  });

  it('falls from the primary address to the machine ID to the host ID', () => {
    const withAddress = (sourceId: string, ip: string, machineId?: string) =>
      makeHost(sourceId, {
        identity: { ips: [ip, '172.17.0.1'] },
        docker: { machineId },
      });

    const home = withAddress('agent-home', '192.168.1.50');
    const hetzner = withAddress('agent-hetzner', '10.20.0.7');
    expect(buildDockerHostIdentities([home, hetzner]).qualifierFor(home)).toBe('192.168.1.50');

    const a = withAddress('agent-a', '192.168.1.50', 'aaaaaaaa11112222');
    const b = withAddress('agent-b', '192.168.1.50', 'bbbbbbbb11112222');
    expect(buildDockerHostIdentities([a, b]).qualifierFor(a)).toBe('aaaaaaaa');

    const bareA = makeHost('agent-a');
    const bareB = makeHost('agent-b');
    expect(buildDockerHostIdentities([bareA, bareB]).qualifierFor(bareB)).toBe('agent-b');
  });

  it('tells a probed guest from an agent host with the same name', () => {
    const probed = makeHost(PROBE_LAB);
    const agent = makeHost('agent-a', { identity: { ips: ['10.20.0.7'] } });
    const identities = buildDockerHostIdentities([probed, agent]);

    expect(identities.qualifierFor(probed)).toBe('lab');
    expect(identities.qualifierFor(agent)).toBe('10.20.0.7');
  });

  it('lets a display name do the telling apart, without repeating it on the row', () => {
    const named = makeHost('agent-a', { name: 'Home' });
    const plain = makeHost('agent-b', { identity: { ips: ['10.20.0.7'] } });
    const identities = buildDockerHostIdentities([named, plain]);

    expect(identities.labelFor(named)).toBe('dock (Home)');
    expect(getDockerHostRowName(named, identities)).toEqual({
      name: 'Home',
      qualifier: null,
      label: 'Home',
    });
    expect(getDockerHostRowName(plain, identities)).toEqual({
      name: 'dock',
      qualifier: '10.20.0.7',
      label: 'dock (10.20.0.7)',
    });
  });
});

describe('dockerResourceMatchesHostScope', () => {
  it('selects one host by ID and every host with a bare hostname', () => {
    const lab = makeContainer('grafana', PROBE_LAB);
    const site = makeContainer('redis', PROBE_SITE);

    expect(dockerResourceMatchesHostScope(lab, PROBE_LAB)).toBe(true);
    expect(dockerResourceMatchesHostScope(site, PROBE_LAB)).toBe(false);
    expect(dockerResourceMatchesHostScope(lab, 'dock')).toBe(true);
    expect(dockerResourceMatchesHostScope(site, 'dock')).toBe(true);
  });
});

describe('same-named Docker hosts on the page', () => {
  const renderContainers = (hosts: Resource[], containers: Resource[]) => {
    const identities = () => buildDockerHostIdentities(hosts);
    return render(() => (
      <Router>
        <Route
          path="/"
          component={
            (() => (
              <DockerHostIdentitiesProvider value={identities}>
                <DockerContainersTable
                  resources={containers}
                  hosts={hosts}
                  emptyIcon={<span />}
                  emptyTitle="No containers"
                  emptyDescription="No containers"
                  showToolbar={false}
                />
              </DockerHostIdentitiesProvider>
            )) as () => JSX.Element
          }
        />
      </Router>
    ));
  };

  const CONTAINERS = [
    makeContainer('grafana', PROBE_LAB),
    makeContainer('loki', PROBE_LAB),
    makeContainer('redis', PROBE_SITE),
  ];

  it("groups each host's containers under its own status", () => {
    const lab = makeHost(PROBE_LAB);
    const site = makeHost(PROBE_SITE, {
      status: 'degraded',
      health: { verdict: 'stale', reasons: [{ code: 'telemetry_stale', detail: '1h' }] },
    } as Partial<Resource>);
    const { container } = renderContainers([site, lab], CONTAINERS);

    const labGroup = container.querySelector<HTMLElement>(
      `tr[data-docker-host-group="${PROBE_LAB}"]`,
    )!;
    const siteGroup = container.querySelector<HTMLElement>(
      `tr[data-docker-host-group="${PROBE_SITE}"]`,
    )!;

    expect(within(labGroup).getByText('lab')).toBeInTheDocument();
    expect(within(labGroup).getByText('2 containers')).toBeInTheDocument();
    expect(labGroup.querySelector('[data-docker-host-group-stale]')).toBeNull();

    expect(within(siteGroup).getByText('site')).toBeInTheDocument();
    expect(within(siteGroup).getByText('1 container')).toBeInTheDocument();
    expect(siteGroup.querySelector('[data-docker-host-group-stale]')).not.toBeNull();

    // The first host found no longer speaks for both, whichever order they come in.
    const headers = [...container.querySelectorAll('[data-docker-host-group]')];
    expect(headers.map((row) => row.getAttribute('data-docker-host-group'))).toEqual([
      PROBE_LAB,
      PROBE_SITE,
    ]);
  });

  it('narrows to one host through its ID in the URL', () => {
    window.history.pushState({}, '', `/?host=${encodeURIComponent(PROBE_SITE)}`);
    const { container } = renderContainers([makeHost(PROBE_LAB), makeHost(PROBE_SITE)], CONTAINERS);

    expect(screen.getByText('redis')).toBeInTheDocument();
    expect(screen.queryByText('grafana')).not.toBeInTheDocument();
    expect(container.querySelectorAll('[data-docker-host-group]')).toHaveLength(1);
  });

  it('still answers an older link naming only the hostname, with both hosts', () => {
    window.history.pushState({}, '', '/?host=dock');
    const { container } = renderContainers([makeHost(PROBE_LAB), makeHost(PROBE_SITE)], CONTAINERS);

    expect(screen.getByText('grafana')).toBeInTheDocument();
    expect(screen.getByText('redis')).toBeInTheDocument();
    expect(container.querySelectorAll('[data-docker-host-group]')).toHaveLength(2);
  });

  it("keeps a group's status dot live while the row stays mounted", () => {
    const [host, setHost] = createStore(makeHost('agent-a', { docker: { hostname: 'tower' } }));
    const [other] = createSignal(makeHost('agent-b', { docker: { hostname: 'nas' } }));
    const { container } = render(() => (
      <Router>
        <Route
          path="/"
          component={
            (() => (
              <DockerContainersTable
                resources={[
                  makeContainer('web', 'agent-a', 'tower'),
                  makeContainer('db', 'agent-b', 'nas'),
                ]}
                hosts={[host, other()]}
                emptyIcon={<span />}
                emptyTitle="No containers"
                emptyDescription="No containers"
                showToolbar={false}
              />
            )) as () => JSX.Element
          }
        />
      </Router>
    ));

    const group = container.querySelector<HTMLElement>('tr[data-docker-host-group="tower"]')!;
    const dotClass = () => group.querySelector('[class*="rounded-full"]')?.className ?? '';
    expect(dotClass()).toContain('emerald');
    setHost('status', 'offline');
    expect(dotClass()).toContain('red');
  });

  it('shows what tells a host row apart, one line beside its name', () => {
    const identities = () => buildDockerHostIdentities([makeHost(PROBE_LAB), makeHost(PROBE_SITE)]);
    render(() => (
      <DockerHostIdentitiesProvider value={identities}>
        <DockerHostsTable
          resources={[makeHost(PROBE_SITE), makeHost(PROBE_LAB)]}
          emptyIcon={<span />}
          emptyTitle="No hosts"
          emptyDescription="No hosts"
          showToolbar={false}
          containerScope={{
            sectionId: 'docker-containers-section',
            isSelected: () => false,
            onShow: vi.fn(),
          }}
        />
      </DockerHostIdentitiesProvider>
    ));

    const buttons = screen.getAllByRole('button', { name: /^Show containers on dock/ });
    expect(buttons.map((button) => button.getAttribute('aria-label'))).toEqual([
      'Show containers on dock (lab)',
      'Show containers on dock (site)',
    ]);
    const row = buttons[0].closest('tr')!;
    expect(row.querySelector('[data-docker-host-qualifier]')).toHaveTextContent('lab');
    expect(buttons[0]).toHaveTextContent(/^dock$/);
  });

  it('adds nothing to a row whose hostname nothing else has', () => {
    const host = makeHost('agent-a', { name: 'tower', docker: { hostname: 'tower' } });
    const identities = () => buildDockerHostIdentities([host]);
    const { container } = render(() => (
      <DockerHostIdentitiesProvider value={identities}>
        <DockerHostsTable
          resources={[host]}
          emptyIcon={<span />}
          emptyTitle="No hosts"
          emptyDescription="No hosts"
          showToolbar={false}
        />
      </DockerHostIdentitiesProvider>
    ));

    expect(container.querySelector('[data-docker-host-qualifier]')).toBeNull();
  });
});

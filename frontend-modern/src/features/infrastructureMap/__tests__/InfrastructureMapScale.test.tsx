import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import {
  fitServiceDiagram,
  flowServiceDiagram,
  layoutServiceDiagram,
} from '../infrastructureMapDiagram';
import type { SceneNode } from '../infrastructureMapLayout';
import { buildInfrastructureMapModel } from '../infrastructureMapModel';
import {
  MAP_INSPECT_ATTRIBUTE,
  MAP_PALETTES,
  renderMapSceneBody,
} from '../infrastructureMapRender';
import {
  buildServiceDiagram,
  collectNetworkTiles,
  countDiagramResources,
} from '../infrastructureMapServices';
import { InfrastructureMapSurface } from '../InfrastructureMapSurface';

const resourcesMock = vi.hoisted(() => ({
  value: [] as Resource[],
  error: undefined as unknown,
  refetch: vi.fn(),
}));
vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    resources: () => resourcesMock.value,
    loading: () => false,
    error: () => resourcesMock.error,
    refetch: resourcesMock.refetch,
  }),
}));
vi.mock('@/contexts/appRuntime', () => ({ useDarkMode: () => () => false }));
vi.mock('@/hooks/useKioskMode', () => ({ useKioskMode: () => () => false }));
vi.mock('@/hooks/useObservedElementWidth', () => ({
  useObservedElementWidth: () => ({ setElement: () => undefined, width: () => 1280 }),
}));
vi.mock('@/api/infrastructureMapActivity', () => ({
  recordInfrastructureMapOpen: vi.fn(),
  recordInfrastructureMapExport: vi.fn(),
}));
vi.mock('@/utils/toast', () => ({ showSuccess: vi.fn(), showError: vi.fn() }));
const tooltipState = vi.hoisted(() => ({
  read: (): { when: boolean; x: number; y: number; direction?: 'up' | 'down' } => ({
    when: false,
    x: 0,
    y: 0,
  }),
}));
vi.mock('@/components/shared/TooltipPortal', () => ({
  TooltipPortal: (props: {
    when: boolean;
    x: number;
    y: number;
    direction?: 'up' | 'down';
    children: unknown;
  }) => {
    tooltipState.read = () => ({
      when: props.when,
      x: props.x,
      y: props.y,
      direction: props.direction,
    });
    return () => (props.when ? props.children : null);
  },
}));

const resource = (partial: Partial<Resource> & Pick<Resource, 'id' | 'type' | 'name'>): Resource =>
  ({
    displayName: partial.name,
    platformId: partial.id,
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: 1,
    ...partial,
  }) as Resource;

const largeEstate = (): Resource[] => {
  const host = resource({
    id: 'pve-01',
    type: 'agent',
    name: 'pve-prod-01',
    agent: {
      networkInterfaces: [{ name: 'vmbr0', addresses: ['10.10.0.11/24'] }],
      defaultGateways: [{ address: '10.10.0.1', interface: 'eth0' }],
    },
  });
  const host2 = resource({
    id: 'pve-02',
    type: 'agent',
    name: 'pve-prod-02',
    agent: {
      networkInterfaces: [{ name: 'vmbr0', addresses: ['10.20.0.11/24'] }],
      defaultGateways: [{ address: '10.20.0.1', interface: 'eth0' }],
    },
  });

  // 70 containers in "other" so +N overflow kicks in even in Compact (limit 48),
  // with a critical resource and a uniquely named target placed near the end.
  const guests: Resource[] = Array.from({ length: 70 }, (_, index) => {
    const padded = String(index + 1).padStart(3, '0');
    const isNeedle = index === 64;
    const isCritical = index === 66;
    const isWarning = index === 67;
    const isStopped = index === 69;
    return resource({
      id: `ct-${padded}`,
      type: 'system-container',
      name: isNeedle
        ? 'zzz-hidden-needle'
        : isCritical
          ? 'zzz-critical-worker'
          : isWarning
            ? 'zzz-warning-worker'
            : `app-worker-${padded}`,
      parentId: 'pve-01',
      status: isCritical ? 'offline' : isWarning ? 'degraded' : isStopped ? 'stopped' : 'running',
      identity: { ips: [`10.10.${Math.floor(index / 10)}.${(index % 10) + 10}`] },
    });
  });

  // Add enough ingress apps so WAYS IN spans the 390px phone width and Internet + gateways stack above it.
  const ingress: Resource[] = Array.from({ length: 7 }, (_, index) =>
    resource({
      id: `ct-traefik-${index + 1}`,
      type: 'system-container',
      name: `traefik-${index + 1}`,
      parentId: 'pve-01',
      status: 'running',
      identity: { ips: [`10.10.0.${index + 5}`] },
    }),
  );

  // Create 8 distinct /24 ranges so the networks bar has 6 visible + "+2 more".
  const netGuests: Resource[] = [];
  for (let subnet = 1; subnet <= 8; subnet += 1) {
    for (let hostIdx = 1; hostIdx <= 2; hostIdx += 1) {
      netGuests.push(
        resource({
          id: `net-${subnet}-${hostIdx}`,
          type: 'vm',
          name: `net${subnet}-node-${hostIdx}`,
          parentId: 'pve-01',
          status: 'running',
          identity: { ips: [`172.16.${subnet}.${hostIdx + 10}`] },
        }),
      );
    }
  }

  return [host, host2, ...ingress, ...guests, ...netGuests];
};

describe('InfrastructureMap scale model and layout', () => {
  it('keeps full uncapped allTiles and filters before capping so hidden overflow items are found', () => {
    const model = buildInfrastructureMapModel(largeEstate());
    const full = buildServiceDiagram(model, { maxTilesPerGroup: 10 });
    const other = full.groups.find((group) => group.key === 'other');

    expect(other).toBeDefined();
    expect(other!.tiles.some((tile) => tile.id === 'other:more')).toBe(true);
    expect(other!.allTiles.length).toBeGreaterThan(60);
    expect(other!.allTiles.some((tile) => tile.name === 'zzz-hidden-needle')).toBe(true);
    expect(other!.allTiles.some((tile) => tile.status === 'stopped')).toBe(true);
    expect(other!.attentionCount).toBe(2);

    // Searching finds the needle even though it was hidden in +N overflow.
    const searched = buildServiceDiagram(model, {
      maxTilesPerGroup: 10,
      filter: { query: 'hidden-needle' },
    });
    const searchedOther = searched.groups.find((group) => group.key === 'other');
    expect(searchedOther?.tiles.map((tile) => tile.name)).toEqual(['zzz-hidden-needle']);
    expect(countDiagramResources(searched)).toBe(1);

    // Attention-only filter returns critical before warning across the full estate.
    const attention = buildServiceDiagram(model, {
      maxTilesPerGroup: 10,
      filter: { attentionOnly: true },
    });
    const attentionOther = attention.groups.find((group) => group.key === 'other');
    expect(attentionOther?.tiles.map((tile) => tile.name)).toEqual([
      'zzz-critical-worker',
      'zzz-warning-worker',
    ]);
    expect(countDiagramResources(attention)).toBe(2);

    // Network member collection works for every network including hidden ones.
    expect(full.networks.length).toBeGreaterThan(6);
    const hiddenNet = full.networks[full.networks.length - 1];
    expect(collectNetworkTiles(full, hiddenNet.id).length).toBeGreaterThan(0);
  });

  it('emits interactive inspect targets for +N overflow tiles, group headers, and network pills', () => {
    const model = buildInfrastructureMapModel(largeEstate());
    const diagram = buildServiceDiagram(model, { maxTilesPerGroup: 10 });
    const scene = layoutServiceDiagram(diagram, { width: 1200, density: 'marks' });

    const hits = scene.nodes.filter(
      (node): node is Extract<SceneNode, { kind: 'hit' }> => node.kind === 'hit',
    );
    const moreHit = hits.find((node) => node.hit.id === 'other:more');
    expect(moreHit?.hit.inspectTarget).toEqual({ kind: 'group', groupKey: 'other' });

    const inspects = scene.nodes.filter(
      (node): node is Extract<SceneNode, { kind: 'inspect' }> => node.kind === 'inspect',
    );
    expect(
      inspects.some((node) => node.target.kind === 'group' && node.target.groupKey === 'other'),
    ).toBe(true);
    expect(inspects.some((node) => node.target.kind === 'network')).toBe(true);
    expect(inspects.some((node) => node.target.kind === 'all-networks')).toBe(true);

    const svgBody = renderMapSceneBody(scene, MAP_PALETTES.light, () => null);
    expect(svgBody).toContain(`${MAP_INSPECT_ATTRIBUTE}="0"`);
    expect(svgBody).toContain('role="button"');
  });

  it('centers top gateways and Internet icon symmetrically on mobile width 390', () => {
    const model = buildInfrastructureMapModel(largeEstate());
    const diagram = buildServiceDiagram(model, { maxTilesPerGroup: 48 });
    const { scene } = flowServiceDiagram(diagram, 390, 'marks');
    const mid = scene.width / 2;

    const gateways = scene.nodes.filter(
      (node): node is Extract<SceneNode, { kind: 'hit' }> =>
        node.kind === 'hit' && node.hit.checkAddress !== undefined,
    );
    expect(gateways).toHaveLength(2);
    const minLeft = Math.min(...gateways.map((node) => node.x));
    const maxRight = Math.max(...gateways.map((node) => node.x + node.w));
    const gatewaysCenter = (minLeft + maxRight) / 2;
    expect(Math.abs(gatewaysCenter - mid)).toBeLessThanOrEqual(1);
  });

  it('draws larger bold monograms in compact mode for readability', () => {
    const model = buildInfrastructureMapModel(largeEstate());
    const diagram = buildServiceDiagram(model, { maxTilesPerGroup: 24 });
    const { scene } = fitServiceDiagram(diagram, { width: 1280, height: 760 }, 'marks');
    const svg = renderMapSceneBody(scene, MAP_PALETTES.dark, () => null);

    // 29px compact icon * 0.45 = 13.05 font-size for two-letter monograms.
    expect(svg).toContain('font-size="13.05" font-weight="700"');
  });
});

describe('InfrastructureMapSurface scale interactions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resourcesMock.value = largeEstate();
    resourcesMock.error = undefined;
    resourcesMock.refetch.mockReset();
  });
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('toggles the clickable N need attention button to filter the map', () => {
    const { container } = render(() => <InfrastructureMapSurface />);
    const attentionBtn = screen.getByTestId('infrastructure-map-attention-filter');
    expect(attentionBtn).toHaveAttribute('aria-pressed', 'false');
    expect(attentionBtn.textContent).toContain('2 need attention');

    fireEvent.click(attentionBtn);

    expect(attentionBtn).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByTestId('infrastructure-map-filter-bar')).toBeInTheDocument();
    expect(container.querySelector('[aria-label^="zzz-critical-worker,"]')).toBeInTheDocument();
    expect(container.querySelector('[aria-label^="zzz-warning-worker,"]')).toBeInTheDocument();
    expect(container.querySelector('[aria-label^="app-worker-001,"]')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }));
    expect(attentionBtn).toHaveAttribute('aria-pressed', 'false');
    expect(screen.queryByTestId('infrastructure-map-filter-bar')).not.toBeInTheDocument();
  });

  it('searches across all resources including items hidden inside +N overflow', () => {
    const { container } = render(() => <InfrastructureMapSurface />);
    expect(container.querySelector('[aria-label^="zzz-hidden-needle,"]')).not.toBeInTheDocument();

    const searchInput = screen.getByTestId('infrastructure-map-search');
    fireEvent.input(searchInput, { target: { value: 'hidden-needle' } });

    expect(container.querySelector('[aria-label^="zzz-hidden-needle,"]')).toBeInTheDocument();
    expect(screen.getByTestId('infrastructure-map-filter-bar').textContent).toContain(
      'Showing 1 of',
    );

    fireEvent.input(searchInput, { target: { value: 'no-such-workload-xyz' } });
    expect(screen.getByText('No matching resources')).toBeInTheDocument();
  });

  it('opens the searchable group drawer when clicking a +N overflow tile and closes on Escape', () => {
    const { container } = render(() => <InfrastructureMapSurface />);
    const overflowTile = container.querySelector('[aria-label^="+"]');
    expect(overflowTile).not.toBeNull();

    fireEvent.click(overflowTile!);

    const drawer = screen.getByTestId('infrastructure-map-drawer');
    expect(drawer).toBeInTheDocument();
    const list = screen.getByTestId('infrastructure-map-drawer-list');
    // Critical resource is sorted first in the drawer.
    expect(list.querySelectorAll('li')[0]?.textContent).toContain('zzz-critical-worker');
    expect(list.textContent).toContain('zzz-hidden-needle');

    // Search inside drawer narrows items.
    const drawerSearch = screen.getByTestId('infrastructure-map-drawer-search');
    fireEvent.input(drawerSearch, { target: { value: 'hidden-needle' } });
    expect(list.querySelectorAll('li')).toHaveLength(1);
    expect(list.querySelector('a')?.getAttribute('href')).toBeTruthy();

    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByTestId('infrastructure-map-drawer')).not.toBeInTheDocument();
  });

  it('opens network inspection drawer from +N more networks pill and filters map by network', () => {
    const { container } = render(() => <InfrastructureMapSurface />);
    const moreNetworks = container.querySelector(
      `[${MAP_INSPECT_ATTRIBUTE}][aria-label^="Inspect all"]`,
    );
    expect(moreNetworks).not.toBeNull();

    fireEvent.click(moreNetworks!);

    const networkList = screen.getByTestId('infrastructure-map-network-list');
    expect(networkList.querySelectorAll('li').length).toBeGreaterThan(6);

    // Click first Filter button in the networks drawer.
    fireEvent.click(networkList.querySelectorAll('button')[1]!);
    expect(screen.queryByTestId('infrastructure-map-drawer')).not.toBeInTheDocument();
    expect(screen.getByTestId('infrastructure-map-filter-bar').textContent).toContain('Network:');
  });

  it('supports zoom in, zoom out, and Fit reset controls', () => {
    render(() => <InfrastructureMapSurface />);
    const resetBtn = screen.getByTestId('infrastructure-map-zoom-reset');
    const zoomIn = screen.getByRole('button', { name: 'Zoom in' });
    const zoomOut = screen.getByRole('button', { name: 'Zoom out' });

    expect(resetBtn.textContent).toBe('100%');
    expect(zoomOut).toBeDisabled();

    fireEvent.click(zoomIn);
    expect(resetBtn.textContent).toBe('125%');
    expect(zoomOut).not.toBeDisabled();

    fireEvent.click(zoomIn);
    expect(resetBtn.textContent).toBe('150%');

    fireEvent.click(screen.getByRole('button', { name: /^Fit$/ }));
    expect(resetBtn.textContent).toBe('100%');
    expect(zoomOut).toBeDisabled();
  });

  it('flips TooltipPortal downward on top tiles so it never overlaps the toolbar', () => {
    const { container } = render(() => <InfrastructureMapSurface />);
    const frame = container.querySelector('[data-testid="infrastructure-map"] > .min-w-0')!;
    vi.spyOn(frame, 'getBoundingClientRect').mockReturnValue({
      top: 180,
      bottom: 800,
      left: 0,
      right: 390,
      width: 390,
      height: 620,
      x: 0,
      y: 180,
      toJSON: () => ({}),
    } as DOMRect);

    const firstHit = container.querySelector('[data-map-hit="0"]')!;
    vi.spyOn(firstHit, 'getBoundingClientRect').mockReturnValue({
      top: 196,
      bottom: 238,
      left: 140,
      right: 182,
      width: 42,
      height: 42,
      x: 140,
      y: 196,
      toJSON: () => ({}),
    } as DOMRect);

    fireEvent.pointerOver(firstHit);
    expect(tooltipState.read()).toEqual({
      when: true,
      x: 161,
      y: 238,
      direction: 'down',
    });

    // Escape dismisses open hover/focus tooltip (WCAG 1.4.13).
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(tooltipState.read().when).toBe(false);
  });

  it('shows an error state with retry when resources fail to load', () => {
    resourcesMock.value = [];
    resourcesMock.error = new Error('HTTP 500');
    render(() => <InfrastructureMapSurface />);

    expect(screen.getByText('Could not load the map')).toBeInTheDocument();
    expect(screen.queryByText('Nothing to draw yet')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Try again' }));
    expect(resourcesMock.refetch).toHaveBeenCalledTimes(1);
  });

  it('keeps accessible zoom label aligned with visible percentage and returns focus after drawer Escape', () => {
    const rafSpy = vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => {
      cb(0);
      return 1;
    });
    try {
      const { container } = render(() => <InfrastructureMapSurface />);
      const resetBtn = screen.getByTestId('infrastructure-map-zoom-reset');
      expect(resetBtn).toHaveAttribute('aria-label', 'Zoom 100%, reset to default');

      const overflowTile = container.querySelector<SVGElement>('[data-map-hit][aria-label^="+"]');
      expect(overflowTile).not.toBeNull();
      overflowTile!.focus();
      fireEvent.keyDown(overflowTile!, { key: 'Enter' });
      expect(screen.getByTestId('infrastructure-map-drawer')).toBeInTheDocument();

      fireEvent.keyDown(document, { key: 'Escape' });
      expect(screen.queryByTestId('infrastructure-map-drawer')).not.toBeInTheDocument();
      expect(document.activeElement).toBe(overflowTile);
    } finally {
      rafSpy.mockRestore();
    }
  });

  it('shows live network metrics, badges, and Traffic sort order inside group and network drawers', () => {
    resourcesMock.value = [
      resource({
        id: 'pve-edge',
        type: 'agent',
        name: 'pve-edge',
        platformType: 'proxmox-pve',
        network: { rxBytes: 1_048_576, txBytes: 1_048_576 },
        agent: {
          networkInterfaces: [
            { name: 'eno1', addresses: ['10.30.0.11/24'], speedMbps: 10000, linkUp: true },
            { name: 'eno2', speedMbps: 1000, linkUp: false },
          ],
        },
      }),
      resource({
        id: 'vm-critical-idle',
        type: 'vm',
        name: 'critical-idle',
        platformType: 'proxmox-pve',
        parentId: 'pve-edge',
        status: 'offline',
        identity: { ips: ['10.30.0.20'] },
        network: { rxBytes: 0, txBytes: 0 },
      }),
      resource({
        id: 'vm-heavy',
        type: 'vm',
        name: 'heavy-talker',
        platformType: 'proxmox-pve',
        parentId: 'pve-edge',
        status: 'running',
        identity: { ips: ['10.30.0.21'] },
        network: { rxBytes: 10_485_760, txBytes: 2_097_152 },
        proxmox: {
          networkInterfaces: [{ name: 'net0', bridge: 'vmbr0', firewall: true }],
        },
      }),
      resource({
        id: 'vm-light',
        type: 'vm',
        name: 'light-talker',
        platformType: 'proxmox-pve',
        parentId: 'pve-edge',
        status: 'running',
        identity: { ips: ['10.30.0.22'] },
        network: { rxBytes: 204_800, txBytes: 102_400 },
      }),
      resource({
        id: 'chk-edge-router',
        type: 'network-endpoint',
        name: 'edge-router',
        platformType: 'availability',
        status: 'online',
        availability: {
          targetKind: 'router',
          address: '10.30.0.1',
          available: true,
          latencyMillis: 4.5,
        },
      }),
    ];

    const { container } = render(() => <InfrastructureMapSurface />);
    fireEvent.click(screen.getByRole('button', { name: 'By role' }));

    // Open the Other group drawer via its header inspect target.
    const otherHeader = container.querySelector(
      `[${MAP_INSPECT_ATTRIBUTE}][aria-label^="Inspect Other"]`,
    );
    expect(otherHeader).not.toBeNull();
    fireEvent.click(otherHeader!);

    const list = screen.getByTestId('infrastructure-map-drawer-list');
    // Default sort is Attention: critical-idle appears before heavy-talker.
    expect(list.querySelectorAll('li')[0]?.textContent).toContain('critical-idle');
    expect(list.textContent).toContain('Net ↓ 10.0 MB/s ↑ 2.00 MB/s');
    expect(list.textContent).toContain('NIC firewall');

    const sortGroup = screen.getByTestId('infrastructure-map-drawer-sort');
    const attentionBtn = sortGroup.querySelectorAll('button')[0]!;
    const trafficBtn = sortGroup.querySelectorAll('button')[1]!;
    expect(attentionBtn).toHaveAttribute('aria-pressed', 'true');
    expect(trafficBtn).toHaveAttribute('aria-pressed', 'false');

    // Switching to Traffic sorts the busiest talker first.
    fireEvent.click(trafficBtn);
    expect(trafficBtn).toHaveAttribute('aria-pressed', 'true');
    expect(list.querySelectorAll('li')[0]?.textContent).toContain('heavy-talker');
    expect(list.querySelectorAll('li')[1]?.textContent).toContain('light-talker');

    // Close and open the Ways in group drawer to verify check latency metric.
    fireEvent.keyDown(document, { key: 'Escape' });
    const waysInHeader = container.querySelector(
      `[${MAP_INSPECT_ATTRIBUTE}][aria-label^="Inspect Ways in"]`,
    );
    expect(waysInHeader).not.toBeNull();
    fireEvent.click(waysInHeader!);
    expect(screen.getByTestId('infrastructure-map-drawer-list').textContent).toContain(
      'Latency 4.5 ms',
    );

    // Close and open the Hardware group drawer to verify link speed and Link down badge.
    fireEvent.keyDown(document, { key: 'Escape' });
    const hardwareHeader = container.querySelector(
      `[${MAP_INSPECT_ATTRIBUTE}][aria-label^="Inspect Hardware"]`,
    );
    expect(hardwareHeader).not.toBeNull();
    fireEvent.click(hardwareHeader!);
    const hardwareList = screen.getByTestId('infrastructure-map-drawer-list');
    expect(hardwareList.textContent).toContain('10G link');
    expect(hardwareList.textContent).toContain('Link down');

    // Close and inspect the network pill to verify aggregate throughput, link speed, and Link down badge.
    fireEvent.keyDown(document, { key: 'Escape' });
    const networkPill = container.querySelector(
      `[${MAP_INSPECT_ATTRIBUTE}][aria-label^="Inspect network "]`,
    );
    expect(networkPill).not.toBeNull();
    fireEvent.click(networkPill!);
    const drawer = screen.getByTestId('infrastructure-map-drawer');
    expect(drawer.textContent).toContain('12.3 MB/s');
    expect(drawer.textContent).toContain('10G');
    expect(drawer.textContent).toContain('Link down');
  });

  it('inspects estate-wide Live traffic drawer with default Traffic sort, Docker networks, probe summaries, and cert badges', () => {
    resourcesMock.value = [
      resource({
        id: 'pve-talker-host',
        type: 'agent',
        name: 'pve-talker-host',
        platformType: 'proxmox-pve',
      }),
      resource({
        id: 'docker-01',
        type: 'docker-host',
        name: 'docker-01',
        platformType: 'docker',
      }),
      resource({
        id: 'ctr-jellyfin',
        type: 'app-container',
        name: 'jellyfin',
        platformType: 'docker',
        parentId: 'docker-01',
        status: 'running',
        network: { rxBytes: 15_728_640, txBytes: 5_242_880 },
        docker: {
          hostSourceId: 'docker-01',
          containerId: 'b'.repeat(64),
          containerState: 'running',
          image: 'jellyfin/jellyfin:latest',
          networkMode: 'bridge',
          networks: [{ name: 'bridge' }, { name: 'frontend' }, { name: 'media' }],
        },
      }),
      resource({
        id: 'vm-degraded-talker',
        type: 'vm',
        name: 'degraded-talker',
        platformType: 'proxmox-pve',
        parentId: 'pve-talker-host',
        status: 'degraded',
        network: { rxBytes: 524_288, txBytes: 524_288 },
      }),
      resource({
        id: 'chk-expired',
        type: 'network-endpoint',
        name: 'portal-check',
        platformType: 'availability',
        status: 'degraded',
        availability: {
          targetKind: 'service',
          address: 'https://portal.example.com',
          protocol: 'https',
          applicationStatusCode: 200,
          available: true,
          latencyMillis: 14,
          certificate: {
            observedAt: '2026-11-27T10:00:00Z',
            notBefore: '2025-01-01T00:00:00Z',
            notAfter: '2026-11-01T00:00:00Z',
            fingerprintSha256: 'def',
            trustStatus: 'expired',
            chainValid: false,
            hostnameValid: true,
            selfSigned: false,
          },
        },
      }),
    ];

    const { container } = render(() => <InfrastructureMapSurface />);

    // Open estate-wide Live traffic drawer via summary pill (20 MB/s + 1 MB/s = 21.0 MB/s).
    const trafficPill = screen.getByTestId('infrastructure-map-traffic-summary');
    expect(trafficPill).toHaveTextContent('21.0 MB/s');
    fireEvent.click(trafficPill);

    const drawer = screen.getByTestId('infrastructure-map-drawer');
    expect(drawer).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Live traffic' })).toBeInTheDocument();
    expect(drawer.textContent).toContain('2 active resources');
    expect(drawer.textContent).toContain('21.0 MB/s total');

    const list = screen.getByTestId('infrastructure-map-drawer-list');
    // Traffic sort is default: jellyfin (20 MB/s) before degraded-talker (1 MB/s).
    expect(list.querySelectorAll('li')[0]?.textContent).toContain('jellyfin');
    expect(list.querySelectorAll('li')[0]?.textContent).toContain('Docker net: frontend, media');
    expect(list.querySelectorAll('li')[1]?.textContent).toContain('degraded-talker');

    // Switching to Attention sort promotes degraded-talker to the top.
    const sortGroup = screen.getByTestId('infrastructure-map-drawer-sort');
    const attentionBtn = sortGroup.querySelectorAll('button')[0]!;
    const trafficBtn = sortGroup.querySelectorAll('button')[1]!;
    expect(trafficBtn).toHaveAttribute('aria-pressed', 'true');
    fireEvent.click(attentionBtn);
    expect(attentionBtn).toHaveAttribute('aria-pressed', 'true');
    expect(list.querySelectorAll('li')[0]?.textContent).toContain('degraded-talker');

    // Inspect Checks drawer to verify combined probeSummary + latency and Cert expired badge.
    fireEvent.keyDown(document, { key: 'Escape' });
    const checksHeader = container.querySelector(
      `[${MAP_INSPECT_ATTRIBUTE}][aria-label^="Inspect Checks"]`,
    );
    expect(checksHeader).not.toBeNull();
    fireEvent.click(checksHeader!);
    const checksList = screen.getByTestId('infrastructure-map-drawer-list');
    expect(checksList.textContent).toContain('HTTPS 200 · 14 ms');
    expect(checksList.textContent).toContain('Cert expired');
  });

  it('filters the map to active talkers from the Live traffic drawer and clears via the filter chip or Show all', () => {
    resourcesMock.value = [
      resource({
        id: 'pve-filter-host',
        type: 'agent',
        name: 'pve-filter-host',
        platformType: 'proxmox-pve',
      }),
      resource({
        id: 'vm-active-one',
        type: 'vm',
        name: 'active-talker',
        platformType: 'proxmox-pve',
        parentId: 'pve-filter-host',
        status: 'running',
        network: { rxBytes: 12_582_912, txBytes: 2_097_152 },
      }),
      resource({
        id: 'vm-idle-one',
        type: 'vm',
        name: 'quiet-vm',
        platformType: 'proxmox-pve',
        parentId: 'pve-filter-host',
        status: 'running',
        network: { rxBytes: 0, txBytes: 0 },
      }),
    ];

    const { container } = render(() => <InfrastructureMapSurface />);
    expect(container.querySelector('[aria-label^="active-talker,"]')).toBeInTheDocument();
    expect(container.querySelector('[aria-label^="quiet-vm,"]')).toBeInTheDocument();

    // Open Live traffic drawer and click Filter map.
    fireEvent.click(screen.getByTestId('infrastructure-map-traffic-summary'));
    expect(screen.getByTestId('infrastructure-map-drawer')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Filter map' }));

    // Drawer closes, Active traffic chip appears, and idle VMs are filtered out.
    expect(screen.queryByTestId('infrastructure-map-drawer')).not.toBeInTheDocument();
    const filterBar = screen.getByTestId('infrastructure-map-filter-bar');
    expect(filterBar.textContent).toContain('Active traffic');
    expect(container.querySelector('[aria-label^="active-talker,"]')).toBeInTheDocument();
    expect(container.querySelector('[aria-label^="quiet-vm,"]')).not.toBeInTheDocument();

    // Clearing the Active traffic chip restores the idle VM.
    fireEvent.click(screen.getByRole('button', { name: 'Clear active traffic filter' }));
    expect(screen.queryByTestId('infrastructure-map-filter-bar')).not.toBeInTheDocument();
    expect(container.querySelector('[aria-label^="quiet-vm,"]')).toBeInTheDocument();

    // Filtering again and clicking Show all inside the Live traffic drawer also restores all items.
    fireEvent.click(screen.getByTestId('infrastructure-map-traffic-summary'));
    fireEvent.click(screen.getByRole('button', { name: 'Filter map' }));
    expect(screen.getByTestId('infrastructure-map-filter-bar').textContent).toContain(
      'Active traffic',
    );
    fireEvent.click(screen.getByTestId('infrastructure-map-traffic-summary'));
    fireEvent.click(screen.getByRole('button', { name: 'Show all' }));
    expect(screen.queryByTestId('infrastructure-map-filter-bar')).not.toBeInTheDocument();
    expect(container.querySelector('[aria-label^="quiet-vm,"]')).toBeInTheDocument();
  });

  it('shows link utilization percentages and Link busy warning badges in drawer rows', () => {
    resourcesMock.value = [
      resource({
        id: 'host-saturated',
        type: 'agent',
        name: 'saturated-node',
        platformType: 'proxmox-pve',
        network: { rxBytes: 110_000_000, txBytes: 10_000_000 },
        agent: {
          networkInterfaces: [
            { name: 'eno1', addresses: ['10.50.0.10/24'], speedMbps: 1000, linkUp: true },
          ],
        },
      }),
      resource({
        id: 'host-normal',
        type: 'agent',
        name: 'moderate-node',
        platformType: 'proxmox-pve',
        network: { rxBytes: 250_000_000, txBytes: 20_000_000 },
        agent: {
          networkInterfaces: [
            { name: 'eno1', addresses: ['10.50.0.11/24'], speedMbps: 10000, linkUp: true },
          ],
        },
      }),
    ];

    const { container } = render(() => <InfrastructureMapSurface />);
    fireEvent.click(screen.getByRole('button', { name: 'By role' }));
    const hardwareHeader = container.querySelector(
      `[${MAP_INSPECT_ATTRIBUTE}][aria-label^="Inspect Hardware"]`,
    );
    expect(hardwareHeader).not.toBeNull();
    fireEvent.click(hardwareHeader!);

    const list = screen.getByTestId('infrastructure-map-drawer-list');
    const items = [...list.querySelectorAll('li')];
    const satRow = items.find((item) => item.textContent?.includes('saturated-node'));
    const modRow = items.find((item) => item.textContent?.includes('moderate-node'));

    expect(satRow?.textContent).toContain('1G link (88%)');
    expect(satRow?.textContent).toContain('Link busy (88%)');
    expect(modRow?.textContent).toContain('10G link (20%)');
    expect(modRow?.textContent).not.toContain('Link busy');
  });
});

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
}));
vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    resources: () => resourcesMock.value,
    loading: () => false,
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
  });
});

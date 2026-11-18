import { MemoryRouter, Route } from '@solidjs/router';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  recordInfrastructureMapExport,
  recordInfrastructureMapOpen,
} from '@/api/infrastructureMapActivity';
import type { Resource } from '@/types/resource';
import { showError, showSuccess } from '@/utils/toast';
import { fitServiceDiagram } from '../infrastructureMapDiagram';
import { buildMapExportSvg, downloadMapImage } from '../infrastructureMapExport';
import { InfrastructureMapSurface } from '../InfrastructureMapSurface';

const resourcesMock = vi.hoisted(() => ({
  value: [] as Resource[],
  read: undefined as (() => Resource[]) | undefined,
}));
vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    resources: () => resourcesMock.read?.() ?? resourcesMock.value,
    loading: () => false,
  }),
}));
vi.mock('@/contexts/appRuntime', () => ({ useDarkMode: () => () => false }));
// jsdom lays nothing out, so the page would never get a width to draw at.
vi.mock('@/hooks/useObservedElementWidth', () => ({
  useObservedElementWidth: () => ({ setElement: () => undefined, width: () => 1200 }),
}));
vi.mock('@/api/infrastructureMapActivity', () => ({
  recordInfrastructureMapOpen: vi.fn(),
  recordInfrastructureMapExport: vi.fn(),
}));
vi.mock('@/utils/toast', () => ({ showSuccess: vi.fn(), showError: vi.fn() }));
// The real download draws to a canvas and saves a file. The SVG builder stays
// real, so a test can read what the saved image would contain.
vi.mock('../infrastructureMapExport', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../infrastructureMapExport')>()),
  downloadMapImage: vi.fn(),
}));
// The real fit, watched: a test reads the room the page offered the drawing.
vi.mock('../infrastructureMapDiagram', async (importOriginal) => {
  const real = await importOriginal<typeof import('../infrastructureMapDiagram')>();
  return { ...real, fitServiceDiagram: vi.fn(real.fitServiceDiagram) };
});

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

// A host and a guest on one reported subnet.
const lab = (): Resource[] => [
  resource({
    id: 'pve',
    type: 'agent',
    name: 'tower-secret-name',
    platformType: 'proxmox-pve',
    agent: { networkInterfaces: [{ name: 'vmbr0', addresses: ['192.168.10.11/24'] }] },
  }),
  resource({
    id: 'vm-lan',
    type: 'vm',
    name: 'payroll-internal',
    platformType: 'proxmox-pve',
    parentId: 'pve',
    status: 'running',
    identity: { ips: ['192.168.10.50'] },
  }),
];

const saveImage = () => fireEvent.click(screen.getByRole('button', { name: 'Save image' }));

describe('InfrastructureMapSurface use counting', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resourcesMock.value = lab();
    vi.mocked(downloadMapImage).mockResolvedValue('png');
  });
  afterEach(cleanup);

  it('counts the page as opened once it is on screen, and passes nothing from the map', () => {
    render(() => <InfrastructureMapSurface />);

    expect(recordInfrastructureMapOpen).toHaveBeenCalledTimes(1);
    expect(vi.mocked(recordInfrastructureMapOpen).mock.calls[0]).toEqual([]);
    expect(recordInfrastructureMapExport).not.toHaveBeenCalled();
  });

  it('counts a saved image', async () => {
    render(() => <InfrastructureMapSurface />);

    saveImage();

    await waitFor(() => expect(showSuccess).toHaveBeenCalled());
    expect(vi.mocked(recordInfrastructureMapExport).mock.calls).toEqual([[{ namesHidden: false }]]);
  });

  it('counts a save with names hidden, and passes only that one fact', async () => {
    render(() => <InfrastructureMapSurface />);

    fireEvent.click(screen.getByRole('button', { name: 'Hide names' }));
    saveImage();

    await waitFor(() => expect(showSuccess).toHaveBeenCalled());
    expect(vi.mocked(recordInfrastructureMapExport).mock.calls).toEqual([[{ namesHidden: true }]]);
    // The image drawn from what was handed to the save has no name or address.
    const [savedModel, savedOptions] = vi.mocked(downloadMapImage).mock.calls[0];
    const { svg } = buildMapExportSvg(savedModel, savedOptions);
    expect(svg).not.toContain('tower-secret-name');
    expect(svg).not.toContain('payroll-internal');
    expect(svg).not.toContain('192.168.10');
  });

  it('does not count an image that could not be saved', async () => {
    vi.mocked(downloadMapImage).mockRejectedValue(new Error('no canvas'));
    render(() => <InfrastructureMapSurface />);

    saveImage();

    await waitFor(() => expect(showError).toHaveBeenCalled());
    expect(recordInfrastructureMapExport).not.toHaveBeenCalled();
  });
});

// jsdom has no ResizeObserver. This one records what is watched and lets a
// test report a resize.
class WatchedBoxes {
  static all: WatchedBoxes[] = [];
  boxes: Element[] = [];
  constructor(private readonly report: () => void) {
    WatchedBoxes.all.push(this);
  }
  observe(box: Element) {
    this.boxes.push(box);
  }
  unobserve() {}
  disconnect() {
    this.boxes = [];
  }
  resize() {
    this.report();
  }
}

describe('InfrastructureMapSurface fit to the window', () => {
  // What the page keeps free under the drawing on a wide window.
  const BELOW = 60;
  const roomOffered = () => vi.mocked(fitServiceDiagram).mock.calls.at(-1)?.[1];
  const pageWatcher = () =>
    WatchedBoxes.all.find((watcher) => watcher.boxes.includes(document.body));

  beforeEach(() => {
    vi.clearAllMocks();
    WatchedBoxes.all = [];
    vi.stubGlobal('ResizeObserver', WatchedBoxes);
    resourcesMock.value = lab();
  });
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('refits when a banner above the page takes room without changing its width', async () => {
    render(() => <InfrastructureMapSurface />);
    expect(roomOffered()).toEqual({ width: 1200, height: window.innerHeight - BELOW });

    // An 82px banner appears above: the map now starts lower in the window.
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
      top: 82,
    } as DOMRect);
    pageWatcher()?.resize();

    await waitFor(() =>
      expect(roomOffered()).toEqual({ width: 1200, height: window.innerHeight - 82 - BELOW }),
    );
  });

  it('gives the room back when the banner is dismissed', async () => {
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
      top: 82,
    } as DOMRect);
    render(() => <InfrastructureMapSurface />);
    expect(roomOffered()).toEqual({ width: 1200, height: window.innerHeight - 82 - BELOW });

    vi.mocked(HTMLElement.prototype.getBoundingClientRect).mockReturnValue({ top: 0 } as DOMRect);
    pageWatcher()?.resize();

    await waitFor(() =>
      expect(roomOffered()).toEqual({ width: 1200, height: window.innerHeight - BELOW }),
    );
  });

  it('stops watching when the page is left', () => {
    const { unmount } = render(() => <InfrastructureMapSurface />);
    const watcher = pageWatcher();
    expect(watcher).toBeDefined();

    unmount();

    expect(watcher?.boxes).toEqual([]);
  });
});

// A Docker host with one container inside another's network, which is the
// one connection between two tiles this estate states.
const behindVpn = (): Resource[] => {
  const hex = (seed: string): string =>
    [...seed]
      .map((char) => char.charCodeAt(0).toString(16).padStart(2, '0'))
      .join('')
      .padEnd(64, '0')
      .slice(0, 64);
  const box = (name: string, docker: NonNullable<Resource['docker']>): Resource =>
    resource({
      id: `c-${name}`,
      type: 'app-container',
      name,
      platformType: 'docker',
      parentId: 'tower',
      status: 'running',
      docker: {
        hostSourceId: 'tower-daemon',
        containerId: hex(name),
        containerState: 'running',
        image: `example/${name}:latest`,
        ...docker,
      },
    });
  return [
    resource({ id: 'tower', type: 'docker-host', name: 'Tower', platformType: 'docker' }),
    box('gluetun', { networkMode: 'bridge', networks: [{ name: 'bridge' }] }),
    box('qbittorrent', { networkMode: `container:${hex('gluetun')}` }),
    box('sonarr', { networkMode: 'bridge', networks: [{ name: 'bridge' }] }),
  ];
};

describe('InfrastructureMapSurface connections', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resourcesMock.value = behindVpn();
  });
  afterEach(() => {
    cleanup();
    resourcesMock.read = undefined;
  });

  const draw = () => render(() => <InfrastructureMapSurface />);
  const tile = (container: HTMLElement, name: string) => {
    const found = container.querySelector(`[aria-label^="${name},"]`);
    if (!found) throw new Error(`no tile for ${name}`);
    return found;
  };
  const rings = (container: HTMLElement) => [
    ...container.querySelectorAll('[data-map-rings] rect'),
  ];
  // The outline of a tile's target, which a ring round that tile repeats.
  const place = (element: Element) => {
    const rect = element.tagName.toLowerCase() === 'rect' ? element : element.querySelector('rect');
    return ['x', 'y', 'width', 'height'].map((name) => rect?.getAttribute(name));
  };

  it('draws an available estate on mount without changing a control', () => {
    const { container } = draw();

    expect(tile(container, 'qbittorrent')).toBeInTheDocument();
    expect(tile(container, 'gluetun')).toBeInTheDocument();
  });

  it('draws the first snapshot and draws again when the SVG remounts', () => {
    const [resources, setResources] = createSignal<Resource[]>([]);
    resourcesMock.read = resources;
    const { container } = render(() => (
      <MemoryRouter>
        <Route path="/" component={InfrastructureMapSurface} />
      </MemoryRouter>
    ));
    expect(screen.queryByRole('group', { name: 'Infrastructure map' })).not.toBeInTheDocument();

    setResources(behindVpn());
    const first = screen.getByRole('group', { name: 'Infrastructure map' });
    expect(tile(container, 'qbittorrent')).toBeInTheDocument();

    setResources([]);
    expect(first).not.toBeInTheDocument();
    setResources(behindVpn());
    expect(screen.getByRole('group', { name: 'Infrastructure map' })).not.toBe(first);
    expect(tile(container, 'qbittorrent')).toBeInTheDocument();
  });

  it('draws no ring until a tile is pointed at, then rings the tile it is joined to', async () => {
    const { container } = draw();
    expect(rings(container)).toHaveLength(0);

    fireEvent.pointerOver(tile(container, 'qbittorrent'));

    expect(rings(container)).toHaveLength(1);
    expect(place(rings(container)[0])).toEqual(place(tile(container, 'gluetun')));
    expect(await screen.findByText('Uses the network of gluetun')).toBeTruthy();

    // A tile with no stated connection rings nothing.
    fireEvent.pointerOver(tile(container, 'sonarr'));
    expect(rings(container)).toHaveLength(0);

    fireEvent.pointerOver(tile(container, 'gluetun'));
    expect(place(rings(container)[0])).toEqual(place(tile(container, 'qbittorrent')));

    fireEvent.pointerLeave(container.querySelector('svg[role="group"]')!);
    expect(rings(container)).toHaveLength(0);
  });

  it('rings the same tile for a keyboard user, while the tile has focus', () => {
    const { container } = draw();

    fireEvent.focusIn(tile(container, 'qbittorrent'));
    expect(rings(container)).toHaveLength(1);
    expect(place(rings(container)[0])).toEqual(place(tile(container, 'gluetun')));

    fireEvent.focusOut(tile(container, 'qbittorrent'));
    expect(rings(container)).toHaveLength(0);
  });

  it('writes the connection under the tile and keeps lines out of the legend', () => {
    const { container } = draw();

    expect(container.querySelector('svg[role="group"]')?.textContent).toContain('via gluetun');
    expect(screen.queryByText('Shared network')).toBeNull();
    expect(screen.queryByText('Proxy route')).toBeNull();
  });
});

import { MemoryRouter, Route } from '@solidjs/router';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  recordInfrastructureMapExport,
  recordInfrastructureMapOpen,
} from '@/api/infrastructureMapActivity';
import { showPressedTooltip } from '@/components/shared/Tooltip';
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
// The shared tooltip a press shows on every device. Watched, so a test can
// read what a tap said.
vi.mock('@/components/shared/Tooltip', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/components/shared/Tooltip')>()),
  showPressedTooltip: vi.fn(),
}));
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

  it('says how a thing gets onto the map, and links the guide that explains it', () => {
    draw();

    expect(screen.getByText(/Apps and network gear are recognised by name\./)).toBeTruthy();
    const guide = screen.getByRole('link', { name: 'How the map works' });
    expect(guide.getAttribute('href')).toBe('/docs/MAP');
  });

  it('writes the connection under the tile and keeps lines out of the legend', () => {
    const { container } = draw();

    expect(container.querySelector('svg[role="group"]')?.textContent).toContain('via gluetun');
    expect(screen.queryByText('Shared network')).toBeNull();
    expect(screen.queryByText('Proxy route')).toBeNull();
  });

  // A press and the click a browser makes of it. jsdom has no PointerEvent,
  // so the press carries its pointer type as a plain property.
  const press = (element: Element, pointerType: string) => {
    const event = new Event('pointerdown', { bubbles: true });
    Object.defineProperty(event, 'pointerType', { value: pointerType });
    element.dispatchEvent(event);
  };
  const tap = (element: Element, pointerType = 'touch'): MouseEvent => {
    press(element, pointerType);
    const click = new MouseEvent('click', { bubbles: true, cancelable: true });
    element.dispatchEvent(click);
    return click;
  };
  const said = () => vi.mocked(showPressedTooltip).mock.calls.map(([text]) => text);

  it('answers a first tap on a touch screen with what the tile is, and does not open it', () => {
    const { container } = draw();
    const qbittorrent = tile(container, 'qbittorrent');
    expect(qbittorrent.hasAttribute('href')).toBe(true);

    const click = tap(qbittorrent);

    // The link was not followed: the tap asked what this is.
    expect(click.defaultPrevented).toBe(true);
    expect(said()).toHaveLength(1);
    const lines = said()[0].split('\n');
    expect(lines[0]).toBe('qbittorrent');
    expect(lines).toContain('Uses the network of gluetun');
    expect(lines.at(-1)).toBe('Tap again to open');
    expect(vi.mocked(showPressedTooltip).mock.calls[0][1]).toBe(qbittorrent);
    // The tapped tile is marked, since no pointer rests on it, and what it
    // is connected to is ringed, as pointing at it does.
    expect(rings(container).map(place)).toEqual([
      place(tile(container, 'qbittorrent')),
      place(tile(container, 'gluetun')),
    ]);
    expect(rings(container)[0].getAttribute('fill')).not.toBe('none');
    expect(rings(container)[1].getAttribute('fill')).toBe('none');
  });

  it('opens the tile on a second tap', () => {
    const { container } = draw();
    const qbittorrent = tile(container, 'qbittorrent');

    tap(qbittorrent);
    const second = tap(qbittorrent);

    expect(second.defaultPrevented).toBe(false);
    expect(said()).toHaveLength(1);
    expect(rings(container)).toHaveLength(0);
  });

  it('ends what a tap showed at the next press elsewhere, and answers a tap on another tile', () => {
    const { container } = draw();

    tap(tile(container, 'qbittorrent'));
    press(document.body, 'touch');
    expect(rings(container)).toHaveLength(0);

    // The tile tapped before is no longer armed: tapping it shows it again.
    expect(tap(tile(container, 'qbittorrent')).defaultPrevented).toBe(true);
    // A tap on a different tile shows that one, and does not open it.
    const other = tap(tile(container, 'gluetun'));
    expect(other.defaultPrevented).toBe(true);
    expect(said().at(-1)?.split('\n')[0]).toBe('gluetun');
    expect(rings(container).map(place)).toEqual([
      place(tile(container, 'gluetun')),
      place(tile(container, 'qbittorrent')),
    ]);
  });

  it('ends what a tap showed when the page is scrolled', () => {
    const { container } = draw();

    tap(tile(container, 'qbittorrent'));
    expect(rings(container)).toHaveLength(2);
    window.dispatchEvent(new Event('scroll'));

    expect(rings(container)).toHaveLength(0);
  });

  describe('a gateway nothing monitors', () => {
    const ADD_CHECK =
      '/settings/monitoring/availability?add=target&targetKind=device&name=Gateway&address=192.168.0.1';
    beforeEach(() => {
      resourcesMock.value = [
        resource({
          id: 'tower',
          type: 'docker-host',
          name: 'Tower',
          platformType: 'docker',
          agent: { defaultGateways: [{ address: '192.168.0.1', interface: 'eth0' }] },
        }),
        ...behindVpn().slice(1),
      ];
    });

    it('opens the form that adds a check for it, with its address filled in', () => {
      const { container } = draw();
      const gateway = tile(container, '192.168.0.1');

      expect(gateway.getAttribute('href')).toBe(ADD_CHECK);
      expect(gateway.getAttribute('aria-label')).toBe(
        '192.168.0.1, Default gateway, not monitored, add a check',
      );
      // A mouse follows the link at once, as on any tile.
      expect(tap(gateway, 'mouse').defaultPrevented).toBe(false);
    });

    it('says on a touch screen what a second tap will do', () => {
      const { container } = draw();
      const gateway = tile(container, '192.168.0.1');

      expect(tap(gateway).defaultPrevented).toBe(true);
      expect(said()[0].split('\n')).toEqual([
        '192.168.0.1',
        'Default gateway, not monitored',
        'Used by Tower',
        'Tap again to add a check',
      ]);
      expect(tap(tile(container, '192.168.0.1')).defaultPrevented).toBe(false);
    });

    it('is no link while names are hidden, when its address is not on the page', () => {
      const { container } = draw();
      fireEvent.click(screen.getByRole('button', { name: 'Hide names' }));

      const gateway = tile(container, 'Gateway');
      expect(gateway.hasAttribute('href')).toBe(false);
      expect(container.innerHTML).not.toContain('192.168.0.1');
    });
  });

  it('opens a tile at once for a mouse, which can hover to read it first', () => {
    const { container } = draw();

    const click = tap(tile(container, 'qbittorrent'), 'mouse');

    expect(click.defaultPrevented).toBe(false);
    expect(said()).toHaveLength(0);
  });
});

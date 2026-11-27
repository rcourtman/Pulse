import { MemoryRouter, Route } from '@solidjs/router';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  recordInfrastructureMapExport,
  recordInfrastructureMapOpen,
} from '@/api/infrastructureMapActivity';
import { showPressedTooltip } from '@/components/shared/Tooltip';
import { syncSessionSettingsCapabilities } from '@/stores/sessionSettingsCapabilities';
import type { SecurityStatusSettingsCapabilities } from '@/types/config';
import type { Resource } from '@/types/resource';
import { showError, showSuccess } from '@/utils/toast';
import { fitServiceDiagram, flowServiceDiagram } from '../infrastructureMapDiagram';
import { buildMapExportSvg, downloadMapImage } from '../infrastructureMapExport';
import {
  INFRASTRUCTURE_MAP_PREFS_KEY,
  InfrastructureMapSurface,
} from '../InfrastructureMapSurface';

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
const kiosk = vi.hoisted(() => ({ on: false }));
vi.mock('@/hooks/useKioskMode', () => ({ useKioskMode: () => () => kiosk.on }));
// jsdom lays nothing out, so the page would never get a width to draw at.
vi.mock('@/hooks/useObservedElementWidth', () => ({
  useObservedElementWidth: () => ({ setElement: () => undefined, width: () => 1200 }),
}));
vi.mock('@/api/infrastructureMapActivity', () => ({
  recordInfrastructureMapOpen: vi.fn(),
  recordInfrastructureMapExport: vi.fn(),
}));
vi.mock('@/utils/toast', () => ({ showSuccess: vi.fn(), showError: vi.fn() }));
// The card a pointed-at or focused tile shows. Watched, so a test can read
// whether it is up and where. What it says is drawn in place, with none of
// the real card's positioning.
const card = vi.hoisted(() => ({
  read: (): { when: boolean; x: number; y: number } => ({ when: false, x: 0, y: 0 }),
}));
vi.mock('@/components/shared/TooltipPortal', () => ({
  TooltipPortal: (props: { when: boolean; x: number; y: number; children: unknown }) => {
    card.read = () => ({ when: props.when, x: props.x, y: props.y });
    return () => (props.when ? props.children : null);
  },
}));
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
  return {
    ...real,
    fitServiceDiagram: vi.fn(real.fitServiceDiagram),
    flowServiceDiagram: vi.fn(real.flowServiceDiagram),
  };
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

// A signed-in session, by what Settings would open for it.
const signedInWith = (granted: Partial<SecurityStatusSettingsCapabilities>) =>
  syncSessionSettingsCapabilities({
    requiresAuth: true,
    settingsCapabilities: granted as SecurityStatusSettingsCapabilities,
  });
const administrator = () => signedInWith({ availabilityRead: true, infrastructureRead: true });

describe('InfrastructureMapSurface use counting', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.removeItem(INFRASTRUCTURE_MAP_PREFS_KEY);
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
    localStorage.removeItem(INFRASTRUCTURE_MAP_PREFS_KEY);
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

  it('leaves room for the navigation bar a window that is not wide keeps at its foot', () => {
    // The bar publishes its height. It is shown up to 1279px wide, not only on a phone.
    document.documentElement.style.setProperty('--pulse-mobile-nav-height', '45px');
    try {
      render(() => <InfrastructureMapSurface />);
      expect(roomOffered()).toEqual({ width: 1200, height: window.innerHeight - BELOW - 45 });
    } finally {
      document.documentElement.style.removeProperty('--pulse-mobile-nav-height');
    }
  });

  it('refits when the bar changes height and nothing else does', async () => {
    render(() => <InfrastructureMapSurface />);
    expect(roomOffered()).toEqual({ width: 1200, height: window.innerHeight - BELOW });

    document.documentElement.style.setProperty('--pulse-mobile-nav-height', '85px');
    try {
      await waitFor(() =>
        expect(roomOffered()).toEqual({ width: 1200, height: window.innerHeight - BELOW - 85 }),
      );
    } finally {
      document.documentElement.style.removeProperty('--pulse-mobile-nav-height');
    }
  });

  it('leaves room for the lines a wrapped legend adds', async () => {
    render(() => <InfrastructureMapSurface />);
    const legend = screen.getByRole('list', { name: 'Legend' });
    // Three lines of 16px text with two 4px gaps.
    Object.defineProperty(legend, 'offsetHeight', { configurable: true, value: 56 });
    WatchedBoxes.all.find((watcher) => watcher.boxes.includes(legend))?.resize();

    await waitFor(() =>
      expect(roomOffered()).toEqual({ width: 1200, height: window.innerHeight - BELOW - 40 }),
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
    localStorage.removeItem(INFRASTRUCTURE_MAP_PREFS_KEY);
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
    ...container.querySelectorAll('[data-map-rings] > rect'),
  ];
  // Where the rings are, in a fixed order.
  const ringed = (container: HTMLElement) => rings(container).map(place).sort();
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

    // What it uses, and the machine it runs on.
    expect(ringed(container)).toEqual(
      [place(tile(container, 'gluetun')), place(tile(container, 'Tower'))].sort(),
    );
    expect(await screen.findByText('Uses the network of gluetun')).toBeTruthy();

    // A tile with no stated connection rings only its machine.
    fireEvent.pointerOver(tile(container, 'sonarr'));
    expect(ringed(container)).toEqual([place(tile(container, 'Tower'))]);

    fireEvent.pointerOver(tile(container, 'gluetun'));
    expect(ringed(container)).toContainEqual(place(tile(container, 'qbittorrent')));

    fireEvent.pointerLeave(container.querySelector('svg[role="group"]')!);
    expect(rings(container)).toHaveLength(0);
  });

  it('rings the same tile for a keyboard user, while the tile has focus', () => {
    const { container } = draw();

    fireEvent.focusIn(tile(container, 'qbittorrent'));
    expect(ringed(container)).toEqual(
      [place(tile(container, 'gluetun')), place(tile(container, 'Tower'))].sort(),
    );

    fireEvent.focusOut(tile(container, 'qbittorrent'));
    expect(rings(container)).toHaveLength(0);
  });

  describe('with a keyboard', () => {
    const tiles = (container: HTMLElement) => [
      ...container.querySelectorAll<SVGElement>('[data-map-hit]'),
    ];
    const stops = (container: HTMLElement) =>
      tiles(container).filter((element) => element.getAttribute('tabindex') === '0');
    // False when the page kept the key for itself.
    const key = (element: Element, name: string, extra: KeyboardEventInit = {}) =>
      fireEvent.keyDown(element, { key: name, ...extra });
    const hint = () => screen.queryByText('Arrow keys move between tiles');

    it('is one stop for the Tab key, the first tile, with every other tile out of its way', () => {
      const { container } = draw();
      const all = tiles(container);
      expect(all.length).toBeGreaterThan(2);
      expect(stops(container)).toEqual([all[0]]);
      expect(all.slice(1).every((element) => element.getAttribute('tabindex') === '-1')).toBe(true);
    });

    it('moves the stop to the tile that takes focus, and keeps it and the focus when the map is drawn again', () => {
      const { container } = draw();
      (tile(container, 'qbittorrent') as SVGElement).focus();
      expect(document.activeElement).toBe(tile(container, 'qbittorrent'));
      expect(stops(container)).toEqual([tile(container, 'qbittorrent')]);

      // The drawing is replaced whole: a new element for the same resource.
      const before = tile(container, 'qbittorrent');
      fireEvent.click(screen.getByRole('button', { name: 'Compact' }));
      expect(tile(container, 'qbittorrent')).not.toBe(before);
      expect(document.activeElement).toBe(tile(container, 'qbittorrent'));
      expect(stops(container)).toEqual([tile(container, 'qbittorrent')]);
      expect(tiles(container).filter((element) => element.hasAttribute('tabindex'))).toHaveLength(
        tiles(container).length,
      );
    });

    it('keeps the focus on the map, on the tile now in its place, when the focused thing is gone', () => {
      const [resources, setResources] = createSignal<Resource[]>(behindVpn());
      resourcesMock.read = resources;
      const { container } = draw();
      const all = tiles(container);
      const focused = tile(container, 'sonarr') as SVGElement;
      const at = all.indexOf(focused);
      focused.focus();
      expect(document.activeElement).toBe(focused);

      setResources(behindVpn().filter((resource) => resource.name !== 'sonarr'));

      const now = tiles(container);
      expect(now.length).toBe(all.length - 1);
      const inItsPlace = now[Math.min(at, now.length - 1)];
      expect(document.activeElement).toBe(inItsPlace);
      expect(stops(container)).toEqual([inItsPlace]);
    });

    it('takes no focus when the map is drawn again while the focus is elsewhere', () => {
      const { container } = draw();
      const button = screen.getByRole('button', { name: 'Compact' });
      button.focus();
      fireEvent.click(button);
      expect(document.activeElement).toBe(button);
      expect(stops(container)).toEqual([tiles(container)[0]]);
    });

    it('moves the focus to another tile on an arrow, Home or End, and keeps the key from the page', () => {
      const { container } = draw();
      const all = tiles(container);
      all[0].focus();
      expect(document.activeElement).toBe(all[0]);

      expect(key(all[0], 'End')).toBe(false);
      expect(document.activeElement).toBe(all[all.length - 1]);
      expect(key(all[all.length - 1], 'Home')).toBe(false);
      expect(document.activeElement).toBe(all[0]);

      expect(key(all[0], 'ArrowRight')).toBe(false);
      expect(document.activeElement).not.toBe(all[0]);
      expect(all).toContain(document.activeElement);
    });

    it('leaves the page a key that moves nowhere, or is held with another', () => {
      const { container } = draw();
      const all = tiles(container);
      all[0].focus();

      expect(key(all[0], 'Home')).toBe(true);
      expect(key(all[0], 'ArrowRight', { ctrlKey: true })).toBe(true);
      expect(key(all[0], 'PageDown')).toBe(true);
      expect(document.activeElement).toBe(all[0]);
    });

    it('says the arrow keys move between tiles, to a keyboard user, until one has been used', () => {
      const { container } = draw();
      const first = tile(container, 'qbittorrent');

      // A pointer is told nothing about keys.
      fireEvent.pointerOver(first);
      expect(hint()).toBeNull();

      fireEvent.keyDown(document.body, { key: 'Tab' });
      fireEvent.focusIn(first);
      expect(hint()).not.toBeNull();

      // Home and End are not what the hint is about.
      key(first, 'End');
      const last = tiles(container)[tiles(container).length - 1];
      fireEvent.focusIn(last);
      expect(hint()).not.toBeNull();

      key(last, 'ArrowLeft');
      fireEvent.focusIn(first);
      expect(hint()).toBeNull();
    });

    it('lets a tap do what it did, whatever key is pressed between the touch and its click', () => {
      const { container } = draw();
      const target = tile(container, 'qbittorrent');
      press(target, 'touch');
      fireEvent.keyDown(document.body, { key: 'Shift' });
      const click = new MouseEvent('click', { bubbles: true, cancelable: true });
      target.dispatchEvent(click);
      // The first tap reads the tile; it does not open it.
      expect(click.defaultPrevented).toBe(true);
      expect(said().at(-1)).toContain('Tap again to open');
    });

    it('gives a map with nothing on it no instructions for moving between tiles', () => {
      resourcesMock.value = [];
      render(() => (
        <MemoryRouter>
          <Route path="/" component={InfrastructureMapSurface} />
        </MemoryRouter>
      ));
      expect(screen.getByText('Nothing to draw yet')).toBeInTheDocument();
      expect(screen.queryByText(/Arrow keys move between tiles\./)).toBeNull();
    });

    it('says nothing about keys to a tile that took focus from a press of the pointer', () => {
      const { container } = draw();
      fireEvent.keyDown(document.body, { key: 'Tab' });
      press(tile(container, 'qbittorrent'), 'mouse');
      fireEvent.focusIn(tile(container, 'qbittorrent'));
      expect(hint()).toBeNull();
    });
  });

  it('says how a thing gets onto the map, and links the guide that explains it', () => {
    draw();

    expect(
      screen.getByText(/Apps are recognised by name, checks are drawn by their type\./),
    ).toBeTruthy();
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
    expect(place(rings(container)[0])).toEqual(place(tile(container, 'qbittorrent')));
    expect(rings(container).slice(1).map(place).sort()).toEqual(
      [place(tile(container, 'gluetun')), place(tile(container, 'Tower'))].sort(),
    );
    expect(rings(container)[0].getAttribute('fill-opacity')).toBe('0.2');
    expect(rings(container)[1].getAttribute('fill-opacity')).toBe('0.06');
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
    expect(place(rings(container)[0])).toEqual(place(tile(container, 'gluetun')));
    expect(rings(container).slice(1).map(place)).toContainEqual(
      place(tile(container, 'qbittorrent')),
    );
  });

  it('ends what a tap showed when the page is scrolled', () => {
    const { container } = draw();

    tap(tile(container, 'qbittorrent'));
    expect(rings(container)).toHaveLength(3);
    window.dispatchEvent(new Event('scroll'));

    expect(rings(container)).toHaveLength(0);
  });

  it("ends a pointer's card when the page scrolls, and keeps the card of the tile with focus", () => {
    const { container } = draw();

    // A pointer was resting on a tile: the tile has moved from under the card.
    fireEvent.pointerOver(tile(container, 'qbittorrent'));
    expect(rings(container)).toHaveLength(2);
    window.dispatchEvent(new Event('scroll'));
    expect(rings(container)).toHaveLength(0);

    expect(card.read().when).toBe(false);

    // The keyboard brought a tile into view: the scroll is how it got there,
    // and the card goes where the tile went.
    const focused = tile(container, 'qbittorrent');
    const active = vi.spyOn(document, 'activeElement', 'get').mockReturnValue(focused);
    const at = (top: number) =>
      ({ left: 100, right: 140, width: 40, top, bottom: top + 40, height: 40 }) as DOMRect;
    const where = vi.spyOn(focused, 'getBoundingClientRect').mockReturnValue(at(300));
    fireEvent.focusIn(focused);
    expect(card.read()).toEqual({ when: true, x: 120, y: 300 });
    where.mockReturnValue(at(210));
    window.dispatchEvent(new Event('scroll'));
    expect(card.read()).toEqual({ when: true, x: 120, y: 210 });
    expect(rings(container)).toHaveLength(2);

    // A scroll of something that does not hold the map moves no tile.
    const menu = document.createElement('div');
    document.body.append(menu);
    where.mockReturnValue(at(-500));
    menu.dispatchEvent(new Event('scroll'));
    expect(card.read()).toEqual({ when: true, x: 120, y: 210 });
    menu.remove();

    // Scrolled off the screen the tile has no card, and gets it back when
    // it returns.
    where.mockReturnValue(at(-90));
    window.dispatchEvent(new Event('scroll'));
    expect(card.read().when).toBe(false);
    expect(rings(container)).toHaveLength(0);
    where.mockReturnValue(at(40));
    window.dispatchEvent(new Event('scroll'));
    expect(card.read()).toEqual({ when: true, x: 120, y: 40 });
    active.mockRestore();
  });

  describe('a gateway nothing monitors', () => {
    const ADD_CHECK =
      '/settings/monitoring/availability?add=target&targetKind=router&name=Gateway&address=192.168.0.1&from=map';
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

    it('offers no check to a session Settings would turn away from the form', () => {
      signedInWith({ availabilityRead: false, infrastructureRead: false });
      try {
        const { container } = draw();
        const gateway = tile(container, '192.168.0.1');

        expect(gateway.hasAttribute('href')).toBe(false);
        expect(gateway.getAttribute('aria-label')).toBe(
          '192.168.0.1, Default gateway, not monitored',
        );
        // A tap still says what the tile is, and promises nothing.
        expect(tap(gateway).defaultPrevented).toBe(true);
        expect(said()[0].split('\n')).toEqual([
          '192.168.0.1',
          'Default gateway, not monitored',
          'Used by Tower',
        ]);
      } finally {
        administrator();
      }
    });

    it('offers no check in kiosk mode, where the shell turns Settings away', () => {
      administrator();
      kiosk.on = true;
      try {
        const { container } = draw();
        const gateway = tile(container, '192.168.0.1');

        expect(gateway.hasAttribute('href')).toBe(false);
        expect(gateway.getAttribute('aria-label')).toBe(
          '192.168.0.1, Default gateway, not monitored',
        );
      } finally {
        kiosk.on = false;
      }
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

describe('InfrastructureMapSurface with nothing to draw', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.removeItem(INFRASTRUCTURE_MAP_PREFS_KEY);
    resourcesMock.read = undefined;
    resourcesMock.value = [];
  });
  afterEach(() => {
    cleanup();
    administrator();
  });

  it('offers the way to add a system to a session that can use it', () => {
    administrator();
    render(() => (
      <MemoryRouter>
        <Route path="/" component={InfrastructureMapSurface} />
      </MemoryRouter>
    ));

    expect(screen.getByText('Nothing to draw yet')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Add a system' })).toHaveAttribute(
      'href',
      '/settings/infrastructure',
    );
  });

  it('offers no way to add a system in kiosk mode', () => {
    administrator();
    kiosk.on = true;
    try {
      render(() => (
        <MemoryRouter>
          <Route path="/" component={InfrastructureMapSurface} />
        </MemoryRouter>
      ));

      expect(screen.getByText('Nothing to draw yet')).toBeInTheDocument();
      expect(screen.queryByRole('link', { name: 'Add a system' })).not.toBeInTheDocument();
    } finally {
      kiosk.on = false;
    }
  });

  it('sends a session that cannot add one to an administrator instead', () => {
    signedInWith({ availabilityRead: false, infrastructureRead: false });
    render(() => (
      <MemoryRouter>
        <Route path="/" component={InfrastructureMapSurface} />
      </MemoryRouter>
    ));

    expect(screen.getByText('Nothing to draw yet')).toBeInTheDocument();
    expect(screen.getByText(/Ask a Pulse administrator to add one\./)).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Add a system' })).not.toBeInTheDocument();
  });
});

describe('InfrastructureMapSurface on a phone', () => {
  const asWindow = (width: number) => vi.stubGlobal('innerWidth', width);
  const lastFlow = () => vi.mocked(flowServiceDiagram).mock.calls.at(-1);
  const pressed = () =>
    screen
      .getByRole('group', { name: 'Map detail' })
      .querySelector('[aria-pressed="true"]')
      ?.getAttribute('aria-label');

  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.removeItem(INFRASTRUCTURE_MAP_PREFS_KEY);
    resourcesMock.read = undefined;
    resourcesMock.value = lab();
  });
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it('draws the map at full size and lets the page scroll, instead of fitting it to the screen', () => {
    asWindow(390);
    render(() => <InfrastructureMapSurface />);

    expect(fitServiceDiagram).not.toHaveBeenCalled();
    // At the width the page has, with no height to fit to.
    expect(lastFlow()?.[1]).toBe(1200);
    const svg = screen.getByRole('group', { name: 'Infrastructure map' });
    expect(svg.getAttribute('viewBox')).toBe(`0 0 1200 ${svg.getAttribute('height')}`);
  });

  it('opens with icons alone on any estate, and writes names when asked', () => {
    asWindow(390);
    render(() => <InfrastructureMapSurface />);

    // This two-thing estate opens with names on a desktop.
    expect(pressed()).toBe('Compact');
    expect(lastFlow()?.[2]).toBe('marks');

    fireEvent.click(screen.getByRole('button', { name: 'Names' }));
    expect(pressed()).toBe('Names');
    expect(lastFlow()?.[2]).toBe('names');
  });

  it('keeps fitting the map to one screen on a tablet and a desktop', () => {
    for (const width of [768, 1440]) {
      vi.clearAllMocks();
      asWindow(width);
      const view = render(() => <InfrastructureMapSurface />);

      expect(flowServiceDiagram, String(width)).not.toHaveBeenCalled();
      expect(fitServiceDiagram, String(width)).toHaveBeenCalled();
      expect(pressed(), String(width)).toBe('Names');
      view.unmount();
    }
  });

  it('changes over when the window is made narrow or wide again', async () => {
    asWindow(1440);
    render(() => <InfrastructureMapSurface />);
    expect(flowServiceDiagram).not.toHaveBeenCalled();

    asWindow(390);
    window.dispatchEvent(new Event('resize'));
    await waitFor(() => expect(flowServiceDiagram).toHaveBeenCalled());

    vi.mocked(fitServiceDiagram).mockClear();
    asWindow(1440);
    window.dispatchEvent(new Event('resize'));
    await waitFor(() => expect(fitServiceDiagram).toHaveBeenCalled());
  });
});

describe('InfrastructureMapSurface live traffic and network telemetry', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.removeItem(INFRASTRUCTURE_MAP_PREFS_KEY);
    resourcesMock.read = undefined;
    resourcesMock.value = [
      resource({
        id: 'pve-net',
        type: 'agent',
        name: 'pve-net',
        platformType: 'proxmox-pve',
        network: { rxBytes: 2_097_152, txBytes: 524_288 },
        agent: {
          networkInterfaces: [
            { name: 'eno1', addresses: ['192.168.10.11/24'], speedMbps: 10000, linkUp: true },
            { name: 'eno2', speedMbps: 1000, linkUp: false },
          ],
        },
      }),
      resource({
        id: 'vm-fw',
        type: 'vm',
        name: 'fw-guest',
        platformType: 'proxmox-pve',
        parentId: 'pve-net',
        status: 'running',
        identity: { ips: ['192.168.10.50'] },
        network: { rxBytes: 400, txBytes: 200 },
        proxmox: {
          networkInterfaces: [{ name: 'net0', bridge: 'vmbr0', firewall: true }],
        },
      }),
      resource({
        id: 'vm-idle',
        type: 'vm',
        name: 'idle-guest',
        platformType: 'proxmox-pve',
        parentId: 'pve-net',
        status: 'running',
        identity: { ips: ['192.168.10.51'] },
        network: { rxBytes: 0, txBytes: 0 },
      }),
      resource({
        id: 'chk-router',
        type: 'network-endpoint',
        name: 'core-router',
        platformType: 'availability',
        status: 'online',
        availability: {
          targetKind: 'router',
          address: '192.168.10.1',
          available: true,
          latencyMillis: 2.4,
        },
      }),
    ];
  });
  afterEach(cleanup);

  const tile = (container: HTMLElement, name: string) => {
    const found = container.querySelector(`[aria-label^="${name},"]`);
    if (!found) throw new Error(`no tile for ${name}`);
    return found;
  };

  it('starts with Live traffic enabled, shows Active traffic in the legend, and toggles cleanly', () => {
    render(() => <InfrastructureMapSurface />);

    const toggle = screen.getByTestId('infrastructure-map-live-traffic');
    expect(toggle).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByTestId('infrastructure-map-legend-traffic')).toHaveTextContent(
      'Active traffic',
    );
    expect(vi.mocked(fitServiceDiagram).mock.calls.at(-1)?.[3]).toEqual({
      liveTraffic: true,
      layoutMode: 'hosts',
    });

    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute('aria-pressed', 'false');
    expect(screen.queryByTestId('infrastructure-map-legend-traffic')).not.toBeInTheDocument();
    expect(vi.mocked(fitServiceDiagram).mock.calls.at(-1)?.[3]).toEqual({
      liveTraffic: false,
      layoutMode: 'hosts',
    });

    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByTestId('infrastructure-map-legend-traffic')).toHaveTextContent(
      'Active traffic',
    );
    expect(vi.mocked(fitServiceDiagram).mock.calls.at(-1)?.[3]).toEqual({
      liveTraffic: true,
      layoutMode: 'hosts',
    });
  });

  it('shows network throughput, check latency, link speed, link-down count, and firewall state in tooltips', async () => {
    const { container } = render(() => <InfrastructureMapSurface />);

    fireEvent.pointerOver(tile(container, 'pve-net'));
    expect(await screen.findByText(/Link 10G/)).toBeInTheDocument();
    expect(screen.getByText(/1 link down/)).toBeInTheDocument();
    expect(screen.getByText(/Net ↓ 2\.00 MB\/s ↑ 512 KB\/s/)).toBeInTheDocument();

    fireEvent.pointerOver(tile(container, 'fw-guest'));
    expect(await screen.findByText(/NIC firewall flag/)).toBeInTheDocument();
    expect(screen.getByText(/Net <1 KB\/s/)).toBeInTheDocument();

    fireEvent.pointerOver(tile(container, 'idle-guest'));
    await waitFor(() =>
      expect(screen.getAllByText('idle-guest').some((el) => el.tagName === 'SPAN')).toBe(true),
    );
    expect(screen.queryByText(/Net /)).not.toBeInTheDocument();

    fireEvent.pointerOver(tile(container, 'core-router'));
    expect(await screen.findByText(/Latency 2\.4 ms/)).toBeInTheDocument();
  });

  it('shows the estate live traffic summary pill, opens the Live traffic drawer, and toggles curved arc flow', async () => {
    resourcesMock.value = [
      ...resourcesMock.value,
      resource({
        id: 'docker-app-host',
        type: 'docker-host',
        name: 'docker-app-host',
        platformType: 'docker',
      }),
      resource({
        id: 'ctr-app',
        type: 'app-container',
        name: 'plex',
        platformType: 'docker',
        parentId: 'docker-app-host',
        status: 'running',
        network: { rxBytes: 5_242_880, txBytes: 1_048_576 },
        docker: {
          hostSourceId: 'docker-app-host',
          containerId: 'a'.repeat(64),
          containerState: 'running',
          image: 'lscr.io/linuxserver/plex:latest',
          networkMode: 'bridge',
          networks: [
            { name: 'bridge' },
            { name: 'media-net' },
            { name: 'proxy-net' },
            { name: 'internal-net' },
          ],
        },
      }),
      resource({
        id: 'chk-web',
        type: 'network-endpoint',
        name: 'app-https',
        platformType: 'availability',
        status: 'online',
        availability: {
          targetKind: 'service',
          address: 'https://app.example.com',
          protocol: 'https',
          applicationStatusCode: 200,
          available: true,
          latencyMillis: 14,
          linkedResourceId: 'ctr-app',
          certificateMonitoring: true,
          certificateExpiryWarningDays: 14,
          certificate: {
            observedAt: new Date().toISOString(),
            notBefore: '2026-01-01T00:00:00Z',
            notAfter: new Date(Date.now() + 5 * 86_400_000).toISOString(),
            fingerprintSha256: 'abc',
            trustStatus: 'trusted',
            chainValid: true,
            hostnameValid: true,
            selfSigned: false,
          },
        },
      }),
    ];

    const { container } = render(() => <InfrastructureMapSurface />);

    // Estate traffic pill appears above threshold and opens the Live traffic drawer.
    const pill = screen.getByTestId('infrastructure-map-traffic-summary');
    expect(pill).toHaveTextContent('6.00 MB/s');

    fireEvent.click(pill);
    const drawer = screen.getByTestId('infrastructure-map-drawer');
    expect(drawer).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Live traffic' })).toBeInTheDocument();
    const list = screen.getByTestId('infrastructure-map-drawer-list');
    // Defaults to Traffic sort with the busiest talker first.
    expect(list.querySelectorAll('li')[0]?.textContent).toContain('plex');
    const sortGroup = screen.getByTestId('infrastructure-map-drawer-sort');
    expect(sortGroup.querySelectorAll('button')[1]).toHaveAttribute('aria-pressed', 'true');
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByTestId('infrastructure-map-drawer')).not.toBeInTheDocument();

    // Hovering a connected tile draws curved connection arcs and live flow overlays.
    fireEvent.pointerOver(tile(container, 'plex'));
    expect(await screen.findByText(/Docker net: internal-net, media-net \+1/)).toBeInTheDocument();
    expect(
      container.querySelectorAll('[data-map-rings] .pulse-map-link-arc').length,
    ).toBeGreaterThan(0);
    expect(
      container.querySelectorAll('[data-map-rings] .pulse-map-link-flow').length,
    ).toBeGreaterThan(0);

    // Toggling Live traffic off removes animated arc flow while keeping static curved arcs.
    fireEvent.click(screen.getByTestId('infrastructure-map-live-traffic'));
    expect(
      container.querySelectorAll('[data-map-rings] .pulse-map-link-arc').length,
    ).toBeGreaterThan(0);
    expect(container.querySelectorAll('[data-map-rings] .pulse-map-link-flow')).toHaveLength(0);

    // Availability check tooltip combines probeSummary + latency and includes cert warning.
    fireEvent.pointerOver(tile(container, 'app-https'));
    expect(await screen.findByText(/HTTPS 200 · 14 ms/, { selector: 'span' })).toBeInTheDocument();
    expect(screen.getByText(/Cert expiring soon/, { selector: 'span' })).toBeInTheDocument();

    // Hide names strips user-defined Docker network names from tooltips.
    fireEvent.click(screen.getByRole('button', { name: 'Hide names' }));
    fireEvent.pointerOver(tile(container, 'Plex'));
    await waitFor(() => expect(screen.queryByText(/Docker net:/)).not.toBeInTheDocument());
  });

  it('hides the estate live traffic summary pill when estate traffic is below threshold', () => {
    resourcesMock.value = [
      resource({
        id: 'pve-quiet',
        type: 'agent',
        name: 'pve-quiet',
        platformType: 'proxmox-pve',
        network: { rxBytes: 1024, txBytes: 512 },
      }),
    ];

    render(() => <InfrastructureMapSurface />);
    expect(screen.queryByTestId('infrastructure-map-traffic-summary')).not.toBeInTheDocument();
  });

  it('highlights member tiles and shows a rich tooltip when hovering a network pill', async () => {
    resourcesMock.value = [
      resource({
        id: 'pve-net',
        type: 'agent',
        name: 'pve-net',
        platformType: 'proxmox-pve',
        network: { rxBytes: 2_097_152, txBytes: 524_288 },
        agent: {
          networkInterfaces: [
            { name: 'eno1', addresses: ['192.168.10.11/24'], speedMbps: 10000, linkUp: true },
            { name: 'eno2', speedMbps: 1000, linkUp: false },
          ],
        },
      }),
      resource({
        id: 'vm-busy',
        type: 'vm',
        name: 'busy-guest',
        platformType: 'proxmox-pve',
        parentId: 'pve-net',
        status: 'running',
        identity: { ips: ['192.168.10.50'] },
        network: { rxBytes: 10_485_760, txBytes: 2_097_152 },
      }),
      resource({
        id: 'vm-peer',
        type: 'vm',
        name: 'peer-guest',
        platformType: 'proxmox-pve',
        parentId: 'pve-net',
        status: 'running',
        identity: { ips: ['192.168.10.51'] },
      }),
    ];

    const { container } = render(() => <InfrastructureMapSurface />);
    const networkPill = container.querySelector(
      '[data-map-inspect][aria-label^="Inspect network "]',
    );
    expect(networkPill).not.toBeNull();

    // Hovering a network pill draws colour-matched rings on member tiles and shows its summary card.
    fireEvent.pointerOver(networkPill!);
    expect(
      container.querySelectorAll('[data-map-rings] .pulse-map-network-ring').length,
    ).toBeGreaterThanOrEqual(2);
    expect(await screen.findByText('Click to inspect or filter')).toBeInTheDocument();
    expect(screen.getByText(/Traffic 12\.0 MB\/s/)).toBeInTheDocument();
    expect(screen.getByText(/10G link/)).toBeInTheDocument();
    expect(screen.getByText(/1 link down/)).toBeInTheDocument();

    // Hovering a tile clears network rings and switches cleanly to the tile tooltip.
    fireEvent.pointerOver(tile(container, 'busy-guest'));
    expect(container.querySelectorAll('[data-map-rings] .pulse-map-network-ring')).toHaveLength(0);
    expect(screen.queryByText('Click to inspect or filter')).not.toBeInTheDocument();
    expect(await screen.findByText(/Net ↓ 10\.0 MB\/s ↑ 2\.00 MB\/s/)).toBeInTheDocument();

    // Hovering the network pill again restores network rings, and Escape dismisses them.
    fireEvent.pointerOver(networkPill!);
    expect(
      container.querySelectorAll('[data-map-rings] .pulse-map-network-ring').length,
    ).toBeGreaterThanOrEqual(2);
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(container.querySelectorAll('[data-map-rings] .pulse-map-network-ring')).toHaveLength(0);
    expect(screen.queryByText('Click to inspect or filter')).not.toBeInTheDocument();

    // Pointer leave also clears network hover rings and tooltip.
    fireEvent.pointerOver(networkPill!);
    expect(
      container.querySelectorAll('[data-map-rings] .pulse-map-network-ring').length,
    ).toBeGreaterThanOrEqual(2);
    fireEvent.pointerLeave(container.querySelector('svg[role="group"]')!);
    expect(container.querySelectorAll('[data-map-rings] .pulse-map-network-ring')).toHaveLength(0);
  });

  it('shows full-duplex link utilization and Link busy warning in tile tooltips', async () => {
    resourcesMock.value = [
      resource({
        id: 'host-sat',
        type: 'agent',
        name: 'sat-host',
        platformType: 'proxmox-pve',
        network: { rxBytes: 110_000_000, txBytes: 5_000_000 },
        agent: {
          networkInterfaces: [
            { name: 'eno1', addresses: ['10.40.0.10/24'], speedMbps: 1000, linkUp: true },
          ],
        },
      }),
      resource({
        id: 'host-mod',
        type: 'agent',
        name: 'mod-host',
        platformType: 'proxmox-pve',
        network: { rxBytes: 50_000_000, txBytes: 250_000_000 },
        agent: {
          networkInterfaces: [
            { name: 'eno1', addresses: ['10.40.0.11/24'], speedMbps: 10000, linkUp: true },
          ],
        },
      }),
      resource({
        id: 'host-idle',
        type: 'agent',
        name: 'idle-host',
        platformType: 'proxmox-pve',
        network: { rxBytes: 10_000, txBytes: 5_000 },
        agent: {
          networkInterfaces: [
            { name: 'eno1', addresses: ['10.40.0.12/24'], speedMbps: 10000, linkUp: true },
          ],
        },
      }),
    ];

    const { container } = render(() => <InfrastructureMapSurface />);

    fireEvent.pointerOver(tile(container, 'sat-host'));
    expect(await screen.findByText(/Link 1G \(88%\) · Link busy/)).toBeInTheDocument();

    fireEvent.pointerOver(tile(container, 'mod-host'));
    expect(await screen.findByText(/Link 10G \(20%\)/)).toBeInTheDocument();
    expect(screen.queryByText(/Link busy/)).not.toBeInTheDocument();

    fireEvent.pointerOver(tile(container, 'idle-host'));
    expect(await screen.findByText(/Link 10G/)).toBeInTheDocument();
    expect(screen.queryByText(/Link 10G \(/)).not.toBeInTheDocument();
  });
});

describe('InfrastructureMapSurface layout toggle, exposed drawer, and enriched tooltips', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.removeItem(INFRASTRUCTURE_MAP_PREFS_KEY);
    resourcesMock.read = undefined;
    vi.mocked(downloadMapImage).mockResolvedValue('png');
    resourcesMock.value = [
      resource({
        id: 'pve-spec',
        type: 'agent',
        name: 'pve-spec',
        platformType: 'proxmox-pve',
        memory: { current: 50, total: 64 * 1024 ** 3, used: 32 * 1024 ** 3 },
        proxmox: { cpus: 16, pveVersion: '8.3.1' },
        agent: {
          networkInterfaces: [
            { name: 'eno1', addresses: ['10.20.0.10/24'], speedMbps: 10000, linkUp: true },
          ],
        },
      }),
      resource({
        id: 'vm-app',
        type: 'vm',
        name: 'app-vm',
        platformType: 'proxmox-pve',
        parentId: 'pve-spec',
        status: 'running',
        identity: { ips: ['10.20.0.50'] },
        proxmox: { vmid: 105 },
      }),
      resource({
        id: 'ct-dns',
        type: 'system-container',
        name: 'dns-ct',
        platformType: 'proxmox-pve',
        parentId: 'pve-spec',
        status: 'running',
        identity: { ips: ['10.20.0.52'] },
        proxmox: { vmid: 102 },
      }),
      resource({
        id: 'pool-nvme',
        type: 'pool',
        name: 'nvme-pool',
        platformType: 'proxmox-pve',
        parentId: 'pve-spec',
        disk: {
          current: 65,
          total: 4 * 1024 ** 4,
          used: Math.round(2.6 * 1024 ** 4),
        },
        storage: { vdevLayout: 'mirror', type: 'zfspool' },
      }),
      resource({
        id: 'docker-exp',
        type: 'docker-host',
        name: 'docker-exp',
        platformType: 'docker',
      }),
      resource({
        id: 'ctr-traefik',
        type: 'app-container',
        name: 'traefik',
        platformType: 'docker',
        parentId: 'docker-exp',
        status: 'running',
        docker: {
          hostSourceId: 'docker-exp',
          containerId: '1'.repeat(64),
          containerState: 'running',
          image: 'traefik:v3',
          networkMode: 'proxy-net',
          networks: [{ name: 'proxy-net' }],
        },
      }),
      resource({
        id: 'ctr-notes',
        type: 'app-container',
        name: 'notes',
        platformType: 'docker',
        parentId: 'docker-exp',
        status: 'running',
        docker: {
          hostSourceId: 'docker-exp',
          containerId: '2'.repeat(64),
          containerState: 'running',
          image: 'example/notes:latest',
          composeProject: 'workspace',
          networkMode: 'proxy-net',
          networks: [{ name: 'proxy-net' }],
          labels: {
            'traefik.http.routers.notes.rule': 'Host(`notes.home.arpa`)',
          },
        },
      }),
      resource({
        id: 'ctr-grafana',
        type: 'app-container',
        name: 'grafana',
        platformType: 'docker',
        parentId: 'docker-exp',
        status: 'running',
        docker: {
          hostSourceId: 'docker-exp',
          containerId: '3'.repeat(64),
          containerState: 'running',
          image: 'grafana/grafana:latest',
          composeProject: 'workspace',
          networkMode: 'bridge',
          networks: [{ name: 'bridge' }],
          ports: [{ publicPort: 3000, privatePort: 3000, protocol: 'tcp' }],
        },
      }),
    ];
  });
  afterEach(cleanup);

  const tile = (container: HTMLElement, name: string) => {
    const found = container.querySelector(`[aria-label^="${name},"]`);
    if (!found) throw new Error(`no tile for ${name}`);
    return found;
  };

  it('defaults to By host layout and passes layoutMode to fitServiceDiagram and downloadMapImage when toggled', async () => {
    render(() => <InfrastructureMapSurface />);

    const layoutGroup = screen.getByTestId('infrastructure-map-layout');
    const byHostButton = screen.getByRole('button', { name: 'By host' });
    const byRoleButton = screen.getByRole('button', { name: 'By role' });
    expect(layoutGroup).toContainElement(byHostButton);
    expect(byHostButton).toHaveAttribute('aria-pressed', 'true');
    expect(byRoleButton).toHaveAttribute('aria-pressed', 'false');
    expect(vi.mocked(fitServiceDiagram).mock.calls.at(-1)?.[3]).toEqual({
      liveTraffic: true,
      layoutMode: 'hosts',
    });

    fireEvent.click(byRoleButton);
    expect(byHostButton).toHaveAttribute('aria-pressed', 'false');
    expect(byRoleButton).toHaveAttribute('aria-pressed', 'true');
    expect(vi.mocked(fitServiceDiagram).mock.calls.at(-1)?.[3]).toEqual({
      liveTraffic: true,
      layoutMode: 'roles',
    });

    saveImage();
    await waitFor(() => expect(showSuccess).toHaveBeenCalledTimes(1));
    expect(vi.mocked(downloadMapImage).mock.calls.at(-1)?.[1]?.layoutMode).toBe('roles');

    fireEvent.click(byHostButton);
    expect(byHostButton).toHaveAttribute('aria-pressed', 'true');
    expect(vi.mocked(fitServiceDiagram).mock.calls.at(-1)?.[3]).toEqual({
      liveTraffic: true,
      layoutMode: 'hosts',
    });

    saveImage();
    await waitFor(() => expect(showSuccess).toHaveBeenCalledTimes(2));
    expect(vi.mocked(downloadMapImage).mock.calls.at(-1)?.[1]?.layoutMode).toBe('hosts');
  });

  it('opens the Published & routed drawer from the exposed summary pill and filters Proxied vs Published ports', async () => {
    render(() => <InfrastructureMapSurface />);

    const exposedPill = screen.getByTestId('infrastructure-map-exposed-summary');
    expect(exposedPill).toHaveTextContent('2 exposed');

    fireEvent.click(exposedPill);
    expect(screen.getByTestId('infrastructure-map-drawer')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Published & routed' })).toBeInTheDocument();

    const list = screen.getByTestId('infrastructure-map-exposed-list');
    expect(list).toHaveTextContent('notes');
    expect(list).toHaveTextContent('notes.home.arpa');
    expect(list).toHaveTextContent('via traefik');
    expect(list).toHaveTextContent('grafana');
    expect(list).toHaveTextContent(':3000');

    const filterGroup = screen.getByTestId('infrastructure-map-exposed-filter');
    const proxiedButton = filterGroup.querySelectorAll('button')[1];
    const portsButton = filterGroup.querySelectorAll('button')[2];
    expect(proxiedButton).toHaveTextContent('Proxied');
    expect(portsButton).toHaveTextContent('Published ports');

    fireEvent.click(proxiedButton);
    expect(proxiedButton).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByTestId('infrastructure-map-exposed-list')).toHaveTextContent('notes');
    expect(screen.getByTestId('infrastructure-map-exposed-list')).not.toHaveTextContent('grafana');

    fireEvent.click(portsButton);
    expect(portsButton).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByTestId('infrastructure-map-exposed-list')).toHaveTextContent('grafana');
    expect(screen.getByTestId('infrastructure-map-exposed-list')).not.toHaveTextContent('notes');

    // Reset to All and search by hostname.
    fireEvent.click(filterGroup.querySelectorAll('button')[0]);
    const search = screen.getByTestId('infrastructure-map-drawer-search');
    fireEvent.input(search, { target: { value: 'notes.home.arpa' } });
    expect(screen.getByTestId('infrastructure-map-exposed-list')).toHaveTextContent('notes');
    expect(screen.getByTestId('infrastructure-map-exposed-list')).not.toHaveTextContent('grafana');

    // Close drawer, enable Hide names, and reopen to confirm hostnames are stripped while ports stay visible.
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByTestId('infrastructure-map-drawer')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Hide names' }));
    fireEvent.click(screen.getByTestId('infrastructure-map-exposed-summary'));
    const hiddenList = screen.getByTestId('infrastructure-map-exposed-list');
    expect(hiddenList).toHaveTextContent(':3000');
    expect(hiddenList).not.toHaveTextContent('notes.home.arpa');
  });

  it('enriches hover tooltips with host hardware specs, VMIDs, Compose stacks, and storage pool usage', async () => {
    const { container } = render(() => <InfrastructureMapSurface />);

    fireEvent.pointerOver(tile(container, 'pve-spec'));
    expect(await screen.findByText(/16 cores · 64 GB RAM · PVE 8\.3/)).toBeInTheDocument();

    fireEvent.pointerOver(tile(container, 'app-vm'));
    expect(await screen.findByText(/VM 105/, { selector: 'span' })).toBeInTheDocument();

    fireEvent.pointerOver(tile(container, 'dns-ct'));
    expect(await screen.findByText(/CT 102/, { selector: 'span' })).toBeInTheDocument();

    fireEvent.pointerOver(tile(container, 'notes'));
    expect(await screen.findByText(/Stack workspace/)).toBeInTheDocument();

    fireEvent.pointerOver(tile(container, 'nvme-pool'));
    expect(await screen.findByText(/mirror · 4 TB · 65% used/)).toBeInTheDocument();
  });

  it('shows the link-speed legend item only when link speeds are reported', () => {
    const view = render(() => <InfrastructureMapSurface />);
    const speedLegend = screen.getByTestId('infrastructure-map-legend-speed');
    expect(speedLegend).toHaveTextContent('10G');
    expect(speedLegend).toHaveTextContent('1G');
    expect(speedLegend).toHaveTextContent('Link speed');
    view.unmount();

    resourcesMock.value = lab();
    render(() => <InfrastructureMapSurface />);
    expect(screen.queryByTestId('infrastructure-map-legend-speed')).not.toBeInTheDocument();
  });

  it('opens the host inspector drawer when clicking a host frame inspect count badge in By host view', () => {
    const { container } = render(() => <InfrastructureMapSurface />);

    const hostInspect = container.querySelector(
      '[data-map-inspect][aria-label="Inspect pve-spec (2)"]',
    );
    expect(hostInspect).not.toBeNull();

    fireEvent.click(hostInspect!);
    expect(screen.getByTestId('infrastructure-map-drawer')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'pve-spec' })).toBeInTheDocument();

    const list = screen.getByTestId('infrastructure-map-drawer-list');
    expect(list).toHaveTextContent('app-vm');
    expect(list).toHaveTextContent('dns-ct');
    expect(list).not.toHaveTextContent('notes');
  });

  it('renders factual relationship pills on hover arcs and strips route hostnames under Hide names', () => {
    resourcesMock.value = [
      ...resourcesMock.value,
      resource({
        id: 'chk-notes',
        type: 'network-endpoint',
        name: 'notes-https',
        platformType: 'availability',
        status: 'online',
        availability: {
          targetKind: 'service',
          address: 'https://notes.home.arpa',
          protocol: 'https',
          applicationStatusCode: 200,
          available: true,
          latencyMillis: 12,
          linkedResourceId: 'ctr-notes',
        },
      }),
    ];

    const { container } = render(() => <InfrastructureMapSurface />);
    const linkLabels = () =>
      [...container.querySelectorAll('[data-map-rings] .pulse-map-link-label')].map(
        (element) => element.textContent?.trim() ?? '',
      );

    fireEvent.pointerOver(tile(container, 'traefik'));
    expect(linkLabels()).toContain('notes.home.arpa');

    fireEvent.pointerOver(tile(container, 'notes-https'));
    expect(linkLabels()).toContain('checks');

    fireEvent.click(screen.getByRole('button', { name: 'Hide names' }));
    fireEvent.pointerOver(tile(container, 'Traefik'));
    expect(linkLabels()).toContain('routes');
    expect(linkLabels().join(' ')).not.toContain('notes.home.arpa');
  });
});

describe('InfrastructureMapSurface spotlight, thermal/disk telemetry, and traffic proportion bars', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.removeItem(INFRASTRUCTURE_MAP_PREFS_KEY);
    resourcesMock.read = undefined;
  });
  afterEach(cleanup);

  const tile = (container: HTMLElement, name: string) => {
    const found = container.querySelector(`[aria-label^="${name},"]`);
    if (!found) throw new Error(`no tile for ${name}`);
    return found;
  };

  it('applies spotlight dimming for linked tiles and network pills, and clears on pointerLeave', () => {
    resourcesMock.value = [
      resource({
        id: 'pve-a',
        type: 'agent',
        name: 'pve-a',
        platformType: 'proxmox-pve',
        agent: {
          networkInterfaces: [
            { name: 'vmbr0', addresses: ['10.30.0.10/24'], speedMbps: 10000, linkUp: true },
          ],
        },
      }),
      resource({
        id: 'vm-alpha',
        type: 'vm',
        name: 'alpha-vm',
        platformType: 'proxmox-pve',
        parentId: 'pve-a',
        status: 'running',
        identity: { ips: ['10.30.0.50'] },
      }),
      resource({
        id: 'pve-b',
        type: 'agent',
        name: 'pve-b',
        platformType: 'proxmox-pve',
        agent: {
          networkInterfaces: [
            { name: 'vmbr1', addresses: ['10.40.0.10/24'], speedMbps: 1000, linkUp: true },
          ],
        },
      }),
      resource({
        id: 'vm-beta',
        type: 'vm',
        name: 'beta-vm',
        platformType: 'proxmox-pve',
        parentId: 'pve-b',
        status: 'running',
        identity: { ips: ['10.40.0.50'] },
      }),
    ];

    const { container } = render(() => <InfrastructureMapSurface />);
    const svg = container.querySelector('svg[role="group"]')!;
    expect(svg.classList.contains('pulse-map-spotlight')).toBe(false);

    // Hovering a linked tile spotlights it and its linked host while dimming unrelated tiles.
    fireEvent.pointerOver(tile(container, 'alpha-vm'));
    expect(svg.classList.contains('pulse-map-spotlight')).toBe(true);
    expect(
      svg
        .querySelector('[data-map-tile="vm-alpha"]')
        ?.classList.contains('pulse-map-spotlight-lit'),
    ).toBe(true);
    expect(
      svg.querySelector('[data-map-tile="pve-a"]')?.classList.contains('pulse-map-spotlight-lit'),
    ).toBe(true);
    expect(
      svg.querySelector('[data-map-tile="vm-beta"]')?.classList.contains('pulse-map-spotlight-lit'),
    ).toBe(false);

    // Pointer leave restores all tiles to full brightness.
    fireEvent.pointerLeave(svg);
    expect(svg.classList.contains('pulse-map-spotlight')).toBe(false);
    expect(svg.querySelectorAll('.pulse-map-spotlight-lit')).toHaveLength(0);

    // Hovering a network pill spotlights all member tiles on that subnet.
    const networkPill = container.querySelector(
      '[data-map-inspect][aria-label^="Inspect network "]',
    );
    expect(networkPill).not.toBeNull();
    fireEvent.pointerOver(networkPill!);
    expect(svg.classList.contains('pulse-map-spotlight')).toBe(true);
    expect(svg.querySelectorAll('.pulse-map-spotlight-lit').length).toBeGreaterThanOrEqual(2);

    fireEvent.pointerLeave(svg);
    expect(svg.classList.contains('pulse-map-spotlight')).toBe(false);
    expect(svg.querySelectorAll('.pulse-map-spotlight-lit')).toHaveLength(0);
  });

  it('displays host temperature and physical disk summary in hover tooltips and drawer rows', async () => {
    resourcesMock.value = [
      resource({
        id: 'pve-therm',
        type: 'agent',
        name: 'pve-therm',
        platformType: 'proxmox-pve',
        proxmox: { cpus: 16, pveVersion: '8.3.1', temperature: 48 },
        agent: {
          networkInterfaces: [
            { name: 'eno1', addresses: ['10.50.0.10/24'], speedMbps: 10000, linkUp: true },
          ],
        },
      }),
      resource({
        id: 'vm-therm',
        type: 'vm',
        name: 'therm-guest',
        platformType: 'proxmox-pve',
        parentId: 'pve-therm',
        status: 'running',
        identity: { ips: ['10.50.0.50'] },
      }),
      resource({
        id: 'disk-nvme-1',
        type: 'physical_disk',
        name: 'nvme0n1',
        platformType: 'proxmox-pve',
        parentId: 'pve-therm',
        physicalDisk: { diskType: 'nvme', temperature: 44, health: 'PASSED' },
      }),
      resource({
        id: 'disk-nvme-2',
        type: 'physical_disk',
        name: 'nvme1n1',
        platformType: 'proxmox-pve',
        parentId: 'pve-therm',
        physicalDisk: { diskType: 'nvme', temperature: 41, health: 'PASSED' },
      }),
      resource({
        id: 'disk-hdd-1',
        type: 'physical_disk',
        name: 'sda',
        platformType: 'proxmox-pve',
        parentId: 'pve-therm',
        physicalDisk: { diskType: 'hdd', temperature: 36, health: 'PASSED' },
      }),
      resource({
        id: 'disk-hdd-2',
        type: 'physical_disk',
        name: 'sdb',
        platformType: 'proxmox-pve',
        parentId: 'pve-therm',
        physicalDisk: { diskType: 'hdd', temperature: 35, health: 'PASSED' },
      }),
      resource({
        id: 'disk-hdd-3',
        type: 'physical_disk',
        name: 'sdc',
        platformType: 'proxmox-pve',
        parentId: 'pve-therm',
        physicalDisk: { diskType: 'hdd', temperature: 37, health: 'PASSED' },
      }),
      resource({
        id: 'disk-hdd-4',
        type: 'physical_disk',
        name: 'sdd',
        platformType: 'proxmox-pve',
        parentId: 'pve-therm',
        physicalDisk: { diskType: 'hdd', temperature: 38, health: 'FAILED' },
      }),
    ];

    const { container } = render(() => <InfrastructureMapSurface />);

    fireEvent.pointerOver(tile(container, 'pve-therm'));
    expect(await screen.findByText(/48°C/, { selector: 'span' })).toBeInTheDocument();
    expect(
      screen.getByText(/Disks: 2× NVMe · 4× HDD · max 44°C · 1 unhealthy/, { selector: 'span' }),
    ).toBeInTheDocument();

    // Open the network drawer to verify thermal and physical disk info in drawer rows.
    const networkPill = container.querySelector(
      '[data-map-inspect][aria-label^="Inspect network "]',
    );
    expect(networkPill).not.toBeNull();
    fireEvent.click(networkPill!);
    const drawerList = screen.getByTestId('infrastructure-map-drawer-list');
    expect(drawerList).toHaveTextContent('48°C');
    expect(drawerList).toHaveTextContent('Disks: 2× NVMe · 4× HDD');
    expect(drawerList).toHaveTextContent('1 disk unhealthy');
  });

  it('renders proportional traffic bars in the Live traffic drawer scaled relative to the top talker', () => {
    resourcesMock.value = [
      resource({
        id: 'pve-flow',
        type: 'agent',
        name: 'pve-flow',
        platformType: 'proxmox-pve',
      }),
      resource({
        id: 'vm-top',
        type: 'vm',
        name: 'top-talker',
        platformType: 'proxmox-pve',
        parentId: 'pve-flow',
        status: 'running',
        network: { rxBytes: 8_000_000, txBytes: 2_000_000 },
      }),
      resource({
        id: 'vm-half',
        type: 'vm',
        name: 'half-talker',
        platformType: 'proxmox-pve',
        parentId: 'pve-flow',
        status: 'running',
        network: { rxBytes: 4_000_000, txBytes: 1_000_000 },
      }),
      resource({
        id: 'vm-trickle',
        type: 'vm',
        name: 'trickle-talker',
        platformType: 'proxmox-pve',
        parentId: 'pve-flow',
        status: 'running',
        network: { rxBytes: 10_000, txBytes: 10_000 },
      }),
    ];

    render(() => <InfrastructureMapSurface />);
    fireEvent.click(screen.getByTestId('infrastructure-map-traffic-summary'));

    const bars = screen.getAllByTestId('infrastructure-map-traffic-bar');
    expect(bars).toHaveLength(3);
    const widths = bars.map((bar) => (bar.firstElementChild as HTMLElement | null)?.style.width);
    expect(widths).toEqual(['100%', '50%', '4%']);
  });
});

describe('InfrastructureMapSurface persisted preferences, networks summary, keyboard shortcuts, and operational telemetry', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.removeItem(INFRASTRUCTURE_MAP_PREFS_KEY);
    resourcesMock.read = undefined;
  });
  afterEach(() => {
    cleanup();
    localStorage.removeItem(INFRASTRUCTURE_MAP_PREFS_KEY);
  });

  const tile = (container: HTMLElement, name: string) => {
    const found = container.querySelector(`[aria-label^="${name},"]`);
    if (!found) throw new Error(`no tile for ${name}`);
    return found;
  };

  it('persists layout mode, density, and live traffic to localStorage and restores on remount without persisting Hide names', () => {
    resourcesMock.value = lab();

    const first = render(() => <InfrastructureMapSurface />);
    fireEvent.click(screen.getByRole('button', { name: 'By role' }));
    fireEvent.click(screen.getByRole('button', { name: 'Compact' }));
    fireEvent.click(screen.getByTestId('infrastructure-map-live-traffic'));
    fireEvent.click(screen.getByRole('button', { name: 'Hide names' }));

    const saved = JSON.parse(localStorage.getItem(INFRASTRUCTURE_MAP_PREFS_KEY) ?? '{}');
    expect(saved).toEqual({
      layoutMode: 'roles',
      density: 'marks',
      liveTraffic: false,
    });
    expect(saved).not.toHaveProperty('namesHidden');

    first.unmount();

    const second = render(() => <InfrastructureMapSurface />);
    expect(screen.getByRole('button', { name: 'By role' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: 'Compact' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByTestId('infrastructure-map-live-traffic')).toHaveAttribute(
      'aria-pressed',
      'false',
    );
    expect(screen.getByRole('button', { name: 'Hide names' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
    second.unmount();

    // Corrupt JSON falls back cleanly to defaults.
    localStorage.setItem(INFRASTRUCTURE_MAP_PREFS_KEY, '{not-valid-json');
    render(() => <InfrastructureMapSurface />);
    expect(screen.getByRole('button', { name: 'By host' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByTestId('infrastructure-map-live-traffic')).toHaveAttribute(
      'aria-pressed',
      'true',
    );
  });

  it('shows the networks summary pill when multiple networks exist, renders proportional traffic bars, and filters in one click', () => {
    resourcesMock.value = lab();
    const singleNet = render(() => <InfrastructureMapSurface />);
    expect(screen.queryByTestId('infrastructure-map-networks-summary')).not.toBeInTheDocument();
    singleNet.unmount();

    resourcesMock.value = [
      resource({
        id: 'pve-lan',
        type: 'agent',
        name: 'pve-lan',
        platformType: 'proxmox-pve',
        agent: {
          networkInterfaces: [
            { name: 'vmbr0', addresses: ['10.10.0.10/24'], speedMbps: 10000, linkUp: true },
          ],
        },
      }),
      resource({
        id: 'vm-lan-busy',
        type: 'vm',
        name: 'lan-busy',
        platformType: 'proxmox-pve',
        parentId: 'pve-lan',
        status: 'running',
        identity: { ips: ['10.10.0.50'] },
        network: { rxBytes: 8_000_000, txBytes: 2_000_000 },
      }),
      resource({
        id: 'pve-iot',
        type: 'agent',
        name: 'pve-iot',
        platformType: 'proxmox-pve',
        agent: {
          networkInterfaces: [
            { name: 'vmbr1', addresses: ['10.20.0.10/24'], speedMbps: 1000, linkUp: true },
          ],
        },
      }),
      resource({
        id: 'vm-iot-light',
        type: 'vm',
        name: 'iot-light',
        platformType: 'proxmox-pve',
        parentId: 'pve-iot',
        status: 'running',
        identity: { ips: ['10.20.0.50'] },
        network: { rxBytes: 2_000_000, txBytes: 500_000 },
      }),
    ];

    render(() => <InfrastructureMapSurface />);

    const summaryPill = screen.getByTestId('infrastructure-map-networks-summary');
    expect(summaryPill).toHaveTextContent('2 networks');

    fireEvent.click(summaryPill);
    expect(screen.getByTestId('infrastructure-map-drawer')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Networks' })).toBeInTheDocument();

    const networkList = screen.getByTestId('infrastructure-map-network-list');
    expect(networkList.querySelectorAll('li')).toHaveLength(2);

    const bars = screen.getAllByTestId('infrastructure-map-network-traffic-bar');
    expect(bars).toHaveLength(2);
    const widths = bars
      .map((bar) => (bar.firstElementChild as HTMLElement | null)?.style.width)
      .sort();
    expect(widths).toEqual(['100%', '25%']);

    const filterButtons = screen.getAllByRole('button', { name: 'Filter' });
    expect(filterButtons).toHaveLength(2);
    fireEvent.click(filterButtons[0]!);

    expect(screen.queryByTestId('infrastructure-map-drawer')).not.toBeInTheDocument();
    expect(screen.getByTestId('infrastructure-map-filter-bar')).toHaveTextContent('Network:');

    // Reopening the networks drawer marks the filtered network as Active.
    fireEvent.click(screen.getByTestId('infrastructure-map-networks-summary'));
    expect(screen.getByRole('button', { name: 'Active' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('supports keyboard shortcuts for search focus and zoom while ignoring keys typed in search input', () => {
    resourcesMock.value = lab();
    render(() => <InfrastructureMapSurface />);

    const searchInput = screen.getByTestId('infrastructure-map-search') as HTMLInputElement;
    const zoomReset = screen.getByTestId('infrastructure-map-zoom-reset');
    expect(searchInput.title).toContain('Press / to focus.');
    expect(zoomReset).toHaveTextContent('100%');

    // '+' and '=' zoom in, '-' zooms out, '0' resets zoom.
    fireEvent.keyDown(document.body, { key: '+' });
    expect(zoomReset).toHaveTextContent('125%');

    fireEvent.keyDown(document.body, { key: '=' });
    expect(zoomReset).toHaveTextContent('150%');

    fireEvent.keyDown(document.body, { key: '-' });
    expect(zoomReset).toHaveTextContent('125%');

    fireEvent.keyDown(document.body, { key: '0' });
    expect(zoomReset).toHaveTextContent('100%');

    // '/' focuses the map search input.
    fireEvent.keyDown(document.body, { key: '/' });
    expect(document.activeElement).toBe(searchInput);

    // Shortcuts typed inside the search input must not alter zoom.
    fireEvent.keyDown(searchInput, { key: '+' });
    fireEvent.keyDown(searchInput, { key: '=' });
    fireEvent.keyDown(searchInput, { key: '-' });
    expect(zoomReset).toHaveTextContent('100%');

    // Modifier-held shortcuts are ignored.
    searchInput.blur();
    fireEvent.keyDown(document.body, { key: '+', ctrlKey: true });
    expect(zoomReset).toHaveTextContent('100%');
  });

  it('surfaces uptime, backup recency, pending updates, and ZFS pool anomalies in hover tooltips and drawer rows', async () => {
    const threeHoursAgo = new Date(Date.now() - 3 * 3600 * 1000).toISOString();
    resourcesMock.value = [
      resource({
        id: 'pve-ops',
        type: 'agent',
        name: 'pve-ops',
        platformType: 'proxmox-pve',
        proxmox: { cpus: 8, uptime: 14 * 86400, pendingUpdates: 3 },
        agent: {
          networkInterfaces: [
            { name: 'vmbr0', addresses: ['10.60.0.10/24'], speedMbps: 10000, linkUp: true },
          ],
        },
      }),
      resource({
        id: 'vm-ops',
        type: 'vm',
        name: 'ops-vm',
        platformType: 'proxmox-pve',
        parentId: 'pve-ops',
        status: 'running',
        identity: { ips: ['10.60.0.50'] },
        proxmox: {
          vmid: 201,
          uptime: 14 * 86400,
          pendingUpdates: 3,
          lastBackup: threeHoursAgo,
        },
      }),
      resource({
        id: 'pool-zfs',
        type: 'pool',
        name: 'tank-zfs',
        platformType: 'proxmox-pve',
        parentId: 'pve-ops',
        status: 'degraded',
        disk: {
          current: 72,
          total: 8 * 1024 ** 4,
          used: Math.round(5.76 * 1024 ** 4),
        },
        storage: {
          vdevLayout: 'raidz1',
          type: 'zfspool',
          zfsPoolState: 'DEGRADED',
          rebuildInProgress: true,
          zfsReadErrors: 1,
          zfsChecksumErrors: 1,
        },
      }),
    ];

    const { container } = render(() => <InfrastructureMapSurface />);

    // Hovering ops-vm surfaces uptime, backup recency, and pending updates.
    fireEvent.pointerOver(tile(container, 'ops-vm'));
    expect(await screen.findByText(/Up 14d/, { selector: 'span' })).toBeInTheDocument();
    expect(screen.getByText(/Backed up 3h ago/, { selector: 'span' })).toBeInTheDocument();
    expect(screen.getByText(/3 updates/, { selector: 'span' })).toBeInTheDocument();

    // Hovering tank-zfs surfaces ZFS pool anomalies.
    fireEvent.pointerOver(tile(container, 'tank-zfs'));
    expect(await screen.findByText(/DEGRADED/, { selector: 'span' })).toBeInTheDocument();
    expect(screen.getByText(/Rebuilding/, { selector: 'span' })).toBeInTheDocument();
    expect(screen.getByText(/2 ZFS errors/, { selector: 'span' })).toBeInTheDocument();

    // Open host drawer to verify uptime, backup age, and pending updates in resource rows.
    const hostInspect = container.querySelector(
      '[data-map-inspect][aria-label="Inspect pve-ops (1)"]',
    );
    expect(hostInspect).not.toBeNull();
    fireEvent.click(hostInspect!);
    const hostList = screen.getByTestId('infrastructure-map-drawer-list');
    expect(hostList).toHaveTextContent('Up 14d');
    expect(hostList).toHaveTextContent('Backed up 3h ago');
    expect(hostList).toHaveTextContent('3 updates');

    // Close drawer, switch to By role, and inspect Storage group for ZFS anomaly badges.
    fireEvent.keyDown(document, { key: 'Escape' });
    fireEvent.click(screen.getByRole('button', { name: 'By role' }));
    const storageInspect = container.querySelector(
      '[data-map-inspect][aria-label^="Inspect Storage"]',
    );
    expect(storageInspect).not.toBeNull();
    fireEvent.click(storageInspect!);

    const storageList = screen.getByTestId('infrastructure-map-drawer-list');
    expect(storageList).toHaveTextContent('DEGRADED');
    expect(storageList).toHaveTextContent('Rebuilding');
    expect(storageList).toHaveTextContent('2 ZFS errors');
  });
});

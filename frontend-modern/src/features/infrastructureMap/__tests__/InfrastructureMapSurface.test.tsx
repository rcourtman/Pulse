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

  it("ends a pointer's card when the page scrolls, and keeps the card of the tile with focus", () => {
    const { container } = draw();

    // A pointer was resting on a tile: the tile has moved from under the card.
    fireEvent.pointerOver(tile(container, 'qbittorrent'));
    expect(rings(container)).toHaveLength(1);
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
    expect(rings(container)).toHaveLength(1);

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

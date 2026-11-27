import {
  For,
  Show,
  createEffect,
  createMemo,
  createSignal,
  on,
  onCleanup,
  onMount,
} from 'solid-js';
import ActivityIcon from 'lucide-solid/icons/activity';
import DownloadIcon from 'lucide-solid/icons/download';
import EyeOffIcon from 'lucide-solid/icons/eye-off';
import MapIcon from 'lucide-solid/icons/map';
import RotateCcwIcon from 'lucide-solid/icons/rotate-ccw';
import SearchIcon from 'lucide-solid/icons/search';
import TriangleAlertIcon from 'lucide-solid/icons/triangle-alert';
import XIcon from 'lucide-solid/icons/x';
import ZoomInIcon from 'lucide-solid/icons/zoom-in';
import ZoomOutIcon from 'lucide-solid/icons/zoom-out';
import {
  recordInfrastructureMapExport,
  recordInfrastructureMapOpen,
} from '@/api/infrastructureMapActivity';
import { Button, ButtonLink } from '@/components/shared/Button';
import { EmptyState } from '@/components/shared/EmptyState';
import { FilterButtonGroup } from '@/components/shared/FilterButtonGroup';
import { FeaturePreviewTag } from '@/components/FeatureRelease';
import { PageHeader } from '@/components/shared/PageHeader';
import { StatusDot } from '@/components/shared/StatusDot';
import { showPressedTooltip } from '@/components/shared/Tooltip';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { buildAvailabilityTargetAddPath } from '@/components/Settings/availabilitySettingsModel';
import { buildInfrastructureWorkspacePath } from '@/components/Settings/infrastructureWorkspaceModel';
import { useDarkMode } from '@/contexts/appRuntime';
import { useKioskMode } from '@/hooks/useKioskMode';
import { buildResourcePagePath } from '@/features/platformNavigation/resourcePagePath';
import { useObservedElementWidth } from '@/hooks/useObservedElementWidth';
import { useResources } from '@/hooks/useResources';
import {
  sessionCanReadAvailabilitySettings,
  sessionCanReadInfrastructureSettings,
} from '@/stores/sessionSettingsCapabilities';
import { MAP_DOC_URL } from '@/utils/docsLinks';
import { formatSpeed } from '@/utils/format';
import type { StatusIndicatorVariant } from '@/utils/status';
import { showError, showSuccess } from '@/utils/toast';
import './infrastructureMap.css';
import { InfrastructureMapDrawer } from './InfrastructureMapDrawer';
import {
  fitServiceDiagram,
  flowServiceDiagram,
  MAP_READABLE_SCALE,
} from './infrastructureMapDiagram';
import {
  downloadMapImage,
  resolveDiagramTileLimit,
  resolveMapPalette,
} from './infrastructureMapExport';
import {
  isMapMoveKey,
  MAP_NAMES_ITEM_LIMIT,
  nextMapHitIndex,
  resolveDefaultMapDensity,
  type MapDensity,
  type MapHit,
  type MapInspectTarget,
} from './infrastructureMapLayout';
import {
  anonymizeInfrastructureMapModel,
  buildInfrastructureMapModel,
} from './infrastructureMapModel';
import {
  MAP_HIT_ATTRIBUTE,
  MAP_INSPECT_ATTRIBUTE,
  MAP_PAGE_PALETTES,
  MAP_RINGS_ATTRIBUTE,
  renderMapLinkRings,
  renderMapSceneBody,
  renderMapTappedTile,
} from './infrastructureMapRender';
import {
  buildServiceDiagram,
  countDiagramResources,
  formatMapLatency,
  formatMapLinkSpeed,
  type ServiceDiagramFilter,
} from './infrastructureMapServices';

// The same dots the tiles carry, drawn with the shared status dot.
const LEGEND: Array<[StatusIndicatorVariant, string]> = [
  ['success', 'Healthy'],
  ['warning', 'Needs attention'],
  ['danger', 'Critical'],
  ['muted', 'Not reporting'],
];

const DENSITY_OPTIONS: Array<{ value: MapDensity; label: string; title: string }> = [
  { value: 'names', label: 'Names', title: 'Write the name of every guest and container' },
  { value: 'marks', label: 'Compact', title: 'Draw each guest and container as its icon only' },
];

// What the page keeps below the drawing: the panel's padding and one line of
// legend. The navigation bar fixed to the bottom of a window that is not
// wide, and any further lines the legend wraps to, are measured.
const SPACE_BELOW = 60;
const LEGEND_LINE = 16;
// The bar publishes its own height, and 0 where it is not shown.
const NAV_HEIGHT_VARIABLE = '--pulse-mobile-nav-height';
const MIN_MAP_HEIGHT = 240;
// Narrower than this is a phone. There the map is drawn at full size in one
// column and the page scrolls, as every page on a phone does: fitted to one
// screen, a large estate's tiles are 13px and its text 4px.
const PHONE_WINDOW = 768;

// Vertical clearance needed above a tile before a hover tooltip flips below it
// so it never covers the map toolbar on desktop or mobile.
const TOOLTIP_TOP_CLEARANCE = 160;
const TOOLTIP_BOTTOM_ROOM = 110;

const MIN_ZOOM = 1;
const MAX_ZOOM = 4;
const ZOOM_STEP = 0.25;
const DRAG_THRESHOLD_PX = 4;

// The name a new check for the gateway starts with. The owner can change it.
const UNMONITORED_CHECK_NAME = 'Gateway';

const formatNetTelemetry = (rxBytes?: number, txBytes?: number): string => {
  if (rxBytes === undefined && txBytes === undefined) return '';
  const rx = rxBytes ?? 0;
  const tx = txBytes ?? 0;
  const total = rx + tx;
  if (total <= 0) return '';
  if (total < 1024) return 'Net <1 KB/s';
  return `Net ↓ ${formatSpeed(rx)} ↑ ${formatSpeed(tx)}`;
};

/** What a tile's card says under its name: what it is, where, what it is connected to, how busy. */
const hitLines = (hit: MapHit, density?: MapDensity): string[] =>
  [
    hit.meta,
    hit.inspectTarget
      ? 'Click to browse all items'
      : hit.id.endsWith(':more') && density === 'names'
        ? 'Compact draws more of them'
        : '',
    [
      // A plain check's caption is its address, so the line above has said it.
      hit.address && !hit.meta.includes(hit.address) ? hit.address : '',
      hit.detail ? `Published ${hit.detail}` : '',
      hit.linkSpeedMbps ? `Link ${formatMapLinkSpeed(hit.linkSpeedMbps)}` : '',
      hit.linkDownCount && hit.linkDownCount > 0 ? `${hit.linkDownCount} link down` : '',
      hit.firewallEnabled ? 'NIC firewall flag' : '',
    ]
      .filter(Boolean)
      .join(' · '),
    ...(hit.links ?? []),
    [
      hit.cpu !== undefined ? `CPU ${Math.round(hit.cpu)}%` : '',
      hit.memory !== undefined ? `Memory ${Math.round(hit.memory)}%` : '',
      formatNetTelemetry(hit.netRxBytes, hit.netTxBytes),
      hit.latencyMs !== undefined ? `Latency ${formatMapLatency(hit.latencyMs)}` : '',
    ]
      .filter(Boolean)
      .join(' · '),
  ].filter(Boolean);

export function InfrastructureMapSurface() {
  const { resources, loading, error, refetch } = useResources();
  const loadError = () => error?.();
  const darkMode = useDarkMode();
  const kioskMode = useKioskMode();
  const observed = useObservedElementWidth();
  // The page offers no Settings destination this session cannot open, and
  // none at all in kiosk mode, where the shell turns Settings away.
  const canAddCheck = () => !kioskMode() && sessionCanReadAvailabilitySettings();
  const canAddSystem = () => !kioskMode() && sessionCanReadInfrastructureSettings();

  const [densityChoice, setDensityChoice] = createSignal<MapDensity | null>(null);
  const [liveTraffic, setLiveTraffic] = createSignal(true);
  const [namesHidden, setNamesHidden] = createSignal(false);
  const [exporting, setExporting] = createSignal(false);
  const [attentionOnly, setAttentionOnly] = createSignal(false);
  const [searchQuery, setSearchQuery] = createSignal('');
  const [networkFilter, setNetworkFilter] = createSignal<string | null>(null);
  const [drawerTarget, setDrawerTarget] = createSignal<MapInspectTarget | null>(null);
  const [zoom, setZoom] = createSignal(MIN_ZOOM);
  const [pan, setPan] = createSignal({ x: 0, y: 0 });
  const [dragging, setDragging] = createSignal(false);
  const [hover, setHover] = createSignal<{
    hit: MapHit;
    x: number;
    y: number;
    direction: 'up' | 'down';
  } | null>(null);
  // The tile a tap on a touch screen is showing, by its id.
  const [armed, setArmed] = createSignal<string | null>(null);
  // Whether the card on show was reached with the keyboard, and whether an
  // arrow key has moved between tiles yet. Until one has, that card says
  // the arrow keys do, because Tab no longer walks the tiles.
  const [byKeys, setByKeys] = createSignal(false);
  const [arrowsUsed, setArrowsUsed] = createSignal(false);

  // Counted once per browser session. Nothing on the map is sent.
  onMount(recordInfrastructureMapOpen);

  const resourceById = createMemo(
    () => new Map(resources().map((resource) => [resource.id, resource] as const)),
  );
  const liveModel = createMemo(() => buildInfrastructureMapModel(resources()));
  const model = createMemo(() =>
    namesHidden() ? anonymizeInfrastructureMapModel(liveModel()) : liveModel(),
  );
  // On the page every colour is a reference to the theme's own variable.
  const palette = createMemo(() => (darkMode() ? MAP_PAGE_PALETTES.dark : MAP_PAGE_PALETTES.light));

  const hasActiveFilter = createMemo(
    () => attentionOnly() || searchQuery().trim().length > 0 || networkFilter() !== null,
  );
  const activeFilter = createMemo<ServiceDiagramFilter | undefined>(() => {
    const query = searchQuery().trim();
    const net = networkFilter();
    if (!attentionOnly() && !query && !net) return undefined;
    return {
      ...(attentionOnly() ? { attentionOnly: true } : {}),
      ...(query ? { query } : {}),
      ...(net ? { networkId: net } : {}),
    };
  });

  // The map is fitted to what is left of the window under its toolbar, so
  // all of it is on screen at once and nothing is read by scrolling.
  let frameRef: HTMLDivElement | undefined;
  let legendRef: HTMLUListElement | undefined;
  const [space, setSpace] = createSignal<{
    width: number;
    height: number;
    phone: boolean;
  } | null>(null);
  const measure = () => {
    const width = observed.width();
    if (!frameRef || !width) return;
    // Where the map starts when nothing is scrolled. The app scrolls inside
    // its own shell, not the document, so every box above is counted.
    let top = frameRef.getBoundingClientRect().top;
    for (let box = frameRef.parentElement; box; box = box.parentElement) top += box.scrollTop;
    const navigation =
      Number.parseFloat(
        getComputedStyle(document.documentElement).getPropertyValue(NAV_HEIGHT_VARIABLE),
      ) || 0;
    const legendWrap = Math.max(0, (legendRef?.offsetHeight ?? 0) - LEGEND_LINE);
    const below = SPACE_BELOW + navigation + legendWrap;
    const height = Math.max(MIN_MAP_HEIGHT, Math.floor(window.innerHeight - top - below));
    const phone = window.innerWidth < PHONE_WINDOW;
    setSpace((was) =>
      was?.width === width && was.height === height && was.phone === phone
        ? was
        : { width, height, phone },
    );
  };
  createEffect(on(observed.width, measure));
  createEffect(
    on(hasActiveFilter, () => {
      if (typeof requestAnimationFrame !== 'undefined') {
        requestAnimationFrame(measure);
      }
    }),
  );
  onMount(() => {
    window.addEventListener('resize', measure);
    onCleanup(() => window.removeEventListener('resize', measure));
    // A banner that appears or is dismissed above the page moves the map
    // without changing its width or the window. Whatever takes room or gives
    // it back resizes one of the boxes the map sits in, so each is watched.
    if (!frameRef || typeof ResizeObserver === 'undefined') return;
    let queued = 0;
    // Measured on the next frame: redrawing inside the observer's own
    // delivery makes the browser report a resize loop.
    const observer = new ResizeObserver(() => {
      cancelAnimationFrame(queued);
      queued = requestAnimationFrame(measure);
    });
    for (let box = frameRef.parentElement; box; box = box.parentElement) observer.observe(box);
    // The navigation bar publishes its height on the document element. A
    // change there alone, with every box the same size, is watched too.
    const published = new MutationObserver(() => {
      cancelAnimationFrame(queued);
      queued = requestAnimationFrame(measure);
    });
    published.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['style'],
    });
    onCleanup(() => {
      cancelAnimationFrame(queued);
      observer.disconnect();
      published.disconnect();
    });
  });

  const fitted = createMemo(() => {
    const room = space();
    if (!room || model().isEmpty) return null;
    const filter = activeFilter();
    const traffic = liveTraffic();
    const fit = (chosen: MapDensity) => {
      const diagram = buildServiceDiagram(model(), {
        maxTilesPerGroup: resolveDiagramTileLimit(chosen),
        ...(filter ? { filter } : {}),
      });
      return {
        density: chosen,
        diagram,
        ...(room.phone
          ? flowServiceDiagram(diagram, room.width, chosen, { liveTraffic: traffic })
          : fitServiceDiagram(diagram, { width: room.width, height: room.height }, chosen, {
              liveTraffic: traffic,
            })),
      };
    };
    const chosen = densityChoice();
    if (chosen) return fit(chosen);
    // On a phone the map scrolls, and icons alone make the shortest page. A
    // tap reads a tile. Names are there to be chosen.
    if (room.phone) return fit('marks');
    // When filtering narrows a large estate to a readable subset, try names first
    // so search results and attention views surface labels immediately.
    const defaultDensity: MapDensity = filter ? 'names' : resolveDefaultMapDensity(liveModel());
    const preferred = fit(defaultDensity);
    if (filter && countDiagramResources(preferred.diagram) > MAP_NAMES_ITEM_LIMIT) {
      return fit('marks');
    }
    if (preferred.density === 'marks' || preferred.scale >= MAP_READABLE_SCALE) return preferred;
    const compact = fit('marks');
    // On a very narrow window both reach the smallest size the map is drawn
    // at. Icons alone are then the shorter map, so they win a tie.
    return compact.scale >= preferred.scale ? compact : preferred;
  });
  const density = createMemo<MapDensity>(
    () => fitted()?.density ?? densityChoice() ?? resolveDefaultMapDensity(liveModel()),
  );
  const activeDiagram = createMemo(() => fitted()?.diagram ?? null);
  // Full unfiltered diagram for inspecting networks and total counts while a filter is active.
  const fullDiagram = createMemo(() => {
    if (!hasActiveFilter()) return activeDiagram();
    if (model().isEmpty) return null;
    return buildServiceDiagram(model(), {
      maxTilesPerGroup: resolveDiagramTileLimit(density()),
    });
  });
  const matchedResourceCount = createMemo(() => {
    const diagram = activeDiagram();
    return diagram ? countDiagramResources(diagram) : 0;
  });
  const totalResourceCount = createMemo(() => {
    const diagram = fullDiagram();
    return diagram ? countDiagramResources(diagram) : 0;
  });
  const drawerDiagram = createMemo(() => {
    const target = drawerTarget();
    if (!target) return null;
    return target.kind === 'group' ? activeDiagram() : fullDiagram();
  });
  const activeNetworkLabel = createMemo(() => {
    const id = networkFilter();
    if (!id) return null;
    const list = fullDiagram()?.networks ?? activeDiagram()?.networks ?? [];
    return list.find((network) => network.id === id)?.label ?? id;
  });

  const hasVisibleMatches = createMemo(() => {
    const diagram = activeDiagram();
    if (!diagram) return false;
    return matchedResourceCount() > 0 || diagram.gateways.length > 0;
  });
  const scene = createMemo(() => (hasVisibleMatches() ? (fitted()?.scene ?? null) : null));
  const hits = createMemo(() => {
    const out: MapHit[] = [];
    for (const node of scene()?.nodes ?? []) if (node.kind === 'hit') out.push(node.hit);
    return out;
  });
  const inspectTargets = createMemo(() => {
    const out: MapInspectTarget[] = [];
    for (const node of scene()?.nodes ?? []) if (node.kind === 'inspect') out.push(node.target);
    return out;
  });
  const hrefForResourceId = (id: string): string | null => {
    const resource = resourceById().get(id);
    return resource ? buildResourcePagePath({ id: resource.id, resource }) : null;
  };
  const hrefForHit = (index: number): string | null => {
    const hit = hits()[index];
    // A thing nothing monitors has no page. Its tile leads to the form that
    // adds a check for it, which is the one thing there is to do about it.
    if (hit?.checkAddress) {
      if (!canAddCheck()) return null;
      return buildAvailabilityTargetAddPath('router', {
        name: UNMONITORED_CHECK_NAME,
        address: hit.checkAddress,
        from: 'map',
      });
    }
    return hit ? hrefForResourceId(hit.id) : null;
  };
  const body = createMemo(() => {
    const current = scene();
    return current ? renderMapSceneBody(current, palette(), hrefForHit) : '';
  });

  // Hoisted memos for TooltipPortal props so Solid never compiles an inline
  // memo() inside requestAnimationFrame (which triggers a createRoot warning).
  const showHoverTooltip = createMemo(() => hover() !== null && armed() === null);
  const tooltipX = createMemo(() => hover()?.x ?? 0);
  const tooltipY = createMemo(() => hover()?.y ?? 0);
  const tooltipDirection = createMemo<'up' | 'down'>(() => hover()?.direction ?? 'up');

  const clampPan = (next: { x: number; y: number }, scaleValue: number) => {
    if (scaleValue <= MIN_ZOOM) return { x: 0, y: 0 };
    const current = fitted();
    const width = current ? Math.floor(current.scene.width * current.scale) : 0;
    const height = current ? Math.floor(current.scene.height * current.scale) : 0;
    const maxX = Math.max(0, ((scaleValue - 1) * width) / 2 + 80);
    const maxY = Math.max(0, (scaleValue - 1) * height + 80);
    return {
      x: Math.min(maxX, Math.max(-maxX, next.x)),
      y: Math.min(80, Math.max(-maxY, next.y)),
    };
  };
  const setZoomClamped = (nextZoom: number) => {
    const clamped = Math.round(Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, nextZoom)) * 100) / 100;
    setZoom(clamped);
    setPan((current) => clampPan(current, clamped));
  };
  const resetZoom = () => {
    clearSuppressedClick();
    setZoom(MIN_ZOOM);
    setPan({ x: 0, y: 0 });
  };
  const clearFilters = () => {
    setAttentionOnly(false);
    setSearchQuery('');
    setNetworkFilter(null);
    resetZoom();
  };

  // Drag-to-pan state when zoomed in.
  let dragStart: { x: number; y: number; panX: number; panY: number; pointerId: number } | null =
    null;
  let suppressNextClick = false;
  let clearSuppressClickTimer: number | undefined;
  const clearSuppressedClick = () => {
    suppressNextClick = false;
    if (clearSuppressClickTimer !== undefined) {
      window.clearTimeout(clearSuppressClickTimer);
      clearSuppressClickTimer = undefined;
    }
  };
  onCleanup(clearSuppressedClick);
  const handleViewportPointerDown = (event: PointerEvent) => {
    clearSuppressedClick();
    if (zoom() <= MIN_ZOOM || event.button !== 0 || event.pointerType === 'touch') return;
    dragStart = {
      x: event.clientX,
      y: event.clientY,
      panX: pan().x,
      panY: pan().y,
      pointerId: event.pointerId,
    };
  };
  const handleViewportPointerMove = (event: PointerEvent) => {
    if (!dragStart || event.pointerId !== dragStart.pointerId) return;
    const dx = event.clientX - dragStart.x;
    const dy = event.clientY - dragStart.y;
    if (!dragging() && Math.hypot(dx, dy) < DRAG_THRESHOLD_PX) return;
    if (!dragging()) {
      setDragging(true);
      setHover(null);
      suppressNextClick = true;
    }
    setPan(clampPan({ x: dragStart.panX + dx, y: dragStart.panY + dy }, zoom()));
  };
  const endViewportDrag = (event: PointerEvent) => {
    if (!dragStart || event.pointerId !== dragStart.pointerId) return;
    dragStart = null;
    if (dragging()) {
      setDragging(false);
    }
    // WebKit/Safari and off-target drag releases do not always fire a synthetic
    // click after pointerup. Clear the suppression on the next macrotask so it
    // never swallows the user's next intentional click on a tile or header.
    if (suppressNextClick && typeof window !== 'undefined') {
      clearSuppressClickTimer = window.setTimeout(() => {
        suppressNextClick = false;
        clearSuppressClickTimer = undefined;
      }, 0);
    }
  };
  const handleViewportWheel = (event: WheelEvent) => {
    if (!event.ctrlKey && !event.metaKey) return;
    event.preventDefault();
    const delta = event.deltaY < 0 ? ZOOM_STEP : -ZOOM_STEP;
    setZoomClamped(zoom() + delta);
  };

  const [svgElement, setSvgElement] = createSignal<SVGSVGElement>();
  // The map is one stop for the Tab key, not one for every tile: with 190
  // tiles that was 190 presses to get past it. The stop is the tile that
  // last had focus, or the first one drawn, and the arrow keys move from
  // there (`handleKey`). Kept by resource, so it survives a redraw.
  let tabStopId: string | undefined;
  const setTabStop = (svg: SVGSVGElement) => {
    const kept = hits().findIndex((hit) => hit.id === tabStopId);
    const stop = String(kept >= 0 ? kept : 0);
    for (const element of svg.querySelectorAll(`[${MAP_HIT_ATTRIBUTE}]`)) {
      element.setAttribute(
        'tabindex',
        element.getAttribute(MAP_HIT_ATTRIBUTE) === stop ? '0' : '-1',
      );
    }
  };
  // The drawing is replaced whole on every refresh. Keep a keyboard user's
  // place by handing focus back to the same resource afterwards. Watch the
  // element as well as the markup: the drawing can be ready before Show
  // mounts the SVG on entry or when the first inventory arrives.
  createEffect(
    on([body, svgElement], ([markup, svg], _previous, previousHits: MapHit[] | undefined) => {
      if (!svg) return;
      const focused = svg.contains(document.activeElement)
        ? document.activeElement?.closest(`[${MAP_HIT_ATTRIBUTE}]`)
        : null;
      const focusedAt = focused ? Number(focused.getAttribute(MAP_HIT_ATTRIBUTE)) : -1;
      const focusedId = focused ? previousHits?.[focusedAt]?.id : undefined;
      svg.innerHTML = markup;
      if (focused && hits().length > 0) {
        // The same resource where it is still drawn. Where it has gone, the
        // tile now in its place: replacing the drawing would otherwise drop
        // the focus off the map, back to the top of the page.
        const same = focusedId ? hits().findIndex((hit) => hit.id === focusedId) : -1;
        const index = same >= 0 ? same : Math.min(Math.max(focusedAt, 0), hits().length - 1);
        tabStopId = hits()[index].id;
        setTabStop(svg);
        svg
          .querySelector<SVGElement>(`[${MAP_HIT_ATTRIBUTE}="${index}"]`)
          ?.focus({ preventScroll: true });
      } else {
        setTabStop(svg);
      }
      return hits();
    }),
  );

  // The map draws no line between two tiles. While one is pointed at or
  // focused, the tiles its connections join it to are ringed, and its card
  // says what each connection is. Declared after the effect above, so it
  // runs once the drawing it adds to has been replaced.
  createEffect(() => {
    body();
    const current = scene();
    const shown = hover();
    const layer = svgElement()?.querySelector(`[${MAP_RINGS_ATTRIBUTE}]`);
    if (!layer) return;
    if (!current || !shown) {
      layer.innerHTML = '';
      return;
    }
    // A tapped tile is marked as well: no pointer rests on it to show which
    // tile the card is about.
    layer.innerHTML =
      (armed() === null ? '' : renderMapTappedTile(current, shown.hit, palette())) +
      renderMapLinkRings(current, shown.hit, palette());
  });

  const hitFromEvent = (event: Event): { hit: MapHit; element: Element } | null => {
    const element = (event.target as Element | null)?.closest?.(`[${MAP_HIT_ATTRIBUTE}]`);
    if (!element) return null;
    const hit = hits()[Number(element.getAttribute(MAP_HIT_ATTRIBUTE))];
    return hit ? { hit, element } : null;
  };
  const inspectFromEvent = (event: Event): MapInspectTarget | null => {
    const element = (event.target as Element | null)?.closest?.(`[${MAP_INSPECT_ATTRIBUTE}]`);
    if (!element) return null;
    return inspectTargets()[Number(element.getAttribute(MAP_INSPECT_ATTRIBUTE))] ?? null;
  };
  const point = (found: { hit: MapHit; element: Element }) => {
    const box = found.element.getBoundingClientRect();
    const frameRect = frameRef?.getBoundingClientRect();
    const hasRealFrame = Boolean(frameRect && frameRect.height > 0);
    const safeTop = hasRealFrame && frameRect ? Math.max(frameRect.top, 0) : 0;
    const roomBelow = typeof window !== 'undefined' ? window.innerHeight - box.bottom : 0;
    const flipDown =
      hasRealFrame && box.top - safeTop < TOOLTIP_TOP_CLEARANCE && roomBelow > TOOLTIP_BOTTOM_ROOM;
    setHover({
      hit: found.hit,
      x: box.left + box.width / 2,
      y: flipDown ? box.bottom : box.top,
      direction: flipDown ? 'down' : 'up',
    });
  };
  const isTouch = (event: Event): boolean => (event as PointerEvent).pointerType === 'touch';
  const showFor = (event: Event) => {
    // A finger is not a pointer that hovers. What it touched is said on the
    // tap, below.
    if (isTouch(event) || dragging()) return;
    const found = hitFromEvent(event);
    if (found) point(found);
    else setHover(null);
  };
  const handleFocusIn = (event: FocusEvent) => {
    const found = hitFromEvent(event);
    const svg = svgElement();
    if (found && svg) {
      tabStopId = found.hit.id;
      setTabStop(svg);
    }
    setByKeys(keyedLast);
    showFor(event);
  };
  // Remember which tile or control opened the drawer so focus returns cleanly
  // on Escape or Close, including for SVG tiles that survive live redraws.
  let drawerReturnHitId: string | null = null;
  let drawerReturnElement: HTMLElement | SVGElement | null = null;
  const openDrawer = (target: MapInspectTarget, returnHitId?: string) => {
    drawerReturnHitId = returnHitId ?? null;
    const active = typeof document !== 'undefined' ? document.activeElement : null;
    drawerReturnElement =
      active instanceof HTMLElement || active instanceof SVGElement ? active : null;
    setHover(null);
    setDrawerTarget(target);
  };
  const closeDrawer = () => {
    const returnHitId = drawerReturnHitId;
    const returnEl = drawerReturnElement;
    drawerReturnHitId = null;
    drawerReturnElement = null;
    setDrawerTarget(null);
    if (typeof requestAnimationFrame === 'undefined') return;
    requestAnimationFrame(() => {
      const svg = svgElement();
      if (returnHitId && svg) {
        const index = hits().findIndex((hit) => hit.id === returnHitId);
        if (index >= 0) {
          tabStopId = returnHitId;
          setTabStop(svg);
          svg
            .querySelector<SVGElement>(`[${MAP_HIT_ATTRIBUTE}="${index}"]`)
            ?.focus({ preventScroll: true });
          return;
        }
      }
      if (returnEl && document.contains(returnEl)) {
        returnEl.focus({ preventScroll: true });
      }
    });
  };

  // An arrow key, Home or End moves the focus to another tile. Enter/Space on
  // an overflow tile opens the group drawer. Escape dismisses an open tooltip.
  // A key that moves nowhere is left to the page, which scrolls with it.
  const handleKey = (event: KeyboardEvent) => {
    if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
    if (event.key === 'Escape') {
      if (hover() !== null || armed() !== null) {
        event.preventDefault();
        setHover(null);
        disarm();
      }
      return;
    }
    if (event.key === 'Enter' || event.key === ' ') {
      const found = hitFromEvent(event);
      if (found?.hit.inspectTarget) {
        event.preventDefault();
        openDrawer(found.hit.inspectTarget, found.hit.id);
      }
      return;
    }
    if (!isMapMoveKey(event.key)) return;
    const current = scene();
    const found = hitFromEvent(event);
    if (!current || !found) return;
    const from = Number(found.element.getAttribute(MAP_HIT_ATTRIBUTE));
    const to = nextMapHitIndex(current, from, event.key);
    if (to === null) return;
    event.preventDefault();
    if (event.key.startsWith('Arrow')) setArrowsUsed(true);
    svgElement()?.querySelector<SVGElement>(`[${MAP_HIT_ATTRIBUTE}="${to}"]`)?.focus();
  };

  // A touch screen has no hover, and a tile is a link, so a tap would open
  // the tile before anyone could read what it is. There the first tap says
  // what the tile is and rings what it is connected to, as pointing does,
  // and a second tap on the same tile opens it.
  let pressedWith = '';
  // Whether a key was the last thing pressed, so that a tile which takes
  // focus next was reached with the keyboard. Kept apart from `pressedWith`:
  // a key pressed between a finger's touch and its click must not change
  // what that tap does.
  let keyedLast = false;
  let armedAtPress: string | null = null;
  const disarm = () => {
    if (armed() === null) return;
    setArmed(null);
    setHover(null);
  };
  onMount(() => {
    // The next press anywhere, or a scroll, ends what a tap showed. The
    // press remembers the tile first, so a tap on it again can open it.
    const press = (event: PointerEvent) => {
      pressedWith = event.pointerType;
      keyedLast = false;
      armedAtPress = armed();
      disarm();
    };
    const typed = (event: KeyboardEvent) => {
      keyedLast = true;
      if (event.key === 'Escape' && !drawerTarget() && (hover() !== null || armed() !== null)) {
        setHover(null);
        disarm();
      }
    };
    // A card is placed where its tile was. On a page that scrolls, the tile
    // moves: a card for the tile with keyboard focus goes with it, leaves
    // while the tile is off the screen and comes back with it, and one for a
    // tile a pointer was resting on ends until the pointer moves. A scroll
    // of something that does not hold the map, a menu's list, moves no tile.
    let scrolledAway: string | null = null;
    const scrolled = (event: Event) => {
      disarm();
      const within = event.target;
      if (within instanceof Element && frameRef && !within.contains(frameRef)) return;
      const shown = hover();
      const focused = document.activeElement?.closest?.(`[${MAP_HIT_ATTRIBUTE}]`);
      const held = focused ? hits()[Number(focused.getAttribute(MAP_HIT_ATTRIBUTE))] : undefined;
      if (!focused || !held || (shown ? shown.hit.id !== held.id : scrolledAway !== held.id)) {
        scrolledAway = null;
        if (shown) setHover(null);
        return;
      }
      const box = focused.getBoundingClientRect();
      if (box.bottom < 0 || box.top > window.innerHeight) {
        scrolledAway = held.id;
        setHover(null);
      } else {
        scrolledAway = null;
        point({ hit: held, element: focused });
      }
    };
    document.addEventListener('pointerdown', press, true);
    document.addEventListener('keydown', typed, true);
    window.addEventListener('scroll', scrolled, true);
    onCleanup(() => {
      document.removeEventListener('pointerdown', press, true);
      document.removeEventListener('keydown', typed, true);
      window.removeEventListener('scroll', scrolled, true);
    });
  });
  const handleSvgClick = (event: MouseEvent) => {
    if (suppressNextClick) {
      clearSuppressedClick();
      event.preventDefault();
      event.stopPropagation();
      return;
    }
    const inspectTarget = inspectFromEvent(event);
    if (inspectTarget) {
      event.preventDefault();
      openDrawer(inspectTarget);
      return;
    }
    const found = hitFromEvent(event);
    if (!found) return;
    if (found.hit.inspectTarget) {
      event.preventDefault();
      openDrawer(found.hit.inspectTarget, found.hit.id);
      return;
    }
    if (pressedWith !== 'touch') return;
    const opens = found.element.hasAttribute('href');
    if (opens && armedAtPress === found.hit.id) return;
    event.preventDefault();
    point(found);
    setArmed(found.hit.id);
    showPressedTooltip(
      [
        found.hit.name,
        ...hitLines(found.hit, density()),
        !opens ? '' : found.hit.checkAddress ? 'Tap again to add a check' : 'Tap again to open',
      ]
        .filter(Boolean)
        .join('\n'),
      found.element,
    );
  };

  const handleDownload = async () => {
    if (exporting()) return;
    setExporting(true);
    // Whether names were hidden when Save was pressed, not once it has saved.
    const saved = { namesHidden: namesHidden() };
    try {
      const format = await downloadMapImage(model(), {
        title: 'Infrastructure map',
        density: density(),
        palette: resolveMapPalette(palette(), darkMode()),
      });
      recordInfrastructureMapExport(saved);
      showSuccess(
        'Map image saved',
        format === 'png'
          ? undefined
          : 'This map is too large for a PNG, so it was saved as an SVG instead.',
      );
    } catch {
      showError('The map image could not be saved');
    } finally {
      setExporting(false);
    }
  };

  return (
    // On a phone the page scrolls, and its last line would end under the
    // navigation bar by the few pixels the shell leaves every page short.
    <div class="flex flex-col gap-3 pb-2 md:pb-0" data-testid="infrastructure-map">
      <div class="flex flex-wrap items-center gap-x-3 gap-y-2">
        {/* AppLayout already titles the tab for this route with the brand name. */}
        <PageHeader
          title="Map"
          titleMeta={<FeaturePreviewTag feature="infrastructureMap" />}
          updateDocumentTitle={false}
        />
        <Show when={liveModel().attentionCount > 0}>
          <button
            type="button"
            aria-pressed={attentionOnly()}
            title="Filter map to resources needing attention (Critical and Warning/Degraded). Click to toggle."
            data-testid="infrastructure-map-attention-filter"
            class={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-medium whitespace-nowrap transition-colors focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-amber-500 ${
              attentionOnly()
                ? 'border-amber-600 bg-amber-600 text-white hover:bg-amber-700 dark:border-amber-500 dark:bg-amber-500 dark:text-slate-950 dark:hover:bg-amber-400'
                : 'border-amber-300 bg-amber-100 text-amber-900 hover:bg-amber-200/80 dark:border-amber-700/80 dark:bg-amber-900/30 dark:text-amber-200 dark:hover:bg-amber-900/50'
            }`}
            onClick={() => setAttentionOnly((active) => !active)}
          >
            <TriangleAlertIcon class="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
            <span>{liveModel().attentionCount.toLocaleString()} need attention</span>
            <Show when={attentionOnly()}>
              <XIcon class="h-3 w-3 shrink-0" aria-hidden="true" />
            </Show>
          </button>
        </Show>
        <Show when={!liveModel().isEmpty}>
          <div class="flex w-full flex-wrap items-center gap-2 sm:ml-auto sm:w-auto">
            <div class="relative w-full sm:w-48 sm:flex-none md:w-56">
              <SearchIcon
                class="pointer-events-none absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2 text-muted"
                aria-hidden="true"
              />
              <input
                type="text"
                inputMode="search"
                value={searchQuery()}
                onInput={(event) => setSearchQuery(event.currentTarget.value)}
                placeholder="Search map..."
                aria-label="Search map resources"
                title="Search across all resources on the map, including grouped overflow items"
                data-testid="infrastructure-map-search"
                class="h-8 w-full rounded-md border border-border bg-surface pr-7 pl-8 text-xs text-base-content placeholder:text-muted focus:border-blue-500 focus:ring-2 focus:ring-blue-500/30 focus:outline-none"
              />
              <Show when={searchQuery().length > 0}>
                <button
                  type="button"
                  class="absolute top-1/2 right-1.5 inline-flex h-5 w-5 -translate-y-1/2 items-center justify-center rounded-full text-muted hover:bg-surface-hover hover:text-base-content"
                  aria-label="Clear map search"
                  onClick={() => setSearchQuery('')}
                >
                  <XIcon class="h-3 w-3" aria-hidden="true" />
                </button>
              </Show>
            </div>
            <FilterButtonGroup
              variant="compact"
              ariaLabel="Map detail"
              options={DENSITY_OPTIONS}
              value={density()}
              onChange={(value) => setDensityChoice(value)}
            />
            <div
              class="inline-flex items-center rounded-md border border-border bg-surface shadow-xs"
              role="group"
              aria-label="Map zoom"
            >
              <button
                type="button"
                class="inline-flex h-8 w-7 items-center justify-center rounded-l-md text-muted transition-colors hover:bg-surface-hover hover:text-base-content disabled:cursor-not-allowed disabled:opacity-40"
                aria-label="Zoom out"
                title="Zoom out"
                disabled={zoom() <= MIN_ZOOM}
                onClick={() => setZoomClamped(zoom() - ZOOM_STEP)}
              >
                <ZoomOutIcon class="h-3.5 w-3.5" aria-hidden="true" />
              </button>
              <button
                type="button"
                class="inline-flex h-8 min-w-11 items-center justify-center border-x border-border px-1.5 text-xs font-medium text-base-content tabular-nums transition-colors hover:bg-surface-hover"
                aria-label={`Zoom ${Math.round(zoom() * 100)}%, reset to default`}
                title={
                  zoom() > MIN_ZOOM
                    ? 'Reset zoom and pan to default'
                    : 'Current zoom (Ctrl+scroll or click + to zoom in)'
                }
                data-testid="infrastructure-map-zoom-reset"
                onClick={resetZoom}
              >
                {Math.round(zoom() * 100)}%
              </button>
              <button
                type="button"
                class="inline-flex h-8 w-7 items-center justify-center rounded-r-md text-muted transition-colors hover:bg-surface-hover hover:text-base-content disabled:cursor-not-allowed disabled:opacity-40"
                aria-label="Zoom in"
                title="Zoom in"
                disabled={zoom() >= MAX_ZOOM}
                onClick={() => setZoomClamped(zoom() + ZOOM_STEP)}
              >
                <ZoomInIcon class="h-3.5 w-3.5" aria-hidden="true" />
              </button>
            </div>
            <Show when={zoom() > MIN_ZOOM}>
              <Button
                variant="secondary"
                size="sm"
                class="gap-1"
                title="Reset zoom and pan to fit"
                onClick={resetZoom}
              >
                <RotateCcwIcon class="h-3.5 w-3.5" aria-hidden="true" />
                Fit
              </Button>
            </Show>
            <Button
              variant={liveTraffic() ? 'primaryFlat' : 'secondary'}
              size="sm"
              class="gap-1.5"
              aria-pressed={liveTraffic()}
              data-testid="infrastructure-map-live-traffic"
              title="Highlight live network traffic on active tiles, networks, and connectors"
              onClick={() => setLiveTraffic((active) => !active)}
            >
              <ActivityIcon class="h-3.5 w-3.5" aria-hidden="true" />
              Live traffic
            </Button>
            <Button
              variant={namesHidden() ? 'primaryFlat' : 'secondary'}
              size="sm"
              class="gap-1.5"
              aria-pressed={namesHidden()}
              title="Replace every name with what the thing is, so the image is safe to post"
              onClick={() => setNamesHidden((hidden) => !hidden)}
            >
              <EyeOffIcon class="h-3.5 w-3.5" aria-hidden="true" />
              Hide names
            </Button>
            <Button
              variant="secondary"
              size="sm"
              class="gap-1.5"
              disabled={exporting()}
              title="Save this map as an image file"
              onClick={() => void handleDownload()}
            >
              <DownloadIcon class="h-3.5 w-3.5" aria-hidden="true" />
              Save image
            </Button>
          </div>
        </Show>
      </div>

      <Show when={hasActiveFilter() && !liveModel().isEmpty}>
        <div
          class="flex flex-wrap items-center gap-2 rounded-md border border-border bg-surface-alt/60 px-3 py-1.5 text-xs text-muted"
          data-testid="infrastructure-map-filter-bar"
        >
          <span>
            Showing{' '}
            <strong class="font-semibold text-base-content">
              {matchedResourceCount().toLocaleString()}
            </strong>{' '}
            of {totalResourceCount().toLocaleString()} resources
          </span>
          <Show when={attentionOnly()}>
            <span class="rounded-full border border-amber-300 bg-amber-100/80 px-2 py-0.5 text-[11px] font-medium text-amber-900 dark:border-amber-700/70 dark:bg-amber-900/30 dark:text-amber-200">
              Critical &amp; Warning
            </span>
          </Show>
          <Show when={activeNetworkLabel()}>
            {(label) => (
              <span class="inline-flex items-center gap-1 rounded-full border border-blue-300 bg-blue-50 px-2 py-0.5 text-[11px] font-medium text-blue-700 dark:border-blue-700 dark:bg-blue-950/60 dark:text-blue-300">
                Network: {label()}
                <button
                  type="button"
                  aria-label="Clear network filter"
                  class="hover:text-blue-900 dark:hover:text-blue-100"
                  onClick={() => setNetworkFilter(null)}
                >
                  <XIcon class="h-3 w-3" aria-hidden="true" />
                </button>
              </span>
            )}
          </Show>
          <button
            type="button"
            class="ml-auto font-medium text-blue-600 hover:underline dark:text-blue-400"
            onClick={clearFilters}
          >
            Clear filters
          </button>
        </div>
      </Show>

      <div
        ref={(element) => {
          frameRef = element;
          observed.setElement(element);
        }}
        class="min-w-0"
      >
        <Show
          when={fitted()}
          fallback={
            <Show
              when={!loading() && liveModel().isEmpty}
              fallback={<div class="p-6 text-sm text-muted">Drawing the map...</div>}
            >
              <Show
                when={Boolean(loadError())}
                fallback={
                  <EmptyState
                    icon={<MapIcon class="h-6 w-6" aria-hidden="true" />}
                    title="Nothing to draw yet"
                    description={
                      canAddSystem()
                        ? 'The map draws itself from the systems Pulse monitors. Add one and it appears here.'
                        : 'The map draws itself from the systems Pulse monitors. None has been added yet. Ask a Pulse administrator to add one.'
                    }
                    actions={
                      <Show when={canAddSystem()}>
                        <ButtonLink
                          href={buildInfrastructureWorkspacePath()}
                          variant="secondary"
                          size="sm"
                        >
                          Add a system
                        </ButtonLink>
                      </Show>
                    }
                  />
                }
              >
                <EmptyState
                  icon={<TriangleAlertIcon class="h-6 w-6 text-amber-500" aria-hidden="true" />}
                  title="Could not load the map"
                  description="Pulse could not load your monitored resources. Check the connection and try again."
                  actions={
                    <Button variant="secondary" size="sm" onClick={() => void refetch?.()}>
                      Try again
                    </Button>
                  }
                />
              </Show>
            </Show>
          }
        >
          {(current) => (
            <Show
              when={hasVisibleMatches()}
              fallback={
                <EmptyState
                  icon={<SearchIcon class="h-6 w-6" aria-hidden="true" />}
                  title="No matching resources"
                  description="No resources on the map match the active search or filter criteria."
                  actions={
                    <Button variant="secondary" size="sm" onClick={clearFilters}>
                      Clear filters
                    </Button>
                  }
                />
              }
            >
              <div
                class={
                  zoom() > MIN_ZOOM
                    ? `relative overflow-hidden rounded-lg border border-border/70 bg-surface/40 ${
                        dragging() ? 'cursor-grabbing' : 'cursor-grab'
                      }`
                    : 'relative'
                }
                style={
                  zoom() > MIN_ZOOM && !space()?.phone
                    ? { height: `${Math.floor(current().scene.height * current().scale)}px` }
                    : undefined
                }
                onPointerDown={handleViewportPointerDown}
                onPointerMove={handleViewportPointerMove}
                onPointerUp={endViewportDrag}
                onPointerCancel={endViewportDrag}
                onWheel={handleViewportWheel}
              >
                <div
                  style={
                    zoom() > MIN_ZOOM
                      ? {
                          transform: `translate3d(${pan().x}px, ${pan().y}px, 0) scale(${zoom()})`,
                          'transform-origin': 'center top',
                        }
                      : undefined
                  }
                >
                  <svg
                    ref={(element) => {
                      setSvgElement(element);
                      // In the capture phase, so it runs before the router's own
                      // click handler, which follows links.
                      element.addEventListener('click', handleSvgClick, true);
                      onCleanup(() => {
                        element.removeEventListener('click', handleSvgClick, true);
                        setSvgElement(undefined);
                      });
                    }}
                    class="mx-auto block max-w-full select-none"
                    width={Math.floor(current().scene.width * current().scale)}
                    height={Math.floor(current().scene.height * current().scale)}
                    viewBox={`0 0 ${current().scene.width} ${current().scene.height}`}
                    role="group"
                    aria-label="Infrastructure map"
                    aria-describedby="infrastructure-map-keys"
                    onPointerOver={(event) => {
                      if (!isTouch(event)) setByKeys(false);
                      showFor(event);
                    }}
                    onPointerLeave={(event) => {
                      if (!isTouch(event)) setHover(null);
                    }}
                    onKeyDown={handleKey}
                    onFocusIn={handleFocusIn}
                    onFocusOut={() => {
                      if (armed() === null) setHover(null);
                    }}
                  />
                </div>
              </div>
            </Show>
          )}
        </Show>
        <Show when={fitted() && hasVisibleMatches()}>
          <p id="infrastructure-map-keys" class="sr-only">
            Arrow keys move between tiles. Home and End go to the first and last. Tab leaves the
            map.
          </p>
        </Show>
      </div>

      <Show when={scene()}>
        <ul
          ref={(element) => {
            legendRef = element;
            // The legend arrives after the first drawing, and wraps to more
            // lines as the window narrows. Either way the map gets less room.
            if (typeof ResizeObserver === 'undefined') return;
            const observer = new ResizeObserver(() => requestAnimationFrame(measure));
            observer.observe(element);
            onCleanup(() => {
              observer.disconnect();
              legendRef = undefined;
            });
          }}
          class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted"
          aria-label="Legend"
        >
          <For each={LEGEND}>
            {([variant, label]) => (
              <li class="inline-flex items-center gap-1.5">
                <StatusDot variant={variant} size="sm" />
                {label}
              </li>
            )}
          </For>
          <li class="inline-flex items-center gap-1.5">
            <span
              class="inline-block h-2 w-2 shrink-0 rounded-full border-[1.5px] border-slate-400"
              aria-hidden="true"
            />
            Off
          </li>
          <Show when={liveTraffic()}>
            <li
              class="inline-flex items-center gap-1.5"
              data-testid="infrastructure-map-legend-traffic"
            >
              <span
                class="inline-block w-4 shrink-0 border-t-2 border-dashed border-blue-600 dark:border-blue-400"
                aria-hidden="true"
              />
              Active traffic
            </li>
          </Show>
          {/* How a thing gets onto the map is not something the map can show. */}
          <li class="ml-auto">
            Apps are recognised by name, checks are drawn by their type.{' '}
            <a href={MAP_DOC_URL} class="underline underline-offset-2 hover:text-base-content">
              How the map works
            </a>
          </li>
        </ul>
      </Show>

      <InfrastructureMapDrawer
        target={drawerTarget()}
        diagram={drawerDiagram()}
        activeNetworkId={networkFilter()}
        hrefForResourceId={hrefForResourceId}
        onSelectTarget={(next) => setDrawerTarget(next)}
        onFilterByNetwork={(next) => setNetworkFilter(next)}
        onClose={closeDrawer}
      />

      {/* A tap's card is the shared pressed tooltip, which a touch screen shows. */}
      <TooltipPortal
        when={showHoverTooltip()}
        x={tooltipX()}
        y={tooltipY()}
        direction={tooltipDirection()}
      >
        <Show when={hover()}>
          {(current) => (
            <div class="flex flex-col gap-0.5">
              <span class="text-[11px] font-semibold">{current().hit.name}</span>
              <For each={hitLines(current().hit, density())}>
                {(line) => <span class="text-muted">{line}</span>}
              </For>
              {/* Every other tile opens the thing it shows. This one does not. */}
              <Show when={current().hit.checkAddress && canAddCheck()}>
                <span class="text-muted">Click to add a check</span>
              </Show>
              <Show when={byKeys() && !arrowsUsed() && hits().length > 1}>
                <span class="text-muted">Arrow keys move between tiles</span>
              </Show>
            </div>
          )}
        </Show>
      </TooltipPortal>
    </div>
  );
}

export default InfrastructureMapSurface;

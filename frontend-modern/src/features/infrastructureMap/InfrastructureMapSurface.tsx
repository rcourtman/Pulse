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
import DownloadIcon from 'lucide-solid/icons/download';
import EyeOffIcon from 'lucide-solid/icons/eye-off';
import MapIcon from 'lucide-solid/icons/map';
import {
  recordInfrastructureMapExport,
  recordInfrastructureMapOpen,
} from '@/api/infrastructureMapActivity';
import { Button, ButtonLink } from '@/components/shared/Button';
import { EmptyState } from '@/components/shared/EmptyState';
import { FilterButtonGroup } from '@/components/shared/FilterButtonGroup';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
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
import type { StatusIndicatorVariant } from '@/utils/status';
import { showError, showSuccess } from '@/utils/toast';
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
  nextMapHitIndex,
  resolveDefaultMapDensity,
  type MapDensity,
  type MapHit,
} from './infrastructureMapLayout';
import {
  anonymizeInfrastructureMapModel,
  buildInfrastructureMapModel,
} from './infrastructureMapModel';
import {
  MAP_HIT_ATTRIBUTE,
  MAP_PAGE_PALETTES,
  MAP_RINGS_ATTRIBUTE,
  renderMapLinkRings,
  renderMapSceneBody,
  renderMapTappedTile,
} from './infrastructureMapRender';
import { buildServiceDiagram } from './infrastructureMapServices';

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

// The name a new check for the gateway starts with. The owner can change it.
const UNMONITORED_CHECK_NAME = 'Gateway';

/** What a tile's card says under its name: what it is, where, what it is connected to, how busy. */
const hitLines = (hit: MapHit, density?: MapDensity): string[] =>
  [
    hit.meta,
    // The tile a full box ends in opens nothing. Say the one thing there is to do.
    hit.id.endsWith(':more') && density === 'names' ? 'Compact draws more of them' : '',
    [
      // A plain check's caption is its address, so the line above has said it.
      hit.address && !hit.meta.includes(hit.address) ? hit.address : '',
      hit.detail ? `Published ${hit.detail}` : '',
    ]
      .filter(Boolean)
      .join(' · '),
    ...(hit.links ?? []),
    [
      hit.cpu !== undefined ? `CPU ${Math.round(hit.cpu)}%` : '',
      hit.memory !== undefined ? `Memory ${Math.round(hit.memory)}%` : '',
    ]
      .filter(Boolean)
      .join(' · '),
  ].filter(Boolean);

export function InfrastructureMapSurface() {
  const { resources, loading } = useResources();
  const darkMode = useDarkMode();
  const kioskMode = useKioskMode();
  const observed = useObservedElementWidth();
  // The page offers no Settings destination this session cannot open, and
  // none at all in kiosk mode, where the shell turns Settings away.
  const canAddCheck = () => !kioskMode() && sessionCanReadAvailabilitySettings();
  const canAddSystem = () => !kioskMode() && sessionCanReadInfrastructureSettings();

  const [densityChoice, setDensityChoice] = createSignal<MapDensity | null>(null);
  const [namesHidden, setNamesHidden] = createSignal(false);
  const [exporting, setExporting] = createSignal(false);
  const [hover, setHover] = createSignal<{ hit: MapHit; x: number; y: number } | null>(null);
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
    const fit = (chosen: MapDensity) => {
      const diagram = buildServiceDiagram(model(), {
        maxTilesPerGroup: resolveDiagramTileLimit(chosen),
      });
      return {
        density: chosen,
        ...(room.phone
          ? flowServiceDiagram(diagram, room.width, chosen)
          : fitServiceDiagram(diagram, { width: room.width, height: room.height }, chosen)),
      };
    };
    const chosen = densityChoice();
    if (chosen) return fit(chosen);
    // On a phone the map scrolls, and icons alone make the shortest page. A
    // tap reads a tile. Names are there to be chosen.
    if (room.phone) return fit('marks');
    const preferred = fit(resolveDefaultMapDensity(liveModel()));
    if (preferred.density === 'marks' || preferred.scale >= MAP_READABLE_SCALE) return preferred;
    const compact = fit('marks');
    // On a very narrow window both reach the smallest size the map is drawn
    // at. Icons alone are then the shorter map, so they win a tie.
    return compact.scale >= preferred.scale ? compact : preferred;
  });
  const density = createMemo<MapDensity>(
    () => fitted()?.density ?? densityChoice() ?? resolveDefaultMapDensity(liveModel()),
  );
  const scene = createMemo(() => fitted()?.scene ?? null);
  const hits = createMemo(() => {
    const out: MapHit[] = [];
    for (const node of scene()?.nodes ?? []) if (node.kind === 'hit') out.push(node.hit);
    return out;
  });
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
    const resource = hit ? resourceById().get(hit.id) : undefined;
    return resource ? buildResourcePagePath({ id: resource.id, resource }) : null;
  };
  const body = createMemo(() => {
    const current = scene();
    return current ? renderMapSceneBody(current, palette(), hrefForHit) : '';
  });

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
  const point = (found: { hit: MapHit; element: Element }) => {
    const box = found.element.getBoundingClientRect();
    setHover({ hit: found.hit, x: box.left + box.width / 2, y: box.top });
  };
  const isTouch = (event: Event): boolean => (event as PointerEvent).pointerType === 'touch';
  const showFor = (event: Event) => {
    // A finger is not a pointer that hovers. What it touched is said on the
    // tap, below.
    if (isTouch(event)) return;
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
  // An arrow key, Home or End moves the focus to another tile. A key that
  // moves nowhere is left to the page, which scrolls with it.
  const handleKey = (event: KeyboardEvent) => {
    if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
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
    const typed = () => {
      keyedLast = true;
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
  const handleTap = (event: MouseEvent) => {
    if (pressedWith !== 'touch') return;
    const found = hitFromEvent(event);
    if (!found) return;
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
          <MetadataBadge tone="warning" size="sm" shape="pill">
            {liveModel().attentionCount.toLocaleString()} need attention
          </MetadataBadge>
        </Show>
        <Show when={!liveModel().isEmpty}>
          <div class="ml-auto flex flex-wrap items-center gap-2">
            <FilterButtonGroup
              variant="compact"
              ariaLabel="Map detail"
              options={DENSITY_OPTIONS}
              value={density()}
              onChange={(value) => setDensityChoice(value)}
            />
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
            </Show>
          }
        >
          {(current) => (
            <svg
              ref={(element) => {
                setSvgElement(element);
                // In the capture phase, so it runs before the router's own
                // click handler, which follows links.
                element.addEventListener('click', handleTap, true);
                onCleanup(() => {
                  element.removeEventListener('click', handleTap, true);
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
          )}
        </Show>
        <Show when={fitted()}>
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
          {/* How a thing gets onto the map is not something the map can show. */}
          <li class="ml-auto">
            Apps are recognised by name, checks are drawn by their type.{' '}
            <a href={MAP_DOC_URL} class="underline underline-offset-2 hover:text-base-content">
              How the map works
            </a>
          </li>
        </ul>
      </Show>

      {/* A tap's card is the shared pressed tooltip, which a touch screen shows. */}
      <TooltipPortal
        when={hover() !== null && armed() === null}
        x={hover()?.x ?? 0}
        y={hover()?.y ?? 0}
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

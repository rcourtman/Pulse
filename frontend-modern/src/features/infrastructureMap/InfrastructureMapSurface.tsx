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
import { PageHeader } from '@/components/shared/PageHeader';
import { StatusDot } from '@/components/shared/StatusDot';
import { showPressedTooltip } from '@/components/shared/Tooltip';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { buildAvailabilityTargetAddPath } from '@/components/Settings/availabilitySettingsModel';
import { buildInfrastructureWorkspacePath } from '@/components/Settings/infrastructureWorkspaceModel';
import { useDarkMode } from '@/contexts/appRuntime';
import { buildResourcePagePath } from '@/features/platformNavigation/resourcePagePath';
import { useObservedElementWidth } from '@/hooks/useObservedElementWidth';
import { useResources } from '@/hooks/useResources';
import type { StatusIndicatorVariant } from '@/utils/status';
import { showError, showSuccess } from '@/utils/toast';
import { fitServiceDiagram } from './infrastructureMapDiagram';
import {
  downloadMapImage,
  resolveDiagramTileLimit,
  resolveMapPalette,
} from './infrastructureMapExport';
import { resolveDefaultMapDensity, type MapDensity, type MapHit } from './infrastructureMapLayout';
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

// What the page keeps below the drawing: the legend and the panel's padding,
// and on a narrow window the navigation bar fixed to its bottom edge.
const SPACE_BELOW = 60;
const SPACE_BELOW_NARROW = 84;
const NARROW_WINDOW = 768;
const MIN_MAP_HEIGHT = 240;
// Below this, names are too small to read and icons alone say more.
const READABLE_SCALE = 0.62;

// The name a new check for the gateway starts with. The owner can change it.
const UNMONITORED_CHECK_NAME = 'Gateway';

/** What a tile's card says under its name: what it is, where, what it is connected to, how busy. */
const hitLines = (hit: MapHit): string[] =>
  [
    hit.meta,
    [hit.address, hit.detail ? `Published ${hit.detail}` : ''].filter(Boolean).join(' · '),
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
  const observed = useObservedElementWidth();

  const [densityChoice, setDensityChoice] = createSignal<MapDensity | null>(null);
  const [namesHidden, setNamesHidden] = createSignal(false);
  const [exporting, setExporting] = createSignal(false);
  const [hover, setHover] = createSignal<{ hit: MapHit; x: number; y: number } | null>(null);
  // The tile a tap on a touch screen is showing, by its id.
  const [armed, setArmed] = createSignal<string | null>(null);

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
  const [space, setSpace] = createSignal<{ width: number; height: number } | null>(null);
  const measure = () => {
    const width = observed.width();
    if (!frameRef || !width) return;
    // Where the map starts when nothing is scrolled. The app scrolls inside
    // its own shell, not the document, so every box above is counted.
    let top = frameRef.getBoundingClientRect().top;
    for (let box = frameRef.parentElement; box; box = box.parentElement) top += box.scrollTop;
    const below = SPACE_BELOW + (window.innerWidth < NARROW_WINDOW ? SPACE_BELOW_NARROW : 0);
    const height = Math.max(MIN_MAP_HEIGHT, Math.floor(window.innerHeight - top - below));
    setSpace((was) => (was?.width === width && was.height === height ? was : { width, height }));
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
    onCleanup(() => {
      cancelAnimationFrame(queued);
      observer.disconnect();
    });
  });

  const fitted = createMemo(() => {
    const room = space();
    if (!room || model().isEmpty) return null;
    const fit = (chosen: MapDensity) => ({
      density: chosen,
      ...fitServiceDiagram(
        buildServiceDiagram(model(), { maxTilesPerGroup: resolveDiagramTileLimit(chosen) }),
        room,
        chosen,
      ),
    });
    const chosen = densityChoice();
    if (chosen) return fit(chosen);
    const preferred = fit(resolveDefaultMapDensity(liveModel()));
    if (preferred.density === 'marks' || preferred.scale >= READABLE_SCALE) return preferred;
    const compact = fit('marks');
    return compact.scale > preferred.scale ? compact : preferred;
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
      return buildAvailabilityTargetAddPath('device', {
        name: UNMONITORED_CHECK_NAME,
        address: hit.checkAddress,
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
      const focusedId = focused
        ? previousHits?.[Number(focused.getAttribute(MAP_HIT_ATTRIBUTE))]?.id
        : undefined;
      svg.innerHTML = markup;
      if (focusedId) {
        const index = hits().findIndex((hit) => hit.id === focusedId);
        svg
          .querySelector<SVGElement>(`[${MAP_HIT_ATTRIBUTE}="${index}"]`)
          ?.focus({ preventScroll: true });
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

  // A touch screen has no hover, and a tile is a link, so a tap would open
  // the tile before anyone could read what it is. There the first tap says
  // what the tile is and rings what it is connected to, as pointing does,
  // and a second tap on the same tile opens it.
  let pressedWith = '';
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
      armedAtPress = armed();
      disarm();
    };
    document.addEventListener('pointerdown', press, true);
    window.addEventListener('scroll', disarm, true);
    onCleanup(() => {
      document.removeEventListener('pointerdown', press, true);
      window.removeEventListener('scroll', disarm, true);
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
        ...hitLines(found.hit),
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
    <div class="flex flex-col gap-3" data-testid="infrastructure-map">
      <div class="flex flex-wrap items-center gap-x-3 gap-y-2">
        {/* AppLayout already titles the tab for this route with the brand name. */}
        <PageHeader title="Map" updateDocumentTitle={false} />
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
                description="The map draws itself from the systems Pulse monitors. Add one and it appears here."
                actions={
                  <ButtonLink
                    href={buildInfrastructureWorkspacePath()}
                    variant="secondary"
                    size="sm"
                  >
                    Add a system
                  </ButtonLink>
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
              onPointerOver={showFor}
              onPointerLeave={(event) => {
                if (!isTouch(event)) setHover(null);
              }}
              onFocusIn={showFor}
              onFocusOut={() => {
                if (armed() === null) setHover(null);
              }}
            />
          )}
        </Show>
      </div>

      <Show when={scene()}>
        <ul
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
          <li class="ml-auto">
            Apps are recognised by their image or name. The Internet is drawn but not monitored.
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
              <For each={hitLines(current().hit)}>
                {(line) => <span class="text-muted">{line}</span>}
              </For>
              {/* Every other tile opens the thing it shows. This one does not. */}
              <Show when={current().hit.checkAddress}>
                <span class="text-muted">Click to add a check</span>
              </Show>
            </div>
          )}
        </Show>
      </TooltipPortal>
    </div>
  );
}

export default InfrastructureMapSurface;

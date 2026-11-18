import { For, Show, createEffect, createMemo, createSignal, on } from 'solid-js';
import { A } from '@solidjs/router';
import DownloadIcon from 'lucide-solid/icons/download';
import EyeOffIcon from 'lucide-solid/icons/eye-off';
import MapIcon from 'lucide-solid/icons/map';
import { EmptyState } from '@/components/shared/EmptyState';
import { FilterButtonGroup } from '@/components/shared/FilterButtonGroup';
import { PageHeader } from '@/components/shared/PageHeader';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { buildInfrastructureWorkspacePath } from '@/components/Settings/infrastructureWorkspaceModel';
import { useDarkMode } from '@/contexts/appRuntime';
import { buildResourcePagePath } from '@/features/platformNavigation/resourcePagePath';
import { useObservedElementWidth } from '@/hooks/useObservedElementWidth';
import { useResources } from '@/hooks/useResources';
import { showError, showSuccess } from '@/utils/toast';
import { layoutServiceDiagram } from './infrastructureMapDiagram';
import { downloadMapImage, resolveDiagramTileLimit } from './infrastructureMapExport';
import {
  layoutInfrastructureMap,
  layoutInfrastructureNetworkMap,
  resolveDefaultMapDensity,
  type MapDensity,
  type MapHit,
  type MapView,
} from './infrastructureMapLayout';
import {
  anonymizeInfrastructureMapModel,
  buildInfrastructureMapModel,
  formatMapTotalLabel,
  type MapHealth,
} from './infrastructureMapModel';
import { buildServiceDiagram } from './infrastructureMapServices';
import {
  MAP_FONT_FAMILY,
  MAP_HIT_ATTRIBUTE,
  MAP_PALETTES,
  renderMapSceneBody,
} from './infrastructureMapRender';

const TOOLBAR_BUTTON_CLASS =
  'inline-flex h-7 items-center gap-1.5 rounded-md border border-border px-2.5 text-xs font-medium transition-colors focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 disabled:cursor-wait disabled:opacity-60';

const LEGEND: Array<[MapHealth, string]> = [
  ['ok', 'Healthy'],
  ['attention', 'Needs attention'],
  ['critical', 'Critical'],
  ['off', 'Off'],
  ['stale', 'Not reporting'],
];

const VIEW_OPTIONS: Array<{ value: MapView; label: string; title: string }> = [
  {
    value: 'services',
    label: 'Services',
    title: 'What the estate runs, boxed by what each thing is for',
  },
  {
    value: 'networks',
    label: 'Connections',
    title: 'Each network as a line, with every machine tied to the networks it is on',
  },
  { value: 'hosts', label: 'Hosts', title: 'Each host with everything that runs on it' },
];

const DENSITY_OPTIONS: Array<{ value: MapDensity; label: string; title: string }> = [
  { value: 'names', label: 'Names', title: 'Write the name of every guest and container' },
  { value: 'marks', label: 'Compact', title: 'Draw each guest and container as one square' },
];

export function InfrastructureMapSurface() {
  const { resources, loading } = useResources();
  const darkMode = useDarkMode();
  const observed = useObservedElementWidth();

  const [viewChoice, setViewChoice] = createSignal<MapView>('services');
  const [densityChoice, setDensityChoice] = createSignal<MapDensity | null>(null);
  const [namesHidden, setNamesHidden] = createSignal(false);
  const [exporting, setExporting] = createSignal(false);
  const [hover, setHover] = createSignal<{ hit: MapHit; x: number; y: number } | null>(null);

  const resourceById = createMemo(
    () => new Map(resources().map((resource) => [resource.id, resource] as const)),
  );
  const liveModel = createMemo(() => buildInfrastructureMapModel(resources()));
  const model = createMemo(() =>
    namesHidden() ? anonymizeInfrastructureMapModel(liveModel()) : liveModel(),
  );
  const density = createMemo(() => densityChoice() ?? resolveDefaultMapDensity(liveModel()));
  // With no address reported anywhere there are no networks to draw.
  const hasNetworks = createMemo(() => liveModel().segments.length > 0);
  const viewOptions = createMemo(() =>
    VIEW_OPTIONS.filter((option) => option.value !== 'networks' || hasNetworks()),
  );
  const view = createMemo<MapView>(() =>
    viewChoice() === 'networks' && !hasNetworks() ? 'services' : viewChoice(),
  );
  const palette = createMemo(() => (darkMode() ? MAP_PALETTES.dark : MAP_PALETTES.light));

  const scene = createMemo(() => {
    const width = observed.width();
    if (!width || model().isEmpty) return null;
    const options = { width, density: density() };
    if (view() === 'services') {
      const diagram = buildServiceDiagram(model(), {
        maxTilesPerGroup: resolveDiagramTileLimit(density()),
      });
      return layoutServiceDiagram(diagram, options);
    }
    const layout = view() === 'networks' ? layoutInfrastructureNetworkMap : layoutInfrastructureMap;
    return layout(model(), options);
  });
  const hits = createMemo(() => {
    const out: MapHit[] = [];
    for (const node of scene()?.nodes ?? []) if (node.kind === 'hit') out.push(node.hit);
    return out;
  });
  const hrefForHit = (index: number): string | null => {
    const hit = hits()[index];
    const resource = hit ? resourceById().get(hit.id) : undefined;
    return resource ? buildResourcePagePath({ id: resource.id, resource }) : null;
  };
  const body = createMemo(() => {
    const current = scene();
    return current ? renderMapSceneBody(current, palette(), hrefForHit) : '';
  });

  let svgRef: SVGSVGElement | undefined;
  // The drawing is replaced whole on every refresh. Keep a keyboard user's
  // place by handing focus back to the same resource afterwards.
  createEffect(
    on(body, (markup, _previousMarkup, previousHits: MapHit[] | undefined) => {
      const svg = svgRef;
      if (!svg) return hits();
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

  const hitFromEvent = (event: Event): { hit: MapHit; element: Element } | null => {
    const element = (event.target as Element | null)?.closest?.(`[${MAP_HIT_ATTRIBUTE}]`);
    if (!element) return null;
    const hit = hits()[Number(element.getAttribute(MAP_HIT_ATTRIBUTE))];
    return hit ? { hit, element } : null;
  };
  const showFor = (event: Event) => {
    const found = hitFromEvent(event);
    if (!found) {
      setHover(null);
      return;
    }
    const box = found.element.getBoundingClientRect();
    setHover({ hit: found.hit, x: box.left + box.width / 2, y: box.top });
  };

  const handleDownload = async () => {
    if (exporting()) return;
    setExporting(true);
    try {
      const format = await downloadMapImage(model(), {
        title: 'Infrastructure map',
        view: view(),
        density: density(),
        palette: palette(),
      });
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
        <ul class="flex flex-wrap items-center gap-1.5" aria-label="What is on the map">
          <For each={liveModel().totals}>
            {(total) => (
              <li class="inline-flex items-center gap-1 rounded-full bg-surface-alt px-2 py-0.5 text-[11px] text-muted ring-1 ring-border-subtle">
                <span class="font-semibold text-base-content">{total.count.toLocaleString()}</span>
                {formatMapTotalLabel(total)}
              </li>
            )}
          </For>
          <Show when={liveModel().attentionCount > 0}>
            <li class="inline-flex items-center gap-1 rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:bg-amber-900/25 dark:text-amber-200">
              <span class="font-semibold">{liveModel().attentionCount.toLocaleString()}</span>
              need attention
            </li>
          </Show>
        </ul>
        <Show when={!liveModel().isEmpty}>
          <div class="ml-auto flex flex-wrap items-center gap-2">
            <FilterButtonGroup
              variant="compact"
              ariaLabel="What the map shows"
              options={viewOptions()}
              value={view()}
              onChange={(value) => setViewChoice(value)}
            />
            <FilterButtonGroup
              variant="compact"
              ariaLabel="Map detail"
              options={DENSITY_OPTIONS}
              value={density()}
              onChange={(value) => setDensityChoice(value)}
            />
            <button
              type="button"
              class={`${TOOLBAR_BUTTON_CLASS} ${
                namesHidden()
                  ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/25 dark:text-blue-300'
                  : 'bg-surface text-base-content hover:bg-surface-hover'
              }`}
              aria-pressed={namesHidden()}
              title="Replace every name with what the thing is, so the image is safe to post"
              onClick={() => setNamesHidden((hidden) => !hidden)}
            >
              <EyeOffIcon class="h-3.5 w-3.5" aria-hidden="true" />
              Hide names
            </button>
            <button
              type="button"
              class={`${TOOLBAR_BUTTON_CLASS} bg-surface text-base-content hover:bg-surface-hover`}
              disabled={exporting()}
              title="Save this map as an image file"
              onClick={() => void handleDownload()}
            >
              <DownloadIcon class="h-3.5 w-3.5" aria-hidden="true" />
              Save image
            </button>
          </div>
        </Show>
      </div>

      <div ref={observed.setElement} class="min-w-0">
        <Show
          when={scene()}
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
                  <A
                    href={buildInfrastructureWorkspacePath()}
                    class={`${TOOLBAR_BUTTON_CLASS} bg-surface text-base-content hover:bg-surface-hover`}
                  >
                    Add a system
                  </A>
                }
              />
            </Show>
          }
        >
          {(current) => (
            <svg
              ref={svgRef}
              class="block max-w-full select-none"
              width={current().width}
              height={current().height}
              viewBox={`0 0 ${current().width} ${current().height}`}
              font-family={MAP_FONT_FAMILY}
              role="group"
              aria-label="Infrastructure map"
              onPointerOver={showFor}
              onPointerLeave={() => setHover(null)}
              onFocusIn={showFor}
              onFocusOut={() => setHover(null)}
            />
          )}
        </Show>
      </div>

      <Show when={scene()}>
        <ul
          class="flex flex-wrap items-center gap-x-4 gap-y-1 text-[11px] text-muted"
          aria-label="Legend"
        >
          <For each={LEGEND}>
            {([health, label]) => (
              <li class="inline-flex items-center gap-1.5">
                <svg class="h-2.5 w-2.5" viewBox="0 0 10 10" aria-hidden="true">
                  <Show
                    when={health !== 'off'}
                    fallback={
                      <rect
                        x="1"
                        y="1"
                        width="8"
                        height="8"
                        rx="2"
                        fill="none"
                        stroke={palette().off}
                        stroke-width="1.5"
                      />
                    }
                  >
                    <rect width="10" height="10" rx="2" fill={palette()[health]} />
                  </Show>
                </svg>
                {label}
              </li>
            )}
          </For>
          <li class="ml-auto">
            {view() === 'services'
              ? 'Apps are recognised by their image or name. The Internet is drawn but not monitored.'
              : view() === 'networks'
                ? 'Each line is a network. Machines are placed by the addresses they report. Pulse does not see firewall rules.'
                : 'Everything here is read from what Pulse monitors. Select anything to open it.'}
          </li>
        </ul>
      </Show>

      <TooltipPortal when={hover() !== null} x={hover()?.x ?? 0} y={hover()?.y ?? 0}>
        <Show when={hover()}>
          {(current) => (
            <div class="flex flex-col gap-0.5">
              <span class="text-[11px] font-semibold">{current().hit.name}</span>
              <span class="text-muted">{current().hit.meta}</span>
              <Show when={current().hit.address || current().hit.detail}>
                <span class="text-muted">
                  {[
                    current().hit.address,
                    current().hit.detail ? `Published ${current().hit.detail}` : '',
                  ]
                    .filter(Boolean)
                    .join(' · ')}
                </span>
              </Show>
              <Show when={current().hit.cpu !== undefined || current().hit.memory !== undefined}>
                <span class="text-muted">
                  {[
                    current().hit.cpu !== undefined ? `CPU ${Math.round(current().hit.cpu!)}%` : '',
                    current().hit.memory !== undefined
                      ? `Memory ${Math.round(current().hit.memory!)}%`
                      : '',
                  ]
                    .filter(Boolean)
                    .join(' · ')}
                </span>
              </Show>
            </div>
          )}
        </Show>
      </TooltipPortal>
    </div>
  );
}

export default InfrastructureMapSurface;

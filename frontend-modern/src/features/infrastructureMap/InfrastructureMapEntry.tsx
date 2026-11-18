import { A, useLocation } from '@solidjs/router';
import MapIcon from 'lucide-solid/icons/map';
import { INFRASTRUCTURE_MAP_PATH } from '@/routing/resourceLinks';
import { preloadRouteModule } from '@/routing/routePreload';

/**
 * The way into the map. It is not a platform tab, because the map is every
 * platform at once: on wide screens it is a tab on the left edge of the
 * window, beside whatever page is open, and on narrow ones a button in the
 * header, where there is no edge to spare.
 */

const warm = () => void preloadRouteModule(INFRASTRUCTURE_MAP_PATH).catch(() => undefined);

const useMapIsOpen = () => {
  const location = useLocation();
  return () => location.pathname.replace(/\/+$/, '') === INFRASTRUCTURE_MAP_PATH;
};

export function InfrastructureMapEdgeTab() {
  const isOpen = useMapIsOpen();
  return (
    <A
      href={INFRASTRUCTURE_MAP_PATH}
      class={`fixed left-0 top-1/2 z-30 hidden -translate-y-1/2 flex-col items-center gap-1.5 rounded-r-md border border-l-0 py-2.5 pl-0.5 pr-1 text-[11px] font-semibold shadow-sm transition-colors focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 xl:flex ${
        isOpen()
          ? 'border-blue-300 bg-blue-100 text-blue-700 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-300'
          : 'border-border bg-surface text-muted hover:bg-surface-hover hover:text-base-content'
      }`}
      aria-label="Map"
      aria-current={isOpen() ? 'page' : undefined}
      title="Map of everything Pulse monitors"
      onMouseEnter={warm}
      onFocus={warm}
      data-testid="infrastructure-map-edge-tab"
    >
      <MapIcon class="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
      <span class="[writing-mode:vertical-rl]" aria-hidden="true">
        Map
      </span>
    </A>
  );
}

export function InfrastructureMapHeaderButton() {
  const isOpen = useMapIsOpen();
  return (
    <A
      href={INFRASTRUCTURE_MAP_PATH}
      class={`flex h-9 w-9 items-center justify-center rounded-full text-xs transition focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 sm:h-10 sm:w-10 xl:hidden ${
        isOpen()
          ? 'bg-blue-100 text-blue-700 hover:bg-blue-200 dark:bg-blue-900/25 dark:text-blue-300 dark:hover:bg-blue-800'
          : 'bg-surface-hover text-base-content hover:bg-border'
      }`}
      aria-label="Map"
      aria-current={isOpen() ? 'page' : undefined}
      title="Map of everything Pulse monitors"
      data-testid="infrastructure-map-header-button"
    >
      <MapIcon class="h-4 w-4 shrink-0" aria-hidden="true" />
    </A>
  );
}

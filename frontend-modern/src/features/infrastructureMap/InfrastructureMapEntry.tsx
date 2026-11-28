import { A, useLocation } from '@solidjs/router';
import MapIcon from 'lucide-solid/icons/map';
import { INFRASTRUCTURE_MAP_PATH } from '@/routing/resourceLinks';
import { preloadRouteModule } from '@/routing/routePreload';

/**
 * The way into the map. On desktop (`lg` and above) it sits at the leftmost
 * position of the primary navigation bar so whole-estate overview leads into
 * individual platforms. On narrower screens (`< lg`, where bottom navigation
 * takes over) it stays as a compact button in the top-left header slot so it
 * remains one tap away without crowding the mobile bottom bar or turning a
 * single-platform install into a two-item switcher.
 */

const warm = () => void preloadRouteModule(INFRASTRUCTURE_MAP_PATH).catch(() => undefined);

const useMapIsOpen = () => {
  const location = useLocation();
  return () => location.pathname.replace(/\/+$/, '') === INFRASTRUCTURE_MAP_PATH;
};

const DESKTOP_TAB_BASE_CLASSES =
  'tab relative px-1 xl:px-2 2xl:px-3 py-1.5 text-xs xl:text-sm font-medium flex items-center gap-1 2xl:gap-1.5 rounded-t border border-transparent transition-colors whitespace-nowrap cursor-pointer select-none focus-visible:outline-solid focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-blue-500';

export function InfrastructureMapNavTab() {
  const isOpen = useMapIsOpen();
  return (
    <A
      href={INFRASTRUCTURE_MAP_PATH}
      class={`${DESKTOP_TAB_BASE_CLASSES} ${
        isOpen()
          ? 'bg-surface text-blue-600 dark:text-blue-400 border-border border-b border-b-surface shadow-xs font-semibold'
          : 'text-muted hover:text-base-content hover:bg-surface-hover'
      }`}
      aria-current={isOpen() ? 'page' : undefined}
      title="Map of everything Pulse monitors"
      onMouseEnter={warm}
      onFocus={warm}
      data-testid="infrastructure-map-nav-tab"
    >
      <MapIcon class="w-4 h-4 shrink-0" aria-hidden="true" />
      <span>Map</span>
    </A>
  );
}

export function InfrastructureMapHeaderButton() {
  const isOpen = useMapIsOpen();
  return (
    <A
      href={INFRASTRUCTURE_MAP_PATH}
      class={`inline-flex h-9 items-center gap-1.5 rounded-md border px-2.5 text-xs font-medium whitespace-nowrap shadow-xs transition-colors focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 sm:px-3 sm:text-sm ${
        isOpen()
          ? 'border-blue-300 bg-blue-100 text-blue-700 hover:bg-blue-200/70 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-300 dark:hover:bg-blue-900/60'
          : 'border-border bg-surface text-base-content hover:bg-surface-hover'
      }`}
      aria-current={isOpen() ? 'page' : undefined}
      title="Map of everything Pulse monitors"
      onMouseEnter={warm}
      onFocus={warm}
      data-testid="infrastructure-map-header-button"
    >
      <MapIcon class="h-4 w-4 shrink-0" aria-hidden="true" />
      <span>Map</span>
    </A>
  );
}

import { A, useLocation } from '@solidjs/router';
import MapIcon from 'lucide-solid/icons/map';
import { INFRASTRUCTURE_MAP_PATH } from '@/routing/resourceLinks';
import { preloadRouteModule } from '@/routing/routePreload';

/**
 * The way into the map. It is not a platform tab, because the map is every
 * platform at once: it sits in the top-left header slot across all viewports,
 * beside the centered brand mark rather than mixed into individual platform tabs.
 */

const warm = () => void preloadRouteModule(INFRASTRUCTURE_MAP_PATH).catch(() => undefined);

const useMapIsOpen = () => {
  const location = useLocation();
  return () => location.pathname.replace(/\/+$/, '') === INFRASTRUCTURE_MAP_PATH;
};

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

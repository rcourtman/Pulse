import { For, Show, onCleanup, onMount } from 'solid-js';
import ChevronsUpDownIcon from 'lucide-solid/icons/chevrons-up-down';
import EllipsisIcon from 'lucide-solid/icons/ellipsis';
import { type MobileNavBarProps, useMobileNavBarState } from './useMobileNavBarState';
import {
  getMobileNavAlertBadgeCounts,
  getMobileNavDestinationKey,
  getMobileNavDestinationHref,
  getMobileNavTabAriaLabel,
  getMobileNavTabButtonClass,
  isMobileNavDestinationActive,
  mobileNavDestinationHasBadge,
  type MobileNavBarDestination,
} from './mobileNavBarModel';

export type {
  MobileNavBarProps,
  MobileNavBarPrimaryTab,
  MobileNavBarUtilityTab,
} from './mobileNavBarModel';

const MOBILE_NAV_OVERFLOW_ID = 'mobile-navigation-overflow';
const MOBILE_NAV_PLATFORM_SWITCHER_ID = 'mobile-navigation-platform-switcher';

function MobileNavDestinationContent(props: {
  destination: MobileNavBarDestination;
  iconClass: string;
}) {
  const tab = () => props.destination.tab;
  const utilityTab = () =>
    props.destination.kind === 'utility' ? props.destination.tab : undefined;
  const alertBadges = () => {
    const utility = utilityTab();
    return utility ? getMobileNavAlertBadgeCounts(utility) : null;
  };
  const genericCount = () => {
    const utility = utilityTab();
    if (!utility || utility.id === 'alerts' || !utility.count || utility.count <= 0) return null;
    return utility.count;
  };
  const Icon = tab().icon;

  return (
    <>
      <span class="relative flex shrink-0 items-center justify-center">
        <span aria-hidden="true" class="inline-flex items-center justify-center">
          <Icon class={props.iconClass} />
        </span>
        <Show when={alertBadges()}>
          {(badges) => (
            <span aria-hidden="true" class="absolute -right-2 -top-1 flex items-center gap-1">
              <Show when={badges().critical > 0}>
                <span class="inline-flex h-4 min-w-[16px] items-center justify-center rounded-full bg-red-600 px-1 text-[10px] font-bold text-white">
                  {badges().critical}
                </span>
              </Show>
              <Show when={badges().warning > 0}>
                <span class="inline-flex h-4 min-w-[16px] items-center justify-center rounded-full bg-amber-200 px-1 text-[10px] font-semibold text-amber-800">
                  {badges().warning}
                </span>
              </Show>
            </span>
          )}
        </Show>
        <Show when={genericCount()}>
          {(count) => (
            <span
              aria-hidden="true"
              class="absolute -right-2 -top-1 inline-flex h-4 min-w-[16px] items-center justify-center rounded-full bg-amber-200 px-1 text-[10px] font-semibold text-amber-800"
            >
              {count()}
            </span>
          )}
        </Show>
      </span>
      <span class="min-w-0 truncate">{tab().label}</span>
      <Show when={props.destination.kind === 'primary' && !props.destination.tab.enabled}>
        <span class="rounded-full bg-amber-100 px-1.5 py-0.5 text-[9px] font-semibold text-amber-700 dark:bg-amber-900 dark:text-amber-200">
          Setup
        </span>
      </Show>
      <Show when={props.destination.kind === 'primary' && props.destination.tab.badge}>
        <span class="rounded-full bg-surface-hover px-1.5 py-0.5 text-[9px] font-semibold text-muted">
          {props.destination.kind === 'primary' ? props.destination.tab.badge : null}
        </span>
      </Show>
      <Show when={utilityTab()?.badge === 'update'}>
        <span aria-hidden="true" class="h-1.5 w-1.5 rounded-full bg-red-500"></span>
      </Show>
      <Show when={utilityTab()?.badge === 'pro'}>
        <span class="rounded-full bg-blue-100 px-1.5 py-0.5 text-[9px] font-semibold text-blue-700 dark:bg-blue-900 dark:text-blue-300">
          Pro
        </span>
      </Show>
    </>
  );
}

export function MobileNavBar(props: MobileNavBarProps) {
  const mobileNav = useMobileNavBarState(props);
  const tabIconClass = 'h-3.5 w-3.5 shrink-0';

  // This bar's height is content-driven and includes the safe-area inset via
  // pb-safe, so anything that has to sit on top of it cannot hardcode a value.
  // Five call sites did, all at 5rem against a bar that measures ~45px, which
  // left a band below the Assistant backdrop that was neither dimmed nor
  // click-blocked. Publish the measured height instead and let them read it.
  // Reports 0 when the bar is hidden at xl, which is the correct clearance
  // there.
  let navElement: HTMLElement | undefined;
  let navObserver: ResizeObserver | undefined;
  const publishNavHeight = () => {
    if (!navElement) return;
    document.documentElement.style.setProperty(
      '--pulse-mobile-nav-height',
      `${navElement.offsetHeight}px`,
    );
  };
  // Publish from onMount, not from the ref: Solid runs refs before the node is
  // in the document, where offsetHeight is 0. Reading it after mount forces
  // layout and yields the real height, so the first paint is already correct
  // without waiting for an observer callback.
  onMount(() => {
    publishNavHeight();
    if (typeof ResizeObserver !== 'undefined' && navElement) {
      navObserver = new ResizeObserver(publishNavHeight);
      navObserver.observe(navElement);
    }
    // The observer covers content-driven height changes, but the bar is
    // xl:hidden, so crossing that breakpoint swaps between its real height and
    // 0 via display alone. Observing a display:none element is the least
    // consistent case across engines, so listen for resize too rather than
    // leaving a stale clearance behind after a rotation or window drag.
    window.addEventListener('resize', publishNavHeight);
  });
  onCleanup(() => {
    navObserver?.disconnect();
    window.removeEventListener('resize', publishNavHeight);
    document.documentElement.style.removeProperty('--pulse-mobile-nav-height');
  });
  const overflowHasBadge = () =>
    mobileNav.overflowDestinations().some(mobileNavDestinationHasBadge);
  const destinationHref = (destination: MobileNavBarDestination) =>
    getMobileNavDestinationHref(destination, props.getPrimaryHref);
  const handleDestinationLinkClick = (event: MouseEvent, destination: MobileNavBarDestination) => {
    // Preserve native link behavior for new tabs/windows and context-menu
    // actions. Plain activation stays in the SPA and retains route memory.
    if (
      event.defaultPrevented ||
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey
    ) {
      return;
    }
    event.preventDefault();
    mobileNav.handleDestinationClick(destination);
  };

  const renderOverflowGroup = (options: {
    label: string;
    destinations: () => MobileNavBarDestination[];
  }) => (
    <Show when={options.destinations().length > 0}>
      <div role="group" aria-label={options.label}>
        <div class="px-3 pb-1 pt-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
          {options.label}
        </div>
        <For each={options.destinations()}>
          {(destination) => {
            const active = () => isMobileNavDestinationActive(destination, props.activeTab());
            const enabled = () =>
              destination.kind === 'primary' ? destination.tab.enabled : undefined;

            return (
              <a
                href={destinationHref(destination)}
                role="menuitem"
                tabIndex={-1}
                data-tab-id={destination.tab.id}
                data-mobile-nav-destination={getMobileNavDestinationKey(destination)}
                aria-current={active() ? 'page' : undefined}
                aria-label={
                  destination.kind === 'utility'
                    ? getMobileNavTabAriaLabel(destination.tab)
                    : undefined
                }
                onClick={(event) => handleDestinationLinkClick(event, destination)}
                title={destination.tab.tooltip}
                class={`flex min-h-11 w-full items-center gap-3 rounded-md px-3 py-2 text-left text-sm transition-colors ${
                  active()
                    ? 'bg-blue-50 font-semibold text-blue-700 dark:bg-blue-900 dark:text-blue-300'
                    : 'text-base-content hover:bg-surface-hover'
                } ${enabled() === false ? 'opacity-70' : ''}`.trim()}
              >
                <MobileNavDestinationContent destination={destination} iconClass={tabIconClass} />
              </a>
            );
          }}
        </For>
      </div>
    </Show>
  );

  return (
    <nav
      ref={(element) => (navElement = element)}
      class="fixed inset-x-0 bottom-0 z-40 border-t border-border bg-surface pb-safe xl:hidden"
      aria-label="Mobile navigation"
    >
      <Show when={mobileNav.isOverflowOpen()}>
        <div
          id={MOBILE_NAV_OVERFLOW_ID}
          ref={mobileNav.setOverflowMenuRef}
          role="menu"
          aria-label="More navigation destinations"
          onKeyDown={mobileNav.handleOverflowMenuKeyDown}
          class="absolute inset-x-2 bottom-full mb-2 max-h-[70vh] overflow-y-auto rounded-lg border border-border bg-surface p-1 shadow-xl"
        >
          {renderOverflowGroup({
            label: 'Pulse',
            destinations: mobileNav.overflowUtilityDestinations,
          })}
        </div>
      </Show>

      <Show when={mobileNav.isPlatformMenuOpen()}>
        <div
          id={MOBILE_NAV_PLATFORM_SWITCHER_ID}
          ref={mobileNav.setPlatformMenuRef}
          role="menu"
          aria-label="Switch platform"
          onKeyDown={mobileNav.handlePlatformMenuKeyDown}
          class="absolute inset-x-2 bottom-full mb-2 max-h-[70vh] overflow-y-auto rounded-lg border border-border bg-surface p-1 shadow-xl"
        >
          {renderOverflowGroup({
            label: 'Infrastructure',
            destinations: mobileNav.platformDestinations,
          })}
        </div>
      </Show>

      <div data-mobile-nav-rail="fixed" class="flex min-w-0 items-stretch gap-0.5 px-0.5 py-0.5">
        <Show when={mobileNav.activePlatformDestination()}>
          {(destination) => {
            const canSwitch = () => mobileNav.platformDestinations().length > 1;
            const currentLabel = () => destination().tab.label;

            const content = () => (
              <MobileNavDestinationContent destination={destination()} iconClass={tabIconClass} />
            );
            const className = () =>
              getMobileNavTabButtonClass({
                active: mobileNav.platformIsActive(),
                enabled: destination().tab.enabled,
              });

            return (
              <Show
                when={canSwitch()}
                fallback={
                  <a
                    href={destinationHref(destination())}
                    data-tab-id="platform-switcher"
                    data-mobile-nav-destination="platform-switcher"
                    aria-label={currentLabel()}
                    aria-current={mobileNav.platformIsActive() ? 'page' : undefined}
                    onClick={(event) => handleDestinationLinkClick(event, destination())}
                    title={destination().tab.tooltip}
                    class={className()}
                  >
                    {content()}
                  </a>
                }
              >
                <button
                  ref={mobileNav.setPlatformTriggerRef}
                  type="button"
                  data-tab-id="platform-switcher"
                  data-mobile-nav-destination="platform-switcher"
                  aria-label={`Switch platform, current ${currentLabel()}`}
                  aria-haspopup="menu"
                  aria-expanded={mobileNav.isPlatformMenuOpen()}
                  aria-controls={MOBILE_NAV_PLATFORM_SWITCHER_ID}
                  aria-current={mobileNav.platformIsActive() ? 'page' : undefined}
                  onClick={mobileNav.handlePlatformTriggerClick}
                  onKeyDown={mobileNav.handlePlatformTriggerKeyDown}
                  title="Switch platform"
                  class={className()}
                >
                  {content()}
                  <ChevronsUpDownIcon
                    aria-hidden="true"
                    class="absolute right-1 top-1 h-2.5 w-2.5 text-muted"
                  />
                </button>
              </Show>
            );
          }}
        </Show>

        <For each={mobileNav.fixedDestinations()}>
          {(destination) => {
            const active = () => isMobileNavDestinationActive(destination, props.activeTab());
            const enabled = () =>
              destination.kind === 'primary' ? destination.tab.enabled : undefined;

            return (
              <a
                href={destinationHref(destination)}
                data-tab-id={destination.tab.id}
                data-mobile-nav-destination={getMobileNavDestinationKey(destination)}
                aria-current={active() ? 'page' : undefined}
                aria-label={
                  destination.kind === 'utility'
                    ? getMobileNavTabAriaLabel(destination.tab)
                    : undefined
                }
                onClick={(event) => handleDestinationLinkClick(event, destination)}
                title={destination.tab.tooltip}
                class={getMobileNavTabButtonClass({ active: active(), enabled: enabled() })}
              >
                <MobileNavDestinationContent destination={destination} iconClass={tabIconClass} />
              </a>
            );
          }}
        </For>

        <Show when={mobileNav.overflowDestinations().length > 0}>
          <button
            ref={mobileNav.setOverflowTriggerRef}
            type="button"
            data-tab-id="more"
            aria-label="More navigation"
            aria-haspopup="menu"
            aria-expanded={mobileNav.isOverflowOpen()}
            aria-controls={MOBILE_NAV_OVERFLOW_ID}
            aria-current={mobileNav.overflowIsActive() ? 'page' : undefined}
            onClick={mobileNav.handleOverflowTriggerClick}
            onKeyDown={mobileNav.handleOverflowTriggerKeyDown}
            title="More navigation destinations"
            class={getMobileNavTabButtonClass({ active: mobileNav.overflowIsActive() })}
          >
            <span class="relative flex items-center justify-center">
              <EllipsisIcon aria-hidden="true" class={tabIconClass} />
              <Show when={overflowHasBadge()}>
                <span
                  aria-hidden="true"
                  class="absolute -right-1.5 -top-1 h-2 w-2 rounded-full bg-red-500 ring-2 ring-surface"
                ></span>
              </Show>
            </span>
            <span>More</span>
          </button>
        </Show>
      </div>
    </nav>
  );
}

export default MobileNavBar;

import { createMemo, type Accessor } from 'solid-js';
import type { MobileNavBarPrimaryTab, MobileNavBarUtilityTab } from './mobileNavBarModel';

/**
 * Reuse previous item references (and the previous array itself) when a
 * rebuilt tab list is structurally unchanged. The nav tab memos rebuild
 * their arrays from live store reads on every websocket tick, and the
 * reference-keyed <For> consumers in AppLayout and MobileNavBar tear down
 * and recreate every button whenever item identity changes — which drops
 * taps that land mid-rebuild. Stabilizing identity here fixes every
 * consumer at once.
 */
export function createStableTabList<T>(
  source: Accessor<T[]>,
  equals: (previous: T, next: T) => boolean,
): Accessor<T[]> {
  let previous: T[] = [];
  return createMemo(() => {
    const next = source();
    let changed = next.length !== previous.length;
    const merged = next.map((item, index) => {
      const previousItem = previous[index];
      if (previousItem !== undefined && equals(previousItem, item)) {
        return previousItem;
      }
      changed = true;
      return item;
    });
    if (!changed) return previous;
    previous = merged;
    return merged;
  });
}

export function primaryNavTabEquals(a: MobileNavBarPrimaryTab, b: MobileNavBarPrimaryTab): boolean {
  return (
    a.id === b.id &&
    a.label === b.label &&
    a.route === b.route &&
    a.settingsRoute === b.settingsRoute &&
    a.tooltip === b.tooltip &&
    a.enabled === b.enabled &&
    a.live === b.live &&
    a.icon === b.icon &&
    a.alwaysShow === b.alwaysShow &&
    a.badge === b.badge
  );
}

export function utilityNavTabEquals(a: MobileNavBarUtilityTab, b: MobileNavBarUtilityTab): boolean {
  return (
    a.id === b.id &&
    a.label === b.label &&
    a.route === b.route &&
    a.tooltip === b.tooltip &&
    a.badge === b.badge &&
    a.count === b.count &&
    a.countLabel === b.countLabel &&
    a.icon === b.icon &&
    (a.breakdown === b.breakdown ||
      (a.breakdown !== undefined &&
        b.breakdown !== undefined &&
        a.breakdown.warning === b.breakdown.warning &&
        a.breakdown.critical === b.breakdown.critical))
  );
}

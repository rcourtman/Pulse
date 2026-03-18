import { createSignal, type Accessor } from 'solid-js';
import { isKioskMode, subscribeToKioskMode } from '@/utils/url';

// Module-level shared signal — all consumers share the same reactive state.
// Subscription is eager (runs at import time) so no updates are missed.
const [kioskMode, setKioskMode] = createSignal(isKioskMode());
subscribeToKioskMode(setKioskMode);

/**
 * Re-sync the shared signal with the current kiosk state.
 * Called after initKioskMode() processes URL params, which may change
 * the stored value without notifying listeners.
 */
export function syncKioskMode(): void {
  setKioskMode(isKioskMode());
}

/**
 * Shared reactive accessor for kiosk mode.
 * All components that call this hook read the same signal,
 * eliminating per-component subscription boilerplate.
 */
export function useKioskMode(): Accessor<boolean> {
  return kioskMode;
}

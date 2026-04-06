import { createSignal } from 'solid-js';
import { logger } from '@/utils/logger';

const [demoModeEnabled, setDemoModeEnabled] = createSignal(false);
const [demoModeResolved, setDemoModeResolved] = createSignal(false);

let pendingDemoModeCheck: Promise<boolean> | null = null;

function applyDemoModeHeaderValue(value: string | null): boolean {
  const enabled = value === 'true';
  setDemoModeEnabled(enabled);
  setDemoModeResolved(true);
  return enabled;
}

export function syncDemoModeFromResponse(response: Response): boolean {
  return applyDemoModeHeaderValue(response.headers.get('X-Demo-Mode'));
}

export async function ensureDemoModeResolved(force = false): Promise<boolean> {
  if (demoModeResolved() && !force) {
    return demoModeEnabled();
  }
  if (pendingDemoModeCheck && !force) {
    return pendingDemoModeCheck;
  }

  pendingDemoModeCheck = fetch('/api/health', {
    method: 'GET',
    cache: 'no-store',
    credentials: 'include',
    headers: {
      Accept: 'application/json',
      'X-Requested-With': 'XMLHttpRequest',
    },
  })
    .then((response) => syncDemoModeFromResponse(response))
    .catch((error) => {
      logger.debug('[demoModeStore] Failed to resolve demo mode from /api/health', error);
      if (!demoModeResolved()) {
        setDemoModeResolved(true);
      }
      return demoModeEnabled();
    })
    .finally(() => {
      pendingDemoModeCheck = null;
    });

  return pendingDemoModeCheck;
}

export { demoModeEnabled, demoModeResolved };

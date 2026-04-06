import { createSignal } from 'solid-js';
import type { SecurityStatus, SecurityStatusSessionCapabilities } from '@/types/config';

const DEFAULT_SESSION_CAPABILITIES: SecurityStatusSessionCapabilities = {
  demoMode: false,
};

const [sessionCapabilities, setSessionCapabilities] = createSignal<SecurityStatusSessionCapabilities>(
  { ...DEFAULT_SESSION_CAPABILITIES },
);
const [sessionCapabilitiesResolved, setSessionCapabilitiesResolved] = createSignal(false);

function normalizeSessionCapabilities(
  capabilities?: Partial<SecurityStatusSessionCapabilities> | null,
): SecurityStatusSessionCapabilities {
  return {
    ...DEFAULT_SESSION_CAPABILITIES,
    demoMode: capabilities?.demoMode === true,
  };
}

export function syncSessionCapabilities(
  status?: Pick<SecurityStatus, 'sessionCapabilities'> | null,
): SecurityStatusSessionCapabilities {
  const next = normalizeSessionCapabilities(status?.sessionCapabilities);
  setSessionCapabilities(next);
  setSessionCapabilitiesResolved(true);
  return next;
}

export { sessionCapabilities, sessionCapabilitiesResolved };

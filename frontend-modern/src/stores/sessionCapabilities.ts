import { createSignal } from 'solid-js';
import { MONITORING_WRITE_SCOPE } from '@/constants/apiScopes';
import type { SecurityStatus, SecurityStatusSessionCapabilities } from '@/types/config';

const DEFAULT_SESSION_CAPABILITIES: SecurityStatusSessionCapabilities = {
  demoMode: false,
};

const [sessionCapabilities, setSessionCapabilities] =
  createSignal<SecurityStatusSessionCapabilities>({ ...DEFAULT_SESSION_CAPABILITIES });
const [sessionCapabilitiesResolved, setSessionCapabilitiesResolved] = createSignal(false);
// Present only for API-token sessions; browser sessions carry no token scopes.
const [sessionTokenScopes, setSessionTokenScopes] = createSignal<readonly string[] | null>(null);

function normalizeSessionCapabilities(
  capabilities?: Partial<SecurityStatusSessionCapabilities> | null,
): SecurityStatusSessionCapabilities {
  return {
    ...DEFAULT_SESSION_CAPABILITIES,
    demoMode: capabilities?.demoMode === true,
  };
}

export function syncSessionCapabilities(
  status?: Pick<SecurityStatus, 'sessionCapabilities' | 'tokenScopes'> | null,
): SecurityStatusSessionCapabilities {
  const next = normalizeSessionCapabilities(status?.sessionCapabilities);
  setSessionCapabilities(next);
  const scopes = status?.tokenScopes?.filter((scope) => scope.trim());
  setSessionTokenScopes(scopes && scopes.length > 0 ? scopes : null);
  setSessionCapabilitiesResolved(true);
  return next;
}

/**
 * Mirrors the backend metadata write gate (`EnsureScope(monitoring:write)`):
 * browser sessions always pass, while API-token sessions such as kiosk links
 * need `monitoring:write` or the wildcard scope.
 */
export function sessionCanWriteMonitoringMetadata(): boolean {
  const scopes = sessionTokenScopes();
  if (!scopes) return true;
  return scopes.includes('*') || scopes.includes(MONITORING_WRITE_SCOPE);
}

export { sessionCapabilities, sessionCapabilitiesResolved };

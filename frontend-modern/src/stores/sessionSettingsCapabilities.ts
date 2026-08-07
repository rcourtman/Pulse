import { createSignal } from 'solid-js';
import type { SecurityStatus, SecurityStatusSettingsCapabilities } from '@/types/config';

const [sessionSettingsCapabilities, setSessionSettingsCapabilities] =
  createSignal<SecurityStatusSettingsCapabilities | null>(null);
const [sessionSettingsCapabilitiesResolved, setSessionSettingsCapabilitiesResolved] =
  createSignal(false);

/**
 * Publishes the session's Settings capabilities from `/api/security/status` so
 * surfaces outside Settings can tell whether a Settings destination is actually
 * reachable. Settings itself resolves the same payload through
 * `useSettingsAccess`, but that fetch only runs once Settings mounts, which is
 * no help to a page deciding whether to link there in the first place.
 */
export function syncSessionSettingsCapabilities(
  status?: Pick<SecurityStatus, 'settingsCapabilities'> | null,
): SecurityStatusSettingsCapabilities | null {
  const next = status?.settingsCapabilities ?? null;
  setSessionSettingsCapabilities(next);
  setSessionSettingsCapabilitiesResolved(true);
  return next;
}

/**
 * Mirrors the Settings nav gate for Infrastructure (`requiredCapability:
 * 'infrastructureRead'` in settingsNavCatalog), so a surface that links to
 * Settings → Infrastructure can never offer a destination the nav itself hides.
 * Reusing the destination's own capability is what keeps the two from drifting.
 *
 * Unresolved sessions keep the link, matching how `settingsNavVisibility`
 * treats an unresolved capability set: the bootstrap fetch is the first request
 * the app makes, and an admin must not flicker through the restricted copy if
 * it is briefly in flight or has failed.
 */
export function sessionCanReadInfrastructureSettings(): boolean {
  if (!sessionSettingsCapabilitiesResolved()) {
    return true;
  }
  return sessionSettingsCapabilities()?.infrastructureRead === true;
}

export { sessionSettingsCapabilities, sessionSettingsCapabilitiesResolved };

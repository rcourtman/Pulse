import { createMemo, onMount } from 'solid-js';

import {
  aiRuntimeSettings,
  aiRuntimeSettingsLoaded,
  loadAIRuntimeSettings,
} from '@/stores/aiRuntimeState';

/**
 * Canonical client-side availability boundary for AI Discovery surfaces.
 *
 * Discovery is opt-in. Consumers stay hidden while settings are unresolved,
 * after a settings read failure, and whenever the runtime does not explicitly
 * report discovery_enabled=true. The shared runtime store deduplicates loads
 * across concurrently opened drawer surfaces and updates reactively after a
 * settings save.
 */
export function useDiscoveryFeatureAvailability() {
  onMount(() => {
    void loadAIRuntimeSettings().catch(() => undefined);
  });

  const discoveryFeatureResolved = createMemo(() => aiRuntimeSettingsLoaded());
  const discoveryFeatureEnabled = createMemo(
    () => discoveryFeatureResolved() && aiRuntimeSettings()?.discovery_enabled === true,
  );
  const discoveryFeatureKnownDisabled = createMemo(
    () => discoveryFeatureResolved() && !discoveryFeatureEnabled(),
  );

  return {
    discoveryFeatureEnabled,
    discoveryFeatureKnownDisabled,
    discoveryFeatureResolved,
  };
}

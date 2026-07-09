// Single source of truth for "can Discovery actually work here?"
//
// Discovery has real prerequisites — the feature toggle, a configured AI
// provider, and an agent with command execution ("Pulse Commands") enabled and
// connected. These were previously re-checked ad hoc in ~5 places (the settings
// section, the per-resource tab, the run gate), which is why the feature could
// be silently on-but-useless and the UI was inconsistent. Compute the verdict
// once here and let every surface render from it.

export type DiscoveryReadinessStatus =
  | 'disabled' // user turned the feature off
  | 'needs_ai_provider' // no AI provider configured to analyze evidence
  | 'needs_commands' // AI ready, but the host agent has Pulse Commands disabled
  | 'needs_connected_agent' // commands enabled, but no agent connected to run them
  | 'ready';

export interface DiscoveryReadinessInputs {
  /** The service context toggle (Settings -> Pulse Intelligence -> Assistant). */
  discoveryEnabled: boolean;
  /** Whether at least one AI provider has credentials configured. */
  aiProviderConfigured: boolean;
  /**
   * Whether the relevant host agent has command execution enabled. `undefined`
   * means "not known for this context" (don't block on it).
   */
  commandsEnabled: boolean | undefined;
  /** Whether an agent is connected and able to run commands. */
  hasConnectedAgent: boolean;
}

export interface DiscoveryReadiness {
  status: DiscoveryReadinessStatus;
  ready: boolean;
}

// Ordered most-fundamental-first: a missing AI provider matters before a
// command-disabled agent, which matters before connectivity. The first unmet
// prerequisite is the one to surface, so the user fixes them in a sensible order.
export function computeDiscoveryReadiness(input: DiscoveryReadinessInputs): DiscoveryReadiness {
  if (!input.discoveryEnabled) return { status: 'disabled', ready: false };
  if (!input.aiProviderConfigured) return { status: 'needs_ai_provider', ready: false };
  if (input.commandsEnabled === false) return { status: 'needs_commands', ready: false };
  if (input.commandsEnabled === true && !input.hasConnectedAgent) {
    return { status: 'needs_connected_agent', ready: false };
  }
  return { status: 'ready', ready: true };
}

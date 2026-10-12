import { settingsTabPath } from '@/components/Settings/settingsNavigationModel';

export interface PatrolRuntimeActionPresentation {
  label: string;
  href: string;
}

export const PATROL_PROVIDER_SETTINGS_ACTION: PatrolRuntimeActionPresentation = {
  label: 'Check Patrol model',
  href: settingsTabPath('system-ai-patrol'),
};

// Config-level causes mean the install has no working provider at all, so
// sending the user to the Patrol model check is a dead end. Route them to
// Provider & Models, where the enable toggle and API key / Ollama fields live.
const PATROL_CONFIG_LEVEL_CAUSES = new Set(['assistant_disabled', 'provider_not_configured']);

/** True when Patrol is blocked because no provider is connected or AI is off. */
export const isPatrolConfigLevelCause = (cause?: string): boolean =>
  !!cause && PATROL_CONFIG_LEVEL_CAUSES.has(cause);

// The provider refused the saved key. Reconnecting fixes it, so the Patrol
// page offers its setup panel rather than the model check.
export const PATROL_PROVIDER_AUTH_CAUSE = 'provider_auth';

/** True when the Patrol page's own setup panel is the fix for the block. */
export const isPatrolConnectCause = (cause?: string): boolean =>
  isPatrolConfigLevelCause(cause) || cause === PATROL_PROVIDER_AUTH_CAUSE;

// Pulse could not reach the saved provider (wrong URL, server down, network).
// The model check cannot fix that; the provider's URL and its test live on
// Provider & Models. It is not a setup-panel cause, because a passing network
// blip would ask a cloud user for a key that is fine.
export const PATROL_PROVIDER_CONNECTION_CAUSE = 'provider_connection';

const isPatrolProviderSettingsCause = (cause?: string): boolean =>
  isPatrolConnectCause(cause) || cause === PATROL_PROVIDER_CONNECTION_CAUSE;

export const getPatrolProviderSettingsAction = (): PatrolRuntimeActionPresentation => ({
  ...PATROL_PROVIDER_SETTINGS_ACTION,
});

// A used-up 30-day cost budget is a spending decision, not a model fault:
// the fix is the budget field on Provider & Models (or a cheaper model), so
// "Check Patrol model" would send the operator to a dead end (#1789).
export const PATROL_BUDGET_EXHAUSTED_CAUSE = 'budget_exhausted';

export const getPatrolSetupAction = (cause?: string): PatrolRuntimeActionPresentation => {
  if (cause === PATROL_BUDGET_EXHAUSTED_CAUSE) {
    return { label: 'Raise the cost budget', href: settingsTabPath('system-ai') };
  }
  if (isPatrolProviderSettingsCause(cause)) {
    return { label: 'Open Provider & Models', href: settingsTabPath('system-ai') };
  }
  return { ...PATROL_PROVIDER_SETTINGS_ACTION };
};

// The tool-check explainer only makes sense once a provider exists; for
// config-level causes the readiness summary already says what to do.
export const getPatrolSetupHint = (cause?: string): string => {
  if (cause === PATROL_BUDGET_EXHAUSTED_CAUSE) {
    return '';
  }
  if (isPatrolProviderSettingsCause(cause)) {
    return '';
  }
  return 'Open Patrol settings and run the model check. Provider connectivity can be healthy even when the selected model cannot use Patrol tools.';
};

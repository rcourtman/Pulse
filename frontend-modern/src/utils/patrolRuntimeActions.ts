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

export const getPatrolProviderSettingsAction = (): PatrolRuntimeActionPresentation => ({
  ...PATROL_PROVIDER_SETTINGS_ACTION,
});

export const getPatrolSetupAction = (cause?: string): PatrolRuntimeActionPresentation => {
  if (cause && PATROL_CONFIG_LEVEL_CAUSES.has(cause)) {
    return { label: 'Open Provider & Models', href: settingsTabPath('system-ai') };
  }
  return { ...PATROL_PROVIDER_SETTINGS_ACTION };
};

// The tool-check explainer only makes sense once a provider exists; for
// config-level causes the readiness summary already says what to do.
export const getPatrolSetupHint = (cause?: string): string => {
  if (cause && PATROL_CONFIG_LEVEL_CAUSES.has(cause)) {
    return '';
  }
  return 'Open Patrol settings and run the model check. Provider connectivity can be healthy even when the selected model cannot use Patrol tools.';
};

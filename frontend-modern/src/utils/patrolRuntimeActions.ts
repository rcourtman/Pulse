import { settingsTabPath } from '@/components/Settings/settingsNavigationModel';

export interface PatrolRuntimeActionPresentation {
  label: string;
  href: string;
}

export const PATROL_PROVIDER_SETTINGS_ACTION: PatrolRuntimeActionPresentation = {
  label: 'Check Patrol model',
  href: settingsTabPath('system-ai-patrol'),
};

export const getPatrolProviderSettingsAction = (): PatrolRuntimeActionPresentation => ({
  ...PATROL_PROVIDER_SETTINGS_ACTION,
});

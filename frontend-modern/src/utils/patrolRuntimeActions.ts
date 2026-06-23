import { SETTINGS_PROVIDER_MODELS_PATH } from '@/components/Settings/settingsNavigationModel';

export interface PatrolRuntimeActionPresentation {
  label: string;
  href: string;
}

export const PATROL_PROVIDER_SETTINGS_ACTION: PatrolRuntimeActionPresentation = {
  label: 'Open Provider & Models',
  href: SETTINGS_PROVIDER_MODELS_PATH,
};

export const getPatrolProviderSettingsAction = (): PatrolRuntimeActionPresentation => ({
  ...PATROL_PROVIDER_SETTINGS_ACTION,
});

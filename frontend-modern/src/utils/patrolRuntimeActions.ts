export interface PatrolRuntimeActionPresentation {
  label: string;
  href: string;
}

export const PATROL_PROVIDER_SETTINGS_ACTION: PatrolRuntimeActionPresentation = {
  label: 'Open Patrol provider settings',
  href: '/settings/system-ai',
};

export const getPatrolProviderSettingsAction = (): PatrolRuntimeActionPresentation => ({
  ...PATROL_PROVIDER_SETTINGS_ACTION,
});

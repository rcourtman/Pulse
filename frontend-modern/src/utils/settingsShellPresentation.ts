export function getSettingsSearchEmptyState(searchQuery: string) {
  return {
    text: `No settings found for "${searchQuery}"`,
  } as const;
}

export function getSettingsLoadingState() {
  return {
    text: 'Loading settings...',
  } as const;
}

export function getSettingsConfigurationLoadingState() {
  return {
    text: 'Loading configuration...',
  } as const;
}

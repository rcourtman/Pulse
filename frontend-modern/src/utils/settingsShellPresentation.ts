export const SETTINGS_SHELL_COPY = {
  navigationAriaLabel: 'Settings navigation',
  navigationTitle: 'Settings',
  searchPlaceholder: 'Search settings...',
  searchShortcutHint: 'Any key',
  mobileBackLabel: 'Settings',
  collapseSidebarLabel: 'Collapse settings navigation',
  expandSidebarLabel: 'Expand settings navigation',
} as const;

export function getSettingsUnsavedChangesBanner() {
  return {
    title: 'Unsaved changes',
    description: 'Your changes will be lost if you navigate away.',
    saveLabel: 'Save Changes',
    discardLabel: 'Discard',
  } as const;
}

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

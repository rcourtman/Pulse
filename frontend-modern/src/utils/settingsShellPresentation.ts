import { DEFAULT_LOCALE, t, type SupportedLocale } from '@/i18n';

export function getSettingsShellCopy(locale?: SupportedLocale) {
  return {
    navigationAriaLabel: t('settings.shell.navigationAriaLabel', {}, locale),
    navigationTitle: t('settings.shell.navigationTitle', {}, locale),
    searchPlaceholder: t('settings.shell.searchPlaceholder', {}, locale),
    searchShortcutHint: undefined,
    mobileBackLabel: t('settings.shell.mobileBackLabel', {}, locale),
    collapseSidebarLabel: t('settings.shell.collapseSidebarLabel', {}, locale),
    expandSidebarLabel: t('settings.shell.expandSidebarLabel', {}, locale),
  } as const;
}

export const SETTINGS_SHELL_COPY = getSettingsShellCopy(DEFAULT_LOCALE);

export function getSettingsUnsavedChangesBanner(locale?: SupportedLocale) {
  return {
    title: t('settings.shell.unsavedTitle', {}, locale),
    description: t('settings.shell.unsavedDescription', {}, locale),
    saveLabel: t('settings.shell.saveChangesLabel', {}, locale),
    discardLabel: t('settings.shell.discardLabel', {}, locale),
  } as const;
}

export function getSettingsSearchEmptyState(searchQuery: string, locale?: SupportedLocale) {
  return {
    text: t('settings.shell.searchEmpty', { query: searchQuery }, locale),
  } as const;
}

export function getSettingsLoadingState(locale?: SupportedLocale) {
  return {
    text: t('settings.shell.loading', {}, locale),
  } as const;
}

export function getSettingsConfigurationLoadingState(locale?: SupportedLocale) {
  return {
    text: t('settings.shell.configurationLoading', {}, locale),
  } as const;
}

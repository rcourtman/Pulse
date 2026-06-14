import {
  DEFAULT_LOCALE,
  FIRST_LOCALIZATION_LOCALES,
  SUPPORTED_LOCALES,
  type SupportedLocale,
} from './locales';
import {
  FIRST_SESSION_MONITORING_MIGRATED_MESSAGE_KEYS,
  SETTINGS_GENERAL_MIGRATED_MESSAGE_KEYS,
  type I18nMessageKey,
} from './messages';

export const LOCALIZATION_FOUNDATION = {
  ownerLayer: 'frontend-modern/src/i18n',
  defaultLocale: DEFAULT_LOCALE,
  supportedLocales: SUPPORTED_LOCALES,
  firstWaveLocales: FIRST_LOCALIZATION_LOCALES,
  fallbackBehavior:
    'Normalize locale input to a supported locale, then fall back to English for unsupported locales and missing catalog entries.',
  catalogShape:
    'English is the source catalog. Every supported locale catalog must expose the same message keys. Migrated first-wave surfaces must provide explicit German and Spanish strings instead of inheriting English copy.',
} as const;

export const NEVER_TRANSLATE_COPY_RULES = [
  'API response fields, request fields, generated type names, and persistence keys stay machine-stable.',
  'Configuration keys, environment variable names, CLI commands, shell snippets, and code samples stay byte-stable.',
  'Resource names, hostnames, user-entered labels, node names, IDs, and log/event payloads stay as reported by the source system.',
  'Product and integration identifiers stay unchanged unless product governance explicitly approves a localized brand form.',
] as const;

export const SETTINGS_GENERAL_NON_TRANSLATABLE_TOKENS = [
  'Pulse',
  'Proxmox VE',
  'Docker / Podman',
  'API',
  'CPU',
  'IP',
  'JSON',
  'PULSE_TELEMETRY',
  'PVE_POLLING_INTERVAL',
  'PULSE_DISABLE_DOCKER_UPDATE_ACTIONS',
  '"Update"',
] as const;

export const FIRST_SESSION_MONITORING_NON_TRANSLATABLE_TOKENS = [
  'Pulse',
  'Pulse Agent',
  'API',
  'URL',
  'Docker',
  'Kubernetes',
  'Proxmox',
  'TrueNAS',
  'VMware',
  'PBS',
  'PMG',
  'LXC',
  '.bootstrap_token',
  '<pulse-container>',
  '<ctid>',
  'sudo pulse bootstrap-token',
  'docker exec',
  'pct exec',
] as const;

export const LOCALIZED_SETTINGS_GENERAL_JOURNEY_KEYS =
  SETTINGS_GENERAL_MIGRATED_MESSAGE_KEYS satisfies readonly I18nMessageKey[];

export const LOCALIZED_FIRST_SESSION_MONITORING_JOURNEY_KEYS =
  FIRST_SESSION_MONITORING_MIGRATED_MESSAGE_KEYS satisfies readonly I18nMessageKey[];

export const SETTINGS_GENERAL_ALLOWED_IDENTICAL_TRANSLATIONS = {
  de: ['settings.nav.group.system', 'settings.general.theme.option.system'],
  es: ['settings.header.systemGeneral.title', 'settings.nav.item.general'],
} as const satisfies Partial<Record<SupportedLocale, readonly I18nMessageKey[]>>;

export const FIRST_SESSION_MONITORING_ALLOWED_IDENTICAL_TRANSLATIONS = {
  de: ['setup.completion.sourceOptions.agent.title', 'setup.security.placeholder.username'],
  es: ['setup.completion.sourceOptions.agent.title', 'setup.security.placeholder.username'],
} as const satisfies Partial<Record<SupportedLocale, readonly I18nMessageKey[]>>;

import {
  ADMITTED_PLATFORM_IDS,
  DEFAULT_INFRASTRUCTURE_SOURCE_ORDER,
  KNOWN_SOURCE_PLATFORM_KEYS,
  PLATFORM_TYPE_KEYS,
  PLATFORM_SUPPORT_MANIFEST_SOURCE,
  PRESENTATION_ONLY_PLATFORM_IDS,
  PLATFORM_SUPPORT_MANIFEST,
  SOURCE_PLATFORM_PRESENTATION,
  SOURCE_PLATFORM_ALIAS_MAP,
  SOURCE_PLATFORM_AUDIT_TOKENS,
  SOURCE_PLATFORM_DISPLAY_TOKENS,
  SOURCE_PLATFORM_MANIFEST_ENTRIES,
  SOURCE_PLATFORM_STORAGE_FAMILY,
  SUPPORTED_PLATFORM_IDS,
  type GeneratedKnownSourcePlatform,
  type GeneratedSourcePlatformManifestEntry,
  type PlatformGovernanceState,
  type SourcePlatformStorageFamily,
} from '@/utils/platformSupportManifest.generated';

export type SourcePlatformManifestEntry = GeneratedSourcePlatformManifestEntry;
export type { GeneratedKnownSourcePlatform, PlatformGovernanceState, SourcePlatformStorageFamily };

const entriesById = new Map<string, SourcePlatformManifestEntry>(
  SOURCE_PLATFORM_MANIFEST_ENTRIES.map((platform) => [platform.id, platform] as const),
);

export {
  ADMITTED_PLATFORM_IDS,
  DEFAULT_INFRASTRUCTURE_SOURCE_ORDER,
  KNOWN_SOURCE_PLATFORM_KEYS,
  PLATFORM_SUPPORT_MANIFEST_SOURCE,
  PLATFORM_TYPE_KEYS,
  PRESENTATION_ONLY_PLATFORM_IDS,
  PLATFORM_SUPPORT_MANIFEST,
  SOURCE_PLATFORM_PRESENTATION,
  SOURCE_PLATFORM_ALIAS_MAP,
  SOURCE_PLATFORM_AUDIT_TOKENS,
  SOURCE_PLATFORM_DISPLAY_TOKENS,
  SOURCE_PLATFORM_MANIFEST_ENTRIES,
  SUPPORTED_PLATFORM_IDS,
};

export const getSourcePlatformManifestEntry = (
  value: string | null | undefined,
): SourcePlatformManifestEntry | null => {
  const normalized = (value || '').trim().toLowerCase();
  if (!normalized) return null;

  const aliasMap = SOURCE_PLATFORM_ALIAS_MAP as Record<string, string>;
  const platformId = aliasMap[normalized] || normalized;
  return entriesById.get(platformId) || null;
};

export const getSourcePlatformStorageFamily = (
  value: string | null | undefined,
): SourcePlatformStorageFamily | null => {
  const manifestPlatform = getSourcePlatformManifestEntry(value);
  if (!manifestPlatform) return null;
  return SOURCE_PLATFORM_STORAGE_FAMILY[manifestPlatform.id];
};

import manifestJson from '../../../docs/release-control/v6/internal/PLATFORM_SUPPORT_MANIFEST.json';

export type PlatformGovernanceState = 'supported' | 'admitted' | 'presentation-only';
export type SourcePlatformStorageFamily = 'onprem' | 'container' | 'virtualization' | 'cloud';

export interface SourcePlatformManifestEntry {
  id: string;
  governanceState: PlatformGovernanceState;
  uiLabel: string;
  uiTone: string;
  aliases: string[];
  displayTokens: string[];
  storageFamily: SourcePlatformStorageFamily;
}

export interface PlatformSupportManifest {
  schemaVersion: number;
  defaultInfrastructureSourceOrder: string[];
  platforms: SourcePlatformManifestEntry[];
}

const VALID_GOVERNANCE_STATES = new Set<PlatformGovernanceState>([
  'supported',
  'admitted',
  'presentation-only',
]);
const VALID_STORAGE_FAMILIES = new Set<SourcePlatformStorageFamily>([
  'onprem',
  'container',
  'virtualization',
  'cloud',
]);

const requireRecord = (value: unknown, label: string): Record<string, unknown> => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error(`platform support manifest: expected ${label} to be an object`);
  }
  return value as Record<string, unknown>;
};

const requireString = (value: unknown, label: string): string => {
  if (typeof value !== 'string' || value.trim().length === 0) {
    throw new Error(`platform support manifest: expected ${label} to be a non-empty string`);
  }
  return value.trim();
};

const requireLowercaseIdentifier = (value: unknown, label: string): string => {
  const normalized = requireString(value, label);
  if (normalized !== normalized.toLowerCase()) {
    throw new Error(`platform support manifest: expected ${label} to be lowercase`);
  }
  return normalized;
};

const requireStringArray = (value: unknown, label: string): string[] => {
  if (!Array.isArray(value)) {
    throw new Error(`platform support manifest: expected ${label} to be an array`);
  }

  return value.map((item, index) => requireString(item, `${label}[${index}]`));
};

const uniqueStrings = (values: Iterable<string>): string[] => Array.from(new Set(values));

const parsePlatformEntry = (value: unknown, index: number): SourcePlatformManifestEntry => {
  const record = requireRecord(value, `platforms[${index}]`);
  const governanceState = requireString(
    record.governance_state,
    `platforms[${index}].governance_state`,
  ) as PlatformGovernanceState;
  if (!VALID_GOVERNANCE_STATES.has(governanceState)) {
    throw new Error(
      `platform support manifest: invalid governance_state ${governanceState} at platforms[${index}]`,
    );
  }

  const storageFamily = requireString(
    record.storage_family,
    `platforms[${index}].storage_family`,
  ) as SourcePlatformStorageFamily;
  if (!VALID_STORAGE_FAMILIES.has(storageFamily)) {
    throw new Error(
      `platform support manifest: invalid storage_family ${storageFamily} at platforms[${index}]`,
    );
  }

  return {
    id: requireLowercaseIdentifier(record.id, `platforms[${index}].id`),
    governanceState,
    uiLabel: requireString(record.ui_label, `platforms[${index}].ui_label`),
    uiTone: requireString(record.ui_tone, `platforms[${index}].ui_tone`),
    aliases: uniqueStrings(
      requireStringArray(record.aliases, `platforms[${index}].aliases`).map((alias, aliasIndex) =>
        requireLowercaseIdentifier(alias, `platforms[${index}].aliases[${aliasIndex}]`),
      ),
    ),
    displayTokens: uniqueStrings(
      requireStringArray(record.display_tokens, `platforms[${index}].display_tokens`),
    ),
    storageFamily,
  };
};

const parsePlatformSupportManifest = (): PlatformSupportManifest => {
  const raw = requireRecord(manifestJson, 'root');
  const schemaVersion = Number(raw.schema_version);
  if (!Number.isInteger(schemaVersion) || schemaVersion < 1) {
    throw new Error('platform support manifest: expected schema_version to be a positive integer');
  }

  if (!Array.isArray(raw.platforms)) {
    throw new Error('platform support manifest: expected platforms to be an array');
  }

  const platforms = raw.platforms.map((platform, index) => parsePlatformEntry(platform, index));
  const knownIds = new Set(platforms.map((platform) => platform.id));
  const platformsById = new Map<string, SourcePlatformManifestEntry>();
  const aliases = new Map<string, string>();

  for (const platform of platforms) {
    if (platformsById.has(platform.id)) {
      throw new Error(`platform support manifest: duplicate platform id ${platform.id}`);
    }
    platformsById.set(platform.id, platform);

    for (const alias of platform.aliases) {
      if (alias === platform.id) {
        throw new Error(`platform support manifest: alias ${alias} duplicates its platform id`);
      }
      if (knownIds.has(alias) || aliases.has(alias)) {
        throw new Error(`platform support manifest: duplicate alias ${alias}`);
      }
      aliases.set(alias, platform.id);
    }
  }

  const defaultInfrastructureSourceOrder = uniqueStrings(
    requireStringArray(
      raw.default_infrastructure_source_order,
      'default_infrastructure_source_order',
    ).map((id, index) =>
      requireLowercaseIdentifier(id, `default_infrastructure_source_order[${index}]`),
    ),
  );
  const supportedIds = new Set(
    platforms
      .filter((platform) => platform.governanceState === 'supported')
      .map((platform) => platform.id),
  );

  for (const platformId of defaultInfrastructureSourceOrder) {
    if (!supportedIds.has(platformId)) {
      throw new Error(
        `platform support manifest: default infrastructure source order contains non-supported platform ${platformId}`,
      );
    }
  }

  return {
    schemaVersion,
    defaultInfrastructureSourceOrder,
    platforms,
  };
};

export const PLATFORM_SUPPORT_MANIFEST = parsePlatformSupportManifest();

const entriesById = new Map(
  PLATFORM_SUPPORT_MANIFEST.platforms.map((platform) => [platform.id, platform] as const),
);

export const SOURCE_PLATFORM_MANIFEST_ENTRIES = Object.freeze([
  ...PLATFORM_SUPPORT_MANIFEST.platforms,
]);

export const SOURCE_PLATFORM_ALIAS_MAP = Object.freeze(
  Object.fromEntries(
    SOURCE_PLATFORM_MANIFEST_ENTRIES.flatMap((platform) =>
      platform.aliases.map((alias) => [alias, platform.id]),
    ),
  ) as Record<string, string>,
);

export const SOURCE_PLATFORM_AUDIT_TOKENS = Object.freeze(
  uniqueStrings(
    SOURCE_PLATFORM_MANIFEST_ENTRIES.flatMap((platform) => [platform.id, ...platform.aliases]),
  ),
);

export const SOURCE_PLATFORM_DISPLAY_TOKENS = Object.freeze(
  uniqueStrings(
    SOURCE_PLATFORM_MANIFEST_ENTRIES.flatMap((platform) => [
      platform.uiLabel,
      ...platform.displayTokens,
    ]),
  ),
);

export const DEFAULT_INFRASTRUCTURE_SOURCE_ORDER = Object.freeze([
  ...PLATFORM_SUPPORT_MANIFEST.defaultInfrastructureSourceOrder,
]);

export const getSourcePlatformManifestEntry = (
  value: string | null | undefined,
): SourcePlatformManifestEntry | null => {
  const normalized = (value || '').trim().toLowerCase();
  if (!normalized) return null;

  const platformId = SOURCE_PLATFORM_ALIAS_MAP[normalized] || normalized;
  return entriesById.get(platformId) || null;
};

export const getSourcePlatformStorageFamily = (
  value: string | null | undefined,
): SourcePlatformStorageFamily | null =>
  getSourcePlatformManifestEntry(value)?.storageFamily || null;

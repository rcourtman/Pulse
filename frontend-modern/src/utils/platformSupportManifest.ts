import {
  ADMITTED_PLATFORM_IDS,
  AGENT_HOST_PROFILE_IDS,
  DEFAULT_INFRASTRUCTURE_SOURCE_ORDER,
  KNOWN_SOURCE_PLATFORM_KEYS,
  PLATFORM_TYPE_KEYS,
  PLATFORM_SUPPORT_MANIFEST_SOURCE,
  PRESENTATION_ONLY_PLATFORM_IDS,
  PLATFORM_SUPPORT_MANIFEST,
  SOURCE_AGENT_HOST_PROFILE_FAMILY,
  SOURCE_AGENT_HOST_PROFILE_GOVERNANCE_STATE,
  SOURCE_AGENT_HOST_PROFILE_HOST_IDENTITY_TOKENS,
  SOURCE_AGENT_HOST_PROFILE_MANIFEST_ENTRIES,
  SOURCE_AGENT_HOST_PROFILE_READINESS_STAGE,
  SOURCE_AGENT_HOST_PROFILE_STORAGE_FAMILY,
  SOURCE_AGENT_HOST_PROFILE_SUPPORT_FLOOR,
  SOURCE_PLATFORM_CANONICAL_PROJECTIONS,
  SOURCE_PLATFORM_ONBOARDING_PATH_KEYS,
  SOURCE_PLATFORM_ONBOARDING_PATHS,
  SOURCE_PLATFORM_PRESENTATION,
  SOURCE_PLATFORM_ALIAS_MAP,
  SOURCE_PLATFORM_AUDIT_TOKENS,
  SOURCE_PLATFORM_DISPLAY_TOKENS,
  SOURCE_PLATFORM_FAMILY,
  SOURCE_PLATFORM_MANIFEST_ENTRIES,
  SOURCE_PLATFORM_PRIMARY_MODE,
  SOURCE_PLATFORM_READINESS_STAGE,
  SOURCE_PLATFORM_STORAGE_FAMILY,
  SOURCE_PLATFORM_SUPPORT_FLOOR,
  SUPPORTED_PLATFORM_IDS,
  type AgentHostProfileGovernanceState,
  type AgentHostProfileReadinessStage,
  type AgentHostProfileSupportFloor,
  type AgentHostProfileSupportFloorValue,
  type GeneratedAgentHostProfileId,
  type GeneratedAgentHostProfileManifestEntry,
  type GeneratedKnownSourcePlatform,
  type GeneratedSourcePlatformOnboardingPath,
  type GeneratedSourcePlatformManifestEntry,
  type PlatformPrimaryMode,
  type PlatformReadinessStage,
  type PlatformSupportFloor,
  type PlatformSupportFloorValue,
  type PlatformGovernanceState,
  type SourcePlatformFamily,
  type SourcePlatformStorageFamily,
} from '@/utils/platformSupportManifest.generated';

export type SourcePlatformManifestEntry = GeneratedSourcePlatformManifestEntry;
export type SourceAgentHostProfileManifestEntry = GeneratedAgentHostProfileManifestEntry;
export type {
  AgentHostProfileGovernanceState,
  AgentHostProfileReadinessStage,
  AgentHostProfileSupportFloor,
  AgentHostProfileSupportFloorValue,
  GeneratedAgentHostProfileId as AgentHostProfileId,
  GeneratedKnownSourcePlatform,
  GeneratedSourcePlatformOnboardingPath as SourcePlatformOnboardingPath,
  PlatformPrimaryMode,
  PlatformReadinessStage,
  PlatformSupportFloor,
  PlatformSupportFloorValue,
  PlatformGovernanceState,
  SourcePlatformFamily,
  SourcePlatformStorageFamily,
};

const entriesById = new Map<string, SourcePlatformManifestEntry>(
  SOURCE_PLATFORM_MANIFEST_ENTRIES.map((platform) => [platform.id, platform] as const),
);
const agentHostProfileEntriesById = new Map<string, SourceAgentHostProfileManifestEntry>(
  SOURCE_AGENT_HOST_PROFILE_MANIFEST_ENTRIES.map((profile) => [profile.id, profile] as const),
);
const agentHostProfileEntriesByToken = new Map<string, SourceAgentHostProfileManifestEntry>(
  SOURCE_AGENT_HOST_PROFILE_MANIFEST_ENTRIES.flatMap((profile) =>
    profile.hostIdentityTokens.map((token) => [token, profile] as const),
  ),
);
const EMPTY_ONBOARDING_PATHS: readonly GeneratedSourcePlatformOnboardingPath[] = [];

export {
  ADMITTED_PLATFORM_IDS,
  AGENT_HOST_PROFILE_IDS,
  DEFAULT_INFRASTRUCTURE_SOURCE_ORDER,
  KNOWN_SOURCE_PLATFORM_KEYS,
  PLATFORM_SUPPORT_MANIFEST_SOURCE,
  PLATFORM_TYPE_KEYS,
  PRESENTATION_ONLY_PLATFORM_IDS,
  PLATFORM_SUPPORT_MANIFEST,
  SOURCE_AGENT_HOST_PROFILE_FAMILY,
  SOURCE_AGENT_HOST_PROFILE_GOVERNANCE_STATE,
  SOURCE_AGENT_HOST_PROFILE_HOST_IDENTITY_TOKENS,
  SOURCE_AGENT_HOST_PROFILE_MANIFEST_ENTRIES,
  SOURCE_AGENT_HOST_PROFILE_READINESS_STAGE,
  SOURCE_AGENT_HOST_PROFILE_STORAGE_FAMILY,
  SOURCE_AGENT_HOST_PROFILE_SUPPORT_FLOOR,
  SOURCE_PLATFORM_CANONICAL_PROJECTIONS,
  SOURCE_PLATFORM_ONBOARDING_PATH_KEYS,
  SOURCE_PLATFORM_ONBOARDING_PATHS,
  SOURCE_PLATFORM_PRESENTATION,
  SOURCE_PLATFORM_ALIAS_MAP,
  SOURCE_PLATFORM_AUDIT_TOKENS,
  SOURCE_PLATFORM_DISPLAY_TOKENS,
  SOURCE_PLATFORM_FAMILY,
  SOURCE_PLATFORM_MANIFEST_ENTRIES,
  SOURCE_PLATFORM_PRIMARY_MODE,
  SOURCE_PLATFORM_READINESS_STAGE,
  SOURCE_PLATFORM_SUPPORT_FLOOR,
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

export const getAgentHostProfileManifestEntry = (
  value: string | null | undefined,
): SourceAgentHostProfileManifestEntry | null => {
  const normalized = (value || '').trim().toLowerCase();
  if (!normalized) return null;

  return (
    agentHostProfileEntriesById.get(normalized) ||
    agentHostProfileEntriesByToken.get(normalized) ||
    null
  );
};

export const getAgentHostProfileFamily = (value: string | null | undefined): string | null => {
  const manifestProfile = getAgentHostProfileManifestEntry(value);
  return manifestProfile?.family ?? null;
};

export const getSourcePlatformStorageFamily = (
  value: string | null | undefined,
): SourcePlatformStorageFamily | null => {
  const manifestPlatform = getSourcePlatformManifestEntry(value);
  if (!manifestPlatform) return null;
  return SOURCE_PLATFORM_STORAGE_FAMILY[manifestPlatform.id];
};

export const getSourcePlatformReadinessStage = (
  value: string | null | undefined,
): PlatformReadinessStage | null => {
  const manifestPlatform = getSourcePlatformManifestEntry(value);
  if (!manifestPlatform) return null;
  return SOURCE_PLATFORM_READINESS_STAGE[manifestPlatform.id];
};

export const getSourcePlatformPrimaryMode = (
  value: string | null | undefined,
): PlatformPrimaryMode | null => {
  const manifestPlatform = getSourcePlatformManifestEntry(value);
  if (!manifestPlatform) return null;
  return SOURCE_PLATFORM_PRIMARY_MODE[manifestPlatform.id];
};

export const getSourcePlatformCanonicalProjections = (
  value: string | null | undefined,
): readonly string[] => {
  const manifestPlatform = getSourcePlatformManifestEntry(value);
  if (!manifestPlatform) return [];
  return SOURCE_PLATFORM_CANONICAL_PROJECTIONS[manifestPlatform.id];
};

export const getSourcePlatformSupportFloor = (
  value: string | null | undefined,
): PlatformSupportFloor | null => {
  const manifestPlatform = getSourcePlatformManifestEntry(value);
  if (!manifestPlatform) return null;
  return SOURCE_PLATFORM_SUPPORT_FLOOR[manifestPlatform.id];
};

export const getSourcePlatformOnboardingPaths = (
  value: string | null | undefined,
): readonly GeneratedSourcePlatformOnboardingPath[] => {
  const manifestPlatform = getSourcePlatformManifestEntry(value);
  if (!manifestPlatform) return EMPTY_ONBOARDING_PATHS;
  return SOURCE_PLATFORM_ONBOARDING_PATHS[manifestPlatform.id];
};

export const getSourcePlatformFamily = (
  value: string | null | undefined,
): SourcePlatformFamily | null => {
  const manifestPlatform = getSourcePlatformManifestEntry(value);
  if (!manifestPlatform) return null;
  return SOURCE_PLATFORM_FAMILY[manifestPlatform.id];
};

export const sourcePlatformSupportsOnboardingPath = (
  value: string | null | undefined,
  onboardingPath: GeneratedSourcePlatformOnboardingPath,
): boolean => getSourcePlatformOnboardingPaths(value).includes(onboardingPath);

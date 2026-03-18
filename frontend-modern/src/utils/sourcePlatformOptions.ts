import { getSourcePlatformLabel, normalizeSourcePlatformQueryValue } from '@/utils/sourcePlatforms';

export interface SourcePlatformOption {
  key: string;
  label: string;
}

const DEFAULT_SOURCE_PLATFORM_ORDER = [
  'proxmox-pve',
  'agent',
  'docker',
  'proxmox-pbs',
  'proxmox-pmg',
  'kubernetes',
  'truenas',
] as const;

export const orderSourcePlatformKeys = (
  keys: Iterable<string>,
  preferredOrder: readonly string[] = DEFAULT_SOURCE_PLATFORM_ORDER,
): string[] =>
  Array.from(new Set(Array.from(keys).map((key) => normalizeSourcePlatformQueryValue(key))))
    .filter(Boolean)
    .sort((a, b) => {
      const indexA = preferredOrder.indexOf(a);
      const indexB = preferredOrder.indexOf(b);
      if (indexA !== -1 || indexB !== -1) {
        if (indexA === -1) return 1;
        if (indexB === -1) return -1;
        return indexA - indexB;
      }
      return getSourcePlatformLabel(a).localeCompare(getSourcePlatformLabel(b));
    });

export const buildSourcePlatformOptions = (
  keys: Iterable<string>,
  preferredOrder?: readonly string[],
): SourcePlatformOption[] =>
  orderSourcePlatformKeys(keys, preferredOrder).map((key) => ({
    key,
    label: getSourcePlatformLabel(key),
  }));

export const DEFAULT_INFRASTRUCTURE_SOURCE_OPTIONS = buildSourcePlatformOptions(
  DEFAULT_SOURCE_PLATFORM_ORDER,
);

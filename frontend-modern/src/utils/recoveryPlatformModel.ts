import type { ProtectionRollup, RecoveryPoint } from '@/types/recovery';

const toTrimmedString = (value: unknown): string =>
  typeof value === 'string' ? value.trim() : '';

export const getRecoveryPointPlatform = (
  point: Pick<RecoveryPoint, 'platform' | 'provider'> | null | undefined,
): string => toTrimmedString(point?.platform) || toTrimmedString(point?.provider);

export const getRecoveryRollupPlatforms = (
  rollup: Pick<ProtectionRollup, 'platforms' | 'providers'> | null | undefined,
): string[] => {
  const values =
    Array.isArray(rollup?.platforms) && rollup.platforms.length > 0
      ? rollup.platforms
      : rollup?.providers || [];

  return values.map((value) => toTrimmedString(value)).filter(Boolean);
};

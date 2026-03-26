import type {
  ProtectionRollup,
  ProtectionRollupTransport,
  RecoveryPoint,
  RecoveryPointsResponse,
  RecoveryPointsTransportResponse,
  RecoveryPointTransport,
  RecoveryResponseMeta,
  RecoveryRollupsResponse,
  RecoveryRollupsTransportResponse,
} from '@/types/recovery';

const toTrimmedString = (value: unknown): string =>
  typeof value === 'string' ? value.trim() : '';

interface RecoveryPointPlatformLike {
  platform?: string | null;
  provider?: string | null;
}

interface RecoveryRollupPlatformsLike {
  platforms?: string[] | null;
  providers?: string[] | null;
}

export const getRecoveryPointPlatform = (
  point: RecoveryPointPlatformLike | null | undefined,
): string => toTrimmedString(point?.platform) || toTrimmedString(point?.provider);

export const getRecoveryRollupPlatforms = (
  rollup: RecoveryRollupPlatformsLike | null | undefined,
): string[] => {
  const values =
    Array.isArray(rollup?.platforms) && rollup.platforms.length > 0
      ? rollup.platforms
      : rollup?.providers || [];

  return values.map((value) => toTrimmedString(value)).filter(Boolean);
};

const normalizeRecoveryMeta = (meta: RecoveryResponseMeta | null | undefined): RecoveryResponseMeta => ({
  page: Number.isFinite(meta?.page) ? meta.page : 1,
  limit: Number.isFinite(meta?.limit) ? meta.limit : 0,
  total: Number.isFinite(meta?.total) ? meta.total : 0,
  totalPages: Number.isFinite(meta?.totalPages) ? meta.totalPages : 1,
});

export const normalizeRecoveryPoint = (
  point: RecoveryPointTransport | RecoveryPoint,
): RecoveryPoint => {
  const { provider: _provider, ...rest } = point as RecoveryPointTransport;
  const platform = getRecoveryPointPlatform(point);
  return platform ? { ...rest, platform } : (rest as RecoveryPoint);
};

export const normalizeRecoveryRollup = (
  rollup: ProtectionRollupTransport | ProtectionRollup,
): ProtectionRollup => {
  const { providers: _providers, ...rest } = rollup as ProtectionRollupTransport;
  const platforms = getRecoveryRollupPlatforms(rollup);
  return platforms.length > 0 ? { ...rest, platforms } : (rest as ProtectionRollup);
};

export const normalizeRecoveryPointsResponse = (
  response: RecoveryPointsTransportResponse,
): RecoveryPointsResponse => ({
  data: Array.isArray(response?.data) ? response.data.map(normalizeRecoveryPoint) : [],
  meta: normalizeRecoveryMeta(response?.meta),
});

export const normalizeRecoveryRollupsResponse = (
  response: RecoveryRollupsTransportResponse,
): RecoveryRollupsResponse => ({
  data: Array.isArray(response?.data) ? response.data.map(normalizeRecoveryRollup) : [],
  meta: normalizeRecoveryMeta(response?.meta),
});

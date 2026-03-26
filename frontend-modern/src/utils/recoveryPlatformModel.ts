import type {
  ProtectionRollup,
  ProtectionRollupTransport,
  RecoveryPointDisplay,
  RecoveryPointDisplayTransport,
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

interface RecoveryItemResourceLike {
  itemResourceId?: string | null;
  subjectResourceId?: string | null;
}

interface RecoveryItemRefLike {
  itemRef?: RecoveryPoint['itemRef'];
  subjectRef?: RecoveryPoint['subjectRef'];
}

const normalizeRecoveryDisplay = (
  display: RecoveryPointDisplay | RecoveryPointDisplayTransport | null | undefined,
): RecoveryPointDisplay | null | undefined => {
  if (display == null) return display;

  const {
    itemLabel,
    itemType,
    subjectLabel: _subjectLabel,
    subjectType: _subjectType,
    ...rest
  } = display as RecoveryPointDisplayTransport;

  const normalizedItemLabel = toTrimmedString(itemLabel) || toTrimmedString(_subjectLabel);
  const normalizedItemType = toTrimmedString(itemType) || toTrimmedString(_subjectType);

  return {
    ...rest,
    ...(normalizedItemLabel ? { itemLabel: normalizedItemLabel } : {}),
    ...(normalizedItemType ? { itemType: normalizedItemType } : {}),
  };
};

const getRecoveryItemResourceId = (
  value: RecoveryItemResourceLike | null | undefined,
): string => toTrimmedString(value?.itemResourceId) || toTrimmedString(value?.subjectResourceId);

const getRecoveryItemRef = (
  value: RecoveryItemRefLike | null | undefined,
): NonNullable<RecoveryPoint['itemRef']> | null => value?.itemRef || value?.subjectRef || null;

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
  const {
    provider: _provider,
    subjectResourceId: _subjectResourceId,
    subjectRef: _subjectRef,
    display,
    ...rest
  } = point as RecoveryPointTransport;
  const platform = getRecoveryPointPlatform(point);
  const itemResourceId = getRecoveryItemResourceId(point);
  const itemRef = getRecoveryItemRef(point);
  const normalizedDisplay = normalizeRecoveryDisplay(display);
  return {
    ...(rest as RecoveryPoint),
    ...(platform ? { platform } : {}),
    ...(itemResourceId ? { itemResourceId } : {}),
    ...(itemRef ? { itemRef } : {}),
    ...(display !== undefined ? { display: normalizedDisplay } : {}),
  };
};

export const normalizeRecoveryRollup = (
  rollup: ProtectionRollupTransport | ProtectionRollup,
): ProtectionRollup => {
  const {
    providers: _providers,
    subjectResourceId: _subjectResourceId,
    subjectRef: _subjectRef,
    display,
    ...rest
  } = rollup as ProtectionRollupTransport;
  const platforms = getRecoveryRollupPlatforms(rollup);
  const itemResourceId = getRecoveryItemResourceId(rollup);
  const itemRef = getRecoveryItemRef(rollup);
  const normalizedDisplay = normalizeRecoveryDisplay(display);
  return {
    ...(rest as ProtectionRollup),
    ...(platforms.length > 0 ? { platforms } : {}),
    ...(itemResourceId ? { itemResourceId } : {}),
    ...(itemRef ? { itemRef } : {}),
    ...(display !== undefined ? { display: normalizedDisplay } : {}),
  };
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

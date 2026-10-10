import type {
  RecoveryPointDisplay,
  RecoveryPointDisplayTransport,
  RecoveryPoint,
  RecoveryPointsResponse,
  RecoveryPointsTransportResponse,
  RecoveryPointTransport,
} from '@/types/recovery';

const toTrimmedString = (value: unknown): string => (typeof value === 'string' ? value.trim() : '');

interface RecoveryPointPlatformLike {
  platform?: string | null;
  provider?: string | null;
}

interface RecoveryItemResourceLike {
  itemResourceId?: string | null;
  subjectResourceId?: string | null;
}

interface RecoveryItemRefLike {
  itemRef?: RecoveryPoint['itemRef'];
  subjectRef?: RecoveryPointTransport['subjectRef'];
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

const getRecoveryItemResourceId = (value: RecoveryItemResourceLike | null | undefined): string =>
  toTrimmedString(value?.itemResourceId) || toTrimmedString(value?.subjectResourceId);

const getRecoveryItemRef = (
  value: RecoveryItemRefLike | null | undefined,
): NonNullable<RecoveryPoint['itemRef']> | null => value?.itemRef || value?.subjectRef || null;

export const getRecoveryPointPlatform = (
  point: RecoveryPointPlatformLike | null | undefined,
): string => toTrimmedString(point?.platform) || toTrimmedString(point?.provider);

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

export const normalizeRecoveryPointsResponse = (
  response: RecoveryPointsTransportResponse,
): RecoveryPointsResponse => ({
  data: Array.isArray(response?.data) ? response.data.map(normalizeRecoveryPoint) : [],
});

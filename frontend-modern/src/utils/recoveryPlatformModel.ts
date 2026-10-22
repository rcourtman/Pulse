import type {
  RecoveryPointDisplay,
  RecoveryPointDisplayTransport,
  RecoveryPoint,
  RecoveryPointsResponse,
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

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

// This is the HTTP boundary, not an empty-inventory fallback. The handler
// explicitly emits data: [] for an empty page. An unreadable envelope or row
// cannot erase earlier restore evidence or pass a partly usable inventory.
export const normalizeRecoveryPointsResponse = (response: unknown): RecoveryPointsResponse => {
  const unreadable = () => new Error('Recovery point response is unreadable.');
  if (!isRecord(response) || !Array.isArray(response.data)) throw unreadable();
  const identities = new Set<string>();
  const data = response.data.map((point: unknown) => {
    if (
      !isRecord(point) ||
      !['id', 'kind', 'mode', 'outcome'].every((key) => toTrimmedString(point[key])) ||
      !['display', 'details', 'itemRef', 'subjectRef', 'repositoryRef'].every(
        (key) => point[key] == null || isRecord(point[key]),
      )
    ) {
      throw unreadable();
    }
    const id = point.id as string;
    if (identities.has(id)) throw unreadable();
    identities.add(id);
    // Kind, mode and outcome are open strings in the transport contract.
    // Unrecognised outcomes remain Unknown in the consumer, not success.
    return normalizeRecoveryPoint(point as unknown as RecoveryPointTransport);
  });
  return { data };
};

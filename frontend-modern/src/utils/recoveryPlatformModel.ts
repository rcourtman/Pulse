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

export class RecoveryPointsFormatError extends Error {
  constructor() {
    // Provider/proxy contents may be private. Do not put them in the error.
    super('The recovery point response format is invalid.');
    this.name = 'RecoveryPointsFormatError';
  }
}

type FieldCheck = (value: unknown) => boolean;
const isRecord = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const isString: FieldCheck = (value) => typeof value === 'string';
const optional =
  (check: FieldCheck): FieldCheck =>
  (value) =>
    value == null || check(value);
const hasShape = (value: unknown, shape: Readonly<Record<string, FieldCheck>>): boolean =>
  isRecord(value) && Object.entries(shape).every(([field, check]) => check(value[field]));

const displayShape = Object.fromEntries(
  [
    'itemLabel',
    'itemType',
    'subjectLabel',
    'subjectType',
    'clusterLabel',
    'nodeHostLabel',
    'nodeAgentLabel',
    'namespaceLabel',
    'entityIdLabel',
    'repositoryLabel',
    'detailsSummary',
  ].map((field) => [field, optional(isString)]),
);
const refShape = Object.fromEntries(
  ['type', 'namespace', 'name', 'uid', 'id', 'class'].map((field) => [field, optional(isString)]),
);
const pointShape = {
  id: (value: unknown) => typeof value === 'string' && value.trim().length > 0,
  kind: isString,
  mode: isString,
  outcome: isString,
  ...Object.fromEntries(
    [
      'platform',
      'provider',
      'entityId',
      'cluster',
      'node',
      'namespace',
      'startedAt',
      'completedAt',
      'itemResourceId',
      'subjectResourceId',
      'repositoryResourceId',
      'providerScope',
    ].map((field) => [field, optional(isString)]),
  ),
  ...Object.fromEntries(
    ['verified', 'encrypted', 'immutable'].map((field) => [
      field,
      optional((value: unknown) => typeof value === 'boolean'),
    ]),
  ),
  sizeBytes: optional((value) => typeof value === 'number' && Number.isFinite(value) && value >= 0),
  display: optional((value) =>
    hasShape(value, {
      ...displayShape,
      isWorkload: optional((value) => typeof value === 'boolean'),
    }),
  ),
  itemRef: optional((value) => hasShape(value, refShape)),
  subjectRef: optional((value) => hasShape(value, refShape)),
  repositoryRef: optional((value) => hasShape(value, refShape)),
  details: optional(isRecord),
};

export const normalizeRecoveryPointsResponse = (response: unknown): RecoveryPointsResponse => {
  // The endpoint explicitly allocates an empty array. Missing/null/wrong-shaped
  // data is an unread inventory, not evidence that there are no restore points.
  // Admit the whole page before normalizing it; never silently drop a bad row.
  if (!isRecord(response) || !Array.isArray(response.data)) throw new RecoveryPointsFormatError();
  const ids = new Set<string>();
  for (const point of response.data) {
    if (!hasShape(point, pointShape) || ids.has(point.id)) throw new RecoveryPointsFormatError();
    ids.add(point.id);
  }
  // Dates remain strings so the consuming table can expose unknown instants;
  // new kind/mode/outcome strings keep their existing fallback presentation.
  return { data: response.data.map(normalizeRecoveryPoint) };
};

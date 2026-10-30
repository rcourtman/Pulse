import type { ReplicationJob } from '@/types/api';

export class ReplicationInventoryFormatError extends Error {
  constructor() {
    // Provider response bodies, identifiers and JSON parser text stay private.
    super(
      'The replication response format is invalid. Missing jobs do not mean none are configured.',
    );
    this.name = 'ReplicationInventoryFormatError';
  }
}

type FieldCheck = (value: unknown) => boolean;
const isRecord = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const isString: FieldCheck = (value) => typeof value === 'string';
const isID: FieldCheck = (value) => isString(value) && (value as string).trim().length > 0;
const isNumber: FieldCheck = (value) => typeof value === 'number' && Number.isFinite(value);
const isNonnegativeNumber: FieldCheck = (value) => isNumber(value) && (value as number) >= 0;
const isCount: FieldCheck = (value) => isNonnegativeNumber(value) && Number.isSafeInteger(value);
const isTime: FieldCheck = (value) => isString(value) || isNumber(value);
const optional =
  (check: FieldCheck): FieldCheck =>
  (value) =>
    value === undefined || check(value);

// Validate the fields the table consumes before it classifies, searches or
// links a job. Do not coerce truthy flags, drop malformed rows or guess identity.
// Unknown fields and status strings remain forward-compatible. Time strings
// keep the existing presentation rules; this is not a chronology assessment.
const jobShape: Readonly<Record<string, FieldCheck>> = {
  id: isID,
  instance: isID,
  jobId: isID,
  enabled: (value) => typeof value === 'boolean',
  guestId: optional(isCount),
  ...Object.fromEntries(
    [
      'guest',
      'guestName',
      'guestType',
      'guestNode',
      'sourceNode',
      'sourceStorage',
      'targetNode',
      'targetStorage',
      'schedule',
      'lastSyncStatus',
      'lastSyncDurationHuman',
      'error',
      'comment',
    ].map((field) => [field, optional(isString)]),
  ),
  lastSyncTime: optional(isTime),
  nextSyncTime: optional(isTime),
  lastSyncUnix: optional(isNumber),
  nextSyncUnix: optional(isNumber),
  lastSyncDurationSeconds: optional(isNonnegativeNumber),
  failCount: optional(isCount),
  rateLimitMbps: optional(isNonnegativeNumber),
};

export function parseReplicationInventory(payload: unknown): ReplicationJob[] {
  // This endpoint returns one complete, unpaginated snapshot. Only an explicit
  // empty array with a matching total establishes absence, never a missing list.
  if (
    !isRecord(payload) ||
    !Array.isArray(payload.data) ||
    !isRecord(payload.meta) ||
    !isCount(payload.meta.total) ||
    payload.meta.total !== payload.data.length
  ) {
    throw new ReplicationInventoryFormatError();
  }
  const fields = Object.entries(jobShape);
  const ids = new Set<string>();
  const subjects = new Set<string>();
  for (const row of payload.data) {
    if (!isRecord(row) || !fields.every(([field, check]) => check(row[field]))) {
      throw new ReplicationInventoryFormatError();
    }
    const id = row.id as string;
    const subject = JSON.stringify([row.instance, row.jobId]);
    // Disclosure and counts require one unambiguous row per native job. Native
    // job IDs can repeat across independent connections, not within one.
    if (ids.has(id) || subjects.has(subject)) throw new ReplicationInventoryFormatError();
    ids.add(id);
    subjects.add(subject);
  }
  return payload.data as ReplicationJob[];
}

export async function readReplicationInventoryJSON(response: Response): Promise<unknown> {
  try {
    return await response.json();
  } catch {
    throw new ReplicationInventoryFormatError();
  }
}

import type {
  BackupTask,
  GuestSnapshot,
  PBSBackup,
  PBSBackupsPayload,
  PVEBackupsPayload,
  StorageBackup,
} from '@/types/api';

export class BackupInventoryFormatError extends Error {
  constructor() {
    // Never include response bodies, provider identifiers or JSON parser text.
    super('The response format is invalid. Missing restore points do not mean no backups exist.');
    this.name = 'BackupInventoryFormatError';
  }
}

type FieldCheck = (value: unknown) => boolean;
type RowShape = Readonly<Record<string, FieldCheck>>;
const isRecord = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const isString: FieldCheck = (value) => typeof value === 'string';
const isID: FieldCheck = (value) => typeof value === 'string' && value.trim().length > 0;
const isBoolean: FieldCheck = (value) => typeof value === 'boolean';
const isNonnegativeNumber: FieldCheck = (value) =>
  typeof value === 'number' && Number.isFinite(value) && value >= 0;
const isGuestID: FieldCheck = (value) => isNonnegativeNumber(value) && Number.isSafeInteger(value);
const optional =
  (check: FieldCheck): FieldCheck =>
  (value) =>
    value === undefined || check(value);
const isFiles: FieldCheck = (value) => Array.isArray(value) && value.every(isString);

// Validate what the recovery model consumes, without guessing missing identity,
// coercing completion flags, silently dropping bad rows or rejecting extra fields.
// Optional display fields retain the model's existing unknown/fallback behaviour.
const pveGuestShape: RowShape = {
  id: isID,
  vmid: isGuestID,
  type: isString,
  node: optional(isString),
  instance: optional(isString),
};
const taskShape: RowShape = {
  ...pveGuestShape,
  status: isString,
  startTime: isString,
  endTime: optional(isString),
  error: optional(isString),
};
const archiveShape: RowShape = {
  ...pveGuestShape,
  time: isString,
  storage: optional(isString),
  format: optional(isString),
  volid: optional(isString),
  size: optional(isNonnegativeNumber),
  protected: optional(isBoolean),
  isPBS: optional(isBoolean),
  verified: optional(isBoolean),
  inProgress: optional(isBoolean),
};
const snapshotShape: RowShape = {
  ...pveGuestShape,
  time: isString,
  name: optional(isString),
  description: optional(isString),
  sizeBytes: optional(isNonnegativeNumber),
};
const pbsShape: RowShape = {
  id: isID,
  vmid: isString,
  backupType: isString,
  backupTime: isString,
  files: isFiles,
  instance: optional(isString),
  datastore: optional(isString),
  namespace: optional(isString),
  size: optional(isNonnegativeNumber),
  protected: optional(isBoolean),
  verified: optional(isBoolean),
  inProgress: optional(isBoolean),
  writeActivityObserved: optional(isBoolean),
  writeActive: optional(isBoolean),
};

function inventoryData(payload: unknown): Record<string, unknown> {
  if (!isRecord(payload) || !isRecord(payload.data)) throw new BackupInventoryFormatError();
  return payload.data;
}

function collection<T>(
  data: Record<string, unknown>,
  key: string,
  shape: RowShape,
  allowNil = false,
): T[] {
  const value = data[key];
  // The PVE handler can encode its empty Go slices as null. A missing field
  // is not that observation. PBS normalizes both its list and files to arrays.
  if (allowNil && value === null) return [];
  const fields = Object.entries(shape);
  if (
    !Array.isArray(value) ||
    !value.every((row) => isRecord(row) && fields.every(([field, check]) => check(row[field])))
  ) {
    throw new BackupInventoryFormatError();
  }
  return value as T[];
}

export function parsePVEBackupInventory(payload: unknown): PVEBackupsPayload {
  const data = inventoryData(payload);
  return {
    backupTasks: collection<BackupTask>(data, 'backupTasks', taskShape, true),
    storageBackups: collection<StorageBackup>(data, 'storageBackups', archiveShape, true),
    guestSnapshots: collection<GuestSnapshot>(data, 'guestSnapshots', snapshotShape, true),
  };
}

export function parsePBSBackupInventory(payload: unknown): PBSBackupsPayload {
  return { backups: collection<PBSBackup>(inventoryData(payload), 'backups', pbsShape) };
}

export async function readBackupInventoryJSON(response: Response): Promise<unknown> {
  try {
    return await response.json();
  } catch {
    throw new BackupInventoryFormatError();
  }
}

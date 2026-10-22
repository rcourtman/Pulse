import type {
  BackupTask,
  GuestSnapshot,
  PBSBackup,
  PBSBackupsPayload,
  PVEBackupsPayload,
  StorageBackup,
} from '@/types/api';
import { apiFetch } from '@/utils/apiClient';

// These are reads of already-collected snapshots, not backup/guest operations.
// Allow the client's existing two-minute Retry-After delay, but never leave the
// manual refresh/retry controls blocked by unanswered headers or a partial body.
export const BACKUP_INVENTORY_READ_TIMEOUT_MS = 150_000;

export class BackupInventoryTimeoutError extends Error {
  constructor() {
    super('The inventory read timed out. Missing restore points do not mean no backups exist.');
    this.name = 'BackupInventoryTimeoutError';
  }
}

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
  observedAt: optional(isString),
  error: optional(isString),
};
const archiveShape: RowShape = {
  ...pveGuestShape,
  time: isString,
  storage: optional(isString),
  storageNode: optional(isString),
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

function artifacts<T extends { id: string }>(
  data: Record<string, unknown>,
  key: string,
  shape: RowShape,
  allowNil = false,
): T[] {
  const rows = collection<T>(data, key, shape, allowNil);
  // These opaque IDs own retained rows and evidence dialogs. Neither response
  // order nor a silent deduplication may choose between duplicate observations.
  // Collections have separate kind prefixes; task observations are not keyed
  // artifact rows and retain their existing chronology/conflict handling.
  const ids = new Set<string>();
  for (const row of rows) {
    if (ids.has(row.id)) throw new BackupInventoryFormatError();
    ids.add(row.id);
  }
  return rows;
}

export function parsePVEBackupInventory(payload: unknown): PVEBackupsPayload {
  const data = inventoryData(payload);
  return {
    backupTasks: collection<BackupTask>(data, 'backupTasks', taskShape, true),
    storageBackups: artifacts<StorageBackup>(data, 'storageBackups', archiveShape, true),
    guestSnapshots: artifacts<GuestSnapshot>(data, 'guestSnapshots', snapshotShape, true),
  };
}

export function parsePBSBackupInventory(payload: unknown): PBSBackupsPayload {
  return { backups: artifacts<PBSBackup>(inventoryData(payload), 'backups', pbsShape) };
}

export async function readBackupInventoryJSON(response: Response): Promise<unknown> {
  try {
    return await response.json();
  } catch {
    throw new BackupInventoryFormatError();
  }
}

async function fetchBackupInventory<T>(
  source: 'pve' | 'pbs',
  parse: (payload: unknown) => T,
  signal: AbortSignal,
): Promise<T> {
  signal.throwIfAborted();
  const controller = new AbortController();
  let cancel!: () => void;
  let timer!: ReturnType<typeof setTimeout>;
  const interrupted = new Promise<never>((_resolve, reject) => {
    cancel = () => {
      reject(signal.reason);
      controller.abort();
    };
    signal.addEventListener('abort', cancel, { once: true });
    timer = setTimeout(() => {
      // Settle locally even if the transport ignores abort. Aborting alone
      // cannot release the query owner or retire an uncooperative late body.
      reject(new BackupInventoryTimeoutError());
      controller.abort();
    }, BACKUP_INVENTORY_READ_TIMEOUT_MS);
  });
  try {
    return await Promise.race([
      interrupted,
      (async () => {
        const response = await apiFetch(`/api/backups/${source}`, { signal: controller.signal });
        if (!response.ok) {
          throw Object.assign(
            new Error(`Failed to load ${source.toUpperCase()} backups (${response.status})`),
            { status: response.status },
          );
        }
        return parse(await readBackupInventoryJSON(response));
      })(),
    ]);
  } finally {
    clearTimeout(timer);
    signal.removeEventListener('abort', cancel);
  }
}

export function fetchPVEBackupInventory(signal: AbortSignal): Promise<PVEBackupsPayload> {
  return fetchBackupInventory('pve', parsePVEBackupInventory, signal);
}

export function fetchPBSBackupInventory(signal: AbortSignal): Promise<PBSBackupsPayload> {
  return fetchBackupInventory('pbs', parsePBSBackupInventory, signal);
}

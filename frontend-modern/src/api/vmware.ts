import { apiFetchJSON } from '@/utils/apiClient';
import {
  arrayOrUndefined,
  finiteNumberOrUndefined,
  optionalTrimmedString,
  strictBoolean,
  trimmedString,
} from './responseUtils';

const VMWARE_CONNECTIONS_PATH = '/api/vmware/connections';

const REDACTED_SECRET = '********';

type RawVMwareConnection = Partial<VMwareConnection>;
type RawVMwareConnectionPollError = Partial<VMwareConnectionPollError>;
type RawVMwareConnectionPoll = Partial<VMwareConnectionPollStatus>;
type RawVMwareConnectionObservedSummary = Partial<VMwareConnectionObservedSummary>;

export interface VMwareConnectionPollError {
  at?: string;
  message?: string;
  category?: string;
}

export interface VMwareConnectionPollStatus {
  intervalSeconds?: number;
  lastAttemptAt?: string;
  lastSuccessAt?: string;
  consecutiveFailures?: number;
  lastError?: VMwareConnectionPollError;
}

export interface VMwareConnectionObservedSummary {
  collectedAt?: string;
  hosts: number;
  vms: number;
  datastores: number;
  viRelease?: string;
}

export interface VMwareConnection {
  id: string;
  name: string;
  host: string;
  port?: number;
  username?: string;
  password?: string;
  insecureSkipVerify: boolean;
  enabled: boolean;
  poll?: VMwareConnectionPollStatus;
  observed?: VMwareConnectionObservedSummary;
}

export interface VMwareConnectionInput {
  name?: string;
  host: string;
  port?: number;
  username?: string;
  password?: string;
  insecureSkipVerify?: boolean;
  enabled?: boolean;
}

export interface VMwareConnectionTestResult {
  success: boolean;
}

const normalizeVMwareConnectionPollError = (
  error: RawVMwareConnectionPollError | undefined,
): VMwareConnectionPollError | undefined => {
  if (!error || typeof error !== 'object') return undefined;
  return {
    at: optionalTrimmedString(error.at),
    message: optionalTrimmedString(error.message),
    category: optionalTrimmedString(error.category),
  };
};

const normalizeVMwareConnectionPoll = (
  poll: RawVMwareConnectionPoll | undefined,
): VMwareConnectionPollStatus | undefined => {
  if (!poll || typeof poll !== 'object') return undefined;
  return {
    intervalSeconds: finiteNumberOrUndefined(poll.intervalSeconds),
    lastAttemptAt: optionalTrimmedString(poll.lastAttemptAt),
    lastSuccessAt: optionalTrimmedString(poll.lastSuccessAt),
    consecutiveFailures: finiteNumberOrUndefined(poll.consecutiveFailures),
    lastError: normalizeVMwareConnectionPollError(poll.lastError),
  };
};

const normalizeVMwareConnectionObservedSummary = (
  observed: RawVMwareConnectionObservedSummary | undefined,
): VMwareConnectionObservedSummary | undefined => {
  if (!observed || typeof observed !== 'object') return undefined;
  return {
    collectedAt: optionalTrimmedString(observed.collectedAt),
    hosts: finiteNumberOrUndefined(observed.hosts) ?? 0,
    vms: finiteNumberOrUndefined(observed.vms) ?? 0,
    datastores: finiteNumberOrUndefined(observed.datastores) ?? 0,
    viRelease: optionalTrimmedString(observed.viRelease),
  };
};

const normalizeVMwareConnection = (
  connection: RawVMwareConnection & { test?: RawVMwareConnectionPoll },
): VMwareConnection => ({
  id: trimmedString(connection.id),
  name: optionalTrimmedString(connection.name) ?? '',
  host: trimmedString(connection.host),
  port: finiteNumberOrUndefined(connection.port),
  username: optionalTrimmedString(connection.username),
  password: optionalTrimmedString(connection.password),
  insecureSkipVerify: strictBoolean(connection.insecureSkipVerify),
  enabled: strictBoolean(connection.enabled),
  poll: normalizeVMwareConnectionPoll(connection.poll ?? connection.test),
  observed: normalizeVMwareConnectionObservedSummary(connection.observed),
});

const serializeVMwareConnectionInput = (input: VMwareConnectionInput) => ({
  ...(input.name !== undefined ? { name: input.name } : {}),
  host: input.host,
  ...(input.port !== undefined ? { port: input.port } : {}),
  ...(input.username !== undefined ? { username: input.username } : {}),
  ...(input.password !== undefined ? { password: input.password } : {}),
  ...(input.insecureSkipVerify !== undefined
    ? { insecureSkipVerify: input.insecureSkipVerify }
    : {}),
  ...(input.enabled !== undefined ? { enabled: input.enabled } : {}),
});

export const isRedactedVMwareSecret = (value: string | null | undefined) =>
  (value || '').trim() === REDACTED_SECRET;

export class VMwareAPI {
  static async listConnections(): Promise<VMwareConnection[]> {
    const response = await apiFetchJSON<RawVMwareConnection[]>(VMWARE_CONNECTIONS_PATH);
    const list = arrayOrUndefined<RawVMwareConnection>(response) ?? [];
    return list.map(normalizeVMwareConnection);
  }

  static async createConnection(input: VMwareConnectionInput): Promise<VMwareConnection> {
    const response = await apiFetchJSON<RawVMwareConnection>(VMWARE_CONNECTIONS_PATH, {
      method: 'POST',
      body: JSON.stringify(serializeVMwareConnectionInput(input)),
    });
    return normalizeVMwareConnection(response);
  }

  static async updateConnection(
    id: string,
    input: VMwareConnectionInput,
  ): Promise<VMwareConnection> {
    const response = await apiFetchJSON<RawVMwareConnection>(
      `${VMWARE_CONNECTIONS_PATH}/${encodeURIComponent(id)}`,
      {
        method: 'PUT',
        body: JSON.stringify(serializeVMwareConnectionInput(input)),
      },
    );
    return normalizeVMwareConnection(response);
  }

  static async deleteConnection(id: string): Promise<{ success: boolean; id: string }> {
    return apiFetchJSON(`${VMWARE_CONNECTIONS_PATH}/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
  }

  static async testConnection(input: VMwareConnectionInput): Promise<VMwareConnectionTestResult> {
    const response = await apiFetchJSON<Partial<VMwareConnectionTestResult>>(
      `${VMWARE_CONNECTIONS_PATH}/test`,
      {
        method: 'POST',
        body: JSON.stringify(serializeVMwareConnectionInput(input)),
      },
    );
    return {
      success: strictBoolean(response.success),
    };
  }

  static async testSavedConnection(
    id: string,
    input?: VMwareConnectionInput,
  ): Promise<VMwareConnectionTestResult> {
    const response = await apiFetchJSON<Partial<VMwareConnectionTestResult>>(
      `${VMWARE_CONNECTIONS_PATH}/${encodeURIComponent(id)}/test`,
      {
        method: 'POST',
        ...(input !== undefined
          ? { body: JSON.stringify(serializeVMwareConnectionInput(input)) }
          : {}),
      },
    );
    return {
      success: strictBoolean(response.success),
    };
  }
}

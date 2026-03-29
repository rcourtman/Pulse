import { apiFetchJSON } from '@/utils/apiClient';
import {
  arrayOrUndefined,
  finiteNumberOrUndefined,
  optionalTrimmedString,
  strictBoolean,
  trimmedString,
} from './responseUtils';

const TRUE_NAS_CONNECTIONS_PATH = '/api/truenas/connections';

const REDACTED_SECRET = '********';

type RawTrueNASConnection = Partial<TrueNASConnection>;

export interface TrueNASConnection {
  id: string;
  name: string;
  host: string;
  port?: number;
  apiKey?: string;
  username?: string;
  password?: string;
  useHttps: boolean;
  insecureSkipVerify: boolean;
  fingerprint?: string;
  enabled: boolean;
  pollIntervalSeconds?: number;
}

export interface TrueNASConnectionInput {
  name?: string;
  host: string;
  port?: number;
  apiKey?: string;
  username?: string;
  password?: string;
  useHttps?: boolean;
  insecureSkipVerify?: boolean;
  fingerprint?: string;
  enabled?: boolean;
  pollIntervalSeconds?: number;
}

export interface TrueNASConnectionTestResult {
  success: boolean;
}

const normalizeTrueNASConnection = (connection: RawTrueNASConnection): TrueNASConnection => ({
  id: trimmedString(connection.id),
  name: optionalTrimmedString(connection.name) ?? '',
  host: trimmedString(connection.host),
  port: finiteNumberOrUndefined(connection.port),
  apiKey: optionalTrimmedString(connection.apiKey),
  username: optionalTrimmedString(connection.username),
  password: optionalTrimmedString(connection.password),
  useHttps: strictBoolean(connection.useHttps),
  insecureSkipVerify: strictBoolean(connection.insecureSkipVerify),
  fingerprint: optionalTrimmedString(connection.fingerprint),
  enabled: strictBoolean(connection.enabled),
  pollIntervalSeconds: finiteNumberOrUndefined(connection.pollIntervalSeconds),
});

const serializeTrueNASConnectionInput = (input: TrueNASConnectionInput) => ({
  ...(input.name !== undefined ? { name: input.name } : {}),
  host: input.host,
  ...(input.port !== undefined ? { port: input.port } : {}),
  ...(input.apiKey !== undefined ? { apiKey: input.apiKey } : {}),
  ...(input.username !== undefined ? { username: input.username } : {}),
  ...(input.password !== undefined ? { password: input.password } : {}),
  ...(input.useHttps !== undefined ? { useHttps: input.useHttps } : {}),
  ...(input.insecureSkipVerify !== undefined
    ? { insecureSkipVerify: input.insecureSkipVerify }
    : {}),
  ...(input.fingerprint !== undefined ? { fingerprint: input.fingerprint } : {}),
  ...(input.enabled !== undefined ? { enabled: input.enabled } : {}),
  ...(input.pollIntervalSeconds !== undefined
    ? { pollIntervalSeconds: input.pollIntervalSeconds }
    : {}),
});

export const isRedactedTrueNASSecret = (value: string | null | undefined) =>
  (value || '').trim() === REDACTED_SECRET;

export class TrueNASAPI {
  static async listConnections(): Promise<TrueNASConnection[]> {
    const response = await apiFetchJSON<RawTrueNASConnection[]>(TRUE_NAS_CONNECTIONS_PATH);
    const list = arrayOrUndefined<RawTrueNASConnection>(response) ?? [];
    return list.map(normalizeTrueNASConnection);
  }

  static async createConnection(input: TrueNASConnectionInput): Promise<TrueNASConnection> {
    const response = await apiFetchJSON<RawTrueNASConnection>(TRUE_NAS_CONNECTIONS_PATH, {
      method: 'POST',
      body: JSON.stringify(serializeTrueNASConnectionInput(input)),
    });
    return normalizeTrueNASConnection(response);
  }

  static async updateConnection(
    id: string,
    input: TrueNASConnectionInput,
  ): Promise<TrueNASConnection> {
    const response = await apiFetchJSON<RawTrueNASConnection>(
      `${TRUE_NAS_CONNECTIONS_PATH}/${encodeURIComponent(id)}`,
      {
        method: 'PUT',
        body: JSON.stringify(serializeTrueNASConnectionInput(input)),
      },
    );
    return normalizeTrueNASConnection(response);
  }

  static async deleteConnection(id: string): Promise<{ success: boolean; id: string }> {
    return apiFetchJSON(`${TRUE_NAS_CONNECTIONS_PATH}/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
  }

  static async testConnection(input: TrueNASConnectionInput): Promise<TrueNASConnectionTestResult> {
    const response = await apiFetchJSON<Partial<TrueNASConnectionTestResult>>(
      `${TRUE_NAS_CONNECTIONS_PATH}/test`,
      {
        method: 'POST',
        body: JSON.stringify(serializeTrueNASConnectionInput(input)),
      },
    );
    return {
      success: strictBoolean(response.success),
    };
  }
}

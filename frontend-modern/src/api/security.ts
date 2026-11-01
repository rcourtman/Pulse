import { apiFetchJSON } from '@/utils/apiClient';
import type { SecurityStatus } from '@/types/config';
import type { APITokenRecord as APITokenRecordModel } from '@/types/api';

export type APITokenRecord = APITokenRecordModel;

function isTokenInventoryRecord(value: unknown): value is APITokenRecord {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return false;
  const record = value as Record<string, unknown>;
  return (
    ['id', 'name', 'prefix', 'suffix', 'createdAt'].every(
      (key) => typeof record[key] === 'string',
    ) &&
    (record.id as string).trim().length > 0 &&
    ['lastUsedAt', 'expiresAt', 'ownerUserId'].every(
      (key) => record[key] == null || typeof record[key] === 'string',
    ) &&
    (record.scopes == null ||
      (Array.isArray(record.scopes) &&
        record.scopes.every((scope) => typeof scope === 'string' && scope.trim().length > 0)))
  );
}

export interface APITokenRequestOptions {
  signal?: AbortSignal;
  expectedOrgID?: string;
}

export interface CreateAPITokenResponse {
  token: string;
  record: APITokenRecord;
}

export class SecurityAPI {
  static async getStatus(): Promise<SecurityStatus> {
    return apiFetchJSON<SecurityStatus>('/api/security/status');
  }

  static async getToken(id: string): Promise<APITokenRecord> {
    const response = await apiFetchJSON<{ record: APITokenRecord }>(
      `/api/security/tokens/${encodeURIComponent(id)}`,
    );
    return response.record;
  }

  static async listTokens(options?: APITokenRequestOptions): Promise<APITokenRecord[]> {
    const response = await apiFetchJSON<unknown>('/api/security/tokens', {
      ...options,
      retry: false,
    });
    if (response && typeof response === 'object' && !Array.isArray(response)) {
      const tokens = (response as { tokens?: unknown }).tokens;
      // A legacy nil list is explicitly empty; a missing or malformed list is not.
      if (tokens === null) return [];
      if (Array.isArray(tokens) && tokens.every(isTokenInventoryRecord)) {
        const ids = new Set(tokens.map((token) => token.id));
        if (ids.size === tokens.length) return tokens;
      }
    }
    throw new Error('Invalid API token inventory.');
  }

  static async createToken(
    name?: string,
    scopes?: string[],
    options?: APITokenRequestOptions,
  ): Promise<CreateAPITokenResponse> {
    const payload: Record<string, unknown> = {};
    if (name) {
      payload.name = name;
    }
    if (scopes) {
      payload.scopes = scopes;
    }

    return apiFetchJSON<CreateAPITokenResponse>('/api/security/tokens', {
      ...options,
      retry: false,
      method: 'POST',
      body: JSON.stringify(payload),
    });
  }

  static async createRelayMobileAccessToken(): Promise<CreateAPITokenResponse> {
    return apiFetchJSON<CreateAPITokenResponse>('/api/security/tokens/relay-mobile', {
      method: 'POST',
      body: JSON.stringify({}),
    });
  }

  static async updateTokenScopes(
    id: string,
    scopes: string[],
    options?: APITokenRequestOptions,
  ): Promise<APITokenRecord> {
    const response = await apiFetchJSON<{ record: APITokenRecord }>(
      `/api/security/tokens/${encodeURIComponent(id)}`,
      {
        ...options,
        retry: false,
        method: 'PATCH',
        body: JSON.stringify({ scopes }),
      },
    );
    return response.record;
  }

  static async deleteToken(id: string, options?: APITokenRequestOptions): Promise<void> {
    await apiFetchJSON(`/api/security/tokens/${encodeURIComponent(id)}`, {
      ...options,
      retry: false,
      method: 'DELETE',
    });
  }

  static async renameToken(
    id: string,
    name: string,
    options?: APITokenRequestOptions,
  ): Promise<APITokenRecord> {
    const response = await apiFetchJSON<{ record: APITokenRecord }>(
      `/api/security/tokens/${encodeURIComponent(id)}`,
      {
        ...options,
        retry: false,
        method: 'PATCH',
        body: JSON.stringify({ name }),
      },
    );
    return response.record;
  }
}

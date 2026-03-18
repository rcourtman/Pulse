import { apiFetchJSON } from '@/utils/apiClient';
import type { SecurityStatus } from '@/types/config';
import type { APITokenRecord as APITokenRecordModel } from '@/types/api';
import { objectArrayFieldOrEmpty } from './responseUtils';

export type APITokenRecord = APITokenRecordModel;

export interface CreateAPITokenResponse {
  token: string;
  record: APITokenRecord;
}

export class SecurityAPI {
  static async getStatus(): Promise<SecurityStatus> {
    return apiFetchJSON<SecurityStatus>('/api/security/status');
  }

  static async listTokens(): Promise<APITokenRecord[]> {
    const response = await apiFetchJSON<{ tokens: APITokenRecord[] }>('/api/security/tokens');
    return objectArrayFieldOrEmpty<APITokenRecord>(response, 'tokens');
  }

  static async createToken(name?: string, scopes?: string[]): Promise<CreateAPITokenResponse> {
    const payload: Record<string, unknown> = {};
    if (name) {
      payload.name = name;
    }
    if (scopes) {
      payload.scopes = scopes;
    }

    return apiFetchJSON<CreateAPITokenResponse>('/api/security/tokens', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  }

  static async deleteToken(id: string): Promise<void> {
    await apiFetchJSON(`/api/security/tokens/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
  }
}

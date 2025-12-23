import { apiFetchJSON } from '@/utils/apiClient';
import type { SecurityStatus } from '@/types/config';

export interface APITokenRecord {
  id: string;
  name: string;
  prefix: string;
  suffix: string;
  createdAt: string;
  lastUsedAt?: string;
  scopes?: string[];
}

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
    return response.tokens ?? [];
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
    await apiFetchJSON(`/api/security/tokens/${id}`, {
      method: 'DELETE',
    });
  }
}

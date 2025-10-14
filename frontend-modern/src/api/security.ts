import { apiFetchJSON } from '@/utils/apiClient';

export interface APITokenRecord {
  id: string;
  name: string;
  prefix: string;
  suffix: string;
  createdAt: string;
  lastUsedAt?: string;
}

export interface CreateAPITokenResponse {
  token: string;
  record: APITokenRecord;
}

export class SecurityAPI {
  static async listTokens(): Promise<APITokenRecord[]> {
    const response = await apiFetchJSON<{ tokens: APITokenRecord[] }>('/api/security/tokens');
    return response.tokens ?? [];
  }

  static async createToken(name?: string): Promise<CreateAPITokenResponse> {
    return apiFetchJSON<CreateAPITokenResponse>('/api/security/tokens', {
      method: 'POST',
      body: JSON.stringify({ name }),
    });
  }

  static async deleteToken(id: string): Promise<void> {
    await apiFetchJSON(`/api/security/tokens/${id}`, {
      method: 'DELETE',
    });
  }
}

import { apiClient } from '@/utils/apiClient';
import { normalizeStructuredAPIError, parseJSONSafe } from './responseUtils';

export interface HostedSignupRequest {
  email: string;
  org_name: string;
  tier?: 'starter' | 'power' | 'max';
}

export interface HostedSignupResponse {
  org_id?: string;
  user_id?: string;
  checkout_url?: string;
  message: string;
}

export interface HostedMagicLinkResponse {
  success: boolean;
  message: string;
}

export interface HostedAPIError {
  code: string;
  message: string;
  details?: Record<string, string>;
}

export type HostedAPIResult<T> =
  | { ok: true; status: number; data: T }
  | { ok: false; status: number; error: HostedAPIError };

export class HostedSignupAPI {
  static async signup(
    payload: HostedSignupRequest,
  ): Promise<HostedAPIResult<HostedSignupResponse>> {
    const response = await apiClient.fetch('/api/public/signup', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
      body: JSON.stringify(payload),
      skipAuth: true,
      skipOrgContext: true,
    });

    const body = await parseJSONSafe<HostedSignupResponse | HostedAPIError>(response);
    if (response.ok) {
      return {
        ok: true,
        status: response.status,
        data: body as HostedSignupResponse,
      };
    }
    return {
      ok: false,
      status: response.status,
      error: normalizeStructuredAPIError(body, response.status),
    };
  }

  static async requestMagicLink(email: string): Promise<HostedAPIResult<HostedMagicLinkResponse>> {
    const response = await apiClient.fetch('/api/public/magic-link/request', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
      body: JSON.stringify({ email }),
      skipAuth: true,
      skipOrgContext: true,
    });

    const body = await parseJSONSafe<HostedMagicLinkResponse | HostedAPIError>(response);
    if (response.ok) {
      return {
        ok: true,
        status: response.status,
        data: body as HostedMagicLinkResponse,
      };
    }
    return {
      ok: false,
      status: response.status,
      error: normalizeStructuredAPIError(body, response.status),
    };
  }
}

import { apiClient } from '@/utils/apiClient';

export interface HostedSignupRequest {
  email: string;
  org_name: string;
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

function normalizeHostedError(payload: unknown, fallbackStatus: number): HostedAPIError {
  if (payload && typeof payload === 'object') {
    const obj = payload as Record<string, unknown>;
    const code =
      typeof obj.code === 'string' && obj.code.trim() !== '' ? obj.code : 'request_failed';
    const message =
      typeof obj.message === 'string' && obj.message.trim() !== ''
        ? obj.message
        : `Request failed (${fallbackStatus})`;
    const details =
      obj.details && typeof obj.details === 'object'
        ? (obj.details as Record<string, string>)
        : undefined;
    return { code, message, details };
  }
  return {
    code: 'request_failed',
    message: `Request failed (${fallbackStatus})`,
  };
}

async function parseJSONSafe(response: Response): Promise<unknown> {
  try {
    return await response.json();
  } catch {
    return null;
  }
}

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

    const body = await parseJSONSafe(response);
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
      error: normalizeHostedError(body, response.status),
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

    const body = await parseJSONSafe(response);
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
      error: normalizeHostedError(body, response.status),
    };
  }
}

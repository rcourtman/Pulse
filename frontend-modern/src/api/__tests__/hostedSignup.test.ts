import { beforeEach, describe, expect, it, vi } from 'vitest';
import { HostedSignupAPI } from '../hostedSignup';
import { apiClient } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiClient: {
    fetch: vi.fn(),
  },
}));

describe('HostedSignupAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('submits hosted signup with public request options', async () => {
    vi.mocked(apiClient.fetch).mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          org_id: 'org-1',
          user_id: 'owner@example.com',
          message: 'Check your email.',
        }),
        { status: 201 },
      ),
    );

    const result = await HostedSignupAPI.signup({
      email: 'owner@example.com',
      org_name: 'Acme',
    });

    expect(apiClient.fetch).toHaveBeenCalledWith('/api/public/signup', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
      body: JSON.stringify({
        email: 'owner@example.com',
        org_name: 'Acme',
      }),
      skipAuth: true,
      skipOrgContext: true,
    });
    expect(result).toEqual({
      ok: true,
      status: 201,
      data: {
        org_id: 'org-1',
        user_id: 'owner@example.com',
        message: 'Check your email.',
      },
    });
  });

  it('returns structured error payload on failed signup', async () => {
    vi.mocked(apiClient.fetch).mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          code: 'invalid_email',
          message: 'Invalid email format',
        }),
        { status: 400 },
      ),
    );

    const result = await HostedSignupAPI.signup({
      email: 'bad',
      org_name: 'Acme',
    });

    expect(result).toEqual({
      ok: false,
      status: 400,
      error: {
        code: 'invalid_email',
        message: 'Invalid email format',
      },
    });
  });

  it('requests public magic link with public request options', async () => {
    vi.mocked(apiClient.fetch).mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          success: true,
          message: "If that email is registered, you'll receive a magic link shortly.",
        }),
        { status: 200 },
      ),
    );

    const result = await HostedSignupAPI.requestMagicLink('owner@example.com');

    expect(apiClient.fetch).toHaveBeenCalledWith('/api/public/magic-link/request', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
      body: JSON.stringify({ email: 'owner@example.com' }),
      skipAuth: true,
      skipOrgContext: true,
    });
    expect(result).toEqual({
      ok: true,
      status: 200,
      data: {
        success: true,
        message: "If that email is registered, you'll receive a magic link shortly.",
      },
    });
  });
});

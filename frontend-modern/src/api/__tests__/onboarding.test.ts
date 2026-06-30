import { describe, expect, it, vi, beforeEach } from 'vitest';
import { OnboardingAPI, OnboardingNotReadyError, type OnboardingQRResponse } from '../onboarding';
import { apiErrorFromResponse, apiFetch } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiErrorFromResponse: vi.fn(),
  apiFetch: vi.fn(),
}));

describe('OnboardingAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getQRPayload', () => {
    it('fetches QR payload for onboarding', async () => {
      const mockResponse: OnboardingQRResponse = {
        schema: 'pulse',
        instance_url: 'https://pulse.example.com',
        instance_id: 'inst-123',
        relay: { enabled: true, url: 'https://relay.example.com' },
        auth_token: 'token-123',
        deep_link: 'pulse://connect',
      };
      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response(JSON.stringify(mockResponse), { status: 200 }),
      );

      const result = await OnboardingAPI.getQRPayload();

      expect(apiFetch).toHaveBeenCalledWith('/api/onboarding/qr');
      expect(result).toEqual(mockResponse);
    });

    it('uses the provided API token when requesting a pairing payload', async () => {
      const mockResponse: OnboardingQRResponse = {
        schema: 'pulse',
        instance_url: 'https://pulse.example.com',
        instance_id: 'inst-123',
        relay: { enabled: true, url: 'https://relay.example.com' },
        auth_token: 'token-123',
        deep_link: 'pulse://connect',
      };
      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response(JSON.stringify(mockResponse), { status: 200 }),
      );

      await OnboardingAPI.getQRPayload('token-123');

      expect(apiFetch).toHaveBeenCalledWith('/api/onboarding/qr', {
        headers: { 'X-API-Token': 'token-123' },
      });
    });

    it('throws readiness diagnostics when the backend refuses an incomplete pairing payload', async () => {
      const diagnostics = [
        {
          code: 'relay_registration_unavailable',
          severity: 'error' as const,
          message: 'Remote Access is enabled, but this Pulse instance is not connected yet.',
          field: 'instance_id',
        },
      ];
      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            code: 'onboarding_not_ready',
            message: 'Pulse Mobile pairing is not ready yet.',
            diagnostics,
          }),
          { status: 409 },
        ),
      );

      await expect(OnboardingAPI.getQRPayload('token-123')).rejects.toMatchObject({
        name: 'OnboardingNotReadyError',
        code: 'onboarding_not_ready',
        status: 409,
        diagnostics,
      } satisfies Partial<OnboardingNotReadyError>);
    });

    it('delegates non-readiness failures to the shared API error parser', async () => {
      const response = new Response('server failed', { status: 500 });
      const parsedError = new Error('parsed server failure');
      vi.mocked(apiFetch).mockResolvedValueOnce(response);
      vi.mocked(apiErrorFromResponse).mockResolvedValueOnce(parsedError);

      await expect(OnboardingAPI.getQRPayload('token-123')).rejects.toBe(parsedError);

      expect(apiErrorFromResponse).toHaveBeenCalledWith(response);
    });
  });
});

import { describe, expect, it, vi, beforeEach } from 'vitest';
import { OnboardingAPI, type OnboardingQRResponse } from '../onboarding';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
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
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockResponse);

      const result = await OnboardingAPI.getQRPayload();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/onboarding/qr');
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
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockResponse);

      await OnboardingAPI.getQRPayload('token-123');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/onboarding/qr', {
        headers: { 'X-API-Token': 'token-123' },
      });
    });
  });
});

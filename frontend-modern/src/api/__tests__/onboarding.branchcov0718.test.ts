import { describe, expect, it, vi, beforeEach } from 'vitest';
import { OnboardingAPI, OnboardingNotReadyError } from '../onboarding';
import { apiErrorFromResponse, apiFetch } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiErrorFromResponse: vi.fn(),
  apiFetch: vi.fn(),
}));

describe('OnboardingAPI – branch coverage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getQRPayload – 409 fall-through arms', () => {
    it('delegates to apiErrorFromResponse when a 409 body carries a non-readiness code', async () => {
      const response = new Response(
        JSON.stringify({ code: 'some_other_code', message: 'unrelated conflict' }),
        { status: 409 },
      );
      const parsedError = new Error('parsed non-readiness conflict');
      vi.mocked(apiFetch).mockResolvedValueOnce(response);
      vi.mocked(apiErrorFromResponse).mockResolvedValueOnce(parsedError);

      await expect(OnboardingAPI.getQRPayload('token-123')).rejects.toBe(parsedError);

      expect(apiErrorFromResponse).toHaveBeenCalledWith(response);
    });

    it('delegates to apiErrorFromResponse when a 409 body is not parseable JSON (readJSON swallows the parse failure)', async () => {
      const response = new Response('<not-json>', { status: 409 });
      const parsedError = new Error('parsed unparseable conflict');
      vi.mocked(apiFetch).mockResolvedValueOnce(response);
      vi.mocked(apiErrorFromResponse).mockResolvedValueOnce(parsedError);

      await expect(OnboardingAPI.getQRPayload()).rejects.toBe(parsedError);

      expect(apiErrorFromResponse).toHaveBeenCalledWith(response);
    });

    it('delegates to apiErrorFromResponse when a 409 body is valid JSON but not an object', async () => {
      const response = new Response(JSON.stringify('a plain string'), { status: 409 });
      const parsedError = new Error('parsed non-object conflict');
      vi.mocked(apiFetch).mockResolvedValueOnce(response);
      vi.mocked(apiErrorFromResponse).mockResolvedValueOnce(parsedError);

      await expect(OnboardingAPI.getQRPayload()).rejects.toBe(parsedError);

      expect(apiErrorFromResponse).toHaveBeenCalledWith(response);
    });
  });

  describe('OnboardingNotReadyError – message/diagnostics fallback arms', () => {
    it('uses response.error for the message when response.message is absent', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            code: 'onboarding_not_ready',
            error: 'Relay has not published its fingerprint yet.',
          }),
          { status: 409 },
        ),
      );

      await expect(OnboardingAPI.getQRPayload('token-123')).rejects.toMatchObject({
        name: 'OnboardingNotReadyError',
        code: 'onboarding_not_ready',
        status: 409,
        message: 'Relay has not published its fingerprint yet.',
      } satisfies Partial<OnboardingNotReadyError>);
    });

    it('falls back to the default readiness message when neither message nor error is present', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            code: 'onboarding_not_ready',
          }),
          { status: 409 },
        ),
      );

      await expect(OnboardingAPI.getQRPayload()).rejects.toMatchObject({
        name: 'OnboardingNotReadyError',
        code: 'onboarding_not_ready',
        status: 409,
        message: 'Pulse Mobile pairing is not ready yet.',
      } satisfies Partial<OnboardingNotReadyError>);
    });

    it('defaults diagnostics to an empty array when the response omits them', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            code: 'onboarding_not_ready',
            message: 'No diagnostics attached.',
          }),
          { status: 409 },
        ),
      );

      let caught: OnboardingNotReadyError | undefined;
      try {
        await OnboardingAPI.getQRPayload('token-123');
      } catch (err) {
        caught = err as OnboardingNotReadyError;
      }

      expect(caught).toBeInstanceOf(OnboardingNotReadyError);
      expect(caught?.code).toBe('onboarding_not_ready');
      expect(caught?.status).toBe(409);
      expect(caught?.diagnostics).toEqual([]);
    });
  });

});

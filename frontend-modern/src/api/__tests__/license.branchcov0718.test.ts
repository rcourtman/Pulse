import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LicenseAPI } from '../license';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('LicenseAPI branch coverage (status / features / activate / clear)', () => {
  const mock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getStatus', () => {
    it('GETs the canonical license status endpoint and returns the parsed body untouched', async () => {
      const status = {
        valid: true,
        tier: 'pro',
        plan_version: 'pro-v1',
        email: 'ops@example.com',
        expires_at: '2027-01-01T00:00:00Z',
        is_lifetime: false,
        days_remaining: 180,
        features: ['relay', 'audit_logging'],
        max_guests: 25,
        in_grace_period: false,
        grace_period_end: null,
      };
      mock.mockResolvedValueOnce(status);

      const result = await LicenseAPI.getStatus();

      expect(mock).toHaveBeenCalledTimes(1);
      // Status reads are a plain GET — no options object is forwarded.
      expect(mock).toHaveBeenCalledWith('/api/license/status');
      // The function performs no normalization: the parsed payload is returned verbatim.
      expect(result).toStrictEqual(status);
    });

    it('returns a lifetime/community payload verbatim when optional fields are absent', async () => {
      const status = {
        valid: true,
        tier: 'community',
        is_lifetime: true,
        days_remaining: 0,
        features: [],
      };
      mock.mockResolvedValueOnce(status);

      const result = await LicenseAPI.getStatus();

      expect(mock).toHaveBeenCalledWith('/api/license/status');
      // Optional fields (plan_version, email, expires_at, max_guests, in_grace_period, grace_period_end)
      // are absent on the response and remain absent on the return value.
      expect(result).toStrictEqual(status);
      expect('email' in result).toBe(false);
      expect('expires_at' in result).toBe(false);
    });

    it('propagates transport errors from a non-ok status response', async () => {
      mock.mockRejectedValueOnce(new Error('license status unavailable'));

      await expect(LicenseAPI.getStatus()).rejects.toThrow('license status unavailable');
      expect(mock).toHaveBeenCalledWith('/api/license/status');
    });
  });

  describe('getFeatures', () => {
    it('GETs the features endpoint and returns the parsed feature map untouched', async () => {
      const features = {
        license_status: 'active',
        features: { relay: true, audit_logging: true, sso: false },
        upgrade_url: '/settings/billing',
      };
      mock.mockResolvedValueOnce(features);

      const result = await LicenseAPI.getFeatures();

      expect(mock).toHaveBeenCalledTimes(1);
      expect(mock).toHaveBeenCalledWith('/api/license/features');
      expect(result).toStrictEqual(features);
    });

    it('preserves an empty feature map without coalescing it to a default', async () => {
      const features = {
        license_status: 'expired',
        features: {},
        upgrade_url: '',
      };
      mock.mockResolvedValueOnce(features);

      const result = await LicenseAPI.getFeatures();

      expect(mock).toHaveBeenCalledWith('/api/license/features');
      // The function does not default-empty feature maps — an empty object stays empty.
      expect(result.features).toStrictEqual({});
      expect(result.upgrade_url).toBe('');
    });

    it('propagates transport errors from a non-ok features response', async () => {
      mock.mockRejectedValueOnce(new Error('features endpoint down'));

      await expect(LicenseAPI.getFeatures()).rejects.toThrow('features endpoint down');
      expect(mock).toHaveBeenCalledWith('/api/license/features');
    });
  });

  describe('activateLicense', () => {
    it('POSTs the license key to /activate under the license_key body field', async () => {
      const activationResponse = {
        success: true,
        message: 'License activated',
        status: {
          valid: true,
          tier: 'pro',
          is_lifetime: false,
          days_remaining: 365,
          features: ['relay'],
        },
      };
      mock.mockResolvedValueOnce(activationResponse);

      const result = await LicenseAPI.activateLicense('PULSE-PRO-AAAA-BBBB-CCCC');

      expect(mock).toHaveBeenCalledTimes(1);
      // Asserts the exact request shape: POST to the fixed activate path with a
      // JSON body that wraps the raw key under `license_key` (NOT `key`, NOT `licenseKey`).
      expect(mock).toHaveBeenCalledWith('/api/license/activate', {
        method: 'POST',
        body: JSON.stringify({ license_key: 'PULSE-PRO-AAAA-BBBB-CCCC' }),
      });
      expect(result).toStrictEqual(activationResponse);
      // The nested `status` object is passed through untouched (no normalization).
      expect(result.status).toMatchObject({ tier: 'pro', days_remaining: 365 });
    });

    it('returns the bare success flag when status/message are absent on the response', async () => {
      const activationResponse = { success: true };
      mock.mockResolvedValueOnce(activationResponse);

      const result = await LicenseAPI.activateLicense('K');

      expect(mock).toHaveBeenCalledWith('/api/license/activate', {
        method: 'POST',
        body: JSON.stringify({ license_key: 'K' }),
      });
      // Optional `status` / `message` fields are absent — function does not synthesize defaults.
      expect(result).toStrictEqual({ success: true });
      expect('status' in result).toBe(false);
      expect('message' in result).toBe(false);
    });

    it('still POSTs an empty-string key as {"license_key":""} rather than omitting the body field', async () => {
      mock.mockResolvedValueOnce({ success: false, message: 'empty key' });

      await LicenseAPI.activateLicense('');

      // Empty-string input exercises the unconditional body-construction arm:
      // there is no `if (licenseKey)` guard, so even "" is serialized.
      expect(mock).toHaveBeenCalledWith('/api/license/activate', {
        method: 'POST',
        body: JSON.stringify({ license_key: '' }),
      });
    });

    it('passes keys containing JSON-significant characters through JSON.stringify untouched', async () => {
      mock.mockResolvedValueOnce({ success: true });

      // Backslash, double-quote and newline would be encoded by JSON.stringify,
      // confirming the body is built via JSON.stringify({ license_key }) and not naive concat.
      const trickyKey = 'a"b\\c\n';
      await LicenseAPI.activateLicense(trickyKey);

      expect(mock).toHaveBeenCalledWith('/api/license/activate', {
        method: 'POST',
        body: JSON.stringify({ license_key: trickyKey }),
      });
      // Re-derive the encoded form to assert the on-the-wire body, not just call args.
      expect(JSON.stringify({ license_key: trickyKey })).toBe('{"license_key":"a\\"b\\\\c\\n"}');
    });

    it('propagates transport errors from a non-ok activate response (rejection invalidates nothing locally)', async () => {
      mock.mockRejectedValueOnce(new Error('invalid license key'));

      await expect(LicenseAPI.activateLicense('garbage')).rejects.toThrow('invalid license key');
      expect(mock).toHaveBeenCalledWith('/api/license/activate', {
        method: 'POST',
        body: JSON.stringify({ license_key: 'garbage' }),
      });
    });
  });

  describe('clearLicense', () => {
    it('POSTs an empty JSON object body to /clear and returns the parsed response', async () => {
      const clearResponse = { success: true, message: 'License cleared' };
      mock.mockResolvedValueOnce(clearResponse);

      const result = await LicenseAPI.clearLicense();

      expect(mock).toHaveBeenCalledTimes(1);
      // Distinct body arm from activateLicense: literal `{}` is serialized (not `{ license_key }`).
      // The body string is exactly two characters — proves no fields are leaked in.
      expect(mock).toHaveBeenCalledWith('/api/license/clear', {
        method: 'POST',
        body: '{}',
      });
      expect(result).toStrictEqual(clearResponse);
    });

    it('returns a bare success:false response verbatim without synthesizing a message', async () => {
      const clearResponse = { success: false };
      mock.mockResolvedValueOnce(clearResponse);

      const result = await LicenseAPI.clearLicense();

      expect(mock).toHaveBeenCalledWith('/api/license/clear', {
        method: 'POST',
        body: '{}',
      });
      expect(result).toStrictEqual({ success: false });
      expect('message' in result).toBe(false);
    });

    it('propagates transport errors from a non-ok clear response', async () => {
      mock.mockRejectedValueOnce(new Error('clear forbidden'));

      await expect(LicenseAPI.clearLicense()).rejects.toThrow('clear forbidden');
      expect(mock).toHaveBeenCalledWith('/api/license/clear', {
        method: 'POST',
        body: '{}',
      });
    });
  });
});

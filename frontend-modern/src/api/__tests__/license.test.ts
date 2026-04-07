import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LicenseAPI } from '../license';
import { apiClient, apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiClient: {
    fetch: vi.fn(),
  },
  apiFetchJSON: vi.fn(),
}));

describe('LicenseAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('reads runtime capabilities from the canonical runtime endpoint', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      capabilities: ['relay'],
      limits: [],
      hosted_mode: false,
      max_history_days: 14,
    });

    const result = await LicenseAPI.getRuntimeCapabilities();

    expect(apiFetchJSON).toHaveBeenCalledWith('/api/license/runtime-capabilities');
    expect(result).toMatchObject({
      capabilities: ['relay'],
      max_history_days: 14,
    });
  });

  it('reads commercial entitlements from the commercial endpoint', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      tier: 'pro',
      subscription_state: 'active',
      capabilities: ['relay'],
      limits: [],
      upgrade_reasons: [],
    });

    const result = await LicenseAPI.getCommercialEntitlements();

    expect(apiFetchJSON).toHaveBeenCalledWith('/api/license/entitlements');
    expect(result).toMatchObject({
      tier: 'pro',
      subscription_state: 'active',
    });
  });

  it('reads commercial posture from the public-safe commercial endpoint', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      tier: 'pro',
      subscription_state: 'active',
      upgrade_reasons: [],
      trial_eligible: false,
    });

    const result = await LicenseAPI.getCommercialPosture();

    expect(apiFetchJSON).toHaveBeenCalledWith('/api/license/commercial-posture');
    expect(result).toMatchObject({
      tier: 'pro',
      subscription_state: 'active',
    });
  });

  it('keeps getEntitlements as a compatibility alias for the commercial endpoint', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      tier: 'free',
      subscription_state: 'expired',
      capabilities: [],
      limits: [],
      upgrade_reasons: [],
    });

    await LicenseAPI.getEntitlements();

    expect(apiFetchJSON).toHaveBeenCalledWith('/api/license/entitlements');
  });

  it('starts trials through the raw license transport', async () => {
    vi.mocked(apiClient.fetch).mockResolvedValueOnce(new Response(null, { status: 202 }));

    const response = await LicenseAPI.startTrial();

    expect(apiClient.fetch).toHaveBeenCalledWith('/api/license/trial/start', {
      method: 'POST',
      headers: { Accept: 'application/json' },
    });
    expect(response.status).toBe(202);
  });
});

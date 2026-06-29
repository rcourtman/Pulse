import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/api/license', () => ({
  LicenseAPI: {
    getStatus: vi.fn(),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: vi.fn(),
    error: vi.fn(),
  },
}));

import { LicenseAPI } from '@/api/license';
import { hasFeature, licenseStatus, loadLicenseStatus } from '@/stores/license';

describe('license store', () => {
  const getStatusMock = vi.mocked(LicenseAPI.getStatus);

  beforeEach(() => {
    getStatusMock.mockReset();
  });

  it('normalizes legacy feature maps before checking features', async () => {
    getStatusMock.mockResolvedValueOnce({
      valid: true,
      tier: 'pro',
      is_lifetime: false,
      days_remaining: 30,
      features: { ai_autofix: true, ai_alerts: false },
    } as any);

    await loadLicenseStatus(true);

    expect(licenseStatus()?.features).toEqual(['ai_autofix']);
    expect(hasFeature('ai_autofix')).toBe(true);
    expect(hasFeature('ai_alerts')).toBe(false);
  });
});

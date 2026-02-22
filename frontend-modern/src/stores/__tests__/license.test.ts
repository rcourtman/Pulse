import { describe, expect, it, vi, beforeEach } from 'vitest';
import { LicenseAPI } from '@/api/license';
import {
    loadLicenseStatus,
    startProTrial,
    isPro,
    hasFeature,
    isMultiTenantEnabled,
    isHostedModeEnabled,
    getUpgradeReason,
    getUpgradeActionUrl,
    getFirstUpgradeActionUrl,
    getUpgradeActionUrlOrFallback,
    getLimit,
    isRangeLocked,
    entitlements,
    licenseLoaded,
} from '@/stores/license';

vi.mock('@/api/license');
vi.mock('@/stores/events', () => ({
    eventBus: { on: vi.fn() },
}));

describe('license store', () => {
    const mockProEntitlements = {
        tier: 'pro' as const,
        subscription_state: 'active',
        capabilities: ['feature1', 'feature2', 'multi_tenant'],
        limits: [{ key: 'limit1', value: 100 }],
        upgrade_reasons: [
            { key: 'reason1', title: 'Reason 1', action_url: '/upgrade/reason1' },
            { key: 'reason2', title: 'Reason 2', action_url: '/upgrade/reason2' },
        ],
        hosted_mode: false,
    };

    const mockFreeEntitlements = {
        tier: 'free' as const,
        subscription_state: 'expired',
        capabilities: [] as string[],
        limits: [] as { key: string; value: number }[],
        upgrade_reasons: [] as { key: string; title: string; action_url: string }[],
        hosted_mode: false,
    };

    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('loads entitlements from API', async () => {
        vi.mocked(LicenseAPI.getEntitlements).mockResolvedValue(mockProEntitlements);

        await loadLicenseStatus();

        expect(LicenseAPI.getEntitlements).toHaveBeenCalled();
        expect(entitlements()).toEqual(mockProEntitlements);
        expect(licenseLoaded()).toBe(true);
    });

    it('returns early if already loaded without force', async () => {
        vi.mocked(LicenseAPI.getEntitlements).mockResolvedValue(mockProEntitlements);

        await loadLicenseStatus(true);
        await loadLicenseStatus();

        expect(LicenseAPI.getEntitlements).toHaveBeenCalledTimes(1);
    });

    it('sets fallback entitlements on error', async () => {
        vi.mocked(LicenseAPI.getEntitlements).mockRejectedValue(new Error('API error'));

        await loadLicenseStatus(true);

        expect(entitlements()).toEqual({
            capabilities: ['update_alerts', 'sso', 'ai_patrol'],
            limits: [],
            subscription_state: 'expired',
            upgrade_reasons: [],
            tier: 'free',
            hosted_mode: false,
        });
    });

    it('startProTrial throws error if trial start fails', async () => {
        vi.mocked(LicenseAPI.startTrial).mockResolvedValue({ ok: false, status: 400 } as Response);

        await expect(startProTrial()).rejects.toThrow('Failed to start trial');
    });

    describe('isPro', () => {
        it('returns true for non-free tier', async () => {
            vi.mocked(LicenseAPI.getEntitlements).mockResolvedValue(mockProEntitlements);
            await loadLicenseStatus(true);
            expect(isPro()).toBe(true);
        });

        it('returns false for free tier', async () => {
            vi.mocked(LicenseAPI.getEntitlements).mockResolvedValue(mockFreeEntitlements);
            await loadLicenseStatus(true);
            expect(isPro()).toBe(false);
        });

        it('returns false when entitlements is null', () => {
            expect(isPro()).toBe(false);
        });
    });

    describe('hasFeature', () => {
        it('returns true when feature is in capabilities', async () => {
            vi.mocked(LicenseAPI.getEntitlements).mockResolvedValue(mockProEntitlements);
            await loadLicenseStatus(true);
            expect(hasFeature('feature1')).toBe(true);
        });

        it('returns false when feature is not in capabilities', async () => {
            vi.mocked(LicenseAPI.getEntitlements).mockResolvedValue(mockProEntitlements);
            await loadLicenseStatus(true);
            expect(hasFeature('missing_feature')).toBe(false);
        });
    });

    describe('isMultiTenantEnabled', () => {
        it('returns true when multi_tenant feature exists', async () => {
            vi.mocked(LicenseAPI.getEntitlements).mockResolvedValue(mockProEntitlements);
            await loadLicenseStatus(true);
            expect(isMultiTenantEnabled()).toBe(true);
        });

        it('returns false when feature does not exist', async () => {
            vi.mocked(LicenseAPI.getEntitlements).mockResolvedValue(mockFreeEntitlements);
            await loadLicenseStatus(true);
            expect(isMultiTenantEnabled()).toBe(false);
        });
    });

    describe('isHostedModeEnabled', () => {
        it('returns true when hosted_mode is true', async () => {
            vi.mocked(LicenseAPI.getEntitlements).mockResolvedValue({
                ...mockProEntitlements,
                hosted_mode: true,
            });
            await loadLicenseStatus(true);
            expect(isHostedModeEnabled()).toBe(true);
        });
    });

    describe('getUpgradeReason', () => {
        it('returns upgrade reason by key', async () => {
            vi.mocked(LicenseAPI.getEntitlements).mockResolvedValue(mockProEntitlements);
            await loadLicenseStatus(true);
            expect(getUpgradeReason('reason2')?.key).toBe('reason2');
        });

        it('returns undefined for unknown key', async () => {
            vi.mocked(LicenseAPI.getEntitlements).mockResolvedValue(mockProEntitlements);
            await loadLicenseStatus(true);
            expect(getUpgradeReason('unknown')).toBeUndefined();
        });
    });

    describe('getUpgradeActionUrl', () => {
        it('returns action URL for upgrade reason', async () => {
            vi.mocked(LicenseAPI.getEntitlements).mockResolvedValue(mockProEntitlements);
            await loadLicenseStatus(true);
            expect(getUpgradeActionUrl('reason1')).toBe('/upgrade/reason1');
        });
    });

    describe('getFirstUpgradeActionUrl', () => {
        it('returns first upgrade reason URL', async () => {
            vi.mocked(LicenseAPI.getEntitlements).mockResolvedValue(mockProEntitlements);
            await loadLicenseStatus(true);
            expect(getFirstUpgradeActionUrl()).toBe('/upgrade/reason1');
        });
    });

    describe('getUpgradeActionUrlOrFallback', () => {
        it('returns pricing fallback when no upgrade reasons', async () => {
            vi.mocked(LicenseAPI.getEntitlements).mockResolvedValue(mockFreeEntitlements);
            await loadLicenseStatus(true);
            expect(getUpgradeActionUrlOrFallback('feature1')).toBe('/pricing?feature=feature1');
        });
    });

    describe('getLimit', () => {
        it('returns limit by key', async () => {
            vi.mocked(LicenseAPI.getEntitlements).mockResolvedValue(mockProEntitlements);
            await loadLicenseStatus(true);
            expect(getLimit('limit1')?.value).toBe(100);
        });
    });

    describe('isRangeLocked', () => {
        it('returns false for ranges within free limit', async () => {
            vi.mocked(LicenseAPI.getEntitlements).mockResolvedValue(mockFreeEntitlements);
            await loadLicenseStatus(true);
            expect(isRangeLocked('1h')).toBe(false);
            expect(isRangeLocked('7d')).toBe(false);
            expect(isRangeLocked('168h')).toBe(false); // exactly 7 days
        });

        it('returns true for ranges exceeding free limit', async () => {
            vi.mocked(LicenseAPI.getEntitlements).mockResolvedValue(mockFreeEntitlements);
            await loadLicenseStatus(true);
            expect(isRangeLocked('8d')).toBe(true); // 8 days
            expect(isRangeLocked('200h')).toBe(true); // 8.3 days
        });

        it('returns false for ranges with long_term_metrics feature', async () => {
            vi.mocked(LicenseAPI.getEntitlements).mockResolvedValue({
                ...mockProEntitlements,
                capabilities: ['long_term_metrics'],
            });
            await loadLicenseStatus(true);
            expect(isRangeLocked('8d')).toBe(false);
        });

        it('handles invalid range strings', async () => {
            vi.mocked(LicenseAPI.getEntitlements).mockResolvedValue(mockFreeEntitlements);
            await loadLicenseStatus(true);
            expect(isRangeLocked('invalid')).toBe(false);
        });
    });
});

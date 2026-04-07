import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  LicenseAPI,
  type LicenseCommercialEntitlements,
  type LicenseRuntimeCapabilities,
} from '@/api/license';
import { demoModeEnabled } from '@/stores/demoMode';
import { getPulseAccountPortalUpgradeUrl } from '@/utils/pricingHandoff';
import {
  getLimit,
  hasFeature,
  isHostedModeEnabled,
  isMultiTenantEnabled,
  isRangeLocked,
  licenseLoadError as runtimeLicenseLoadError,
  licenseLoaded as runtimeLicenseLoaded,
  loadLicenseStatus as loadRuntimeLicenseStatus,
  maxHistoryDays,
  runtimeCapabilities,
} from '@/stores/license';
import {
  entitlements,
  getFirstUpgradeActionUrl,
  getUpgradeActionUrl,
  getUpgradeActionUrlOrFallback,
  getUpgradeReason,
  hasMigrationGap,
  isPro,
  legacyConnections,
  licenseLoadError as commercialLicenseLoadError,
  loadLicenseStatus as loadCommercialLicenseStatus,
  startProTrial,
} from '@/stores/licenseCommercial';

vi.mock('@/api/license');
vi.mock('@/stores/demoMode', () => ({
  demoModeEnabled: vi.fn(() => false),
}));
vi.mock('@/stores/events', () => ({
  eventBus: { on: vi.fn() },
}));

describe('license stores', () => {
  const mockRuntimeCapabilities: LicenseRuntimeCapabilities = {
    capabilities: ['feature1', 'feature2', 'multi_tenant'],
    limits: [{ key: 'limit1', limit: 100, current: 25, state: 'ok' }],
    hosted_mode: false,
    max_history_days: 90,
  };

  const mockCommercialEntitlements: LicenseCommercialEntitlements = {
    tier: 'pro',
    subscription_state: 'active',
    capabilities: mockRuntimeCapabilities.capabilities,
    limits: mockRuntimeCapabilities.limits,
    upgrade_reasons: [
      { key: 'reason1', reason: 'Reason 1', action_url: '/upgrade/reason1' },
      { key: 'reason2', reason: 'Reason 2', action_url: '/upgrade/reason2' },
    ],
    hosted_mode: false,
    max_history_days: 90,
    legacy_connections: {
      proxmox_nodes: 2,
      docker_hosts: 1,
      kubernetes_clusters: 0,
    },
    has_migration_gap: true,
  };

  const mockFreeRuntimeCapabilities: LicenseRuntimeCapabilities = {
    capabilities: [],
    limits: [],
    hosted_mode: false,
    max_history_days: 7,
  };

  const mockFreeCommercialEntitlements: LicenseCommercialEntitlements = {
    tier: 'free',
    subscription_state: 'expired',
    capabilities: [],
    limits: [],
    upgrade_reasons: [],
    hosted_mode: false,
    max_history_days: 7,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(demoModeEnabled).mockReturnValue(false);
  });

  describe('runtime capability store', () => {
    it('loads runtime capabilities from API', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockRuntimeCapabilities);

      await loadRuntimeLicenseStatus();

      expect(LicenseAPI.getRuntimeCapabilities).toHaveBeenCalled();
      expect(runtimeCapabilities()).toEqual(mockRuntimeCapabilities);
      expect(runtimeLicenseLoaded()).toBe(true);
    });

    it('returns early if already loaded without force', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockRuntimeCapabilities);

      await loadRuntimeLicenseStatus(true);
      await loadRuntimeLicenseStatus();

      expect(LicenseAPI.getRuntimeCapabilities).toHaveBeenCalledTimes(1);
    });

    it('sets fallback runtime capabilities on error', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockRejectedValue(new Error('API error'));

      await loadRuntimeLicenseStatus(true);

      expect(runtimeCapabilities()).toEqual({
        capabilities: ['update_alerts', 'sso', 'ai_patrol'],
        limits: [],
        hosted_mode: false,
        max_history_days: 7,
      });
    });

    it('sets loadError on runtime API failure', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockRejectedValue(new Error('Network timeout'));

      await loadRuntimeLicenseStatus(true);

      expect(runtimeLicenseLoadError()).toBeInstanceOf(Error);
      expect(runtimeLicenseLoadError()?.message).toBe('Network timeout');
    });

    it('clears runtime loadError on successful load after failure', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockRejectedValue(new Error('Network timeout'));
      await loadRuntimeLicenseStatus(true);
      expect(runtimeLicenseLoadError()).toBeInstanceOf(Error);

      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockRuntimeCapabilities);
      await loadRuntimeLicenseStatus(true);
      expect(runtimeLicenseLoadError()).toBeNull();
    });

    it('returns true when feature is in capabilities', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockRuntimeCapabilities);
      await loadRuntimeLicenseStatus(true);
      expect(hasFeature('feature1')).toBe(true);
    });

    it('returns false when feature is not in capabilities', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockRuntimeCapabilities);
      await loadRuntimeLicenseStatus(true);
      expect(hasFeature('missing_feature')).toBe(false);
    });

    it('returns true when multi_tenant feature exists', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockRuntimeCapabilities);
      await loadRuntimeLicenseStatus(true);
      expect(isMultiTenantEnabled()).toBe(true);
    });

    it('returns false when multi_tenant feature does not exist', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockFreeRuntimeCapabilities);
      await loadRuntimeLicenseStatus(true);
      expect(isMultiTenantEnabled()).toBe(false);
    });

    it('returns true when hosted_mode is true', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue({
        ...mockRuntimeCapabilities,
        hosted_mode: true,
      });
      await loadRuntimeLicenseStatus(true);
      expect(isHostedModeEnabled()).toBe(true);
    });

    it('returns limit by key', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockRuntimeCapabilities);
      await loadRuntimeLicenseStatus(true);
      expect(getLimit('limit1')?.limit).toBe(100);
    });

    it('returns false for ranges within free limit (7d)', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue({
        ...mockFreeRuntimeCapabilities,
        max_history_days: 7,
      });
      await loadRuntimeLicenseStatus(true);
      expect(isRangeLocked('1h')).toBe(false);
      expect(isRangeLocked('7d')).toBe(false);
      expect(isRangeLocked('168h')).toBe(false);
    });

    it('returns true for ranges exceeding free limit (7d)', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue({
        ...mockFreeRuntimeCapabilities,
        max_history_days: 7,
      });
      await loadRuntimeLicenseStatus(true);
      expect(isRangeLocked('8d')).toBe(true);
      expect(isRangeLocked('200h')).toBe(true);
    });

    it('uses tier-specific max_history_days (relay = 14d)', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue({
        ...mockRuntimeCapabilities,
        max_history_days: 14,
      });
      await loadRuntimeLicenseStatus(true);
      expect(isRangeLocked('14d')).toBe(false);
      expect(isRangeLocked('15d')).toBe(true);
      expect(maxHistoryDays()).toBe(14);
    });

    it('uses tier-specific max_history_days (pro = 90d)', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockRuntimeCapabilities);
      await loadRuntimeLicenseStatus(true);
      expect(isRangeLocked('90d')).toBe(false);
      expect(isRangeLocked('91d')).toBe(true);
      expect(maxHistoryDays()).toBe(90);
    });

    it('defaults to 7 days when max_history_days is not set', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue({
        ...mockFreeRuntimeCapabilities,
        max_history_days: undefined,
      });
      await loadRuntimeLicenseStatus(true);
      expect(maxHistoryDays()).toBe(7);
      expect(isRangeLocked('8d')).toBe(true);
    });

    it('handles invalid range strings', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockFreeRuntimeCapabilities);
      await loadRuntimeLicenseStatus(true);
      expect(isRangeLocked('invalid')).toBe(false);
    });
  });

  describe('commercial entitlement store', () => {
    it('suppresses commercial entitlement reads in demo mode', async () => {
      vi.mocked(demoModeEnabled).mockReturnValue(true);

      await loadCommercialLicenseStatus(true);

      expect(LicenseAPI.getCommercialEntitlements).not.toHaveBeenCalled();
      expect(entitlements()).toEqual({
        capabilities: ['update_alerts', 'sso', 'ai_patrol'],
        limits: [],
        subscription_state: 'expired',
        upgrade_reasons: [],
        tier: 'free',
        hosted_mode: false,
        trial_eligible: false,
        legacy_connections: {
          proxmox_nodes: 0,
          docker_hosts: 0,
          kubernetes_clusters: 0,
        },
        has_migration_gap: false,
        commercial_migration: undefined,
      });
      expect(commercialLicenseLoadError()).toBeNull();
    });

    it('loads commercial entitlements from API', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue(mockCommercialEntitlements);

      await loadCommercialLicenseStatus(true);

      expect(LicenseAPI.getCommercialEntitlements).toHaveBeenCalled();
      expect(entitlements()).toEqual(mockCommercialEntitlements);
    });

    it('sets commercial fallback entitlements on error', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockRejectedValue(new Error('API error'));

      await loadCommercialLicenseStatus(true);

      expect(entitlements()).toEqual({
        capabilities: ['update_alerts', 'sso', 'ai_patrol'],
        limits: [],
        subscription_state: 'expired',
        upgrade_reasons: [],
        tier: 'free',
        hosted_mode: false,
        trial_eligible: false,
        legacy_connections: {
          proxmox_nodes: 0,
          docker_hosts: 0,
          kubernetes_clusters: 0,
        },
        has_migration_gap: false,
        commercial_migration: undefined,
      });
    });

    it('sets commercial loadError on API failure', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockRejectedValue(
        new Error('Network timeout'),
      );

      await loadCommercialLicenseStatus(true);

      expect(commercialLicenseLoadError()).toBeInstanceOf(Error);
      expect(commercialLicenseLoadError()?.message).toBe('Network timeout');
    });

    it('startProTrial throws error if trial start fails', async () => {
      vi.mocked(LicenseAPI.startTrial).mockResolvedValue({
        ok: false,
        status: 400,
        headers: new Headers(),
        json: vi.fn().mockResolvedValue({ code: 'trial_failed' }),
      } as unknown as Response);

      await expect(startProTrial()).rejects.toThrow('Failed to start trial');
    });

    it('startProTrial fails closed in demo mode', async () => {
      vi.mocked(demoModeEnabled).mockReturnValue(true);

      await expect(startProTrial()).rejects.toMatchObject({
        message: 'Trial activation unavailable in demo mode',
        status: 404,
        code: 'demo_mode_unavailable',
      });
      expect(LicenseAPI.startTrial).not.toHaveBeenCalled();
    });

    it('startProTrial preserves backend error details for trial-not-available responses', async () => {
      vi.mocked(LicenseAPI.startTrial).mockResolvedValue({
        ok: false,
        status: 409,
        headers: new Headers(),
        json: vi.fn().mockResolvedValue({
          code: 'trial_not_available',
          error: 'Trial cannot be started while a paid v5 license migration is pending',
          details: { org_id: 'default' },
        }),
      } as unknown as Response);

      await expect(startProTrial()).rejects.toMatchObject({
        status: 409,
        code: 'trial_not_available',
        message: 'Trial cannot be started while a paid v5 license migration is pending',
        details: { org_id: 'default' },
      });
    });

    it('startProTrial returns redirect action when hosted signup is required', async () => {
      vi.mocked(LicenseAPI.startTrial).mockResolvedValue({
        ok: false,
        status: 409,
        json: vi.fn().mockResolvedValue({
          code: 'trial_signup_required',
          details: { action_url: 'https://pulserelay.pro/pricing?intent=pro_trial' },
        }),
      } as unknown as Response);

      await expect(startProTrial()).resolves.toEqual({
        outcome: 'redirect',
        actionUrl: 'https://pulserelay.pro/pricing?intent=pro_trial',
      });
    });

    it('startProTrial refreshes commercial and runtime state when local activation succeeds', async () => {
      vi.mocked(LicenseAPI.startTrial).mockResolvedValue({ ok: true } as Response);
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue(mockCommercialEntitlements);
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockRuntimeCapabilities);

      await expect(startProTrial()).resolves.toEqual({ outcome: 'activated' });
      expect(LicenseAPI.getCommercialEntitlements).toHaveBeenCalled();
      expect(LicenseAPI.getRuntimeCapabilities).toHaveBeenCalled();
    });

    it('returns true for pro tier', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue(mockCommercialEntitlements);
      await loadCommercialLicenseStatus(true);
      expect(isPro()).toBe(true);
    });

    it('returns true for pro_plus tier', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue({
        ...mockCommercialEntitlements,
        tier: 'pro_plus',
      });
      await loadCommercialLicenseStatus(true);
      expect(isPro()).toBe(true);
    });

    it('returns false for relay tier (paid but not Pro)', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue({
        ...mockCommercialEntitlements,
        tier: 'relay',
      });
      await loadCommercialLicenseStatus(true);
      expect(isPro()).toBe(false);
    });

    it('returns false for free tier', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue(
        mockFreeCommercialEntitlements,
      );
      await loadCommercialLicenseStatus(true);
      expect(isPro()).toBe(false);
    });

    it('returns upgrade reason by key', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue(mockCommercialEntitlements);
      await loadCommercialLicenseStatus(true);
      expect(getUpgradeReason('reason2')?.key).toBe('reason2');
    });

    it('returns undefined for unknown key', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue(mockCommercialEntitlements);
      await loadCommercialLicenseStatus(true);
      expect(getUpgradeReason('unknown')).toBeUndefined();
    });

    it('returns legacy connection counts from entitlements', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue(mockCommercialEntitlements);
      await loadCommercialLicenseStatus(true);

      expect(legacyConnections()).toEqual({
        proxmox_nodes: 2,
        docker_hosts: 1,
        kubernetes_clusters: 0,
      });
      expect(hasMigrationGap()).toBe(true);
    });

    it('falls back to zero legacy counts when absent', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue({
        ...mockFreeCommercialEntitlements,
        legacy_connections: undefined,
        has_migration_gap: undefined,
      });
      await loadCommercialLicenseStatus(true);

      expect(legacyConnections()).toEqual({
        proxmox_nodes: 0,
        docker_hosts: 0,
        kubernetes_clusters: 0,
      });
      expect(hasMigrationGap()).toBe(false);
    });

    it('returns action URL for upgrade reason', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue(mockCommercialEntitlements);
      await loadCommercialLicenseStatus(true);
      expect(getUpgradeActionUrl('reason1')).toBe('/upgrade/reason1');
    });

    it('returns first upgrade reason URL', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue(mockCommercialEntitlements);
      await loadCommercialLicenseStatus(true);
      expect(getFirstUpgradeActionUrl()).toBe('/upgrade/reason1');
    });

    it('routes missing self-hosted upgrades to Pulse Account when no upgrade reasons exist', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue(
        mockFreeCommercialEntitlements,
      );
      await loadCommercialLicenseStatus(true);
      expect(getUpgradeActionUrlOrFallback('relay')).toBe(getPulseAccountPortalUpgradeUrl('relay'));
    });

    it('routes monitored-system limit fallbacks to the billing upgrade arrival', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue({
        ...mockCommercialEntitlements,
        upgrade_reasons: [{ key: 'reason1', reason: 'Reason 1', action_url: '/upgrade/reason1' }],
      });
      await loadCommercialLicenseStatus(true);
      expect(getUpgradeActionUrlOrFallback('max_monitored_systems')).toBe(
        '/settings/system/billing/plan?intent=max_monitored_systems',
      );
    });

    it('routes cloud fallbacks to the in-product cloud plans page', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue(
        mockFreeCommercialEntitlements,
      );
      await loadCommercialLicenseStatus(true);
      expect(getUpgradeActionUrlOrFallback('cloud')).toBe('/cloud');
    });

    it('routes generic upgrade fallbacks to Pulse Account', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue(
        mockFreeCommercialEntitlements,
      );
      await loadCommercialLicenseStatus(true);
      expect(getUpgradeActionUrlOrFallback('upgrade')).toBe(
        getPulseAccountPortalUpgradeUrl('upgrade'),
      );
    });

    it('prefers a specific monitored-system upgrade action when provided', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue({
        ...mockCommercialEntitlements,
        upgrade_reasons: [
          {
            key: 'max_monitored_systems',
            reason: 'Expand monitored-system capacity',
            action_url: '/upgrade/max-monitored-systems',
          },
        ],
      });
      await loadCommercialLicenseStatus(true);
      expect(getUpgradeActionUrlOrFallback('max_monitored_systems')).toBe(
        '/upgrade/max-monitored-systems',
      );
    });

    it('routes unknown keys to Pulse Account as a last resort', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue(
        mockFreeCommercialEntitlements,
      );
      await loadCommercialLicenseStatus(true);
      expect(getUpgradeActionUrlOrFallback('feature1')).toBe(
        getPulseAccountPortalUpgradeUrl('feature1'),
      );
    });
  });
});

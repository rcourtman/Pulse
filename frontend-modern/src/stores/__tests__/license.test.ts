import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  LicenseAPI,
  type LicenseCommercialEntitlements,
  type LicenseCommercialPosture,
  type LicenseRuntimeCapabilities,
} from '@/api/license';
import {
  presentationPolicyHidesCommercialSurfaces,
  sessionPresentationPolicyResolved,
} from '@/stores/sessionPresentationPolicy';
import { getUpgradeFallbackDestination } from '@/utils/pricingHandoff';
import {
  getRuntimeLimit,
  hasFeature,
  isHostedModeEnabled,
  isMultiTenantEnabled,
  isRangeLocked,
  runtimeCapabilitiesLoadError as runtimeLicenseLoadError,
  runtimeCapabilitiesLoaded as runtimeLicenseLoaded,
  loadRuntimeCapabilities,
  maxHistoryDays,
  runtimeCapabilities,
} from '@/stores/license';
import {
  commercialPosture,
  getFirstUpgradeActionUrl,
  getUpgradeActionUrl,
  getUpgradeActionUrlOrFallback,
  getUpgradeReason,
  hasMigrationGap,
  isPro,
  legacyConnections,
  commercialPostureLoadError,
  loadCommercialPosture,
  startProTrial,
} from '@/stores/licenseCommercial';
import {
  licenseEntitlements,
  licenseEntitlementsLoadError,
  loadLicenseEntitlements,
} from '@/stores/licenseEntitlements';

vi.mock('@/api/license');
vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesCommercialSurfaces: vi.fn(() => false),
  sessionPresentationPolicyResolved: vi.fn(() => true),
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

  const mockCommercialPosture: LicenseCommercialPosture = {
    tier: 'pro',
    subscription_state: 'active',
    upgrade_reasons: [
      { key: 'reason1', reason: 'Reason 1', action_url: '/upgrade/reason1' },
      { key: 'reason2', reason: 'Reason 2', action_url: '/upgrade/reason2' },
    ],
    legacy_connections: {
      proxmox_nodes: 2,
      docker_hosts: 1,
      kubernetes_clusters: 0,
    },
    has_migration_gap: true,
  };
  const mockCommercialEntitlements: LicenseCommercialEntitlements = {
    ...mockCommercialPosture,
    capabilities: mockRuntimeCapabilities.capabilities,
    limits: mockRuntimeCapabilities.limits,
    hosted_mode: false,
    max_history_days: 90,
  };

  const mockFreeRuntimeCapabilities: LicenseRuntimeCapabilities = {
    capabilities: [],
    limits: [],
    hosted_mode: false,
    max_history_days: 7,
  };

  const mockFreeCommercialPosture: LicenseCommercialPosture = {
    tier: 'free',
    subscription_state: 'expired',
    upgrade_reasons: [],
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(presentationPolicyHidesCommercialSurfaces).mockReturnValue(false);
    vi.mocked(sessionPresentationPolicyResolved).mockReturnValue(true);
  });

  describe('runtime capability store', () => {
    it('loads runtime capabilities from API', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockRuntimeCapabilities);

      await loadRuntimeCapabilities();

      expect(LicenseAPI.getRuntimeCapabilities).toHaveBeenCalled();
      expect(runtimeCapabilities()).toEqual(mockRuntimeCapabilities);
      expect(runtimeLicenseLoaded()).toBe(true);
    });

    it('returns early if already loaded without force', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockRuntimeCapabilities);

      await loadRuntimeCapabilities(true);
      await loadRuntimeCapabilities();

      expect(LicenseAPI.getRuntimeCapabilities).toHaveBeenCalledTimes(1);
    });

    it('sets fallback runtime capabilities on error', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockRejectedValue(new Error('API error'));

      await loadRuntimeCapabilities(true);

      expect(runtimeCapabilities()).toEqual({
        capabilities: ['update_alerts', 'sso', 'ai_patrol'],
        limits: [],
        hosted_mode: false,
        max_history_days: 7,
      });
    });

    it('sets loadError on runtime API failure', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockRejectedValue(new Error('Network timeout'));

      await loadRuntimeCapabilities(true);

      expect(runtimeLicenseLoadError()).toBeInstanceOf(Error);
      expect(runtimeLicenseLoadError()?.message).toBe('Network timeout');
    });

    it('clears runtime loadError on successful load after failure', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockRejectedValue(new Error('Network timeout'));
      await loadRuntimeCapabilities(true);
      expect(runtimeLicenseLoadError()).toBeInstanceOf(Error);

      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockRuntimeCapabilities);
      await loadRuntimeCapabilities(true);
      expect(runtimeLicenseLoadError()).toBeNull();
    });

    it('returns true when feature is in capabilities', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockRuntimeCapabilities);
      await loadRuntimeCapabilities(true);
      expect(hasFeature('feature1')).toBe(true);
    });

    it('returns false when feature is not in capabilities', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockRuntimeCapabilities);
      await loadRuntimeCapabilities(true);
      expect(hasFeature('missing_feature')).toBe(false);
    });

    it('returns true when multi_tenant feature exists', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockRuntimeCapabilities);
      await loadRuntimeCapabilities(true);
      expect(isMultiTenantEnabled()).toBe(true);
    });

    it('returns false when multi_tenant feature does not exist', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockFreeRuntimeCapabilities);
      await loadRuntimeCapabilities(true);
      expect(isMultiTenantEnabled()).toBe(false);
    });

    it('returns true when hosted_mode is true', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue({
        ...mockRuntimeCapabilities,
        hosted_mode: true,
      });
      await loadRuntimeCapabilities(true);
      expect(isHostedModeEnabled()).toBe(true);
    });

    it('returns limit by key', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockRuntimeCapabilities);
      await loadRuntimeCapabilities(true);
      expect(getRuntimeLimit('limit1')?.limit).toBe(100);
    });

    it('returns false for ranges within free limit (7d)', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue({
        ...mockFreeRuntimeCapabilities,
        max_history_days: 7,
      });
      await loadRuntimeCapabilities(true);
      expect(isRangeLocked('1h')).toBe(false);
      expect(isRangeLocked('7d')).toBe(false);
      expect(isRangeLocked('168h')).toBe(false);
    });

    it('returns true for ranges exceeding free limit (7d)', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue({
        ...mockFreeRuntimeCapabilities,
        max_history_days: 7,
      });
      await loadRuntimeCapabilities(true);
      expect(isRangeLocked('8d')).toBe(true);
      expect(isRangeLocked('200h')).toBe(true);
    });

    it('uses tier-specific max_history_days (relay = 14d)', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue({
        ...mockRuntimeCapabilities,
        max_history_days: 14,
      });
      await loadRuntimeCapabilities(true);
      expect(isRangeLocked('14d')).toBe(false);
      expect(isRangeLocked('15d')).toBe(true);
      expect(maxHistoryDays()).toBe(14);
    });

    it('uses tier-specific max_history_days (pro = 90d)', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockRuntimeCapabilities);
      await loadRuntimeCapabilities(true);
      expect(isRangeLocked('90d')).toBe(false);
      expect(isRangeLocked('91d')).toBe(true);
      expect(maxHistoryDays()).toBe(90);
    });

    it('defaults to 7 days when max_history_days is not set', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue({
        ...mockFreeRuntimeCapabilities,
        max_history_days: undefined,
      });
      await loadRuntimeCapabilities(true);
      expect(maxHistoryDays()).toBe(7);
      expect(isRangeLocked('8d')).toBe(true);
    });

    it('handles invalid range strings', async () => {
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockFreeRuntimeCapabilities);
      await loadRuntimeCapabilities(true);
      expect(isRangeLocked('invalid')).toBe(false);
    });
  });

  describe('commercial posture store', () => {
    it('defers commercial posture reads until presentation policy resolves', async () => {
      vi.mocked(sessionPresentationPolicyResolved).mockReturnValue(false);

      await loadCommercialPosture(true);

      expect(LicenseAPI.getCommercialPosture).not.toHaveBeenCalled();
      expect(commercialPosture()).toBeNull();
    });

    it('suppresses commercial posture reads when commercial surfaces are hidden', async () => {
      vi.mocked(presentationPolicyHidesCommercialSurfaces).mockReturnValue(true);

      await loadCommercialPosture(true);

      expect(LicenseAPI.getCommercialPosture).not.toHaveBeenCalled();
      expect(commercialPosture()).toEqual({
        subscription_state: 'expired',
        upgrade_reasons: [],
        tier: 'free',
        trial_eligible: false,
        legacy_connections: {
          proxmox_nodes: 0,
          docker_hosts: 0,
          kubernetes_clusters: 0,
        },
        has_migration_gap: false,
        commercial_migration: undefined,
      });
      expect(commercialPostureLoadError()).toBeNull();
    });

    it('loads commercial posture from API', async () => {
      vi.mocked(LicenseAPI.getCommercialPosture).mockResolvedValue(mockCommercialPosture);

      await loadCommercialPosture(true);

      expect(LicenseAPI.getCommercialPosture).toHaveBeenCalled();
      expect(commercialPosture()).toEqual(mockCommercialPosture);
    });

    it('deduplicates in-flight commercial posture loads', async () => {
      let resolveLoad!: (value: typeof mockCommercialPosture) => void;
      vi.mocked(LicenseAPI.getCommercialPosture).mockImplementation(
        () =>
          new Promise<typeof mockCommercialPosture>((resolve) => {
            resolveLoad = resolve;
          }),
      );

      const firstLoad = loadCommercialPosture(true);
      const secondLoad = loadCommercialPosture();

      expect(LicenseAPI.getCommercialPosture).toHaveBeenCalledTimes(1);
      resolveLoad(mockCommercialPosture);
      await Promise.all([firstLoad, secondLoad]);

      expect(commercialPosture()).toEqual(mockCommercialPosture);
    });

    it('sets commercial fallback posture on error', async () => {
      vi.mocked(LicenseAPI.getCommercialPosture).mockRejectedValue(new Error('API error'));

      await loadCommercialPosture(true);

      expect(commercialPosture()).toEqual({
        subscription_state: 'expired',
        upgrade_reasons: [],
        tier: 'free',
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
      vi.mocked(LicenseAPI.getCommercialPosture).mockRejectedValue(new Error('Network timeout'));

      await loadCommercialPosture(true);

      expect(commercialPostureLoadError()).toBeInstanceOf(Error);
      expect(commercialPostureLoadError()?.message).toBe('Network timeout');
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

    it('startProTrial fails closed when commercial surfaces are hidden', async () => {
      vi.mocked(presentationPolicyHidesCommercialSurfaces).mockReturnValue(true);

      await expect(startProTrial()).rejects.toMatchObject({
        message: 'Trial activation unavailable under the current presentation policy',
        status: 404,
        code: 'presentation_policy_unavailable',
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
      vi.mocked(LicenseAPI.getCommercialPosture).mockResolvedValue(mockCommercialPosture);
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue(mockCommercialEntitlements);
      vi.mocked(LicenseAPI.getRuntimeCapabilities).mockResolvedValue(mockRuntimeCapabilities);

      await expect(startProTrial()).resolves.toEqual({ outcome: 'activated' });
      expect(LicenseAPI.getCommercialPosture).toHaveBeenCalled();
      expect(LicenseAPI.getCommercialEntitlements).toHaveBeenCalled();
      expect(LicenseAPI.getRuntimeCapabilities).toHaveBeenCalled();
    });

    it('returns true for pro tier', async () => {
      vi.mocked(LicenseAPI.getCommercialPosture).mockResolvedValue(mockCommercialPosture);
      await loadCommercialPosture(true);
      expect(isPro()).toBe(true);
    });

    it('returns true for pro_plus tier', async () => {
      vi.mocked(LicenseAPI.getCommercialPosture).mockResolvedValue({
        ...mockCommercialPosture,
        tier: 'pro_plus',
      });
      await loadCommercialPosture(true);
      expect(isPro()).toBe(true);
    });

    it('returns false for relay tier (paid but not Pro)', async () => {
      vi.mocked(LicenseAPI.getCommercialPosture).mockResolvedValue({
        ...mockCommercialPosture,
        tier: 'relay',
      });
      await loadCommercialPosture(true);
      expect(isPro()).toBe(false);
    });

    it('returns false for free tier', async () => {
      vi.mocked(LicenseAPI.getCommercialPosture).mockResolvedValue(mockFreeCommercialPosture);
      await loadCommercialPosture(true);
      expect(isPro()).toBe(false);
    });

    it('returns upgrade reason by key', async () => {
      vi.mocked(LicenseAPI.getCommercialPosture).mockResolvedValue(mockCommercialPosture);
      await loadCommercialPosture(true);
      expect(getUpgradeReason('reason2')?.key).toBe('reason2');
    });

    it('returns undefined for unknown key', async () => {
      vi.mocked(LicenseAPI.getCommercialPosture).mockResolvedValue(mockCommercialPosture);
      await loadCommercialPosture(true);
      expect(getUpgradeReason('unknown')).toBeUndefined();
    });

    it('returns legacy connection counts from posture', async () => {
      vi.mocked(LicenseAPI.getCommercialPosture).mockResolvedValue(mockCommercialPosture);
      await loadCommercialPosture(true);

      expect(legacyConnections()).toEqual({
        proxmox_nodes: 2,
        docker_hosts: 1,
        kubernetes_clusters: 0,
      });
      expect(hasMigrationGap()).toBe(true);
    });

    it('falls back to zero legacy counts when absent', async () => {
      vi.mocked(LicenseAPI.getCommercialPosture).mockResolvedValue({
        ...mockFreeCommercialPosture,
        legacy_connections: undefined,
        has_migration_gap: undefined,
      });
      await loadCommercialPosture(true);

      expect(legacyConnections()).toEqual({
        proxmox_nodes: 0,
        docker_hosts: 0,
        kubernetes_clusters: 0,
      });
      expect(hasMigrationGap()).toBe(false);
    });

    it('returns action URL for upgrade reason', async () => {
      vi.mocked(LicenseAPI.getCommercialPosture).mockResolvedValue(mockCommercialPosture);
      await loadCommercialPosture(true);
      expect(getUpgradeActionUrl('reason1')).toBe('/upgrade/reason1');
    });

    it('returns first upgrade reason URL', async () => {
      vi.mocked(LicenseAPI.getCommercialPosture).mockResolvedValue(mockCommercialPosture);
      await loadCommercialPosture(true);
      expect(getFirstUpgradeActionUrl()).toBe('/upgrade/reason1');
    });

    it('routes missing self-hosted upgrades to Pulse Account when no upgrade reasons exist', async () => {
      vi.mocked(LicenseAPI.getCommercialPosture).mockResolvedValue(mockFreeCommercialPosture);
      await loadCommercialPosture(true);
      expect(getUpgradeActionUrlOrFallback('relay')).toBe(getUpgradeFallbackDestination('relay'));
    });

    it('routes monitored-system limit fallbacks to the billing upgrade arrival', async () => {
      vi.mocked(LicenseAPI.getCommercialPosture).mockResolvedValue({
        ...mockCommercialPosture,
        upgrade_reasons: [{ key: 'reason1', reason: 'Reason 1', action_url: '/upgrade/reason1' }],
      });
      await loadCommercialPosture(true);
      expect(getUpgradeActionUrlOrFallback('max_monitored_systems')).toBe(
        '/settings/system/billing/plan?intent=max_monitored_systems',
      );
    });

    it('routes cloud fallbacks to the in-product cloud plans page', async () => {
      vi.mocked(LicenseAPI.getCommercialPosture).mockResolvedValue(mockFreeCommercialPosture);
      await loadCommercialPosture(true);
      expect(getUpgradeActionUrlOrFallback('cloud')).toBe('/cloud');
    });

    it('routes generic upgrade fallbacks to Pulse Account', async () => {
      vi.mocked(LicenseAPI.getCommercialPosture).mockResolvedValue(mockFreeCommercialPosture);
      await loadCommercialPosture(true);
      expect(getUpgradeActionUrlOrFallback('upgrade')).toBe(getUpgradeFallbackDestination('upgrade'));
    });

    it('prefers a specific monitored-system upgrade action when provided', async () => {
      vi.mocked(LicenseAPI.getCommercialPosture).mockResolvedValue({
        ...mockCommercialPosture,
        upgrade_reasons: [
          {
            key: 'max_monitored_systems',
            reason: 'Expand monitored-system capacity',
            action_url: '/upgrade/max-monitored-systems',
          },
        ],
      });
      await loadCommercialPosture(true);
      expect(getUpgradeActionUrlOrFallback('max_monitored_systems')).toBe(
        getUpgradeFallbackDestination('max_monitored_systems'),
      );
    });

    it('routes unknown keys to Pulse Account as a last resort', async () => {
      vi.mocked(LicenseAPI.getCommercialPosture).mockResolvedValue(mockFreeCommercialPosture);
      await loadCommercialPosture(true);
      expect(getUpgradeActionUrlOrFallback('feature1')).toBe(
        getUpgradeFallbackDestination('feature1'),
      );
    });
  });

  describe('billing entitlements store', () => {
    it('defers full entitlements reads until presentation policy resolves', async () => {
      vi.mocked(sessionPresentationPolicyResolved).mockReturnValue(false);
      const previous = licenseEntitlements();

      await loadLicenseEntitlements(true);

      expect(LicenseAPI.getCommercialEntitlements).not.toHaveBeenCalled();
      expect(licenseEntitlements()).toBe(previous);
    });

    it('suppresses full entitlements reads when commercial surfaces are hidden', async () => {
      vi.mocked(presentationPolicyHidesCommercialSurfaces).mockReturnValue(true);

      await loadLicenseEntitlements(true);

      expect(LicenseAPI.getCommercialEntitlements).not.toHaveBeenCalled();
      expect(licenseEntitlements()).toEqual({
        capabilities: ['update_alerts', 'sso', 'ai_patrol'],
        limits: [],
        subscription_state: 'expired',
        upgrade_reasons: [],
        tier: 'free',
        trial_eligible: false,
        legacy_connections: {
          proxmox_nodes: 0,
          docker_hosts: 0,
          kubernetes_clusters: 0,
        },
        has_migration_gap: false,
        commercial_migration: undefined,
      });
      expect(licenseEntitlementsLoadError()).toBeNull();
    });

    it('loads full billing entitlements from API', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockResolvedValue(mockCommercialEntitlements);

      await loadLicenseEntitlements(true);

      expect(LicenseAPI.getCommercialEntitlements).toHaveBeenCalled();
      expect(licenseEntitlements()).toEqual(mockCommercialEntitlements);
    });

    it('sets billing loadError on API failure', async () => {
      vi.mocked(LicenseAPI.getCommercialEntitlements).mockRejectedValue(
        new Error('billing unavailable'),
      );

      await loadLicenseEntitlements(true);

      expect(licenseEntitlementsLoadError()).toBeInstanceOf(Error);
      expect(licenseEntitlementsLoadError()?.message).toBe('billing unavailable');
    });
  });
});

import { createEffect, createMemo, onMount } from 'solid-js';
import { presentationPolicyHidesCommercialSurfaces } from '@/stores/sessionPresentationPolicy';
import {
  getRuntimeMonitoredSystemCapacity,
  getRuntimeLimit,
  loadRuntimeCapabilities,
} from '@/stores/license';
import {
  getUpgradeActionDestination,
  hasMigrationGap,
} from '@/stores/licenseCommercial';
import { resolveUpgradeDestination } from '@/utils/upgradeNavigation';
import {
  scopeSelfHostedBillingDestination,
  SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
} from '@/utils/pricingHandoff';
import {
  trackUpgradeClicked,
  trackUpgradeMetricEvent,
  UPGRADE_METRIC_EVENTS,
} from '@/utils/upgradeMetrics';
import {
  getMonitoredSystemBannerToneClass,
  getMonitoredSystemSummary,
  isMonitoredSystemLimitUrgent,
  MONITORED_SYSTEM_LIMIT_INSTALL_COLLECTORS_HREF,
  MONITORED_SYSTEM_LIMIT_KEY,
  MONITORED_SYSTEM_LIMIT_VIEW_CAPACITY_HREF,
  shouldShowMonitoredSystemLimitBanner,
} from './monitoredSystemLimitWarningBannerModel';

export function useMonitoredSystemLimitWarningBannerState() {
  onMount(() => {
    void loadRuntimeCapabilities();
  });

  const monitoredSystemLimit = createMemo(() => getRuntimeLimit(MONITORED_SYSTEM_LIMIT_KEY));
  const monitoredSystemCapacity = createMemo(() => getRuntimeMonitoredSystemCapacity());
  const isUrgent = createMemo(() =>
    isMonitoredSystemLimitUrgent(monitoredSystemLimit(), monitoredSystemCapacity()),
  );
  const showBanner = createMemo(
    () =>
      !presentationPolicyHidesCommercialSurfaces() &&
      shouldShowMonitoredSystemLimitBanner(monitoredSystemLimit(), monitoredSystemCapacity()),
  );
  const migrationGap = createMemo(() => hasMigrationGap());
  const monitoredSystemSummary = createMemo(() =>
    getMonitoredSystemSummary(monitoredSystemLimit(), monitoredSystemCapacity()),
  );
  const toneClass = createMemo(() => getMonitoredSystemBannerToneClass(isUrgent()));
  const suppressUpgrade = createMemo(
    () =>
      monitoredSystemCapacity()?.reason?.trim().toLowerCase() ===
      'legacy_migration_capture_pending',
  );
  const viewCapacityDestination = createMemo(() =>
    resolveUpgradeDestination(MONITORED_SYSTEM_LIMIT_VIEW_CAPACITY_HREF),
  );
  const installCollectorsDestination = createMemo(() =>
    resolveUpgradeDestination(MONITORED_SYSTEM_LIMIT_INSTALL_COLLECTORS_HREF),
  );
  const upgradeDestination = createMemo(() =>
    suppressUpgrade()
      ? null
      : scopeSelfHostedBillingDestination(
          getUpgradeActionDestination(MONITORED_SYSTEM_LIMIT_KEY),
          'plan',
          {
            intent: SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
          },
        ),
  );

  let wasUrgent = false;
  createEffect(() => {
    const urgent = isUrgent();
    const visible = showBanner();
    const limit = monitoredSystemLimit();
    if (visible && urgent && !wasUrgent && limit) {
      trackUpgradeMetricEvent({
        type: UPGRADE_METRIC_EVENTS.LIMIT_WARNING_SHOWN,
        surface: 'monitored_system_limit_banner',
        limit_key: MONITORED_SYSTEM_LIMIT_KEY,
        current_value: limit.current,
        limit_value: limit.limit,
      });
    }
    wasUrgent = visible && urgent;
  });

  const handleInstallCollectorsClick = () => {
    trackUpgradeClicked(
      'monitored_system_limit_banner_install_v6_collectors',
      MONITORED_SYSTEM_LIMIT_KEY,
    );
  };

  const handleUpgradeClick = () => {
    trackUpgradeClicked('monitored_system_limit_banner_upgrade', MONITORED_SYSTEM_LIMIT_KEY);
  };

  return {
    handleInstallCollectorsClick,
    handleUpgradeClick,
    installCollectorsDestination,
    isUrgent,
    viewCapacityDestination,
    migrationGap,
    monitoredSystemSummary,
    showBanner,
    toneClass,
    upgradeDestination,
  };
}

import { createEffect, createMemo, onMount } from 'solid-js';
import { demoModeEnabled } from '@/stores/demoMode';
import {
  entitlements,
  getLimit,
  getUpgradeActionDestination,
  hasMigrationGap,
  legacyConnections,
  loadLicenseStatus,
} from '@/stores/license';
import { resolveUpgradeDestination } from '@/utils/upgradeNavigation';
import {
  scopeSelfHostedBillingDestination,
  SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT,
} from '@/utils/pricingHandoff';
import {
  trackUpgradeClicked,
  trackUpgradeMetricEvent,
  UPGRADE_METRIC_EVENTS,
} from '@/utils/upgradeMetrics';
import {
  getMonitoredSystemBannerToneClass,
  getMonitoredSystemMigrationMessage,
  getMonitoredSystemMigrationTextClass,
  getMonitoredSystemOverflowSummary,
  getMonitoredSystemSummary,
  isMonitoredSystemLimitUrgent,
  MONITORED_SYSTEM_LIMIT_INSTALL_COLLECTORS_HREF,
  MONITORED_SYSTEM_LIMIT_KEY,
  MONITORED_SYSTEM_LIMIT_LEARN_MORE_HREF,
  shouldShowMonitoredSystemLimitBanner,
} from './monitoredSystemLimitWarningBannerModel';

export function useMonitoredSystemLimitWarningBannerState() {
  onMount(() => {
    void loadLicenseStatus();
  });

  const monitoredSystemLimit = createMemo(() => getLimit(MONITORED_SYSTEM_LIMIT_KEY));
  const isUrgent = createMemo(() => isMonitoredSystemLimitUrgent(monitoredSystemLimit()));
  const showBanner = createMemo(
    () => !demoModeEnabled() && shouldShowMonitoredSystemLimitBanner(monitoredSystemLimit()),
  );
  const migrationGap = createMemo(() => hasMigrationGap());
  const migrationCounts = createMemo(() => legacyConnections());
  const monitoredSystemSummary = createMemo(() =>
    getMonitoredSystemSummary(monitoredSystemLimit()),
  );
  const migrationMessage = createMemo(() =>
    getMonitoredSystemMigrationMessage(migrationCounts()),
  );
  const overflowSummary = createMemo(() =>
    getMonitoredSystemOverflowSummary(entitlements()?.overflow_days_remaining),
  );
  const toneClass = createMemo(() => getMonitoredSystemBannerToneClass(isUrgent()));
  const migrationTextClass = createMemo(() =>
    getMonitoredSystemMigrationTextClass(isUrgent()),
  );
  const learnMoreDestination = createMemo(() =>
    resolveUpgradeDestination(MONITORED_SYSTEM_LIMIT_LEARN_MORE_HREF),
  );
  const installCollectorsDestination = createMemo(() =>
    resolveUpgradeDestination(MONITORED_SYSTEM_LIMIT_INSTALL_COLLECTORS_HREF),
  );
  const upgradeDestination = createMemo(() =>
    scopeSelfHostedBillingDestination(
      getUpgradeActionDestination(MONITORED_SYSTEM_LIMIT_KEY),
      'plan',
      {
        intent: SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT,
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
    learnMoreDestination,
    migrationGap,
    migrationMessage,
    migrationTextClass,
    monitoredSystemSummary,
    overflowSummary,
    showBanner,
    toneClass,
    upgradeDestination,
  };
}

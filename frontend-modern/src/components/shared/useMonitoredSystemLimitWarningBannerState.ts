import { createEffect, createMemo, onMount } from 'solid-js';
import {
  entitlements,
  getLimit,
  getUpgradeActionDestination,
  hasMigrationGap,
  legacyConnections,
  loadLicenseStatus,
} from '@/stores/license';
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
  MONITORED_SYSTEM_LIMIT_KEY,
  shouldShowMonitoredSystemLimitBanner,
} from './monitoredSystemLimitWarningBannerModel';

export function useMonitoredSystemLimitWarningBannerState() {
  onMount(() => {
    void loadLicenseStatus();
  });

  const monitoredSystemLimit = createMemo(() => getLimit(MONITORED_SYSTEM_LIMIT_KEY));
  const isUrgent = createMemo(() => isMonitoredSystemLimitUrgent(monitoredSystemLimit()));
  const showBanner = createMemo(() => shouldShowMonitoredSystemLimitBanner(monitoredSystemLimit()));
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
  const upgradeDestination = createMemo(() =>
    getUpgradeActionDestination(MONITORED_SYSTEM_LIMIT_KEY),
  );

  let wasUrgent = false;
  createEffect(() => {
    const urgent = isUrgent();
    const limit = monitoredSystemLimit();
    if (urgent && !wasUrgent && limit) {
      trackUpgradeMetricEvent({
        type: UPGRADE_METRIC_EVENTS.LIMIT_WARNING_SHOWN,
        surface: 'monitored_system_limit_banner',
        limit_key: MONITORED_SYSTEM_LIMIT_KEY,
        current_value: limit.current,
        limit_value: limit.limit,
      });
    }
    wasUrgent = urgent;
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
    isUrgent,
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

import { createEffect, createMemo, onMount } from 'solid-js';
import {
  presentationPolicyHidesCommercialSurfaces,
  presentationPolicyHidesUpgradePrompts,
} from '@/stores/sessionPresentationPolicy';
import {
  getRuntimeMonitoredSystemCapacity,
  getRuntimeLimit,
  isHostedModeEnabled,
  loadRuntimeCapabilities,
} from '@/stores/license';
import { hasMigrationGap } from '@/stores/licenseCommercial';
import { resolveUpgradeDestination } from '@/utils/upgradeNavigation';
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
  MONITORED_SYSTEM_LIMIT_REVIEW_POLICY_HREF,
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
      isHostedModeEnabled() &&
      !presentationPolicyHidesCommercialSurfaces() &&
      !presentationPolicyHidesUpgradePrompts() &&
      shouldShowMonitoredSystemLimitBanner(monitoredSystemLimit(), monitoredSystemCapacity()),
  );
  const migrationGap = createMemo(() => hasMigrationGap());
  const monitoredSystemSummary = createMemo(() =>
    getMonitoredSystemSummary(monitoredSystemLimit(), monitoredSystemCapacity()),
  );
  const toneClass = createMemo(() => getMonitoredSystemBannerToneClass(isUrgent()));
  const reviewPolicyDestination = createMemo(() =>
    resolveUpgradeDestination(MONITORED_SYSTEM_LIMIT_REVIEW_POLICY_HREF),
  );
  const installCollectorsDestination = createMemo(() =>
    resolveUpgradeDestination(MONITORED_SYSTEM_LIMIT_INSTALL_COLLECTORS_HREF),
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

  return {
    handleInstallCollectorsClick,
    installCollectorsDestination,
    isUrgent,
    reviewPolicyDestination,
    migrationGap,
    monitoredSystemSummary,
    showBanner,
    toneClass,
  };
}

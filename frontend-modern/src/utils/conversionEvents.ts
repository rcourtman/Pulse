// Compatibility wrapper: migrate callers to "@/utils/upgradeMetrics".
// Keep these exports stable for older modules/tests.

export type { UpgradeMetricEvent as ConversionEvent } from './upgradeMetrics';
export {
  UPGRADE_METRIC_EVENTS as CONVERSION_EVENTS,
  trackUpgradeMetricEvent as trackConversionEvent,
  trackPaywallViewed,
  trackUpgradeClicked,
  trackAgentInstallTokenGenerated,
  trackAgentInstallCommandCopied,
  trackAgentInstallProfileSelected,
  trackAgentFirstConnected,
} from './upgradeMetrics';

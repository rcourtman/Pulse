// Compatibility wrapper for retired customer-side commercial analytics exports.
// Keep these stable while older modules/tests migrate away from the names.

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

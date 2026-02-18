// Backwards-compat shim. The canonical hook is useDashboardRecovery.
export {
  type DashboardRecoverySummary as DashboardBackupSummary,
  useDashboardRecovery as useDashboardBackups,
} from './useDashboardRecovery';


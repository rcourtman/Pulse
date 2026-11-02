import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import {
  describeStorageAlertHeadline,
  getStorageAlertForecastDaysToFull,
} from '@/features/storageBackups/storageRowAlertPresentation';

// "How long until it fills" for one PBS datastore. Pulse already forecasts it:
// the PBS poller evaluates each datastore as storage `pbs-<connection>-<store>`
// (alias `<instance id>/<store>`, monitor_pbs_pmg.go), and its capacity alert
// carries forecastDaysToFull only once the usage trend is confident
// (internal/alerts/capacity_forecast.go). This reads that alert while it carries
// a forecast; it never estimates a runway of its own. No countdown does not mean
// the runway is safe: a store over the usage threshold alerts on usage instead
// (the percentage already shows it), and a younger trend is not yet trusted.

export interface DatastoreFillForecast {
  days: number;
  level: 'warning' | 'critical';
  // "Full in ~5 days"
  label: string;
  // "Full in ~5d": the Used cell's pill, or the phone layout's State cell.
  shortLabel: string;
  // Drawer text: the label plus the observed growth when it is known.
  detail: string;
}

// The poller's instance id is already "pbs-<connection>", so the storage id is
// "<instance id>-<store>". The resource name can be the PBS hostname instead of
// the connection name, so it is not used.
export const backupServerDatastoreAlertIds = (
  server: Resource,
  datastoreName: string,
): string[] => {
  const name = datastoreName.trim();
  const instanceId = server.pbs?.instanceId?.trim();
  if (!name || !instanceId) return [];
  return [`${instanceId}-${name}`, `${instanceId}/${name}`];
};

// The dashed storage id is ambiguous: connection "main" with store "daily-tank"
// and connection "main-daily" with store "tank" share "pbs-main-daily-tank". The
// capacity alert names its exact "<instance id>/<store>" alias in its policy
// aliases, and otherwise its instance and store name, so require one of those.
export const isBackupServerDatastoreAlert = (
  alert: Alert,
  server: Resource,
  datastoreName: string,
): boolean => {
  const name = datastoreName.trim();
  const instanceId = server.pbs?.instanceId?.trim();
  if (!name || !instanceId) return false;
  const aliases = alert.metadata?.storagePolicyAliases;
  if (Array.isArray(aliases)) return aliases.includes(`${instanceId}/${name}`);
  return alert.instance === instanceId && alert.resourceName === name;
};

const dailyGrowth = (alert: Alert): number | null => {
  const value = alert.metadata?.forecastDailyChangePct;
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : null;
};

// The soonest forecast among the datastore's alerts. Acknowledging an alert
// does not change when the store fills, so acknowledged forecasts count too.
export const getDatastoreFillForecast = (
  alerts: readonly Alert[],
): DatastoreFillForecast | null => {
  let soonest: { alert: Alert; days: number } | null = null;
  for (const alert of alerts) {
    const days = getStorageAlertForecastDaysToFull(alert);
    if (days === null) continue;
    if (!soonest || days < soonest.days) soonest = { alert, days };
  }
  if (!soonest) return null;
  const label = describeStorageAlertHeadline(soonest.alert);
  const growth = dailyGrowth(soonest.alert);
  return {
    days: soonest.days,
    level: soonest.alert.level === 'critical' ? 'critical' : 'warning',
    label,
    shortLabel: describeStorageAlertHeadline(soonest.alert, { compact: true }),
    detail: growth === null ? label : `${label}, growing ${growth.toFixed(1)}% a day`,
  };
};

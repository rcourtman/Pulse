import type { Alert, MetricAlertPhase, MetricAlertStatus } from '@/types/api';
import { formatRelativeTime } from '@/utils/format';
import { formatTemperature } from '@/utils/temperature';
import { alertTypeDisplayLabel } from './helpers';

/**
 * A live status older than this is no longer "now": the resource stopped
 * reporting, or Pulse stopped evaluating it. Generous enough for the slowest
 * poll intervals.
 */
export const METRIC_ALERT_STATUS_STALE_MS = 10 * 60 * 1000;

export interface MetricAlertPresentation {
  phase: MetricAlertPhase;
  /** The live status is too old to describe as the current reading. */
  stale: boolean;
  /** Leads with the reading Pulse is evaluating now and where it stands. */
  summary: string;
  /** What clears the alert from here. */
  detail: string;
  /** Short state for compact cells and badges. */
  phaseLabel: string;
  /** The last reading that met the trigger, once the live one differs. */
  lastBreach?: string;
}

const formatMetricValue = (value: number, unit: string | undefined): string => {
  if (unit === '°C') return formatTemperature(value);
  if (unit === '%') {
    return `${Math.abs(value) >= 10 ? Math.round(value) : Number(value.toFixed(1))}%`;
  }
  const rounded = Number(value.toFixed(1));
  return unit ? `${rounded} ${unit}` : String(rounded);
};

const formatSeconds = (seconds: number): string => {
  if (seconds < 60) return `${seconds} ${seconds === 1 ? 'second' : 'seconds'}`;
  if (seconds < 3600) {
    const minutes = Math.round(seconds / 60);
    return `${minutes} ${minutes === 1 ? 'minute' : 'minutes'}`;
  }
  const hours = Number((seconds / 3600).toFixed(1));
  return `${hours} ${hours === 1 ? 'hour' : 'hours'}`;
};

const describeReading = (label: string, status: MetricAlertStatus): string => {
  const value = formatMetricValue(status.value, status.unit);
  const window = status.evaluationWindowSeconds ?? 0;
  if (window <= 0) return `${label} ${value}`;
  const latest =
    typeof status.rawValue === 'number' && Number.isFinite(status.rawValue)
      ? `, latest ${formatMetricValue(status.rawValue, status.unit)}`
      : '';
  return `${label} averaged ${value} over ${formatSeconds(window)}${latest}`;
};

const describeClearRule = (status: MetricAlertStatus, lead: string): string => {
  const recovery = formatMetricValue(status.recovery, status.unit);
  const delay = status.recoveryDelaySeconds ?? 0;
  return delay > 0
    ? `${lead} ${recovery} or lower and stays there for ${formatSeconds(delay)}.`
    : `${lead} ${recovery} or lower.`;
};

const isUsableStatus = (status: MetricAlertStatus | undefined): status is MetricAlertStatus =>
  !!status &&
  Number.isFinite(status.value) &&
  Number.isFinite(status.trigger) &&
  Number.isFinite(status.recovery) &&
  (status.phase === 'breaching' || status.phase === 'latched' || status.phase === 'recovering');

/**
 * When the alert's last reading at or above the trigger was observed.
 * Websocket alerts omit lastSeen, so the live status dates the breach; each
 * candidate is checked before it is chosen, and Go's zero time is rejected.
 */
export const getMetricAlertLastBreachMs = (
  alert: Pick<Alert, 'lastSeen' | 'metricStatus'>,
): number | undefined => {
  for (const candidate of [alert.metricStatus?.lastBreachAt, alert.lastSeen]) {
    const ms = candidate ? Date.parse(candidate) : NaN;
    if (Number.isFinite(ms) && ms > 0) return ms;
  }
  return undefined;
};

/**
 * Describes an open threshold alert from the backend's live evaluation, so
 * every surface says what the reading is now and why the alert is still open
 * instead of repeating the last breach. Returns null when the alert carries
 * no live status (non-threshold alerts, or the moments after a restart).
 */
export function getMetricAlertPresentation(
  alert: Pick<Alert, 'type' | 'value' | 'lastSeen' | 'metricStatus'>,
  now: number = Date.now(),
): MetricAlertPresentation | null {
  const status = alert.metricStatus;
  if (!isUsableStatus(status)) return null;

  const label = alertTypeDisplayLabel(alert.type);
  const reading = describeReading(label, status);
  const trigger = formatMetricValue(status.trigger, status.unit);
  const observedAt = Date.parse(status.observedAt);
  const stale = !Number.isFinite(observedAt) || now - observedAt > METRIC_ALERT_STATUS_STALE_MS;

  let summary: string;
  let detail: string;
  let phaseLabel: string;
  switch (status.phase) {
    case 'breaching':
      summary = `${reading}, above the ${trigger} alert level`;
      detail = describeClearRule(status, 'Clears once it drops to');
      phaseLabel = `Above ${trigger}`;
      break;
    case 'latched':
      summary = `${reading} now, back under the ${trigger} alert level`;
      detail = describeClearRule(status, 'Stays open until it reaches');
      phaseLabel = 'Alert still open';
      break;
    case 'recovering': {
      const delay = status.recoveryDelaySeconds ?? 0;
      const elapsed = status.recoveryElapsedSeconds ?? 0;
      const recovery = formatMetricValue(status.recovery, status.unit);
      summary = `${reading} now, recovering`;
      detail =
        delay > 0
          ? `Clears after ${formatSeconds(delay)} at ${recovery} or lower${
              elapsed > 0 ? `, ${formatSeconds(Math.min(elapsed, delay))} so far` : ''
            }.`
          : `Clears at ${recovery} or lower.`;
      phaseLabel = 'Recovering';
      break;
    }
  }

  if (stale) {
    summary = `Last reading: ${reading}${
      Number.isFinite(observedAt) ? `, ${formatRelativeTime(observedAt)}` : ''
    }`;
    phaseLabel = 'No recent reading';
  }

  let lastBreach: string | undefined;
  if (status.phase !== 'breaching' && Number.isFinite(alert.value)) {
    const breachedAt = getMetricAlertLastBreachMs(alert);
    const when = breachedAt !== undefined ? `, ${formatRelativeTime(breachedAt)}` : '';
    lastBreach = `Last reading at or above ${trigger}: ${formatMetricValue(alert.value, status.unit)}${when}`;
  }

  return { phase: status.phase, stale, summary, detail, phaseLabel, lastBreach };
}

export interface AlertAttentionCopy {
  message: string;
  detail?: string;
  title?: string;
}

/** Copy for an alert row in a drawer's "Needs attention" list. */
export function getAlertAttentionCopy(
  alert: Pick<Alert, 'type' | 'message' | 'value' | 'lastSeen' | 'metricStatus'>,
  now: number = Date.now(),
): AlertAttentionCopy {
  const presentation = getMetricAlertPresentation(alert, now);
  if (!presentation) return { message: alert.message };
  return {
    message: presentation.summary,
    detail: presentation.detail,
    title: [presentation.summary, presentation.detail, presentation.lastBreach]
      .filter(Boolean)
      .join('\n'),
  };
}

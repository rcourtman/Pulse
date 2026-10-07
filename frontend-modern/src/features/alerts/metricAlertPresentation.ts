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
  /** The level that opens the alert, in the metric's unit. */
  alertLevel: string;
  /** The level the reading must reach before the alert can clear. */
  clearLevel: string;
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

// Progress rounds down, so a run four minutes and fifty seconds into a five
// minute delay does not read as finished while the alert is still open.
const formatElapsed = (seconds: number): string => {
  if (seconds < 60) return formatSeconds(seconds);
  if (seconds < 3600) return formatSeconds(Math.floor(seconds / 60) * 60);
  return formatSeconds(Math.floor(seconds / 360) * 360);
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

// A guest raises one disk alert per disk, all named after the guest, so the
// reading names the disk the way the alert message does ("VM disk (/var)").
const readingLabel = (alert: Pick<Alert, 'type' | 'metadata'>): string => {
  const label = alertTypeDisplayLabel(alert.type);
  const disk = alert.type === 'disk' ? alert.metadata?.label : undefined;
  return typeof disk === 'string' && disk.trim() ? `${label} (${disk.trim()})` : label;
};

const isUsableStatus = (status: MetricAlertStatus | undefined): status is MetricAlertStatus =>
  !!status &&
  Number.isFinite(status.value) &&
  Number.isFinite(status.trigger) &&
  Number.isFinite(status.recovery) &&
  (status.phase === 'breaching' || status.phase === 'latched' || status.phase === 'recovering');

/**
 * Describes an open threshold alert from the backend's live evaluation, so
 * every surface says what the reading is now and why the alert is still open
 * instead of repeating the last breach. Returns null when the alert carries
 * no live status (non-threshold alerts, or the moments after a restart).
 *
 * `now` is the caller's clock, normally the `useRelativeTimeNow` reading, and
 * every time-derived part (the stale cut-off and both ages) measures from it.
 * The status stops changing when the resource stops reporting, so a mounted
 * surface that measured from its render time would keep calling an old
 * reading "now".
 */
export function getMetricAlertPresentation(
  alert: Pick<Alert, 'type' | 'value' | 'lastSeen' | 'metricStatus' | 'metadata'>,
  now: number,
): MetricAlertPresentation | null {
  const status = alert.metricStatus;
  if (!isUsableStatus(status)) return null;

  const reading = describeReading(readingLabel(alert), status);
  const trigger = formatMetricValue(status.trigger, status.unit);
  const recovery = formatMetricValue(status.recovery, status.unit);
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
      summary = `${reading} now, recovering`;
      detail =
        delay > 0
          ? `Clears after ${formatSeconds(delay)} at ${recovery} or lower${
              elapsed > 0 ? `, ${formatElapsed(Math.min(elapsed, delay))} so far` : ''
            }.`
          : `Clears at ${recovery} or lower.`;
      phaseLabel = 'Recovering';
      break;
    }
  }

  if (stale) {
    summary = `Last reading: ${reading}${
      Number.isFinite(observedAt) ? `, ${formatRelativeTime(observedAt, { now })}` : ''
    }`;
    phaseLabel = 'No recent reading';
  }

  let lastBreach: string | undefined;
  if (status.phase !== 'breaching' && Number.isFinite(alert.value)) {
    const when = alert.lastSeen ? `, ${formatRelativeTime(alert.lastSeen, { now })}` : '';
    lastBreach = `Last reading at or above ${trigger}: ${formatMetricValue(alert.value, status.unit)}${when}`;
  }

  return {
    phase: status.phase,
    stale,
    summary,
    detail,
    phaseLabel,
    lastBreach,
    alertLevel: trigger,
    clearLevel: recovery,
  };
}

export interface AlertAttentionCopy {
  message: string;
  detail?: string;
  title?: string;
}

/**
 * Copy for an alert row in a drawer's "Needs attention" list. `now` is the
 * drawer's `useRelativeTimeNow` accessor, read only for an alert that carries
 * a live status, so a list whose alerts have none does not re-render on every
 * tick of the shared clock.
 */
export function getAlertAttentionCopy(
  alert: Pick<Alert, 'type' | 'message' | 'value' | 'lastSeen' | 'metricStatus' | 'metadata'>,
  now: () => number,
): AlertAttentionCopy {
  const presentation = alert.metricStatus ? getMetricAlertPresentation(alert, now()) : null;
  if (!presentation) return { message: alert.message };
  return {
    message: presentation.summary,
    detail: presentation.detail,
    title: [presentation.summary, presentation.detail, presentation.lastBreach]
      .filter(Boolean)
      .join('\n'),
  };
}

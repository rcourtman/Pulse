import type { CapacityForecast } from '@/api/patrol';

export type CapacityForecastTone = 'critical' | 'warning' | 'info';

export interface CapacityForecastPresentation {
  /** Short scannable direction: "Filling up", "Stable", "Declining". */
  direction: string;
  /** Urgency detail, e.g. "~5 days to full at +1.2%/day". */
  detail: string;
  /** Current utilization readout, e.g. "86% used". */
  current: string;
  tone: CapacityForecastTone;
}

function formatDays(days: number): string {
  if (days <= 0) return 'now';
  if (days === 1) return '1 day';
  if (days < 7) return `${days} days`;
  if (days < 14) return '~1 week';
  if (days < 30) return `~${Math.round(days / 7)} weeks`;
  const months = Math.round(days / 30);
  if (months <= 1) return '~1 month';
  if (months < 12) return `~${months} months`;
  return 'over a year';
}

function formatRate(changePerDay: number): string {
  const sign = changePerDay > 0 ? '+' : '';
  return `${sign}${changePerDay.toFixed(changePerDay >= 1 || changePerDay <= -1 ? 1 : 2)}%/day`;
}

/**
 * Turn a deterministic capacity forecast into a plain operator-facing urgency
 * line. Returns null when no forecast applies so callers can skip rendering.
 *
 * This is the canonical projection for the capacity-forecast signal: the
 * backend owns the computation (linear regression over utilization samples),
 * this helper owns the operator vocabulary. Kept free of component concerns so
 * it can be exercised directly in tests and reused across surfaces.
 */
export function presentCapacityForecast(
  fc?: CapacityForecast | null,
): CapacityForecastPresentation | null {
  if (!fc) return null;

  const days = fc.days_to_full;
  const change = fc.daily_change ?? 0;
  const current = `${Math.round(fc.current_pct)}% used`;

  if (days > 0) {
    const tone: CapacityForecastTone = days <= 7 ? 'critical' : days <= 30 ? 'warning' : 'info';
    return {
      direction: 'Filling up',
      detail: `${formatDays(days)} to full at ${formatRate(change)}`,
      current,
      tone,
    };
  }

  if (change < -0.1) {
    return {
      direction: 'Declining',
      detail: `usage falling ${formatRate(Math.abs(change))}`,
      current,
      tone: 'info',
    };
  }

  return {
    direction: 'Stable',
    detail: 'no clear fill trend',
    current,
    tone: 'info',
  };
}

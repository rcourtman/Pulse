const RATE_UNITS = [
  { threshold: 1e9, suffix: 'GB/s', precision: 1 },
  { threshold: 1e6, suffix: 'MB/s', precision: 1 },
  { threshold: 1e3, suffix: 'KB/s', precision: 0 },
] as const;

export function formatThroughputRate(bytesPerSecond: number): string {
  if (!Number.isFinite(bytesPerSecond) || bytesPerSecond < 0) {
    return '0 B/s';
  }

  for (const unit of RATE_UNITS) {
    if (bytesPerSecond >= unit.threshold) {
      return `${(bytesPerSecond / unit.threshold).toFixed(unit.precision)} ${unit.suffix}`;
    }
  }

  return `${Math.round(bytesPerSecond)} B/s`;
}

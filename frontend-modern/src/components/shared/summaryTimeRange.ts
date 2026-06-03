export const SUMMARY_TIME_RANGES = ['1h', '12h', '24h', '7d'] as const;

export type SummaryTimeRange = (typeof SUMMARY_TIME_RANGES)[number];

export const SUMMARY_TIME_RANGE_LABEL: Record<SummaryTimeRange, string> = {
  '1h': '1h',
  '12h': '12h',
  '24h': '24h',
  '7d': '7d',
};

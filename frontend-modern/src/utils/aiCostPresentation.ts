import { segmentedButtonClass } from '@/utils/segmentedButton';

export const AI_COST_EMPTY_STATE = 'Usage data will appear here once activity is recorded.';
export const AI_COST_DAILY_USD_EMPTY_STATE =
  'Daily cost trend will appear here once activity is recorded.';
export const AI_COST_DAILY_TOKEN_EMPTY_STATE =
  'Daily token trend will appear here once activity is recorded.';

export function getAICostLoadingState() {
  return {
    text: 'Loading usage data…',
  } as const;
}

export function getAICostRangeButtonClass(selected: boolean, disabled = false): string {
  return `min-h-10 sm:min-h-9 min-w-10 border px-2.5 py-2 text-sm ${segmentedButtonClass(selected, disabled)}`;
}

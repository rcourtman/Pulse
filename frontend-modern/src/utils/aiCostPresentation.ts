import { segmentedButtonClass } from '@/utils/segmentedButton';

export const AI_COST_PANEL_TITLE = 'Provider Usage & Spend';
export const AI_COST_PANEL_DESCRIPTION =
  'Token usage and estimated spend across providers backing Pulse Assistant and Patrol.';
export const AI_COST_PROVIDER_MODEL_PAIR_LABEL = 'Provider/model pairs';
export const AI_COST_ASSISTANT_USE_CASE_LABEL = 'Pulse Assistant';
export const AI_COST_PATROL_USE_CASE_LABEL = 'Patrol';
export const AI_COST_BUDGET_LABEL = '30-day budget (USD)';
export const AI_COST_RESET_HISTORY_LABEL = 'Reset usage history';
export const AI_COST_EMPTY_STATE =
  'Provider usage data will appear here once Pulse Assistant or Patrol activity is recorded.';
export const AI_COST_DAILY_USD_EMPTY_STATE =
  'Daily spend trend will appear here once provider activity is recorded.';
export const AI_COST_DAILY_TOKEN_EMPTY_STATE =
  'Daily token trend will appear here once provider activity is recorded.';

export function getAICostLoadingState() {
  return {
    text: 'Loading provider usage…',
  } as const;
}

export function getAICostRefreshErrorMessage() {
  return 'Failed to refresh provider usage summary';
}

export function getAICostBudgetNote(rangeDays: number) {
  return `Configured in Assistant & Patrol settings. Pro-rated for ${rangeDays}d:`;
}

export function getAICostResetHistoryConfirmationMessage() {
  return 'Reset provider usage history? A backup will be created in the Pulse config directory.';
}

export function getAICostResetHistorySuccessMessage(backupFile?: string | null) {
  return backupFile
    ? `Provider usage history reset (backup: ${backupFile})`
    : 'Provider usage history reset';
}

export function getAICostResetHistoryErrorMessage() {
  return 'Failed to reset provider usage history';
}

export function getAICostExportHistoryErrorMessage() {
  return 'Failed to export provider usage history';
}

export function buildAICostExportFilename(
  rangeDays: number,
  format: 'csv' | 'json',
  now = new Date(),
) {
  return `pulse-provider-usage-${now.toISOString().split('T')[0]}-${rangeDays}d.${format}`;
}

export function getAICostRangeButtonClass(selected: boolean, disabled = false): string {
  return `min-h-10 sm:min-h-9 min-w-10 border px-2.5 py-2 text-sm ${segmentedButtonClass(selected, disabled)}`;
}

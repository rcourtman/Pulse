import { segmentedButtonClass } from '@/utils/segmentedButton';

export const AI_COST_PANEL_TITLE = 'Provider Usage & Spend';
export const AI_COST_PANEL_DESCRIPTION =
  'Token usage and estimated spend across providers backing Pulse Assistant and Patrol.';
export const AI_COST_PROVIDER_MODEL_PAIR_LABEL = 'Provider/model pairs';
export const AI_COST_TARGET_TABLE_LABEL = 'Usage by task';
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
  return `Configured in Provider & Models settings. Pro-rated for ${rangeDays}d:`;
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

export interface AICostTargetPresentationInput {
  target_type?: string | null;
  target_id?: string | null;
}

export interface AICostTargetPresentation {
  label: string;
  detail?: string;
  rawLabel: string;
}

const TARGET_TYPE_LABELS: Record<string, string> = {
  assistant_session: 'Assistant sessions',
  assistant_session_title: 'Assistant sessions',
  chat: 'Assistant sessions',
  discovery: 'Discovery runs',
  discovery_run: 'Discovery runs',
  patrol: 'Patrol runs',
  patrol_run: 'Patrol runs',
  patrol_session: 'Patrol runs',
  resource: 'Resource checks',
  resource_context: 'Resource checks',
};

const RESOURCE_TYPE_PREFIXES: Record<string, string> = {
  container: 'Container',
  lxc: 'Container',
  vm: 'VM',
};

function humanizeIdentifier(value: string): string {
  return value
    .replace(/[_-]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/\b\w/g, (letter) => letter.toUpperCase());
}

function isOpaqueTargetId(value: string): boolean {
  const trimmed = value.trim();
  if (!trimmed) return false;
  if (/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(trimmed)) {
    return true;
  }
  if (/^[0-9a-f]{20,}$/i.test(trimmed)) return true;
  return trimmed.length > 32 && /^[a-z0-9:_-]+$/i.test(trimmed);
}

function readableTargetType(targetType: string): string {
  const normalized = targetType.trim().toLowerCase();
  return TARGET_TYPE_LABELS[normalized] || humanizeIdentifier(normalized || 'usage');
}

export function getAICostTargetPresentation(
  target: AICostTargetPresentationInput,
): AICostTargetPresentation {
  const targetType = target.target_type?.trim() || 'usage';
  const targetId = target.target_id?.trim() || '';
  const normalizedType = targetType.toLowerCase();
  const rawLabel = targetId ? `${targetType}:${targetId}` : targetType;
  const resourcePrefix = RESOURCE_TYPE_PREFIXES[normalizedType];

  if (resourcePrefix) {
    return {
      label:
        targetId && !isOpaqueTargetId(targetId)
          ? `${resourcePrefix} ${targetId}`
          : `${resourcePrefix}s`,
      rawLabel,
    };
  }

  const label = readableTargetType(targetType);
  const detail = targetId && !isOpaqueTargetId(targetId) ? targetId : undefined;

  return {
    label,
    detail,
    rawLabel,
  };
}

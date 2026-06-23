import { describe, expect, it } from 'vitest';
import {
  AI_COST_ASSISTANT_USE_CASE_LABEL,
  AI_COST_BUDGET_LABEL,
  AI_COST_DAILY_TOKEN_EMPTY_STATE,
  AI_COST_DAILY_USD_EMPTY_STATE,
  AI_COST_EMPTY_STATE,
  AI_COST_PANEL_DESCRIPTION,
  AI_COST_PANEL_TITLE,
  AI_COST_PATROL_USE_CASE_LABEL,
  AI_COST_PROVIDER_MODEL_PAIR_LABEL,
  AI_COST_RESET_HISTORY_LABEL,
  AI_COST_TARGET_TABLE_LABEL,
  buildAICostExportFilename,
  getAICostBudgetNote,
  getAICostExportHistoryErrorMessage,
  getAICostLoadingState,
  getAICostRefreshErrorMessage,
  getAICostRangeButtonClass,
  getAICostResetHistoryConfirmationMessage,
  getAICostResetHistoryErrorMessage,
  getAICostResetHistorySuccessMessage,
  getAICostTargetPresentation,
} from '@/utils/aiCostPresentation';

describe('aiCostPresentation', () => {
  it('returns canonical AI cost range button classes', () => {
    expect(getAICostRangeButtonClass(true)).toContain('inline-flex items-center');
    expect(getAICostRangeButtonClass(true)).toContain('bg-surface');
    expect(getAICostRangeButtonClass(false, true)).toContain('cursor-not-allowed');
  });

  it('exports canonical AI cost presentation copy', () => {
    expect(AI_COST_PANEL_TITLE).toBe('Provider Usage & Spend');
    expect(AI_COST_PANEL_DESCRIPTION).toBe(
      'Token usage and estimated spend across providers backing Pulse Assistant and Patrol.',
    );
    expect(AI_COST_PROVIDER_MODEL_PAIR_LABEL).toBe('Provider/model pairs');
    expect(AI_COST_ASSISTANT_USE_CASE_LABEL).toBe('Pulse Assistant');
    expect(AI_COST_PATROL_USE_CASE_LABEL).toBe('Patrol');
    expect(AI_COST_BUDGET_LABEL).toBe('30-day budget (USD)');
    expect(AI_COST_RESET_HISTORY_LABEL).toBe('Reset usage history');
    expect(AI_COST_TARGET_TABLE_LABEL).toBe('Usage by task');
    expect(AI_COST_EMPTY_STATE).toBe(
      'Provider usage data will appear here once Pulse Assistant or Patrol activity is recorded.',
    );
    expect(AI_COST_DAILY_USD_EMPTY_STATE).toBe(
      'Daily spend trend will appear here once provider activity is recorded.',
    );
    expect(AI_COST_DAILY_TOKEN_EMPTY_STATE).toBe(
      'Daily token trend will appear here once provider activity is recorded.',
    );
    expect(getAICostLoadingState()).toEqual({
      text: 'Loading provider usage…',
    });
    expect(getAICostRefreshErrorMessage()).toBe('Failed to refresh provider usage summary');
    expect(getAICostBudgetNote(7)).toBe(
      'Configured in Provider & Models settings. Pro-rated for 7d:',
    );
    expect(getAICostResetHistoryConfirmationMessage()).toBe(
      'Reset provider usage history? A backup will be created in the Pulse config directory.',
    );
    expect(getAICostResetHistorySuccessMessage('/tmp/backup.json')).toBe(
      'Provider usage history reset (backup: /tmp/backup.json)',
    );
    expect(getAICostResetHistorySuccessMessage()).toBe('Provider usage history reset');
    expect(getAICostResetHistoryErrorMessage()).toBe('Failed to reset provider usage history');
    expect(getAICostExportHistoryErrorMessage()).toBe('Failed to export provider usage history');
    expect(buildAICostExportFilename(30, 'csv', new Date('2026-03-01T12:00:00Z'))).toBe(
      'pulse-provider-usage-2026-03-01-30d.csv',
    );
  });

  it('presents usage targets as product concepts instead of raw internal IDs', () => {
    expect(
      getAICostTargetPresentation({
        target_type: 'assistant_session_title',
        target_id: '7f5941d9-a503-416d-b84e-5a46c9e1e11f',
      }),
    ).toEqual({
      label: 'Assistant sessions',
      rawLabel: 'assistant_session_title:7f5941d9-a503-416d-b84e-5a46c9e1e11f',
    });

    expect(
      getAICostTargetPresentation({
        target_type: 'patrol_run',
        target_id: 'ed21e8612df6450fab4ebd4ae2502259',
      }),
    ).toEqual({
      label: 'Patrol runs',
      rawLabel: 'patrol_run:ed21e8612df6450fab4ebd4ae2502259',
    });

    expect(
      getAICostTargetPresentation({
        target_type: 'vm',
        target_id: '100',
      }),
    ).toEqual({
      label: 'VM 100',
      rawLabel: 'vm:100',
    });
  });
});

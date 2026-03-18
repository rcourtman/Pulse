import { describe, expect, it } from 'vitest';
import {
  getInvestigationMessagesState,
  getInvestigationSectionState,
  getRunHistoryEmptyState,
} from '@/utils/patrolEmptyStatePresentation';

describe('patrolEmptyStatePresentation', () => {
  it('returns investigation message loading and empty states', () => {
    expect(getInvestigationMessagesState(true, false)).toEqual({
      text: 'Loading messages...',
      empty: false,
    });
    expect(getInvestigationMessagesState(false, false)).toEqual({
      text: 'No investigation messages available.',
      empty: true,
    });
  });

  it('returns the patrol run history empty state', () => {
    expect(getRunHistoryEmptyState()).toEqual({
      text: 'No patrol runs yet. Trigger a run to populate history.',
    });
  });

  it('returns investigation section loading and empty states', () => {
    expect(getInvestigationSectionState(true, false)).toEqual({
      text: 'Loading investigation...',
      empty: false,
    });
    expect(getInvestigationSectionState(false, false)).toEqual({
      text: 'No investigation data available. Enable patrol autonomy to investigate findings.',
      empty: true,
    });
  });
});

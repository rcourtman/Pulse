import { describe, expect, it } from 'vitest';
import {
  getRecoveryActivityEmptyState,
  getRecoveryActivityLoadingState,
  getRecoveryHistoryEmptyState,
  getRecoveryPointsFailureState,
  getRecoveryPointsLoadingState,
  getRecoveryProtectedItemsFailureState,
  getRecoveryProtectedItemsLoadingState,
  getRecoveryProtectedItemsEmptyState,
} from '@/utils/recoveryEmptyStatePresentation';

describe('recoveryEmptyStatePresentation', () => {
  it('returns the protection coverage empty state copy', () => {
    expect(getRecoveryProtectedItemsEmptyState()).toEqual({
      title: 'No protection coverage yet',
      description: 'Pulse has not observed recovery coverage for this org yet.',
    });
  });

  it('returns the recovery loading-state copy', () => {
    expect(getRecoveryProtectedItemsLoadingState()).toEqual({
      text: 'Loading protection coverage...',
    });
    expect(getRecoveryActivityLoadingState()).toEqual({
      text: 'Loading recovery activity...',
    });
    expect(getRecoveryPointsLoadingState()).toEqual({
      text: 'Loading recovery points...',
    });
  });

  it('returns the recovery failure-state copy', () => {
    expect(getRecoveryProtectedItemsFailureState()).toEqual({
      title: 'Failed to load protection coverage',
    });
    expect(getRecoveryPointsFailureState()).toEqual({
      title: 'Failed to load recovery points',
    });
  });

  it('returns the recovery activity empty-state copy', () => {
    expect(getRecoveryActivityEmptyState()).toEqual({
      text: 'No recovery activity in the selected window.',
    });
  });

  it('returns the recovery-history filter empty state copy', () => {
    expect(getRecoveryHistoryEmptyState()).toEqual({
      title: 'No recovery history matches your filters',
      description: 'Adjust your search, platform, method, status, or verification filters.',
    });
  });
});

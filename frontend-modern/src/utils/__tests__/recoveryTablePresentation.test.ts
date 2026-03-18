import { describe, expect, it } from 'vitest';
import type { ProtectionRollup, RecoveryPoint } from '@/types/recovery';
import {
  getRecoveryArtifactColumnHeaderClass,
  getRecoveryArtifactRowClass,
  getRecoveryEventTimeTextClass,
  getRecoveryGroupNoTimestampLabel,
  getRecoveryHistorySearchPlaceholder,
  getRecoveryProtectedSearchPlaceholder,
  getRecoveryRollupAgeTextClass,
  getRecoveryRollupIssueTone,
  getRecoverySearchHistoryEmptyMessage,
  getRecoverySubjectTypeBadgeClass,
  getRecoverySubjectTypeLabel,
  isRecoveryRollupStale,
  RECOVERY_ADVANCED_FILTER_FIELD_CLASS,
  RECOVERY_ADVANCED_FILTER_LABEL_CLASS,
  RECOVERY_GROUP_HEADER_ROW_CLASS,
  RECOVERY_GROUP_HEADER_TEXT_CLASS,
  RECOVERY_GROUP_NO_TIMESTAMP_LABEL,
  RECOVERY_HISTORY_SEARCH_PLACEHOLDER,
  RECOVERY_PROTECTED_SEARCH_PLACEHOLDER,
  RECOVERY_SEARCH_HISTORY_EMPTY_MESSAGE,
} from '@/utils/recoveryTablePresentation';

describe('recoveryTablePresentation', () => {
  it('exposes shared recovery table classes', () => {
    expect(RECOVERY_GROUP_HEADER_ROW_CLASS).toContain('bg-surface-alt');
    expect(RECOVERY_GROUP_HEADER_TEXT_CLASS).toContain('font-semibold');
    expect(RECOVERY_ADVANCED_FILTER_LABEL_CLASS).toContain('text-muted');
    expect(RECOVERY_ADVANCED_FILTER_FIELD_CLASS).toContain('focus:border-blue-500');
    expect(RECOVERY_GROUP_NO_TIMESTAMP_LABEL).toBe('No Timestamp');
    expect(RECOVERY_PROTECTED_SEARCH_PLACEHOLDER).toBe('Search protected items...');
    expect(RECOVERY_HISTORY_SEARCH_PLACEHOLDER).toBe('Search recovery history...');
    expect(RECOVERY_SEARCH_HISTORY_EMPTY_MESSAGE).toBe('Recent searches appear here.');
    expect(getRecoveryGroupNoTimestampLabel()).toBe('No Timestamp');
    expect(getRecoveryProtectedSearchPlaceholder()).toBe('Search protected items...');
    expect(getRecoveryHistorySearchPlaceholder()).toBe('Search recovery history...');
    expect(getRecoverySearchHistoryEmptyMessage()).toBe('Recent searches appear here.');
  });

  it('derives time text classes from event recency', () => {
    const now = Date.UTC(2026, 2, 9, 12, 0, 0);
    expect(getRecoveryEventTimeTextClass(now - 2 * 60 * 60 * 1000, now)).toContain(
      'text-emerald-600',
    );
    expect(getRecoveryEventTimeTextClass(now - 3 * 24 * 60 * 60 * 1000, now)).toContain(
      'text-amber-600',
    );
    expect(getRecoveryEventTimeTextClass(now - 14 * 24 * 60 * 60 * 1000, now)).toContain(
      'text-orange-600',
    );
    expect(getRecoveryEventTimeTextClass(0, now)).toBe('text-muted');
  });

  it('derives workload subject badges from canonical workload presentation', () => {
    const point = {
      display: { subjectType: 'proxmox-vm' },
      subjectRef: { type: 'proxmox-vm' },
    } as RecoveryPoint;

    expect(getRecoverySubjectTypeLabel(point)).toBe('VM');
    expect(getRecoverySubjectTypeBadgeClass(point)).toBe(
      'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300',
    );
  });

  it('normalizes proxmox lxc subjects to the canonical container badge', () => {
    const point = {
      display: { subjectType: 'proxmox-lxc' },
      subjectRef: { type: 'proxmox-lxc' },
    } as RecoveryPoint;

    expect(getRecoverySubjectTypeLabel(point)).toBe('Container');
    expect(getRecoverySubjectTypeBadgeClass(point)).toBe(
      'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
    );
  });

  it('falls back cleanly for unknown subject types', () => {
    const point = {
      display: { subjectType: 'custom-thing' },
      subjectRef: { type: 'custom-thing' },
    } as RecoveryPoint;

    expect(getRecoverySubjectTypeLabel(point)).toBe('Custom Thing');
    expect(getRecoverySubjectTypeBadgeClass(point)).toBe('bg-surface-alt text-base-content');
  });

  it('derives artifact header and row classes', () => {
    expect(getRecoveryArtifactColumnHeaderClass('time')).toContain('text-right');
    expect(getRecoveryArtifactColumnHeaderClass('type')).toContain('w-[72px]');
    expect(getRecoveryArtifactColumnHeaderClass('subject')).toContain('w-[248px]');
    expect(getRecoveryArtifactRowClass(true)).toContain('outline-blue-200/80');
    expect(getRecoveryArtifactRowClass(false)).toBe('hover:bg-surface-hover');
  });

  it('derives rollup issue tone, stale state, and age class from outcome and timing', () => {
    const now = Date.UTC(2026, 2, 9, 12, 0, 0);
    const failed = {
      lastOutcome: 'failed',
      lastSuccessAt: '2026-03-09T10:00:00.000Z',
    } as ProtectionRollup;
    const stale = {
      lastOutcome: 'success',
      lastSuccessAt: '2026-02-28T12:00:00.000Z',
    } as ProtectionRollup;
    const aging = {
      lastOutcome: 'success',
      lastSuccessAt: '2026-03-06T12:00:00.000Z',
    } as ProtectionRollup;

    expect(getRecoveryRollupIssueTone(failed, now)).toBe('rose');
    expect(getRecoveryRollupIssueTone(stale, now)).toBe('amber');
    expect(isRecoveryRollupStale(stale, now)).toBe(true);
    expect(getRecoveryRollupAgeTextClass(stale, now)).toContain('text-rose-700');
    expect(getRecoveryRollupAgeTextClass(aging, now)).toContain('text-amber-700');
  });
});
